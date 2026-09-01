package tcp

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/fahimimam/chatApplication/internal/application"
	"github.com/fahimimam/chatApplication/internal/infra/db"
	"github.com/fahimimam/chatApplication/internal/logger"
	"github.com/fahimimam/chatApplication/internal/repository/sqlite"
)

// testServer spins up a chat server on a random localhost port and returns
// its address plus a cleanup function.
func testServer(t *testing.T) (addr string, closer func()) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	repos := sqlite.New(d)

	// Use a clock that advances per call so timestamps differ.
	var i int
	clock := func() time.Time {
		i++
		return time.Date(2025, 1, 1, 12, 0, 0, i*int(time.Millisecond), time.UTC)
	}
	auth := application.NewAuthService(repos.Users, repos.Sessions, clock)
	conv := application.NewConversationService(repos.Conversations, repos.Users, clock)
	msg := application.NewMessageService(repos.Messages, repos.Conversations, repos.Users, clock)

	srv := NewServer(auth, conv, msg, repos.Users, logger.Discard())

	// Listen on a free port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = l.Addr().String()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = srv.Serve(ctx, l)
	}()

	return addr, func() {
		cancel()
		_ = l.Close()
		_ = d.Close()
	}
}

type client struct {
	conn net.Conn
	r    *bufio.Reader
}

func newClient(t *testing.T, addr string) *client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	return &client{conn: conn, r: bufio.NewReader(conn)}
}

func (c *client) close() { _ = c.conn.Close() }

func (c *client) cmd(format string, args ...any) {
	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}
	_, _ = c.conn.Write([]byte(format + "\n"))
}

func (c *client) readLine(t *testing.T) string {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := c.r.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return strings.TrimRight(line, "\r\n")
}

func (c *client) expectOK(t *testing.T, bodyContains string) {
	t.Helper()
	line := c.readLine(t)
	if !strings.HasPrefix(line, PrefixOK) {
		t.Fatalf("want OK, got %q", line)
	}
	if bodyContains != "" && !strings.Contains(line, bodyContains) {
		t.Fatalf("want body to contain %q, got %q", bodyContains, line)
	}
}

func (c *client) expectERR(t *testing.T, bodyContains string) {
	t.Helper()
	line := c.readLine(t)
	if !strings.HasPrefix(line, PrefixERR) {
		t.Fatalf("want ERR, got %q", line)
	}
	if bodyContains != "" && !strings.Contains(line, bodyContains) {
		t.Fatalf("want body to contain %q, got %q", bodyContains, line)
	}
}

func TestServerEndToEnd(t *testing.T) {
	addr, stop := testServer(t)
	defer stop()

	alice := newClient(t, addr)
	defer alice.close()
	bob := newClient(t, addr)
	defer bob.close()

	alice.expectOK(t, "welcome")
	bob.expectOK(t, "welcome")

	// Register both users.
	alice.cmd("register alice password123 Alice")
	alice.expectOK(t, "registered alice")
	bob.cmd("register bob password123 Bob")
	bob.expectOK(t, "registered bob")

	// Login.
	alice.cmd("login alice password123")
	alice.expectOK(t, "logged in")
	bob.cmd("login bob password123")
	bob.expectOK(t, "logged in")

	// Open a conversation.
	alice.cmd("open bob")
	convLine := alice.readLine(t)
	if !strings.HasPrefix(convLine, PrefixOK) || !strings.Contains(convLine, "conv id=") {
		t.Fatalf("open: %q", convLine)
	}
	// Parse "OK conv id=<uuid> kind=direct" -> id
	var convID string
	for _, field := range strings.Fields(strings.TrimPrefix(convLine, PrefixOK)) {
		if strings.HasPrefix(field, "id=") {
			convID = strings.TrimPrefix(field, "id=")
			break
		}
	}
	if convID == "" {
		t.Fatalf("could not parse conv id from %q", convLine)
	}

	// Send a message.
	alice.cmd("send %s hello-from-alice", convID)
	alice.expectOK(t, "sent id=")

	// History from bob's side.
	bob.cmd("history %s", convID)
	// One message line then an end marker.
	msgLine := bob.readLine(t)
	if !strings.HasPrefix(msgLine, PrefixOK) || !strings.Contains(msgLine, "msg id=") {
		t.Fatalf("history msg: %q", msgLine)
	}
	if !strings.Contains(msgLine, "hello-from-alice") {
		t.Fatalf("history content: %q", msgLine)
	}
	bob.expectOK(t, "end")

	// List conversations for alice.
	alice.cmd("list")
	line := alice.readLine(t)
	if !strings.HasPrefix(line, PrefixOK) || !strings.Contains(line, "conv id=") {
		t.Fatalf("list: %q", line)
	}
	tail := alice.readLine(t)
	if !strings.HasPrefix(tail, PrefixOK) || !strings.Contains(tail, "(1)") {
		t.Fatalf("list end: %q", tail)
	}

	// Unauthenticated history is rejected.
	bob.cmd("logout")
	bob.expectOK(t, "logged out")
	bob.cmd("history %s", convID)
	bob.expectERR(t, "not logged in")
}

func TestServerForbiddenMessage(t *testing.T) {
	addr, stop := testServer(t)
	defer stop()

	alice := newClient(t, addr)
	defer alice.close()
	bob := newClient(t, addr)
	defer bob.close()
	eve := newClient(t, addr)
	defer eve.close()

	alice.expectOK(t, "welcome")
	bob.expectOK(t, "welcome")
	eve.expectOK(t, "welcome")

	alice.cmd("register alice password123 Alice")
	alice.expectOK(t, "registered")
	bob.cmd("register bob password123 Bob")
	bob.expectOK(t, "registered")
	eve.cmd("register eve password123 Eve")
	eve.expectOK(t, "registered")

	alice.cmd("login alice password123")
	alice.expectOK(t, "logged in")
	eve.cmd("login eve password123")
	eve.expectOK(t, "logged in")

	alice.cmd("open bob")
	convLine := alice.readLine(t)
	var convID string
	for _, field := range strings.Fields(strings.TrimPrefix(convLine, PrefixOK)) {
		if strings.HasPrefix(field, "id=") {
			convID = strings.TrimPrefix(field, "id=")
			break
		}
	}
	if convID == "" {
		t.Fatalf("could not parse conv id from %q", convLine)
	}

	eve.cmd("send %s sneaky", convID)
	eve.expectERR(t, "forbidden")
}