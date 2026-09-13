// Package application: presence.go tracks which users currently have at
// least one live session on the server. The TCP transport is the source of
// truth (a session is alive while its connection is open), so the
// transport owns the registry — this file only defines the small data
// types that flow between layers.
package application

import (
	"sort"
	"time"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// PresenceEntry describes one online user as far as the TUI cares.
type PresenceEntry struct {
	UserID      domain.UserID
	Username    string
	DisplayName string
	LastSeen    time.Time
	// Connections is the number of currently-open sessions for this user.
	Connections int
}

// SortPresence sorts a slice of PresenceEntry in place by username
// (case-insensitive). Provided for callers that build the slice themselves.
func SortPresence(es []PresenceEntry) {
	sort.Slice(es, func(i, j int) bool {
		return lessCI(es[i].Username, es[j].Username)
	})
}

func lessCI(a, b string) bool {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		ca := lower(ar[i])
		cb := lower(br[j])
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	return len(ar) < len(br)
}

func lower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}
