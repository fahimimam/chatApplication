package tcp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// Client is a blocking-style wrapper around a single chatd connection. It
// owns the read/write goroutine pair and exposes typed methods that the
// TUI uses directly. The client is goroutine-safe.
type Client struct {
	addr string
	conn net.Conn
	r    *bufio.Reader
	wmu  sync.Mutex
	w    *bufio.Writer

	// pending maps in-flight requests to a channel that delivers the next
	// non-BYE server line.
	mu       sync.Mutex
	pending  map[int]chan string
	nextID   int
	closeCh  chan struct{}
	closeErr error

	// eventCh receives unsolicited server-pushed messages (e.g. EVT).
	eventCh chan string
}

// Dial connects to a chatd server. The caller must Close.
func Dial(addr string) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	c := &Client{
		addr:    addr,
		conn:    conn,
		r:       bufio.NewReader(conn),
		w:       bufio.NewWriter(conn),
		pending: make(map[int]chan string),
		closeCh: make(chan struct{}),
		eventCh: make(chan string, 16),
	}
	// Read the welcome banner synchronously BEFORE starting the read loop,
	// so we don't race with subsequent responses.
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	welcome, err := c.r.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("welcome: %w", err)
	}
	if strings.HasPrefix(welcome, PrefixERR) {
		_ = conn.Close()
		return nil, fmt.Errorf("%s", strings.TrimPrefix(welcome, PrefixERR))
	}
	// Clear the read deadline so subsequent reads are bounded only by
	// the per-call context, not by the 5s we used for the welcome read.
	// Without this, any command sent more than 5s after Dial would time
	// out because the underlying conn still has the stale deadline.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("clear read deadline: %w", err)
	}
	go c.readLoop()
	_ = welcome
	return c, nil
}

// Close terminates the connection.
func (c *Client) Close() error {
	select {
	case <-c.closeCh:
		return c.closeErr
	default:
	}
	err := c.conn.Close()
	c.closeErr = err
	close(c.closeCh)
	return err
}

// Events returns the channel of unsolicited server events. The channel is
// closed when the connection closes.
func (c *Client) Events() <-chan string { return c.eventCh }

// Register creates a new account.
func (c *Client) Register(ctx context.Context, username, password, display string) error {
	return c.simple(ctx, CmdRegister, username, password, display)
}

// Login authenticates and stashes the session for the lifetime of the
// connection.
func (c *Client) Login(ctx context.Context, username, password string) error {
	return c.simple(ctx, CmdLogin, username, password)
}

// Logout ends the session on the server side.
func (c *Client) Logout(ctx context.Context) error { return c.simple(ctx, CmdLogout) }

// ListConversations returns one conversation per entry.
func (c *Client) ListConversations(ctx context.Context) ([]string, error) {
	return c.collectOK(ctx, CmdList)
}

// OpenConversation creates (or finds) a direct conversation with username
// and returns its id.
func (c *Client) OpenConversation(ctx context.Context, username string) (string, error) {
	lines, err := c.collectOK(ctx, CmdOpen, username)
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "conv id=") {
			rest := strings.TrimPrefix(line, "conv id=")
			if sp := strings.IndexByte(rest, ' '); sp >= 0 {
				rest = rest[:sp]
			}
			return rest, nil
		}
	}
	return "", errors.New("server did not return a conversation id")
}

// History returns up to limit messages from convID (oldest first). If
// beforeUnixMilli is non-zero the server returns only messages strictly
// older than that timestamp. limit <= 0 means "use the server default".
//
// The wire format is "history <conv> [before_ms] [limit]". The client
// always sends both slots (defaulting to 0) so positional parsing on the
// server side is unambiguous.
func (c *Client) History(ctx context.Context, convID string, beforeUnixMilli int64, limit int) ([]string, error) {
	if beforeUnixMilli < 0 {
		beforeUnixMilli = 0
	}
	if limit < 0 {
		limit = 0
	}
	return c.collectOK(ctx, CmdHistory, convID,
		fmt.Sprintf("%d", beforeUnixMilli),
		fmt.Sprintf("%d", limit))
}

// Send posts a message and returns the assigned message id.
func (c *Client) Send(ctx context.Context, convID, content string) (string, error) {
	lines, err := c.collectOK(ctx, CmdSend, convID, content)
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "sent id=") {
			rest := strings.TrimPrefix(line, "sent id=")
			if i := strings.IndexByte(rest, ' '); i >= 0 {
				return rest[:i], nil
			}
			return rest, nil
		}
	}
	return "", errors.New("server did not acknowledge send")
}

// Ping sends a keepalive and waits for pong.
func (c *Client) Ping(ctx context.Context) error { return c.simple(ctx, CmdPing) }

// Whoami asks the server for the current user.
func (c *Client) Whoami(ctx context.Context) (string, error) {
	lines, err := c.collectOK(ctx, CmdWhoami)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", errors.New("empty whoami response")
	}
	return strings.TrimPrefix(lines[0], "OK "), nil
}

