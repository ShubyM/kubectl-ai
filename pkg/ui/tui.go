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
	"io"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/agent"
	"github.com/GoogleCloudPlatform/kubectl-ai/pkg/api"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/klog/v2"
)

// ASCII Art Logo

const logo = `
 _          _               _   _             _ 
| | ___   _| |__   ___  ___| |_| |       __ _(_)
| |/ / | | | '_ \ / _ \/ __| __| |_____ / _  | |
|   <| |_| | |_) |  __/ (__| |_| |_____| (_| | |
|_|\_\\__,_|_.__/ \___|\___|\__|_|      \__,_|_|
                                                
`

// Color palette - Google Material Design colors
var (
	colorPrimary   = lipgloss.Color("#8AB4F8") // Blue 200
	colorSecondary = lipgloss.Color("#81C995") // Green 200
	colorSuccess   = lipgloss.Color("#81C995") // Green 200
	colorError     = lipgloss.Color("#F28B82") // Red 200
	colorWarning   = lipgloss.Color("#FDD663") // Yellow 200
	colorText      = lipgloss.Color("#E8EAED") // Grey 200
	colorTextMuted = lipgloss.Color("#9AA0A6") // Grey 500
	colorTextDim   = lipgloss.Color("#5F6368") // Grey 700
	colorBgSubtle  = lipgloss.Color("#303134") // Surface variant
	colorBgCode    = lipgloss.Color("#1E1E1E") // Code background
)

// Pre-built styles (created once, reused)
var (
	statusBarStyle    = lipgloss.NewStyle().Background(colorBgSubtle).Foreground(colorText)
	statusItemStyle   = lipgloss.NewStyle().Foreground(colorTextMuted)
	statusActiveStyle = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)

	userMessageStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(colorPrimary).
				PaddingLeft(1).
				MarginBottom(1)

	agentMessageStyle = lipgloss.NewStyle().
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(colorSecondary).
				PaddingLeft(1).
				MarginBottom(1)

	userLabelStyle  = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	agentLabelStyle = lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)
	timestampStyle  = lipgloss.NewStyle().Foreground(colorTextDim).Italic(true)

	toolBoxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorSecondary).Padding(0, 1).MarginBottom(1)
	toolIconStyle    = lipgloss.NewStyle().Foreground(colorSecondary).Bold(true)
	toolCommandStyle = lipgloss.NewStyle().Foreground(colorText).Background(colorBgCode).Padding(0, 1)

	errorBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorError).Padding(0, 1).MarginBottom(1)
	errorStyle    = lipgloss.NewStyle().Foreground(colorError).Bold(true)

	inputBoxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPrimary).Padding(0, 1)
	inputBoxDimStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorTextDim).Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().Foreground(colorPrimary)
	helpStyle    = lipgloss.NewStyle().Foreground(colorTextDim)

	statusSepStyle       = lipgloss.NewStyle().Foreground(colorTextDim)
	statusAppStyle       = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	statusSessionStyle   = lipgloss.NewStyle().Foreground(colorTextMuted)
	statusModelStyle     = lipgloss.NewStyle().Foreground(colorSecondary)
	statusContextError   = lipgloss.NewStyle().Foreground(colorError)
	statusContextWarning = lipgloss.NewStyle().Foreground(colorWarning)
	dividerStyle         = lipgloss.NewStyle().Foreground(colorTextDim)
	logoStyle            = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)

	listTitleStyle    = lipgloss.NewStyle().Foreground(colorWarning).Bold(true).Padding(0, 0, 1, 0)
	listItemStyle     = lipgloss.NewStyle().Foreground(colorTextMuted).PaddingLeft(2)
	listSelectedStyle = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).PaddingLeft(0)
)

// item represents a choice in the list
type item string

func (i item) FilterValue() string { return "" }

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}
	str := string(i)
	if index == m.Index() {
		fmt.Fprint(w, listSelectedStyle.Render("> "+str))
	} else {
		fmt.Fprint(w, listItemStyle.Render("  "+str))
	}
}

var cachedUsername string
var usernameOnce sync.Once

func getCurrentUsername() string {
	usernameOnce.Do(func() {
		if u, err := user.Current(); err == nil {
			cachedUsername = u.Username
		} else if username := os.Getenv("USER"); username != "" {
			cachedUsername = username
		} else {
			cachedUsername = "You"
		}
	})
	return cachedUsername
}

// TUI is a rich terminal user interface for the agent.
type TUI struct {
	program *tea.Program
	agent   *agent.Agent
}

