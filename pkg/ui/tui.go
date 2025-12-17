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
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/sessions"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/klog/v2"
)

// View represents which panel is currently focused
type viewState int

const (
	viewChat viewState = iota
	viewSidebar
	viewOptions
)

// Color palette for a modern, sleek look
var (
	// Primary colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Amber

	// Neutral colors
	textColor      = lipgloss.Color("#F9FAFB") // Almost white
	textMutedColor = lipgloss.Color("#9CA3AF") // Gray
	borderColor    = lipgloss.Color("#4B5563") // Border gray
	highlightColor = lipgloss.Color("#6366F1") // Indigo

	// Sidebar styles
	sidebarWidth = 28
	sidebarStyle = lipgloss.NewStyle().
			Width(sidebarWidth).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(1, 1)

	sidebarActiveStyle = lipgloss.NewStyle().
				Width(sidebarWidth).
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1, 1)

	sidebarTitleStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				Padding(0, 0, 1, 0)

	sessionItemStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Padding(0, 1)

	sessionItemSelectedStyle = lipgloss.NewStyle().
					Foreground(textColor).
					Background(highlightColor).
					Bold(true).
					Padding(0, 1)

	sessionItemActiveStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Padding(0, 1)

	// Chat panel styles
	chatPanelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	chatPanelActiveStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	// Message styles
	userMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#60A5FA")). // Blue
				Bold(true)

	aiMessageStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	systemMessageStyle = lipgloss.NewStyle().
				Foreground(textMutedColor).
				Italic(true)

	// Input styles
	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Padding(0, 1)

	inputActiveStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1)

	// Status bar
	statusBarStyle = lipgloss.NewStyle().
			Foreground(textMutedColor).
			Padding(0, 1)

	// Help text
	helpStyle = lipgloss.NewStyle().
			Foreground(textMutedColor)

	// Option list styles
	optionStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 2)

	optionSelectedStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(primaryColor).
				Bold(true).
				Padding(0, 2)

	// Spinner
	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)
)

// Key bindings
type keyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Tab         key.Binding
	ShiftTab    key.Binding
	Enter       key.Binding
	NewSession  key.Binding
	DeleteSess  key.Binding
	Quit        key.Binding
	Help        key.Binding
	ScrollUp    key.Binding
	ScrollDown  key.Binding
	PageUp      key.Binding
	PageDown    key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch panel"),
	),
	ShiftTab: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "switch panel"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "send/select"),
	),
	NewSession: key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "new session"),
	),
	DeleteSess: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "delete session"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "ctrl+q"),
		key.WithHelp("ctrl+c", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("ctrl+h", "?"),
		key.WithHelp("?", "help"),
	),
	ScrollUp: key.NewBinding(
		key.WithKeys("ctrl+u"),
		key.WithHelp("ctrl+u", "scroll up"),
	),
	ScrollDown: key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", "scroll down"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown"),
		key.WithHelp("pgdn", "page down"),
	),
}

// Message types for async updates
type (
	sessionListMsg      []*api.Session
	agentOutputMsg      *api.Message
	sessionSwitchedMsg  string
	sessionCreatedMsg   *api.Session
	sessionDeletedMsg   string
	tickMsg             time.Time
	errMsg              error
)

// TUI is the modern terminal user interface with multi-session support
type TUI struct {
	program        *tea.Program
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context
	cancel         context.CancelFunc

	// For listening to agent outputs
	outputListeners map[string]context.CancelFunc
	listenerMu      sync.Mutex
}

// NewTUI creates a new TUI with multi-session support
func NewTUI(manager *agent.AgentManager, sessionManager *sessions.SessionManager) *TUI {
	return &TUI{
		manager:         manager,
		sessionManager:  sessionManager,
		outputListeners: make(map[string]context.CancelFunc),
	}
}

func (t *TUI) Run(ctx context.Context) error {
	t.ctx, t.cancel = context.WithCancel(ctx)

	model := newModel(t.manager, t.sessionManager, t.ctx)
	t.program = tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	// Set up agent created callback
	if t.manager != nil {
		t.manager.SetAgentCreatedCallback(func(a *agent.Agent) {
			t.listenToAgent(a)
		})
	}

	_, err := t.program.Run()
	t.cancel()
	return err
}

