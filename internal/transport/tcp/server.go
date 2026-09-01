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
	auth     *application.AuthService
	convs    *application.ConversationService
	msgs     *application.MessageService
	users    repository.UserRepository
	log      *logger.Logger

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
	return &Server{auth: auth, convs: convs, msgs: msgs, users: users, log: log}
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
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	sess := &session{
		conn:   conn,
		r:      bufio.NewReader(conn),
		w:      bufio.NewWriter(conn),
		server: s,
	}
	sess.writeLine(PrefixOK, "welcome to chatd. try: register|login|ping|quit")

	for {
		line, err := sess.r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				s.log.Debug("read", "error", err.Error())
			}
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
		s.dispatch(ctx, sess, verb, args)
		if sess.server.shouldClose(verb) {
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
	sess.writeLine(PrefixOK, fmt.Sprintf("logged in id=%s", u.ID))
}

func (s *Server) cmdLogout(ctx context.Context, sess *session) {
	if sess.token == "" {
		sess.errorf("not logged in")
		return
	}
	_ = s.auth.Logout(ctx, sess.token)
	sess.token = ""
	sess.user = nil
	sess.writeLine(PrefixOK, "logged out")
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
	sess.writeLine(PrefixOK, fmt.Sprintf("conv id=%s kind=%s", c.ID, c.Kind))
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
	if len(args) >= 2 {
		ms, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			sess.errorf("bad before_ms: %s", err.Error())
			return
		}
		in.Before = time.UnixMilli(ms).UTC()
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
		sess.writeLine(PrefixOK, fmt.Sprintf("msg id=%s sender=%s ts=%d content=%q",
			m.ID, m.SenderID, m.CreatedAt.UnixMilli(), m.Content))
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

	if s.Hook != nil {
		s.Hook(m.ConversationID, m)
	}
}

func (s *Server) cmdWhoami(sess *session) {
	if !sess.requireUser() {
		return
	}
	sess.writeLine(PrefixOK, fmt.Sprintf("user id=%s username=%s", sess.user.ID, sess.user.Username))
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

func (sess *session) writeLine(prefix, body string) {
	sess.wmu.Lock()
	defer sess.wmu.Unlock()
	_, _ = sess.w.WriteString(prefix)
	_, _ = sess.w.WriteString(body)
	_ = sess.w.WriteByte('\n')
	_ = sess.w.Flush()
}