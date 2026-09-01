// Package sqlite implements the repository interfaces from
// internal/repository using SQLite (modernc.org/sqlite, pure Go).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/repository"
)

// Repos bundles the per-table repositories so callers can construct them once
// and share the *sql.DB.
type Repos struct {
	DB           *sql.DB
	Users        *UserRepo
	Conversations *ConversationRepo
	Messages     *MessageRepo
	Sessions     *SessionRepo
}

// New constructs all repositories backed by db. The database must have
// Migrate applied first.
func New(db *sql.DB) *Repos {
	return &Repos{
		DB:            db,
		Users:         &UserRepo{db: db},
		Conversations: &ConversationRepo{db: db},
		Messages:      &MessageRepo{db: db},
		Sessions:      &SessionRepo{db: db},
	}
}

var _ repository.UserRepository = (*UserRepo)(nil)
var _ repository.ConversationRepository = (*ConversationRepo)(nil)
var _ repository.MessageRepository = (*MessageRepo)(nil)
var _ repository.SessionRepository = (*SessionRepo)(nil)

// helper to translate sql.ErrNoRows into domain.ErrNotFound without leaking
// the driver error to upper layers.
func wrapNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// timeToUnix stores time as integer unix-millis so columns can be indexed and
// compared deterministically. Micros would also work; millis keeps things
// human-readable in the SQLite shell.
func timeToUnix(t time.Time) int64 { return t.UnixMilli() }
func unixToTime(ms int64) time.Time {
	return time.UnixMilli(ms).UTC()
}

// execer is satisfied by both *sql.DB and *sql.Tx. It lets repository methods
// participate in transactions when callers wrap them.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// inTx runs fn inside a transaction with the given options. Used by services
// that need multi-statement atomicity.
func inTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	return fn(tx)
}