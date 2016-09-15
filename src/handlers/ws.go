package handlers

import (
	"ws"

	"context"
	// "encoding/json"
	// "golang.org/x/net/websocket"
	"net/http"
	// "github.com/gorilla/websocket"
	"log"
)

// type WSHandler struct {
// 	service *Service
// 	user    string
// }

// func NewWSHandler(service *Service, user string) *WSHandler {
// 	return &WSHandler{
// 		service: service,
// 		user:    user,
// 	}
// }

func (s *Service) WSHandle(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	// ctx := SetUserName(context.Background(), wsConn.Request())
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

	// wsConn, err := upgrader.Upgrade(w, r, nil)
	// if err != nil {
	// 	log.Print(err)
	// 	return
	// }

	s.clients[user] = ws
	defer func() {
		s.Lock()
		delete(s.clients, user)
		s.Unlock()
	}()
	s.Unlock()

	ch := ws.Run()
	for msg := range ch {
		// log.Print(string(msg))
		if err := s.SentFrom(user, msg); err != nil {
			log.Printf("SEND ERROR: %s", err)
		}
	}
}
