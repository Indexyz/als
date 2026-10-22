package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

const (
	maxDuration    = 120 * time.Second
	writeTimeout   = 5 * time.Second
	maxMessageSize = 1024
)

var (
	upgrader    = websocket.Upgrader{HandshakeTimeout: 5 * time.Second}
	errStopped  = errors.New("stopped by client")
	errProtocol = errors.New("only a stop message is allowed after start")
	limits      = runLimiter{sessions: make(map[*client.ClientSession]bool), capacity: 4}
)

type request struct {
	Type   string `json:"type"`
	Tool   string `json:"tool"`
	Target string `json:"target"`
	Family string `json:"family"`
}

type event struct {
	Type     string `json:"type"`
	Tool     string `json:"tool,omitempty"`
	Target   string `json:"target,omitempty"`
	Address  string `json:"address,omitempty"`
	Line     string `json:"line,omitempty"`
	Row      *row   `json:"row,omitempty"`
	Rows     []*row `json:"rows,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Status   string `json:"status,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

type apiError struct{ code, message string }

func (e *apiError) Error() string { return e.message }

type runLimiter struct {
	sync.Mutex
	sessions map[*client.ClientSession]bool
	capacity int
}

func (l *runLimiter) acquire(session *client.ClientSession) bool {
	l.Lock()
	defer l.Unlock()
	if l.sessions[session] || len(l.sessions) >= l.capacity {
		return false
	}
	l.sessions[session] = true
	return true
}
func (l *runLimiter) release(session *client.ClientSession) {
	l.Lock()
	defer l.Unlock()
	delete(l.sessions, session)
}

func decode(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}

func validateRequest(r *request, cfg *config.ALSConfig) *apiError {
	if r.Type != "start" {
		return &apiError{"invalid_request", "First message must be a start request."}
	}
	if !ValidTarget(r.Target) {
		return &apiError{"invalid_target", "Enter a bare IPv4, IPv6 or ASCII/Punycode domain name."}
	}
	if r.Family == "" {
		r.Family = "auto"
	}
	if r.Family != "auto" && r.Family != "4" && r.Family != "6" {
		return &apiError{"invalid_family", "IP version must be auto, 4 or 6."}
	}
	var enabled bool
	switch r.Tool {
	case "ping":
		enabled = cfg.FeaturePing
	case "traceroute":
		enabled = cfg.FeatureTraceroute
	case "mtr":
		enabled = cfg.FeatureMTR
	case "nexttrace":
		enabled = cfg.FeatureNextTrace
	default:
		return &apiError{"invalid_tool", "Unknown diagnostic tool."}
	}
	if !enabled {
		return &apiError{"disabled", "This diagnostic tool is disabled."}
	}
	return nil
}

// Handle implements one bounded test per connection. A browser must be same
// origin (the Gorilla default); a session ID is not authorization to run a shell.
func Handle(c *gin.Context) { handle(c, execute) }

func handle(c *gin.Context, run executor) {
	v, ok := c.Get("clientSession")
	session, valid := v.(*client.ClientSession)
	if !ok || !valid {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if !limits.acquire(session) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "busy", "error": "A diagnostic is already running or the server is busy."})
		return
	}
	defer limits.release(session)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	send := func(e event) bool {
		conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		return conn.WriteJSON(e) == nil
	}
	fail := func(e *apiError) { send(event{Type: "error", Code: e.code, Message: e.message}) }
	kind, data, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var req request
	if kind != websocket.TextMessage || decode(data, &req) != nil {
		fail(&apiError{"invalid_request", "Expected a JSON start request with no extra fields."})
		return
	}
	if e := validateRequest(&req, config.Config); e != nil {
		fail(e)
		return
	}

	timeoutCtx, timeoutCancel := context.WithTimeout(c.Request.Context(), maxDuration)
	defer timeoutCancel()
	ctx, cancel := context.WithCancelCause(timeoutCtx)
	defer cancel(nil)
	stopSession := context.AfterFunc(session.Context(), func() { cancel(context.Canceled) })
	defer stopSession()
	conn.SetReadDeadline(time.Now().Add(maxDuration + writeTimeout))
	// This is the only reader; the handler below is the only data writer.
	go func() {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			cancel(err)
			return
		}
		var control struct {
			Type string `json:"type"`
		}
		if kind != websocket.TextMessage || decode(data, &control) != nil || control.Type != "stop" {
			cancel(errProtocol)
			return
		}
		cancel(errStopped)
	}()

	events := make(chan event, 32)
	done := make(chan runResult, 1)
	go func() {
		result := run(ctx, req, func(e event) bool {
			select {
			case events <- e:
				return true
			case <-ctx.Done():
				return false
			}
		})
		done <- result
		close(events)
	}()
	// Keep idle proxies alive while a report or a hop is being collected.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	writeOK := true
stream:
	for {
		select {
		case e, ok := <-events:
			if !ok {
				break stream
			}
			if writeOK && !send(e) {
				writeOK = false
				cancel(io.ErrClosedPipe)
			}
		case <-ticker.C:
			if writeOK && ctx.Err() == nil {
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
					writeOK = false
					cancel(err)
				}
			}
		}
	}
	result := <-done // Join probe workers and close sockets before releasing the slot.
	if !writeOK {
		return
	}
	status := "completed"
	switch {
	case errors.Is(context.Cause(ctx), errStopped):
		status = "stopped"
	case errors.Is(context.Cause(ctx), context.DeadlineExceeded):
		status = "timeout"
	case errors.Is(context.Cause(ctx), errProtocol):
		status = "error"
		fail(&apiError{"protocol_error", errProtocol.Error()})
	case ctx.Err() != nil:
		return
	case result.err != nil:
		status = "error"
		fail(result.err)
	}
	send(event{Type: "done", Status: status, ExitCode: &result.exitCode})
	conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Test finished"), time.Now().Add(writeTimeout))
}