func (t *TUI) listenToAgent(a *agent.Agent) {
	if a.Session == nil {
		return
	}

	sessionID := a.Session.ID

	t.listenerMu.Lock()
	// Cancel existing listener if any
	if cancel, ok := t.outputListeners[sessionID]; ok {
		cancel()
	}

	ctx, cancel := context.WithCancel(t.ctx)
	t.outputListeners[sessionID] = cancel
	t.listenerMu.Unlock()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-a.Output:
				if !ok {
					return
				}
				if t.program != nil {
					if apiMsg, ok := msg.(*api.Message); ok {
						t.program.Send(agentOutputMsg(apiMsg))
					}
				}
			}
		}
	}()
}

func (t *TUI) ClearScreen() {
	fmt.Print("\033[H\033[2J")
}

// model is the Bubble Tea model
type model struct {
	// Core dependencies
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context

	// UI state
	view           viewState
	width          int
	height         int
	ready          bool
	showHelp       bool

	// Session list
	sessions          []*api.Session
	selectedSession   int
	activeSessionID   string

	// Chat viewport
	viewport    viewport.Model
	messages    []*api.Message

	// Input
	textarea    textarea.Model
	inputFocused bool

	// Options (for user choice requests)
	options         []string
	selectedOption  int
	showOptions     bool

	// Spinner for loading states
	spinner     spinner.Model

	// Cached markdown renderer
	mdRenderer  *glamour.TermRenderer
	mdRendererWidth int

	// Username
	username string
}

func newModel(manager *agent.AgentManager, sessionManager *sessions.SessionManager, ctx context.Context) model {
	// Create textarea
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send)"
	ta.Focus()
	ta.CharLimit = 4096
	ta.ShowLineNumbers = false
	ta.Prompt = "│ "
	ta.SetHeight(3)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	ta.BlurredStyle.Base = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(false)

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	// Create viewport
	vp := viewport.New(80, 20)

	// Get username
	username := "You"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	return model{
		manager:        manager,
		sessionManager: sessionManager,
		ctx:            ctx,
		view:           viewChat,
		textarea:       ta,
		viewport:       vp,
		spinner:        s,
		username:       username,
		inputFocused:   true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.loadSessions,
		m.tick(),
	)
}

func (m model) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) loadSessions() tea.Msg {
	if m.sessionManager == nil {
		return sessionListMsg(nil)
	}
	sessions, err := m.sessionManager.ListSessions()
	if err != nil {
		klog.Errorf("Failed to load sessions: %v", err)
		return sessionListMsg(nil)
	}
	return sessionListMsg(sessions)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m = m.updateLayout()

	case tea.KeyMsg:
		cmd := m.handleKeyPress(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case sessionListMsg:
		m.sessions = msg
		if len(m.sessions) > 0 && m.activeSessionID == "" {
			// Auto-select first session
			m.activeSessionID = m.sessions[0].ID
			cmds = append(cmds, m.switchToSession(m.activeSessionID))
		}

	case agentOutputMsg:
		m = m.handleAgentOutput(msg)

	case sessionSwitchedMsg:
		m.activeSessionID = string(msg)
		m = m.loadMessagesForSession()
		m = m.updateViewport()

	case sessionCreatedMsg:
		m.sessions = append([]*api.Session{msg}, m.sessions...)
		m.activeSessionID = msg.ID
		m.selectedSession = 0
		m.messages = nil
		m = m.updateViewport()

	case sessionDeletedMsg:
		m = m.removeSession(string(msg))

	case tickMsg:
		cmds = append(cmds, m.tick())

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case errMsg:
		klog.Errorf("Error: %v", msg)
	}

	// Update components based on view
	if m.view == viewChat && m.inputFocused && !m.showOptions {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.view == viewChat && !m.showOptions {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	// Global keys
	switch {
	case key.Matches(msg, keys.Quit):
		return tea.Quit

	case key.Matches(msg, keys.Help):
		m.showHelp = !m.showHelp
		return nil

	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.ShiftTab):
		return m.toggleView()

	case key.Matches(msg, keys.NewSession):
		return m.createNewSession()
	}

	// View-specific keys
	switch m.view {
	case viewSidebar:
		return m.handleSidebarKey(msg)
	case viewChat:
		if m.showOptions {
			return m.handleOptionsKey(msg)
		}
		return m.handleChatKey(msg)
	}

	return nil
}

func (m *model) toggleView() tea.Cmd {
	if m.view == viewSidebar {
		m.view = viewChat
		m.textarea.Focus()
		m.inputFocused = true
	} else {
		m.view = viewSidebar
		m.textarea.Blur()
		m.inputFocused = false
	}
	return nil
}

func (m model) handleSidebarKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Up):
		if m.selectedSession > 0 {
			m.selectedSession--
		}
	case key.Matches(msg, keys.Down):
		if m.selectedSession < len(m.sessions)-1 {
			m.selectedSession++
		}
	case key.Matches(msg, keys.Enter), key.Matches(msg, keys.Right):
		if len(m.sessions) > 0 {
			session := m.sessions[m.selectedSession]
			if session.ID != m.activeSessionID {
				return m.switchToSession(session.ID)
			}
			m.view = viewChat
			m.textarea.Focus()
			m.inputFocused = true
		}
	case key.Matches(msg, keys.DeleteSess):
		if len(m.sessions) > 0 {
			return m.deleteSession(m.sessions[m.selectedSession].ID)
		}
	}
	return nil
}

