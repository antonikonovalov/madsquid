package handlers

import (
	"context"
	"net/http"
	"strings"
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

	pathParts := strings.Split(req.URL.Path, "/")
	if len(pathParts) != 2 || pathParts[1] == "" {
		http.Error(rw, "Destination user not found", http.StatusBadRequest)
		return
	}

	userFor := pathParts[1]

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
}

func (s *Service) GetMessages(ctx context.Context, rw http.ResponseWriter, req *http.Request) {
}
