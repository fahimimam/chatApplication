// Package repository defines the persistence interfaces used by the
// application layer. Concrete implementations live in subpackages
// (e.g. internal/repository/sqlite).
package repository

import (
	"context"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// UserRepository persists users.
type UserRepository interface {
	Create(ctx context.Context, u domain.User) error
	ByID(ctx context.Context, id domain.UserID) (domain.User, error)
	ByUsername(ctx context.Context, username string) (domain.User, error)
	// ListAll returns every user known to the system, sorted by username.
	// Used by the directory command so clients can pick a username to chat
	// with without having to remember one.
	ListAll(ctx context.Context) ([]domain.User, error)
}

// ConversationRepository persists conversations and their membership.
type ConversationRepository interface {
	Create(ctx context.Context, c domain.Conversation) error
	ByID(ctx context.Context, id domain.ConversationID) (domain.Conversation, error)

	// ListByUser returns conversations the user participates in, most recent
	// first.
	ListByUser(ctx context.Context, userID domain.UserID) ([]domain.Conversation, error)

	// FindDirect returns the existing direct conversation between two users,
	// or domain.ErrNotFound if none exists.
	FindDirect(ctx context.Context, a, b domain.UserID) (domain.Conversation, error)

	AddParticipant(ctx context.Context, p domain.Participant) error
	RemoveParticipant(ctx context.Context, convID domain.ConversationID, userID domain.UserID) error
	IsParticipant(ctx context.Context, convID domain.ConversationID, userID domain.UserID) (bool, error)
	ListParticipants(ctx context.Context, convID domain.ConversationID) ([]domain.Participant, error)
}

// MessageRepository persists messages and orders them deterministically.
type MessageRepository interface {
	Create(ctx context.Context, m domain.Message) error
	ByID(ctx context.Context, id domain.MessageID) (domain.Message, error)

	// ListByConversation returns messages ordered by (CreatedAt, ID). The
	// `before` cursor is exclusive; pass the zero time to read from the
	// newest. `limit` is the page size.
	ListByConversation(
		ctx context.Context,
		convID domain.ConversationID,
		before time.Time,
		limit int,
	) ([]domain.Message, error)
}

// SessionRepository persists opaque session tokens mapped to user IDs. The
// concrete storage is implementation-defined; the interface only needs the
// minimum needed by the application layer.
type SessionRepository interface {
	Create(ctx context.Context, token domain.SessionToken, userID domain.UserID, expiresAt time.Time) error
	UserForToken(ctx context.Context, token domain.SessionToken) (domain.UserID, time.Time, error)
	Delete(ctx context.Context, token domain.SessionToken) error
}