package diagnostics

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
	"github.com/nxtrace/NTrace-core/util"
	"github.com/nxtrace/NTrace-core/wshandle"
)

// The upstream trace engine uses process-scoped ICMP identifiers and GeoIP v3
// state. Isolate route diagnostics from one another; Ping has its own engine.
var routeProbeSlot = make(chan struct{}, 1)

type nextTraceRunner func(context.Context, trace.Method, trace.Config) (*trace.Result, error)

func acquireRouteProbe() (func(), error) {
	if util.EnvDevMode {
		return nil, &apiError{"execution_failed", "Disable NEXTTRACE_DEVMODE: the upstream CLI developer mode can panic the ALS process on network errors."}
	}
	select {
	case routeProbeSlot <- struct{}{}:
		return func() { <-routeProbeSlot }, nil
	default:
		return nil, &apiError{"busy", "A route diagnostic is already running. Try again shortly."}
	}
}

func executeTrace(ctx context.Context, req request, address netip.Addr, emit func(event) bool) runResult {
	return runNextTrace(ctx, req, address, emit, trace.TracerouteWithContext)
}

func runNextTrace(ctx context.Context, req request, address netip.Addr, emit func(event) bool, run nextTraceRunner) runResult {
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
	if req.Tool == "nexttrace" {
		geo := newNextTraceGeo(s.ctx, ipgeo.GetSourceDescriptor(ipgeo.NextTraceAPIProvider))
		defer geo.close()
		cfg.IPGeoSource = geo.cachedSource()
	}
	if !s.send(event{Type: "started", Tool: req.Tool, Target: req.Target, Address: address.String()}) {
		return s.finish(nil)
	}
	cfg.RealtimePrinter = func(snapshot *trace.Result, ttlIndex int) {
		// Upstream supplies detached snapshots, not the mutable live result.
		if snapshot == nil || ttlIndex < 0 || ttlIndex >= len(snapshot.Hops) || ttlIndex >= cfg.MaxHops {
			return
		}
		for _, r := range nextTraceRows(ttlIndex+1, snapshot.Hops[ttlIndex]) {
			if req.Tool != "nexttrace" {
				r.ASN = ""
				r.Location = ""
				r.Owner = ""
			}
			if !s.send(event{Type: "row", Row: r}) || !s.send(event{Type: "output", Line: nextTraceRowText(r)}) {
				return
			}
		}
	}
	_, err = run(s.ctx, trace.ICMPTrace, cfg)
	return s.finish(err)
}

func nativeTraceConfig(address netip.Addr) trace.Config {
	osType := 3
	if runtime.GOOS == "darwin" {
		osType = 1
	}
	if runtime.GOOS == "windows" {
		osType = 2
	}
	return trace.Config{
		OSType: osType, DstIP: net.IP(address.AsSlice()), BeginHop: 1,
		MaxHops: 30, NumMeasurements: 3, MaxAttempts: 3, ParallelRequests: 6,
		Timeout: time.Second, PacketInterval: 50, TTLInterval: 300,
		Lang: "cn", RDNS: false, Maptrace: false,
		IPGeoSource: ipgeo.GetSource("disable-geoip"),
	}
}

func nextTraceRows(hop int, probes []trace.Hop) []*row {
	var rows []*row
	byHost := make(map[string]*row)
	for _, probe := range probes {
		host := "*"
		if probe.Address != nil {
			host = probe.Address.String()
		}
		r := byHost[host]
		if r == nil {
			r = &row{Hop: hop, Host: host}
			byHost[host] = r
			rows = append(rows, r)
		}
		if probe.Address == nil {
			r.Timeouts++
			continue
		}
		r.RTTs = append(r.RTTs, float64(probe.RTT)/float64(time.Millisecond))
		if probe.Geo != nil {
			r.ASN = probe.Geo.Asnumber
			r.Owner = probe.Geo.Owner
			var location []string
			for _, part := range [][2]string{{probe.Geo.Country, probe.Geo.CountryEn}, {probe.Geo.Prov, probe.Geo.ProvEn}, {probe.Geo.City, probe.Geo.CityEn}} {
				value := part[0]
				if value == "" {
					value = part[1]
				}
				if value != "" {
					location = append(location, value)
				}
			}
			r.Location = strings.Join(location, " / ")
		}
	}
	return rows
}

func nextTraceRowText(r *row) string {
	var samples []string
	for _, rtt := range r.RTTs {
		samples = append(samples, fmt.Sprintf("%.2f ms", rtt))
	}
	for i := 0; i < r.Timeouts; i++ {
		samples = append(samples, "*")
	}
	return strings.TrimSpace(fmt.Sprintf("%d  %s  %s  %s  %s  %s", r.Hop, r.Host, strings.Join(samples, " / "), r.ASN, r.Location, r.Owner))
}

// Bind the legacy GeoIP callback to a run. Close its transport and drain actual
// provider calls before releasing the shared route slot; late cache callbacks
// cannot use a subsequent run's process-global GeoIP WebSocket.
type nextTraceGeo struct {
	ctx        context.Context
	cancel     context.CancelFunc
	descriptor ipgeo.SourceDescriptor
	mu         sync.Mutex
	closed     bool
	pending    sync.WaitGroup
	conn       *wshandle.WsConn
}

func newNextTraceGeo(ctx context.Context, descriptor ipgeo.SourceDescriptor) *nextTraceGeo {
	ctx, cancel := context.WithCancel(ctx)
	return &nextTraceGeo{ctx: ctx, cancel: cancel, descriptor: descriptor}
}
func (g *nextTraceGeo) cachedSource() ipgeo.Source {
	descriptor := g.descriptor
	descriptor.Source = g.lookup
	return trace.CachedGeoSource(descriptor)
}
func (g *nextTraceGeo) lookup(ip string, timeout time.Duration, language string, maptrace bool) (*ipgeo.IPGeoData, error) {
	g.mu.Lock()
	if g.closed || g.ctx.Err() != nil {
		g.mu.Unlock()
		return nil, context.Canceled
	}
	g.pending.Add(1)
	if g.conn == nil && g.descriptor.Backend == ipgeo.SourceBackendNextTraceAPIV3 {
		g.conn = wshandle.NewWithContextAsync(g.ctx)
		if g.conn != nil && g.conn.Interrupt != nil {
			signal.Stop(g.conn.Interrupt)
		}
	}
	g.mu.Unlock()
	defer g.pending.Done()
	if timeout <= 0 || timeout > 2*time.Second {
		timeout = 2 * time.Second
	}
	return g.descriptor.Source(ip, timeout, language, maptrace)
}
func (g *nextTraceGeo) close() {
	g.mu.Lock()
	g.closed = true
	g.cancel()
	conn := g.conn
	g.mu.Unlock()
	if conn != nil {
		conn.Close()
	}
	g.pending.Wait()
}
