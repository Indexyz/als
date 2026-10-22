package speedtest

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
)

func send(ctx context.Context, session *client.ClientSession, message *client.Message) {
	select {
	case session.Channel <- message:
	case <-ctx.Done():
	}
}

func HandleSpeedtestDotNet(c *gin.Context) {
	nodeID, ok := c.GetQuery("node_id")
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)
	if !ok {
		nodeID = ""
	}

	ctx, cancel := context.WithTimeout(clientSession.GetContext(c.Request.Context()), 60*time.Second)
	defer cancel()

	client.WaitQueue(ctx, func() {
		pos, totalPos := client.GetQueuePostitionByCtx(ctx)
		msg, _ := json.Marshal(gin.H{"type": "queue", "pos": pos, "totalPos": totalPos})
		send(ctx, clientSession, &client.Message{Name: "SpeedtestStream", Content: string(msg)})
	})
	if ctx.Err() != nil {
		c.JSON(200, &gin.H{"success": false})
		return
	}

	args := []string{"--accept-license", "-f", "jsonl"}
	if nodeID != "" {
		args = append(args, "-s", nodeID)
	}
	cmd := exec.CommandContext(ctx, "speedtest", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.JSON(500, &gin.H{"success": false})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		c.JSON(500, &gin.H{"success": false})
		return
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		c.JSON(400, &gin.H{"success": false})
		return
	}

	var readers sync.WaitGroup
	readOutput := func(pipe io.ReadCloser) {
		defer readers.Done()
		defer pipe.Close()
		buf := make([]byte, 1024)
		for {
			n, readErr := pipe.Read(buf)
			if n > 0 {
				send(ctx, clientSession, &client.Message{Name: "SpeedtestStream", Content: string(buf[:n])})
			}
			if readErr != nil {
				return
			}
		}
	}
	readers.Add(2)
	go readOutput(stdout)
	go readOutput(stderr)

	_ = cmd.Wait()
	readers.Wait()
	c.JSON(200, &gin.H{"success": true})
}
