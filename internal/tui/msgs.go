package tui

import tea "github.com/charmbracelet/bubbletea"

// backMsg is emitted by a feature view when the user backs out (Esc). The
// menu root swaps back to the menu; the standalone wrapper quits instead.
type backMsg struct{}

func back() tea.Msg { return backMsg{} }

// errMsg carries a fatal engine error into a view.
type errMsg struct{ err error }

// waitFor bridges an engine's snapshot channel into the Bubble Tea loop: it
// delivers one message per receive and must be re-armed by the view after
// each message. A closed channel delivers the done message.
func waitFor[T any](ch <-chan T, wrap func(T) tea.Msg, done tea.Msg) tea.Cmd {
	return func() tea.Msg {
		v, ok := <-ch
		if !ok {
			return done
		}
		return wrap(v)
	}
}
