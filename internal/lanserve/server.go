// Package lanserve hosts a self-contained LAN speed test: an HTTP server with
// an embedded browser page so any device on the network can test its speed
// against this machine.
package lanserve

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/PLACEHOLDER/netpulse/internal/netutil"
)

// DefaultPort is tried first; if taken (and the user didn't pick a port
// explicitly), the OS assigns a free one.
const DefaultPort = 7333

// ReqEvent describes one completed client request, for the activity log.
type ReqEvent struct {
	Time     time.Time
	ClientIP string
	Path     string
	Bytes    int64
	Duration time.Duration
	RateBps  float64 // 0 for non-transfer endpoints
}

// Server is a running LAN speed test server.
type Server struct {
	URLs   []string        // http://<lan-ip>:<port> for every LAN interface
	Port   int             // actual bound port
	Events <-chan ReqEvent // completed-request activity feed (buffered, lossy)

	http *http.Server
}

// Start binds 0.0.0.0 and begins serving. portExplicit disables the
// random-port fallback so a user-chosen port fails loudly instead.
func Start(ctx context.Context, port int, portExplicit bool) (*Server, error) {
	ln, err := net.Listen("tcp4", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil && !portExplicit {
		ln, err = net.Listen("tcp4", "0.0.0.0:0")
	}
	if err != nil {
		return nil, fmt.Errorf("bind port %d: %w", port, err)
	}
	actualPort := ln.Addr().(*net.TCPAddr).Port

	events := make(chan ReqEvent, 64)
	mux := newMux(events)
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	s := &Server{Port: actualPort, Events: events, http: srv}
	for _, ip := range netutil.LanIPs() {
		s.URLs = append(s.URLs, fmt.Sprintf("http://%s:%d", ip, actualPort))
	}
	if len(s.URLs) == 0 {
		s.URLs = append(s.URLs, fmt.Sprintf("http://localhost:%d", actualPort))
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Surface fatal serve errors through the event channel.
			select {
			case events <- ReqEvent{Time: time.Now(), Path: "!error", ClientIP: err.Error()}:
			default:
			}
		}
	}()
	return s, nil
}
