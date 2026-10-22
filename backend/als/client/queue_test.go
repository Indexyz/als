package client

import (
	"context"
	"testing"
	"time"
)

func TestQueueConcurrentPositionAndCancellation(t *testing.T) {
	queueLock.Lock()
	queueLine = make(map[context.Context]context.CancelFunc)
	queueNotify = make(map[context.Context]func())
	queueLock.Unlock()
	go HandleQueue()

	ctx, cancel := context.WithCancel(context.Background())
	queued := make(chan struct{})
	go func() {
		close(queued)
		WaitQueue(ctx, nil)
	}()
	<-queued

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		position, total := GetQueuePostitionByCtx(ctx)
		if position == 1 && total == 1 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("queue entry was not registered")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	cancel()
	deadline.Reset(time.Second)
	for {
		position, total := GetQueuePostitionByCtx(ctx)
		if position == 0 && total == 0 {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("queue entry was not removed after cancellation")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
