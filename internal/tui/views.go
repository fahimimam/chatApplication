package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fahimimam/chatApplication/internal/domain"
)

// --- palette ---------------------------------------------------------------

var (
	// Primary brand
	brand       = lipgloss.Color("#A78BFA") // soft violet
	brandStrong = lipgloss.Color("#7C3AED") // deep violet
	brandFaint  = lipgloss.Color("#3B2B5E") // deep violet bg

	// Accents
	green       = lipgloss.Color("#34D399")
	greenSoft   = lipgloss.Color("#86EFAC")
	amber       = lipgloss.Color("#FBBF24")
	red         = lipgloss.Color("#F87171")
	redSoft     = lipgloss.Color("#FCA5A5")
	blue        = lipgloss.Color("#60A5FA")
	pink        = lipgloss.Color("#F472B6")

	// Neutrals
	bgDark      = lipgloss.Color("#0F0B1A")
	surface     = lipgloss.Color("#16121F")
	surface2    = lipgloss.Color("#1F1A2E")
	surface3    = lipgloss.Color("#2A2440")
	borderDim   = lipgloss.Color("#3A3251")
	borderHi    = lipgloss.Color("#6D5BB5")
	text        = lipgloss.Color("#E5E1F4")
	textDim     = lipgloss.Color("#9B95B5")
	textFaint   = lipgloss.Color("#5E5879")
	selectedBg  = lipgloss.Color("#3B2B5E")
)

// --- styles ---------------------------------------------------------------

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(brand).
			Padding(0, 1)

	statusOkStyle  = lipgloss.NewStyle().Foreground(green)
	statusErrStyle = lipgloss.NewStyle().Foreground(red)
	statusDimStyle = lipgloss.NewStyle().Foreground(textDim)

	dimStyle = lipgloss.NewStyle().Foreground(textDim)
	faintStyle = lipgloss.NewStyle().Foreground(textFaint)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(selectedBg)

	meStyle   = lipgloss.NewStyle().Foreground(pink)
	themStyle = lipgloss.NewStyle().Foreground(greenSoft)
	helpStyle = lipgloss.NewStyle().Foreground(textFaint)

	paneFocusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderHi)

	paneUnfocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderDim)

	paneTitleFocusedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(text).
				Background(brandStrong).
				Padding(0, 1)

	paneTitleUnfocusedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textDim).
				Background(surface2).
				Padding(0, 1)

	onlineDotStyle  = lipgloss.NewStyle().Foreground(green).Bold(true)
	offlineDotStyle = lipgloss.NewStyle().Foreground(textFaint)
	onlineDot       = onlineDotStyle.Render("●")
	offlineDot      = offlineDotStyle.Render("○")

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(brandStrong).
			Padding(0, 1)

	inputUnfocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(borderDim).
				Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(textDim).
			Padding(0, 1)

	footerKeyStyle = lipgloss.NewStyle().
			Foreground(brand).
			Bold(true)

	footerSepStyle = lipgloss.NewStyle().
			Foreground(textFaint)

	appHeaderStyle = lipgloss.NewStyle().
			Background(bgDark).
			Foreground(brand).
			Bold(true).
			Padding(0, 2).
			Width(80)

	appHeaderAccentStyle = lipgloss.NewStyle().
				Background(bgDark).
				Foreground(green).
				Bold(true).
				Padding(0, 1)

	appBarStyle = lipgloss.NewStyle().
			Background(bgDark).
			Foreground(textDim)

	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(brand).
			Background(surface).
			Padding(1, 4).
			MarginTop(1).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(brandStrong)

	hintStyle = lipgloss.NewStyle().Foreground(textDim).Italic(true)

	logoStyle = lipgloss.NewStyle().
			Foreground(brand).
			Bold(true)

	logoAccentStyle = lipgloss.NewStyle().
			Foreground(brandStrong).
			Bold(true)

	badgeStyle = lipgloss.NewStyle().
			Foreground(text).
			Background(brandStrong).
			Bold(true).
			Padding(0, 1)

	badgeOkStyle = lipgloss.NewStyle().
			Foreground(bgDark).
			Background(green).
			Bold(true).
			Padding(0, 1)

	badgeWarnStyle = lipgloss.NewStyle().
			Foreground(bgDark).
			Background(amber).
			Bold(true).
			Padding(0, 1)

	chatHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(text).
			Background(surface3).
			Padding(0, 1)

	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(textFaint).
				Italic(true)
)

