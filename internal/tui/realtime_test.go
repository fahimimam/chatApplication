package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// TestRealtimePushAppendsToOpenChat is the regression for the user's
// bug: a realtime EVT arriving on the wire MUST be parsed and appended
// to m.messages, otherwise the other party's messages stay invisible
// until a manual refresh triggers a full history reload.
//
// The earlier implementation stripped the "msg " prefix before calling
// ParseMessage, but ParseMessage expects the full "msg ..." line.
// Stripping caused every realtime push to silently fail parsing and
// fall through as a no-op.
func TestRealtimePushAppendsToOpenChat(t *testing.T) {
	m := NewModel()
	m.user = domain.User{ID: "u1", Username: "alice"}
	m.current = &ConvSummary{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"}
	m.messages = []MessageRow{
		{ID: "1", Sender: "u2", SenderName: "bob", ConversationID: "conv-1",
			Content: "first", CreatedAt: time.Unix(1735732800, 0).UTC()},
	}

	evt := `msg id=new sender=u2 sender_name=bob conv=conv-1 ts=1735732860 content="hi alice"`
	nm, _ := m.Update(serverEventMsg{line: evt})
	m = nm.(*Model)
	if len(m.messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(m.messages))
	}
	if m.messages[1].ID != "new" {
		t.Fatalf("realtime message not appended at bottom: %+v", m.messages)
	}
}

// TestRealtimePushForOtherConversationNotifies confirms that an EVT for
// a conversation the user isn't viewing lands in m.notifs (so the user
// at least knows about it) instead of silently disappearing.
func TestRealtimePushForOtherConversationNotifies(t *testing.T) {
	m := NewModel()
	m.user = domain.User{ID: "u1", Username: "alice"}
	m.current = &ConvSummary{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"}

	evt := `msg id=new sender=u2 sender_name=bob conv=conv-OTHER ts=1735732860 content="hi"`
	nm, _ := m.Update(serverEventMsg{line: evt})
	m = nm.(*Model)
	if len(m.notifs) == 0 {
		t.Fatalf("expected a notification, got none; status=%q", m.status)
	}
}

// TestRealtimePushMalformedLineIgnored makes sure a bogus event doesn't
// panic the model.
func TestRealtimePushMalformedLineIgnored(t *testing.T) {
	m := NewModel()
	m.user = domain.User{ID: "u1", Username: "alice"}
	m.current = &ConvSummary{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"}

	// noinspection ALL
	evt := `msg this is not a valid line`
	nm, cmd := m.Update(serverEventMsg{line: evt})
	m = nm.(*Model)
	if len(m.messages) != 0 {
		t.Fatalf("malformed event must not be appended; got %d messages", len(m.messages))
	}
	if cmd == nil {
		// accepted: handler returns a loadConvsCmd on success path; on
		// failure we may return nil. Either is fine.
		_ = cmd
	}
}

// TestMessageOrderIsChronologicalOldestFirst protects the second
// regression: when a chat is opened, m.messages must be in chronological
// order (oldest at index 0, newest at the end). The earlier code reversed
// the server's already-oldest-first response, putting the newest message
// at the TOP of the chat (latest-on-first) and confusing the user.
func TestMessageOrderIsChronologicalOldestFirst(t *testing.T) {
	m := NewModel()
	m.user = domain.User{ID: "u1", Username: "alice"}
	m.current = &ConvSummary{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"}

	// Simulate a messagesLoadedMsg from loadHistoryCmd after the fix:
	// server returns oldest-first, client stores as-is.
	nm, _ := m.Update(messagesLoadedMsg{
		conv: "conv-1",
		msgs: []MessageRow{
			{ID: "a", Sender: "u2", SenderName: "bob", ConversationID: "conv-1",
				Content: "first", CreatedAt: time.Unix(100, 0).UTC()},
			{ID: "b", Sender: "u1", SenderName: "alice", ConversationID: "conv-1",
				Content: "second", CreatedAt: time.Unix(200, 0).UTC()},
			{ID: "c", Sender: "u2", SenderName: "bob", ConversationID: "conv-1",
				Content: "third (newest)", CreatedAt: time.Unix(300, 0).UTC()},
		},
	})
	m = nm.(*Model)

	if len(m.messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(m.messages))
	}
	if m.messages[0].ID != "a" {
		t.Errorf("oldest message should be at index 0, got %s", m.messages[0].ID)
	}
	if m.messages[len(m.messages)-1].ID != "c" {
		t.Errorf("newest message should be at the bottom, got %s", m.messages[len(m.messages)-1].ID)
	}
}

// silence unused import in case tea isn't referenced elsewhere.
var _ tea.KeyMsg
