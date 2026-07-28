package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CognitoBit/netpulse/internal/netutil"
	"github.com/CognitoBit/netpulse/internal/trace"
)

type mtrStartedMsg struct {
	ch     <-chan trace.Snapshot
	cancel context.CancelFunc
}
type mtrSnapMsg trace.Snapshot
type mtrClosedMsg struct{}

type mtrView struct {
	cfg    trace.Config
	prompt bool // asking for a hostname first (menu flow)
	input  textinput.Model

	cancel context.CancelFunc
	ch     <-chan trace.Snapshot
	snap   trace.Snapshot
	err    error
	done   bool
	width  int
}

func newMTRView(cfg trace.Config, prompt bool) *mtrView {
	ti := textinput.New()
	ti.Placeholder = "google.com"
	ti.CharLimit = 253
	ti.Width = 40
	ti.Focus()
	return &mtrView{cfg: cfg, prompt: prompt, input: ti, width: 80}
}

func (v *mtrView) Init() tea.Cmd {
	if v.prompt {
		return textinput.Blink
	}
	return v.start
}

func (v *mtrView) start() tea.Msg {
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := trace.Run(ctx, v.cfg)
	if err != nil {
		cancel()
		return errMsg{err}
	}
	return mtrStartedMsg{ch: ch, cancel: cancel}
}

func (v *mtrView) wait() tea.Cmd {
	return waitFor(v.ch, func(s trace.Snapshot) tea.Msg { return mtrSnapMsg(s) }, mtrClosedMsg{})
}

func (v *mtrView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width

	case mtrStartedMsg:
		v.ch = msg.ch
		v.cancel = msg.cancel
		return v, v.wait()

	case mtrSnapMsg:
		v.snap = trace.Snapshot(msg)
		if v.snap.Done {
			v.done = true
		}
		return v, v.wait()

	case mtrClosedMsg:
		v.done = true

	case errMsg:
		v.err = msg.err

	case tea.KeyMsg:
		if v.prompt {
			switch msg.String() {
			case "esc":
				return v, back
			case "enter":
				host := strings.TrimSpace(v.input.Value())
				if host == "" {
					host = v.input.Placeholder
				}
				v.cfg.Host = host
				v.prompt = false
				return v, v.start
			default:
				var cmd tea.Cmd
				v.input, cmd = v.input.Update(msg)
				return v, cmd
			}
		}
		switch msg.String() {
		case "esc", "q":
			v.stop()
			return v, back
		case "r":
			if v.err == nil && !v.done {
				break
			}
			v.stop()
			v.snap = trace.Snapshot{}
			v.done, v.err = false, nil
			if v.cfg.Host == "" {
				v.prompt = true
				return v, textinput.Blink
			}
			return v, v.start
		}
	}
	return v, nil
}

func (v *mtrView) stop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *mtrView) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("MTR") + "\n")

	if v.prompt {
		b.WriteString(styleDim.Render("traceroute with per-hop loss and latency") + "\n\n")
		b.WriteString("Host to trace:\n" + v.input.View() + "\n\n")
		b.WriteString(styleDim.Render("enter start · esc back"))
		return b.String()
	}

	target := v.snap.Target
	if target == "" {
		target = v.cfg.Host
	}
	sub := target
	if v.snap.TargetIP != "" && v.snap.TargetIP != target {
		sub += " (" + v.snap.TargetIP + ")"
	}
	b.WriteString(styleDim.Render(sub+fmt.Sprintf(" · cycle %d", v.snap.Cycles)) + "\n\n")

	if v.err != nil {
		b.WriteString(renderErr(v.err) + "\n\n" + styleDim.Render("r retry · esc back"))
		return b.String()
	}

	hostW := max(20, v.width-72)
	header := fmt.Sprintf("%3s  %-*s %7s %5s %9s %9s %9s %9s %9s",
		"HOP", hostW, "HOST", "LOSS", "SNT", "LAST", "AVG", "BEST", "WORST", "JITTER")
	b.WriteString(styleHeader.Render(header) + "\n")

	if len(v.snap.Hops) == 0 {
		b.WriteString(styleDim.Render("probing…") + "\n")
	}
	for _, h := range v.snap.Hops {
		name := h.Addr
		if h.Host != "" {
			name = h.Host
		}
		if name == "" {
			name = "???"
		}
		loss := "-"
		if h.Sent > 0 {
			loss = fmt.Sprintf("%5.1f%%", h.LossPct)
		}
		row := fmt.Sprintf("%3d  %-*s %7s %5d %9s %9s %9s %9s %9s",
			h.TTL, hostW, clip(name, hostW),
			lossStyle(h.LossPct).Render(loss), h.Sent,
			netutil.FormatLatency(h.Last), netutil.FormatLatency(h.Avg),
			netutil.FormatLatency(h.Best), netutil.FormatLatency(h.Worst),
			netutil.FormatLatency(h.Jitter))
		b.WriteString(row + "\n")
	}

	b.WriteString("\n" + styleDim.Render("high loss at a middle hop but 0% at the end is normal (routers deprioritize ICMP)") + "\n")
	if v.done {
		b.WriteString(styleGood.Render("done") + " " + styleDim.Render("· r rerun · esc back"))
	} else {
		b.WriteString(styleDim.Render("esc stop & back"))
	}
	return b.String()
}
