package sqlite

import (
	"context"
	"database/sql"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// UserRepo persists users.
type UserRepo struct {
	db *sql.DB
}

// Create inserts a user.
func (r *UserRepo) Create(ctx context.Context, u domain.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users(id, username, display_name, password_hash, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		string(u.ID), u.Username, u.DisplayName, u.PasswordHash, timeToUnix(u.CreatedAt))
	return err
}

// ByID looks up a user by id.
func (r *UserRepo) ByID(ctx context.Context, id domain.UserID) (domain.User, error) {
	return r.queryOne(ctx,
		`SELECT id, username, display_name, password_hash, created_at FROM users WHERE id = ?`,
		string(id))
}

// ByUsername looks up a user by username.
func (r *UserRepo) ByUsername(ctx context.Context, username string) (domain.User, error) {
	return r.queryOne(ctx,
		`SELECT id, username, display_name, password_hash, created_at FROM users WHERE username = ?`,
		username)
}

func (r *UserRepo) queryOne(ctx context.Context, q string, args ...any) (domain.User, error) {
	var (
		u         domain.User
		createdMs int64
	)
	err := r.db.QueryRowContext(ctx, q, args...).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.PasswordHash, &createdMs,
	)
	if err != nil {
		return domain.User{}, wrapNotFound(err)
	}
	u.CreatedAt = unixToTime(createdMs)
	return u, nil
}