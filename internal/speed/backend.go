// Package speed implements internet speed tests behind a common Backend
// interface, with two implementations: Ookla (speedtest.net) and Cloudflare
// (speed.cloudflare.com).
package speed

import (
	"context"
	"time"
)

// Phase identifies which part of the test an Event belongs to.
type Phase string

const (
	PhaseConnect  Phase = "connecting"
	PhaseLatency  Phase = "latency"
	PhaseDownload Phase = "download"
	PhaseUpload   Phase = "upload"
)

// Event is a live progress update streamed to the UI during a run.
type Event struct {
	Phase    Phase
	Progress float64       // 0..1 within the phase, -1 if indeterminate
	Rate     float64       // bits/sec, current smoothed rate (download/upload)
	Latency  time.Duration // filled during PhaseLatency
	Msg      string        // human status line, e.g. server being used
}

// Result is the final outcome of a speed test.
type Result struct {
	Backend     string        `json:"backend"`
	Server      string        `json:"server"`
	Location    string        `json:"location"`
	ISP         string        `json:"isp,omitempty"`
	ClientIP    string        `json:"client_ip,omitempty"`
	Latency     time.Duration `json:"latency_ns"`
	Jitter      time.Duration `json:"jitter_ns"`
	DownloadBps float64       `json:"download_bps"`
	UploadBps   float64       `json:"upload_bps"`
}

// Backend runs a full speed test, streaming progress events (throttled to
// ~10/sec) into events. Implementations must not close the events channel.
type Backend interface {
	Name() string
	Run(ctx context.Context, events chan<- Event) (*Result, error)
}

// New returns the backend for name ("ookla" or "cloudflare").
func New(name string, serverID int) (Backend, bool) {
	switch name {
	case "ookla", "":
		return &Ookla{ServerID: serverID}, true
	case "cloudflare", "cf":
		return &Cloudflare{}, true
	}
	return nil, false
}

// emit sends without blocking; the UI keeps up via a buffered channel, and
// dropping a progress tick is harmless.
func emit(ch chan<- Event, ev Event) {
	select {
	case ch <- ev:
	default:
	}
}
