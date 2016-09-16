package handlers

import (
	"encoding/json"
	"fmt"
)

type InMessage struct {
	For     string          `json:"callee"`
	Content json.RawMessage `json:"content"`
}

type OutMessage struct {
	From    string          `json:"from"`
	Content json.RawMessage `json:"content"`
}

func (s *Service) SentFrom(userFrom string, m []byte) error {
	var err error
	inMsg := &InMessage{}
	if err = json.Unmarshal(m, inMsg); err != nil {
		return err
	} else {
		ws, ok := s.clients[inMsg.For]
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