func NewTUI(agent *agent.Agent) *TUI {
	return &TUI{
		program: tea.NewProgram(newModel(agent), tea.WithAltScreen(), tea.WithMouseAllMotion()),
		agent:   agent,
	}
}

func (u *TUI) Run(ctx context.Context) error {
	// Suppress stderr to prevent klog from breaking TUI
	originalStderr := os.Stderr
	if devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0); err == nil {
		os.Stderr = devNull
		defer func() {
			os.Stderr = originalStderr
			devNull.Close()
		}()
	}
	klog.SetOutput(io.Discard)
	klog.LogToStderr(false)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-u.agent.Output:
				if !ok {
					return
				}
				u.program.Send(msg)
			}
		}
	}()

	_, err := u.program.Run()
	return err
}

func (u *TUI) ClearScreen() {}

type tickMsg time.Time

// renderCache holds cached rendered messages
type renderCache struct {
	mu       sync.RWMutex
	cache    map[string]string
	width    int
	renderer *glamour.TermRenderer
}

func newRenderCache() *renderCache {
	return &renderCache{cache: make(map[string]string)}
}

func (rc *renderCache) get(id string) (string, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	content, ok := rc.cache[id]
	return content, ok
}

func (rc *renderCache) set(id string, content string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cache[id] = content
}

func (rc *renderCache) getRenderer(width int) (*glamour.TermRenderer, error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.width != width {
		rc.cache = make(map[string]string)
		rc.width = width
		rc.renderer = nil
	}

	if rc.renderer == nil {
		var err error
		rc.renderer, err = glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(width),
		)
		if err != nil {
			return nil, err
		}
	}
	return rc.renderer, nil
}

type model struct {
	viewport    viewport.Model
	textinput   textinput.Model
	spinner     spinner.Model
	list        list.Model
	agent       *agent.Agent
	messages    []*api.Message
	renderCache *renderCache

	// Cached values to avoid repeated calls
	cachedState   api.AgentState
	cachedSession *api.Session

	quitting      bool
	width, height int
	thinkingStart time.Time
	startTime     time.Time

	// Pre-rendered content cache
	viewportContent   string
	viewportDirty     bool
	lastRenderedWidth int

	// Session picker state
	showSessionPicker  bool
	sessionPickerItems []api.SessionInfo
	sessionPickerIndex int
}

func newModel(agent *agent.Agent) model {
	ti := textinput.New()
	ti.Placeholder = "Ask kubectl-ai anything..."
	ti.Focus()
	ti.Prompt = ""
	ti.CharLimit = 4096
	ti.Width = 80
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorText)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorTextDim)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = spinnerStyle

	l := list.New([]list.Item{}, itemDelegate{}, 40, 5)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowTitle(false)

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true

	return model{
		agent:         agent,
		textinput:     ti,
		viewport:      vp,
		spinner:       sp,
		list:          l,
		renderCache:   newRenderCache(),
		cachedState:   api.AgentStateIdle,
		startTime:     time.Now(),
		viewportDirty: true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick, m.tickCmd())
}

func (m model) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewportDirty = true
		return m.handleResize(), nil

	case tea.KeyMsg:
		return m.handleKeypress(msg)

	case tea.MouseMsg:
		// Only handle wheel events
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.ScrollUp(3)
			return m, nil
		case tea.MouseButtonWheelDown:
			m.viewport.ScrollDown(3)
			return m, nil
		}
		return m, nil

	case *api.Message:
		return m.handleMessage(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tickMsg:
		// Only tick if we're in a state that needs time updates
		if m.cachedState == api.AgentStateRunning || m.cachedState == api.AgentStateInitializing {
			return m, m.tickCmd()
		}
		return m, m.tickCmd()
	}

	return m, nil
}

func (m model) isWaitingForChoice() bool {
	if m.cachedState != api.AgentStateWaitingForInput || len(m.messages) == 0 {
		return false
	}
	lastMsg := m.messages[len(m.messages)-1]
	return lastMsg.Type == api.MessageTypeUserChoiceRequest
}

func (m model) handleResize() model {
	fixedHeight := 7 // status + 2 dividers + input(3) + help
	contentHeight := m.height - fixedHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	m.viewport.Width = m.width - 2
	m.viewport.Height = contentHeight
	m.textinput.Width = m.width - 6
	m.list.SetWidth(m.width - 4)

	if m.viewportDirty || m.lastRenderedWidth != m.width {
		m.refreshViewport()
		m.lastRenderedWidth = m.width
	}
	m.viewport.GotoBottom()

	return m
}

