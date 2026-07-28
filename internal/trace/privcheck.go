package trace

import (
	"runtime"

	"golang.org/x/net/icmp"

	"github.com/CognitoBit/netpulse/internal/netutil"
)

// Strategy describes which ICMP socket flavor the MTR engine should use on
// this OS/user combination.
type Strategy struct {
	// Network is the icmp.ListenPacket network: "ip4:icmp" (raw) or "udp4"
	// (unprivileged datagram ICMP).
	Network string
}

// detectStrategy probes what kind of ICMP socket we can actually open.
//
//   - Windows: raw sockets work without elevation; datagram ICMP does not exist.
//   - macOS: datagram ICMP delivers TimeExceeded on the normal read path, so
//     unprivileged mode works; raw is the fallback for odd setups.
//   - Linux: datagram ("ping") sockets queue TimeExceeded on the error queue,
//     not the read path, so MTR needs a raw socket (root / cap_net_raw) — the
//     same requirement real mtr has.
func detectStrategy() (Strategy, error) {
	var order []string
	switch runtime.GOOS {
	case "darwin":
		order = []string{"udp4", "ip4:icmp"}
	default:
		order = []string{"ip4:icmp"}
	}

	var lastErr error
	for _, network := range order {
		conn, err := icmp.ListenPacket(network, "0.0.0.0")
		if err == nil {
			conn.Close()
			return Strategy{Network: network}, nil
		}
		lastErr = err
	}
	return Strategy{}, &netutil.PrivilegeError{Underlying: lastErr}
}
