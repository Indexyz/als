package als

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

func TestWebTerminalRemovedAndLegacyPingValidates(t *testing.T) {
	previous := config.Config
	config.Config = config.GetDefaultConfig()
	t.Cleanup(func() { config.Config = previous })
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupHttpRoute(router)
	session := &client.ClientSession{}
	session.SetContext(context.Background())
	client.Register("route-test", session)
	t.Cleanup(func() { client.Remove("route-test") })
	for _, target := range []string{"/session/route-test/shell", "/session/missing/shell"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("old terminal route remains accessible: %d", response.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/method/ping?ip=example.com%3Bwhoami", nil)
	req.Header.Set("session", "route-test")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("legacy ping accepted invalid target: %d", response.Code)
	}
}
