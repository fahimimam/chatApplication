package domain

import (
	"strings"
	"time"
)

// MessageID uniquely identifies a message.
type MessageID string

func (id MessageID) String() string { return string(id) }

// MessageKind represents the kind of payload a message carries.
type MessageKind string

const (
	KindText MessageKind = "text"
)

// Message is a single entry in a conversation. Ordering is by
// (CreatedAt, ID) — repositories MUST return messages ordered this way.
type Message struct {
	ID             MessageID
	ConversationID ConversationID
	SenderID       UserID
	Kind           MessageKind
	Content        string
	CreatedAt      time.Time
	EditedAt       *time.Time
	DeletedAt      *time.Time
}

// MaxContentLength caps a single message body. The value is generous enough
// for chat-style content but stops abuse.
const MaxContentLength = 8192

// Validate enforces invariants on a Message before persistence.
func (m *Message) Validate() error {
	if m.ConversationID == "" {
		return &ValidationError{Field: "conversation_id", Message: "required"}
	}
	if m.SenderID == "" {
		return &ValidationError{Field: "sender_id", Message: "required"}
	}
	if m.Kind != KindText {
		return &ValidationError{Field: "kind", Message: "unsupported"}
	}
	if strings.TrimSpace(m.Content) == "" {
		return &ValidationError{Field: "content", Message: "must not be empty"}
	}
	if len(m.Content) > MaxContentLength {
		return &ValidationError{Field: "content", Message: "too long"}
	}
	return nil
}

// Edited reports whether the message has been edited.
func (m *Message) Edited() bool { return m.EditedAt != nil }

// Deleted reports whether the message has been soft-deleted.
func (m *Message) Deleted() bool { return m.DeletedAt != nil }
