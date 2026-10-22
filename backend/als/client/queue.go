package client

import (
	"context"
	"sync"
)

var queueLine = make(map[context.Context]context.CancelFunc)
var queueLock sync.RWMutex
var queueNotify = make(map[context.Context]func())
var queueWakeup = make(chan struct{}, 1)

func WaitQueue(ctx context.Context, cb func()) {
	queueCtx, cancel := context.WithCancel(ctx)

	queueLock.Lock()
	queueLine[ctx] = cancel
	if cb != nil {
		queueNotify[ctx] = cb
	}
	queueLock.Unlock()

	select {
	case queueWakeup <- struct{}{}:
	default:
	}

	select {
	case <-queueCtx.Done():
	case <-ctx.Done():
	}
}

func GetQueuePostitionByCtx(ctx context.Context) (int, int) {
	queueLock.RLock()
	defer queueLock.RUnlock()

	total := len(queueLine)
	count := 0
	for queuedCtx := range queueLine {
		count++
		if queuedCtx == ctx {
			return count, total
		}
	}
	return 0, 0
}

func HandleQueue() {
	for range queueWakeup {
		queueLock.RLock()
		queued := make([]context.Context, 0, len(queueLine))
		for ctx := range queueLine {
			queued = append(queued, ctx)
		}
		queueLock.RUnlock()

		for _, ctx := range queued {
			queueLock.RLock()
			cancel, notify := queueLine[ctx], queueNotify[ctx]
			queueLock.RUnlock()
			if cancel == nil {
				continue
			}

			if notify != nil {
				notify()
			}
			// Release this queue slot. WaitQueue returns and starts the work;
			// the original context keeps the slot occupied until that work ends.
			cancel()
			<-ctx.Done()

			queueLock.Lock()
			delete(queueLine, ctx)
			delete(queueNotify, ctx)
			remaining := make([]func(), 0, len(queueNotify))
			for _, callNotify := range queueNotify {
				remaining = append(remaining, callNotify)
			}
			queueLock.Unlock()

			for _, callNotify := range remaining {
				if callNotify != nil {
					callNotify()
				}
			}
		}
	}
}
