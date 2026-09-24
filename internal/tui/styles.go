package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Brand colours per design/design.md: pulse red is the product colour,
	// the warm grey is the CognitoBit "known bit" grey. Both have the
	// spec's light- and dark-background values.
	cPulse = lipgloss.AdaptiveColor{Light: "#C2472F", Dark: "#E8785F"}
	cBit   = lipgloss.AdaptiveColor{Light: "#C9C5BC", Dark: "#4A4843"}

	cAccent = lipgloss.AdaptiveColor{Light: "#1a7f4b", Dark: "#4cc38a"}
	cDim    = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#8a93a6"}
	cDown   = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"}
	cUp     = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"}
	cBad    = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"}
	cWarn   = lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"}

	stylePulse = lipgloss.NewStyle().Foreground(cPulse)

	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(cPulse)
	styleDim   = lipgloss.NewStyle().Foreground(cDim)
	styleBold  = lipgloss.NewStyle().Bold(true)
	styleBad   = lipgloss.NewStyle().Foreground(cBad)
	styleWarn  = lipgloss.NewStyle().Foreground(cWarn)
	styleGood  = lipgloss.NewStyle().Foreground(cAccent)
	styleDown  = lipgloss.NewStyle().Foreground(cDown).Bold(true)
	styleUp    = lipgloss.NewStyle().Foreground(cUp).Bold(true)

	styleHeader = lipgloss.NewStyle().Foreground(cDim).Bold(true).Underline(true)

	styleSelected = lipgloss.NewStyle().Foreground(cPulse).Bold(true)

	styleErrBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBad).
			Padding(0, 1)

	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cDim).
			Padding(0, 2)

	styleApp = lipgloss.NewStyle().Padding(1, 2)
)

// lossStyle color-grades a loss percentage.
func lossStyle(pct float64) lipgloss.Style {
	switch {
	case pct >= 2:
		return styleBad
	case pct > 0:
		return styleWarn
	default:
		return styleGood
	}
}