func (m model) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle session picker navigation
	if m.showSessionPicker {
		return m.handleSessionPickerKeypress(msg)
	}

	// Handle choice selection if active
	if m.isWaitingForChoice() {
		// Allow quitting even during choice
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleEnter()
		}

		// Forward other keys to list (up/down/j/k navigation)
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlD:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEsc:
		m.textinput.Reset()
		return m, nil

	case tea.KeyEnter:
		return m.handleEnter()

	case tea.KeyUp:
		m.viewport.ScrollUp(1)
		return m, nil
	case tea.KeyDown:
		m.viewport.ScrollDown(1)
		return m, nil
	case tea.KeyPgUp:
		m.viewport.ScrollUp(m.viewport.Height / 2)
		return m, nil
	case tea.KeyPgDown:
		m.viewport.ScrollDown(m.viewport.Height / 2)
		return m, nil
	}

	switch msg.String() {
	case "ctrl+u":
		m.viewport.ScrollUp(m.viewport.Height / 2)
		return m, nil
	case "ctrl+d":
		m.viewport.ScrollDown(m.viewport.Height / 2)
		return m, nil
	}

	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)
	return m, cmd
}

func (m model) handleSessionPickerKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyCtrlD:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyEsc:
		// Cancel picker
		m.showSessionPicker = false
		m.sessionPickerItems = nil
		return m, func() tea.Msg {
			m.agent.Input <- &api.SessionPickerResponse{Cancelled: true}
			return nil
		}

	case tea.KeyEnter:
		// Select session
		if len(m.sessionPickerItems) > 0 {
			selected := m.sessionPickerItems[m.sessionPickerIndex]
			m.showSessionPicker = false
			m.sessionPickerItems = nil
			return m, func() tea.Msg {
				m.agent.Input <- &api.SessionPickerResponse{SessionID: selected.ID}
				return nil
			}
		}
		return m, nil

	case tea.KeyUp:
		if m.sessionPickerIndex > 0 {
			m.sessionPickerIndex--
		}
		return m, nil

	case tea.KeyDown:
		if m.sessionPickerIndex < len(m.sessionPickerItems)-1 {
			m.sessionPickerIndex++
		}
		return m, nil
	}

	// j/k vim-style navigation
	switch msg.String() {
	case "j":
		if m.sessionPickerIndex < len(m.sessionPickerItems)-1 {
			m.sessionPickerIndex++
		}
		return m, nil
	case "k":
		if m.sessionPickerIndex > 0 {
			m.sessionPickerIndex--
		}
		return m, nil
	}

	return m, nil
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	session := m.agent.GetSession()
	m.cachedState = session.AgentState
	m.cachedSession = session

	if m.cachedState == api.AgentStateWaitingForInput && len(m.messages) > 0 {
		if lastMsg := m.messages[len(m.messages)-1]; lastMsg.Type == api.MessageTypeUserChoiceRequest {
			if _, ok := m.list.SelectedItem().(item); ok {
				choice := m.list.Index() + 1
				return m, func() tea.Msg {
					m.agent.Input <- &api.UserChoiceResponse{Choice: choice}
					return nil
				}
			}
			return m, nil
		}
	}

	value := strings.TrimSpace(m.textinput.Value())
	if value == "" {
		return m, nil
	}

	// Intercept "sessions" command for interactive picker
	if value == "sessions" {
		m.textinput.Reset()
		sessions, err := m.agent.ListSessions()
		if err != nil {
			klog.FromContext(context.Background()).Error(err, "failed to list sessions")
			return m, nil
		}
		if len(sessions) == 0 {
			return m, nil
		}
		m.showSessionPicker = true
		m.sessionPickerItems = sessions
		m.sessionPickerIndex = 0
		return m, nil
	}

	userMsg := &api.Message{
		Source:    api.MessageSourceUser,
		Type:      api.MessageTypeText,
		Payload:   value,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, userMsg)
	m.viewportDirty = true
	m.refreshViewport()
	m.viewport.GotoBottom()

	m.textinput.Reset()
	m.thinkingStart = time.Now()

	return m, func() tea.Msg {
		m.agent.Input <- &api.UserInputResponse{Query: value}
		return nil
	}
}