// smallCaps is a no-op transform; placeholder for future locale handling.
func smallCaps(s string) string { return s }

// compactTimestamp formats a time as "HH:MM" without seconds; "yesterday"
// for older entries would be nice but we don't track dates here.
func compactTimestamp(t time.Time) string {
	return t.Format("15:04")
}

// styledStatus returns a status line wrapped in a small inline "pill" so
// it stands out from the rest of the chrome.
func styledStatus(s string, isErr bool) string {
	label := "info"
	style := statusOkStyle
	if isErr {
		label = "error"
		style = statusErrStyle
	}
	badge := badgeStyle.Render(label)
	if isErr {
		badge = badgeWarnStyle.Render(label)
	}
	return badge + " " + style.Render(s)
}

// --- login view -----------------------------------------------------------

func (m *Model) viewLogin() string {
	// Big centered banner with the app name and tagline.
	logo := lipgloss.JoinHorizontal(lipgloss.Center,
		logoStyle.Render("◆ "),
		logoAccentStyle.Render("chatd"),
		logoStyle.Render(" terminal"),
	)
	tagline := hintStyle.Render("direct messages · live presence · zero browser")
	banner := lipgloss.JoinVertical(lipgloss.Center, logo, tagline)
	bannerBox := bannerStyle.Render(banner)

	mode := "login"
	if m.authMode == ModeRegister {
		mode = "register (creating a new account)"
	}
	modePill := badgeStyle.Render("mode: " + mode)

	form := lipgloss.JoinVertical(lipgloss.Left,
		modePill,
		"",
		dimStyle.Render("username"),
		m.username.View(),
		"",
		dimStyle.Render("password"),
		m.password.View(),
	)

	status := ""
	if m.status != "" {
		status = "\n" + styledStatus(m.status, m.statusErr)
	}

	help := renderHelp([]helpEntry{
		{"tab", "switch field"},
		{"enter", "submit"},
		{"ctrl+r", "toggle login/register"},
		{"ctrl+c", "quit"},
	})

	body := lipgloss.JoinVertical(lipgloss.Center,
		"",
		bannerBox,
		"",
		form,
		status,
		"",
		help,
	)
	return body
}

// --- main view ------------------------------------------------------------

