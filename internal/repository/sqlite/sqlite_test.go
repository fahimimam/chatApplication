package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/infra/db"
)

// newTestRepos returns a Repos backed by an in-memory SQLite database with
// migrations applied. The caller does not need to close anything.
func newTestRepos(t *testing.T) *Repos {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.Migrate(context.Background(), d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(d)
}

func TestUserRepoCreateAndFetch(t *testing.T) {
	r := newTestRepos(t)
	ctx := context.Background()

	u := domain.User{
		ID:           domain.UserID("u1"),
		Username:     "alice",
		DisplayName:  "Alice",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := r.Users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}

	got, err := r.Users.ByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != u.Username {
		t.Fatalf("username: got %q want %q", got.Username, u.Username)
	}

	got2, err := r.Users.ByUsername(ctx, u.Username)
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != u.ID {
		t.Fatalf("id mismatch")
	}

	if _, err := r.Users.ByID(ctx, domain.UserID("missing")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestConversationLifecycle(t *testing.T) {
	r := newTestRepos(t)
	ctx := context.Background()

	now := time.Now().UTC()

	u1 := domain.User{ID: "u1", Username: "alice", CreatedAt: now}
	u2 := domain.User{ID: "u2", Username: "bob", CreatedAt: now}
	if err := r.Users.Create(ctx, u1); err != nil {
		t.Fatal(err)
	}
	if err := r.Users.Create(ctx, u2); err != nil {
		t.Fatal(err)
	}

	conv := domain.Conversation{ID: "c1", Kind: domain.KindDirect, CreatedAt: now}
	if err := r.Conversations.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}

	if err := r.Conversations.AddParticipant(ctx, domain.Participant{
		ConversationID: conv.ID, UserID: u1.ID, JoinedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := r.Conversations.AddParticipant(ctx, domain.Participant{
		ConversationID: conv.ID, UserID: u2.ID, JoinedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	ok, err := r.Conversations.IsParticipant(ctx, conv.ID, u1.ID)
	if err != nil || !ok {
		t.Fatalf("u1 should be participant: %v %v", ok, err)
	}

	parts, err := r.Conversations.ListParticipants(ctx, conv.ID)
	if err != nil || len(parts) != 2 {
		t.Fatalf("participants: %v %d", err, len(parts))
	}

	list, err := r.Conversations.ListByUser(ctx, u1.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list by user: %v len=%d", err, len(list))
	}

	found, err := r.Conversations.FindDirect(ctx, u1.ID, u2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != conv.ID {
		t.Fatalf("FindDirect: %v", found.ID)
	}

	if err := r.Conversations.RemoveParticipant(ctx, conv.ID, u1.ID); err != nil {
		t.Fatal(err)
	}
	ok, _ = r.Conversations.IsParticipant(ctx, conv.ID, u1.ID)
	if ok {
		t.Fatal("u1 should be removed")
	}
}

func TestMessageOrderingAndPagination(t *testing.T) {
	r := newTestRepos(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := r.Users.Create(ctx, domain.User{ID: "u1", Username: "alice", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := r.Conversations.Create(ctx, domain.Conversation{ID: "c1", Kind: domain.KindDirect, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := r.Conversations.AddParticipant(ctx, domain.Participant{ConversationID: "c1", UserID: "u1", JoinedAt: now}); err != nil {
		t.Fatal(err)
	}

	// Insert 5 messages with strictly increasing timestamps.
	base := now
	for i := 0; i < 5; i++ {
		m := domain.Message{
			ID:             domain.MessageID(string('a' + rune(i))),
			ConversationID: "c1",
			SenderID:       "u1",
			Kind:           domain.KindText,
			Content:        "msg",
			CreatedAt:      base.Add(time.Duration(i) * time.Millisecond),
		}
		if err := r.Messages.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
	}

	// Default page: all 5, newest first.
	page, err := r.Messages.ListByConversation(ctx, "c1", time.Time{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 5 {
		t.Fatalf("want 5 messages, got %d", len(page))
	}
	if page[0].ID != "e" {
		t.Fatalf("first should be newest (e), got %s", page[0].ID)
	}
	if page[4].ID != "a" {
		t.Fatalf("last should be oldest (a), got %s", page[4].ID)
	}

	// Cursor pagination: from a given timestamp, exclude it and earlier.
	// Pick the third-newest's timestamp as the cursor; we expect 2 messages
	// strictly newer.
	cursor := page[2].CreatedAt
	older, err := r.Messages.ListByConversation(ctx, "c1", cursor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(older) != 2 {
		t.Fatalf("expected 2 newer, got %d", len(older))
	}
}

func TestSessionRepo(t *testing.T) {
	r := newTestRepos(t)
	ctx := context.Background()
	if err := r.Users.Create(ctx, domain.User{ID: "u1", Username: "alice", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	tok := domain.SessionToken("tok-1")
	if err := r.Sessions.Create(ctx, tok, "u1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	uid, exp, err := r.Sessions.UserForToken(ctx, tok)
	if err != nil || uid != "u1" || exp.Before(time.Now()) {
		t.Fatalf("got %v %v %v", uid, exp, err)
	}

	if err := r.Sessions.Delete(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Sessions.UserForToken(ctx, tok); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}

	// Expired sessions still resolve at the repository level — the auth
	// service is responsible for checking expiry with its own clock.
	expired := domain.SessionToken("tok-2")
	if err := r.Sessions.Create(ctx, expired, "u1", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	uid2, _, err := r.Sessions.UserForToken(ctx, expired)
	if err != nil || uid2 != "u1" {
		t.Fatalf("repo should not enforce expiry: %v %v", err, uid2)
	}
}
