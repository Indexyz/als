package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

const maxOutputBytes = 256 * 1024

type runResult struct {
	exitCode int // Protocol compatibility: 0 = finished, -1 = failed/cancelled.
	err      *apiError
}

type executor func(context.Context, request, func(event) bool) runResult

type row struct {
	Seq      *int      `json:"seq,omitempty"`
	Hop      int       `json:"hop,omitempty"`
	Host     string    `json:"host"`
	TTL      int       `json:"ttl,omitempty"`
	Latency  *float64  `json:"latency_ms,omitempty"`
	RTTs     []float64 `json:"rtts_ms,omitempty"`
	Timeouts int       `json:"timeouts,omitempty"`
	Timeout  bool      `json:"timeout,omitempty"`
	Loss     *float64  `json:"loss_percent,omitempty"`
	Sent     int       `json:"sent,omitempty"`
	Last     *float64  `json:"last_ms,omitempty"`
	Avg      *float64  `json:"avg_ms,omitempty"`
	Best     *float64  `json:"best_ms,omitempty"`
	Worst    *float64  `json:"worst_ms,omitempty"`
	StdDev   *float64  `json:"stddev_ms,omitempty"`
	ASN      string    `json:"asn,omitempty"`
	Location string    `json:"location,omitempty"`
	Owner    string    `json:"owner,omitempty"`
}

func execute(ctx context.Context, req request, emit func(event) bool) runResult {
	address, err := resolveTarget(ctx, req.Target, req.Family)
	if err != nil {
		return runResult{-1, &apiError{"resolve_failed", err.Error()}}
	}
	switch req.Tool {
	case "ping":
		return executePing(ctx, req, address, emit)
	case "traceroute", "nexttrace":
		return executeTrace(ctx, req, address, emit)
	case "mtr":
		return executeMTR(ctx, req, address, emit)
	default:
		return runResult{-1, &apiError{"invalid_tool", "Unknown diagnostic tool."}}
	}
}

// All runners publish bounded JSON and receive cancellation when a slow or
// disconnected consumer stops accepting events. Callbacks may be concurrent.
type diagnosticStream struct {
	ctx       context.Context
	cancel    context.CancelFunc
	emit      func(event) bool
	mu        sync.Mutex
	bytesSent int
	err       *apiError
}

func newDiagnosticStream(ctx context.Context, emit func(event) bool) *diagnosticStream {
	ctx, cancel := context.WithCancel(ctx)
	return &diagnosticStream{ctx: ctx, cancel: cancel, emit: emit}
}

func (s *diagnosticStream) send(e event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx.Err() != nil {
		return false
	}
	encoded, err := json.Marshal(e)
	limit := 16 * 1024
	if e.Type == "snapshot" {
		limit = 64 * 1024
	}
	if err != nil || len(encoded) > limit || s.bytesSent+len(encoded) > maxOutputBytes {
		s.err = &apiError{"output_limit", "Diagnostic output exceeded the allowed size."}
		s.cancel()
		return false
	}
	s.bytesSent += len(encoded)
	if !s.emit(e) {
		s.cancel()
		return false
	}
	return true
}

// Runners must join their callbacks before finish. Cancellation is classified
// by the WebSocket handler using its parent context and stop reason.
func (s *diagnosticStream) finish(runErr error) runResult {
	s.mu.Lock()
	streamErr := s.err
	s.mu.Unlock()
	if streamErr != nil {
		return runResult{-1, streamErr}
	}
	if s.ctx.Err() != nil {
		return runResult{-1, nil}
	}
	if runErr != nil {
		var apiErr *apiError
		if !errors.As(runErr, &apiErr) {
			apiErr = &apiError{"execution_failed", "The diagnostic failed. Check the output and the ALS process's network permissions."}
		}
		s.send(event{Type: "output", Line: runErr.Error()})
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.err != nil {
			return runResult{-1, s.err}
		}
		return runResult{-1, apiErr}
	}
	return runResult{0, nil}
}
