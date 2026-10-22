package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/nxtrace/NTrace-core/trace"
)

type mtrRunner func(context.Context, trace.Method, trace.Config, trace.MTROptions, trace.MTROnSnapshot) error

func executeMTR(ctx context.Context, req request, address netip.Addr, emit func(event) bool) runResult {
	return runMTR(ctx, req, address, emit, trace.RunMTR)
}

func runMTR(ctx context.Context, req request, address netip.Addr, emit func(event) bool, run mtrRunner) runResult {
	s := newDiagnosticStream(ctx, emit)
	defer s.cancel()
	if s.ctx.Err() != nil {
		return s.finish(nil)
	}
	release, err := acquireRouteProbe()
	if err != nil {
		return s.finish(err)
	}
	defer release()
	cfg := nativeTraceConfig(address)
	cfg.NumMeasurements = 1
	cfg.MaxAttempts = 1
	if !s.send(event{Type: "started", Tool: "mtr", Target: req.Target, Address: address.String()}) {
		return s.finish(nil)
	}
	opts := trace.MTROptions{HopInterval: time.Second, MaxPerHop: 10, ProgressThrottle: time.Second}
	var previous string
	var lastRows []*row
	err = run(s.ctx, trace.ICMPTrace, cfg, opts, func(_ int, stats []trace.MTRHopStat) {
		rows := mtrRows(stats)
		encoded, encodeErr := json.Marshal(rows)
		if encodeErr == nil && string(encoded) == previous {
			return
		}
		previous = string(encoded)
		lastRows = rows
		// A complete snapshot also removes stale timeout responders and hops
		// past a newly discovered path end. The UI must replace, not append.
		s.send(event{Type: "snapshot", Rows: rows})
	})
	if err == nil && s.ctx.Err() == nil {
		for _, r := range lastRows {
			average := "—"
			if r.Avg != nil {
				average = fmt.Sprintf("%.2f ms", *r.Avg)
			}
			if !s.send(event{Type: "output", Line: fmt.Sprintf("%d  %s  loss %.1f%%  sent %d  avg %s", r.Hop, r.Host, *r.Loss, r.Sent, average)}) {
				break
			}
		}
	}
	return s.finish(err)
}

func floatPointer(n float64) *float64 { return &n }

func mtrRows(stats []trace.MTRHopStat) []*row {
	rows := make([]*row, 0, len(stats))
	for _, stat := range stats {
		if stat.TTL < 1 || stat.TTL > 30 || stat.Snt == 0 {
			continue
		}
		host := stat.IP
		if host == "" {
			host = stat.Host
		}
		if host == "" || host == "???" {
			host = "*"
		}
		r := &row{Hop: stat.TTL, Host: host, Loss: floatPointer(stat.Loss), Sent: stat.Snt}
		if stat.Received > 0 {
			r.Last = floatPointer(stat.Last)
			r.Avg = floatPointer(stat.Avg)
			r.Best = floatPointer(stat.Best)
			r.Worst = floatPointer(stat.Wrst)
			r.StdDev = floatPointer(stat.StDev)
		}
		rows = append(rows, r)
	}
	return rows
}
