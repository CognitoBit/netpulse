// Package tui implements the interactive terminal UI: a main menu that hosts
// one feature view at a time, plus standalone wrappers so each subcommand can
// run its view directly.
//
// TODO: migrate to charm.land v2 module paths once the v2 ecosystem settles.
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CognitoBit/netpulse/internal/pingtest"
	"github.com/CognitoBit/netpulse/internal/trace"
)

type appState int

const (
	stateMenu appState = iota
	stateFeature
)

type appModel struct {
	version string
	state   appState
	menu    menuModel
	active  tea.Model
	size    tea.WindowSizeMsg
}

// RunMenu starts the full-menu TUI (the bare `netpulse` command).
func RunMenu(version string) error {
	p := tea.NewProgram(appModel{version: version}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m appModel) Init() tea.Cmd { return nil }

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.size = msg
		if m.active != nil {
			var cmd tea.Cmd
			m.active, cmd = m.active.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == stateMenu {
				return m, tea.Quit
			}
		}

	case menuChoiceMsg:
		return m.openFeature(int(msg))

	case backMsg:
		m.state = stateMenu
		m.active = nil
		return m, nil
	}

	if m.state == stateFeature && m.active != nil {
		var cmd tea.Cmd
		m.active, cmd = m.active.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.menu, cmd = m.menu.Update(msg)
	return m, cmd
}

func (m appModel) openFeature(idx int) (tea.Model, tea.Cmd) {
	switch idx {
	case 0:
		m.active = newSpeedView(speedOptions{PickBackend: true})
	case 1:
		m.active = newServeView(0, false)
	case 2:
		m.active = newLossView(pingtest.DefaultConfig())
	case 3:
		m.active = newMTRView(trace.DefaultConfig(""), true)
	default:
		return m, nil
	}
	m.state = stateFeature
	cmds := []tea.Cmd{m.active.Init()}
	if m.size.Width > 0 {
		var cmd tea.Cmd
		m.active, cmd = m.active.Update(m.size)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m appModel) View() string {
	if m.state == stateFeature && m.active != nil {
		return styleApp.Render(m.active.View())
	}
	return styleApp.Render(m.menu.View(m.version))
}

// standalone wraps a feature view launched directly from a subcommand:
// backing out quits the program instead of returning to a menu.
type standalone struct {
	inner tea.Model
}

// RunStandalone runs one feature view as the whole program.
func RunStandalone(inner tea.Model) error {
	p := tea.NewProgram(standalone{inner: inner}, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (s standalone) Init() tea.Cmd { return s.inner.Init() }

func (s standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case backMsg:
		return s, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return s, tea.Quit
		}
	}
	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

func (s standalone) View() string { return styleApp.Render(s.inner.View()) }

// Constructors exported for cmd/ so subcommands can launch views directly.

func NewLossView(cfg pingtest.Config) tea.Model { return newLossView(cfg) }
func NewMTRView(cfg trace.Config) tea.Model     { return newMTRView(cfg, false) }
func NewServeView(port int, explicit bool) tea.Model {
	return newServeView(port, explicit)
}
func NewSpeedView(backend string, serverID int) tea.Model {
	return newSpeedView(speedOptions{Backend: backend, ServerID: serverID})
}
