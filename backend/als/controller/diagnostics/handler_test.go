package diagnostics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/als/controller"
	"github.com/samlm0/als/v2/config"
)

func testServer(t *testing.T, runners ...executor) (string, context.CancelFunc) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	previous := config.Config
	config.Config = config.GetDefaultConfig()
	t.Cleanup(func() { config.Config = previous })
	ctx, cancel := context.WithCancel(context.Background())
	session := &client.ClientSession{}
	session.SetContext(ctx)
	client.Register("test-session", session)
	t.Cleanup(func() { cancel(); client.Remove("test-session") })
	run := executor(execute)
	if len(runners) > 0 {
		run = runners[0]
	}
	router := gin.New()
	router.GET("/session/:session/diagnostics", controller.MiddlewareSessionOnUrl(), func(c *gin.Context) { handle(c, run) })
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http") + "/session/test-session/diagnostics", cancel
}
func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	t.Cleanup(func() { conn.Close() })
	return conn
}
func readUntil(t *testing.T, conn *websocket.Conn, kind string) event {
	t.Helper()
	for {
		var e event
		if err := conn.ReadJSON(&e); err != nil {
			t.Fatal(err)
		}
		if e.Type == kind {
			return e
		}
	}
}
func startPing(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.WriteJSON(request{Type: "start", Tool: "ping", Target: "127.0.0.1"}); err != nil {
		t.Fatal(err)
	}
}
func TestWebSocketValidationAndOrigin(t *testing.T) {
	url, _ := testServer(t)
	for _, body := range []string{
		`{"type":"start","tool":"ping","target":"127.0.0.1;id"}`,
		`{"type":"start","tool":"ping","target":"--help"}`,
		`{"type":"start","tool":"ping","target":"example.com","args":["-f"]}`,
		`{"type":"start","tool":"sh","target":"example.com"}`,
	} {
		conn := dial(t, url)
		conn.WriteMessage(websocket.TextMessage, []byte(body))
		if e := readUntil(t, conn, "error"); e.Code == "" {
			t.Fatal("missing error code")
		}
		conn.Close()
	}
	conn, response, err := websocket.DefaultDialer.Dial(url, http.Header{"Origin": []string{"https://untrusted.example"}})
	if conn != nil {
		conn.Close()
	}
	if err == nil || response.StatusCode != http.StatusForbidden {
		t.Fatal("cross-origin upgrade accepted", err)
	}
	conn, response, err = websocket.DefaultDialer.Dial(strings.Replace(url, "test-session", "missing", 1), nil)
	if conn != nil {
		conn.Close()
	}
	if err == nil || response.StatusCode != http.StatusBadRequest {
		t.Fatal("missing session accepted", err)
	}
}
func TestWebSocketStreamsAndCompletes(t *testing.T) {
	url, _ := testServer(t, func(_ context.Context, req request, emit func(event) bool) runResult {
		emit(event{Type: "started", Address: req.Target})
		seq := 1
		emit(event{Type: "row", Row: &row{Seq: &seq, Host: req.Target, Latency: floatPointer(0.012)}})
		return runResult{0, nil}
	})
	conn := dial(t, url)
	startPing(t, conn)
	if e := readUntil(t, conn, "started"); e.Address != "127.0.0.1" {
		t.Fatal(e)
	}
	if e := readUntil(t, conn, "row"); e.Row.Latency == nil || *e.Row.Latency != 0.012 {
		t.Fatal(e)
	}
	if e := readUntil(t, conn, "done"); e.Status != "completed" || *e.ExitCode != 0 {
		t.Fatal(e)
	}
}
func TestStopDisconnectAndSessionEndJoinNativeRun(t *testing.T) {
	for _, action := range []string{"stop", "disconnect", "session", "protocol"} {
		t.Run(action, func(t *testing.T) {
			closed := make(chan struct{})
			url, cancelSession := testServer(t, func(ctx context.Context, req request, emit func(event) bool) runResult {
				defer close(closed)
				emit(event{Type: "started", Address: req.Target})
				<-ctx.Done()
				return runResult{-1, nil}
			})
			conn := dial(t, url)
			startPing(t, conn)
			readUntil(t, conn, "started")
			other, response, err := websocket.DefaultDialer.Dial(url, nil)
			if other != nil {
				other.Close()
			}
			if err == nil || response.StatusCode != http.StatusTooManyRequests {
				t.Fatal("concurrency limit bypassed")
			}
			switch action {
			case "stop":
				conn.WriteJSON(map[string]string{"type": "stop"})
				if e := readUntil(t, conn, "done"); e.Status != "stopped" {
					t.Fatal(e)
				}
			case "protocol":
				conn.WriteJSON(map[string]string{"type": "start"})
				if e := readUntil(t, conn, "error"); e.Code != "protocol_error" {
					t.Fatal(e)
				}
				readUntil(t, conn, "done")
			case "disconnect":
				conn.Close()
			case "session":
				cancelSession()
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("native run not cancelled/joined")
			}
		})
	}
}
func TestWebSocketFrameAndFeatureRestrictions(t *testing.T) {
	for _, test := range []struct {
		name     string
		kind     int
		payload  string
		disabled bool
	}{
		{"binary", websocket.BinaryMessage, `{"type":"start","tool":"ping","target":"127.0.0.1"}`, false},
		{"oversized", websocket.TextMessage, strings.Repeat("x", maxMessageSize+1), false},
		{"disabled", websocket.TextMessage, `{"type":"start","tool":"ping","target":"127.0.0.1"}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			url, _ := testServer(t)
			config.Config.FeaturePing = !test.disabled
			conn := dial(t, url)
			conn.WriteMessage(test.kind, []byte(test.payload))
			var e event
			err := conn.ReadJSON(&e)
			if test.name == "oversized" {
				if !websocket.IsCloseError(err, websocket.CloseMessageTooBig) {
					t.Fatalf("expected oversized frame rejection: %v", err)
				}
			} else if err != nil || e.Type != "error" {
				t.Fatalf("unexpected response: %+v %v", e, err)
			} else if test.disabled && e.Code != "disabled" {
				t.Fatal(e)
			}
		})
	}
}
func TestNativeOutputBudget(t *testing.T) {
	s := newDiagnosticStream(context.Background(), func(event) bool { return true })
	defer s.cancel()
	for i := 0; i < 40 && s.ctx.Err() == nil; i++ {
		s.send(event{Type: "output", Line: strings.Repeat("x", 10000)})
	}
	if result := s.finish(nil); result.err == nil || result.err.code != "output_limit" {
		t.Fatal(result)
	}
}
