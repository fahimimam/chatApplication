package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Schema is the SQL DDL that bootstraps an empty database. Migrations are
// append-only; the current schema includes everything the application needs.
//
// Index choices:
//   * messages(conv_id, created_at, id) — primary chat list query.
//   * participants(user_id, conv_id) — "my conversations" lookup.
//   * users(username) — unique login lookup.
//   * sessions(token) — token lookup.
const Schema = `
CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY,
    kind        TEXT NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS participants (
    conversation_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    joined_at       INTEGER NOT NULL,
    PRIMARY KEY (conversation_id, user_id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id)         REFERENCES users(id)          ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_participants_user
    ON participants(user_id, conversation_id);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_id       TEXT NOT NULL,
    kind            TEXT NOT NULL,
    content         TEXT NOT NULL,
    created_at      INTEGER NOT NULL,
    edited_at       INTEGER,
    deleted_at      INTEGER,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY (sender_id)       REFERENCES users(id)          ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_conv_time
    ON messages(conversation_id, created_at, id);

CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    expires_at  INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
`

// Migrate applies the current schema. It is idempotent and safe to call on
// every boot.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}