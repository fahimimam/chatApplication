package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// MessageRepo persists messages with deterministic ordering.
type MessageRepo struct {
	db *sql.DB
}

// Create inserts a message.
func (r *MessageRepo) Create(ctx context.Context, m domain.Message) error {
	var editedAt, deletedAt *int64
	if m.EditedAt != nil {
		v := timeToUnix(*m.EditedAt)
		editedAt = &v
	}
	if m.DeletedAt != nil {
		v := timeToUnix(*m.DeletedAt)
		deletedAt = &v
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO messages(id, conversation_id, sender_id, kind, content, created_at, edited_at, deleted_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		string(m.ID), string(m.ConversationID), string(m.SenderID),
		string(m.Kind), m.Content, timeToUnix(m.CreatedAt), editedAt, deletedAt)
	return err
}

// ByID fetches a single message.
func (r *MessageRepo) ByID(ctx context.Context, id domain.MessageID) (domain.Message, error) {
	return r.queryOne(ctx,
		`SELECT id, conversation_id, sender_id, kind, content, created_at, edited_at, deleted_at
		 FROM messages WHERE id = ?`, string(id))
}

// ListByConversation returns messages newest-first, paginated by a cursor on
// created_at. Pass the zero time to read from the newest; the returned slice
// is in descending order so callers can render the latest page directly.
func (r *MessageRepo) ListByConversation(
	ctx context.Context,
	convID domain.ConversationID,
	before time.Time,
	limit int,
) ([]domain.Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var (
		rows *sql.Rows
		err  error
	)
	if before.IsZero() {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, conversation_id, sender_id, kind, content, created_at, edited_at, deleted_at
			 FROM messages WHERE conversation_id = ?
			 ORDER BY created_at DESC, id DESC
			 LIMIT ?`,
			string(convID), limit)
	} else {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id, conversation_id, sender_id, kind, content, created_at, edited_at, deleted_at
			 FROM messages WHERE conversation_id = ? AND created_at < ?
			 ORDER BY created_at DESC, id DESC
			 LIMIT ?`,
			string(convID), timeToUnix(before), limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *MessageRepo) queryOne(ctx context.Context, q string, args ...any) (domain.Message, error) {
	row := r.db.QueryRowContext(ctx, q, args...)
	m, err := scanMessage(row)
	if err != nil {
		return domain.Message{}, wrapNotFound(err)
	}
	return m, nil
}

// rowScanner is satisfied by *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanMessage(s rowScanner) (domain.Message, error) {
	var (
		m                 domain.Message
		createdMs         int64
		editedMs, delMs   sql.NullInt64
	)
	if err := s.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Kind,
		&m.Content, &createdMs, &editedMs, &delMs); err != nil {
		return domain.Message{}, err
	}
	m.CreatedAt = unixToTime(createdMs)
	if editedMs.Valid {
		v := unixToTime(editedMs.Int64)
		m.EditedAt = &v
	}
	if delMs.Valid {
		v := unixToTime(delMs.Int64)
		m.DeletedAt = &v
	}
	return m, nil
}