// Package tui implements the terminal chat client. The TUI speaks the
// chatd TCP protocol via internal/transport/tcp.Client and renders the
// result with Bubble Tea.
//
// Layout: a three-pane grid with a status/help bar at the bottom.
//
//   ┌──────────────┬──────────────────────────────────────┬──────────────────┐
//   │ conversations│             chat log                 │   online users   │
//   │   (left)     │            (centre)                  │     (right)      │
//   ├──────────────┴──────────────────────────────────────┴──────────────────┤
//   │ status / help bar (1 line)                                            │
//   ├────────────────────────────────────────────────────────────────────────┤
//   │ input                                                                  │
//   └────────────────────────────────────────────────────────────────────────┘
package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/transport/tcp"
)

// View is the top-level screen.
type View int

const (
	ViewLogin View = iota
	ViewMain
)

// AuthMode is whether the login screen is for register or login.
type AuthMode int

const (
	ModeLogin AuthMode = iota
	ModeRegister
)

// Pane identifies which sub-area of the main view has focus.
type Pane int

const (
	PaneConvs Pane = iota
	PaneChat
	PaneUsers
)

// Model is the Bubble Tea root model. It owns the TCP client, the focused
// view, and the per-pane state.
type Model struct {
	client *tcp.Client
	width  int
	height int

	view     View
	authMode AuthMode

	// login screen
	username textinput.Model
	password textinput.Model

	// logged-in user
	user     domain.User
	meLoaded bool

	// main screen state
	pane        Pane
	convs       []ConvSummary
	convCursor  int
	convLoading bool

	users    []UserEntry
	userCur  int

	current  *ConvSummary
	messages []MessageRow
	viewport viewport.Model
	input    textinput.Model

	// overlays
	overlay     OverlayKind
	overlayBuf  textinput.Model

	// status / help
	status    string
	statusErr bool
	statusAt  time.Time
	helpText  string

	quitting bool
	err      error

	// notifications buffer — small list of push messages we show in the
	// status bar.
	notifs []string
}

// OverlayKind identifies modal screens.
type OverlayKind int

const (
	OverlayNone OverlayKind = iota
	OverlayNewChat
	OverlaySearchUsers
	OverlayHelp
)

// UserEntry is a row in the right-hand sidebar.
type UserEntry struct {
	ID       domain.UserID
	Username string
	Display  string
	Online   bool
	Self     bool
}

// --- messages from async commands / server events ---

type errMsg struct{ err error }

type convsLoadedMsg struct{ items []ConvSummary }
type whoLoadedMsg struct {
	online   []UserEntry
	allUsers []UserEntry
}
type usersLoadedMsg struct {
	users []UserEntry
}
type messagesLoadedMsg struct {
	conv domain.ConversationID
	msgs []MessageRow
}
type authedMsg struct{ user domain.User }
type openConversationMsg struct{ conv ConvSummary }

// serverEventMsg wraps a single EVT line from the server. The model
// matches on the prefix.
type serverEventMsg struct{ line string }

// subscribedMsg is sent after Subscribe succeeds. The model responds by
// scheduling a NextEventCmd to begin forwarding server events to the
// Update loop.
type subscribedMsg struct{}

// connectionClosedMsg signals the underlying TCP connection was lost. The
// model displays a clear error and exits.
type connectionClosedMsg struct{}

// NextEventCmd returns a tea.Cmd that waits for the next server event
// from the underlying TCP client and forwards it to the model. The cmd
// reschedules itself so events flow continuously until the connection
// closes.
func (m *Model) NextEventCmd() tea.Cmd {
	c := m.client
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		events := c.Events()
		line, ok := <-events
		if !ok {
			return connectionClosedMsg{}
		}
		if !strings.HasPrefix(line, tcp.PrefixEVT) {
			// Non-EVT lines (e.g. EVT-less server messages) get
			// forwarded too, but stripped of any prefix.
			return serverEventMsg{line: strings.TrimPrefix(line, "EVT ")}
		}
		return serverEventMsg{line: strings.TrimPrefix(line, tcp.PrefixEVT)}
	}
}

