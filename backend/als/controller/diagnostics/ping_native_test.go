package diagnostics

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type fakePingSocket struct {
	address             netip.Addr
	packet              []byte
	writes              int
	timeout             bool
	block               bool
	writeErr            error
	closed              chan struct{}
	reading             chan struct{}
	closeOnce, readOnce sync.Once
}

func newFakePing(address netip.Addr) *fakePingSocket {
	return &fakePingSocket{address: address, closed: make(chan struct{}), reading: make(chan struct{})}
}
func (s *fakePingSocket) isRaw() bool  { return true }
func (s *fakePingSocket) close() error { s.closeOnce.Do(func() { close(s.closed) }); return nil }
func (s *fakePingSocket) write(data []byte, _ time.Time) error {
	s.writes++
	s.packet = append([]byte(nil), data...)
	return s.writeErr
}
func (s *fakePingSocket) read(buf []byte, _ time.Time) (int, int, netip.Addr, error) {
	s.readOnce.Do(func() { close(s.reading) })
	if s.block {
		<-s.closed
		return 0, 0, netip.Addr{}, net.ErrClosed
	}
	if s.timeout {
		return 0, 0, netip.Addr{}, &net.DNSError{IsTimeout: true, Err: "timeout"}
	}
	protocol := 1
	var replyType icmp.Type = ipv4.ICMPTypeEchoReply
	if s.address.Is6() {
		protocol = 58
		replyType = ipv6.ICMPTypeEchoReply
	}
	msg, err := icmp.ParseMessage(protocol, s.packet)
	if err != nil {
		return 0, 0, netip.Addr{}, err
	}
	reply := icmp.Message{Type: replyType, Body: msg.Body}
	encoded, err := reply.Marshal(nil)
	return copy(buf, encoded), 64, s.address, err
}
func TestPingReplyIdentity(t *testing.T) {
	for _, target := range []string{"127.0.0.1", "::1"} {
		address := netip.MustParseAddr(target)
		protocol := 1
		var replyType icmp.Type = ipv4.ICMPTypeEchoReply
		if address.Is6() {
			protocol = 58
			replyType = ipv6.ICMPTypeEchoReply
		}
		payload := []byte("per-run-random-payload")
		for _, kind := range []string{"valid", "wrong-source", "wrong-sequence", "wrong-id", "wrong-payload", "wrong-type", "truncated", "kernel-id"} {
			t.Run(target+"/"+kind, func(t *testing.T) {
				body := &icmp.Echo{ID: 1234, Seq: 2, Data: append([]byte(nil), payload...)}
				message := icmp.Message{Type: replyType, Body: body}
				from := address
				raw := true
				switch kind {
				case "wrong-source":
					from = netip.MustParseAddr("192.0.2.1")
				case "wrong-sequence":
					body.Seq++
				case "wrong-id":
					body.ID++
				case "wrong-payload":
					body.Data[0] ^= 1
				case "wrong-type":
					if address.Is4() {
						message.Type = ipv4.ICMPTypeEcho
					} else {
						message.Type = ipv6.ICMPTypeEchoRequest
					}
				case "kernel-id":
					raw = false
					body.ID++
				}
				encoded, err := message.Marshal(nil)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "truncated" {
					encoded = encoded[:2]
				}
				got := matchingPingReply(protocol, replyType, encoded, from, address, 1234, 2, payload, raw)
				if got != (kind == "valid" || kind == "kernel-id") {
					t.Fatalf("reply accepted=%v", got)
				}
			})
		}
	}
}
func TestNativePingRowsAndTimeouts(t *testing.T) {
	for _, target := range []string{"127.0.0.1", "::1"} {
		for _, timeout := range []bool{false, true} {
			address := netip.MustParseAddr(target)
			socket := newFakePing(address)
			socket.timeout = timeout
			var rows []*row
			result := runPing(context.Background(), request{Tool: "ping", Target: target}, address, func(e event) bool {
				if e.Type == "row" {
					rows = append(rows, e.Row)
				}
				return true
			}, func(netip.Addr) (pingSocket, error) { return socket, nil }, pingSettings{3, time.Nanosecond, time.Millisecond})
			if result.err != nil || len(rows) != 3 || socket.writes != 3 {
				t.Fatal(result, len(rows), socket.writes)
			}
			for index, r := range rows {
				if *r.Seq != index+1 || r.Timeout != timeout {
					t.Fatalf("row=%+v", r)
				}
				if !timeout && (r.Latency == nil || r.TTL != 64 || r.Host != target) {
					t.Fatalf("reply=%+v", r)
				}
			}
			select {
			case <-socket.closed:
			default:
				t.Fatal("ping socket leaked")
			}
		}
	}
}
func TestNativePingCancellationClosesSocket(t *testing.T) {
	address := netip.MustParseAddr("127.0.0.1")
	socket := newFakePing(address)
	socket.block = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan runResult, 1)
	go func() {
		done <- runPing(ctx, request{Tool: "ping", Target: address.String()}, address, func(event) bool { return true }, func(netip.Addr) (pingSocket, error) { return socket, nil }, defaultPingSettings)
	}()
	<-socket.reading
	cancel()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked read survived cancellation")
	}
}
func TestNativePingPermissionFailureDoesNotRetryForever(t *testing.T) {
	address := netip.MustParseAddr("127.0.0.1")
	socket := newFakePing(address)
	socket.writeErr = syscall.EACCES
	result := runPing(context.Background(), request{Tool: "ping", Target: address.String()}, address, func(event) bool { return true }, func(netip.Addr) (pingSocket, error) { return socket, nil }, defaultPingSettings)
	if result.err == nil || result.err.code != "execution_failed" || socket.writes != 1 {
		t.Fatal(result, socket.writes)
	}
}
func TestNativePingWithoutExecutables(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, target := range []string{"127.0.0.1", "::1"} {
		t.Run(target, func(t *testing.T) {
			address := netip.MustParseAddr(target)
			var reply *row
			var detail string
			result := runPing(context.Background(), request{Tool: "ping", Target: target}, address, func(e event) bool {
				if e.Type == "row" {
					reply = e.Row
				}
				detail += e.Line
				return true
			}, listenPing, pingSettings{1, time.Millisecond, 500 * time.Millisecond})
			if result.err != nil && (strings.Contains(detail, "not permitted") || strings.Contains(detail, "permission denied") || strings.Contains(detail, "not supported")) {
				t.Skip("host does not permit ICMP: " + detail)
			}
			if result.err != nil || reply == nil || reply.Timeout {
				t.Fatalf("native loopback failed: %+v reply=%+v %s", result, reply, detail)
			}
		})
	}
}
