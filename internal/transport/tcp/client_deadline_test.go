package tcp

import (
	"context"
	"testing"
	"time"
)

// TestClientSurvivesSlowLogin is a regression test for a bug where the
// client.Dial helper set a 5-second read deadline on the connection to
// bound the welcome-banner read but never cleared it. Any command sent
// more than 5 seconds after Dial would fail with context.DeadlineExceeded
// even though the per-call context still had time left, because the
// underlying conn still had the stale deadline.
//
// Now Dial clears the read deadline after reading the welcome banner,
// so per-call timeouts are governed solely by the caller's context.
func TestClientSurvivesSlowLogin(t *testing.T) {
	addr, stop := testServer(t)
	defer stop()

	c, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Register first so a later login has something to match. (Register
	// does not log the user in.)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Register(ctx, "alice", "password123", ""); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Wait longer than the old stale deadline before logging in.
	time.Sleep(6 * time.Second)

	loginCtx, loginCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer loginCancel()
	if err := c.Login(loginCtx, "alice", "password123"); err != nil {
		t.Fatalf("login after slow delay: %v", err)
	}
}