// Quit cleanly logs out and closes the connection.
func (c *Client) Quit(ctx context.Context) error {
	defer c.Close()
	return c.simple(ctx, CmdQuit)
}

// --- internal helpers ---

func (c *Client) simple(ctx context.Context, verb string, args ...string) error {
	_, err := c.collectOK(ctx, verb, args...)
	return err
}

func (c *Client) collectOK(ctx context.Context, verb string, args ...string) ([]string, error) {
	id, ch, err := c.startRequest(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.writeRequest(id, verb, args...); err != nil {
		c.cancelRequest(id)
		return nil, err
	}

	var lines []string
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				return lines, io.EOF
			}
			if strings.HasPrefix(line, PrefixERR) {
				c.cancelRequest(id)
				return nil, errors.New(strings.TrimPrefix(line, PrefixERR))
			}
			if strings.HasPrefix(line, PrefixBYE) {
				return lines, nil
			}
			// Strip the OK prefix from collected lines.
			stripped := strings.TrimPrefix(line, PrefixOK)
			lines = append(lines, stripped)
			if isTerminal(verb, stripped) {
				c.completeRequest(id)
				return lines, nil
			}
		case <-ctx.Done():
			c.cancelRequest(id)
			return nil, ctx.Err()
		}
	}
}

// isTerminal recognises the response that signals "end of multi-line reply".
// For now we use a per-verb marker so the protocol stays simple.
func isTerminal(verb, line string) bool {
	if verb == CmdHistory && strings.HasPrefix(line, "end ") {
		return true
	}
	if verb == CmdList && strings.HasPrefix(line, "(") && strings.HasSuffix(line, ")") {
		return true
	}
	if verb == CmdPing && line == "pong" {
		return true
	}
	if verb == CmdLogin && strings.HasPrefix(line, "logged in id=") {
		return true
	}
	if verb == CmdRegister && strings.HasPrefix(line, "registered ") {
		return true
	}
	if verb == CmdLogout && line == "logged out" {
		return true
	}
	if verb == CmdWhoami && strings.HasPrefix(line, "user ") {
		return true
	}
	if verb == CmdOpen && strings.HasPrefix(line, "conv id=") {
		return true
	}
	if verb == CmdSend && strings.HasPrefix(line, "sent id=") {
		return true
	}
	return false
}

// readOK is unused now (welcome is read synchronously in Dial). It is
// retained so external callers can still drain a single line if needed.
func (c *Client) readOK(ctx context.Context) (string, error) {
	select {
	case line := <-c.eventCh:
		if strings.HasPrefix(line, PrefixBYE) {
			return "", io.EOF
		}
		return strings.TrimPrefix(line, PrefixOK), nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-c.closeCh:
		return "", io.EOF
	}
}

func (c *Client) startRequest(ctx context.Context) (int, chan string, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan string, 8)
	c.pending[id] = ch
	c.mu.Unlock()
	return id, ch, nil
}

func (c *Client) cancelRequest(id int) {
	c.mu.Lock()
	ch, ok := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ok {
		close(ch)
	}
}

func (c *Client) completeRequest(id int) {
	c.mu.Lock()
	ch, ok := c.pending[id]
	delete(c.pending, id)
	c.mu.Unlock()
	if ok {
		close(ch)
	}
}

// writeRequest sends "id <n> <verb> [args...]" to the server. The id is a
// monotonically increasing integer the read loop uses to route responses
// back to the right caller.
func (c *Client) writeRequest(id int, verb string, args ...string) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, _ = fmt.Fprintf(c.w, "id %d %s", id, verb)
	for _, a := range args {
		_ = c.w.WriteByte(' ')
		_, _ = c.w.WriteString(a)
	}
	_ = c.w.WriteByte('\n')
	return c.w.Flush()
}

// readLoop reads server lines, routing them to the right pending request
// when prefixed with "id N ", and to the event channel otherwise.
func (c *Client) readLoop() {
	for {
		line, err := c.r.ReadString('\n')
		if err != nil {
			c.mu.Lock()
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = map[int]chan string{}
			c.mu.Unlock()
			return
		}
		line = strings.TrimRight(line, "\r\n")

		// Try to route to a pending request.
		if strings.HasPrefix(line, "id ") {
			rest := strings.TrimPrefix(line, "id ")
			sp := strings.IndexByte(rest, ' ')
			if sp > 0 {
				idStr := rest[:sp]
				body := rest[sp+1:]
				id := atoi(idStr)
				c.mu.Lock()
				ch, ok := c.pending[id]
				c.mu.Unlock()
				if ok {
					ch <- body
					continue
				}
			}
		}

		// Unsolicited — push to event channel for the caller.
		select {
		case c.eventCh <- line:
		default:
			// Drop if backlog full; the client will resync on next request.
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// _ silences unused-import for domain in case future methods need it.
var _ = domain.ErrNotFound