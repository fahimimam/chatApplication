package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// driveKeys updates the model with each key in order, replacing m with
// the returned model on each step so subsequent keys see the new state.
func driveKeys(t *testing.T, m *Model, keys []tea.KeyMsg) *Model {
	t.Helper()
	for i, k := range keys {
		nm, _ := m.Update(k)
		mm, ok := nm.(*Model)
		if !ok {
			t.Fatalf("step %d: Update returned %T, want *Model", i, nm)
		}
		m = mm
	}
	return m
}

func runeKeys(s string) []tea.KeyMsg {
	out := make([]tea.KeyMsg, 0, len(s))
	for _, r := range s {
		out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return out
}

// TestChatInputSwallowsGlobalShortcuts is the regression for the bug
// where typing "going north" in the chat input would open the new-chat
// overlay because the bare `n` key triggered the global `n` shortcut
// before reaching the text input.
func TestChatInputSwallowsGlobalShortcuts(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 110
	m.height = 32
	cs := ConvSummary{ID: "conv-1", Kind: "direct", Title: "bob"}
	m.current = &cs
	m.pane = PaneChat
	m.input.Focus() // mimic what cyclePane does on landing in chat

	m = driveKeys(t, m, runeKeys("going north"))

	if m.overlay != OverlayNone {
		t.Fatalf("overlay opened while typing: got %v want OverlayNone", m.overlay)
	}
	if !strings.Contains(m.input.Value(), "going north") {
		t.Fatalf("input missing typed text, got %q", m.input.Value())
	}
}

// TestChatInputSwallowsSlash verifies that "/" doesn't open the help
// overlay when the chat input is focused.
func TestChatInputSwallowsSlash(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 110
	m.height = 32
	cs := ConvSummary{ID: "conv-1", Kind: "direct", Title: "bob"}
	m.current = &cs
	m.pane = PaneChat
	m.input.Focus()

	m = driveKeys(t, m, runeKeys("/help me"))

	if m.overlay != OverlayNone {
		t.Fatalf("overlay opened while typing /help me: got %v", m.overlay)
	}
	if !strings.Contains(m.input.Value(), "/help me") {
		t.Fatalf("input missing '/help me', got %q", m.input.Value())
	}
}

// TestChatInputSwallowsRefreshKey verifies that "r" doesn't trigger a
// refresh while the user is typing. We can't directly observe a fired
// batch command, but if the shortcut ran we'd see two side-effects:
// (1) no `r` would land in the input buffer, and (2) the conversation
// list would be replaced by an empty reload (no client wired here, so
// the batch would fail silently — but the input definitely would be
// empty). We assert the rune landed in the input.
func TestChatInputSwallowsRefreshKey(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 110
	m.height = 32
	cs := ConvSummary{ID: "conv-1", Kind: "direct", Title: "bob"}
	m.current = &cs
	m.pane = PaneChat
	m.input.Focus()

	beforeConvs := len(m.convs)
	m = driveKeys(t, m, runeKeys("render this"))
	if len(m.convs) != beforeConvs {
		t.Fatalf("refresh fired while typing 'render this': convs=%d want %d",
			len(m.convs), beforeConvs)
	}
	if !strings.Contains(m.input.Value(), "render this") {
		t.Fatalf("input missing text, got %q", m.input.Value())
	}
}

// TestGlobalShortcutsStillWorkWhenInputUnfocused ensures we didn't break
// the original use of the shortcuts — `n` from the conversations pane
// must still open the new-chat overlay.
func TestGlobalShortcutsStillWorkWhenInputUnfocused(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 110
	m.height = 32
	cs := ConvSummary{ID: "conv-1", Kind: "direct", Title: "bob"}
	m.current = &cs
	m.pane = PaneConvs
	m.input.Blur()

	m = driveKeys(t, m, []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'n'}}})
	if m.overlay != OverlayNewChat {
		t.Fatalf("n shortcut broken: overlay=%v want OverlayNewChat", m.overlay)
	}
}

// TestCtrlCAlwaysQuits confirms the always-on global escape works even
// while the chat input has focus.
func TestCtrlCAlwaysQuits(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 110
	m.height = 32
	cs := ConvSummary{ID: "conv-1", Kind: "direct", Title: "bob"}
	m.current = &cs
	m.pane = PaneChat
	m.input.Focus()

	nm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	mm := nm.(*Model)
	if !mm.quitting {
		t.Fatalf("ctrl+c did not set quitting flag")
	}
	if cmd == nil {
		t.Fatalf("ctrl+c did not return tea.Quit")
	}
}

// TestEnterSubmitsFromFocusedInput makes sure Enter still goes through
// to handleChatKey when the input is focused (i.e. dispatching the
// chat submission path works).
func TestEnterSubmitsFromFocusedInput(t *testing.T) {
	m := makeReadyModel(t)
	m.width = 110
	m.height = 32
	cs := ConvSummary{ID: "conv-1", Kind: "direct", Title: "bob"}
	m.current = &cs
	m.pane = PaneChat
	m.input.Focus()
	m.input.SetValue("hello there")

	m = driveKeys(t, m, []tea.KeyMsg{{Type: tea.KeyEnter}})

	// After submit, the input buffer is cleared and the message went
	// out as a Send command (a tea.Cmd, not directly verifiable here,
	// but the input being cleared is a reliable side-effect).
	if m.input.Value() != "" {
		t.Fatalf("input not cleared after enter, got %q", m.input.Value())
	}
}