package kurento

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1 * 1024,
		WriteBufferSize: 1 * 1024,
		CheckOrigin: func(*http.Request) bool {
			return true
		},
	}

	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 40 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	// (If you want send many tracks in one stream SDP may be very big)
	maxMessageSize int64 = 8 * 1024
)

func NewService(ctx context.Context) (http.Handler, error) {
	cli, err := New(ctx)
	if err != nil {
		return nil, err
	}

	return &service{
		cli:   cli,
		lock:  &sync.RWMutex{},
		rooms: make(map[string]*Room, 0),
	}, nil
}

/*
{
  "rooms": {
    "Room Name": {
      // link to media pipe line
      "mediaPipeline": "mediaPipeline-1",
      // users of room
      "users": {
        "User Name First": {
          "wsConnection": "ws//sdfsdf",
          // stream from user browser
          "webrtcEndpointIn": "webrtcEndpointIn-1",
          // streams to user browser
          "webrtcEndpointOut": {
            // where webrtcEndpointIn-3 have connected to webrtcEndpointIn-2
            "User Name Second": {
              "point":"webrtcEndpointIn-3",
              "source":"webrtcEndpointIn-2"
            }
          }
        },
        "User Name Second": {
          "wsConnection": "ws//sdfsdf",
          // stream from user browser
          "webrtcEndpointIn": "webrtcEndpointIn-2",
          // streams to user browser
          "webrtcEndpointOut": {
            "User Name First": {
              "point":"webrtcEndpointIn-4",
              "source":"webrtcEndpointIn-1"
            }
          }
        }
      }
    }
  }
}
*/

type service struct {
	cli Kurento

	lock *sync.RWMutex
	// rooms registry
	rooms map[string]*Room
}

type WsCmd string

const (
	JoinRoomWsCmd              WsCmd = `joinRoom`
	ExistingParticipantsWsCmd  WsCmd = `existingParticipants`
	ReceiveVideoFromWsCmd      WsCmd = `receiveVideoFrom`
	ReceiveVideoAnswerWsCmd    WsCmd = `receiveVideoAnswer`
	NewParticipantArrivedWsCmd WsCmd = `newParticipantArrived`
	ParticipantLeavedWsCmd     WsCmd = `participantLeaved`
	OnIceCandidateWsCmd        WsCmd = `onIceCandidate`
	IceCandidateWsCmd          WsCmd = `iceCandidate`
	leaveWsCmd                 WsCmd = `leave`
	hangupWsCmd                WsCmd = `hangup`
)

type WsRequest struct {
	Cmd WsCmd `json:"cmd"`

	Room string `json:"room,omitempty"`
	User string `json:"user,omitempty"`

	Sender   string `json:"sender,omitempty"`
	SdpOffer string `json:"sdpOffer,omitempty"`

	Candidate *json.RawMessage `json:"candidate,omitempty"`
}

type WsErrAnswer struct {
	Request *WsRequest `json:"request"`
	Error   string     `json:"error"`
}

