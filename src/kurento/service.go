package kurento

import (
	"context"
	"log"
	"net/http"
)

func NewService(ctx context.Context) (http.Handler, error) {
	cli, err := New(ctx)
	if err != nil {
		return nil, err
	}

	root := &MediaObject{
		Type: MediaPipeline,
	}

	// создаем комнату сразу
	err = cli.Create(context.Background(), root)
	if err != nil {
		return nil, err
	}

	return &service{room: &Room{root
	}}, nil
}

type service struct {
	room *Room
}

func (s *service) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	userName := r.FormValue("user")
	if userName == "" {
		log.Print("Source user not found")
		return
	}

	wsConn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	defer wsConn.Close()

	userMediaObject := &MediaObject{
		Parent: s.root,
		Type:   WebRtcEndpoint,
	}
	err = s.cli.Create(r.Context(), userMediaObject)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	iceCandidateFoundChannel, err := s.cli.Subscribe(r.Context(), userMediaObject, IceCandidateFound)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}

	for {
		// get offer and send to media server
		mt, message, err := wsConn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)

		err = wsConn.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}

}

type RoomManager interface {
	NewRoom() (*Room,error)
	AddUserToRoom(*Room,userName string) error
}

type Room struct {
	MediaPipeline *MediaObject
}
