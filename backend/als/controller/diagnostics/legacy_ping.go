package diagnostics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

// HandleLegacyPing preserves the old SSE endpoint using the same bounded
// native runner and session limit as the WebSocket API.
func HandleLegacyPing(c *gin.Context) {
	target, ok := c.GetQuery("ip")
	if !ok || !ValidTarget(target) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid target"})
		return
	}
	value, _ := c.Get("clientSession")
	session, ok := value.(*client.ClientSession)
	if !ok {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if !limits.acquire(session) {
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "busy"})
		return
	}
	defer limits.release(session)
	ctx, cancel := context.WithTimeout(c.Request.Context(), maxDuration)
	defer cancel()
	stop := context.AfterFunc(session.Context(), cancel)
	defer stop()
	result := execute(ctx, request{Tool: "ping", Target: target, Family: "auto"}, func(e event) bool {
		if e.Type != "row" {
			return ctx.Err() == nil
		}
		r := e.Row
		var latency time.Duration
		if r.Latency != nil {
			latency = time.Duration(*r.Latency * float64(time.Millisecond))
		}
		payload, _ := json.Marshal(struct {
			IsTimeout bool          `json:"is_timeout"`
			From      string        `json:"from"`
			Seq       int           `json:"seq"`
			Size      int           `json:"size"`
			TTL       int           `json:"ttl"`
			Latency   time.Duration `json:"latency"`
		}{r.Timeout, r.Host, *r.Seq, 40, r.TTL, latency})
		select {
		case session.Channel <- &client.Message{Name: "Ping", Content: string(payload)}:
			return true
		case <-ctx.Done():
			return false
		}
	})
	if result.err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": result.err.message})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": ctx.Err() == nil})
}
