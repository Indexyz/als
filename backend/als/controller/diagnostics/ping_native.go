package diagnostics

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type pingSettings struct {
	count             int
	interval, timeout time.Duration
}

var defaultPingSettings = pingSettings{10, time.Second, 2 * time.Second}

type pingSocket interface {
	write([]byte, time.Time) error
	read([]byte, time.Time) (int, int, netip.Addr, error)
	close() error
	isRaw() bool
}
type pingListener func(netip.Addr) (pingSocket, error)

func executePing(ctx context.Context, req request, address netip.Addr, emit func(event) bool) runResult {
	return runPing(ctx, req, address, emit, listenPing, defaultPingSettings)
}

func runPing(ctx context.Context, req request, address netip.Addr, emit func(event) bool, listen pingListener, settings pingSettings) runResult {
	s := newDiagnosticStream(ctx, emit)
	defer s.cancel()
	if !s.send(event{Type: "started", Tool: "ping", Target: req.Target, Address: address.String()}) {
		return s.finish(nil)
	}
	conn, err := listen(address)
	if err != nil {
		return s.finish(err)
	}
	watchDone := make(chan struct{})
	stopWatch := context.AfterFunc(s.ctx, func() { defer close(watchDone); conn.close() })
	defer func() {
		if !stopWatch() {
			<-watchDone
		}
		conn.close()
	}()
	payload := make([]byte, 32)
	if _, err = rand.Read(payload); err != nil {
		return s.finish(err)
	}
	id := int(binary.BigEndian.Uint16(payload[:2]))
	protocol := 1
	var echoType, replyType icmp.Type = ipv4.ICMPTypeEcho, ipv4.ICMPTypeEchoReply
	if address.Is6() {
		protocol = 58
		echoType = ipv6.ICMPTypeEchoRequest
		replyType = ipv6.ICMPTypeEchoReply
	}
	buf := make([]byte, 1500)
	for seq := 1; seq <= settings.count; seq++ {
		if s.ctx.Err() != nil {
			return s.finish(nil)
		}
		message := icmp.Message{Type: echoType, Code: 0, Body: &icmp.Echo{ID: id, Seq: seq, Data: payload}}
		packet, marshalErr := message.Marshal(nil)
		if marshalErr != nil {
			return s.finish(marshalErr)
		}
		start := time.Now()
		deadline := start.Add(settings.timeout)
		if err = conn.write(packet, deadline); err != nil {
			return s.finish(err)
		}
		n := seq
		r := &row{Seq: &n, Host: "*", Timeout: true}
		for {
			size, ttl, from, readErr := conn.read(buf, deadline)
			if readErr != nil {
				if s.ctx.Err() != nil {
					return s.finish(nil)
				}
				if timeout, ok := readErr.(net.Error); ok && timeout.Timeout() {
					break
				}
				return s.finish(readErr)
			}
			if !matchingPingReply(protocol, replyType, buf[:size], from, address, id, seq, payload, conn.isRaw()) {
				continue
			}
			r.Host = from.String()
			r.Timeout = false
			r.Latency = floatPointer(float64(time.Since(start)) / float64(time.Millisecond))
			if ttl > 0 {
				r.TTL = ttl
			}
			break
		}
		if !s.send(event{Type: "row", Row: r}) {
			return s.finish(nil)
		}
		line := fmt.Sprintf("icmp_seq=%d no reply", seq)
		if !r.Timeout {
			line = fmt.Sprintf("from %s: icmp_seq=%d ttl=%d time=%.2f ms", r.Host, seq, r.TTL, *r.Latency)
		}
		if !s.send(event{Type: "output", Line: line}) {
			return s.finish(nil)
		}
		if seq < settings.count {
			timer := time.NewTimer(max(time.Until(start.Add(settings.interval)), 0))
			select {
			case <-s.ctx.Done():
				timer.Stop()
				return s.finish(nil)
			case <-timer.C:
			}
		}
	}
	return s.finish(nil)
}

func matchingPingReply(protocol int, replyType icmp.Type, data []byte, from, target netip.Addr, id, seq int, payload []byte, raw bool) bool {
	if !from.IsValid() || from.Unmap() != target.Unmap() {
		return false
	}
	message, err := icmp.ParseMessage(protocol, data)
	if err != nil || message.Type != replyType || message.Code != 0 {
		return false
	}
	echo, ok := message.Body.(*icmp.Echo)
	// Datagram ping sockets may rewrite the identifier. A random per-run
	// payload and exact sequence/source match still isolate concurrent runs.
	return ok && echo.Seq == seq && (!raw || echo.ID == id) && bytes.Equal(echo.Data, payload)
}

type icmpPingSocket struct {
	conn    *icmp.PacketConn
	address netip.Addr
	raw     bool
}

func listenPing(address netip.Addr) (pingSocket, error) {
	network, rawNetwork, local := "udp4", "ip4:icmp", "0.0.0.0"
	if address.Is6() {
		network, rawNetwork, local = "udp6", "ip6:ipv6-icmp", "::"
	}
	conn, err := icmp.ListenPacket(network, local)
	raw := false
	if err != nil {
		conn, err = icmp.ListenPacket(rawNetwork, local)
		raw = true
	}
	if err != nil {
		return nil, fmt.Errorf("open ICMP socket: %w", err)
	}
	if address.Is4() {
		err = conn.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true)
	} else {
		err = conn.IPv6PacketConn().SetControlMessage(ipv6.FlagHopLimit, true)
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &icmpPingSocket{conn, address, raw}, nil
}
func (c *icmpPingSocket) isRaw() bool  { return c.raw }
func (c *icmpPingSocket) close() error { return c.conn.Close() }
func (c *icmpPingSocket) write(data []byte, deadline time.Time) error {
	if err := c.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	var to net.Addr = &net.UDPAddr{IP: net.IP(c.address.AsSlice())}
	if c.raw {
		to = &net.IPAddr{IP: net.IP(c.address.AsSlice())}
	}
	_, err := c.conn.WriteTo(data, to)
	return err
}
func (c *icmpPingSocket) read(data []byte, deadline time.Time) (int, int, netip.Addr, error) {
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return 0, 0, netip.Addr{}, err
	}
	var n, ttl int
	var from net.Addr
	var err error
	if c.address.Is4() {
		var control *ipv4.ControlMessage
		n, control, from, err = c.conn.IPv4PacketConn().ReadFrom(data)
		if control != nil {
			ttl = control.TTL
		}
	} else {
		var control *ipv6.ControlMessage
		n, control, from, err = c.conn.IPv6PacketConn().ReadFrom(data)
		if control != nil {
			ttl = control.HopLimit
		}
	}
	var ip net.IP
	switch source := from.(type) {
	case *net.IPAddr:
		ip = source.IP
	case *net.UDPAddr:
		ip = source.IP
	}
	address, _ := netip.AddrFromSlice(ip)
	return n, ttl, address.Unmap(), err
}