// tickMsg drives the periodic refresh of conv/who lists.
type tickMsg time.Time

// Styles live in views.go.

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

	v := viewport.New(40, 20)
	v.SetContent("")

	ci := textinput.New()
	ci.Placeholder = "type a message — enter to send"
	ci.CharLimit = 4096

	ob := textinput.New()
	ob.CharLimit = 64
	ob.Width = 40

	return &Model{
		authMode:  ModeLogin,
		username:  u,
		password:  p,
		viewport:  v,
		input:     ci,
		overlayBuf: ob,
		pane:     PaneConvs,
		helpText: "tab: switch pane · ↑/↓: move · enter: open · n: new chat · /: command · ?: help · ctrl+c: quit",
	}
}

// Init kicks off the async event subscription and periodic refresh.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.subscribeCmd(),
		tickCmd(),
	)
}

// subscribeCmd subscribes the underlying TCP client to server-pushed
// events. If the call fails because the user is not logged in yet, the
// model silently retries on the next authedMsg — there's no point in
// showing "not logged in" in the status bar for a benign race during
// startup.
func (m *Model) subscribeCmd() tea.Cmd {
	c := m.client
	if c == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := c.Subscribe(ctx); err != nil {
			if strings.Contains(err.Error(), "not logged in") {
				return subscribedMsg{} // will retry after login
			}
			return errMsg{err}
		}
		return subscribedMsg{}
	}
}

// SetClient is called once the dial succeeds.
func (m *Model) SetClient(c *tcp.Client) {
	m.client = c
	m.view = ViewLogin
}

// --- tea commands ---

func tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadConvsCmd(c *tcp.Client) tea.Cmd {
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

func loadWhoCmd(c *tcp.Client, me domain.User) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		onlineRaw, oerr := c.Who(ctx)
		if oerr != nil {
			return errMsg{oerr}
		}
		usersRaw, uerr := c.Users(ctx, "")
		if uerr != nil {
			return errMsg{uerr}
		}
		online := parseOnline(onlineRaw, me)
		all := parseUsers(usersRaw, me, online)
		return whoLoadedMsg{online: online, allUsers: all}
	}
}

func loadUsersCmd(c *tcp.Client, me domain.User, filter string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		raw, err := c.Users(ctx, filter)
		if err != nil {
			return errMsg{err}
		}
		onlineRaw, _ := c.Who(ctx)
		online := parseOnline(onlineRaw, me)
		all := parseUsers(raw, me, online)
		return usersLoadedMsg{users: all}
	}
}

func loadHistoryCmd(c *tcp.Client, conv domain.ConversationID) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		raw, err := c.History(ctx, string(conv), 0, 200)
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
		// Server returns oldest-first; no reversal needed.
		return messagesLoadedMsg{conv: conv, msgs: out}
	}
}

// parseOnline turns "online id=... username=... display=... conn=N" rows
// into UserEntry values.
func parseOnline(lines []string, me domain.User) []UserEntry {
	var out []UserEntry
	for _, line := range lines {
		if !strings.HasPrefix(line, "online ") {
			continue
		}
		var u UserEntry
		for _, f := range fieldsAfter(line, "online ") {
			switch {
			case strings.HasPrefix(f, "id="):
				u.ID = domain.UserID(strings.TrimPrefix(f, "id="))
			case strings.HasPrefix(f, "username="):
				u.Username = strings.TrimPrefix(f, "username=")
			case strings.HasPrefix(f, "display="):
				u.Display = strings.TrimPrefix(f, "display=")
			}
		}
		if u.Username == "" {
			continue
		}
		u.Online = true
		u.Self = u.Username == me.Username
		out = append(out, u)
	}
	return out
}

