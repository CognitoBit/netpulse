// Package pingtest implements an ICMP burst packet-loss test in the spirit of
// packetlosstest.com: fixed-rate pings to one or more targets, reporting loss
// percentage, latency distribution and jitter.
package pingtest

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"

	"github.com/CognitoBit/netpulse/internal/netutil"
)

// Config controls a loss test run.
type Config struct {
	Targets  []string      // hostnames or IPs
	Interval time.Duration // time between pings per target
	Size     int           // ICMP payload bytes
	Duration time.Duration // total test length (ignored if Count > 0)
	Count    int           // exact number of pings per target (0 = use Duration)
}

// DefaultConfig mirrors packetlosstest.com's defaults: 10 pings/sec for 30s.
func DefaultConfig() Config {
	return Config{
		Targets:  []string{"8.8.8.8", "1.1.1.1"},
		Interval: 100 * time.Millisecond,
		Size:     56,
		Duration: 30 * time.Second,
	}
}

// TargetStats is a point-in-time snapshot of one target's counters.
type TargetStats struct {
	Target   string        `json:"target"`
	Addr     string        `json:"resolved_addr"`
	Sent     int           `json:"sent"`
	Recv     int           `json:"received"`
	Dup      int           `json:"duplicates"`
	LossPct  float64       `json:"loss_pct"`
	Min      time.Duration `json:"min_rtt_ns"`
	Avg      time.Duration `json:"avg_rtt_ns"`
	Max      time.Duration `json:"max_rtt_ns"`
	Jitter   time.Duration `json:"jitter_ns"`
	LastRTT  time.Duration `json:"last_rtt_ns"`
	Err      string        `json:"error,omitempty"`
	Finished bool          `json:"finished"`
}

// Snapshot is the engine's periodic emission: all targets plus progress.
type Snapshot struct {
	Targets  []TargetStats `json:"targets"`
	Elapsed  time.Duration `json:"elapsed_ns"`
	Total    time.Duration `json:"total_ns"` // 0 when count-based
	Done     bool          `json:"done"`
	FatalErr string        `json:"error,omitempty"`
}

type targetState struct {
	mu     sync.Mutex
	stats  TargetStats
	rtt    netutil.RTTStats
	jit    netutil.JitterTracker
	pinger *probing.Pinger
}

// Run starts the loss test and returns a channel of snapshots emitted roughly
// every 200ms; the final snapshot has Done=true and the channel is then
// closed. Cancel ctx to stop early (a final snapshot is still delivered).
func Run(ctx context.Context, cfg Config) (<-chan Snapshot, error) {
	if len(cfg.Targets) == 0 {
		return nil, fmt.Errorf("no targets given")
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 100 * time.Millisecond
	}
	// pro-bing needs room for its tracking UUID + timestamp in the payload.
	if cfg.Size < 24 {
		cfg.Size = 24
	}

	states := make([]*targetState, len(cfg.Targets))
	for i, t := range cfg.Targets {
		p, err := newPinger(t, cfg)
		if err != nil {
			return nil, fmt.Errorf("target %s: %w", t, err)
		}
		st := &targetState{pinger: p, stats: TargetStats{Target: t}}
		wirePinger(p, st)
		states[i] = st
	}

	out := make(chan Snapshot, 8)
	go run(ctx, cfg, states, out)
	return out, nil
}

func newPinger(target string, cfg Config) (*probing.Pinger, error) {
	p, err := probing.NewPinger(target)
	if err != nil {
		return nil, err
	}
	p.Interval = cfg.Interval
	p.Size = cfg.Size
	p.RecordRtts = false
	p.Timeout = timeoutFor(cfg)
	if cfg.Count > 0 {
		p.Count = cfg.Count
	}
	// Windows has no unprivileged ICMP datagram sockets; raw sockets work
	// without elevation there. Elsewhere start unprivileged (macOS always
	// works; Linux depends on net.ipv4.ping_group_range).
	p.SetPrivileged(runtime.GOOS == "windows")
	return p, nil
}

func timeoutFor(cfg Config) time.Duration {
	if cfg.Count > 0 {
		// Count mode: enough time for all pings plus slack for stragglers.
		return time.Duration(cfg.Count)*cfg.Interval + 5*time.Second
	}
	return cfg.Duration + 2*time.Second
}

func wirePinger(p *probing.Pinger, st *targetState) {
	p.OnRecv = func(pkt *probing.Packet) {
		st.mu.Lock()
		st.stats.Recv++
		st.stats.LastRTT = pkt.Rtt
		st.rtt.Add(pkt.Rtt)
		st.jit.Add(pkt.Rtt)
		st.mu.Unlock()
	}
	p.OnDuplicateRecv = func(pkt *probing.Packet) {
		st.mu.Lock()
		st.stats.Dup++
		st.mu.Unlock()
	}
}

func run(ctx context.Context, cfg Config, states []*targetState, out chan<- Snapshot) {
	defer close(out)
	start := time.Now()

	total := cfg.Duration
	if cfg.Count > 0 {
		total = 0
	}

	runCtx, cancel := context.WithCancel(ctx)
	if cfg.Count == 0 {
		runCtx, cancel = context.WithTimeout(ctx, cfg.Duration)
	}
	defer cancel()

	var wg sync.WaitGroup
	for _, st := range states {
		wg.Add(1)
		go func(st *targetState) {
			defer wg.Done()
			err := runPinger(runCtx, st.pinger)
			st.mu.Lock()
			if err != nil && ctx.Err() == nil && runCtx.Err() == nil {
				st.stats.Err = err.Error()
			}
			st.stats.Finished = true
			st.mu.Unlock()
		}(st)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			out <- snapshot(states, time.Since(start), total, false)
		case <-done:
			out <- snapshot(states, time.Since(start), total, true)
			return
		case <-ctx.Done():
			// Stop pingers, then wait for them to flush and emit the final.
			for _, st := range states {
				st.pinger.Stop()
			}
			<-done
			out <- snapshot(states, time.Since(start), total, true)
			return
		}
	}
}

// runPinger runs a pro-bing pinger under ctx. On Linux an unprivileged run
// can fail with a permission error; retry once in privileged (raw socket)
// mode before reporting the failure.
func runPinger(ctx context.Context, p *probing.Pinger) error {
	err := p.RunWithContext(ctx)
	if err != nil && !p.Privileged() && netutil.IsPermissionErr(err) {
		p.SetPrivileged(true)
		if err2 := p.RunWithContext(ctx); err2 == nil {
			return nil
		}
		return &netutil.PrivilegeError{Underlying: err}
	}
	return err
}

func snapshot(states []*targetState, elapsed, total time.Duration, done bool) Snapshot {
	snap := Snapshot{Elapsed: elapsed, Total: total, Done: done}
	for _, st := range states {
		st.mu.Lock()
		s := st.stats
		s.Sent = st.pinger.PacketsSent
		if addr := st.pinger.IPAddr(); addr != nil {
			s.Addr = addr.String()
		}
		if s.Sent > 0 {
			s.LossPct = 100 * float64(s.Sent-s.Recv) / float64(s.Sent)
			if s.LossPct < 0 {
				s.LossPct = 0
			}
		}
		s.Min, s.Avg, s.Max = st.rtt.Min, st.rtt.Avg(), st.rtt.Max
		s.Jitter = st.jit.Value()
		st.mu.Unlock()
		snap.Targets = append(snap.Targets, s)
	}
	sort.SliceStable(snap.Targets, func(i, j int) bool { return snap.Targets[i].Target < snap.Targets[j].Target })
	return snap
}
