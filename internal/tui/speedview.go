package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CognitoBit/netpulse/internal/netutil"
	"github.com/CognitoBit/netpulse/internal/speed"
)

type speedOptions struct {
	PickBackend bool   // show the backend picker first (menu flow)
	Backend     string // "ookla" or "cloudflare" when preselected
	ServerID    int
}

type speedStartedMsg struct {
	events <-chan speed.Event
	result <-chan speedOutcome
	cancel context.CancelFunc
}
type speedOutcome struct {
	res *speed.Result
	err error
}
type speedEventMsg speed.Event
type speedEventsClosedMsg struct{}
type speedFinishedMsg speedOutcome

var backendChoices = []string{"ookla", "cloudflare"}

type speedView struct {
	opts   speedOptions
	pick   bool
	cursor int

	spin   spinner.Model
	cancel context.CancelFunc
	events <-chan speed.Event
	result <-chan speedOutcome

	phase    speed.Phase
	statusMsg string
	rate     float64
	latency  string
	progress float64
	down, up float64 // final phase rates observed live

	res *speed.Result
	err error

	width int
}

func newSpeedView(opts speedOptions) *speedView {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleGood
	return &speedView{opts: opts, pick: opts.PickBackend, spin: sp, width: 80}
}

func (v *speedView) Init() tea.Cmd {
	if v.pick {
		return nil
	}
	return tea.Batch(v.start, v.spin.Tick)
}

func (v *speedView) start() tea.Msg {
	backend, ok := speed.New(v.opts.Backend, v.opts.ServerID)
	if !ok {
		return errMsg{fmt.Errorf("unknown backend %q (want ookla or cloudflare)", v.opts.Backend)}
	}
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan speed.Event, 32)
	result := make(chan speedOutcome, 1)
	go func() {
		res, err := backend.Run(ctx, events)
		close(events)
		result <- speedOutcome{res: res, err: err}
	}()
	return speedStartedMsg{events: events, result: result, cancel: cancel}
}

func (v *speedView) waitEvent() tea.Cmd {
	return waitFor(v.events, func(e speed.Event) tea.Msg { return speedEventMsg(e) }, speedEventsClosedMsg{})
}

func (v *speedView) waitResult() tea.Cmd {
	res := v.result
	return func() tea.Msg { return speedFinishedMsg(<-res) }
}

func (v *speedView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width

	case spinner.TickMsg:
		var cmd tea.Cmd
		v.spin, cmd = v.spin.Update(msg)
		return v, cmd

	case speedStartedMsg:
		v.events = msg.events
		v.result = msg.result
		v.cancel = msg.cancel
		return v, v.waitEvent()

	case speedEventMsg:
		e := speed.Event(msg)
		v.phase = e.Phase
		v.progress = e.Progress
		if e.Msg != "" {
			v.statusMsg = e.Msg
		}
		if e.Rate > 0 {
			v.rate = e.Rate
			switch e.Phase {
			case speed.PhaseDownload:
				v.down = e.Rate
			case speed.PhaseUpload:
				v.up = e.Rate
			}
		}
		if e.Latency > 0 {
			v.latency = netutil.FormatLatency(e.Latency)
		}
		if v.events != nil {
			return v, v.waitEvent()
		}

	case speedEventsClosedMsg:
		// events channel closed; switch to awaiting the result
		return v, v.waitResult()

	case speedFinishedMsg:
		v.res = msg.res
		v.err = msg.err
		return v, nil

	case errMsg:
		v.err = msg.err

	case tea.KeyMsg:
		if v.pick {
			switch msg.String() {
			case "esc":
				return v, back
			case "up", "k":
				if v.cursor > 0 {
					v.cursor--
				}
			case "down", "j":
				if v.cursor < len(backendChoices)-1 {
					v.cursor++
				}
			case "enter":
				v.opts.Backend = backendChoices[v.cursor]
				v.pick = false
				return v, tea.Batch(v.start, v.spin.Tick)
			}
			return v, nil
		}
		switch msg.String() {
		case "esc", "q":
			v.stop()
			return v, back
		case "r":
			if v.res != nil || v.err != nil {
				v.stop()
				v.res, v.err = nil, nil
				v.rate, v.down, v.up, v.progress = 0, 0, 0, 0
				v.latency, v.statusMsg = "", ""
				v.phase = ""
				if v.opts.PickBackend {
					v.pick = true
					return v, nil
				}
				return v, tea.Batch(v.start, v.spin.Tick)
			}
		}
	}
	return v, nil
}

