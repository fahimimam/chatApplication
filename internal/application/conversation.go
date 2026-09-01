package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/fahimimam/chatApplication/internal/domain"
	"github.com/fahimimam/chatApplication/internal/repository"
	"github.com/google/uuid"
)

// ConversationService orchestrates conversation lifecycle: create/list/find
// direct conversations, manage participants, and authorize reads.
type ConversationService struct {
	convs  repository.ConversationRepository
	users  repository.UserRepository
	clock  Clock
}

// NewConversationService wires the dependencies.
func NewConversationService(convs repository.ConversationRepository, users repository.UserRepository, clock Clock) *ConversationService {
	if clock == nil {
		clock = realClock
	}
	return &ConversationService{convs: convs, users: users, clock: clock}
}

// CreateDirect opens (or returns) the existing 1:1 conversation between two
// users. Authorization: the caller must be one of the participants.
func (s *ConversationService) CreateDirect(ctx context.Context, callerID, otherID domain.UserID) (domain.Conversation, error) {
	if callerID == "" || otherID == "" {
		return domain.Conversation{}, &domain.ValidationError{Field: "user_id", Message: "required"}
	}
	if callerID == otherID {
		return domain.Conversation{}, &domain.ValidationError{Field: "user_id", Message: "cannot start a chat with yourself"}
	}
	if _, err := s.users.ByID(ctx, otherID); err != nil {
		return domain.Conversation{}, err
	}

	if existing, err := s.convs.FindDirect(ctx, callerID, otherID); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return domain.Conversation{}, err
	}

	conv := domain.Conversation{
		ID:        domain.ConversationID(uuid.NewString()),
		Kind:      domain.KindDirect,
		CreatedAt: s.clock(),
	}
	if err := s.convs.Create(ctx, conv); err != nil {
		return domain.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	now := s.clock()
	if err := s.convs.AddParticipant(ctx, domain.Participant{ConversationID: conv.ID, UserID: callerID, JoinedAt: now}); err != nil {
		return domain.Conversation{}, err
	}
	if err := s.convs.AddParticipant(ctx, domain.Participant{ConversationID: conv.ID, UserID: otherID, JoinedAt: now}); err != nil {
		return domain.Conversation{}, err
	}
	return conv, nil
}

// CreateGroup creates a new group conversation with the caller as the first
// participant.
func (s *ConversationService) CreateGroup(ctx context.Context, callerID domain.UserID, title string, members ...domain.UserID) (domain.Conversation, error) {
	if callerID == "" {
		return domain.Conversation{}, &domain.ValidationError{Field: "caller", Message: "required"}
	}
	if title == "" {
		return domain.Conversation{}, &domain.ValidationError{Field: "title", Message: "required"}
	}
	if len(members) == 0 {
		return domain.Conversation{}, &domain.ValidationError{Field: "members", Message: "at least one other participant required"}
	}

	conv := domain.Conversation{
		ID:        domain.ConversationID(uuid.NewString()),
		Kind:      domain.KindGroup,
		Title:     title,
		CreatedAt: s.clock(),
	}
	if err := s.convs.Create(ctx, conv); err != nil {
		return domain.Conversation{}, err
	}

	joined := map[domain.UserID]bool{callerID: true}
	add := func(uid domain.UserID) error {
		if joined[uid] {
			return nil
		}
		if _, err := s.users.ByID(ctx, uid); err != nil {
			return err
		}
		joined[uid] = true
		return s.convs.AddParticipant(ctx, domain.Participant{
			ConversationID: conv.ID,
			UserID:         uid,
			JoinedAt:       s.clock(),
		})
	}
	if err := add(callerID); err != nil {
		return domain.Conversation{}, err
	}
	for _, m := range members {
		if err := add(m); err != nil {
			return domain.Conversation{}, err
		}
	}
	return conv, nil
}

// ListForUser returns conversations the caller participates in.
func (s *ConversationService) ListForUser(ctx context.Context, callerID domain.UserID) ([]domain.Conversation, error) {
	if callerID == "" {
		return nil, &domain.ValidationError{Field: "caller", Message: "required"}
	}
	return s.convs.ListByUser(ctx, callerID)
}

// EnsureParticipant checks that caller is a member of convID. Used by other
// services before reading/writing messages.
func (s *ConversationService) EnsureParticipant(ctx context.Context, callerID domain.UserID, convID domain.ConversationID) error {
	ok, err := s.convs.IsParticipant(ctx, convID, callerID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: not a participant of %s", domain.ErrForbidden, convID)
	}
	return nil
}

// Participants returns the participants of a conversation. Authorization is
// enforced via EnsureParticipant.
func (s *ConversationService) Participants(ctx context.Context, callerID domain.UserID, convID domain.ConversationID) ([]domain.Participant, error) {
	if err := s.EnsureParticipant(ctx, callerID, convID); err != nil {
		return nil, err
	}
	return s.convs.ListParticipants(ctx, convID)
}