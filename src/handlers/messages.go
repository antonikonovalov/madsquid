package handlers

import (
	"encoding/json"
	"fmt"
	"log"
)

type PostMessage struct {
	For     string          `json:"for"`
	From    string          `json:"from"`
	Content json.RawMessage `json:"content"`
}

type InMessage struct {
	For     string          `json:"for"`
	Content json.RawMessage `json:"content"`
}

type OutMessage struct {
	From    string          `json:"from"`
	Content json.RawMessage `json:"content"`
}

type UsersListMessage struct {
	Content struct {
		Type  string   `json:"type"`
		Users []string `json:"users"`
	} `json:"content"`
}

func (s *Service) SentFrom(userFrom string, m []byte) error {
	var err error
	inMsg := &InMessage{}
	if err = json.Unmarshal(m, inMsg); err != nil {
		return err
	} else {

		s.RLock()
		ws, ok := s.clients[inMsg.For]
		s.RUnlock()
		if !ok {
			return fmt.Errorf("destination user not registered: %s", inMsg.For)
		}
		outM := &OutMessage{From: userFrom, Content: inMsg.Content}
		if m, err = json.Marshal(outM); err != nil {
			return err
		}
		if err = ws.Send(m); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UsersListSend() {
	users := []string{}
	for u, _ := range s.clients {
		users = append(users, u)
	}

	msg := &UsersListMessage{
		Content: struct {
			Type  string   `json:"type"`
			Users []string `json:"users"`
		}{
			Type:  "users",
			Users: users,
		},
	}

	m, err := json.Marshal(msg)
	if err != nil {
		log.Printf("MARSHAL ERROR: %s", err)
		return
	}

	for _, ws := range s.clients {
		if err = ws.Send(m); err != nil {
			log.Printf("WEBSOCKET SENT ERROR: %s", err)
		}
	}
}
