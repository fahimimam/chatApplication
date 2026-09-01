// Package db opens SQLite databases used by the repository layer. We use the
// pure-Go modernc.org/sqlite driver so the project builds without cgo.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Open opens a SQLite database at path and applies sane connection settings.
// For in-memory test databases, pass ":memory:" — note that to share data
// across goroutines in `:memory:` mode the caller should pass a named
// in-memory DSN via OpenShared or use a temp file instead.
func Open(path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	// _busy_timeout=5s, _foreign_keys=on. WAL only when persistent —
	// modernc SQLite does not honour it for ":memory:".
	var dsn string
	if path == ":memory:" {
		// Use a unique DSN per :memory: Open so each test gets its own
		// private database; modernc's cache=shared is unreliable across
		// connections in the same process.
		dsn = fmt.Sprintf("file:%s:?mode=memory&cache=private&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	} else {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// Use a single connection so all reads see the writes of every goroutine.
	// SQLite is single-writer; multiple connections would require shared
	// cache which is unreliable across drivers.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	return db, nil
}

// Ping waits until the database is reachable or the context expires.
func Ping(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return db.PingContext(ctx)
		}
		time.Sleep(50 * time.Millisecond)
	}
}