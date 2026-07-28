// Package trace implements an mtr-style combined traceroute+ping: TTL-limited
// ICMP echoes with per-hop loss and latency statistics, updated continuously.
package trace

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/CognitoBit/netpulse/internal/netutil"
)

// Config controls an MTR session.
type Config struct {
	Host     string
	MaxHops  int           // default 30
	Interval time.Duration // time between cycles, default 1s
	Timeout  time.Duration // per-probe reply deadline, default 800ms
	Cycles   int           // 0 = run until ctx is cancelled
	NoDNS    bool          // skip reverse DNS lookups
}

// DefaultConfig returns mtr-like defaults for host.
func DefaultConfig(host string) Config {
	return Config{Host: host, MaxHops: 30, Interval: time.Second, Timeout: 800 * time.Millisecond}
}

// Hop is one row of the MTR table.
type Hop struct {
	TTL     int           `json:"ttl"`
	Addr    string        `json:"addr"`
	Host    string        `json:"host,omitempty"`
	Sent    int           `json:"sent"`
	Recv    int           `json:"received"`
	LossPct float64       `json:"loss_pct"`
	Last    time.Duration `json:"last_ns"`
	Avg     time.Duration `json:"avg_ns"`
	Best    time.Duration `json:"best_ns"`
	Worst   time.Duration `json:"worst_ns"`
	Jitter  time.Duration `json:"jitter_ns"`
}

// Snapshot is the periodically emitted state of the whole session.
type Snapshot struct {
	Target   string `json:"target"`
	TargetIP string `json:"target_ip"`
	Hops     []Hop  `json:"hops"`
	Cycles   int    `json:"cycles"`
	Done     bool   `json:"done"`
}

type hopState struct {
	addr     net.IP
	sent     int
	recv     int
	timedOut int
	last     time.Duration
	rtt      netutil.RTTStats
	jit      netutil.JitterTracker
}

type pendingProbe struct {
	ttl    int
	sentAt time.Time
}

// Run resolves cfg.Host, opens the ICMP socket and starts probing. Snapshots
// arrive roughly every 250ms; the last one has Done=true, then the channel
// closes. Errors opening the socket surface immediately (possibly as a
// *netutil.PrivilegeError with remediation text).
func Run(ctx context.Context, cfg Config) (<-chan Snapshot, error) {
	if cfg.MaxHops <= 0 {
		cfg.MaxHops = 30
	}
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 800 * time.Millisecond
	}

	ipaddr, err := net.ResolveIPAddr("ip4", cfg.Host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", cfg.Host, err)
	}

	strategy, err := detectStrategy()
	if err != nil {
		return nil, err
	}
	pr, err := newProber(strategy)
	if err != nil {
		return nil, err
	}

	out := make(chan Snapshot, 8)
	go runSession(ctx, cfg, pr, ipaddr.IP, out)
	return out, nil
}

type session struct {
	cfg     cfg
	pr      *prober
	dst     net.IP
	hops    []*hopState // index = ttl
	pending map[int]pendingProbe
	seq     int
	pathLen int
	cycles  int

	dns   map[string]string
	dnsMu sync.Mutex
	inDNS map[string]bool
}

// cfg aliases Config to keep the session struct literal short.
type cfg = Config

func runSession(ctx context.Context, c Config, pr *prober, dst net.IP, out chan<- Snapshot) {
	defer close(out)
	defer pr.close()

	s := &session{
		cfg:     c,
		pr:      pr,
		dst:     dst,
		hops:    make([]*hopState, c.MaxHops+1),
		pending: map[int]pendingProbe{},
		seq:     1,
		pathLen: c.MaxHops,
		dns:     map[string]string{},
		inDNS:   map[string]bool{},
	}

	replies := make(chan reply, 64)
	recvCtx, stopRecv := context.WithCancel(ctx)
	defer stopRecv()
	go pr.receiveLoop(recvCtx, dst, replies)

	// sendQueue holds the TTLs still to send this cycle; probes are paced
	// 15ms apart rather than burst.
	var sendQueue []int
	pace := time.NewTicker(15 * time.Millisecond)
	defer pace.Stop()

	cycle := time.NewTicker(c.Interval)
	defer cycle.Stop()
	house := time.NewTicker(250 * time.Millisecond)
	defer house.Stop()

	var finishAt time.Time // set once the last cycle has been sent
	sendQueue = s.startCycle()

	for {
		select {
		case <-ctx.Done():
			s.sweep(time.Now())
			out <- s.snapshot(true)
			return

		case r := <-replies:
			s.handleReply(r)

		case <-pace.C:
			if len(sendQueue) > 0 {
				ttl := sendQueue[0]
				sendQueue = sendQueue[1:]
				s.sendProbe(ttl)
			}

		case <-house.C:
			now := time.Now()
			s.sweep(now)
			select {
			case out <- s.snapshot(false):
			default: // never block the engine on a slow consumer
			}
			if !finishAt.IsZero() && now.After(finishAt) {
				out <- s.snapshot(true)
				return
			}

		case <-cycle.C:
			if !finishAt.IsZero() {
				continue
			}
			s.cycles++
			if c.Cycles > 0 && s.cycles >= c.Cycles {
				// Last cycle already sent; allow its probes to resolve.
				finishAt = time.Now().Add(c.Timeout + 300*time.Millisecond)
				continue
			}
			sendQueue = s.startCycle()
		}
	}
}

