package handlers

import (
	"ws"

	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type Service struct {
	sync.RWMutex
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
		s.Unlock()
		httpErr(rw, "User already exists", http.StatusBadRequest)
		return
	}

	ws, err := ws.NewWS(rw, req)
	if err != nil {
		return
	}

	defer ws.Close()

	s.clients[user] = ws
	s.UsersListSend()

	defer func() {
		s.Lock()
		delete(s.clients, user)
		s.UsersListSend()
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

func (s *Service) PostMessage(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		httpErr(rw, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	decoder := json.NewDecoder(req.Body)
	var postMsg PostMessage
	if err := decoder.Decode(&postMsg); err != nil {
		httpErr(rw, fmt.Sprintf("ERROR JSON DECODE: %s", err), http.StatusInternalServerError)
	}
	req.Body.Close()

	msg, err := json.Marshal(&InMessage{
		For:     postMsg.For,
		Content: postMsg.Content,
	})
	if err != nil {
		httpErr(rw, fmt.Sprintf("ERROR JSON MARSHAL: %s", err), http.StatusInternalServerError)
		return
	}
	if err := s.SentFrom(postMsg.From, msg); err != nil {
		httpErr(rw, fmt.Sprintf("SEND ERROR: %s", err), http.StatusInternalServerError)
		return
	}
}

func httpErr(rw http.ResponseWriter, msg string, code int) {
	log.Print(msg)
	http.Error(rw, msg, code)
}
