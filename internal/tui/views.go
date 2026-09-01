package tui

import (
	"fmt"
	"strings"
	"time"
)

func (m *Model) viewLogin() string {
	var b strings.Builder
	title := titleStyle.Render("chatd login")
	b.WriteString(title + "\n\n")

	mode := "login"
	if m.authMode == ModeRegister {
		mode = "register"
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("mode: %s   (ctrl+r to switch)", mode)) + "\n\n")
	b.WriteString(m.username.View() + "\n")
	b.WriteString(m.password.View() + "\n\n")
	if m.status != "" {
		if m.statusErr {
			b.WriteString(errorStyle.Render(m.status))
		} else {
			b.WriteString(statusStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("tab: switch field   enter: submit   ctrl+c: quit"))
	return b.String()
}

func (m *Model) viewConvs() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("chats — %s", m.user.Username)) + "\n\n")
	if m.loading {
		b.WriteString(dimStyle.Render("loading…") + "\n")
	} else if len(m.convs) == 0 {
		b.WriteString(dimStyle.Render("no conversations yet. press n to open one.") + "\n")
	} else {
		for i, c := range m.convs {
			if i == m.cursor {
				line := selectedStyle.Render(fmt.Sprintf("> %s  %s", string(c.Kind), displayTitle(c)))
				b.WriteString(line + "\n")
			} else {
				b.WriteString(fmt.Sprintf("  %s  %s\n", string(c.Kind), displayTitle(c)))
			}
		}
	}
	b.WriteString("\n")
	if m.status != "" {
		if m.statusErr {
			b.WriteString(errorStyle.Render(m.status))
		} else {
			b.WriteString(statusStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("↑/↓: move   enter: open   n: new chat   r: refresh   ctrl+c: quit"))
	return b.String()
}

func (m *Model) viewChat() string {
	var b strings.Builder
	header := "chat"
	if m.current != nil {
		header = fmt.Sprintf("chat — %s", displayTitle(*m.current))
	} else {
		header = "chat — no conversation"
	}
	b.WriteString(titleStyle.Render(header) + "\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n")

	// Body. Use viewport when we have a conversation so the user can
	// scroll older messages; otherwise show a hint.
	if m.current == nil {
		b.WriteString(dimStyle.Render("type /open <username> to start a conversation") + "\n")
	} else {
		var lines []string
		for _, mr := range m.messages {
			prefix := fmt.Sprintf("[%s] %s", shortTime(mr.CreatedAt), shortID(string(mr.Sender)))
			style := themStyle
			if string(mr.Sender) == m.user.Username || string(mr.Sender) == string(m.user.ID) {
				style = meStyle
			}
			lines = append(lines, style.Render(prefix)+" "+mr.Content)
		}
		if len(lines) == 0 {
			lines = append(lines, dimStyle.Render("no messages yet — say hi!"))
		}
		m.viewport.SetContent(strings.Join(lines, "\n"))
		b.WriteString(m.viewport.View())
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", m.width) + "\n")
	b.WriteString(m.input.View() + "\n")
	if m.status != "" {
		if m.statusErr {
			b.WriteString(errorStyle.Render(m.status))
		} else {
			b.WriteString(statusStyle.Render(m.status))
		}
		b.WriteString("\n")
	}
	b.WriteString(helpStyle.Render("enter: send   esc: back to chats   ctrl+c: quit"))
	return b.String()
}

func displayTitle(c ConvSummary) string {
	if c.Title != "" && c.Title != string(c.Kind) {
		return c.Title
	}
	return string(c.ID)[:8]
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func shortTime(t time.Time) string {
	return t.Format("15:04:05")
}
