// Package application holds the use-case services that orchestrate the
// domain and the repository layer. Services expose domain-typed inputs and
// outputs and never mention transport-specific concepts (no HTTP, no TCP,
// no TUI types).
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Clock lets services obtain the current time. Tests override it to make
// session expiration deterministic.
type Clock func() time.Time

func realClock() time.Time { return time.Now().UTC() }

// SessionTTL controls how long an issued session token is valid.
const SessionTTL = 24 * time.Hour

// AuthService handles registration, login, and session resolution.
type AuthService struct {
	users    repository.UserRepository
	sessions repository.SessionRepository
	clock    Clock
}

// NewAuthService wires the dependencies. The clock may be nil; the service
// falls back to time.Now.
func NewAuthService(users repository.UserRepository, sessions repository.SessionRepository, clock Clock) *AuthService {
	if clock == nil {
		clock = realClock
	}
	return &AuthService{users: users, sessions: sessions, clock: clock}
}

// Register creates a new user with a bcrypt-hashed password.
func (s *AuthService) Register(ctx context.Context, username, password, displayName string) (domain.User, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)

	if len(password) < 8 {
		return domain.User{}, &domain.ValidationError{Field: "password", Message: "must be at least 8 characters"}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, fmt.Errorf("hash password: %w", err)
	}

	u := domain.User{
		ID:           domain.UserID(uuid.NewString()),
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		CreatedAt:    s.clock(),
	}
	if err := u.Validate(); err != nil {
		return domain.User{}, err
	}

	if err := s.users.Create(ctx, u); err != nil {
		// Surface uniqueness conflicts as a friendlier error; let other
		// errors bubble unchanged.
		if isUniqueViolation(err) {
			return domain.User{}, fmt.Errorf("%w: username %q", domain.ErrAlreadyExists, username)
		}
		return domain.User{}, err
	}
	return u, nil
}

// Login validates credentials and issues a session token.
func (s *AuthService) Login(ctx context.Context, username, password string) (domain.SessionToken, domain.User, error) {
	u, err := s.users.ByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", domain.User{}, domain.ErrInvalidCredential
		}
		return "", domain.User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", domain.User{}, domain.ErrInvalidCredential
	}

	tok, err := newToken()
	if err != nil {
		return "", domain.User{}, err
	}
	if err := s.sessions.Create(ctx, tok, u.ID, s.clock().Add(SessionTTL)); err != nil {
		return "", domain.User{}, fmt.Errorf("create session: %w", err)
	}
	return tok, u, nil
}

// Resolve validates a session token and returns the owning user. Expired
// tokens are rejected as invalid credentials.
func (s *AuthService) Resolve(ctx context.Context, token domain.SessionToken) (domain.User, error) {
	uid, exp, err := s.sessions.UserForToken(ctx, token)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, domain.ErrInvalidCredential
		}
		return domain.User{}, err
	}
	if !exp.IsZero() && exp.Before(s.clock()) {
		return domain.User{}, domain.ErrInvalidCredential
	}
	return s.users.ByID(ctx, uid)
}

// Logout deletes a session token.
func (s *AuthService) Logout(ctx context.Context, token domain.SessionToken) error {
	return s.sessions.Delete(ctx, token)
}

func newToken() (domain.SessionToken, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return domain.SessionToken(hex.EncodeToString(b[:])), nil
}

// isUniqueViolation returns true for SQLite UNIQUE constraint errors. The
// modernc driver returns a generic error string we have to inspect; future
// versions may expose a typed error we can match instead.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}