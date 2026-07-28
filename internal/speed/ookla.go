package speed

import (
	"context"
	"fmt"
	"time"

	"github.com/showwin/speedtest-go/speedtest"
)

// Ookla tests against the speedtest.net server network.
type Ookla struct {
	ServerID int // 0 = auto-select nearest
}

func (o *Ookla) Name() string { return "ookla" }

func (o *Ookla) Run(ctx context.Context, events chan<- Event) (*Result, error) {
	client := speedtest.New()

	emit(events, Event{Phase: PhaseConnect, Progress: -1, Msg: "fetching speedtest.net server list"})
	user, _ := client.FetchUserInfoContext(ctx)

	servers, err := client.FetchServerListContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch speedtest.net servers (try --backend cloudflare): %w", err)
	}
	var ids []int
	if o.ServerID > 0 {
		ids = []int{o.ServerID}
	}
	targets, err := servers.FindServer(ids)
	if err != nil || len(targets) == 0 {
		return nil, fmt.Errorf("no suitable speedtest.net server (try --backend cloudflare): %w", err)
	}
	srv := targets[0]
	serverDesc := fmt.Sprintf("%s — %s (%s)", srv.Sponsor, srv.Name, srv.Country)
	emit(events, Event{Phase: PhaseConnect, Progress: -1, Msg: "server: " + serverDesc})

	emit(events, Event{Phase: PhaseLatency, Progress: -1, Msg: "measuring latency"})
	if err := srv.PingTestContext(ctx, func(latency time.Duration) {
		emit(events, Event{Phase: PhaseLatency, Progress: -1, Latency: latency})
	}); err != nil {
		return nil, fmt.Errorf("ping test: %w", err)
	}

	throttle := newThrottle(100 * time.Millisecond)
	client.SetCallbackDownload(func(rate speedtest.ByteRate) {
		if throttle.ok() {
			emit(events, Event{Phase: PhaseDownload, Progress: -1, Rate: float64(rate) * 8})
		}
	})
	emit(events, Event{Phase: PhaseDownload, Progress: -1, Msg: "testing download"})
	if err := srv.DownloadTestContext(ctx); err != nil {
		return nil, fmt.Errorf("download test: %w", err)
	}

	client.SetCallbackUpload(func(rate speedtest.ByteRate) {
		if throttle.ok() {
			emit(events, Event{Phase: PhaseUpload, Progress: -1, Rate: float64(rate) * 8})
		}
	})
	emit(events, Event{Phase: PhaseUpload, Progress: -1, Msg: "testing upload"})
	if err := srv.UploadTestContext(ctx); err != nil {
		return nil, fmt.Errorf("upload test: %w", err)
	}

	res := &Result{
		Backend:     "ookla",
		Server:      serverDesc,
		Location:    fmt.Sprintf("%s, %s", srv.Name, srv.Country),
		Latency:     srv.Latency,
		Jitter:      srv.Jitter,
		DownloadBps: float64(srv.DLSpeed) * 8,
		UploadBps:   float64(srv.ULSpeed) * 8,
	}
	if user != nil {
		res.ISP = user.Isp
		res.ClientIP = user.IP
	}
	return res, nil
}

// throttle rate-limits progress callbacks that fire per-chunk.
type throttle struct {
	min  time.Duration
	last time.Time
}

func newThrottle(min time.Duration) *throttle { return &throttle{min: min} }

func (t *throttle) ok() bool {
	now := time.Now()
	if now.Sub(t.last) < t.min {
		return false
	}
	t.last = now
	return true
}
