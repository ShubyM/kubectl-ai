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
	"os"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/GoogleCloudPlatform/kubectl-ai/internal/mocks"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sessions"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/tools"
)

func TestHandleMetaQuery(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		query        string
		expectations func(t *testing.T) *Agent
		verify       func(t *testing.T, a *Agent, answer string)
		expect       string
	}{
		{
			name:   "clear",
			query:  "clear",
			expect: "Cleared the conversation.",
			expectations: func(t *testing.T) *Agent {
				ctrl := gomock.NewController(t)
				t.Cleanup(ctrl.Finish)
				store := mocks.NewMockChatMessageStore(ctrl)
				chat := mocks.NewMockChat(ctrl)
				store.EXPECT().ClearChatMessages().Return(nil)
				store.EXPECT().ChatMessages().Return([]*api.Message{})
				chat.EXPECT().Initialize([]*api.Message{})
				a := &Agent{llmChat: chat}
				a.session = &api.Session{ChatMessageStore: store}
				return a
			},
		},
		{
			name:   "exit",
			query:  "exit",
			expect: "It has been a pleasure assisting you. Have a great day!",
			expectations: func(t *testing.T) *Agent {
				a := &Agent{}
				a.session = &api.Session{}
				return a
			},
			verify: func(t *testing.T, a *Agent, _ string) {
				if a.AgentState() != api.AgentStateExited {
					t.Fatalf("expected agent to exit")
				}
			},
		},
		{
			name:   "model",
			query:  "model",
			expect: "Current model is `test-model`",
			expectations: func(t *testing.T) *Agent {
				a := &Agent{Model: "test-model"}
				a.session = &api.Session{}
				return a
			},
		},
		{
			name:   "models",
			query:  "models",
			expect: "Available models:\n\n  - a\n  - b\n\n",
			expectations: func(t *testing.T) *Agent {
				ctrl := gomock.NewController(t)
				t.Cleanup(ctrl.Finish)
				llm := mocks.NewMockClient(ctrl)
				llm.EXPECT().ListModels(ctx).Return([]string{"a", "b"}, nil)
				a := &Agent{LLM: llm}
				a.session = &api.Session{}
				return a
			},
		},
		{
			name:   "tools",
			query:  "tools",
			expect: "Available tools:",
			expectations: func(t *testing.T) *Agent {
				a := &Agent{Tools: tools.Default()}
				a.session = &api.Session{}
				return a
			},
			verify: func(t *testing.T, _ *Agent, answer string) {
				if !strings.Contains(answer, "kubectl") {
					t.Fatalf("expected kubectl tool in output: %q", answer)
				}
			},
		},
		{
			name:   "session",
			query:  "session",
			expect: "Current session:",
			expectations: func(t *testing.T) *Agent {
				oldHome := os.Getenv("HOME")
				t.Cleanup(func() { os.Setenv("HOME", oldHome) })
				home := t.TempDir()
				os.Setenv("HOME", home)
				manager, err := sessions.NewSessionManager()
				if err != nil {
					t.Fatalf("creating session manager: %v", err)
				}
				sess, err := manager.NewSession(sessions.Metadata{ProviderID: "p", ModelID: "m"})
				if err != nil {
					t.Fatalf("creating session: %v", err)
				}
				a := &Agent{ChatMessageStore: sess}
				a.session = &api.Session{ChatMessageStore: sess}
				return a
			},
			verify: func(t *testing.T, _ *Agent, answer string) {
				if !strings.Contains(answer, "ID:") {
					t.Fatalf("expected session info, got %q", answer)
				}
			},
		},
		{
			name:   "sessions",
			query:  "sessions",
			expect: "Available sessions:",
			expectations: func(t *testing.T) *Agent {
				oldHome := os.Getenv("HOME")
				t.Cleanup(func() { os.Setenv("HOME", oldHome) })
				home := t.TempDir()
				os.Setenv("HOME", home)
				manager, err := sessions.NewSessionManager()
				if err != nil {
					t.Fatalf("creating session manager: %v", err)
				}
				_, err = manager.NewSession(sessions.Metadata{ProviderID: "p1", ModelID: "m1"})
				if err != nil {
					t.Fatalf("creating session: %v", err)
				}
				_, err = manager.NewSession(sessions.Metadata{ProviderID: "p2", ModelID: "m2"})
				if err != nil {
					t.Fatalf("creating session: %v", err)
				}
				a := &Agent{}
				a.session = &api.Session{ChatMessageStore: sessions.NewInMemoryChatStore()}
				return a
			},
			verify: func(t *testing.T, _ *Agent, answer string) {
				if !strings.Contains(answer, "Available sessions:") {
					t.Fatalf("unexpected answer: %q", answer)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := tt.expectations(t)
			ans, handled, err := a.handleMetaQuery(ctx, tt.query)
			if err != nil {
				t.Fatalf("handleMetaQuery returned error: %v", err)
			}
			if !handled {
				t.Fatalf("expected query %q to be handled", tt.query)
			}
			if tt.expect != "" && !strings.Contains(ans, tt.expect) {
				t.Fatalf("expected %q to contain %q", ans, tt.expect)
			}
			if tt.verify != nil {
				tt.verify(t, a, ans)
			}
		})
	}
}
