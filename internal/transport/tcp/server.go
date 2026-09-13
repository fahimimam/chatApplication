package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fahimimam/chatApplication/internal/application"
	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/logger"
	"github.com/fahimimam/chatApplication/internal/repository"
)

// Server is the TCP transport. It binds to a listener, accepts connections,
// and runs each connection in its own goroutine. All business logic is
// delegated to the application services.
type Server struct {
	auth  *application.AuthService
	convs *application.ConversationService
	msgs  *application.MessageService
	users repository.UserRepository
	log   *logger.Logger

	// presence tracks every logged-in user across all open connections.
	presence *presenceServer

	// subscribers is the set of sessions that have opted in to receiving
	// real-time EVT pushes (incoming messages, presence changes). A nil
	// map entry means "subscribed". Per-session writes are serialised by
	// session.wmu, so we only need a coarse lock to mutate the set.
	subMu  sync.Mutex
	subscribers map[*session]struct{}

	// Hook is invoked whenever a successful message is sent through
	// SendMessage. The transport uses it to fan out realtime notifications
	// to subscribed clients. nil disables realtime.
	Hook func(conversationID domain.ConversationID, message domain.Message)
}

// NewServer wires the transport to its dependencies.
func NewServer(
	auth *application.AuthService,
	convs *application.ConversationService,
	msgs *application.MessageService,
	users repository.UserRepository,
	log *logger.Logger,
) *Server {
	if log == nil {
		log = logger.Discard()
	}
	return &Server{
		auth:        auth,
		convs:       convs,
		msgs:        msgs,
		users:       users,
		log:         log,
		presence:    newPresence(),
		subscribers: map[*session]struct{}{},
	}
}

// Serve accepts connections from l until it is closed.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.log.Warn("accept", "error", err.Error())
			continue
		}
		go s.handle(ctx, conn)
	}
}

// session is the per-connection state.
type session struct {
	conn   net.Conn
	r      *bufio.Reader
	wmu    sync.Mutex
	w      *bufio.Writer
	user   *domain.User
	token  domain.SessionToken
	server *Server
	reqID  string // correlation id from the current command, if any

	// subscribed is set by `subscribe` and reset by `unsubscribe`. While
	// true, the server pushes EVT lines as they happen.
	subscribed bool
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	sess := &session{
		conn:   conn,
		r:      bufio.NewReader(conn),
		w:      bufio.NewWriter(conn),
		server: s,
	}
	sess.writeLine(PrefixOK, "welcome to chatd. try: register|login|who|ping|quit")

	for {
		line, err := sess.r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				s.log.Debug("read", "error", err.Error())
			}
			s.cleanupSession(sess)
			return
		}
		verb, args, comment, perr := ParseCommand(line)
		if perr != nil {
			sess.errorf("%s", perr.Error())
			continue
		}
		if comment {
			continue
		}
		// Optional correlation id "id <n> <verb> ..." — used by the
		// in-process client to multiplex requests. We strip it; the
		// server doesn't need it.
		if verb == "id" && len(args) >= 2 {
			sess.reqID = args[0]
			verb = args[1]
			args = args[2:]
		} else {
			sess.reqID = ""
		}
		s.dispatch(ctx, sess, verb, args)
		if sess.server.shouldClose(verb) {
			s.cleanupSession(sess)
			return
		}
	}
}

func (s *Server) shouldClose(verb string) bool { return verb == CmdQuit }

// dispatch routes a single command to the right handler.
func (s *Server) dispatch(ctx context.Context, sess *session, verb string, args []string) {
	switch verb {
	case CmdRegister:
		s.cmdRegister(ctx, sess, args)
	case CmdLogin:
		s.cmdLogin(ctx, sess, args)
	case CmdLogout:
		s.cmdLogout(ctx, sess)
	case CmdList:
		s.cmdList(ctx, sess)
	case CmdOpen:
		s.cmdOpen(ctx, sess, args)
	case CmdHistory:
		s.cmdHistory(ctx, sess, args)
	case CmdSend:
		s.cmdSend(ctx, sess, args)
	case CmdWhoami:
		s.cmdWhoami(sess)
	case CmdWho:
		s.cmdWho(sess)
	case CmdUsers:
		s.cmdUsers(ctx, sess, args)
	case CmdSubscribe:
		s.cmdSubscribe(sess)
	case CmdUnsub:
		s.cmdUnsubscribe(sess)
	case CmdPing:
		sess.writeLine(PrefixOK, "pong")
	case CmdQuit:
		sess.writeLine(PrefixBYE, "bye")
	default:
		sess.errorf("unknown command: %s", verb)
	}
}

// --- handlers ---

func (s *Server) cmdRegister(ctx context.Context, sess *session, args []string) {
	if len(args) < 2 {
		sess.errorf("usage: register <username> <password> [display_name]")
		return
	}
	username := args[0]
	password := args[1]
	display := ""
	if len(args) >= 3 {
		display = strings.Join(args[2:], " ")
	}
	u, err := s.auth.Register(ctx, username, password, display)
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("registered %s id=%s", u.Username, u.ID))
}

