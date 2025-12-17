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

var (
	// Input box style
	inputBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	// Status bar style
	statusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	// Message prefixes
	userStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true)

	aiStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("35")).
		Bold(true)

	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))
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

type model struct {
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context
	sessionID      string
	modelName      string

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
	ta.Prompt = ""
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()

	return model{
		manager:        manager,
		sessionManager: sessionManager,
		ctx:            ctx,
		textarea:       ta,
		viewport:       viewport.New(80, 20),
		modelName:      "gemini-2.5-pro", // default, will be updated
	}
}

func (m model) Init() tea.Cmd {
	return m.initSession
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
	return sessionReady{id: session.ID, model: session.ModelID}
}

type sessionReady struct {
	id    string
	model string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 6 // room for input + status
		m.textarea.SetWidth(msg.Width - 4)
		m.ready = true
		m.updateViewport()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			if val := m.textarea.Value(); val != "" {
				m.messages = append(m.messages, userStyle.Render("You: ")+val)
				m.textarea.Reset()
				m.updateViewport()
				return m, m.sendMessage(val)
			}
		}

	case sessionReady:
		m.sessionID = msg.id
		if msg.model != "" {
			m.modelName = msg.model
		}
		return m, nil

	case *api.Message:
		m.handleMessage(msg)
		m.updateViewport()
		return m, nil
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
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
		if msg.Source != api.MessageSourceUser {
			m.messages = append(m.messages, aiStyle.Render("AI: ")+text)
		}
	case api.MessageTypeToolCallRequest:
		m.messages = append(m.messages, toolStyle.Render("› "+text))
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
		return ""
	}

	// Status bar at top
	status := statusBar.Render(fmt.Sprintf("Model: %s", m.modelName))

	// Input box at bottom
	input := inputBox.Width(m.width - 2).Render(m.textarea.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		status,
		"",
		m.viewport.View(),
		"",
		input,
	)
}
