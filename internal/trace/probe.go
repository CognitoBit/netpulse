package trace

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// reply is one demultiplexed ICMP response to a probe we sent.
type reply struct {
	seq   int
	from  net.IP
	at    time.Time
	final bool // echo reply from the destination itself (path end)
}

// prober owns one ICMP socket and turns raw packets into replies.
//
// Contingency note: if raw-socket receive ever proves unreliable on Windows,
// the fallback is the iphlpapi.dll IcmpSendEcho2Ex API (per-call TTL, returns
// the responding router address; it is how tracert works unprivileged).
type prober struct {
	conn *icmp.PacketConn
	p4   *ipv4.PacketConn
	dgram bool // "udp4" datagram mode (macOS unprivileged)

	mu sync.Mutex // serializes SetTTL+WriteTo pairs
	id int
}

func newProber(s Strategy) (*prober, error) {
	conn, err := icmp.ListenPacket(s.Network, "0.0.0.0")
	if err != nil {
		return nil, err
	}
	p4 := conn.IPv4PacketConn()
	if p4 == nil {
		conn.Close()
		return nil, errors.New("no IPv4 packet conn available")
	}
	return &prober{
		conn:  conn,
		p4:    p4,
		dgram: s.Network == "udp4",
		id:    os.Getpid() & 0xffff,
	}, nil
}

func (p *prober) close() { p.conn.Close() }

// send emits one TTL-limited echo request and returns the send timestamp.
// TTL is a socket option, so SetTTL and WriteTo must be paired under a lock.
func (p *prober) send(dst net.IP, ttl, seq int) (time.Time, error) {
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: &icmp.Echo{
			ID:   p.id,
			Seq:  seq,
			Data: []byte("netpulse-mtr-probe-data-padding!"),
		},
	}
	wire, err := msg.Marshal(nil)
	if err != nil {
		return time.Time{}, err
	}

	var addr net.Addr
	if p.dgram {
		addr = &net.UDPAddr{IP: dst}
	} else {
		addr = &net.IPAddr{IP: dst}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.p4.SetTTL(ttl); err != nil {
		return time.Time{}, err
	}
	sentAt := time.Now()
	_, err = p.conn.WriteTo(wire, addr)
	return sentAt, err
}

// receiveLoop reads the socket until ctx is done, pushing demuxed replies.
func (p *prober) receiveLoop(ctx context.Context, dst net.IP, out chan<- reply) {
	buf := make([]byte, 1500)
	for ctx.Err() == nil {
		_ = p.conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		n, peer, err := p.conn.ReadFrom(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			continue
		}
		at := time.Now()

		msg := parseTolerant(buf[:n])
		if msg == nil {
			continue
		}
		from := peerIP(peer)

		switch body := msg.Body.(type) {
		case *icmp.Echo:
			// Echo reply straight from the destination.
			if msg.Type == ipv4.ICMPTypeEchoReply && p.matchesID(body.ID) {
				select {
				case out <- reply{seq: body.Seq, from: from, at: at, final: from.Equal(dst)}:
				case <-ctx.Done():
					return
				}
			}
		case *icmp.TimeExceeded:
			p.forwardEmbedded(ctx, body.Data, from, at, false, out)
		case *icmp.DstUnreach:
			// Treated as path end: the sender of the unreachable is the
			// last hop that saw our packet.
			p.forwardEmbedded(ctx, body.Data, from, at, true, out)
		}
	}
}

// forwardEmbedded recovers our echo's ID/Seq from the quoted original
// datagram inside a TimeExceeded/DstUnreach body and emits a reply.
func (p *prober) forwardEmbedded(ctx context.Context, data []byte, from net.IP, at time.Time, final bool, out chan<- reply) {
	id, seq, ok := embeddedEcho(data)
	if !ok || !p.matchesID(id) {
		return
	}
	select {
	case out <- reply{seq: seq, from: from, at: at, final: final}:
	case <-ctx.Done():
	}
}

// matchesID checks the echo ID against ours. In datagram mode the kernel may
// rewrite the ID (Linux ping sockets do; macOS behavior varies), so we only
// enforce the check on raw sockets and rely on unique Seq values otherwise.
func (p *prober) matchesID(id int) bool {
	if p.dgram {
		return true
	}
	return id == p.id
}

// parseTolerant parses an ICMP message that may or may not be prefixed with
// an IPv4 header — raw-socket reads include the header on some platforms
// (notably Windows), and datagram reads can too on Darwin.
func parseTolerant(b []byte) *icmp.Message {
	if m, err := icmp.ParseMessage(1, b); err == nil && knownType(m) {
		return m
	}
	if len(b) > 20 && b[0]>>4 == 4 {
		ihl := int(b[0]&0x0f) * 4
		if ihl >= 20 && len(b) > ihl {
			if m, err := icmp.ParseMessage(1, b[ihl:]); err == nil && knownType(m) {
				return m
			}
		}
	}
	return nil
}

func knownType(m *icmp.Message) bool {
	switch m.Type {
	case ipv4.ICMPTypeEchoReply, ipv4.ICMPTypeTimeExceeded, ipv4.ICMPTypeDestinationUnreachable:
		return true
	}
	return false
}

// embeddedEcho extracts (ID, Seq) of our original echo request from the
// quoted datagram of a TimeExceeded/DstUnreach body: an IPv4 header followed
// by at least the first 8 bytes of the ICMP echo.
func embeddedEcho(data []byte) (id, seq int, ok bool) {
	if len(data) >= 8 && data[0] == 8 { // some stacks omit the inner IP header
		return int(binary.BigEndian.Uint16(data[4:6])), int(binary.BigEndian.Uint16(data[6:8])), true
	}
	if len(data) < 28 || data[0]>>4 != 4 {
		return 0, 0, false
	}
	ihl := int(data[0]&0x0f) * 4
	if ihl < 20 || len(data) < ihl+8 {
		return 0, 0, false
	}
	inner := data[ihl:]
	if inner[0] != 8 { // must be an echo request (what we sent)
		return 0, 0, false
	}
	return int(binary.BigEndian.Uint16(inner[4:6])), int(binary.BigEndian.Uint16(inner[6:8])), true
}

func peerIP(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.IPAddr:
		return v.IP
	case *net.UDPAddr:
		return v.IP
	}
	return nil
}
