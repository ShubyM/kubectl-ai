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

// Styles with fixed colors (no terminal queries)
var (
	// User bubble - right aligned, blue-ish
	userBubble = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1).
			MarginLeft(2)

	userLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Bold(true)

	// AI bubble - left aligned, gray
	aiBubble = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1).
			MarginRight(2)

	aiLabel = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Bold(true)

	// Tool style - subtle
	toolStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)

	// Input area
	inputBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
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

	viewport viewport.Model
	textarea textarea.Model
	messages []chatMessage

	width  int
	height int
	ready  bool
}

type chatMessage struct {
	source  string // "user", "ai", "tool"
	content string
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
	return sessionReady(session.ID)
}

type sessionReady string

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 5 // room for input box
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
				m.messages = append(m.messages, chatMessage{source: "user", content: val})
				m.textarea.Reset()
				m.updateViewport()
				return m, m.sendMessage(val)
			}
		}

	case sessionReady:
		m.sessionID = string(msg)
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
			m.messages = append(m.messages, chatMessage{source: "ai", content: text})
		}
	case api.MessageTypeToolCallRequest:
		m.messages = append(m.messages, chatMessage{source: "tool", content: text})
	}
}

func (m *model) updateViewport() {
	var rendered []string
	maxWidth := m.width - 4
	if maxWidth < 20 {
		maxWidth = 20
	}
	bubbleWidth := maxWidth * 3 / 4 // bubbles take 75% width max

	for _, msg := range m.messages {
		rendered = append(rendered, m.renderMessage(msg, bubbleWidth, maxWidth))
	}

	m.viewport.SetContent(strings.Join(rendered, "\n\n"))
	m.viewport.GotoBottom()
}

func (m model) renderMessage(msg chatMessage, bubbleWidth, totalWidth int) string {
	// Wrap text to bubble width
	wrapped := wordWrap(msg.content, bubbleWidth-2)

	switch msg.source {
	case "user":
		bubble := userBubble.Width(bubbleWidth).Render(wrapped)
		// Right align
		padding := totalWidth - lipgloss.Width(bubble)
		if padding < 0 {
			padding = 0
		}
		return strings.Repeat(" ", padding) + bubble

	case "ai":
		return aiBubble.Width(bubbleWidth).Render(wrapped)

	case "tool":
		return toolStyle.Render("⚡ " + msg.content)

	default:
		return msg.content
	}
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

	input := inputBorder.Width(m.width - 2).Render(m.textarea.View())

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewport.View(),
		"",
		input,
	)
}

// wordWrap wraps text to the specified width
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		lineLen := 0
		for j, word := range words {
			wordLen := len(word)
			if j > 0 && lineLen+1+wordLen > width {
				result.WriteString("\n")
				lineLen = 0
			} else if j > 0 {
				result.WriteString(" ")
				lineLen++
			}
			result.WriteString(word)
			lineLen += wordLen
		}
	}

	return result.String()
}
