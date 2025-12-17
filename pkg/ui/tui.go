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
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/klog/v2"
)

// Minimal color palette - works well on both light and dark terminals
var (
	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7C3AED"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}
	muted     = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#626262"}

	// Tab styles
	activeTab = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight).
			Padding(0, 2)

	inactiveTab = lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 2)

	newTabStyle = lipgloss.NewStyle().
			Foreground(special).
			Padding(0, 1)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle)

	// Message styles
	userStyle = lipgloss.NewStyle().
			Foreground(highlight).
			Bold(true)

	aiStyle = lipgloss.NewStyle().
		Foreground(special).
		Bold(true)

	toolStyle = lipgloss.NewStyle().
			Foreground(muted).
			Italic(true)

	// Input style
	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(subtle)

	// Status bar
	statusStyle = lipgloss.NewStyle().
			Foreground(muted).
			Padding(0, 1)

	// Option styles for choices
	optionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#333", Dark: "#EEE"}).
			Padding(0, 2)

	selectedOptionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(highlight).
				Padding(0, 2)
)

// Message types
type (
	sessionListMsg     []*api.Session
	agentOutputMsg     *api.Message
	sessionSwitchedMsg string
	sessionCreatedMsg  *api.Session
	sessionDeletedMsg  string
	tickMsg            time.Time
	errMsg             error
)

// TUI is the terminal user interface
type TUI struct {
	program        *tea.Program
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context
	cancel         context.CancelFunc

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

	m := newModel(t.manager, t.sessionManager, t.ctx)
	t.program = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

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
	manager        *agent.AgentManager
	sessionManager *sessions.SessionManager
	ctx            context.Context

	// Dimensions
	width  int
	height int
	ready  bool

	// Sessions
	sessions        []*api.Session
	activeSessionID string
	selectedTab     int

	// Chat
	viewport viewport.Model
	messages []*api.Message

	// Input
	textarea textarea.Model

	// Options (for choices)
	options        []string
	selectedOption int
	showOptions    bool

	// State
	spinner    spinner.Model
	mdRenderer *glamour.TermRenderer
	username   string
}

func newModel(manager *agent.AgentManager, sessionManager *sessions.SessionManager, ctx context.Context) model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.CharLimit = 4096
	ta.ShowLineNumbers = false
	ta.Prompt = "› "
	ta.SetHeight(2)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.KeyMap.InsertNewline.SetEnabled(false)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(highlight)

	vp := viewport.New(80, 20)

	username := "You"
	if u, err := user.Current(); err == nil && u.Username != "" {
		username = u.Username
	}

	return model{
		manager:        manager,
		sessionManager: sessionManager,
		ctx:            ctx,
		textarea:       ta,
		viewport:       vp,
		spinner:        s,
		username:       username,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.loadSessions,
	)
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
		if m.showOptions {
			return m.handleOptionsKey(msg)
		}
		// Handle special keys, but let regular typing pass through
		if cmd := m.handleKey(msg); cmd != nil {
			return m, cmd
		}

	case sessionListMsg:
		m.sessions = msg
		if len(m.sessions) > 0 && m.activeSessionID == "" {
			m.activeSessionID = m.sessions[0].ID
			cmds = append(cmds, m.switchSession(m.activeSessionID))
		}

	case agentOutputMsg:
		m = m.handleAgentOutput(msg)

	case sessionSwitchedMsg:
		m.activeSessionID = string(msg)
		m = m.loadMessages()
		m = m.refreshViewport()

	case sessionCreatedMsg:
		m.sessions = append([]*api.Session{msg}, m.sessions...)
		m.activeSessionID = msg.ID
		m.selectedTab = 0
		m.messages = nil
		m = m.refreshViewport()

	case sessionDeletedMsg:
		m = m.removeSession(string(msg))

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case errMsg:
		klog.Errorf("Error: %v", msg)
	}

	// Update textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "ctrl+q":
		return tea.Quit

	case "ctrl+n":
		return m.createSession()

	case "ctrl+w":
		if len(m.sessions) > 1 {
			return m.deleteCurrentSession()
		}

	case "ctrl+tab":
		// Switch tabs
		if len(m.sessions) > 0 {
			m.selectedTab = (m.selectedTab + 1) % len(m.sessions)
			if m.sessions[m.selectedTab].ID != m.activeSessionID {
				return m.switchSession(m.sessions[m.selectedTab].ID)
			}
		}

	case "ctrl+shift+tab":
		if len(m.sessions) > 0 {
			m.selectedTab--
			if m.selectedTab < 0 {
				m.selectedTab = len(m.sessions) - 1
			}
			if m.sessions[m.selectedTab].ID != m.activeSessionID {
				return m.switchSession(m.sessions[m.selectedTab].ID)
			}
		}

	case "enter":
		if m.textarea.Value() != "" {
			return m.sendMessage()
		}

	case "ctrl+l":
		m.viewport.GotoBottom()

	case "pgup":
		m.viewport.HalfViewUp()

	case "pgdown":
		m.viewport.HalfViewDown()
	}

	// Return nil to let the key pass through to textarea
	return nil
}

func (m model) handleOptionsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "up", "k":
		if m.selectedOption > 0 {
			m.selectedOption--
		}

	case "down", "j":
		if m.selectedOption < len(m.options)-1 {
			m.selectedOption++
		}

	case "enter":
		return m, m.selectOption()
	}

	return m, nil
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

