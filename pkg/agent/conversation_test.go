// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/gollm"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sessions"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/tools"
)

type mockLLM struct{}

func (mockLLM) Close() error { return nil }

func (mockLLM) StartChat(systemPrompt, model string) gollm.Chat { return mockChat{} }

func (mockLLM) GenerateCompletion(ctx context.Context, req *gollm.CompletionRequest) (gollm.CompletionResponse, error) {
	return nil, nil
}

func (mockLLM) SetResponseSchema(schema *gollm.Schema) error { return nil }

func (mockLLM) ListModels(ctx context.Context) ([]string, error) { return []string{"mock-model"}, nil }

type mockChat struct{}

func (mockChat) Initialize(messages []*api.Message) error {
	return nil
}

func (mockChat) Send(ctx context.Context, contents ...any) (gollm.ChatResponse, error) {
	return nil, nil
}

func (mockChat) SendStreaming(ctx context.Context, contents ...any) (gollm.ChatResponseIterator, error) {
	return nil, nil
}

func (mockChat) SetFunctionDefinitions(functionDefinitions []*gollm.FunctionDefinition) error {
	return nil
}

func (mockChat) IsRetryableError(error) bool { return false }

func newTestAgent() *Agent {
	chatStore := sessions.NewInMemoryChatStore()
	chatStore.AddChatMessage(&api.Message{Source: api.MessageSourceUser, Payload: "previous message"})
	return &Agent{
		Input:            make(chan any, 1),
		Output:           make(chan any, 10),
		session:          &api.Session{AgentState: api.AgentStateIdle, ChatMessageStore: chatStore},
		LLM:              mockLLM{},
		llmChat:          mockChat{},
		Model:            "test-model",
		Tools:            tools.Default(),
		ChatMessageStore: chatStore,
	}
}

func TestRunHandlesMetaQueries(t *testing.T) {
	testCases := []struct {
		name                    string
		query                   string
		expectedResponseContain string
		checkSideEffect         func(t *testing.T, a *Agent)
	}{
		{
			name:                    "model",
			query:                   "model",
			expectedResponseContain: "Current model is `test-model`",
		},
		{
			name:                    "models",
			query:                   "models",
			expectedResponseContain: "mock-model",
		},
		{
			name:                    "tools",
			query:                   "tools",
			expectedResponseContain: "kubectl", // Check for a default tool
		},
		{
			name:                    "clear",
			query:                   "clear",
			expectedResponseContain: "Cleared the conversation.",
			checkSideEffect: func(t *testing.T, a *Agent) {
				if len(a.session.ChatMessageStore.ChatMessages()) != 0 {
					t.Errorf("expected chat messages to be cleared, but got %d messages", len(a.session.ChatMessageStore.ChatMessages()))
				}
			},
		},
		{
			name:                    "reset",
			query:                   "reset",
			expectedResponseContain: "Cleared the conversation.",
			checkSideEffect: func(t *testing.T, a *Agent) {
				if len(a.session.ChatMessageStore.ChatMessages()) != 0 {
					t.Errorf("expected chat messages to be cleared, but got %d messages", len(a.session.ChatMessageStore.ChatMessages()))
				}
			},
		},
		{
			name:  "exit",
			query: "exit",
			checkSideEffect: func(t *testing.T, a *Agent) {
				if a.AgentState() != api.AgentStateExited {
					t.Errorf("expected agent state to be Exited, but got %s", a.AgentState())
				}
			},
			expectedResponseContain: "It has been a pleasure assisting you",
		},
		{
			name:  "quit",
			query: "quit",
			checkSideEffect: func(t *testing.T, a *Agent) {
				if a.AgentState() != api.AgentStateExited {
					t.Errorf("expected agent state to be Exited, but got %s", a.AgentState())
				}
			},
			expectedResponseContain: "It has been a pleasure assisting you",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			a := newTestAgent()

			if err := a.Run(ctx, ""); err != nil {
				t.Fatalf("Run returned error: %v", err)
			}

			// Wait for the agent to prompt for input
			timeout := time.After(time.Second)
		WaitForPrompt:
			for {
				select {
				case <-timeout:
					t.Fatalf("did not receive prompt for %q", tc.query)
				case msg := <-a.Output:
					m := msg.(*api.Message)
					if m.Type == api.MessageTypeUserInputRequest && m.Payload.(string) == ">>>" {
						a.Input <- &api.UserInputResponse{Query: tc.query}
						break WaitForPrompt
					}
				}
			}

			responseTimeout := time.After(time.Second)
			var foundResponse bool
		WaitForResponse:
			for {
				select {
				case <-responseTimeout:
					if tc.expectedResponseContain != "" && !foundResponse {
						t.Fatalf("timed out waiting for response for query %q", tc.query)
					}
					break WaitForResponse
				case msg, ok := <-a.Output:
					if !ok { // channel closed
						break WaitForResponse
					}
					m := msg.(*api.Message)
					if m.Source == api.MessageSourceAgent && m.Type == api.MessageTypeText {
						payload, ok := m.Payload.(string)
						if !ok {
							t.Fatalf("unexpected payload type: %T", m.Payload)
						}
						if strings.Contains(payload, tc.expectedResponseContain) {
							foundResponse = true
							break WaitForResponse
						}
					}
				}
			}

			if !foundResponse && tc.expectedResponseContain != "" {
				t.Errorf("did not find expected response for query %q", tc.query)
			}

			if tc.checkSideEffect != nil {
				// Give a moment for state to update for async operations
				if tc.name == "exit" || tc.name == "quit" {
					time.Sleep(50 * time.Millisecond)
				}
				tc.checkSideEffect(t, a)
			}
		})
	}
}