// viewMain renders the three-pane layout with a header strip and a footer
// strip. Layout is computed from the current terminal size.
func (m *Model) viewMain() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	// Vertical regions: header (3) · body · footer (3).
	const headerH = 3
	const footerH = 3
	bodyH := m.height - headerH - footerH
	if bodyH < 6 {
		bodyH = 6
	}

	// Horizontal regions inside the body: left (22-32) · centre · right (24-36).
	leftW := m.width / 4
	if leftW < 22 {
		leftW = 22
	}
	if leftW > 32 {
		leftW = 32
	}
	rightW := m.width / 4
	if rightW < 22 {
		rightW = 22
	}
	if rightW > 36 {
		rightW = 36
	}
	centreW := m.width - leftW - rightW - 4 // borders
	if centreW < 24 {
		centreW = 24
	}

	header := m.viewHeader(m.width)
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.viewConversations(leftW, bodyH),
		m.viewChat(centreW, bodyH),
		m.viewUsers(rightW, bodyH),
	)
	footer := m.viewFooter(m.width)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m *Model) viewHeader(width int) string {
	left := lipgloss.JoinHorizontal(lipgloss.Center,
		logoStyle.Render("◆ "),
		logoAccentStyle.Render("chatd"),
	)
	mid := dimStyle.Render("·  ") + statusDimStyle.Render("you are "+meStyle.Render(m.user.Username))
	right := ""
	if m.current != nil {
		right = chatHeaderStyle.Render("  #  " + displayTitle(*m.current) + "  ")
	} else {
		right = faintStyle.Render("  no conversation  ")
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	return appBarStyle.Width(width).Render(
		left + strings.Repeat(" ", gap/2) + mid + strings.Repeat(" ", gap-gap/2) + right,
	)
}

func (m *Model) viewConversations(width, height int) string {
	title := paneTitleFocusedStyle.Render("Conversations")
	if m.pane != PaneConvs {
		title = paneTitleUnfocusedStyle.Render("Conversations")
	}

	var content strings.Builder
	inner := width - 4
	if inner < 8 {
		inner = 8
	}
	switch {
	case m.convLoading && len(m.convs) == 0:
		content.WriteString(dimStyle.Render("  loading…"))
	case len(m.convs) == 0:
		content.WriteString(faintStyle.Render("  no conversations yet"))
		content.WriteString("\n")
		content.WriteString(hintStyle.Render("  press n to start one"))
	default:
		for i, c := range m.convs {
			dot := "@"
			if c.Kind == "group" {
				dot = "#"
			}
			name := displayTitle(c)
			line := fmt.Sprintf(" %s  %s", dot, name)
			if len(line) > inner {
				line = line[:inner-1] + "…"
			}
			if i == m.convCursor {
				content.WriteString(selectedStyle.Width(width - 4).Render(" "+line))
			} else {
				content.WriteString(line)
			}
			content.WriteString("\n")
		}
	}

	frame := paneFocusedStyle
	if m.pane != PaneConvs {
		frame = paneUnfocusedStyle
	}
	body := title + "\n" + content.String()
	return frame.Width(width - 2).Height(height - 2).Render(body)
}

func (m *Model) viewChat(width, height int) string {
	header := "Chat"
	if m.current != nil {
		header = "Chat — " + displayTitle(*m.current)
	}
	title := paneTitleFocusedStyle.Render(header)
	if m.pane != PaneChat {
		title = paneTitleUnfocusedStyle.Render(header)
	}

	var content strings.Builder
	if m.current == nil {
		content.WriteString("\n")
		content.WriteString(hintStyle.Render("  no conversation open"))
		content.WriteString("\n\n")
		content.WriteString(bullet("press ", "n", " to start a new chat"))
		content.WriteString("\n")
		content.WriteString(bullet("press ", "u", " to browse users"))
		content.WriteString("\n")
		content.WriteString(bullet("press ", "?", " for help"))
	} else {
		var lines []string
		if len(m.messages) == 0 {
			lines = append(lines, hintStyle.Render("  no messages yet — say hi!"))
		} else {
			// Show a scroll indicator if there are more messages above.
			if !m.viewport.AtTop() {
				lines = append(lines, scrollIndicatorStyle.Render("  ↑ older messages"))
			}
			inner := width - 6
			if inner < 12 {
				inner = 12
			}
			for _, mr := range m.messages {
				ts := compactTimestamp(mr.CreatedAt)
				name := displaySender(mr, m.user)
				if len(name) > 14 {
					name = name[:13] + "…"
				}
				prefix := fmt.Sprintf("%s  %s", faintStyle.Render(ts), themStyle.Render(fmt.Sprintf("%-14s", name)))
				style := themStyle
				if isMe(mr, m.user) {
					style = meStyle
				}
				wrapped := wrap(mr.Content, inner-18)
				if len(wrapped) == 0 {
					lines = append(lines, style.Render(prefix)+" ")
				} else {
					lines = append(lines, style.Render(prefix)+" "+wrapped[0])
					for _, extra := range wrapped[1:] {
						lines = append(lines, strings.Repeat(" ", 18)+style.Render(extra))
					}
				}
			}
		}
		m.viewport.SetContent(strings.Join(lines, "\n"))
		content.WriteString(m.viewport.View())
	}

	frame := paneFocusedStyle
	if m.pane != PaneChat {
		frame = paneUnfocusedStyle
	}
	body := title + "\n" + content.String()
	return frame.Width(width - 2).Height(height - 2).Render(body)
}

func (m *Model) viewUsers(width, height int) string {
	title := paneTitleFocusedStyle.Render("People")
	if m.pane != PaneUsers {
		title = paneTitleUnfocusedStyle.Render("People")
	}

	var content strings.Builder
	inner := width - 6
	if inner < 8 {
		inner = 8
	}
	if len(m.users) == 0 {
		content.WriteString(hintStyle.Render("  loading directory…"))
	} else {
		for i, u := range m.users {
			dot := offlineDot
			if u.Online {
				dot = onlineDot
			}
			name := u.Display
			if name == "" {
				name = u.Username
			}
			if u.Display != "" && u.Display != u.Username {
				name = fmt.Sprintf("%s (%s)", u.Display, u.Username)
			}
			if len(name) > inner-2 {
				name = name[:inner-3] + "…"
			}
			tag := ""
			if u.Self {
				tag = badgeOkStyle.Render(" you ")
			}
			line := fmt.Sprintf(" %s  %s", dot, name)
			if i == m.userCur && m.pane == PaneUsers {
				content.WriteString(selectedStyle.Width(width - 4).Render(line) + " " + tag)
			} else {
				if u.Online {
					content.WriteString(line + " " + tag)
				} else {
					content.WriteString(faintStyle.Render(line) + " " + tag)
				}
			}
			content.WriteString("\n")
		}
	}

	// Footer line: online count badge.
	online := m.countOnline()
	total := len(m.users)
	footer := " " + badgeOkStyle.Render(fmt.Sprintf("%d online", online)) +
		"  " + faintStyle.Render(fmt.Sprintf("/ %d total", total))

	frame := paneFocusedStyle
	if m.pane != PaneUsers {
		frame = paneUnfocusedStyle
	}
	body := title + "\n" + content.String() + "\n" + footer
	return frame.Width(width - 2).Height(height - 2).Render(body)
}

// viewFooter renders the bottom block: a single status line, the chat
// input (or overlay input), and the help text. The total height matches
// the footerH reservation in viewMain.
func (m *Model) viewFooter(width int) string {
	var status string
	switch {
	case m.status != "":
		status = styledStatus(m.status, m.statusErr)
	case len(m.notifs) > 0:
		status = badgeStyle.Render("notify") + " " + dimStyle.Render(strings.Join(m.notifs, " · "))
	default:
		status = badgeOkStyle.Render("ready") + " " + dimStyle.Render("connected to server")
	}

	// Input row.
	var inputView string
	switch m.overlay {
	case OverlayNewChat:
		inputView = paneTitleFocusedStyle.Render("New chat") + " " + m.overlayBuf.View()
	case OverlaySearchUsers:
		inputView = paneTitleFocusedStyle.Render("Search users") + " " + m.overlayBuf.View()
	case OverlayHelp:
		help := renderHelp(m.helpEntries())
		return appBarStyle.Width(width).Render(status) + "\n" +
			appBarStyle.Width(width).Render(help) + "\n" +
			appBarStyle.Width(width).Render(m.helpBar(width))
	default:
		if m.current != nil {
			// Wrap input in a rounded box when focused.
			inputView = m.input.View()
		} else {
			inputView = hintStyle.Render("  (open or start a conversation to type)")
		}
	}

	// Compose: status (1) · input (3 incl. border) · help (1) — wait,
	// that overflows. We reserve footerH = 3 lines so we pack status on
	// top of input/hint and the help bar on its own line.
	return appBarStyle.Width(width).Render(status) + "\n" +
		appBarStyle.Width(width).Render(inputView) + "\n" +
		appBarStyle.Width(width).Render(m.helpBar(width))
}

// helpBar renders the global shortcut strip at the bottom of the screen.
func (m *Model) helpBar(width int) string {
	entries := m.helpEntries()
	return renderHelp(entries)
}

func (m *Model) helpEntries() []helpEntry {
	switch m.overlay {
	case OverlayHelp:
		return []helpEntry{
			{"tab / shift+tab", "switch pane"},
			{"n", "new chat"},
			{"u", "search users"},
			{"r", "refresh"},
			{"/", "slash command"},
			{"esc", "close this help"},
			{"ctrl+c", "quit"},
		}
	}
	switch m.pane {
	case PaneConvs:
		return []helpEntry{
			{"tab", "switch pane"},
			{"↑/↓", "move"},
			{"enter", "open chat"},
			{"n", "new chat"},
			{"r", "refresh"},
			{"?", "help"},
			{"ctrl+c", "quit"},
		}
	case PaneUsers:
		return []helpEntry{
			{"tab", "switch pane"},
			{"↑/↓", "move"},
			{"enter", "open chat"},
			{"u", "search"},
			{"r", "refresh"},
			{"?", "help"},
			{"ctrl+c", "quit"},
		}
	case PaneChat:
		return []helpEntry{
			{"tab", "leave input"},
			{"enter", "send"},
			{"esc", "leave input"},
			{"/open <u>", "open chat (in input)"},
			{"/users", "refresh people (in input)"},
			{"/logout", "log out (in input)"},
			{"ctrl+c", "quit"},
		}
	}
	return nil
}

// --- helpers --------------------------------------------------------------

type helpEntry struct {
	key  string
	desc string
}

func renderHelp(entries []helpEntry) string {
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		key := footerKeyStyle.Render(e.key)
		parts[i] = key + " " + footerSepStyle.Render(e.desc)
	}
	return strings.Join(parts, footerSepStyle.Render("  ·  "))
}

