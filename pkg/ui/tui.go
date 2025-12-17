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

package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sessions"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Simple styles - no color queries
var (
	userPrefix = lipgloss.NewStyle().Bold(true).SetString("You: ")
	aiPrefix   = lipgloss.NewStyle().Bold(true).SetString("AI: ")
)

type TUI struct {
	program        *tea.Program
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context
	cancel         context.CancelFunc
}

func NewTUI(manager *agent.AgentManager, sessionManager *sessions.SessionManager) *TUI {
	return &TUI{
		manager:        manager,
		sessionManager: sessionManager,
	}
}

func (t *TUI) Run(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	m := newModel(t.manager, t.sessionManager, t.ctx)
	t.program = tea.NewProgram(m, tea.WithAltScreen())

	// Listen to agent output
	if t.manager != nil {
		t.manager.SetAgentCreatedCallback(func(a *agent.Agent) {
			go func() {
				for msg := range a.Output {
					if apiMsg, ok := msg.(*api.Message); ok {
						t.program.Send(apiMsg)
					}
				}
			}()
		})
	}

	_, err := t.program.Run()
	t.cancel()
	return err
}

func (t *TUI) ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// model
type model struct {
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context
	sessionID      string

	viewport viewport.Model
	textarea textarea.Model
	messages []string

	width  int
	height int
	ready  bool
}

func newModel(manager *agent.AgentManager, sessionManager *sessions.SessionManager, ctx context.Context) model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.Prompt = "> "
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return model{
		manager:        manager,
		sessionManager: sessionManager,
		ctx:            ctx,
		textarea:       ta,
		viewport:       viewport.New(80, 20),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.initSession)
}

func (m model) initSession() tea.Msg {
	if m.sessionManager == nil {
		return nil
	}
	session, err := m.sessionManager.NewSession(sessions.Metadata{})
	if err != nil {
		return nil
	}
	if m.manager != nil {
		m.manager.GetAgent(m.ctx, session.ID)
	}
	return sessionReady(session.ID)
}

type sessionReady string

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 3 // room for input
		m.textarea.SetWidth(msg.Width - 2)
		m.ready = true

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if val := m.textarea.Value(); val != "" {
				m.messages = append(m.messages, userPrefix.String()+val)
				m.textarea.Reset()
				m.updateViewport()
				return m, m.sendMessage(val)
			}
		}

	case sessionReady:
		m.sessionID = string(msg)

	case *api.Message:
		m.handleMessage(msg)
		m.updateViewport()
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) handleMessage(msg *api.Message) {
	if msg.Type == api.MessageTypeUserInputRequest {
		return
	}

	var text string
	switch p := msg.Payload.(type) {
	case string:
		text = p
	default:
		return
	}

	if text == "" {
		return
	}

	switch msg.Type {
	case api.MessageTypeText:
		if msg.Source == api.MessageSourceUser {
			// Already shown
		} else {
			m.messages = append(m.messages, aiPrefix.String()+text)
		}
	case api.MessageTypeToolCallRequest:
		m.messages = append(m.messages, "→ "+text)
	}
}

func (m *model) updateViewport() {
	m.viewport.SetContent(strings.Join(m.messages, "\n\n"))
	m.viewport.GotoBottom()
}

func (m model) sendMessage(query string) tea.Cmd {
	return func() tea.Msg {
		if m.manager == nil || m.sessionID == "" {
			return nil
		}
		agent, err := m.manager.GetAgent(m.ctx, m.sessionID)
		if err != nil {
			return nil
		}
		agent.Input <- &api.UserInputResponse{Query: query}
		return nil
	}
}

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	return fmt.Sprintf("%s\n\n%s", m.viewport.View(), m.textarea.View())
}