func (m model) handleChatKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Enter):
		if m.textarea.Value() != "" {
			return m.sendMessage()
		}
	case key.Matches(msg, keys.ScrollUp):
		m.viewport.LineUp(5)
	case key.Matches(msg, keys.ScrollDown):
		m.viewport.LineDown(5)
	case key.Matches(msg, keys.PageUp):
		m.viewport.HalfViewUp()
	case key.Matches(msg, keys.PageDown):
		m.viewport.HalfViewDown()
	case key.Matches(msg, keys.Left):
		m.view = viewSidebar
		m.textarea.Blur()
		m.inputFocused = false
	}
	return nil
}

func (m model) handleOptionsKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, keys.Up):
		if m.selectedOption > 0 {
			m.selectedOption--
		}
	case key.Matches(msg, keys.Down):
		if m.selectedOption < len(m.options)-1 {
			m.selectedOption++
		}
	case key.Matches(msg, keys.Enter):
		return m.selectOption()
	}
	return nil
}

func (m model) sendMessage() tea.Cmd {
	if m.manager == nil || m.activeSessionID == "" {
		return nil
	}

	query := m.textarea.Value()
	m.textarea.Reset()

	return func() tea.Msg {
		agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID)
		if err != nil {
			return errMsg(err)
		}
		agent.Input <- &api.UserInputResponse{Query: query}
		return nil
	}
}

func (m model) selectOption() tea.Cmd {
	if m.manager == nil || m.activeSessionID == "" {
		return nil
	}

	choice := m.selectedOption + 1
	m.showOptions = false
	m.selectedOption = 0

	return func() tea.Msg {
		agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID)
		if err != nil {
			return errMsg(err)
		}
		agent.Input <- &api.UserChoiceResponse{Choice: choice}
		return nil
	}
}

func (m model) switchToSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if m.manager != nil {
			_, err := m.manager.GetAgent(m.ctx, sessionID)
			if err != nil {
				return errMsg(err)
			}
		}
		return sessionSwitchedMsg(sessionID)
	}
}

func (m model) createNewSession() tea.Cmd {
	return func() tea.Msg {
		if m.sessionManager == nil {
			return nil
		}
		session, err := m.sessionManager.NewSession(sessions.Metadata{})
		if err != nil {
			return errMsg(err)
		}
		// Start agent for new session
		if m.manager != nil {
			if _, err := m.manager.GetAgent(m.ctx, session.ID); err != nil {
				klog.Errorf("Failed to start agent for new session: %v", err)
			}
		}
		return sessionCreatedMsg(session)
	}
}

