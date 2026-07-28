package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CognitoBit/netpulse/internal/netutil"
	"github.com/CognitoBit/netpulse/internal/pingtest"
)

type lossStartedMsg struct {
	ch     <-chan pingtest.Snapshot
	cancel context.CancelFunc
}
type lossSnapMsg pingtest.Snapshot
type lossClosedMsg struct{}

type lossView struct {
	cfg    pingtest.Config
	cancel context.CancelFunc
	ch     <-chan pingtest.Snapshot
	snap   pingtest.Snapshot
	err    error
	done   bool
	width  int
}

func newLossView(cfg pingtest.Config) *lossView {
	return &lossView{cfg: cfg, width: 80}
}

func (v *lossView) Init() tea.Cmd { return v.start }

func (v *lossView) start() tea.Msg {
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := pingtest.Run(ctx, v.cfg)
	if err != nil {
		cancel()
		return errMsg{err}
	}
	return lossStartedMsg{ch: ch, cancel: cancel}
}

func (v *lossView) wait() tea.Cmd {
	return waitFor(v.ch, func(s pingtest.Snapshot) tea.Msg { return lossSnapMsg(s) }, lossClosedMsg{})
}

func (v *lossView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
	case lossStartedMsg:
		v.ch = msg.ch
		v.cancel = msg.cancel
		return v, v.wait()
	case lossSnapMsg:
		v.snap = pingtest.Snapshot(msg)
		if v.snap.Done {
			v.done = true
		}
		return v, v.wait()
	case lossClosedMsg:
		v.done = true
	case errMsg:
		v.err = msg.err
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			v.stop()
			return v, back
		case "r":
			if v.done || v.err != nil {
				v.stop()
				v.snap = pingtest.Snapshot{}
				v.done, v.err = false, nil
				return v, v.start
			}
		}
	}
	return v, nil
}

func (v *lossView) stop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *lossView) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("Packet Loss Test") + "\n")
	b.WriteString(styleDim.Render(fmt.Sprintf("%d ms interval · %d byte payload", v.cfg.Interval.Milliseconds(), v.cfg.Size)) + "\n\n")

	if v.err != nil {
		b.WriteString(renderErr(v.err) + "\n\n" + styleDim.Render("r retry · esc back"))
		return b.String()
	}

	if v.snap.Total > 0 {
		frac := float64(v.snap.Elapsed) / float64(v.snap.Total)
		b.WriteString(renderBar(frac, min(v.width-8, 60)) +
			styleDim.Render(fmt.Sprintf("  %ds/%ds", int(v.snap.Elapsed.Seconds()), int(v.snap.Total.Seconds()))) + "\n\n")
	}

	header := fmt.Sprintf("%-16s %-16s %5s %5s %8s %9s %9s %9s %9s %9s",
		"TARGET", "ADDR", "SENT", "RECV", "LOSS", "LAST", "AVG", "BEST", "WORST", "JITTER")
	b.WriteString(styleHeader.Render(header) + "\n")
	for _, t := range v.snap.Targets {
		loss := fmt.Sprintf("%6.1f%%", t.LossPct)
		row := fmt.Sprintf("%-16s %-16s %5d %5d %8s %9s %9s %9s %9s %9s",
			clip(t.Target, 16), clip(t.Addr, 16), t.Sent, t.Recv,
			lossStyle(t.LossPct).Render(loss),
			netutil.FormatLatency(t.LastRTT), netutil.FormatLatency(t.Avg),
			netutil.FormatLatency(t.Min), netutil.FormatLatency(t.Max),
			netutil.FormatLatency(t.Jitter))
		b.WriteString(row + "\n")
		if t.Err != "" {
			b.WriteString("  " + styleBad.Render(clip(t.Err, v.width-4)) + "\n")
		}
	}

	b.WriteString("\n")
	if v.done {
		b.WriteString(styleGood.Render("test complete") + "\n" + styleDim.Render("r rerun · esc back"))
	} else {
		b.WriteString(styleDim.Render("esc stop & back"))
	}
	return b.String()
}

// renderErr shows privilege errors with their remediation text.
func renderErr(err error) string {
	var priv *netutil.PrivilegeError
	if errors.As(err, &priv) {
		return styleErrBox.Render(styleBad.Render(priv.Error()) + "\n\n" + priv.Remediation())
	}
	return styleErrBox.Render(styleBad.Render(err.Error()))
}

func renderBar(frac float64, width int) string {
	if width < 10 {
		width = 10
	}
	frac = max(0, min(1, frac))
	filled := int(frac * float64(width))
	return styleGood.Render(strings.Repeat("█", filled)) + styleDim.Render(strings.Repeat("░", width-filled))
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