// parseUsers turns "user id=... username=... display=... online=N" rows
// into UserEntry values. The online map overlays the online flag.
func parseUsers(lines []string, me domain.User, online []UserEntry) []UserEntry {
	onlineSet := map[string]bool{}
	for _, o := range online {
		onlineSet[o.Username] = true
	}
	var out []UserEntry
	seen := map[string]bool{}
	for _, line := range lines {
		if !strings.HasPrefix(line, "user ") {
			continue
		}
		var u UserEntry
		for _, f := range fieldsAfter(line, "user ") {
			switch {
			case strings.HasPrefix(f, "id="):
				u.ID = domain.UserID(strings.TrimPrefix(f, "id="))
			case strings.HasPrefix(f, "username="):
				u.Username = strings.TrimPrefix(f, "username=")
			case strings.HasPrefix(f, "display="):
				u.Display = strings.TrimPrefix(f, "display=")
			}
		}
		if u.Username == "" || seen[u.Username] {
			continue
		}
		seen[u.Username] = true
		u.Online = onlineSet[u.Username]
		u.Self = u.Username == me.Username
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username) })
	return out
}

// Update handles keyboard and async messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.relayout()
		return m, nil

	case tickMsg:
		if m.view == ViewMain && m.client != nil && m.overlay == OverlayNone {
			cmds = append(cmds, tea.Batch(loadConvsCmd(m.client), loadWhoCmd(m.client, m.user)))
		}
		cmds = append(cmds, tickCmd())
		return m, tea.Batch(cmds...)

	case subscribedMsg:
		// Begin forwarding server events to the UI.
		return m, m.NextEventCmd()

	case serverEventMsg:
		evtCmd := m.handleServerEvent(msg.line)
		// Always reschedule so the next event is consumed.
		if evtCmd != nil {
			return m, tea.Batch(evtCmd, m.NextEventCmd())
		}
		return m, m.NextEventCmd()

	case connectionClosedMsg:
		m.setStatus("server connection closed", true)
		m.quitting = true
		return m, tea.Quit

	case errMsg:
		m.setStatus(msg.err.Error(), true)
		return m, nil

	case authedMsg:
		m.user = msg.user
		m.meLoaded = true
		m.view = ViewMain
		// Clear any leftover status (e.g. "not logged in" from the
		// early subscribe attempt before login).
		m.status = ""
		m.statusErr = false
		m.notifs = nil
		// Make sure we are subscribed to realtime events now that we
		// are authenticated. The earlier Subscribe in Init may have
		// failed with "not logged in"; we retry here.
		cmds = append(cmds, m.subscribeCmd())
		cmds = append(cmds, tea.Batch(
			loadConvsCmd(m.client),
			loadWhoCmd(m.client, m.user),
		))
		m.relayout()
		return m, tea.Batch(cmds...)

	case convsLoadedMsg:
		m.convs = msg.items
		if m.convCursor >= len(m.convs) {
			m.convCursor = max(0, len(m.convs)-1)
		}
		return m, nil

	case whoLoadedMsg:
		m.users = msg.allUsers
		if m.userCur >= len(m.users) {
			m.userCur = max(0, len(m.users)-1)
		}
		return m, nil

	case usersLoadedMsg:
		m.users = msg.users
		if m.userCur >= len(m.users) {
			m.userCur = max(0, len(m.users)-1)
		}
		return m, nil

	case messagesLoadedMsg:
		if m.current != nil && m.current.ID == msg.conv {
			m.messages = msg.msgs
			m.viewport.GotoBottom()
		}
		return m, nil

	case openConversationMsg:
		cs := msg.conv
		m.current = &cs
		m.messages = nil
		m.pane = PaneChat
		m.input.Focus()
		m.relayout()
		cmds = append(cmds, loadHistoryCmd(m.client, cs.ID))
		m.setStatus(fmt.Sprintf("opened conversation with %s", displayTitle(cs)), false)
		return m, tea.Batch(cmds...)
	}

	// Keyboard.
	if km, ok := msg.(tea.KeyMsg); ok {
		return m.handleKey(km)
	}
	return m, nil
}

