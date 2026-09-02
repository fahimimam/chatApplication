package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/transport/tcp"
)

// View is the top-level screen.
type View int

const (
	ViewLogin View = iota
	ViewConvs
	ViewChat
)

// AuthMode is whether the login screen is for register or login.
type AuthMode int

const (
	ModeLogin AuthMode = iota
	ModeRegister
)

// Model is the Bubble Tea root model. It owns the TCP client, the focused
// view, and the per-view sub-models.
type Model struct {
	client *tcp.Client
	width  int
	height int

	view View

	// login screen state
	authMode  AuthMode
	username  textinput.Model
	password  textinput.Model
	status    string
	statusErr bool

	// logged-in user (only known after login completes)
	user domain.User

	// conversation list
	convs   []ConvSummary
	cursor  int
	loading bool

	// chat view
	current *ConvSummary
	messages []MessageRow
	viewport viewport.Model
	input    textinput.Model

	// shared
	quitting bool
	err      error
}

// Styles. Keep them simple — the goal is to be readable, not flashy.
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F56"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#3C3C3C"))
	meStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#7D56F4"))
	themStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
)

// NewModel returns an uninitialised model. The caller wires the client in.
func NewModel() *Model {
	u := textinput.New()
	u.Placeholder = "username"
	u.Focus()
	u.CharLimit = 32
	u.Width = 30

	p := textinput.New()
	p.Placeholder = "password"
	p.EchoMode = textinput.EchoPassword
	p.EchoCharacter = '•'
	p.CharLimit = 128
	p.Width = 30

	v := viewport.New(80, 20)
	v.SetContent("")

	ci := textinput.New()
	ci.Placeholder = "type a message and press Enter"
	ci.CharLimit = 4096

	return &Model{
		authMode: ModeLogin,
		username: u,
		password: p,
		viewport: v,
		input:    ci,
	}
}

// Init kicks off the first async action (just sizing for now).
func (m *Model) Init() tea.Cmd { return nil }

// SetClient is called once the dial succeeds.
func (m *Model) SetClient(c *tcp.Client) {
	m.client = c
	m.view = ViewLogin
}

// Messages from the TCP read loop or async work.

// errMsg carries an async error to the Update loop.
type errMsg struct{ err error }

// convsLoadedMsg carries a freshly fetched conversation list.
type convsLoadedMsg struct{ items []ConvSummary }

// messagesLoadedMsg carries history fetched for a conversation.
type messagesLoadedMsg struct {
	conv domain.ConversationID
	msgs []MessageRow
}

// authedMsg reports that the login succeeded and we know who the user is.
type authedMsg struct {
	username string
}

// openConversationMsg tells the model to transition to the chat view for
// a freshly-opened conversation and load its history.
type openConversationMsg struct {
	conv ConvSummary
}

func loadConvs(c *tcp.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		raw, err := c.ListConversations(ctx)
		if err != nil {
			return errMsg{err}
		}
		var out []ConvSummary
		for _, line := range raw {
			if IsEndLine(line) {
				continue
			}
			cs, err := ParseConv(line)
			if err != nil {
				continue
			}
			out = append(out, cs)
		}
		return convsLoadedMsg{items: out}
	}
}

func loadHistory(c *tcp.Client, conv domain.ConversationID) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		raw, err := c.History(ctx, string(conv), 0, 100)
		if err != nil {
			return errMsg{err}
		}
		var out []MessageRow
		for _, line := range raw {
			if IsEndLine(line) {
				continue
			}
			mr, err := ParseMessage(line)
			if err != nil {
				continue
			}
			out = append(out, mr)
		}
		return messagesLoadedMsg{conv: conv, msgs: out}
	}
}

// Update handles keyboard and async messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4 // header + input + chrome
		m.input.Width = msg.Width
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case authedMsg:
		m.user.Username = msg.username
		m.view = ViewConvs
		m.loading = true
		return m, loadConvs(m.client)
	case convsLoadedMsg:
		m.loading = false
		m.convs = msg.items
		return m, nil
	case openConversationMsg:
		cs := msg.conv
		m.current = &cs
		m.messages = nil
		m.input.Placeholder = "type a message and press Enter"
		m.view = ViewChat
		m.status = fmt.Sprintf("opened conversation with %s", cs.Title)
		m.statusErr = false
		return m, loadHistory(m.client, cs.ID)
	case messagesLoadedMsg:
		if m.current != nil && m.current.ID == msg.conv {
			m.messages = msg.msgs
			m.viewport.GotoBottom()
		}
		return m, nil
	case errMsg:
		errStr := msg.err.Error()
		// Friendly hint: "invalid credential" usually means the user
		// hasn't registered yet. Suggest switching to register.
		if strings.Contains(errStr, "invalid credential") && m.authMode == ModeLogin {
			errStr += "  — tip: press ctrl+r to switch to register"
		}
		m.status = errStr
		m.statusErr = true
		m.loading = false
		return m, nil
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewLogin:
		return m.handleLoginKey(msg)
	case ViewConvs:
		return m.handleConvsKey(msg)
	case ViewChat:
		return m.handleChatKey(msg)
	}
	return m, nil
}

