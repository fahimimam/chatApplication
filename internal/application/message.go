package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/repository"
	"github.com/google/uuid"
)

// MessageService handles message creation, retrieval, and authorization.
// The transport layer talks to this service — never directly to the
// repository — so a future WebSocket transport can reuse it verbatim.
type MessageService struct {
	msgs     repository.MessageRepository
	convs    repository.ConversationRepository
	users    repository.UserRepository
	clock    Clock
}

// NewMessageService wires the dependencies.
func NewMessageService(
	msgs repository.MessageRepository,
	convs repository.ConversationRepository,
	users repository.UserRepository,
	clock Clock,
) *MessageService {
	if clock == nil {
		clock = realClock
	}
	return &MessageService{msgs: msgs, convs: convs, users: users, clock: clock}
}

// SendInput is the input for Send.
type SendInput struct {
	ConversationID domain.ConversationID
	SenderID       domain.UserID
	Content        string
}

// Send validates input, ensures the sender is a participant, and persists
// the message.
func (s *MessageService) Send(ctx context.Context, in SendInput) (domain.Message, error) {
	if in.ConversationID == "" {
		return domain.Message{}, &domain.ValidationError{Field: "conversation_id", Message: "required"}
	}
	if in.SenderID == "" {
		return domain.Message{}, &domain.ValidationError{Field: "sender_id", Message: "required"}
	}
	m := domain.Message{
		ID:             domain.MessageID(uuid.NewString()),
		ConversationID: in.ConversationID,
		SenderID:       in.SenderID,
		Kind:           domain.KindText,
		Content:        in.Content,
		CreatedAt:      s.clock(),
	}
	if err := m.Validate(); err != nil {
		return domain.Message{}, err
	}

	ok, err := s.convs.IsParticipant(ctx, in.ConversationID, in.SenderID)
	if err != nil {
		return domain.Message{}, err
	}
	if !ok {
		return domain.Message{}, fmt.Errorf("%w: cannot send to %s", domain.ErrForbidden, in.ConversationID)
	}

	if err := s.msgs.Create(ctx, m); err != nil {
		return domain.Message{}, err
	}
	return m, nil
}

// HistoryInput controls message history retrieval.
type HistoryInput struct {
	ConversationID domain.ConversationID
	CallerID       domain.UserID
	Before         time.Time // zero = start from newest
	Limit          int       // 0 -> default 100
}

// History returns a page of messages newest-first.
func (s *MessageService) History(ctx context.Context, in HistoryInput) ([]domain.Message, error) {
	if in.ConversationID == "" {
		return nil, &domain.ValidationError{Field: "conversation_id", Message: "required"}
	}
	ok, err := s.convs.IsParticipant(ctx, in.ConversationID, in.CallerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: cannot read %s", domain.ErrForbidden, in.ConversationID)
	}
	return s.msgs.ListByConversation(ctx, in.ConversationID, in.Before, in.Limit)
}

// ByID fetches a single message by id, enforcing membership.
func (s *MessageService) ByID(ctx context.Context, callerID domain.UserID, id domain.MessageID) (domain.Message, error) {
	m, err := s.msgs.ByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Message{}, err
		}
		return domain.Message{}, err
	}
	ok, err := s.convs.IsParticipant(ctx, m.ConversationID, callerID)
	if err != nil {
		return domain.Message{}, err
	}
	if !ok {
		return domain.Message{}, fmt.Errorf("%w: cannot read message", domain.ErrForbidden)
	}
	return m, nil
}

// IsConversationMember is a convenience helper for transports that want to
// perform additional checks without re-fetching the conversation.
func (s *MessageService) IsConversationMember(ctx context.Context, callerID domain.UserID, convID domain.ConversationID) (bool, error) {
	return s.convs.IsParticipant(ctx, convID, callerID)
}