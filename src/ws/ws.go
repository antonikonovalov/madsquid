package ws

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1 * 1024,
		WriteBufferSize: 1 * 1024,
	}

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	// (If you want send many tracks in one stream SDP may be very big)
	maxMessageSize int64 = 8 * 1024
)

type WS struct {
	conn *websocket.Conn
	in   chan []byte
	done chan struct{}
}

func NewWS(rw http.ResponseWriter, req *http.Request) (*WS, error) {
	wsConn, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		return nil, err
	}
	return &WS{
		conn: wsConn,
		in:   make(chan []byte, 100),
		done: make(chan struct{}),
	}, nil
}

func (s *WS) Send(msg []byte) error {
	if err := s.write(websocket.TextMessage, msg); err != nil {
		return err
	}
	return nil
}

func (s *WS) Close() error {
	if err := s.conn.Close(); err != nil {
		return err
	}
	<-s.done
	return nil
}

func (s *WS) Run() <-chan []byte {
	out := make(chan []byte)
	runSend := make(chan struct{})
	runReceive := make(chan struct{})
	go s.pingProcess(runSend)
	go s.receiveProcess(out, runReceive)

	var (
		okSend    = false
		okReceive = false
	)
	for {
		select {
		case <-runSend:
			okSend = true
		case <-runReceive:
			okReceive = true
		default:
			if okSend && okReceive {
				return out
			}
		}
	}

	return out
}

func (s *WS) pingProcess(runned chan<- struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		s.conn.Close()
	}()

	runned <- struct{}{}
	for {
		select {
		case <-ticker.C:
			if err := s.write(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("WEBSOCKET PING ERROR: %s", err)
				return
			}

		case <-s.done:
			return
		}
	}
}

func (s *WS) receiveProcess(out chan<- []byte, runned chan<- struct{}) {
	defer func() {
		close(out)
		close(s.done)
	}()

	s.conn.SetReadLimit(maxMessageSize)
	s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error { s.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	runned <- struct{}{}

	for {
		_, msg, err := s.conn.ReadMessage()
		if err != nil {
			log.Printf("WEBSOCKET RECEIVE ERROR: %s", err)
			if err = s.conn.Close(); err != nil {
				log.Printf("WEBSOCKET CLOSE ERROR: %s", err)
			}
			return
		}
		out <- msg
	}
}

func (s *WS) write(messageType int, data []byte) error {
	s.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := s.conn.WriteMessage(messageType, data); err != nil {
		return err
	}
	return nil
}
