package diagnostics

import (
	"context"
	"errors"
	"go/parser"
	"go/token"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
)

func TestNativeMTRLiveSnapshots(t *testing.T) {
	address := netip.MustParseAddr("127.0.0.1")
	var snapshots [][]*row
	result := runMTR(context.Background(), request{Tool: "mtr", Target: address.String()}, address, func(e event) bool {
		if e.Type == "snapshot" {
			snapshots = append(snapshots, e.Rows)
		}
		return true
	}, func(ctx context.Context, method trace.Method, cfg trace.Config, opts trace.MTROptions, onSnapshot trace.MTROnSnapshot) error {
		if method != trace.ICMPTrace || cfg.NumMeasurements != 1 || cfg.MaxAttempts != 1 || cfg.MaxHops != 30 || opts.MaxPerHop != 10 || opts.HopInterval != time.Second || opts.ProgressThrottle != time.Second || cfg.RDNS || cfg.Maptrace {
			t.Fatal("unbounded MTR configuration", cfg, opts)
		}
		initial := []trace.MTRHopStat{{TTL: 1, IP: "127.0.0.1", Snt: 1, Received: 1, Last: 1, Avg: 1, Best: 1, Wrst: 1}, {TTL: 2, Snt: 1, Loss: 100}}
		onSnapshot(1, initial)
		onSnapshot(1, initial) // Identical previews should not consume the budget.
		initial[0].Avg = 900
		onSnapshot(2, []trace.MTRHopStat{{TTL: 1, IP: "127.0.0.1", Snt: 2, Received: 2, Last: 3, Avg: 2, Best: 1, Wrst: 3, StDev: 1}})
		return nil
	})
	if result.err != nil || len(snapshots) != 2 {
		t.Fatal(result, len(snapshots))
	}
	if len(snapshots[0]) != 2 || snapshots[0][1].Avg != nil || *snapshots[0][0].Avg != 1 || len(snapshots[1]) != 1 || *snapshots[1][0].Avg != 2 || snapshots[1][0].Sent != 2 {
		t.Fatal("snapshot data is stale or aliases its input", snapshots)
	}
}
func TestNativeMTRCancellationAndSharedRouteLimit(t *testing.T) {
	address := netip.MustParseAddr("127.0.0.1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	done := make(chan runResult, 1)
	go func() {
		done <- runMTR(ctx, request{Tool: "mtr", Target: address.String()}, address, func(event) bool { return true }, func(ctx context.Context, _ trace.Method, _ trace.Config, _ trace.MTROptions, _ trace.MTROnSnapshot) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})
	}()
	<-started
	called := false
	result := runNextTrace(context.Background(), request{Tool: "traceroute", Target: address.String()}, address, func(event) bool { return true }, func(context.Context, trace.Method, trace.Config) (*trace.Result, error) {
		called = true
		return nil, nil
	})
	if called || result.err == nil || result.err.code != "busy" {
		t.Fatal("route engines overlapped", result)
	}
	cancel()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result)
		}
	case <-time.After(time.Second):
		t.Fatal("MTR cancellation did not join")
	}
}
func TestDiagnosticPackageCannotExecuteCommands(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if path == "os/exec" || strings.Contains(path, "fakeshell") {
				t.Fatalf("diagnostic command execution dependency restored in %s: %s", name, path)
			}
		}
	}
	if _, err := os.Stat("../../../fakeshell/commands/map.go"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("legacy command forwarding still present", err)
	}
}
