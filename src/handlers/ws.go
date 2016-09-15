package handlers

import (
	"ws"

	"context"
	"log"
	"net/http"
)

func (s *Service) WSHandle(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	user := GetUserName(ctx)
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
