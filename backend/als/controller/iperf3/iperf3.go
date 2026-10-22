package iperf3

import (
	"context"
	"io"
	"math/rand"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samlm0/als/v2/als/client"
	"github.com/samlm0/als/v2/config"
)

func random(min, max int) int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return r.Intn(max-min+1) + min
}

func send(ctx context.Context, session *client.ClientSession, message *client.Message) {
	select {
	case session.Channel <- message:
	case <-ctx.Done():
	}
}

func Handle(c *gin.Context) {
	v, _ := c.Get("clientSession")
	clientSession := v.(*client.ClientSession)

	port := random(config.Config.Iperf3StartPort, config.Config.Iperf3EndPort)
	ctx, cancel := context.WithTimeout(clientSession.GetContext(c.Request.Context()), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "iperf3", "-s", "--forceflush", "-p", strconv.Itoa(port))
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

	send(ctx, clientSession, &client.Message{Name: "Iperf3", Content: strconv.Itoa(port)})
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
				send(ctx, clientSession, &client.Message{Name: "Iperf3Stream", Content: string(buf[:n])})
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