func (s *Server) cmdLogin(ctx context.Context, sess *session, args []string) {
	if len(args) < 2 {
		sess.errorf("usage: login <username> <password>")
		return
	}
	tok, u, err := s.auth.Login(ctx, args[0], args[1])
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	sess.user = &u
	sess.token = tok
	s.presence.markOnline(u, sess)
	sess.writeLine(PrefixOK, fmt.Sprintf("logged in id=%s", u.ID))
	s.broadcastPresence(u.Username, true)
}

func (s *Server) cmdLogout(ctx context.Context, sess *session) {
	if sess.token == "" {
		sess.errorf("not logged in")
		return
	}
	username := ""
	if sess.user != nil {
		username = sess.user.Username
	}
	_ = s.auth.Logout(ctx, sess.token)
	sess.token = ""
	sess.user = nil
	sess.writeLine(PrefixOK, "logged out")
	if username != "" {
		s.broadcastPresence(username, false)
	}
}

func (s *Server) cmdList(ctx context.Context, sess *session) {
	if !sess.requireUser() {
		return
	}
	convs, err := s.convs.ListForUser(ctx, sess.user.ID)
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	for _, c := range convs {
		title := c.Title
		if title == "" {
			title = string(c.Kind)
		}
		// Resolve a sensible title for direct chats: the other user's
		// username. Group chats use their stored title.
		if c.Kind == domain.KindDirect {
			if ps, perr := s.convs.Participants(ctx, sess.user.ID, c.ID); perr == nil {
				for _, p := range ps {
					if p.UserID != sess.user.ID {
						if u, uerr := s.users.ByID(ctx, p.UserID); uerr == nil {
							title = u.Username
						}
						break
					}
				}
			}
		}
		sess.writeLine(PrefixOK, fmt.Sprintf("conv id=%s kind=%s title=%s", c.ID, c.Kind, title))
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("(%d)", len(convs)))
}

func (s *Server) cmdOpen(ctx context.Context, sess *session, args []string) {
	if !sess.requireUser() {
		return
	}
	if len(args) < 1 {
		sess.errorf("usage: open <username>")
		return
	}
	other, err := s.users.ByUsername(ctx, args[0])
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	c, err := s.convs.CreateDirect(ctx, sess.user.ID, other.ID)
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("conv id=%s kind=%s title=%s", c.ID, c.Kind, other.Username))
}

func (s *Server) cmdHistory(ctx context.Context, sess *session, args []string) {
	if !sess.requireUser() {
		return
	}
	if len(args) < 1 {
		sess.errorf("usage: history <conv_id> [before_ms] [limit]")
		return
	}
	in := application.HistoryInput{
		ConversationID: domain.ConversationID(args[0]),
		CallerID:       sess.user.ID,
	}
	// before_ms <= 0 means "start from newest".
	if len(args) >= 2 {
		ms, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			sess.errorf("bad before_ms: %s", err.Error())
			return
		}
		if ms > 0 {
			in.Before = time.UnixMilli(ms).UTC()
		}
	}
	if len(args) >= 3 {
		lim, err := strconv.Atoi(args[2])
		if err != nil {
			sess.errorf("bad limit: %s", err.Error())
			return
		}
		in.Limit = lim
	}
	msgs, err := s.msgs.History(ctx, in)
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	// Reverse to oldest-first so the client sees a chronological log.
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		// Resolve sender username so the TUI doesn't have to look it up.
		senderName := string(m.SenderID)
		if u, uerr := s.users.ByID(ctx, m.SenderID); uerr == nil {
			senderName = u.Username
		}
		sess.writeLine(PrefixOK, fmt.Sprintf("msg id=%s sender=%s sender_name=%s conv=%s ts=%d content=%q",
			m.ID, m.SenderID, senderName, m.ConversationID, m.CreatedAt.UnixMilli(), m.Content))
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("end (%d)", len(msgs)))
}

func (s *Server) cmdSend(ctx context.Context, sess *session, args []string) {
	if !sess.requireUser() {
		return
	}
	if len(args) < 2 {
		sess.errorf("usage: send <conv_id> <message...>")
		return
	}
	convID := domain.ConversationID(args[0])
	content := strings.Join(args[1:], " ")

	m, err := s.msgs.Send(ctx, application.SendInput{
		ConversationID: convID,
		SenderID:       sess.user.ID,
		Content:        content,
	})
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("sent id=%s ts=%d", m.ID, m.CreatedAt.UnixMilli()))

	// Broadcast to subscribers. The server is the source of truth, so the
	// sender's other sessions also receive the push (useful when a user
	// is connected twice). The TUI dedups by message id.
	s.broadcastMessage(m, sess.user.Username)

	if s.Hook != nil {
		s.Hook(m.ConversationID, m)
	}
}

func (s *Server) cmdWhoami(sess *session) {
	if !sess.requireUser() {
		return
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("user id=%s username=%s display=%s",
		sess.user.ID, sess.user.Username, sess.user.DisplayName))
}

