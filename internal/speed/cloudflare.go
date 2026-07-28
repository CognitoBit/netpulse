package speed

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Cloudflare tests against speed.cloudflare.com using the same endpoints as
// Cloudflare's own web client: GET /__down?bytes=N and POST /__up.
type Cloudflare struct{}

const (
	cfBase          = "https://speed.cloudflare.com"
	cfLatencyProbes = 20
	cfDownChunk     = 25 << 20 // bytes per download request
	cfUpChunk       = 10 << 20 // bytes per upload request
	cfDownWindow    = 12 * time.Second
	cfUpWindow      = 10 * time.Second
	cfWarmup        = 2 * time.Second
)

func (c *Cloudflare) Name() string { return "cloudflare" }

func (c *Cloudflare) Run(ctx context.Context, events chan<- Event) (*Result, error) {
	client := &http.Client{Transport: &http.Transport{
		DisableCompression:  true,
		ForceAttemptHTTP2:   true,
		MaxIdleConnsPerHost: 8,
		TLSHandshakeTimeout: 10 * time.Second,
	}}

	emit(events, Event{Phase: PhaseConnect, Progress: -1, Msg: "contacting speed.cloudflare.com"})
	meta := c.fetchTrace(ctx, client)
	serverDesc := "Cloudflare"
	location := ""
	if meta != nil {
		location = strings.TrimSpace(strings.Join(nonEmpty(meta.Colo, meta.Country), ", "))
		serverDesc = "Cloudflare edge " + location
		emit(events, Event{Phase: PhaseConnect, Progress: -1, Msg: "server: " + serverDesc})
	}

	latency, jitter, err := c.latencyTest(ctx, client, events)
	if err != nil {
		return nil, fmt.Errorf("latency test: %w", err)
	}

	downBps, err := c.throughputTest(ctx, client, events, PhaseDownload)
	if err != nil {
		return nil, fmt.Errorf("download test: %w", err)
	}
	upBps, err := c.throughputTest(ctx, client, events, PhaseUpload)
	if err != nil {
		return nil, fmt.Errorf("upload test: %w", err)
	}

	res := &Result{
		Backend:     "cloudflare",
		Server:      serverDesc,
		Location:    location,
		Latency:     latency,
		Jitter:      jitter,
		DownloadBps: downBps,
		UploadBps:   upBps,
	}
	if meta != nil {
		res.ClientIP = meta.ClientIP
	}
	return res, nil
}

type cfMeta struct {
	ClientIP string
	Colo     string
	Country  string
}

// fetchTrace reads /cdn-cgi/trace (key=value lines) for client IP and the
// serving colo. The richer /meta endpoint 403s non-browser clients.
func (c *Cloudflare) fetchTrace(ctx context.Context, client *http.Client) *cfMeta {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfBase+"/cdn-cgi/trace", nil)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil
	}
	m := &cfMeta{}
	for _, line := range strings.Split(string(body), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch k {
		case "ip":
			m.ClientIP = v
		case "colo":
			m.Colo = v
		case "loc":
			m.Country = v
		}
	}
	return m
}

