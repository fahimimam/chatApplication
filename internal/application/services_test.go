package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/repository/sqlite"
)

func newServiceFixture(t *testing.T) (*ConversationService, *MessageService, *AuthService, *sqlite.Repos) {
	t.Helper()
	d, err := openInMemoryDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repos := sqlite.New(d)
	clock, _ := newTestClock()
	auth := NewAuthService(repos.Users, repos.Sessions, clock)
	conv := NewConversationService(repos.Conversations, repos.Users, clock)
	msg := NewMessageService(repos.Messages, repos.Conversations, repos.Users, clock)
	return conv, msg, auth, repos
}

func seedUser(t *testing.T, auth *AuthService, username string) domain.User {
	t.Helper()
	u, err := auth.Register(context.Background(), username, "password123", username)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestConversationCreateDirectIsIdempotent(t *testing.T) {
	ctx := context.Background()
	conv, _, auth, _ := newServiceFixture(t)
	alice := seedUser(t, auth, "alice")
	bob := seedUser(t, auth, "bob")

	c1, err := conv.CreateDirect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := conv.CreateDirect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c1.ID != c2.ID {
		t.Fatalf("CreateDirect should be idempotent: %s vs %s", c1.ID, c2.ID)
	}
}

func TestConversationCreateDirectRejectsSelf(t *testing.T) {
	ctx := context.Background()
	conv, _, auth, _ := newServiceFixture(t)
	alice := seedUser(t, auth, "alice")
	if _, err := conv.CreateDirect(ctx, alice.ID, alice.ID); err == nil {
		t.Fatal("expected error")
	}
}

func TestConversationList(t *testing.T) {
	ctx := context.Background()
	conv, _, auth, _ := newServiceFixture(t)
	alice := seedUser(t, auth, "alice")
	bob := seedUser(t, auth, "bob")
	carol := seedUser(t, auth, "carol")

	if _, err := conv.CreateDirect(ctx, alice.ID, bob.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := conv.CreateDirect(ctx, alice.ID, carol.ID); err != nil {
		t.Fatal(err)
	}

	list, err := conv.ListForUser(ctx, alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("alice should have 2 conversations, got %d", len(list))
	}
}

func TestMessageSendAndHistory(t *testing.T) {
	ctx := context.Background()
	d, err := openInMemoryDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repos := sqlite.New(d)

	// Use a clock that advances by a millisecond on every call so message
	// timestamps differ.
	var i int
	clock := func() time.Time {
		i++
		return time.Date(2025, 1, 1, 12, 0, 0, i*int(time.Millisecond), time.UTC)
	}
	auth := NewAuthService(repos.Users, repos.Sessions, clock)
	conv := NewConversationService(repos.Conversations, repos.Users, clock)
	msg := NewMessageService(repos.Messages, repos.Conversations, repos.Users, clock)

	alice := seedUser(t, auth, "alice")
	bob := seedUser(t, auth, "bob")

	c, err := conv.CreateDirect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := msg.Send(ctx, SendInput{ConversationID: c.ID, SenderID: alice.ID, Content: "hi"}); err != nil {
		t.Fatal(err)
	}
	if _, err := msg.Send(ctx, SendInput{ConversationID: c.ID, SenderID: bob.ID, Content: "hello"}); err != nil {
		t.Fatal(err)
	}

	history, err := msg.History(ctx, HistoryInput{ConversationID: c.ID, CallerID: alice.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("want 2 messages, got %d", len(history))
	}
	if history[0].SenderID != bob.ID {
		t.Fatalf("history should be newest-first; got sender=%s", history[0].SenderID)
	}
	if history[1].SenderID != alice.ID {
		t.Fatalf("oldest should be alice; got %s", history[1].SenderID)
	}
}

func TestMessageSendForbidden(t *testing.T) {
	ctx := context.Background()
	conv, msg, auth, _ := newServiceFixture(t)
	alice := seedUser(t, auth, "alice")
	bob := seedUser(t, auth, "bob")
	eve := seedUser(t, auth, "eve")

	c, err := conv.CreateDirect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = msg.Send(ctx, SendInput{ConversationID: c.ID, SenderID: eve.ID, Content: "sneaky"})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestMessageHistoryForbidden(t *testing.T) {
	ctx := context.Background()
	conv, msg, auth, _ := newServiceFixture(t)
	alice := seedUser(t, auth, "alice")
	bob := seedUser(t, auth, "bob")
	eve := seedUser(t, auth, "eve")

	c, err := conv.CreateDirect(ctx, alice.ID, bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := msg.Send(ctx, SendInput{ConversationID: c.ID, SenderID: alice.ID, Content: "hi"}); err != nil {
		t.Fatal(err)
	}

	_, err = msg.History(ctx, HistoryInput{ConversationID: c.ID, CallerID: eve.ID, Limit: 10})
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestMessageSendValidation(t *testing.T) {
	ctx := context.Background()
	_, msg, auth, _ := newServiceFixture(t)
	alice := seedUser(t, auth, "alice")

	cases := []struct {
		name string
		in   SendInput
	}{
		{"no conv", SendInput{SenderID: alice.ID, Content: "hi"}},
		{"no sender", SendInput{ConversationID: domain.ConversationID("x"), Content: "hi"}},
		{"empty content", SendInput{ConversationID: domain.ConversationID("x"), SenderID: alice.ID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := msg.Send(ctx, tc.in)
			if err == nil {
				t.Fatal("want error")
			}
		})
	}
}