func (m *Model) handleServerEvent(line string) tea.Cmd {
	switch {
	case strings.HasPrefix(line, "msg "):
		mr, err := ParseMessage(line)
		if err != nil {
			return nil
		}
		// Always append to the message log if it belongs to the open
		// conversation, even if the user is on another pane.
		if m.current != nil && m.current.ID == mr.ConversationID {
			m.messages = append(m.messages, mr)
			m.viewport.GotoBottom()
			m.setStatus(fmt.Sprintf("new message in %s", displayTitle(*m.current)), false)
		} else {
			m.notifs = append(m.notifs, fmt.Sprintf("new message from %s", shortID(string(mr.Sender))))
			if len(m.notifs) > 5 {
				m.notifs = m.notifs[len(m.notifs)-5:]
			}
			m.setStatus(fmt.Sprintf("new message from %s — open %s to view", shortID(string(mr.Sender)), shortID(string(mr.ConversationID))), false)
		}
		// Refresh conv list so new conversations bubble up.
		if m.client != nil {
			return loadConvsCmd(m.client)
		}
		return nil
	case strings.HasPrefix(line, "presence "):
		body := strings.TrimPrefix(line, "presence ")
		var (
			uname string
			isOn  bool
		)
		for _, f := range fieldsAfter(body, "") {
			switch {
			case strings.HasPrefix(f, "username="):
				uname = strings.TrimPrefix(f, "username=")
			case strings.HasPrefix(f, "online="):
				isOn = f[len("online="):] == "1"
			}
		}
		if uname == "" {
			return nil
		}
		for i := range m.users {
			if m.users[i].Username == uname {
				m.users[i].Online = isOn
			}
		}
		if isOn {
			m.setStatus(uname+" joined", false)
		} else {
			m.setStatus(uname+" left", false)
		}
	}
	return nil
}

// handleKey is the keyboard router.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Overlays consume keys first.
	if m.overlay != OverlayNone {
		return m.handleOverlayKey(msg)
	}
	switch m.view {
	case ViewLogin:
		return m.handleLoginKey(msg)
	case ViewMain:
		return m.handleMainKey(msg)
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
		m.setStatus("username and password are required", true)
		return m, nil
	}
	mode := m.authMode
	displayName := u
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var err error
		if mode == ModeRegister {
			err = m.client.Register(ctx, u, p, displayName)
			if err != nil {
				return errMsg{err}
			}
			// Register does not log the user in. Auto-login so the
			// account is usable immediately. If the user already
			// existed and login fails (e.g. wrong password), surface
			// that error instead.
			if err := m.client.Login(ctx, u, p); err != nil {
				return errMsg{err}
			}
		} else {
			if err := m.client.Login(ctx, u, p); err != nil {
				return errMsg{err}
			}
		}
		// Resolve whoami to get our user id.
		who, werr := m.client.Whoami(ctx)
		if werr != nil {
			return errMsg{werr}
		}
		u := domain.User{Username: nameFromWhoami(who), ID: idFromWhoami(who)}
		return authedMsg{user: u}
	}
}

