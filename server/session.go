package server

import (
	"github.com/lesismal/nbio/nbhttp/websocket"
	"sync"
	"time"
)

const (
	writeTimeout     = 5 * time.Second
	maxMessageBuffer = 4
)

type Message struct {
	g    *SessionGroup
	data []byte
}

type Session struct {
	TeamID   string
	UserID   string
	Messages chan *Message
	Stop     chan struct{}
	*websocket.Conn
}

func NewSession(tid, uid string, conn *websocket.Conn) *Session {
	return &Session{
		TeamID:   tid,
		UserID:   uid,
		Conn:     conn,
		Messages: make(chan *Message, maxMessageBuffer),
		Stop:     make(chan struct{}),
	}
}

// Send 发送消息到客户端
func (s *Session) Send(g *SessionGroup, data []byte) error {
	s.Messages <- &Message{
		g:    g,
		data: data,
	}
	return nil
}

func (s *Session) Start() (err error) {
	for {
		select {
		case msg := <-s.Messages:
			if err = s.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
				if msg.g != nil {
					msg.g.Remove(s)
				}
				return
			}
			if err = s.WriteMessage(websocket.BinaryMessage, msg.data); err != nil {
				if msg.g != nil {
					msg.g.Remove(s)
				}
				return
			}
		case <-s.Stop:
			return
		}
	}
}

type SessionGroup struct {
	locker   sync.RWMutex
	Sessions map[string]*Session
}

func NewSessionGroup() *SessionGroup {
	return &SessionGroup{
		Sessions: make(map[string]*Session),
	}
}

func (g *SessionGroup) Add(s *Session) {
	g.locker.Lock()
	defer g.locker.Unlock()

	g.Sessions[s.UserID] = s
}

func (g *SessionGroup) Remove(s *Session) {
	g.locker.Lock()
	defer g.locker.Unlock()

	_ = s.WriteMessage(websocket.CloseMessage, nil)
	_ = s.Close()
	s.Stop <- struct{}{}
	delete(g.Sessions, s.UserID)
}

func (g *SessionGroup) Get(uid string) *Session {
	g.locker.RLock()
	defer g.locker.RUnlock()

	return g.Sessions[uid]
}

func (g *SessionGroup) Count() int {
	g.locker.RLock()
	defer g.locker.RUnlock()

	return len(g.Sessions)
}

func (g *SessionGroup) Close() {
	g.locker.Lock()
	defer g.locker.Unlock()

	for _, s := range g.Sessions {
		_ = s.WriteMessage(websocket.CloseMessage, nil)
		_ = s.Close()
	}
	clear(g.Sessions)
}

type SessionMap struct {
	locker      sync.RWMutex
	groups      map[string]*SessionGroup
	connections map[*websocket.Conn]*Session
}

func NewSessionMap() *SessionMap {
	return &SessionMap{
		groups:      make(map[string]*SessionGroup),
		connections: make(map[*websocket.Conn]*Session),
	}
}

func (m *SessionMap) Add(s *Session) {
	m.locker.Lock()
	defer m.locker.Unlock()

	m.connections[s.Conn] = s

	g := m.groups[s.TeamID]
	if g == nil {
		g = NewSessionGroup()
		m.groups[s.TeamID] = g
	}
	g.Add(s)
}

func (m *SessionMap) Remove(conn *websocket.Conn) {
	m.locker.Lock()
	defer m.locker.Unlock()

	s := m.connections[conn]
	if s == nil {
		return
	}
	g := m.groups[s.TeamID]
	if g != nil {
		g.Remove(s)
		// remove group if empty
		if g.Count() == 0 {
			delete(m.groups, s.TeamID)
		}
	}
}

func (m *SessionMap) Get(tid string) *SessionGroup {
	m.locker.RLock()
	defer m.locker.RUnlock()

	return m.groups[tid]
}

func (m *SessionMap) Count() int {
	m.locker.RLock()
	defer m.locker.RUnlock()

	return len(m.groups)
}

func (m *SessionMap) Close() {
	m.locker.Lock()
	defer m.locker.Unlock()

	for _, g := range m.groups {
		g.Close()
	}
	clear(m.groups)
}
