package application

import (
	"context"
	"database/sql"
	"testing"

	"github.com/fahimimam/chatApplication/internal/infra/db"
)

// openInMemoryDB returns an in-memory SQLite database with migrations applied.
// The caller is responsible for closing it.
func openInMemoryDB(t *testing.T) (*sql.DB, error) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(context.Background(), d); err != nil {
		_ = d.Close()
		return nil, err
	}
	t.Cleanup(func() { _ = d.Close() })
	return d, nil
}