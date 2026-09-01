package domain

import (
	"strings"
	"time"
)

// ConversationID uniquely identifies a conversation.
type ConversationID string

func (id ConversationID) String() string { return string(id) }

// ConversationKind enumerates the kinds of conversations the system supports.
// Today only Direct and Group are used; Channel is reserved for future use.
type ConversationKind string

const (
	KindDirect ConversationKind = "direct"
	KindGroup  ConversationKind = "group"
)

// Conversation is a thread of messages between two or more participants.
type Conversation struct {
	ID        ConversationID
	Kind      ConversationKind
	Title     string
	CreatedAt time.Time
}

// Validate enforces invariants.
func (c *Conversation) Validate() error {
	switch c.Kind {
	case KindDirect, KindGroup:
	default:
		return &ValidationError{Field: "kind", Message: "must be direct or group"}
	}
	if c.Kind == KindGroup && strings.TrimSpace(c.Title) == "" {
		return &ValidationError{Field: "title", Message: "group conversations need a title"}
	}
	return nil
}

// Participant links a user to a conversation and tracks when they joined.
type Participant struct {
	ConversationID ConversationID
	UserID         UserID
	JoinedAt       time.Time
}