func (m *Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always-on global keys (work no matter what has focus).
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "tab":
		// Cycle focus pane left → right → back to input when chat open.
		m.cyclePane(+1)
		return m, nil
	case "shift+tab":
		m.cyclePane(-1)
		return m, nil
	}

	// If the chat input is focused, every other key goes to it. This is
	// what lets the user type "going north" without `n` opening the new-
	// chat overlay, or "/help me" without `?` flipping to the help
	// overlay. enter / esc / backspace all reach handleChatKey which
	// knows how to submit / clear.
	if m.pane == PaneChat && m.input.Focused() {
		return m.handleChatKey(msg)
	}

	// Global shortcuts (only reachable when no text input has focus).
	switch msg.String() {
	case "?":
		m.overlay = OverlayHelp
		return m, nil
	case "esc":
		if m.current != nil {
			m.current = nil
			m.messages = nil
			m.input.Blur()
			m.input.SetValue("")
			m.pane = PaneConvs
			m.relayout()
		}
		return m, nil
	case "/":
		// Focus the chat input and pre-fill a slash so the user can
		// start a slash command directly. Only valid when a chat is
		// actually open — otherwise we open the new-chat overlay
		// behaviour by falling through.
		if m.pane == PaneChat && m.current != nil {
			m.input.Focus()
			m.input.SetValue("/")
			return m, nil
		}
	case "n":
		m.openOverlay(OverlayNewChat, "open chat with username:", "")
		return m, nil
	case "u":
		m.openOverlay(OverlaySearchUsers, "search users:", "")
		return m, nil
	case "r":
		if m.client != nil {
			return m, tea.Batch(loadConvsCmd(m.client), loadWhoCmd(m.client, m.user))
		}
	}

	// Pane-specific keys.
	switch m.pane {
	case PaneConvs:
		return m.handleConvsKey(msg)
	case PaneUsers:
		return m.handleUsersKey(msg)
	case PaneChat:
		return m.handleChatKey(msg)
	}
	return m, nil
}

func (m *Model) handleConvsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.convs) == 0 {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if m.convCursor > 0 {
			m.convCursor--
		}
	case "down", "j":
		if m.convCursor < len(m.convs)-1 {
			m.convCursor++
		}
	case "enter":
		if m.convCursor >= 0 && m.convCursor < len(m.convs) {
			cs := m.convs[m.convCursor]
			m.current = &cs
			m.pane = PaneChat
			m.relayout()
			return m, loadHistoryCmd(m.client, cs.ID)
		}
	}
	return m, nil
}

func (m *Model) handleUsersKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.users) == 0 {
		return m, nil
	}
	switch msg.String() {
	case "up", "k":
		if m.userCur > 0 {
			m.userCur--
		}
	case "down", "j":
		if m.userCur < len(m.users)-1 {
			m.userCur++
		}
	case "enter":
		if m.userCur >= 0 && m.userCur < len(m.users) {
			u := m.users[m.userCur]
			if u.Self {
				m.setStatus("that's you — pick someone else", true)
				return m, nil
			}
			return m, m.openChatWith(u.Username)
		}
	}
	return m, nil
}

func (m *Model) handleChatKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.current == nil {
		// chat pane focused but no open conversation; just type into input.
	}
	switch msg.String() {
	case "enter":
		return m.submitChatLine()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// submitChatLine handles Enter inside the chat input. It dispatches slash
// commands and otherwise sends a chat message to the current conversation.
func (m *Model) submitChatLine() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(m.input.Value())
	if line == "" {
		return m, nil
	}
	m.input.SetValue("")

	switch {
	case strings.HasPrefix(line, "/open "):
		username := strings.TrimSpace(strings.TrimPrefix(line, "/open "))
		if username == "" {
			m.setStatus("usage: /open <username>", true)
			return m, nil
		}
		return m, m.openChatWith(username)
	case line == "/quit" || line == "/exit":
		m.quitting = true
		return m, tea.Quit
	case line == "/logout":
		return m, m.logoutCmd()
	case line == "/users":
		m.pane = PaneUsers
		m.relayout()
		return m, loadWhoCmd(m.client, m.user)
	case line == "/help" || line == "/?":
		m.overlay = OverlayHelp
		return m, nil
	case strings.HasPrefix(line, "/"):
		m.setStatus("unknown command: "+line+" (try /help)", true)
		return m, nil
	}

	if m.current == nil {
		m.setStatus("open a conversation first — press n or /open <user>", true)
		return m, nil
	}

	convID := m.current.ID
	body := line
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := m.client.Send(ctx, string(convID), body); err != nil {
			return errMsg{err}
		}
		// Refresh history to pick up server-side timestamps.
		return m.refreshHistoryCmd(convID)()
	}
}

