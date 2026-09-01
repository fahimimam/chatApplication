package tcp

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Reuses testServer from server_test.go (same package).

func TestClientRegisterLoginAndOpen(t *testing.T) {
	addr, stop := testServer(t)
	defer stop()

	c, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Register(ctx, "alice", "password123", "Alice"); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := c.Login(ctx, "alice", "password123"); err != nil {
		t.Fatalf("login: %v", err)
	}

	// Register a second user via a separate connection so we can chat.
	c2, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	if err := c2.Register(ctx, "bob", "password123", "Bob"); err != nil {
		t.Fatalf("register bob: %v", err)
	}

	convID, err := c.OpenConversation(ctx, "bob")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if convID == "" {
		t.Fatal("empty conv id")
	}
	t.Logf("convID=%q", convID)

	if _, err := c.Send(ctx, convID, "hello from alice"); err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Logf("sent")

	hist, err := c.History(ctx, convID, 0, 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	t.Logf("history lines: %#v", hist)
	joined := strings.Join(hist, "\n")
	if !strings.Contains(joined, "hello from alice") {
		t.Fatalf("expected message in history, got %q", joined)
	}

	// Regression: history with limit but no before_ms must not return
	// empty (the client sends "0 50" not just "50"). Sending just "50"
	// caused the server to interpret 50 as before_ms (unix epoch+50ms),
	// which excluded every real message.
	histLimit, err := c.History(ctx, convID, 0, 50)
	if err != nil {
		t.Fatalf("history with limit: %v", err)
	}
	if !strings.Contains(strings.Join(histLimit, "\n"), "hello from alice") {
		t.Fatalf("limit-only history lost messages: %v", histLimit)
	}
}

func TestClientInvalidLogin(t *testing.T) {
	addr, stop := testServer(t)
	defer stop()

	c, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Register(ctx, "carol", "password123", ""); err != nil {
		t.Fatal(err)
	}
	err = c.Login(ctx, "carol", "wrong")
	if err == nil || !strings.Contains(err.Error(), "invalid credential") {
		t.Fatalf("want invalid credential, got %v", err)
	}
}