func bullet(prefix, key, suffix string) string {
	return "  " + prefix + footerKeyStyle.Render(key) + suffix
}

// wrap breaks s into lines no longer than width, splitting on word
// boundaries. It preserves explicit spaces by treating each rune as a
// potential split point only when no whitespace is available.
func wrap(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	var lines []string
	for {
		s = strings.TrimLeft(s, " ")
		if len(s) == 0 {
			return lines
		}
		if len(s) <= width {
			lines = append(lines, s)
			return lines
		}
		// Find a space at or before width.
		cut := width
		sp := strings.LastIndex(s[:width+1], " ")
		if sp > 0 {
			cut = sp
		}
		lines = append(lines, s[:cut])
		s = s[cut:]
	}
}

func displayTitle(c ConvSummary) string {
	if c.Title != "" && c.Title != string(c.Kind) {
		return c.Title
	}
	return string(c.ID)[:min(8, len(c.ID))]
}

func displaySender(mr MessageRow, me domain.User) string {
	if mr.SenderName != "" {
		return mr.SenderName
	}
	if string(mr.Sender) == me.Username {
		return me.Username
	}
	return shortID(string(mr.Sender))
}

func isMe(mr MessageRow, me domain.User) bool {
	if string(mr.Sender) == string(me.ID) || string(mr.Sender) == me.Username {
		return true
	}
	if mr.SenderName == me.Username && me.Username != "" {
		return true
	}
	return false
}

func dotFor(c ConvSummary) string {
	if c.Kind == "group" {
		return "#"
	}
	return "@"
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func shortTime(t time.Time) string {
	return t.Format("15:04")
}

func (m *Model) countOnline() int {
	n := 0
	for _, u := range m.users {
		if u.Online {
			n++
		}
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// suppress unused import for domain; the viewConversations type uses it
// indirectly via ConvSummary.
var _ = domain.UserID("")