// refreshHistoryCmd returns a tea.Cmd that reloads the conversation
// history (used after sending a message).
func (m *Model) refreshHistoryCmd(conv domain.ConversationID) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		raw, err := c.History(ctx, string(conv), 0, 200)
		if err != nil {
			return errMsg{err}
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
		// Server returns oldest-first; no reversal needed.
		return messagesLoadedMsg{conv: conv, msgs: msgs}
	}
}

func (m *Model) openChatWith(username string) tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		convID, err := c.OpenConversation(ctx, username)
		if err != nil {
			return errMsg{err}
		}
		cs := ConvSummary{
			ID:    domain.ConversationID(convID),
			Kind:  domain.KindDirect,
			Title: username,
		}
		// Pull full conv info so the conv list reflects it.
		return openConversationMsg{conv: cs}
	}
}

func (m *Model) logoutCmd() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.Logout(ctx)
		return authedMsg{user: domain.User{}}
	}
}

// --- overlays ---

func (m *Model) openOverlay(kind OverlayKind, placeholder, initial string) {
	m.overlay = kind
	m.overlayBuf.Placeholder = placeholder
	m.overlayBuf.SetValue(initial)
	m.overlayBuf.Focus()
}

func (m *Model) closeOverlay() {
	m.overlay = OverlayNone
	m.overlayBuf.Blur()
	m.overlayBuf.SetValue("")
}

func (m *Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeOverlay()
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.overlayBuf.Value())
		switch m.overlay {
		case OverlayNewChat:
			m.closeOverlay()
			if val == "" {
				return m, nil
			}
			return m, m.openChatWith(val)
		case OverlaySearchUsers:
			m.closeOverlay()
			if val == "" {
				val = ""
			}
			return m, loadUsersCmd(m.client, m.user, val)
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.overlayBuf, cmd = m.overlayBuf.Update(msg)
	return m, cmd
}

// --- helpers ---

func (m *Model) setStatus(s string, isErr bool) {
	m.status = s
	m.statusErr = isErr
	m.statusAt = time.Now()
}

func (m *Model) cyclePane(delta int) {
	order := []Pane{PaneConvs, PaneChat, PaneUsers}
	idx := 0
	for i, p := range order {
		if p == m.pane {
			idx = i
			break
		}
	}
	for {
		idx = (idx + delta + len(order)) % len(order)
		next := order[idx]
		// Skip chat pane if no conversation is open.
		if next == PaneChat && m.current == nil {
			continue
		}
		m.pane = next
		break
	}
	switch m.pane {
	case PaneChat:
		m.input.Focus()
	default:
		m.input.Blur()
	}
	m.relayout()
}

// relayout recomputes the geometry of every pane based on the current
// terminal size. Safe to call multiple times.
func (m *Model) relayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	const (
		footerLines = 3 // status + input + help
		topReserve  = 2 // title + spacer
	)
	gridHeight := m.height - footerLines - topReserve
	if gridHeight < 5 {
		gridHeight = 5
	}
	leftW := m.width / 4
	if leftW < 18 {
		leftW = 18
	}
	rightW := m.width / 4
	if rightW < 18 {
		rightW = 18
	}
	centreW := m.width - leftW - rightW - 6
	if centreW < 20 {
		centreW = 20
	}
	m.viewport.Width = centreW - 2
	m.viewport.Height = gridHeight - 2
	m.input.Width = m.width - 2
}

// View renders the current screen.
func (m *Model) View() string {
	if m.quitting {
		return "bye.\n"
	}
	switch m.view {
	case ViewLogin:
		return m.viewLogin()
	case ViewMain:
		return m.viewMain()
	}
	return ""
}

// helpers for whoami response parsing.

func idFromWhoami(line string) domain.UserID {
	// "user id=… username=… display=…"
	for _, f := range fieldsAfter(line, "user ") {
		if strings.HasPrefix(f, "id=") {
			return domain.UserID(strings.TrimPrefix(f, "id="))
		}
	}
	return ""
}

func nameFromWhoami(line string) string {
	for _, f := range fieldsAfter(line, "user ") {
		if strings.HasPrefix(f, "username=") {
			return strings.TrimPrefix(f, "username=")
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
