package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CognitoBit/netpulse/internal/lanserve"
	"github.com/CognitoBit/netpulse/internal/netutil"
)

type serveStartedMsg struct {
	srv    *lanserve.Server
	cancel context.CancelFunc
}
type serveEventMsg lanserve.ReqEvent
type serveStoppedMsg struct{}

const serveLogLines = 10

type serveView struct {
	port     int
	explicit bool

	srv    *lanserve.Server
	cancel context.CancelFunc

	log        []lanserve.ReqEvent
	totalBytes int64
	tests      int
	err        error
	width      int
}

func newServeView(port int, explicit bool) *serveView {
	if port == 0 {
		port = lanserve.DefaultPort
	}
	return &serveView{port: port, explicit: explicit, width: 80}
}

func (v *serveView) Init() tea.Cmd { return v.start }

func (v *serveView) start() tea.Msg {
	ctx, cancel := context.WithCancel(context.Background())
	srv, err := lanserve.Start(ctx, v.port, v.explicit)
	if err != nil {
		cancel()
		return errMsg{err}
	}
	return serveStartedMsg{srv: srv, cancel: cancel}
}

func (v *serveView) wait() tea.Cmd {
	return waitFor(v.srv.Events, func(e lanserve.ReqEvent) tea.Msg { return serveEventMsg(e) }, serveStoppedMsg{})
}

func (v *serveView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width

	case serveStartedMsg:
		v.srv = msg.srv
		v.cancel = msg.cancel
		return v, v.wait()

	case serveEventMsg:
		e := lanserve.ReqEvent(msg)
		v.log = append(v.log, e)
		if len(v.log) > serveLogLines {
			v.log = v.log[len(v.log)-serveLogLines:]
		}
		v.totalBytes += e.Bytes
		if e.Path == "/" {
			v.tests++
		}
		return v, v.wait()

	case serveStoppedMsg:
		return v, nil

	case errMsg:
		v.err = msg.err

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			v.stop()
			return v, back
		}
	}
	return v, nil
}

func (v *serveView) stop() {
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
}

func (v *serveView) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("LAN Speed Server") + "\n\n")

	if v.err != nil {
		b.WriteString(renderErr(v.err) + "\n\n" + styleDim.Render("esc back"))
		return b.String()
	}
	if v.srv == nil {
		b.WriteString(styleDim.Render("starting…"))
		return b.String()
	}

	b.WriteString("Open on any device on this network:\n")
	for _, u := range v.srv.URLs {
		b.WriteString("  " + styleGood.Render(u) + "\n")
	}
	b.WriteString("\n" + styleDim.Render("If other devices can't connect, allow netpulse through the firewall (Private networks).") + "\n\n")

	b.WriteString(fmt.Sprintf("%s %d visits · %s served\n\n",
		styleBold.Render("activity:"), v.tests, netutil.FormatBytes(v.totalBytes)))

	if len(v.log) == 0 {
		b.WriteString(styleDim.Render("waiting for clients…") + "\n")
	} else {
		header := fmt.Sprintf("%-9s %-16s %-10s %10s %11s", "TIME", "CLIENT", "PATH", "SIZE", "RATE")
		b.WriteString(styleHeader.Render(header) + "\n")
		for _, e := range v.log {
			rate := "-"
			if e.RateBps > 0 {
				rate = netutil.FormatBitRate(e.RateBps)
			}
			b.WriteString(fmt.Sprintf("%-9s %-16s %-10s %10s %11s\n",
				e.Time.Format("15:04:05"), clip(e.ClientIP, 16), clip(e.Path, 10),
				netutil.FormatBytes(e.Bytes), rate))
		}
	}

	b.WriteString("\n" + styleDim.Render("esc stop server & back"))
	return b.String()
}
