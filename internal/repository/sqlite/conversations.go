package sqlite

import (
	"context"
	"database/sql"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// ConversationRepo persists conversations and their participants.
type ConversationRepo struct {
	db *sql.DB
}

// Create inserts a conversation.
func (r *ConversationRepo) Create(ctx context.Context, c domain.Conversation) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO conversations(id, kind, title, created_at) VALUES (?, ?, ?, ?)`,
		string(c.ID), string(c.Kind), c.Title, timeToUnix(c.CreatedAt))
	return err
}

// ByID returns a conversation.
func (r *ConversationRepo) ByID(ctx context.Context, id domain.ConversationID) (domain.Conversation, error) {
	var (
		c         domain.Conversation
		createdMs int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, kind, title, created_at FROM conversations WHERE id = ?`,
		string(id)).Scan(&c.ID, &c.Kind, &c.Title, &createdMs)
	if err != nil {
		return domain.Conversation{}, wrapNotFound(err)
	}
	c.CreatedAt = unixToTime(createdMs)
	return c, nil
}

// ListByUser returns conversations the user participates in, most recent first.
func (r *ConversationRepo) ListByUser(ctx context.Context, userID domain.UserID) ([]domain.Conversation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.kind, c.title, c.created_at
		FROM conversations c
		JOIN participants p ON p.conversation_id = c.id
		WHERE p.user_id = ?
		ORDER BY c.created_at DESC, c.id DESC`,
		string(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Conversation
	for rows.Next() {
		var (
			c         domain.Conversation
			createdMs int64
		)
		if err := rows.Scan(&c.ID, &c.Kind, &c.Title, &createdMs); err != nil {
			return nil, err
		}
		c.CreatedAt = unixToTime(createdMs)
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindDirect returns the existing 1:1 conversation between two users, if any.
func (r *ConversationRepo) FindDirect(ctx context.Context, a, b domain.UserID) (domain.Conversation, error) {
	var (
		c         domain.Conversation
		createdMs int64
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT c.id, c.kind, c.title, c.created_at
		FROM conversations c
		JOIN participants p1 ON p1.conversation_id = c.id AND p1.user_id = ?
		JOIN participants p2 ON p2.conversation_id = c.id AND p2.user_id = ?
		WHERE c.kind = ?
		LIMIT 1`,
		string(a), string(b), string(domain.KindDirect)).Scan(&c.ID, &c.Kind, &c.Title, &createdMs)
	if err != nil {
		return domain.Conversation{}, wrapNotFound(err)
	}
	c.CreatedAt = unixToTime(createdMs)
	return c, nil
}

// AddParticipant inserts a participant row.
func (r *ConversationRepo) AddParticipant(ctx context.Context, p domain.Participant) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO participants(conversation_id, user_id, joined_at)
		 VALUES (?, ?, ?)`,
		string(p.ConversationID), string(p.UserID), timeToUnix(p.JoinedAt))
	return err
}

// RemoveParticipant deletes a participant row.
func (r *ConversationRepo) RemoveParticipant(ctx context.Context, convID domain.ConversationID, userID domain.UserID) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM participants WHERE conversation_id = ? AND user_id = ?`,
		string(convID), string(userID))
	return err
}

// IsParticipant returns true if userID is in convID.
func (r *ConversationRepo) IsParticipant(ctx context.Context, convID domain.ConversationID, userID domain.UserID) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM participants WHERE conversation_id = ? AND user_id = ?`,
		string(convID), string(userID)).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ListParticipants returns the participants of a conversation.
func (r *ConversationRepo) ListParticipants(ctx context.Context, convID domain.ConversationID) ([]domain.Participant, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT conversation_id, user_id, joined_at FROM participants WHERE conversation_id = ?`,
		string(convID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Participant
	for rows.Next() {
		var (
			p        domain.Participant
			joinedMs int64
		)
		if err := rows.Scan(&p.ConversationID, &p.UserID, &joinedMs); err != nil {
			return nil, err
		}
		p.JoinedAt = unixToTime(joinedMs)
		out = append(out, p)
	}
	return out, rows.Err()
}