func (m model) deleteSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if m.manager == nil {
			return nil
		}
		if err := m.manager.DeleteSession(sessionID); err != nil {
			return errMsg(err)
		}
		return sessionDeletedMsg(sessionID)
	}
}

func (m model) removeSession(sessionID string) model {
	for i, s := range m.sessions {
		if s.ID == sessionID {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			break
		}
	}
	if m.selectedSession >= len(m.sessions) {
		m.selectedSession = len(m.sessions) - 1
	}
	if m.selectedSession < 0 {
		m.selectedSession = 0
	}
	// If deleted active session, switch to another
	if sessionID == m.activeSessionID && len(m.sessions) > 0 {
		m.activeSessionID = m.sessions[m.selectedSession].ID
	}
	return m
}

func (m model) handleAgentOutput(msg *api.Message) model {
	if msg == nil {
		return m
	}

	// Check if this is for active session
	if m.manager != nil && m.activeSessionID != "" {
		agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID)
		if err == nil && agent.Session != nil {
			m.messages = agent.Session.AllMessages()
		}
	}

	// Check for choice request
	if msg.Type == api.MessageTypeUserChoiceRequest {
		if req, ok := msg.Payload.(*api.UserChoiceRequest); ok {
			m.options = make([]string, len(req.Options))
			for i, opt := range req.Options {
				m.options[i] = opt.Label
			}
			m.showOptions = true
			m.selectedOption = 0
		}
	}

	m = m.updateViewport()
	return m
}

func (m model) loadMessagesForSession() model {
	if m.manager == nil || m.activeSessionID == "" {
		return m
	}

	agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID)
	if err != nil {
		return m
	}

	if agent.Session != nil {
		m.messages = agent.Session.AllMessages()
	}

	return m
}

func (m model) updateLayout() model {
	// Calculate dimensions
	chatWidth := m.width - sidebarWidth - 4 // borders and margins
	if chatWidth < 40 {
		chatWidth = 40
	}

	inputHeight := 5
	statusHeight := 1
	chatHeight := m.height - inputHeight - statusHeight - 4 // borders

	// Update viewport
	m.viewport.Width = chatWidth - 4
	m.viewport.Height = chatHeight

	// Update textarea
	m.textarea.SetWidth(chatWidth - 4)

	// Invalidate cached renderer if width changed
	if m.mdRendererWidth != m.viewport.Width {
		m.mdRenderer = nil
		m.mdRendererWidth = m.viewport.Width
	}

	return m.updateViewport()
}

func (m model) updateViewport() model {
	content := m.renderMessages()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
	return m
}

func (m model) getMarkdownRenderer() *glamour.TermRenderer {
	if m.mdRenderer != nil {
		return m.mdRenderer
	}

	width := m.viewport.Width - 4
	if width < 40 {
		width = 40
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return nil
	}

	m.mdRenderer = renderer
	return renderer
}

func (m model) renderMessages() string {
	var sb strings.Builder

	for _, msg := range m.messages {
		// Skip internal messages
		if msg.Type == api.MessageTypeUserInputRequest && msg.Payload == ">>>" {
			continue
		}

		rendered := m.renderMessage(msg)
		if rendered != "" {
			sb.WriteString(rendered)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (m model) renderMessage(msg *api.Message) string {
	var prefix string
	var content string

	switch msg.Source {
	case api.MessageSourceUser:
		prefix = userMessageStyle.Render(m.username + ": ")
	case api.MessageSourceModel, api.MessageSourceAgent:
		prefix = aiMessageStyle.Render("AI: ")
	default:
		prefix = systemMessageStyle.Render("System: ")
	}

	switch p := msg.Payload.(type) {
	case string:
		content = p
	case *api.UserChoiceRequest:
		content = p.Prompt
	default:
		return ""
	}

	switch msg.Type {
	case api.MessageTypeToolCallRequest:
		content = fmt.Sprintf("🔧 Running: `%s`", content)
	case api.MessageTypeToolCallResponse:
		return "" // Skip tool responses in main view
	case api.MessageTypeError:
		content = fmt.Sprintf("❌ Error: %s", content)
	}

	// Render markdown
	renderer := m.getMarkdownRenderer()
	if renderer != nil {
		rendered, err := renderer.Render(content)
		if err == nil {
			content = strings.TrimSpace(rendered)
		}
	}

	return prefix + content
}

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Build sidebar
	sidebar := m.renderSidebar()

	// Build chat panel
	chat := m.renderChatPanel()

	// Combine horizontally
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, "  ", chat)

	// Add status bar
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, main, status)
}