// latencyTest issues small sequential requests and measures time-to-response,
// subtracting the server's own processing time reported via Server-Timing
// (cfRequestDuration), the same correction Cloudflare's web client applies.
func (c *Cloudflare) latencyTest(ctx context.Context, client *http.Client, events chan<- Event) (latency, jitter time.Duration, err error) {
	samples := make([]time.Duration, 0, cfLatencyProbes)
	for i := 0; i < cfLatencyProbes; i++ {
		if ctx.Err() != nil {
			return 0, 0, ctx.Err()
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfBase+"/__down?bytes=0", nil)
		start := time.Now()
		resp, reqErr := client.Do(req)
		if reqErr != nil {
			if err == nil {
				err = reqErr
			}
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		rtt := time.Since(start) - serverDuration(resp.Header)
		if rtt < 0 {
			rtt = 0
		}
		// The first request pays for TCP+TLS setup; keep it out of the stats.
		if i == 0 {
			continue
		}
		samples = append(samples, rtt)
		emit(events, Event{Phase: PhaseLatency, Progress: float64(i+1) / cfLatencyProbes, Latency: rtt})
	}
	if len(samples) == 0 {
		if err == nil {
			err = fmt.Errorf("no successful latency probes")
		}
		return 0, 0, err
	}
	return median(samples), meanAbsDelta(samples), nil
}

// serverDuration sums every dur= metric in the Server-Timing header — the
// server-side processing time to subtract from measured TTFB. Cloudflare
// currently reports cfSpeedEdge and cfSpeedWorker (ms); older responses had
// cfRequestDuration. Either way the sum is what the wire didn't spend.
func serverDuration(h http.Header) time.Duration {
	var total time.Duration
	for _, v := range h.Values("Server-Timing") {
		for _, part := range strings.Split(v, ",") {
			for _, f := range strings.Split(part, ";") {
				f = strings.TrimSpace(f)
				if rest, ok := strings.CutPrefix(f, "dur="); ok {
					if ms, err := strconv.ParseFloat(rest, 64); err == nil {
						total += time.Duration(ms * float64(time.Millisecond))
					}
				}
			}
		}
	}
	return total
}

// throughputTest saturates the link with 4 parallel HTTP streams for a fixed
// window, samples a shared byte counter every 200ms, and reports the mean of
// post-warmup samples.
func (c *Cloudflare) throughputTest(ctx context.Context, client *http.Client, events chan<- Event, phase Phase) (float64, error) {
	window := cfDownWindow
	if phase == PhaseUpload {
		window = cfUpWindow
	}
	emit(events, Event{Phase: phase, Progress: 0, Msg: "testing " + string(phase)})

	runCtx, cancel := context.WithTimeout(ctx, window)
	defer cancel()

	var counter atomic.Int64
	var wg sync.WaitGroup
	var firstErr atomic.Value

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for runCtx.Err() == nil {
				var err error
				if phase == PhaseUpload {
					err = c.uploadOnce(runCtx, client, &counter)
				} else {
					err = c.downloadOnce(runCtx, client, &counter)
				}
				if err != nil && runCtx.Err() == nil {
					firstErr.CompareAndSwap(nil, err)
					// Brief pause so a persistent failure doesn't spin.
					select {
					case <-time.After(300 * time.Millisecond):
					case <-runCtx.Done():
					}
				}
			}
		}()
	}

	start := time.Now()
	var rates []float64
	var lastBytes int64
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
sampling:
	for {
		select {
		case <-runCtx.Done():
			break sampling
		case <-tick.C:
			elapsed := time.Since(start)
			cur := counter.Load()
			rate := float64(cur-lastBytes) * 8 / 0.2 // bits/sec over the tick
			lastBytes = cur
			if elapsed > cfWarmup {
				rates = append(rates, rate)
			}
			smoothed := rate
			if n := len(rates); n > 0 {
				smoothed = mean(rates[max(0, n-5):])
			}
			emit(events, Event{Phase: phase, Progress: min(1, elapsed.Seconds()/window.Seconds()), Rate: smoothed})
		}
	}
	cancel()
	wg.Wait()

	if len(rates) == 0 {
		if err, _ := firstErr.Load().(error); err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("no throughput samples collected")
	}
	return mean(rates), nil
}

func (c *Cloudflare) downloadOnce(ctx context.Context, client *http.Client, counter *atomic.Int64) error {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/__down?bytes=%d", cfBase, cfDownChunk), nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	buf := make([]byte, 128<<10)
	for {
		n, err := resp.Body.Read(buf)
		counter.Add(int64(n))
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// randomBlock is a 1MB pseudorandom payload reused across upload requests
// (random enough to defeat any transparent compression, cheap to generate).
var randomBlock = func() []byte {
	b := make([]byte, 1<<20)
	rng := rand.New(rand.NewSource(0x6e657470756c7365)) // "netpulse"
	rng.Read(b)
	return b
}()

func (c *Cloudflare) uploadOnce(ctx context.Context, client *http.Client, counter *atomic.Int64) error {
	body := &countingUploadReader{total: cfUpChunk, counter: counter}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, cfBase+"/__up", body)
	req.ContentLength = cfUpChunk
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// countingUploadReader serves total bytes from randomBlock, counting as read.
type countingUploadReader struct {
	total   int64
	served  int64
	counter *atomic.Int64
}

func (r *countingUploadReader) Read(p []byte) (int, error) {
	if r.served >= r.total {
		return 0, io.EOF
	}
	off := int(r.served % int64(len(randomBlock)))
	n := copy(p, randomBlock[off:])
	if rem := r.total - r.served; int64(n) > rem {
		n = int(rem)
	}
	r.served += int64(n)
	r.counter.Add(int64(n))
	return n, nil
}

func median(d []time.Duration) time.Duration {
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

func meanAbsDelta(d []time.Duration) time.Duration {
	if len(d) < 2 {
		return 0
	}
	var sum time.Duration
	for i := 1; i < len(d); i++ {
		delta := d[i] - d[i-1]
		if delta < 0 {
			delta = -delta
		}
		sum += delta
	}
	return sum / time.Duration(len(d)-1)
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
