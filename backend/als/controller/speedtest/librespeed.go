package speedtest

import (
	"crypto/rand"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	downloadChunkBytes = 1 << 20 // 1 MiB
	maxDownloadChunks  = 1024
	// MaxUploadBytes bounds a single upload request body. The browser speed test sends at most
	// 32 MiB per request and LibreSpeed's worker 20 MiB; anything larger is rejected.
	MaxUploadBytes = 64 << 20
)

func setNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0, s-maxage=0, post-check=0, pre-check=0")
	c.Header("Pragma", "no-cache")
}

// HandleDownload streams ckSize MiB (default 4, max 1024) of incompressible data. The length is
// announced up front so clients can track progress and overlap the next request.
func HandleDownload(c *gin.Context) {
	chunks := 4
	if ckSize, ok := c.GetQuery("ckSize"); ok {
		if ckSizeInt, err := strconv.Atoi(ckSize); err == nil && ckSizeInt > 0 {
			chunks = min(ckSizeInt, maxDownloadChunks)
		}
	}

	data := make([]byte, downloadChunkBytes)
	_, _ = rand.Read(data)

	setNoStore(c)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.Itoa(chunks*downloadChunkBytes))
	c.Status(http.StatusOK)
	for i := 0; i < chunks; i++ {
		if _, err := c.Writer.Write(data); err != nil {
			return
		}
	}
}

// HandleUpload drains and discards the request body.
func HandleUpload(c *gin.Context) {
	setNoStore(c)
	c.Header("Connection", "keep-alive")
	body := http.MaxBytesReader(c.Writer, c.Request.Body, MaxUploadBytes)
	_, err := io.Copy(io.Discard, body)
	_ = body.Close()
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusOK)
}

// HandlePing answers immediately with an empty body, for round-trip latency probes.
func HandlePing(c *gin.Context) {
	setNoStore(c)
	c.Status(http.StatusNoContent)
}
