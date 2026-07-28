package lanserve

import (
	_ "embed"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"time"
)

//go:embed web/index.html
var indexHTML []byte

const (
	defaultChunk = 25 << 20 // /download default when ?bytes absent
	maxChunk     = 1 << 30  // /download upper bound
)

// payload is a 4MB pseudorandom block streamed repeatedly by /download —
// incompressible so middleboxes can't inflate the measured rate.
var payload = func() []byte {
	b := make([]byte, 4<<20)
	rng := rand.New(rand.NewSource(0x6c616e7365727665)) // "lanserve"
	rng.Read(b)
	return b
}()

func newMux(events chan<- ReqEvent) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
		report(events, r, "/", int64(len(indexHTML)), 0)
	})
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		noStore(w)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /download", func(w http.ResponseWriter, r *http.Request) {
		n := int64(defaultChunk)
		if v := r.URL.Query().Get("bytes"); v != "" {
			if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
				n = min(parsed, maxChunk)
			}
		}
		noStore(w)
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.FormatInt(n, 10))
		start := time.Now()
		var sent int64
		for sent < n {
			chunk := payload
			if rem := n - sent; rem < int64(len(chunk)) {
				chunk = chunk[:rem]
			}
			wrote, err := w.Write(chunk)
			sent += int64(wrote)
			if err != nil {
				break
			}
		}
		report(events, r, "/download", sent, time.Since(start))
	})
	mux.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		n, _ := io.Copy(io.Discard, r.Body)
		noStore(w)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(strconv.FormatInt(n, 10)))
		report(events, r, "/upload", n, time.Since(start))
	})
	return mux
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	// The page and endpoints are same-origin, but some browsers preflight
	// anyway when the page is opened via a different LAN IP than it fetches.
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func report(events chan<- ReqEvent, r *http.Request, path string, bytes int64, d time.Duration) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ev := ReqEvent{Time: time.Now(), ClientIP: host, Path: path, Bytes: bytes, Duration: d}
	if d > 0 && bytes > 0 {
		ev.RateBps = float64(bytes) * 8 / d.Seconds()
	}
	select {
	case events <- ev:
	default: // activity log is best-effort
	}
}