/*
 мы просто слушаем вебсокет

 потом пользователь присылает нам

 -> {"cmd":"joinRoom","room":"Room Name","user":"user1"}

 мы смотрим существует ли комната (если нет то создаем и медиа пайп в меди сервере)

 <- {"cmd":"existingParticipants","data":["test2","test1"]} // OR <- {"cmd":"existingParticipants","data":[]}

 далее пользователь должен отправить нам свой оффео для того чтобы начали получать от него стрим иначе никто его не увидит

 -> {"cmd":"receiveVideoFrom","sender":"user1","sdpOffer":"v=0\r\no=- 8086186447058305456 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE audio video\r\na=msid-semantic: WMS VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\nm=audio 58610 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126\r\nc=IN IP4 10.1.10.37\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=candidate:3883225187 1 udp 2122260223 10.1.10.37 58610 typ host generation 0 network-id 1\r\na=ice-ufrag:tIed\r\na=ice-pwd:LJ2l0LfqHstmOEiXI8YvWHsG\r\na=fingerprint:sha-256 9C:A0:CF:54:7D:40:3E:AB:2A:76:33:ED:62:BB:08:78:C1:D5:65:A1:83:7E:19:5C:86:1F:19:3C:FE:5D:08:C3\r\na=setup:actpass\r\na=mid:audio\r\na=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level\r\na=sendonly\r\na=rtcp-mux\r\na=rtpmap:111 opus/48000/2\r\na=rtcp-fb:111 transport-cc\r\na=fmtp:111 minptime=10;useinbandfec=1\r\na=rtpmap:103 ISAC/16000\r\na=rtpmap:104 ISAC/32000\r\na=rtpmap:9 G722/8000\r\na=rtpmap:0 PCMU/8000\r\na=rtpmap:8 PCMA/8000\r\na=rtpmap:106 CN/32000\r\na=rtpmap:105 CN/16000\r\na=rtpmap:13 CN/8000\r\na=rtpmap:110 telephone-event/48000\r\na=rtpmap:112 telephone-event/32000\r\na=rtpmap:113 telephone-event/16000\r\na=rtpmap:126 telephone-event/8000\r\na=ssrc:585125553 cname:dcqNeJ+yAB5VNYWu\r\na=ssrc:585125553 msid:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s 5233a3e1-e203-4e54-ad23-fae53fbd6274\r\na=ssrc:585125553 mslabel:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\na=ssrc:585125553 label:5233a3e1-e203-4e54-ad23-fae53fbd6274\r\nm=video 55247 UDP/TLS/RTP/SAVPF 96 98 100 102 127 97 99 101 125\r\nc=IN IP4 10.1.10.37\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=candidate:3883225187 1 udp 2122260223 10.1.10.37 55247 typ host generation 0 network-id 1\r\na=ice-ufrag:tIed\r\na=ice-pwd:LJ2l0LfqHstmOEiXI8YvWHsG\r\na=fingerprint:sha-256 9C:A0:CF:54:7D:40:3E:AB:2A:76:33:ED:62:BB:08:78:C1:D5:65:A1:83:7E:19:5C:86:1F:19:3C:FE:5D:08:C3\r\na=setup:actpass\r\na=mid:video\r\na=extmap:2 urn:ietf:params:rtp-hdrext:toffset\r\na=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\na=extmap:4 urn:3gpp:video-orientation\r\na=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\r\na=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay\r\na=sendonly\r\na=rtcp-mux\r\na=rtcp-rsize\r\na=rtpmap:96 VP8/90000\r\na=rtcp-fb:96 ccm fir\r\na=rtcp-fb:96 nack\r\na=rtcp-fb:96 nack pli\r\na=rtcp-fb:96 goog-remb\r\na=rtcp-fb:96 transport-cc\r\na=rtpmap:98 VP9/90000\r\na=rtcp-fb:98 ccm fir\r\na=rtcp-fb:98 nack\r\na=rtcp-fb:98 nack pli\r\na=rtcp-fb:98 goog-remb\r\na=rtcp-fb:98 transport-cc\r\na=rtpmap:100 H264/90000\r\na=rtcp-fb:100 ccm fir\r\na=rtcp-fb:100 nack\r\na=rtcp-fb:100 nack pli\r\na=rtcp-fb:100 goog-remb\r\na=rtcp-fb:100 transport-cc\r\na=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\na=rtpmap:102 red/90000\r\na=rtpmap:127 ulpfec/90000\r\na=rtpmap:97 rtx/90000\r\na=fmtp:97 apt=96\r\na=rtpmap:99 rtx/90000\r\na=fmtp:99 apt=98\r\na=rtpmap:101 rtx/90000\r\na=fmtp:101 apt=100\r\na=rtpmap:125 rtx/90000\r\na=fmtp:125 apt=102\r\na=ssrc-group:FID 3716766466 3368104928\r\na=ssrc:3716766466 cname:dcqNeJ+yAB5VNYWu\r\na=ssrc:3716766466 msid:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s a6358110-8b42-459a-83aa-bca8c8cab150\r\na=ssrc:3716766466 mslabel:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\na=ssrc:3716766466 label:a6358110-8b42-459a-83aa-bca8c8cab150\r\na=ssrc:3368104928 cname:dcqNeJ+yAB5VNYWu\r\na=ssrc:3368104928 msid:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s a6358110-8b42-459a-83aa-bca8c8cab150\r\na=ssrc:3368104928 mslabel:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\na=ssrc:3368104928 label:a6358110-8b42-459a-83aa-bca8c8cab150\r\n"}

на сервере мы видим что пользователь отправил нам сам от себя, то есть

user1 -> for user1 значит это его стрим и того мы просто создаем webrct точку от медиа пайпа и соединяем созданую webrtc точку с user1

когда пользователь отправляет такой запрос от первого пользователя

-> {"cmd":"receiveVideoFrom","sender":"user2","sdpOffer":"v=0\r\no=- 8086186447058305456 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=group:BUNDLE audio video\r\na=msid-semantic: WMS VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\nm=audio 58610 UDP/TLS/RTP/SAVPF 111 103 104 9 0 8 106 105 13 110 112 113 126\r\nc=IN IP4 10.1.10.37\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=candidate:3883225187 1 udp 2122260223 10.1.10.37 58610 typ host generation 0 network-id 1\r\na=ice-ufrag:tIed\r\na=ice-pwd:LJ2l0LfqHstmOEiXI8YvWHsG\r\na=fingerprint:sha-256 9C:A0:CF:54:7D:40:3E:AB:2A:76:33:ED:62:BB:08:78:C1:D5:65:A1:83:7E:19:5C:86:1F:19:3C:FE:5D:08:C3\r\na=setup:actpass\r\na=mid:audio\r\na=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level\r\na=sendonly\r\na=rtcp-mux\r\na=rtpmap:111 opus/48000/2\r\na=rtcp-fb:111 transport-cc\r\na=fmtp:111 minptime=10;useinbandfec=1\r\na=rtpmap:103 ISAC/16000\r\na=rtpmap:104 ISAC/32000\r\na=rtpmap:9 G722/8000\r\na=rtpmap:0 PCMU/8000\r\na=rtpmap:8 PCMA/8000\r\na=rtpmap:106 CN/32000\r\na=rtpmap:105 CN/16000\r\na=rtpmap:13 CN/8000\r\na=rtpmap:110 telephone-event/48000\r\na=rtpmap:112 telephone-event/32000\r\na=rtpmap:113 telephone-event/16000\r\na=rtpmap:126 telephone-event/8000\r\na=ssrc:585125553 cname:dcqNeJ+yAB5VNYWu\r\na=ssrc:585125553 msid:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s 5233a3e1-e203-4e54-ad23-fae53fbd6274\r\na=ssrc:585125553 mslabel:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\na=ssrc:585125553 label:5233a3e1-e203-4e54-ad23-fae53fbd6274\r\nm=video 55247 UDP/TLS/RTP/SAVPF 96 98 100 102 127 97 99 101 125\r\nc=IN IP4 10.1.10.37\r\na=rtcp:9 IN IP4 0.0.0.0\r\na=candidate:3883225187 1 udp 2122260223 10.1.10.37 55247 typ host generation 0 network-id 1\r\na=ice-ufrag:tIed\r\na=ice-pwd:LJ2l0LfqHstmOEiXI8YvWHsG\r\na=fingerprint:sha-256 9C:A0:CF:54:7D:40:3E:AB:2A:76:33:ED:62:BB:08:78:C1:D5:65:A1:83:7E:19:5C:86:1F:19:3C:FE:5D:08:C3\r\na=setup:actpass\r\na=mid:video\r\na=extmap:2 urn:ietf:params:rtp-hdrext:toffset\r\na=extmap:3 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\na=extmap:4 urn:3gpp:video-orientation\r\na=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\r\na=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay\r\na=sendonly\r\na=rtcp-mux\r\na=rtcp-rsize\r\na=rtpmap:96 VP8/90000\r\na=rtcp-fb:96 ccm fir\r\na=rtcp-fb:96 nack\r\na=rtcp-fb:96 nack pli\r\na=rtcp-fb:96 goog-remb\r\na=rtcp-fb:96 transport-cc\r\na=rtpmap:98 VP9/90000\r\na=rtcp-fb:98 ccm fir\r\na=rtcp-fb:98 nack\r\na=rtcp-fb:98 nack pli\r\na=rtcp-fb:98 goog-remb\r\na=rtcp-fb:98 transport-cc\r\na=rtpmap:100 H264/90000\r\na=rtcp-fb:100 ccm fir\r\na=rtcp-fb:100 nack\r\na=rtcp-fb:100 nack pli\r\na=rtcp-fb:100 goog-remb\r\na=rtcp-fb:100 transport-cc\r\na=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\na=rtpmap:102 red/90000\r\na=rtpmap:127 ulpfec/90000\r\na=rtpmap:97 rtx/90000\r\na=fmtp:97 apt=96\r\na=rtpmap:99 rtx/90000\r\na=fmtp:99 apt=98\r\na=rtpmap:101 rtx/90000\r\na=fmtp:101 apt=100\r\na=rtpmap:125 rtx/90000\r\na=fmtp:125 apt=102\r\na=ssrc-group:FID 3716766466 3368104928\r\na=ssrc:3716766466 cname:dcqNeJ+yAB5VNYWu\r\na=ssrc:3716766466 msid:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s a6358110-8b42-459a-83aa-bca8c8cab150\r\na=ssrc:3716766466 mslabel:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\na=ssrc:3716766466 label:a6358110-8b42-459a-83aa-bca8c8cab150\r\na=ssrc:3368104928 cname:dcqNeJ+yAB5VNYWu\r\na=ssrc:3368104928 msid:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s a6358110-8b42-459a-83aa-bca8c8cab150\r\na=ssrc:3368104928 mslabel:VZYx5AI05sUhSjKTISF9VpNJtTzc1ikz6y7s\r\na=ssrc:3368104928 label:a6358110-8b42-459a-83aa-bca8c8cab150\r\n"}
user1 -> for user2 значит первый пользователь хочет увидель второго пользователя и для этого мы

1 - проверяем что user2 существует и у его есть точка в медиа пайпе
2 - создаем новую webrtcX точку и коннектим её к  user2.webrtcIn
3 - процессим оффер для webrtcX пользователя user1

*/
func (s *service) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, `_schema`) {
		s.lock.RLock()
		json.NewEncoder(rw).Encode(s.rooms)
		s.lock.RUnlock()
		return
	}

	wsConn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
	defer wsConn.Close()

	wsConn.SetReadLimit(maxMessageSize)
	wsConn.SetReadDeadline(time.Now().Add(pongWait))
	wsConn.SetPongHandler(func(string) error { wsConn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	ctx := r.Context()

	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer func() {
			ticker.Stop()
			wsConn.Close()
		}()

		for {
			select {
			case <-ticker.C:
				if err := wsConn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
					log.Printf("WEBSOCKET PING ERROR: %s", err)
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	messages := make(chan *WsRequest, 0)
	go func() {
		for {
			wsReq := &WsRequest{}
			// get offer and send to media server
			err := wsConn.ReadJSON(wsReq)
			if err != nil {
				log.Println("read:", err)
				close(messages)
				break
			}
			messages <- wsReq
		}
	}()

	currentUser := NewUser("", wsConn, nil)

	defer func(user *User) {
		err = s.leave(context.Background(), user)
		if err != nil {
			log.Printf(`can't correct leave current session of user %s(%s)': %s`, currentUser.name, currentUser.roomName, err)
		}
	}(currentUser)
	for {

		select {
		// processing message from webSocket
		case wsReq, ok := <-messages:
			if !ok {
				return
			}
			log.Printf("start processed %s", wsReq.Cmd)

			switch wsReq.Cmd {
			case JoinRoomWsCmd:
				log.Print("started call joinRoom")
				err = s.joinRoom(ctx, currentUser, wsReq)
				log.Print("ended call joinRoom")
			case ReceiveVideoFromWsCmd:
				log.Print("started call receiveVideoFrom")
				err = s.receiveVideoFrom(ctx, currentUser, wsReq)
				log.Print("ended call receiveVideoFrom")
			case OnIceCandidateWsCmd:
				log.Print("started call onIceCandidate")
				err = s.onIceCandidate(ctx, currentUser, wsReq)
				log.Print("ended call onIceCandidate")
			case leaveWsCmd:
				log.Print("started call leaveWsCmd")
				err = s.leave(ctx, currentUser)
				log.Print("ended call leaveWsCmd")
			case hangupWsCmd:
				log.Print("started call hangupWsCmd")
				err = s.hangUp(ctx, currentUser, wsReq)
				log.Print("ended call hangupWsCmd")
			}

			// error processing
			if err != nil {
				log.Printf(`cmd:%s: err: %s`, wsReq.Cmd, err)
				wsConn.SetWriteDeadline(time.Now().Add(writeWait))
				err = wsConn.WriteJSON(&WsErrAnswer{Request: wsReq, Error: err.Error()})
				if err != nil {
					log.Printf(`can't write to web socket: %s'`, err)
					return
				}
			}

		case <-ctx.Done():
			err := ctx.Err()
			log.Printf(`context of web socket is done: %s`, err)
			return
		}
	}
}

func (s *service) hangUp(ctx context.Context, currentUser *User, req *WsRequest) error {
	currentUser.lock.RLock()
	connector, ok := currentUser.Out[req.Sender]
	currentUser.lock.RUnlock()

	if !ok {
		return nil
	}

	// удаяем точку выхода этого пользователя для себя
	err := s.cli.Release(ctx, connector.Point)
	if err != nil {
		return err
	}

	currentUser.lock.Lock()
	delete(currentUser.Out, req.Sender)
	currentUser.lock.Unlock()

	return nil
}

type ExistingParticipantsForm struct {
	Cmd  WsCmd    `json:"cmd"`
	Data []string `json:"data"`
}

type NewParticipantArrivedForm struct {
	Cmd  WsCmd  `json:"cmd"`
	Name string `json:"name"`
}

//{"id":"receiveVideoAnswer","name":"test1","sdpAnswer":
type ReceiveVideoAnswerForm struct {
	Cmd       WsCmd            `json:"cmd"`
	Name      string           `json:"name"`
	SdpAnswer *json.RawMessage `json:"sdpAnswer"`
}

func (s *service) onIceCandidate(ctx context.Context, currentUser *User, req *WsRequest) error {
	var err error
	if currentUser.name == req.Sender {
		err = s.cli.Invoke(ctx, currentUser.In, AddIceCandidateInvokeOperation, req.Candidate)
		if err != nil {
			return err
		}
	} else {
		currentUser.lock.Lock()
		connectorMedia, ok := currentUser.Out[req.Sender]
		currentUser.lock.Unlock()
		if !ok {
			return fmt.Errorf("[%s] user %s not found in out %s ", req.Cmd, req.Sender, currentUser.name)
		}
		err = s.cli.Invoke(ctx, connectorMedia.Point, AddIceCandidateInvokeOperation, req.Candidate)
		if err != nil {
			return err
		}
	}

	return nil
}

type ParticipantLeavedForm struct {
	Cmd  WsCmd  `json:"cmd"`
	Name string `json:"name"`
}

func (s *service) leave(ctx context.Context, currentUser *User) error {
	if currentUser == nil {
		return fmt.Errorf("currentUser for req %s is nil for leave ")
	}

	s.lock.Lock()
	currentRoom, ok := s.rooms[currentUser.roomName]
	s.lock.Unlock()

	var userName = currentUser.name
	if !ok {
		return fmt.Errorf("currentRoom %s for req %s is nil ", currentUser.roomName, "leave")
	}
	if !currentRoom.HasUser(userName) {
		return nil
	}

	var removeRoomNeeded = false
	currentRoom.lock.Lock()
	delete(currentRoom.Users, userName)
	err := s.cli.Release(ctx, currentUser.In)
	if err != nil {
		log.Printf("ERR: can't release object %s", currentUser.In.ID)
	}

	// удаляем видео которе стримят к нашему пользователю другие пользователи
	for _, connector := range currentUser.Out {
		err := s.cli.Release(ctx, connector.Point)
		if err != nil {
			log.Printf("ERR: can't release object %s", connector.Point.ID)
		}
	}
	// удалем видео нашего пользователя которое стримется другим пользователям
	for _, user := range currentRoom.Users {
		user.lock.RLock()
		connectToUser, ok := user.Out[userName]
		user.lock.RUnlock()

		if ok {
			err := s.cli.Release(ctx, connectToUser.Point)
			if err != nil {
				log.Printf("ERR: can't release object %s", connectToUser.Point.ID)
			}

			user.lock.Lock()
			delete(user.Out, userName)
			user.lock.Unlock()
		}
		// точки нет , но нужно донести на UI
		user.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
		user.lock.Lock()
		_ = user.wsConn.WriteJSON(&ParticipantLeavedForm{
			Cmd:  ParticipantLeavedWsCmd,
			Name: userName,
		})
		user.lock.Unlock()
	}

	if len(currentRoom.Users) == 0 {
		removeRoomNeeded = true
	}
	currentRoom.lock.Unlock()

	if removeRoomNeeded {
		err := s.cli.Release(ctx, currentRoom.MediaPipeline)
		if err != nil {
			log.Printf("ERR: can't release object %s", currentRoom.MediaPipeline.ID)
		}
		s.lock.Lock()
		delete(s.rooms, currentUser.roomName)
		s.lock.Unlock()
	}

	return nil
}

type IceCandidateAnswer struct {
	Cmd       WsCmd            `json:"cmd"`
	Name      string           `json:"name"`
	Candidate *json.RawMessage `json:"candidate"`
}

func (s *service) receiveVideoFrom(ctx context.Context, currentUser *User, req *WsRequest) error {
	if currentUser == nil {
		return fmt.Errorf("currentUser for req %s is nil ", req.Cmd)
	}

	s.lock.Lock()
	currentRoom, ok := s.rooms[currentUser.roomName]
	s.lock.Unlock()

	if !ok {
		return fmt.Errorf("currentRoom %s for req %s is nil ", currentUser.roomName, req.Cmd)
	}

	var (
		err               error
		sinkMediaObject   *MediaObject
		needNotification  bool
		AnswerForUserName = req.Sender
	)

	log.Printf("currentUser.name %s <=> %s", currentUser.name, req.Sender)
	if currentUser.name == req.Sender {
		needNotification = true

		err = s.cli.Create(ctx, currentUser.In)
		if err != nil {
			return err
		}
		sinkMediaObject = currentUser.In

	} else {
		currentRoom.lock.RLock()
		sourceUser, ok := currentRoom.Users[req.Sender]
		currentRoom.lock.RUnlock()
		if !ok {
			raw, _ := json.MarshalIndent(currentRoom, "", "\t")
			return fmt.Errorf("can't find user %s in room : %s", req.Sender, string(raw))
		}

		sinkMediaObject = &MediaObject{
			Parent: currentRoom.MediaPipeline,
			Type:   WebRtcEndpoint,
		}

		err = s.cli.Create(ctx, sinkMediaObject)
		if err != nil {
			return err
		}

		sink := &struct {
			Sink string `json:"sink"`
		}{
			Sink: sinkMediaObject.ID,
		}

		raw, err := json.Marshal(sink)
		if err != nil {
			return err
		}

		payload := &json.RawMessage{}
		_ = payload.UnmarshalJSON(raw)
		err = s.cli.Invoke(ctx, sourceUser.In, ConnectInvokeOperation, payload)
		if err != nil {
			return err
		}

		currentUser.lock.Lock()
		currentUser.Out[sourceUser.name] = &MediaConnector{
			Point:  sinkMediaObject,
			Source: sourceUser.In,
		}
		currentUser.lock.Unlock()
	}

	eventIceCandidateFound, err := s.cli.Subscribe(ctx, sinkMediaObject, IceCandidateFound)
	if err != nil {
		return err
	}

	// add listener - closed, when closed WS
	go func() {
		for event := range eventIceCandidateFound {
			answer := &IceCandidateAnswer{}
			_ = json.Unmarshal(event, answer)
			answer.Cmd = IceCandidateWsCmd
			answer.Name = AnswerForUserName
			currentUser.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
			currentUser.lock.Lock()
			_ = currentUser.wsConn.WriteJSON(answer)
			currentUser.lock.Unlock()
		}
	}()

	raw, err := json.Marshal(map[string]string{"offer": req.SdpOffer})
	if err != nil {
		return err
	}

	payload := &json.RawMessage{}
	_ = payload.UnmarshalJSON(raw)
	// process Offer
	err = s.cli.Invoke(ctx, sinkMediaObject, ProcessOfferInvokeOperation, payload)
	if err != nil {
		return err
	}

	currentUser.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
	currentUser.lock.Lock()
	err = currentUser.wsConn.WriteJSON(&ReceiveVideoAnswerForm{
		Cmd:       ReceiveVideoAnswerWsCmd,
		Name:      AnswerForUserName,
		SdpAnswer: payload,
	})
	currentUser.lock.Unlock()
	if err != nil {
		return err
	}

	// fire event!
	err = s.cli.Invoke(ctx, sinkMediaObject, GatherCandidatesInvokeOperation, nil)
	if err != nil {
		return err
	}

	if !needNotification {
		return nil
	}

	// NOTIFICATION
	// только после того как пользователь создал webrct создедиение - мы говорим что он есть
	users := []string{}
	//time.Sleep(100 * time.Millisecond)
	for _, user := range currentRoom.ListUsers() {
		if user.name == currentUser.name {
			continue
		}

		user.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
		user.lock.Lock()
		err = user.wsConn.WriteJSON(&NewParticipantArrivedForm{
			Cmd:  NewParticipantArrivedWsCmd,
			Name: currentUser.name,
		})
		user.lock.Unlock()
		if err != nil {
			continue
		}
		if len(user.In.ID) != 0 {
			users = append(users, user.name)
		}
	}

	// и отправляем ему данные о других практикантах
	currentUser.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
	currentUser.lock.Lock()
	err = currentUser.wsConn.WriteJSON(&ExistingParticipantsForm{
		Cmd:  ExistingParticipantsWsCmd,
		Data: users,
	})
	currentUser.lock.Unlock()

	return err
}

func (s *service) joinRoom(ctx context.Context, currentUser *User, req *WsRequest) error {
	var (
		err  error
		room *Room
		ok   bool
	)

	s.lock.Lock()
	room, ok = s.rooms[req.Room]
	s.lock.Unlock()

	if !ok {
		room = NewRoom()
		err = s.cli.Create(ctx, room.MediaPipeline)
		if err != nil {
			return err
		}
		// устанавливаем рум без пользователя
		// пользователь появиться после того как там появиться webrtcEndpoint
		// но таким образом пользоватль может присоеденить туда в любой момент - хоть все сразу(после лока :)))))
		s.lock.Lock()
		s.rooms[req.Room] = room
		s.lock.Unlock()
	} else {
		if room.HasUser(req.User) {
			return fmt.Errorf(`user %s already exist in room %s`, req.User, req.Room)
		}
	}

	// JOIN, BUT HIDE
	currentUser.roomName = req.Room
	currentUser.name = req.User
	currentUser.In = &MediaObject{
		Parent: room.MediaPipeline,
		Type:   WebRtcEndpoint,
	}

	room.AddUser(currentUser)

	return nil
}

/*
{
      // link to media pipe line
      "mediaPipeline": "mediaPipeline-1",
      // users of room
      "users": {
        "User Name First": {
          "wsConnection": "ws//sdfsdf",
          // stream from user browser
          "webrtcEndpointIn": "webrtcEndpointIn-1",
          // streams to user browser
          "webrtcEndpointOut": {
            // where webrtcEndpointIn-3 have connected to webrtcEndpointIn-2
            "User Name Second": {
              "point":"webrtcEndpointIn-3",
              "source":"webrtcEndpointIn-2"
            }
          }
        },
        "User Name Second": {
          "wsConnection": "ws//sdfsdf",
          // stream from user browser
          "webrtcEndpointIn": "webrtcEndpointIn-2",
          // streams to user browser
          "webrtcEndpointOut": {
            "User Name First": {
              "point":"webrtcEndpointIn-4",
              "source":"webrtcEndpointIn-1"
            }
          }
        }
      }
    }
*/

type MediaConnector struct {
	Point  *MediaObject `json:"point"`
	Source *MediaObject `json:"source"`
}

func NewUser(name string, c *websocket.Conn, in *MediaObject) *User {
	return &User{
		name:   name,
		wsConn: c,
		lock:   &sync.RWMutex{},
		In:     in,
		Out:    make(map[string]*MediaConnector, 0),
	}
}

type User struct {
	name string        `json:"-"`
	lock *sync.RWMutex `json:"-"`
	// websocket
	wsConn *websocket.Conn `json:"-"`
	// in stream
	In       *MediaObject               `json:"in"`
	Out      map[string]*MediaConnector `json:"out"`
	roomName string                     `json:"-"`
}

func (u *User) IsEmptyIn() bool {
	return len(u.In.ID) == 0
}

func NewRoom() *Room {
	return &Room{
		lock:          &sync.RWMutex{},
		MediaPipeline: &MediaObject{Type: MediaPipeline},
		Users:         make(map[string]*User, 0),
	}
}

type Room struct {
	lock *sync.RWMutex `json:"-"`
	// link to media pipe line
	MediaPipeline *MediaObject `json:"media_pipeline"`
	// users of room
	Users map[string]*User `json:"users"`
}

func (r *Room) ListUsers() []*User {
	users := []*User{}
	r.lock.RLock()
	for _, u := range r.Users {
		users = append(users, u)
	}
	r.lock.RUnlock()
	return users
}

func (r *Room) HasUser(name string) bool {
	r.lock.RLock()
	_, ok := r.Users[name]
	r.lock.RUnlock()
	return ok
}

func (r *Room) AddUser(u *User) {
	r.lock.Lock()
	r.Users[u.name] = u
	r.lock.Unlock()
}
