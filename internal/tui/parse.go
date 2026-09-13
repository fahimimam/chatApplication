// Package tui implements the terminal chat client. The TUI talks to the
// chatd server through the internal/transport/tcp.Client and never to
// repositories directly. Wire-format parsing happens here so the Client
// remains transport-shaped.
package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// ConvSummary is a row in the conversation list.
type ConvSummary struct {
	ID        domain.ConversationID
	Kind      domain.ConversationKind
	Title     string
	CreatedAt time.Time
}

// MessageRow is one history entry, ready to render.
type MessageRow struct {
	ID             domain.MessageID
	ConversationID domain.ConversationID
	Sender         domain.UserID
	SenderName     string
	Content        string
	CreatedAt      time.Time
}

// ParseConv parses "conv id=<uuid> kind=<direct|group> title=<text>".
func ParseConv(line string) (ConvSummary, error) {
	if !strings.HasPrefix(line, "conv ") {
		return ConvSummary{}, fmt.Errorf("not a conv line: %q", line)
	}
	var c ConvSummary
	for _, f := range fieldsAfter(line, "conv ") {
		switch {
		case strings.HasPrefix(f, "id="):
			c.ID = domain.ConversationID(strings.TrimPrefix(f, "id="))
		case strings.HasPrefix(f, "kind="):
			c.Kind = domain.ConversationKind(strings.TrimPrefix(f, "kind="))
		case strings.HasPrefix(f, "title="):
			c.Title = strings.TrimPrefix(f, "title=")
		}
	}
	if c.ID == "" {
		return ConvSummary{}, fmt.Errorf("conv line missing id: %q", line)
	}
	return c, nil
}

// ParseMessage parses "msg id=<uuid> sender=<uuid> sender_name=<u> ts=<ms> content=<quoted>".
// The fields may appear in any order and trailing fields (e.g. "conv=")
// may be present. content= is the trailing quoted string.
func ParseMessage(line string) (MessageRow, error) {
	if !strings.HasPrefix(line, "msg ") {
		return MessageRow{}, fmt.Errorf("not a msg line: %q", line)
	}
	rest := strings.TrimPrefix(line, "msg ")
	// content= may contain spaces; it is the trailing quoted string.
	parts := strings.SplitN(rest, " content=", 2)
	if len(parts) != 2 {
		return MessageRow{}, fmt.Errorf("msg line missing content: %q", line)
	}
	head := strings.Fields(parts[0])
	if len(head) < 3 {
		return MessageRow{}, fmt.Errorf("msg line malformed head: %q", line)
	}
	content := strings.Trim(parts[1], `"`)
	mr := MessageRow{
		Content: content,
	}
	for _, h := range head {
		switch {
		case strings.HasPrefix(h, "id="):
			mr.ID = domain.MessageID(strings.TrimPrefix(h, "id="))
		case strings.HasPrefix(h, "sender="):
			mr.Sender = domain.UserID(strings.TrimPrefix(h, "sender="))
		case strings.HasPrefix(h, "sender_name="):
			mr.SenderName = strings.TrimPrefix(h, "sender_name=")
		case strings.HasPrefix(h, "conv="):
			mr.ConversationID = domain.ConversationID(strings.TrimPrefix(h, "conv="))
		case strings.HasPrefix(h, "ts="):
			ts, err := strconv.ParseInt(strings.TrimPrefix(h, "ts="), 10, 64)
			if err != nil {
				return MessageRow{}, fmt.Errorf("msg line bad ts: %w", err)
			}
			mr.CreatedAt = time.UnixMilli(ts).UTC()
		}
	}
	if mr.ID == "" || mr.CreatedAt.IsZero() {
		return MessageRow{}, fmt.Errorf("msg line missing required fields: %q", line)
	}
	return mr, nil
}

// IsEndLine returns true if the line is the terminal marker for a multi-
// line reply. The marker is "end (N)" for history and "(N)" for list.
func IsEndLine(line string) bool {
	return strings.HasPrefix(line, "end (") || strings.HasPrefix(line, "(")
}

// ExtractCount returns the integer inside "end (N)" or "(N)".
func ExtractCount(line string) int {
	s := line
	s = strings.TrimPrefix(s, "end (")
	s = strings.TrimSuffix(s, ")")
	s = strings.TrimPrefix(s, "(")
	n, _ := strconv.Atoi(strings.TrimSuffix(s, ")"))
	return n
}

func fieldsAfter(s, prefix string) []string {
	if prefix != "" {
		s = strings.TrimPrefix(s, prefix)
	}
	return strings.Fields(s)
}
