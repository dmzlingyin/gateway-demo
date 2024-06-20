package server

import (
	"github.com/cuigh/auxo/log"
	"github.com/lesismal/nbio/nbhttp/websocket"
	"net/http"
	"time"
)

func NewServer() *Server {
	s := &Server{
		sm:     NewSessionMap(),
		logger: log.Get("server.websocket"),
	}
	s.upgrader = s.newUpgrader()
	return s
}

type Server struct {
	server   *http.Server
	sm       *SessionMap
	upgrader *websocket.Upgrader
	logger   log.Logger
}

func (s *Server) Serve() (err error) {
	s.server = &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(s.handleRequest),
	}
	s.logger.Info("server started at ", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("failed to upgrade connection: ", err)
		return
	}

	tid := r.URL.Query().Get("tid")
	uid := r.URL.Query().Get("uid")
	if tid == "" || uid == "" {
		s.logger.Errorf("empty teamID or UserID, tid: %s, uid: %s", tid, uid)
		_ = conn.Close()
		return
	}

	session := NewSession(tid, uid, conn)
	s.sm.Add(session)

	// 间隔 10s 发送一次消息
	go s.SendMessage(tid)

	if err = session.Start(); err != nil {
		s.logger.Errorf("session exited, tid: %s, uid: %s, err: %s ", tid, uid, err)
	}
}

func (s *Server) SendMessage(tid string) {
	g := s.Sessions().Get(tid)
	if g == nil {
		panic(nil)
	}
	for {
		for _, session := range g.Sessions {
			if err := session.Send(g, []byte("hello")); err != nil {
				panic(err)
			}
		}
		time.Sleep(10 * time.Second)
	}
}

func (s *Server) newUpgrader() *websocket.Upgrader {
	u := websocket.NewUpgrader()
	u.SetPingHandler(func(c *websocket.Conn, appData string) {
		if err := c.WriteMessage(websocket.PongMessage, nil); err != nil {
			s.logger.Errorf("failed to send pong message to client: %v", err)
		}
	})
	u.OnOpen(func(c *websocket.Conn) {
		s.logger.Debugf("OnOpen: %s", c.RemoteAddr().String())
	})
	u.OnMessage(func(c *websocket.Conn, messageType websocket.MessageType, data []byte) {
		switch messageType {
		case websocket.BinaryMessage:
			s.logger.Infof("message: %s", string(data))
		default:
			s.logger.Errorf("got unexpect message type: %v", messageType)
		}
	})
	u.OnClose(func(c *websocket.Conn, err error) {
		s.logger.Debugf("OnClose: %s, err: %s", c.RemoteAddr().String(), err)
		s.sm.Remove(c)
	})
	return u
}

// Sessions 获取所有会话
func (s *Server) Sessions() *SessionMap {
	return s.sm
}

func (s *Server) Close(timeout time.Duration) {
	if s.server != nil {
		_ = s.server.Close()
		s.server = nil
	}

	// close all sessions
	finished := make(chan struct{})
	go func() {
		s.sm.Close()
		finished <- struct{}{}
	}()

	// wait for all sessions closed
	select {
	case <-time.After(timeout):
		s.logger.Warn("server closed by timeout")
	case <-finished:
		s.logger.Info("server closed")
	}
}