func (m model) switchSession(sessionID string) tea.Cmd {
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

func (m model) createSession() tea.Cmd {
	return func() tea.Msg {
		if m.sessionManager == nil {
			return nil
		}
		session, err := m.sessionManager.NewSession(sessions.Metadata{})
		if err != nil {
			return errMsg(err)
		}
		if m.manager != nil {
			if _, err := m.manager.GetAgent(m.ctx, session.ID); err != nil {
				klog.Errorf("Failed to start agent: %v", err)
			}
		}
		return sessionCreatedMsg(session)
	}
}

func (m model) deleteCurrentSession() tea.Cmd {
	if len(m.sessions) == 0 {
		return nil
	}
	sessionID := m.sessions[m.selectedTab].ID
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
	if m.selectedTab >= len(m.sessions) {
		m.selectedTab = len(m.sessions) - 1
	}
	if m.selectedTab < 0 {
		m.selectedTab = 0
	}
	if sessionID == m.activeSessionID && len(m.sessions) > 0 {
		m.activeSessionID = m.sessions[m.selectedTab].ID
	}
	return m
}

func (m model) handleAgentOutput(msg *api.Message) model {
	if msg == nil {
		return m
	}

	if m.manager != nil && m.activeSessionID != "" {
		agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID)
		if err == nil && agent.Session != nil {
			m.messages = agent.Session.AllMessages()
		}
	}

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

	m = m.refreshViewport()
	return m
}

func (m model) loadMessages() model {
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
	// Tab bar: 1 line + border
	// Status bar: 1 line
	// Input: 3 lines + border
	// Remaining: viewport

	tabHeight := 2
	statusHeight := 1
	inputHeight := 4
	viewportHeight := m.height - tabHeight - statusHeight - inputHeight

	if viewportHeight < 5 {
		viewportHeight = 5
	}

	m.viewport.Width = m.width
	m.viewport.Height = viewportHeight
	m.textarea.SetWidth(m.width - 4)

	// Reset renderer on resize
	m.mdRenderer = nil

	return m.refreshViewport()
}

func (m model) refreshViewport() model {
	content := m.renderMessages()
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
	return m
}

func (m model) getRenderer() *glamour.TermRenderer {
	if m.mdRenderer != nil {
		return m.mdRenderer
	}

	width := m.width - 8
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
		if msg.Type == api.MessageTypeUserInputRequest && msg.Payload == ">>>" {
			continue
		}

		line := m.renderMessage(msg)
		if line != "" {
			sb.WriteString(line)
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
		prefix = userStyle.Render(m.username) + " "
	case api.MessageSourceModel, api.MessageSourceAgent:
		prefix = aiStyle.Render("AI") + " "
	default:
		prefix = ""
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
		return toolStyle.Render("⚡ " + content)
	case api.MessageTypeToolCallResponse:
		return ""
	case api.MessageTypeError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render("✗ " + content)
	}

	// Render markdown
	if renderer := m.getRenderer(); renderer != nil {
		if rendered, err := renderer.Render(content); err == nil {
			content = strings.TrimSpace(rendered)
		}
	}

	return prefix + content
}

func (m model) View() string {
	if !m.ready {
		return ""
	}

	var sections []string

	// Tab bar
	sections = append(sections, m.renderTabs())

	// Chat viewport
	sections = append(sections, m.viewport.View())

	// Options or input
	if m.showOptions {
		sections = append(sections, m.renderOptions())
	} else {
		sections = append(sections, m.renderInput())
	}

	// Status bar
	sections = append(sections, m.renderStatus())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m model) renderTabs() string {
	var tabs []string

	for i, session := range m.sessions {
		name := session.Name
		if len(name) > 15 {
			name = name[:12] + "..."
		}

		if i == m.selectedTab {
			tabs = append(tabs, activeTab.Render("● "+name))
		} else {
			tabs = append(tabs, inactiveTab.Render("○ "+name))
		}
	}

	tabs = append(tabs, newTabStyle.Render("[+]"))

	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return tabBarStyle.Width(m.width).Render(row)
}

func (m model) renderInput() string {
	// Show spinner if agent is running
	prefix := ""
	if m.activeSessionID != "" && m.manager != nil {
		if agent, err := m.manager.GetAgent(m.ctx, m.activeSessionID); err == nil && agent.Session != nil {
			if agent.Session.AgentState == api.AgentStateRunning {
				prefix = m.spinner.View() + " "
			}
		}
	}

	input := m.textarea.View()
	if prefix != "" {
		input = prefix + input
	}

	return inputStyle.Width(m.width).Render(input)
}

func (m model) renderOptions() string {
	var sb strings.Builder
	sb.WriteString("\n")

	for i, opt := range m.options {
		style := optionStyle
		marker := "  "
		if i == m.selectedOption {
			style = selectedOptionStyle
			marker = "› "
		}
		sb.WriteString(style.Render(marker + opt))
		sb.WriteString("\n")
	}

	return inputStyle.Width(m.width).Render(sb.String())
}

func (m model) renderStatus() string {
	left := "Ctrl+Tab: switch • Ctrl+N: new • Ctrl+W: close"
	right := fmt.Sprintf("%d sessions", len(m.sessions))

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	return statusStyle.Render(left + strings.Repeat(" ", gap) + right)
}
