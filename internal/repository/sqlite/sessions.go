package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// SessionRepo persists opaque session tokens.
type SessionRepo struct {
	db *sql.DB
}

// Create inserts a session row.
func (r *SessionRepo) Create(ctx context.Context, token domain.SessionToken, userID domain.UserID, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions(token, user_id, expires_at) VALUES (?, ?, ?)`,
		string(token), string(userID), timeToUnix(expiresAt))
	return err
}

// UserForToken resolves a token to a (user, expiry) pair or returns
// domain.ErrNotFound when the token does not exist. Expiration is NOT
// checked here — callers (the auth service) decide policy using their own
// clock so test code can use a frozen clock.
func (r *SessionRepo) UserForToken(ctx context.Context, token domain.SessionToken) (domain.UserID, time.Time, error) {
	var (
		uid       string
		expiresMs int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token = ?`,
		string(token)).Scan(&uid, &expiresMs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", time.Time{}, domain.ErrNotFound
		}
		return "", time.Time{}, err
	}
	return domain.UserID(uid), unixToTime(expiresMs), nil
}

// Delete removes a session.
func (r *SessionRepo) Delete(ctx context.Context, token domain.SessionToken) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, string(token))
	return err
}