func (m *Model) handleLoginKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "shift+tab":
		if m.username.Focused() {
			m.username.Blur()
			m.password.Focus()
		} else {
			m.password.Blur()
			m.username.Focus()
		}
		return m, nil
	case "ctrl+r":
		if m.authMode == ModeLogin {
			m.authMode = ModeRegister
		} else {
			m.authMode = ModeLogin
		}
		return m, nil
	case "enter":
		return m.submitAuth()
	case "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	if m.username.Focused() {
		m.username, cmd = m.username.Update(msg)
	} else {
		m.password, cmd = m.password.Update(msg)
	}
	return m, cmd
}

func (m *Model) submitAuth() (tea.Model, tea.Cmd) {
	u := strings.TrimSpace(m.username.Value())
	p := m.password.Value()
	if u == "" || p == "" {
		m.status = "username and password are required"
		m.statusErr = true
		return m, nil
	}
	m.status = ""
	m.statusErr = false
	mode := m.authMode
	return m, func() tea.Msg {
		// Per-command context with timeout. The lifetime matches this
		// closure, not the calling Update — otherwise the deferred
		// cancel() in the caller would cancel ctx before the closure
		// even runs.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		if mode == ModeRegister {
			err = m.client.Register(ctx, u, p, u)
		} else {
			err = m.client.Login(ctx, u, p)
		}
		if err != nil {
			return errMsg{err}
		}
		return authedMsg{username: u}
	}
}

func (m *Model) handleConvsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.convs)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < 0 || m.cursor >= len(m.convs) {
			return m, nil
		}
		cs := m.convs[m.cursor]
		m.current = &cs
		m.view = ViewChat
		m.input.Focus()
		return m, loadHistory(m.client, cs.ID)
	case "n":
		// Open conversation with arbitrary user via a prompt. For the
		// initial cut we use a small inline prompt by toggling view.
		m.view = ViewChat
		m.current = nil
		m.messages = nil
		m.input.Placeholder = "open: <username>"
		m.input.Focus()
		m.input.SetValue("")
	case "r":
		// Refresh conversation list.
		return m, loadConvs(m.client)
	case "ctrl+c", "esc":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = ViewConvs
		m.input.Blur()
		m.input.SetValue("")
		m.input.Placeholder = "type a message and press Enter"
		return m, nil
	case "enter":
		return m.submitChatLine()
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) submitChatLine() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.input.Value())
	if line == "" {
		return m, nil
	}
	m.input.SetValue("")

	// Slash-style meta commands.
	switch {
	case strings.HasPrefix(line, "/open "):
		username := strings.TrimSpace(strings.TrimPrefix(line, "/open "))
		if username == "" {
			m.status = "usage: /open <username>"
			m.statusErr = true
			return m, nil
		}
		// Run synchronously inside the cmd so the response is available
		// before we transition views.
		return m, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			convID, err := m.client.OpenConversation(ctx, username)
			if err != nil {
				return errMsg{err}
			}
			return openConversationMsg{
				conv: ConvSummary{
					ID:    domain.ConversationID(convID),
					Kind:  domain.KindDirect,
					Title: username,
				},
			}
		}
	case strings.HasPrefix(line, "/quit"), line == "/exit":
		m.quitting = true
		return m, tea.Quit
	case strings.HasPrefix(line, "/"):
		m.status = "unknown command: " + line
		m.statusErr = true
		return m, nil
	}

	if m.current == nil {
		m.status = "open a conversation first (/open <username>)"
		m.statusErr = true
		return m, nil
	}

	convID := m.current.ID
	body := line
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := m.client.Send(ctx, string(convID), body)
		if err != nil {
			return errMsg{err}
		}
		// Refresh history after a successful send.
		raw, herr := m.client.History(ctx, string(convID), 0, 100)
		if herr != nil {
			return errMsg{herr}
		}
		var msgs []MessageRow
		for _, ln := range raw {
			if IsEndLine(ln) {
				continue
			}
			mr, perr := ParseMessage(ln)
			if perr != nil {
				continue
			}
			msgs = append(msgs, mr)
		}
		return messagesLoadedMsg{conv: convID, msgs: msgs}
	}
}

// View renders the current screen.
func (m *Model) View() string {
	if m.quitting {
		return "bye.\n"
	}
	switch m.view {
	case ViewLogin:
		return m.viewLogin()
	case ViewConvs:
		return m.viewConvs()
	case ViewChat:
		return m.viewChat()
	}
	return ""
}
