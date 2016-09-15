package handlers

import (
	"ws"

	"log"
	"net/http"
	"sync"
)

type Service struct {
	sync.Mutex
	clients map[string]*ws.WS
}

func NewService() *Service {
	return &Service{
		clients: map[string]*ws.WS{},
	}
}

func (s *Service) WSHandle(rw http.ResponseWriter, req *http.Request) {
	user := req.FormValue("user")
	if user == "" {
		log.Print("Source user not found")
		return
	}
	s.Lock()
	if _, ok := s.clients[user]; ok {
		log.Print("User already exists")
		s.Unlock()
		http.Error(rw, "Not found type of message", http.StatusBadRequest)
		return
	}

	ws, err := ws.NewWS(rw, req)
	if err != nil {
		return
	}

	defer ws.Close()

	s.clients[user] = ws
	defer func() {
		s.Lock()
		delete(s.clients, user)
		s.Unlock()
	}()
	s.Unlock()

	ch := ws.Run()
	for msg := range ch {
		if err := s.SentFrom(user, msg); err != nil {
			log.Printf("SEND ERROR: %s", err)
		}
	}
}
