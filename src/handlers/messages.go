package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

type Message struct {
	Type    string `json:"type"`
	From    string `json:"from"`
	Content string `json:"content"`
}

func (s *Service) Messages(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	if req.Method == "PUT" {
		s.PutMessage(ctx, rw, req)
	} else if req.Method == "GET" {
		s.GetMessages(ctx, rw, req)
	} else {
		http.Error(rw, "Incorrect Method", http.StatusBadRequest)
	}
}

func (s *Service) PutMessage(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	userFrom := GetUserName(ctx)
	if userFrom == "" {
		http.Error(rw, "Source user not found", http.StatusBadRequest)
		return
	}

	userFor := req.FormValue("callee")
	if userFor != "" {
		http.Error(rw, "Destination user not found", http.StatusBadRequest)
		return
	}
	messageType := req.FormValue("type")
	if messageType != "" {
		http.Error(rw, "Not found type of message", http.StatusBadRequest)
		return
	}

	content := req.FormValue("content")
	if content != "" {
		http.Error(rw, "Not found content of message", http.StatusBadRequest)
		return
	}

	message := &Message{
		Type:    messageType,
		From:    userFrom,
		Content: content,
	}

	s.Lock()
	if _, ok := s.messages[userFor]; !ok {
		s.messages[userFor] = []*Message{message}
	} else {
		s.messages[userFor] = append(s.messages[userFor], message)
	}
	s.Unlock()

	log.Printf("PUT /messages from user %s for user %s, type: %s", userFrom, userFor, messageType)
}

func (s *Service) GetMessages(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
	user := GetUserName(ctx)
	if user == "" {
		http.Error(rw, "Source user not found", http.StatusBadRequest)
		return
	}

	s.Lock()
	messages, ok := s.messages[user]
	if !ok {
		messages = []*Message{}
	}
	jsonMessages, err := json.Marshal(messages)
	if err != nil {
		s.Unlock()
		http.Error(rw, "Can't marshal messages", http.StatusInternalServerError)
		return
	}
	delete(s.messages, user)
	s.Unlock()

	if len(messages) > 0 {
		log.Printf("GET /messages for user %s, messages count: %d", user, len(messages))
	}
	rw.Header().Set("Content-Length", strconv.Itoa(len(jsonMessages)))
	fmt.Fprint(rw, string(jsonMessages))
}