func (v *speedView) stop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *speedView) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Speed Test") + "\n")

	if v.pick {
		b.WriteString(styleDim.Render("choose a backend") + "\n\n")
		descs := map[string]string{
			"ookla":      "speedtest.net server network (familiar numbers)",
			"cloudflare": "speed.cloudflare.com (no server selection, very reliable)",
		}
		for i, c := range backendChoices {
			cursor := "  "
			name := c
			if i == v.cursor {
				cursor = styleSelected.Render("> ")
				name = styleSelected.Render(name)
			}
			b.WriteString(fmt.Sprintf("%s%-12s %s\n", cursor, name, styleDim.Render(descs[c])))
		}
		b.WriteString("\n" + styleDim.Render("enter start · esc back"))
		return b.String()
	}

	backend := v.opts.Backend
	if backend == "" {
		backend = "ookla"
	}
	b.WriteString(styleDim.Render("backend: "+backend) + "\n\n")

	if v.err != nil {
		b.WriteString(renderErr(v.err) + "\n\n" + styleDim.Render("r retry · esc back"))
		return b.String()
	}

	if v.res != nil {
		r := v.res
		panel := fmt.Sprintf("%s  %s\n%s  %s\n\n%s  %s   %s  %s",
			styleDown.Render("↓ Download"), styleDown.Render(netutil.FormatBitRate(r.DownloadBps)),
			styleUp.Render("↑ Upload  "), styleUp.Render(netutil.FormatBitRate(r.UploadBps)),
			styleBold.Render("Latency"), netutil.FormatLatency(r.Latency),
			styleBold.Render("Jitter"), netutil.FormatLatency(r.Jitter))
		b.WriteString(stylePanel.Render(panel) + "\n\n")
		b.WriteString(styleDim.Render("server: "+r.Server) + "\n")
		if r.ISP != "" {
			b.WriteString(styleDim.Render(fmt.Sprintf("you: %s (%s)", r.ISP, r.ClientIP)) + "\n")
		}
		b.WriteString("\n" + styleDim.Render("r rerun · esc back"))
		return b.String()
	}

	// Running.
	phaseLabel := string(v.phase)
	if phaseLabel == "" {
		phaseLabel = "starting"
	}
	b.WriteString(v.spin.View() + " " + styleBold.Render(phaseLabel) + "\n")
	if v.statusMsg != "" {
		b.WriteString(styleDim.Render(v.statusMsg) + "\n")
	}
	b.WriteString("\n")
	if v.latency != "" {
		b.WriteString(fmt.Sprintf("%s %s\n", styleBold.Render("latency"), v.latency))
	}
	if v.down > 0 {
		b.WriteString(styleDown.Render(fmt.Sprintf("↓ %s", netutil.FormatBitRate(v.down))) + "\n")
	}
	if v.up > 0 {
		b.WriteString(styleUp.Render(fmt.Sprintf("↑ %s", netutil.FormatBitRate(v.up))) + "\n")
	}
	if v.progress >= 0 && (v.phase == speed.PhaseDownload || v.phase == speed.PhaseUpload) {
		b.WriteString("\n" + renderBar(v.progress, min(v.width-8, 50)) + "\n")
	}
	b.WriteString("\n" + styleDim.Render("esc cancel & back"))
	return b.String()
}
