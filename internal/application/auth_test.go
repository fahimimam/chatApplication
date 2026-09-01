package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/repository/sqlite"
)

func newTestClock() (Clock, *time.Time) {
	t := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }, &t
}

func newAuthFixture(t *testing.T) (*AuthService, *sqlite.Repos, func()) {
	t.Helper()
	d, err := openInMemoryDB(t)
	if err != nil {
		t.Fatal(err)
	}
	repos := sqlite.New(d)
	clock, _ := newTestClock()
	svc := NewAuthService(repos.Users, repos.Sessions, clock)
	return svc, repos, func() { _ = d.Close() }
}

func TestAuthRegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	svc, _, closer := newAuthFixture(t)
	defer closer()

	u, err := svc.Register(ctx, "alice", "password123", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" {
		t.Fatal("id should be set")
	}

	tok, who, err := svc.Login(ctx, "alice", "password123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if who.ID != u.ID {
		t.Fatalf("login user mismatch")
	}
	if tok == "" {
		t.Fatal("token should be non-empty")
	}

	if _, _, err := svc.Login(ctx, "alice", "wrong"); err != domain.ErrInvalidCredential {
		t.Fatalf("want invalid credential, got %v", err)
	}
}

func TestAuthRegisterValidation(t *testing.T) {
	ctx := context.Background()
	svc, _, closer := newAuthFixture(t)
	defer closer()

	cases := []struct {
		name, user, pass string
	}{
		{"empty user", "", "password123"},
		{"short pass", "alice", "short"},
		{"space user", "al ice", "password123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Register(ctx, tc.user, tc.pass, "")
			if err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func TestAuthRegisterDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	svc, _, closer := newAuthFixture(t)
	defer closer()
	if _, err := svc.Register(ctx, "alice", "password123", ""); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Register(ctx, "alice", "password123", "")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists, got %v", err)
	}
}

func TestAuthResolveAndLogout(t *testing.T) {
	ctx := context.Background()
	svc, _, closer := newAuthFixture(t)
	defer closer()
	_, err := svc.Register(ctx, "alice", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	tok, who, err := svc.Login(ctx, "alice", "password123")
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := svc.Resolve(ctx, tok)
	if err != nil || resolved.ID != who.ID {
		t.Fatalf("resolve: %v %v", err, resolved)
	}

	if err := svc.Logout(ctx, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(ctx, tok); err != domain.ErrInvalidCredential {
		t.Fatalf("expected invalid after logout, got %v", err)
	}
}