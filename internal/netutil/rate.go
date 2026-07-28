package netutil

import (
	"fmt"
	"time"
)

// FormatBitRate renders a bits-per-second figure as a human string, e.g.
// "934.2 Mbps". Input is bits/sec.
func FormatBitRate(bps float64) string {
	switch {
	case bps >= 1e9:
		return fmt.Sprintf("%.2f Gbps", bps/1e9)
	case bps >= 1e6:
		return fmt.Sprintf("%.1f Mbps", bps/1e6)
	case bps >= 1e3:
		return fmt.Sprintf("%.1f Kbps", bps/1e3)
	default:
		return fmt.Sprintf("%.0f bps", bps)
	}
}

// FormatBytes renders a byte count as a human string, e.g. "25.0 MB".
func FormatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// FormatLatency renders a duration with millisecond precision suited to
// network RTTs, e.g. "23.4 ms".
func FormatLatency(d time.Duration) string {
	if d <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f ms", float64(d.Microseconds())/1000)
}

// JitterTracker computes smoothed inter-arrival jitter in the style of
// RFC 3550: j += (|delta| - j) / 16, where delta is the difference between
// consecutive RTT samples.
type JitterTracker struct {
	last   time.Duration
	jitter float64 // nanoseconds
	n      int
}

// Add feeds one RTT sample.
func (j *JitterTracker) Add(rtt time.Duration) {
	if j.n > 0 {
		delta := float64(rtt - j.last)
		if delta < 0 {
			delta = -delta
		}
		j.jitter += (delta - j.jitter) / 16
	}
	j.last = rtt
	j.n++
}

// Value returns the current smoothed jitter.
func (j *JitterTracker) Value() time.Duration {
	return time.Duration(j.jitter)
}

// RTTStats accumulates min/avg/max over RTT samples without storing them.
type RTTStats struct {
	Min, Max time.Duration
	sum      time.Duration
	N        int
}

// Add feeds one RTT sample.
func (s *RTTStats) Add(rtt time.Duration) {
	if s.N == 0 || rtt < s.Min {
		s.Min = rtt
	}
	if rtt > s.Max {
		s.Max = rtt
	}
	s.sum += rtt
	s.N++
}

// Avg returns the mean of all samples, or 0 if none.
func (s *RTTStats) Avg() time.Duration {
	if s.N == 0 {
		return 0
	}
	return s.sum / time.Duration(s.N)
}