func (m *model) handleMessage(msg *api.Message) (tea.Model, tea.Cmd) {
	session := m.agent.GetSession()
	m.messages = session.AllMessages()
	m.cachedState = session.AgentState
	m.cachedSession = session
	m.viewportDirty = true

	m.refreshViewport()
	m.viewport.GotoBottom()

	if m.cachedState == api.AgentStateRunning || m.cachedState == api.AgentStateInitializing {
		return m, m.spinner.Tick
	}

	// Handle choice request list initialization
	if msg.Type == api.MessageTypeUserChoiceRequest {
		if choiceReq, ok := msg.Payload.(*api.UserChoiceRequest); ok {
			items := make([]list.Item, len(choiceReq.Options))
			for i, opt := range choiceReq.Options {
				items[i] = item(opt.Label)
			}
			m.list.SetItems(items)
			// Reset selection
			m.list.Select(0)
		}
	}

	return m, nil
}

func (m *model) refreshViewport() {
	if !m.viewportDirty && m.lastRenderedWidth == m.viewport.Width {
		return
	}
	m.viewportContent = m.renderAllMessages()
	m.viewport.SetContent(m.viewportContent)
	m.viewportDirty = false
	m.lastRenderedWidth = m.viewport.Width
}

func (m model) renderAllMessages() string {
	if len(m.messages) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left,
			"",
			logoStyle.Render(logo),
			"",
			lipgloss.NewStyle().PaddingLeft(1).Foreground(colorTextMuted).Render("Your AI-powered Kubernetes assistant"),
			lipgloss.NewStyle().PaddingLeft(1).Foreground(colorTextDim).Render("Type a message to get started"),
			"",
		)
	}

	var sb strings.Builder
	renderWidth := m.viewport.Width - 6
	if renderWidth > 90 {
		renderWidth = 90
	}
	if renderWidth < 40 {
		renderWidth = 40
	}

	renderer, err := m.renderCache.getRenderer(renderWidth)
	if err != nil {
		return "Error rendering messages"
	}

	for _, msg := range m.messages {
		if rendered := m.renderMessage(msg, renderer, renderWidth); rendered != "" {
			sb.WriteString(rendered)
		}
	}

	return sb.String()
}

func (m model) renderMessage(msg *api.Message, renderer *glamour.TermRenderer, width int) string {
	if msg.Type == api.MessageTypeUserInputRequest {
		if payload, ok := msg.Payload.(string); ok && payload == ">>>" {
			return ""
		}
	}
	if msg.Type == api.MessageTypeToolCallResponse {
		return ""
	}

	if msg.ID != "" && msg.Type != api.MessageTypeToolCallRequest {
		if cached, ok := m.renderCache.get(msg.ID); ok {
			return cached
		}
	}

	var result string
	switch msg.Type {
	case api.MessageTypeToolCallRequest:
		result = m.renderToolCall(msg, width)
	case api.MessageTypeError:
		result = m.renderError(msg, width)
	case api.MessageTypeUserChoiceRequest:
		result = m.renderChoiceRequest(msg)
	default:
		result = m.renderTextMessage(msg, renderer, width)
	}

	if msg.ID != "" && result != "" && msg.Type != api.MessageTypeToolCallRequest {
		m.renderCache.set(msg.ID, result)
	}

	return result
}

func (m model) renderTextMessage(msg *api.Message, renderer *glamour.TermRenderer, width int) string {
	payload, ok := msg.Payload.(string)
	if !ok {
		return ""
	}

	timestamp := ""
	if !msg.Timestamp.IsZero() {
		timestamp = timestampStyle.Render(msg.Timestamp.Format("15:04"))
	}

	switch msg.Source {
	case api.MessageSourceUser:
		label := userLabelStyle.Render("You")
		if timestamp != "" {
			label += " " + timestamp
		}
		content := lipgloss.NewStyle().Foreground(colorText).Width(width).Render(payload)
		return userMessageStyle.Width(width+2).Render(label+"\n"+content) + "\n"

	case api.MessageSourceModel, api.MessageSourceAgent:
		label := agentLabelStyle.Render("kubectl-ai")
		if timestamp != "" {
			label += " " + timestamp
		}
		rendered, err := renderer.Render(payload)
		if err != nil {
			rendered = payload
		}
		rendered = strings.TrimSpace(rendered)
		return agentMessageStyle.Width(width+2).Render(label+"\n"+rendered) + "\n"
	}

	return ""
}

func (m model) renderToolCall(msg *api.Message, width int) string {
	payload, ok := msg.Payload.(string)
	if !ok {
		return ""
	}
	icon := toolIconStyle.Render("⚡")
	status := lipgloss.NewStyle().Foreground(colorSecondary).Render("Running")
	command := toolCommandStyle.Render(payload)
	content := icon + " " + status + "\n" + command
	return toolBoxStyle.Width(width).Render(content) + "\n"
}