func (m model) renderSidebar() string {
	var sb strings.Builder

	// Title
	sb.WriteString(sidebarTitleStyle.Render("📋 Sessions"))
	sb.WriteString("\n")

	// Session list
	for i, session := range m.sessions {
		name := session.Name
		if len(name) > sidebarWidth-6 {
			name = name[:sidebarWidth-9] + "..."
		}

		var style lipgloss.Style
		if i == m.selectedSession && m.view == viewSidebar {
			style = sessionItemSelectedStyle
		} else if session.ID == m.activeSessionID {
			style = sessionItemActiveStyle
		} else {
			style = sessionItemStyle
		}

		prefix := "  "
		if session.ID == m.activeSessionID {
			prefix = "● "
		}

		sb.WriteString(style.Render(prefix + name))
		sb.WriteString("\n")
	}

	// Help text
	if len(m.sessions) == 0 {
		sb.WriteString(helpStyle.Render("\n  No sessions yet\n  Press Ctrl+N to create"))
	}

	// Apply sidebar style
	style := sidebarStyle
	if m.view == viewSidebar {
		style = sidebarActiveStyle
	}

	return style.Height(m.height - 4).Render(sb.String())
}

func (m model) renderChatPanel() string {
	chatWidth := m.width - sidebarWidth - 6
	if chatWidth < 40 {
		chatWidth = 40
	}

	var sb strings.Builder

	// Chat header
	sessionName := "No session selected"
	if m.activeSessionID != "" {
		for _, s := range m.sessions {
			if s.ID == m.activeSessionID {
				sessionName = s.Name
				break
			}
		}
	}
	header := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Render("💬 " + sessionName)
	sb.WriteString(header)
	sb.WriteString("\n\n")

	// Viewport (messages)
	sb.WriteString(m.viewport.View())
	sb.WriteString("\n\n")

	// Options or input
	if m.showOptions {
		sb.WriteString(m.renderOptions())
	} else {
		// Show agent state indicator
		if m.activeSessionID != "" && m.manager != nil {
			if agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID); err == nil && agent.Session != nil {
				if agent.Session.AgentState == api.AgentStateRunning {
					sb.WriteString(m.spinner.View() + " Thinking...\n")
				}
			}
		}

		inputStyle := inputStyle
		if m.view == viewChat && m.inputFocused {
			inputStyle = inputActiveStyle
		}
		sb.WriteString(inputStyle.Width(chatWidth - 4).Render(m.textarea.View()))
	}

	// Apply chat panel style
	style := chatPanelStyle
	if m.view == viewChat {
		style = chatPanelActiveStyle
	}

	return style.Width(chatWidth).Height(m.height - 4).Render(sb.String())
}

func (m model) renderOptions() string {
	var sb strings.Builder
	sb.WriteString(lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("Choose an option:"))
	sb.WriteString("\n\n")

	for i, opt := range m.options {
		style := optionStyle
		if i == m.selectedOption {
			style = optionSelectedStyle
		}
		sb.WriteString(style.Render(fmt.Sprintf("%d. %s", i+1, opt)))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m model) renderStatusBar() string {
	// Left side: help hints
	left := helpStyle.Render("Tab: switch panel • Ctrl+N: new session • ?: help • Ctrl+C: quit")

	// Right side: session count
	right := helpStyle.Render(fmt.Sprintf("%d sessions", len(m.sessions)))

	// Calculate spacing
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 0 {
		gap = 0
	}

	return statusBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)
}
