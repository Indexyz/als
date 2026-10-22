package client

import (
	"context"
	"sync"
)

var clients = make(map[string]*ClientSession)
var clientsMu sync.RWMutex

type Message struct {
	Name    string
	Content string
}

type ClientSession struct {
	Channel chan *Message
	ctx     context.Context
}

func Register(id string, session *ClientSession) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	clients[id] = session
}

func Lookup(id string) (*ClientSession, bool) {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	session, ok := clients[id]
	return session, ok
}

func Remove(id string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, id)
}

// SetContext must be called before publishing the session with Register.
func (c *ClientSession) SetContext(ctx context.Context) { c.ctx = ctx }
func (c *ClientSession) Context() context.Context       { return c.ctx }

func (c *ClientSession) GetContext(requestCtx context.Context) context.Context {
	ctx, cancel := context.WithCancel(requestCtx)
	stop := context.AfterFunc(c.ctx, cancel)
	context.AfterFunc(ctx, func() { stop() })
	return ctx
}

func (c *ClientSession) Send(message *Message) {
	select {
	case c.Channel <- message:
	case <-c.ctx.Done():
	}
}

func BroadCastMessage(name string, content string) {
	clientsMu.RLock()
	sessions := make([]*ClientSession, 0, len(clients))
	for _, session := range clients {
		sessions = append(sessions, session)
	}
	clientsMu.RUnlock()
	for _, session := range sessions {
		select {
		case session.Channel <- &Message{Name: name, Content: content}:
		case <-session.ctx.Done():
		}
	}
}
