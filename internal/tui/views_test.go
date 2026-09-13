package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// TestViewLoginRenders checks the login view renders to a non-empty string
// without panicking, and that the expected key bindings are visible.
func TestViewLoginRenders(t *testing.T) {
	m := NewModel()
	m.width = 80
	m.height = 24
	got := stripANSI(m.viewLogin())
	if got == "" {
		t.Fatal("login view empty")
	}
	for _, want := range []string{"chatd", "tab", "ctrl+c"} {
		if !strings.Contains(got, want) {
			t.Errorf("login view missing %q", want)
		}
	}
}

// TestViewMainRenders exercises the main view at a few sizes to make
// sure the panes fit and the helpers don't panic on edge cases.
func TestViewMainRenders(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24},
		{120, 36},
		{40, 24}, // narrow
	}
	for _, sz := range sizes {
		m := makeReadyModel(t)
		m.width = sz.w
		m.height = sz.h
		got := stripANSI(m.viewMain())
		if got == "" {
			t.Errorf("viewMain empty at %dx%d", sz.w, sz.h)
		}
		// The output should not exceed the terminal height by more than
		// a small slack (rounded borders + status line).
		lines := strings.Split(got, "\n")
		if len(lines) > sz.h+4 {
			t.Errorf("viewMain too many lines at %dx%d: %d", sz.w, sz.h, len(lines))
		}
	}
}

func TestViewMainWithChat(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 100
	m.height = 30
	cs := ConvSummary{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"}
	m.current = &cs
	m.messages = []MessageRow{
		{ID: "1", Sender: "u1", SenderName: "bob", ConversationID: "conv-1", Content: "hello", CreatedAt: time.Unix(1735732800, 0).UTC()},
		{ID: "2", Sender: domain.UserID(m.user.ID), SenderName: m.user.Username, ConversationID: "conv-1", Content: "hi back", CreatedAt: time.Unix(1735732810, 0).UTC()},
	}
	got := stripANSI(m.viewMain())
	if !strings.Contains(got, "hello") || !strings.Contains(got, "hi back") {
		t.Errorf("view missing messages: %q", got)
	}
}

// TestHelpBarChanges ensures the help strip is different per pane so
// users get context-relevant shortcuts.
func TestHelpBarChanges(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 100
	m.height = 24
	cs := ConvSummary{ID: "c", Kind: domain.KindDirect, Title: "bob"}
	m.current = &cs

	m.pane = PaneConvs
	convs := stripANSI(m.helpBar(100))
	m.pane = PaneChat
	chat := stripANSI(m.helpBar(100))
	m.pane = PaneUsers
	users := stripANSI(m.helpBar(100))

	if convs == chat || chat == users || convs == users {
		t.Errorf("help bars identical for panes: %q == %q", convs, chat)
	}
}

// --- helpers -------------------------------------------------------------

// makeReadyModel returns a model positioned at ViewMain with a logged-in
// user, fake conv list, and a populated People pane.
func makeReadyModel(t *testing.T) *Model {
	t.Helper()
	m := NewModel()
	m.view = ViewMain
	m.user = domain.User{ID: "u-1", Username: "alice", DisplayName: "Alice"}
	m.meLoaded = true
	m.pane = PaneConvs
	m.convs = []ConvSummary{
		{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"},
		{ID: "conv-2", Kind: domain.KindGroup, Title: "team"},
	}
	m.users = []UserEntry{
		{ID: "u-1", Username: "alice", Display: "Alice", Online: true, Self: true},
		{ID: "u-2", Username: "bob", Display: "Bob", Online: true},
		{ID: "u-3", Username: "carol", Display: "Carol", Online: false},
	}
	// Ensure textinputs have non-zero widths so the .View() strings
	// don't end up zero-length and trip up the pane heights.
	m.username = textinput.New()
	m.username.Placeholder = "username"
	m.username.CharLimit = 32
	m.username.Width = 24
	m.password = textinput.New()
	m.password.Placeholder = "password"
	m.password.CharLimit = 128
	m.password.Width = 24
	m.input.CharLimit = 4096
	m.input.Width = 80
	m.overlayBuf.CharLimit = 64
	m.overlayBuf.Width = 30
	return m
}

// stripANSI removes ANSI escape sequences so substring assertions don't
// get tripped up by colour codes.
func stripANSI(s string) string {
	var b strings.Builder
	state := 0
	for _, r := range s {
		switch state {
		case 0:
			if r == 0x1b {
				state = 1
				continue
			}
			b.WriteRune(r)
		case 1:
			if r == '[' {
				state = 2
				continue
			}
			state = 0
		case 2:
			if (r >= '0' && r <= '9') || r == ';' {
				continue
			}
			state = 0
		}
	}
	return b.String()
}