package speedtest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func newRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/download", HandleDownload)
	router.POST("/upload", HandleUpload)
	router.HEAD("/ping", HandlePing)
	return router
}

func TestDownloadAnnouncesLengthAndClampsChunks(t *testing.T) {
	router := newRouter()
	for query, chunks := range map[string]int{"": 4, "?ckSize=2": 2, "?ckSize=0": 4, "?ckSize=x": 4, "?ckSize=-3": 4} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/download"+query, nil))
		want := chunks * downloadChunkBytes
		if response.Code != http.StatusOK || response.Body.Len() != want {
			t.Fatalf("%q: status %d, %d bytes, want %d", query, response.Code, response.Body.Len(), want)
		}
		if got := response.Header().Get("Content-Length"); got != strconv.Itoa(want) {
			t.Fatalf("%q: Content-Length %q, want %d", query, got, want)
		}
		if response.Header().Get("Cache-Control") == "" {
			t.Fatalf("%q: missing Cache-Control", query)
		}
	}
}

func TestUploadAcceptsBodyAndRejectsOversize(t *testing.T) {
	router := newRouter()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(make([]byte, 1<<20))))
	if response.Code != http.StatusOK {
		t.Fatalf("upload status %d", response.Code)
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/upload", bytes.NewReader(make([]byte, MaxUploadBytes+1))))
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize upload status %d", response.Code)
	}
}

func TestPingIsEmptyAndUncached(t *testing.T) {
	router := newRouter()
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodHead, "/ping", nil))
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("ping status %d, %d bytes", response.Code, response.Body.Len())
	}
	if response.Header().Get("Cache-Control") == "" {
		t.Fatalf("ping missing Cache-Control")
	}
}