// cmdWho lists users currently connected to the server.
func (s *Server) cmdWho(sess *session) {
	if !sess.requireUser() {
		return
	}
	for _, e := range s.presence.snapshot() {
		sess.writeLine(PrefixOK, fmt.Sprintf("online id=%s username=%s display=%s conn=%d",
			e.UserID, e.Username, e.DisplayName, e.Connections))
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("(%d)", s.presence.size()))
}

// cmdUsers lists every user known to the directory. With one arg it acts
// as a substring filter — handy for picking a username when starting a new
// chat. The list is sorted by username for predictable display.
func (s *Server) cmdUsers(ctx context.Context, sess *session, args []string) {
	if !sess.requireUser() {
		return
	}
	filter := ""
	if len(args) >= 1 {
		filter = strings.ToLower(strings.TrimSpace(args[0]))
	}
	users, err := s.users.ListAll(ctx)
	if err != nil {
		sess.errorf("%s", err.Error())
		return
	}
	count := 0
	for _, u := range users {
		if filter != "" && !strings.Contains(strings.ToLower(u.Username), filter) {
			continue
		}
		count++
		online := 0
		if s.presence.isOnline(u.ID) {
			online = 1
		}
		sess.writeLine(PrefixOK, fmt.Sprintf("user id=%s username=%s display=%s online=%d",
			u.ID, u.Username, u.DisplayName, online))
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("(%d)", count))
}

func (s *Server) cmdSubscribe(sess *session) {
	if !sess.requireUser() {
		return
	}
	s.subMu.Lock()
	s.subscribers[sess] = struct{}{}
	s.subMu.Unlock()
	sess.subscribed = true
	sess.writeLine(PrefixOK, "subscribed")
}

func (s *Server) cmdUnsubscribe(sess *session) {
	s.subMu.Lock()
	delete(s.subscribers, sess)
	s.subMu.Unlock()
	sess.subscribed = false
	sess.writeLine(PrefixOK, "unsubscribed")
}

// --- session helpers ---

func (sess *session) requireUser() bool {
	if sess.user == nil {
		sess.errorf("not logged in")
		return false
	}
	return true
}

func (sess *session) errorf(format string, args ...any) {
	sess.writeLine(PrefixERR, fmt.Sprintf(format, args...))
}

// writeLine prefixes the line with an optional correlation id so the
// in-process client can route the response back to the right caller.
func (sess *session) writeLine(prefix, body string) {
	sess.wmu.Lock()
	defer sess.wmu.Unlock()
	if sess.reqID != "" {
		_, _ = sess.w.WriteString("id ")
		_, _ = sess.w.WriteString(sess.reqID)
		_ = sess.w.WriteByte(' ')
	}
	_, _ = sess.w.WriteString(prefix)
	_, _ = sess.w.WriteString(body)
	_ = sess.w.WriteByte('\n')
	_ = sess.w.Flush()
}

// writeEvent pushes a server-initiated line (no correlation id). Used for
// EVT messages and presence notifications.
func (sess *session) writeEvent(prefix, body string) {
	sess.wmu.Lock()
	defer sess.wmu.Unlock()
	_, _ = sess.w.WriteString(prefix)
	_, _ = sess.w.WriteString(body)
	_ = sess.w.WriteByte('\n')
	_ = sess.w.Flush()
}

// cleanupSession removes the session from presence + subscribers when the
// connection drops.
func (s *Server) cleanupSession(sess *session) {
	if sess == nil {
		return
	}
	s.subMu.Lock()
	delete(s.subscribers, sess)
	s.subMu.Unlock()
	if sess.user != nil {
		s.presence.markOffline(sess)
		s.broadcastPresence(sess.user.Username, false)
	}
}

// broadcastMessage fans an EVT out to every subscribed session that is a
// participant of convID. Format:
//   EVT msg id=<uuid> conv=<uuid> sender=<id> sender_name=<u> ts=<ms> content="<text>"
func (s *Server) broadcastMessage(m domain.Message, senderName string) {
	body := fmt.Sprintf("msg id=%s conv=%s sender=%s sender_name=%s ts=%d content=%q",
		m.ID, m.ConversationID, m.SenderID, senderName, m.CreatedAt.UnixMilli(), m.Content)
	s.subMu.Lock()
	targets := make([]*session, 0, len(s.subscribers))
	for sess := range s.subscribers {
		if sess.user == nil {
			continue
		}
		isMember, err := s.msgs.IsConversationMember(context.Background(), sess.user.ID, m.ConversationID)
		if err != nil || !isMember {
			continue
		}
		targets = append(targets, sess)
	}
	s.subMu.Unlock()
	for _, t := range targets {
		t.writeEvent(PrefixEVT, body)
	}
}

// broadcastPresence sends "EVT presence username=<u> online=<0|1>" to all
// subscribers so TUIs can refresh their sidebar.
func (s *Server) broadcastPresence(username string, online bool) {
	v := 0
	if online {
		v = 1
	}
	body := fmt.Sprintf("presence username=%s online=%d", username, v)
	s.subMu.Lock()
	targets := make([]*session, 0, len(s.subscribers))
	for sess := range s.subscribers {
		targets = append(targets, sess)
	}
	s.subMu.Unlock()
	for _, t := range targets {
		t.writeEvent(PrefixEVT, body)
	}
}