func (m model) renderError(msg *api.Message, width int) string {
	payload, ok := msg.Payload.(string)
	if !ok {
		return ""
	}
	icon := errorStyle.Render("✗")
	label := errorStyle.Render("Error")
	content := lipgloss.NewStyle().Foreground(colorError).Width(width - 4).Render(payload)
	return errorBoxStyle.Width(width).Render(icon+" "+label+"\n"+content) + "\n"
}

func (m model) renderChoiceRequest(msg *api.Message) string {
	choiceReq, ok := msg.Payload.(*api.UserChoiceRequest)
	if !ok {
		return ""
	}
	return lipgloss.NewStyle().Foreground(colorWarning).Bold(true).Render("? "+choiceReq.Prompt) + "\n\n"
}

func (m model) View() string {
	if m.quitting {
		return lipgloss.NewStyle().Foreground(colorTextMuted).Padding(1).Render("Goodbye!")
	}

	// Cache session for this render
	if m.cachedSession == nil {
		m.cachedSession = m.agent.GetSession()
		m.cachedState = m.cachedSession.AgentState
	}

	// Show session picker overlay if active
	if m.showSessionPicker {
		return lipgloss.JoinVertical(lipgloss.Left,
			m.renderStatusBar(),
			m.renderDivider(),
			m.renderSessionPicker(),
			m.renderDivider(),
			m.renderSessionPickerHelp(),
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.renderStatusBar(),
		m.renderDivider(),
		lipgloss.NewStyle().PaddingLeft(1).Render(m.viewport.View()),
		m.renderDivider(),
		m.renderInputArea(),
		m.renderHelp(),
	)
}

func (m model) renderStatusBar() string {
	session := m.cachedSession
	if session == nil {
		session = m.agent.GetSession()
	}

	left := m.renderStatusLeft(session)
	right := m.renderStatusRight(session)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 0 {
		gap = 0
	}

	content := " " + left + strings.Repeat(" ", gap) + right + " "
	return statusBarStyle.Width(m.width).Render(content)
}

func (m model) renderStatusLeft(session *api.Session) string {
	sep := statusSepStyle.Render(" | ")
	appName := statusAppStyle.Render("kubectl-ai")

	sessionName := session.Name
	if sessionName == "" {
		sessionName = session.ID
	}
	sessionNameStyled := statusSessionStyle.Render(sessionName)

	state := m.renderState(session.AgentState)

	return appName + sep + sessionNameStyled + sep + state
}

func (m model) renderStatusRight(session *api.Session) string {
	sep := statusSepStyle.Render(" | ")

	modelName := session.ModelID
	if modelName == "" {
		modelName = "unknown"
	}
	modelStyled := statusModelStyle.Render(modelName)

	contextStyled := m.renderContextUsage(session)

	return modelStyled + sep + contextStyled
}

func (m model) renderContextUsage(session *api.Session) string {
	var contextStr string
	if session.MaxTokens > 0 && session.TotalTokens > 0 {
		pct := float64(session.TotalTokens) / float64(session.MaxTokens) * 100
		contextStr = fmt.Sprintf("%.0f%% context", pct)
	} else if session.TotalTokens > 0 {
		// No max tokens known, just show token count
		contextStr = formatTokenCount(session.TotalTokens)
	} else {
		contextStr = "0% context"
	}

	if session.MaxTokens > 0 {
		pct := float64(session.TotalTokens) / float64(session.MaxTokens) * 100
		if pct >= 90 {
			return statusContextError.Render(contextStr)
		} else if pct >= 70 {
			return statusContextWarning.Render(contextStr)
		}
	}
	return statusItemStyle.Render(contextStr)
}

func (m model) renderState(state api.AgentState) string {
	switch state {
	case api.AgentStateRunning:
		elapsed := ""
		if !m.thinkingStart.IsZero() {
			elapsed = " " + formatDuration(time.Since(m.thinkingStart))
		}
		return statusActiveStyle.Render("● Running") + statusItemStyle.Render(elapsed)
	case api.AgentStateInitializing:
		return statusItemStyle.Render(" Initializing...")
	case api.AgentStateWaitingForInput:
		return statusActiveStyle.Render("● Ready")
	case api.AgentStateIdle:
		return statusItemStyle.Render("○ Idle")
	case api.AgentStateDone:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("✓ Done")
	case api.AgentStateExited:
		return statusItemStyle.Render("○ Exited")
	default:
		return statusItemStyle.Render(string(state))
	}
}

func (m model) renderDivider() string {
	return dividerStyle.Render(strings.Repeat("─", m.width))
}

func (m model) renderInputArea() string {
	state := m.cachedState

	if state == api.AgentStateWaitingForInput && len(m.messages) > 0 {
		if lastMsg := m.messages[len(m.messages)-1]; lastMsg.Type == api.MessageTypeUserChoiceRequest {
			if _, ok := lastMsg.Payload.(*api.UserChoiceRequest); ok {
				return lipgloss.NewStyle().Padding(0, 1).Render(m.list.View())
			}
		}
	}

	inputContent := m.textinput.View()
	var box string
	if state == api.AgentStateRunning || state == api.AgentStateInitializing {
		elapsed := ""
		if !m.thinkingStart.IsZero() {
			elapsed = " " + formatDuration(time.Since(m.thinkingStart))
		}
		content := spinnerStyle.Render(m.spinner.View()) + statusActiveStyle.Render(" Thinking...") + statusItemStyle.Render(elapsed)
		// Ensure height matches input box (roughly) to prevent jump
		content = lipgloss.NewStyle().Height(1).Render(content)
		box = inputBoxDimStyle.Width(m.width - 4).Render(content)
	} else {
		box = inputBoxStyle.Width(m.width - 4).Render(inputContent)
	}

	return lipgloss.NewStyle().Padding(0, 1).Render(box)
}

func (m model) renderHelp() string {
	var hints []string
	if m.cachedState == api.AgentStateRunning {
		hints = []string{"Thinking...", "Ctrl+C: cancel"}
	} else {
		hints = []string{"Enter: send", "Esc: clear", "Ctrl+C: quit"}
	}
	if m.viewport.TotalLineCount() > m.viewport.Height {
		hints = append(hints, "↑/↓: scroll")
	}
	return helpStyle.Padding(0, 2).Render(strings.Join(hints, " • "))
}

func (m model) renderSessionPicker() string {
	if len(m.sessionPickerItems) == 0 {
		return lipgloss.NewStyle().Padding(1, 2).Foreground(colorTextMuted).Render("No sessions found")
	}

	titleStyle := lipgloss.NewStyle().Foreground(colorWarning).Bold(true).Padding(0, 0, 1, 0)
	title := titleStyle.Render("Select a session:")

	// Calculate available height for the list
	availableHeight := m.height - 8 // status bar, dividers, help, title
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Build the table rows
	var rows []string

	// Header
	headerStyle := lipgloss.NewStyle().Foreground(colorTextDim).Bold(true)
	header := headerStyle.Render(fmt.Sprintf("  %-12s  %-16s  %-16s  %-20s  %s",
		"ID", "Created", "Last Modified", "Model", "Messages"))
	rows = append(rows, header)

	// Separator
	rows = append(rows, lipgloss.NewStyle().Foreground(colorTextDim).Render(strings.Repeat("─", m.width-4)))

	// Session rows - use consistent formatting without relying on style padding
	selectedStyle := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorTextMuted)

	for i, session := range m.sessionPickerItems {
		// Truncate ID for display
		displayID := session.ID
		if len(displayID) > 12 {
			displayID = displayID[:12]
		}

		created := session.CreatedAt.Format("Jan 02 15:04")
		modified := session.LastModified.Format("Jan 02 15:04")

		model := session.ModelID
		if len(model) > 20 {
			model = model[:17] + "..."
		}
		if model == "" {
			model = "-"
		}

		// Build row with consistent prefix width
		row := fmt.Sprintf("%-12s  %-16s  %-16s  %-20s  %d",
			displayID, created, modified, model, session.MessageCount)

		if i == m.sessionPickerIndex {
			rows = append(rows, selectedStyle.Render("> "+row))
		} else {
			rows = append(rows, normalStyle.Render("  "+row))
		}

		// Stop if we've filled the available height
		if len(rows) >= availableHeight {
			break
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, content),
	)
}

func (m model) renderSessionPickerHelp() string {
	hints := []string{"↑/↓: navigate", "Enter: select", "Esc: cancel"}
	return helpStyle.Padding(0, 2).Render(strings.Join(hints, " • "))
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func formatTokenCount(tokens int64) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d tokens", tokens)
	}
	if tokens < 1000000 {
		return fmt.Sprintf("%.1fk tokens", float64(tokens)/1000)
	}
	return fmt.Sprintf("%.1fM tokens", float64(tokens)/1000000)
}
