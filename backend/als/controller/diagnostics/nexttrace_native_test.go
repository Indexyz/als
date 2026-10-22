package diagnostics

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

func TestNativeNextTraceStructuredRows(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	address := netip.MustParseAddr("2001:db8::1")
	var events []event
	result := runNextTrace(context.Background(), request{Tool: "nexttrace", Target: address.String()}, address, func(e event) bool {
		events = append(events, e)
		return true
	}, func(ctx context.Context, method trace.Method, cfg trace.Config) (*trace.Result, error) {
		if method != trace.ICMPTrace || !cfg.DstIP.Equal(net.ParseIP(address.String())) || cfg.MaxHops != 30 || cfg.NumMeasurements != 3 || cfg.MaxAttempts != 3 || cfg.Timeout != time.Second || cfg.IPGeoSource == nil || cfg.RDNS || cfg.Maptrace {
			t.Fatalf("unsafe or incomplete library configuration: %+v", cfg)
		}
		geo := &ipgeo.IPGeoData{Asnumber: "64500", CountryEn: "Example country", City: "Example city", Owner: "Test network"}
		snapshot := &trace.Result{Hops: [][]trace.Hop{{
			{Address: &net.IPAddr{IP: net.ParseIP(address.String())}, Success: true, RTT: 2300 * time.Microsecond, Geo: geo},
			{Address: &net.IPAddr{IP: net.ParseIP(address.String())}, Success: true, RTT: 2100 * time.Microsecond, Geo: geo},
			{Error: errors.New("timeout")},
		}}}
		cfg.RealtimePrinter(snapshot, 0)
		// Published rows must own their data, independently of upstream snapshots.
		geo.Asnumber = "changed"
		snapshot.Hops[0][0].RTT = time.Hour
		return snapshot, nil
	})
	if result.err != nil || result.exitCode != 0 {
		t.Fatal(result)
	}
	var rows []*row
	for _, e := range events {
		if e.Type == "row" {
			rows = append(rows, e.Row)
		}
	}
	if len(events) == 0 || events[0].Type != "started" || len(rows) != 2 {
		t.Fatalf("events: %+v", events)
	}
	if rows[0].ASN != "64500" || rows[0].Location != "Example country / Example city" || len(rows[0].RTTs) != 2 || rows[0].RTTs[0] != 2.3 || rows[0].RTTs[1] != 2.1 || rows[1].Timeouts != 1 {
		t.Fatalf("rows: %+v %+v", rows[0], rows[1])
	}
}

func TestNativeNextTraceCancellationAndConcurrency(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	finished := make(chan runResult, 1)
	address := netip.MustParseAddr("127.0.0.1")
	req := request{Tool: "nexttrace", Target: address.String()}
	emit := func(event) bool { return true }
	go func() {
		finished <- runNextTrace(ctx, req, address, emit, func(ctx context.Context, _ trace.Method, _ trace.Config) (*trace.Result, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("native run did not start")
	}
	called := false
	runner := func(context.Context, trace.Method, trace.Config) (*trace.Result, error) {
		called = true
		return &trace.Result{}, nil
	}
	busy := runNextTrace(context.Background(), req, address, emit, runner)
	if called || busy.err == nil || busy.err.code != "busy" {
		t.Fatal("overlapping native trace was allowed", busy)
	}
	cancel()
	select {
	case result := <-finished:
		if result.err != nil {
			t.Fatal(result)
		}
	case <-time.After(time.Second):
		t.Fatal("native cancellation did not finish")
	}
	if result := runNextTrace(context.Background(), req, address, emit, runner); result.err != nil || !called {
		t.Fatal("native slot leaked", result)
	}
}

func TestNativeNextTraceErrorsAndOutputLimit(t *testing.T) {
	for _, kind := range []string{"permission", "output", "consumer"} {
		t.Run(kind, func(t *testing.T) {
			address := netip.MustParseAddr("127.0.0.1")
			var output string
			result := runNextTrace(context.Background(), request{Tool: "nexttrace", Target: address.String()}, address, func(e event) bool {
				if e.Type == "output" {
					output += e.Line
				}
				return kind != "consumer" || e.Type != "row"
			}, func(ctx context.Context, _ trace.Method, cfg trace.Config) (*trace.Result, error) {
				if kind == "permission" {
					return nil, errors.New("socket: operation not permitted")
				}
				owner := "Test network"
				if kind == "output" {
					owner = strings.Repeat("x", 20*1024)
				}
				cfg.RealtimePrinter(&trace.Result{Hops: [][]trace.Hop{{{Address: &net.IPAddr{IP: net.ParseIP("127.0.0.1")}, Geo: &ipgeo.IPGeoData{Owner: owner}}}}}, 0)
				if ctx.Err() == nil {
					t.Fatal("stream failure did not cancel the library")
				}
				return nil, ctx.Err()
			})
			if kind == "permission" && (result.err == nil || result.err.code != "execution_failed" || !strings.Contains(output, "operation not permitted")) {
				t.Fatal(result, output)
			}
			if kind == "output" && (result.err == nil || result.err.code != "output_limit") {
				t.Fatal(result)
			}
		})
	}
}

func TestNextTraceGeoCloseDrainsLookups(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	geo := newNextTraceGeo(context.Background(), ipgeo.SourceDescriptor{Source: func(_ string, timeout time.Duration, _ string, _ bool) (*ipgeo.IPGeoData, error) {
		calls.Add(1)
		if timeout != 2*time.Second {
			t.Errorf("lookup timeout was not bounded: %v", timeout)
		}
		close(entered)
		<-release
		return &ipgeo.IPGeoData{}, nil
	}})
	lookupDone := make(chan struct{})
	go func() { defer close(lookupDone); geo.lookup("192.0.2.1", time.Hour, "cn", false) }()
	<-entered
	closed := make(chan struct{})
	go func() { geo.close(); close(closed) }()
	<-geo.ctx.Done()
	select {
	case <-closed:
		t.Fatal("GeoIP close did not drain its provider")
	default:
	}
	close(release)
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("GeoIP close blocked")
	}
	<-lookupDone
	if _, err := geo.lookup("192.0.2.2", time.Second, "cn", false); !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatal("late lookup escaped its closed session", err)
	}
}

func TestNativeNextTraceWebSocketWithoutExecutable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	url, _ := testServer(t)
	conn := dial(t, url)
	conn.WriteJSON(request{Type: "start", Tool: "nexttrace", Target: "127.0.0.1"})
	readUntil(t, conn, "started")
	var sawRow, permissionError bool
	for {
		var e event
		if err := conn.ReadJSON(&e); err != nil {
			t.Fatal(err)
		}
		switch e.Type {
		case "row":
			sawRow = true
		case "output":
			permissionError = permissionError || strings.Contains(e.Line, "operation not permitted") || strings.Contains(e.Line, "permission denied")
		case "error":
			if e.Code != "execution_failed" {
				t.Fatalf("native API unexpectedly depends on a binary: %+v", e)
			}
		case "done":
			if e.Status == "completed" && sawRow {
				return
			}
			if e.Status == "error" && permissionError {
				return
			}
			t.Fatalf("unexpected native result: %+v, row=%v permission=%v", e, sawRow, permissionError)
		}
	}
}
