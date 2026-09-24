package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// renderMark draws the netpulse bit-grid (design/design.md) as a square:
//
//	k c k k        (k = known bit, c = pulse bit, m = missing bit)
//	c k c k
//	k k m c
//	k k k k
//
// On a typical 1:2 terminal font a bit is "██" (two chars ≈ one square) and
// the grid needs equal thin gaps both ways: one char horizontally, half a
// line vertically. Half-line gaps come from splitting alternate grid rows
// across two terminal lines as "▄▄" then "▀▀" — the halves stack into a full
// square bit with empty half-lines above and below. Total: 11 chars wide by
// 5.5 half-lines tall, i.e. equal width and height. The gap at row 3, col 3
// is the dropped packet and stays empty in every variant of the mark.
func renderMark() string {
	kSt := lipgloss.NewStyle().Foreground(cBit)
	cSt := lipgloss.NewStyle().Foreground(cPulse)
	rows := [4]string{"kckk", "ckck", "kkmc", "kkkk"}
	row := func(r string, glyph string) string {
		cells := make([]string, 0, 4)
		for _, b := range r {
			switch b {
			case 'c':
				cells = append(cells, cSt.Render(glyph))
			case 'k':
				cells = append(cells, kSt.Render(glyph))
			default: // missing bit: always empty
				cells = append(cells, "  ")
			}
		}
		return strings.Join(cells, " ")
	}
	return strings.Join([]string{
		row(rows[0], "██"), // row 1, full line
		row(rows[1], "▄▄"), // row 2, lower half…
		row(rows[1], "▀▀"), // …plus upper half: square bit, half-line gaps
		row(rows[2], "██"), // row 3, full line
		row(rows[3], "▄▄"), // row 4, split like row 2
		row(rows[3], "▀▀"),
	}, "\n")
}

type menuItem struct {
	title, desc string
}

var menuItems = []menuItem{
	{"Speed Test", "internet download/upload/latency (Ookla or Cloudflare)"},
	{"LAN Speed Server", "host a browser speed test for devices on your network"},
	{"Packet Loss", "ICMP burst test — loss %, latency, jitter"},
	{"MTR", "traceroute with per-hop loss and latency"},
}

// menuChoiceMsg tells the app model which feature was selected.
type menuChoiceMsg int

type menuModel struct {
	cursor int
}

func (m menuModel) Update(msg tea.Msg) (menuModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(menuItems)-1 {
				m.cursor++
			}
		case "enter":
			choice := m.cursor
			return m, func() tea.Msg { return menuChoiceMsg(choice) }
		case "1", "2", "3", "4":
			choice := int(key.String()[0] - '1')
			return m, func() tea.Msg { return menuChoiceMsg(choice) }
		}
	}
	return m, nil
}

func (m menuModel) View(version string) string {
	var b strings.Builder
	head := styleTitle.Render("netpulse") + styleDim.Render("  "+version) + "\n" +
		styleDim.Render("network performance toolkit")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, renderMark(), "  ", head) + "\n\n")
	for i, item := range menuItems {
		cursor := "  "
		title := item.title
		if i == m.cursor {
			cursor = styleSelected.Render("> ")
			title = styleSelected.Render(title)
		}
		b.WriteString(fmt.Sprintf("%s%d. %-18s %s\n", cursor, i+1, title, styleDim.Render(item.desc)))
	}
	b.WriteString("\n" + styleDim.Render("↑/↓ move · enter select · q quit"))
	return b.String()
}
