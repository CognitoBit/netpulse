package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

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
	b.WriteString(styleTitle.Render("netpulse") + styleDim.Render("  "+version) + "\n")
	b.WriteString(styleDim.Render("network performance toolkit") + "\n\n")
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
