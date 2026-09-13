package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// TestSnapshotLoginView dumps the login screen view at a known size so
// the visuals can be eyeballed from the test output.
func TestSnapshotLoginView(t *testing.T) {
	m := NewModel()
	m.username.Width = 28
	m.password.Width = 28
	m.width = 100
	m.height = 32

	got := stripANSI(m.viewLogin())
	t.Logf("\n%s", got)
}

func TestSnapshotMainView(t *testing.T) {
	m := makeReadyModel(t)
	// Force a comfortable viewport.
	m.viewport = viewport.New(70, 18)
	m.viewport.SetContent("")
	m.width = 110
	m.height = 32

	cs := ConvSummary{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"}
	m.current = &cs
	m.convCursor = 0
	m.userCur = 1
	m.pane = PaneConvs

	m.messages = []MessageRow{
		{ID: "1", Sender: "u-2", SenderName: "bob", ConversationID: "conv-1",
			Content: "hey alice, are you around?", CreatedAt: time.Unix(1735732800, 0).UTC()},
		{ID: "2", Sender: domain.UserID(m.user.ID), SenderName: m.user.Username, ConversationID: "conv-1",
			Content: "yep — just sitting down with coffee. what's up?", CreatedAt: time.Unix(1735732860, 0).UTC()},
		{ID: "3", Sender: "u-2", SenderName: "bob", ConversationID: "conv-1",
			Content: "wanna hop on the deploy thing? carol said she wants to be on it too.", CreatedAt: time.Unix(1735732920, 0).UTC()},
	}
	m.input.SetValue("count me in — pulling up the repo now")
	m.convs = []ConvSummary{
		{ID: "conv-1", Kind: domain.KindDirect, Title: "bob"},
		{ID: "conv-2", Kind: domain.KindGroup, Title: "team-platform"},
	}
	m.notifs = []string{"new message from bob"}

	got := stripANSI(m.viewMain())
	t.Logf("\n%s", got)
}

// TestSnapshotInputDriving shows the model's Update accepting
// key-equivalent messages, simulating the user typing a username and
// password, without needing a real TTY. This is the canonical end-to-end
// repro for the "not logged in" bug: if typing into the username field
// doesn't accumulate, the login submit will fail and a stale status
// will linger.
func TestSnapshotInputDriving(t *testing.T) {
	m := NewModel()
	m.width = 100
	m.height = 32
	m.username.Focus()
	m.password.Blur()

	for _, r := range "alice" {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		nm, _ := m.Update(key)
		m = nm.(*Model)
	}
	// Tab to password.
	tab := tea.KeyMsg{Type: tea.KeyTab}
	nm, _ := m.Update(tab)
	m = nm.(*Model)
	for _, r := range "hunter2" {
		key := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		nm2, _ := m.Update(key)
		m = nm2.(*Model)
	}
	if !strings.Contains(m.username.Value(), "alice") {
		t.Errorf("username input missing 'alice': %q", m.username.Value())
	}
	if !strings.Contains(m.password.Value(), "hunter2") {
		t.Errorf("password input missing 'hunter2': %q", m.password.Value())
	}
}

// Compile-time guard: textinput is used in main package; keep the import.
var _ textinput.Model
