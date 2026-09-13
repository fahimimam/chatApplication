package tcp

import (
	"sync"
	"time"

	"github.com/fahimimam/chatApplication/internal/application"
	"github.com/fahimimam/chatApplication/internal/domain"
)

// presenceServer is the per-server online-user registry. It is
// concurrency-safe and is updated by the TCP handler whenever a session
// logs in or disconnects.
type presenceServer struct {
	mu     sync.RWMutex
	byID   map[domain.UserID]*application.PresenceEntry
	byConn map[*session]domain.UserID
	nowFn  func() time.Time
}

func newPresence() *presenceServer {
	return &presenceServer{
		byID:   map[domain.UserID]*application.PresenceEntry{},
		byConn: map[*session]domain.UserID{},
		nowFn:  func() time.Time { return time.Now().UTC() },
	}
}

// markOnline associates a session with the given user. Multiple connections
// from the same user collapse into a single entry but increment Connections.
func (p *presenceServer) markOnline(u domain.User, sess *session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byConn[sess] = u.ID
	e, ok := p.byID[u.ID]
	if !ok {
		e = &application.PresenceEntry{
			UserID:      u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
		}
		p.byID[u.ID] = e
	}
	e.Connections++
	e.LastSeen = p.nowFn()
}

// markOffline drops a session. The user remains online if other connections
// are still open.
func (p *presenceServer) markOffline(sess *session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	uid, ok := p.byConn[sess]
	if !ok {
		return
	}
	delete(p.byConn, sess)
	if e, ok := p.byID[uid]; ok {
		e.Connections--
		if e.Connections <= 0 {
			delete(p.byID, uid)
		}
	}
}

// snapshot returns a copy sorted by username.
func (p *presenceServer) snapshot() []application.PresenceEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]application.PresenceEntry, 0, len(p.byID))
	for _, e := range p.byID {
		out = append(out, *e)
	}
	application.SortPresence(out)
	return out
}

func (p *presenceServer) size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byID)
}

func (p *presenceServer) isOnline(uid domain.UserID) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.byID[uid]
	return ok
}