func (s *session) startCycle() []int {
	ttls := make([]int, 0, s.pathLen)
	for ttl := 1; ttl <= s.pathLen; ttl++ {
		ttls = append(ttls, ttl)
	}
	return ttls
}

func (s *session) sendProbe(ttl int) {
	if ttl > s.pathLen {
		return // path shrank while queued
	}
	seq := s.seq & 0xffff
	s.seq++
	sentAt, err := s.pr.send(s.dst, ttl, seq)
	if err != nil {
		return
	}
	if s.hops[ttl] == nil {
		s.hops[ttl] = &hopState{}
	}
	s.hops[ttl].sent++
	s.pending[seq] = pendingProbe{ttl: ttl, sentAt: sentAt}
}

func (s *session) handleReply(r reply) {
	p, ok := s.pending[r.seq]
	if !ok {
		return // late (already swept) or foreign packet
	}
	delete(s.pending, r.seq)

	h := s.hops[p.ttl]
	if h == nil {
		return
	}
	rtt := r.at.Sub(p.sentAt)
	h.recv++
	h.last = rtt
	h.rtt.Add(rtt)
	h.jit.Add(rtt)
	if r.from != nil {
		h.addr = r.from
		s.resolveAsync(r.from.String())
	}

	if r.final && p.ttl < s.pathLen {
		s.pathLen = p.ttl
	}
}

func (s *session) sweep(now time.Time) {
	for seq, p := range s.pending {
		if now.Sub(p.sentAt) > s.cfg.Timeout {
			delete(s.pending, seq)
			if h := s.hops[p.ttl]; h != nil {
				h.timedOut++
			}
		}
	}
}

func (s *session) resolveAsync(addr string) {
	if s.cfg.NoDNS {
		return
	}
	s.dnsMu.Lock()
	defer s.dnsMu.Unlock()
	if _, done := s.dns[addr]; done || s.inDNS[addr] {
		return
	}
	s.inDNS[addr] = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		names, _ := net.DefaultResolver.LookupAddr(ctx, addr)
		name := ""
		if len(names) > 0 {
			name = names[0]
			if len(name) > 0 && name[len(name)-1] == '.' {
				name = name[:len(name)-1]
			}
		}
		s.dnsMu.Lock()
		s.dns[addr] = name
		delete(s.inDNS, addr)
		s.dnsMu.Unlock()
	}()
}

func (s *session) snapshot(done bool) Snapshot {
	snap := Snapshot{
		Target:   s.cfg.Host,
		TargetIP: s.dst.String(),
		Cycles:   s.cycles,
		Done:     done,
	}
	deepest := 0
	for ttl := 1; ttl <= s.pathLen; ttl++ {
		if s.hops[ttl] != nil && s.hops[ttl].sent > 0 {
			deepest = ttl
		}
	}
	for ttl := 1; ttl <= deepest; ttl++ {
		h := s.hops[ttl]
		hop := Hop{TTL: ttl}
		if h != nil {
			if h.addr != nil {
				hop.Addr = h.addr.String()
				s.dnsMu.Lock()
				hop.Host = s.dns[hop.Addr]
				s.dnsMu.Unlock()
			}
			hop.Sent = h.sent
			hop.Recv = h.recv
			if resolved := h.recv + h.timedOut; resolved > 0 {
				hop.LossPct = 100 * float64(h.timedOut) / float64(resolved)
			}
			hop.Last = h.last
			hop.Avg = h.rtt.Avg()
			hop.Best = h.rtt.Min
			hop.Worst = h.rtt.Max
			hop.Jitter = h.jit.Value()
		}
		snap.Hops = append(snap.Hops, hop)
	}
	return snap
}
