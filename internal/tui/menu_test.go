package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderMarkShape(t *testing.T) {
	lines := strings.Split(renderMark(), "\n")
	if len(lines) != 6 {
		t.Fatalf("mark should render as 6 terminal lines, got %d", len(lines))
	}
	// Four 2-cell-wide bits per line with single-space gaps: 4*2 + 3 = 11,
	// which also equals the mark's height (5.5 half-lines) on a 1:2 font.
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 11 {
			t.Errorf("line %d visible width = %d, want 11", i+1, w)
		}
	}
	// Split rows render as lower-half then upper-half blocks.
	for _, i := range []int{1, 4} {
		if !strings.Contains(lines[i], "▄") {
			t.Errorf("line %d should use lower-half blocks", i+1)
		}
	}
	for _, i := range []int{2, 5} {
		if !strings.Contains(lines[i], "▀") {
			t.Errorf("line %d should use upper-half blocks", i+1)
		}
	}
	// The missing bit (row 3, col 3) must stay empty: grid row 3 is terminal
	// line 4, where the empty cell plus its gaps form a 4-space hole.
	if !strings.Contains(lines[3], "    ") {
		t.Error("line 4 should contain the empty missing-bit cell")
	}
	if strings.Contains(lines[0], "    ") {
		t.Error("line 1 should have no missing cells")
	}
}

func TestMenuViewContainsBrandAndItems(t *testing.T) {
	view := menuModel{}.View("v9.9.9")
	for _, want := range []string{"netpulse", "v9.9.9", "Speed Test", "LAN Speed Server", "Packet Loss", "MTR"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu view missing %q", want)
		}
	}
}
