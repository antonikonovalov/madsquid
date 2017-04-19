package kurento

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sync"

	"errors"
	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
)

var addr = flag.String(`kurento.addr`, `ws://localhost:8888/kurento`, `set your kurento media server WS endpoint`)

func New(ctx context.Context) (Kurento, error) {
	c, _, err := websocket.DefaultDialer.Dial(*addr, nil)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	//configure connection here
	cli := &kurentoClient{
		cctx:      ctx,
		ws:        c,
		queueLock: sync.RWMutex{},
		queue:     make(map[string]chan *response, 0),
		cancel:    cancel,
	}

	go cli.loop()
	return cli, nil
}

type MediaType string

const (
	// MediaPipeline: Media Pipeline to be created.
	MediaPipeline MediaType = `MediaPipeline`
	// WebRtcEndpoint: This Endpoint offers media streaming using WebRTC
	WebRtcEndpoint MediaType = `WebRtcEndpoint`
	// RtpEndpoint: Endpoint that provides bidirectional content delivery capabilities with remote networked peers through RTP protocol.
	// It contains paired sink and source MediaPad for audio and video.
	RtpEndpoint MediaType = `RtpEndpoint`
	// HttpPostEndpoint: This type of Endpoint provide unidirectional communications. Its MediaSource are related to HTTP POST method.
	// It contains sink MediaPad for audio and video, which provide access to an HTTP file upload function.
	HttpPostEndpoint MediaType = `HttpPostEndpoint`
	// PlayerEndpoint: It provides function to retrieve contents from seekable sources in reliable mode (does not discard media information) and inject them into KMS.
	// It contains one MediaSource for each media type detected.
	PlayerEndpoint MediaType = `PlayerEndpoint`
	// RecorderEndpoint: Provides function to store contents in reliable mode (doesn't discard data).
	// It contains MediaSink pads for audio and video.
	RecorderEndpoint MediaType = `RecorderEndpoint`
	// FaceOverlayFilter: It detects faces in a video feed. The face is then overlaid with an image.
	FaceOverlayFilter MediaType = `FaceOverlayFilter`
	// ZBarFilter: This Filter detects QR and bar codes in a video feed. When a code is found, the filter raises a CodeFound.
	ZBarFilter MediaType = `ZBarFilter`
	// GStreamerFilter: This is a generic Filter interface, that creates GStreamer filters in the media server.
	GStreamerFilter MediaType = `GStreamerFilter`
	// Composite: A Hub that mixes the audio stream of its connected sources and constructs a grid with the video streams of its connected sources into its sink.
	Composite MediaType = `Composite`
	// Dispatcher: A Hub that allows routing between arbitrary port pairs.
	Dispatcher MediaType = `Dispatcher`
	// DispatcherOneToMany: A Hub that sends a given source to all the connected sinks.
	DispatcherOneToMany MediaType = `DispatcherOneToMany`
)

type InvokeOperation string

const (
	// connect. Connect two media elements.
	ConnectInvokeOperation InvokeOperation = `connect`
	// play. Start the play of a media (PlayerEndpoint).
	PlayInvokeOperation InvokeOperation = `play`
	// record. Start the record of a media (RecorderEndpoint).
	RecordInvokeOperation InvokeOperation = `record`
	// setOverlayedImage. Set the image that is going to be overlaid on the detected faces in a media stream (FaceOverlayFilter).
	SetOverlayedImageInvokeOperation InvokeOperation = `setOverlayedImage`
	// processOffer. Process the offer in the SDP negotiation (WebRtcEndpoint).
	ProcessOfferInvokeOperation InvokeOperation = `processOffer`
	//gatherCandidates. Start the ICE candidates gathering to establish a WebRTC media session (WebRtcEndpoint).
	GatherCandidatesInvokeOperation InvokeOperation = `gatherCandidates`
	//addIceCandidate. Add ICE candidate (WebRtcEndpoint).
	AddIceCandidateInvokeOperation InvokeOperation = `addIceCandidate`
)

type SubscribeTopic string

const (
	// CodeFoundEvent: raised by a ZBarFilter when a code is found in the data being streamed.
	CodeFoundEvent SubscribeTopic = `CodeFoundEvent`
	// ConnectionStateChanged: Indicates that the state of the connection has changed.
	ConnectionStateChanged SubscribeTopic = `ConnectionStateChanged`
	// ElementConnected: Indicates that an element has been connected to other.
	ElementConnected SubscribeTopic = `ElementConnected`
	// ElementDisconnected: Indicates that an element has been disconnected.
	ElementDisconnected SubscribeTopic = `ElementDisconnected`
	// EndOfStream: Event raised when the stream that the element sends out is finished.
	EndOfStream SubscribeTopic = `EndOfStream`
	// Error: An error related to the MediaObject has occurred.
	ErrorEvent SubscribeTopic = `Error`
	// MediaSessionStarted: Event raised when a session starts. This event has no data.
	MediaSessionStarted SubscribeTopic = `MediaSessionStarted`
	// MediaSessionTerminated: Event raised when a session is terminated. This event has no data.
	MediaSessionTerminated SubscribeTopic = `MediaSessionTerminated`
	// MediaStateChanged: Indicates that the state of the media has changed.
	MediaStateChanged SubscribeTopic = `MediaStateChanged`
	// ObjectCreated: Indicates that an object has been created on the media server.
	ObjectCreated SubscribeTopic = `ObjectCreated`
	// ObjectDestroyed: Indicates that an object has been destroyed on the media server.
	ObjectDestroyed SubscribeTopic = `ObjectDestroyed`
	// OnIceCandidate: Notify of a new gathered local candidate.
	OnIceCandidate SubscribeTopic = `OnIceCandidate`
	// OnIceComponentStateChanged: Notify about the change of an ICE component state.
	OnIceComponentStateChanged SubscribeTopic = `OnIceComponentStateChanged`
	// OnIceGatheringDone: Notify that all candidates have been gathered.
	OnIceGatheringDone SubscribeTopic = `OnIceGatheringDone`

	IceCandidateFound SubscribeTopic = `IceCandidateFound`
)

type ConstructorParams struct {
	//  (optional, string): This parameter is only mandatory for Media Elements.
	// In that case, the value of this parameter is the identifier of the media pipeline which is going to contain the Media Element to be created.
	MediaPipeline *string `json:"mediaPipeline,omitempty"`
	// (optional, string): This parameter is only required for Media Elements such as PlayerEndpoint or RecorderEndpoint.
	// It is an URI used in the Media Element, i.e. the media to be played (for PlayerEndpoint) or the location of the recording (for RecorderEndpoint).
	Uri *string `json:"uri,omitempty"`
}

type MediaObject struct {
	Parent *MediaObject
	ID     string
	Type   MediaType
}

func (m *MediaObject) String() string {
	var str string
	if m.Parent != nil {
		str = m.Parent.ID
	}

	return str + `/` + m.ID
}

type Kurento interface {
	//create: Instantiates a new media object, that is, a pipeline or media element.
	Create(ctx context.Context, obj *MediaObject) error
	// invoke: Calls a method of an existing media object.
	Invoke(ctx context.Context, obj *MediaObject, operation InvokeOperation, payload *json.RawMessage) error
	// subscribe: Creates a subscription to an event in a object.
	Subscribe(ctx context.Context, obj *MediaObject, topic SubscribeTopic) (<-chan []byte, error)
	// unsubscribe: Removes an existing subscription to an event.
	// release: Deletes the object and release resources used by it.

	//The Kurento Protocol allows to Kurento Media Server send requests to clients:
	//onEvent: This request is sent from Kurento Media server to clients when an event occurs.
	Close() error
}

type errorKurento struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

func (e *errorKurento) Error() string {
	return fmt.Sprintf("[%d] %s : %s", e.Code, e.Message, e.Data)
}

type response struct {
	Jsonrpc string `json:"jsonrpc"`
	// Result/Error
	ID string `json:"id"`

	// Result
	Result *json.RawMessage `json:"result,omitempty"`

	//Error
	Error *errorKurento `json:"error,omitempty"`

	// OnEvent
	Method string `json:"method"`

	Params *struct {
		Value struct {
			Data   *json.RawMessage `json:"data"`
			Object string           `json:"object"`
			Type   SubscribeTopic   `json:"type"`
		} `json:"value"`
	} `json:params,omitempty`
}

func (r *response) QueueName() string {
	if r.IsEvent() {
		return r.Params.Value.Object + `/` + string(r.Params.Value.Type)
	}

	return r.ID
}

func (a *response) IsEvent() bool {
	return a.Method == `onEvent`
}

func (a *response) Err() error {
	return a.Error
}

type kurentoClient struct {
	cctx      context.Context
	ws        *websocket.Conn
	sessionID string

	queueLock sync.RWMutex
	queue     map[string]chan *response

	cancel context.CancelFunc
}

func (k *kurentoClient) loop() error {
	for {
		select {
		case <-k.cctx.Done():
			return k.cctx.Err()
		default:
		}

		resp := &response{}
		err := k.ws.ReadJSON(resp)
		if err != nil {
			return err
		}

		k.queueLock.RLock()
		out, ok := k.queue[resp.QueueName()]
		k.queueLock.Unlock()
		if !ok {
			log.Print(`not found listener for response %v`, resp)
		}
		out <- resp
	}
	return nil
}

type request struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

func newRequest(method string, p interface{}) *request {
	return &request{
		Jsonrpc: `2.0`,
		ID:      uuid.New(),
		Method:  method,
		Params:  p,
	}
}

func (k *kurentoClient) send(req *request) (chan *response, error) {
	queueName := req.ID
	out := make(chan *response, 0)

	k.queueLock.Lock()
	k.queue[queueName] = out
	k.queueLock.Unlock()

	err := k.ws.WriteJSON(req)
	if err != nil {

		k.queueLock.Lock()
		delete(k.queue, queueName)
		k.queueLock.Unlock()

		return nil, err
	}
	return out, nil
}

func (k *kurentoClient) Create(ctx context.Context, obj *MediaObject) error {
	params := &struct {
		Type              MediaType          `json:"type"`
		SessionID         string             `json:"sessionId"`
		ConstructorParams *ConstructorParams `json:"constructorParams"`
	}{
		Type:      obj.Type,
		SessionID: k.sessionID,
	}

	if obj.Type != MediaPipeline {
		if &obj.Parent != nil {
			return errors.New(`not found media pipeline`)
		}
		params.ConstructorParams = &ConstructorParams{
			MediaPipeline: &obj.Parent.ID,
		}
	}

	out, err := k.send(newRequest(`create`, params))
	if err != nil {
		return err
	}
	select {
	case <-k.cctx.Done():
		return k.cctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-out:
		if resp.Error != nil {
			return resp.Err()
		}
		result := &struct {
			Value     string `json:"value"`
			SessionID string `json:"sessionId"`
		}{}
		err = json.Unmarshal(*resp.Result, result)
		if err != nil {
			return err
		}
		obj.ID = result.Value
		k.sessionID = result.SessionID
	}
	return nil
}

func (k *kurentoClient) Invoke(ctx context.Context, obj *MediaObject, operation InvokeOperation, buffer *json.RawMessage) error {
	params := &struct {
		Object          string           `json:"object"`
		Operation       string           `json:"operation"`
		OperationParams *json.RawMessage `json:"operationParams"`
		SessionID       string           `json:"sessionId"`
	}{
		Object:          obj.ID,
		Operation:       string(operation),
		OperationParams: buffer,
		SessionID:       k.sessionID,
	}

	out, err := k.send(newRequest(`invoke`, params))
	if err != nil {
		return err
	}
	select {
	case <-k.cctx.Done():
		return k.cctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-out:
		if resp.Error != nil {
			return resp.Err()
		}
		result := &struct {
			Value     *json.RawMessage `json:"value"`
			SessionID string           `json:"sessionId"`
		}{}
		err = json.Unmarshal(*resp.Result, result)
		if err != nil {
			return err
		}
		k.sessionID = result.SessionID
		if result.Value != nil {
			buffer = result.Value
		}
	}
	return nil
}

func (k *kurentoClient) Subscribe(ctx context.Context, obj *MediaObject, topic SubscribeTopic) (<-chan []byte, error) {
	params := &struct {
		Type      SubscribeTopic `json:"type"`
		Object    string         `json:"object"`
		SessionID string         `json:"sessionId"`
	}{
		Type:      topic,
		Object:    obj.ID,
		SessionID: k.sessionID,
	}

	out, err := k.send(newRequest(`subscribe`, params))
	if err != nil {
		return nil, err
	}

	result := &struct {
		Value     string `json:"value"`
		SessionID string `json:"sessionId"`
	}{}

	// got answer for subscribe
	select {
	case <-k.cctx.Done():
		return nil, k.cctx.Err()
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-out:
		if resp.Error != nil {
			return nil, resp.Err()
		}
		err = json.Unmarshal(*resp.Result, result)
		if err != nil {
			return nil, err
		}
		k.sessionID = result.SessionID
	}

	//registry subscribe path to route queue
	outBuffer := make(chan []byte, 0)
	topicUrl := obj.ID + `/` + string(topic)
	topicChannel := make(chan *response, 0)
	go func() {
		defer close(topicChannel)
		defer close(outBuffer)
		defer k.unsubscribe(ctx, obj, result.Value)

		for {
			select {
			case <-k.cctx.Done():
				return
			case <-ctx.Done():
				return
			case event, ok := <-topicChannel:
				if !ok {
					return
				}
				outBuffer <- *event.Params.Value.Data
			}
		}
	}()

	k.queueLock.Lock()
	k.queue[topicUrl] = topicChannel
	k.queueLock.Unlock()

	return outBuffer, nil
}

func (k *kurentoClient) unsubscribe(ctx context.Context, obj *MediaObject, subscriptionID string) error {
	params := &struct {
		Subscription string `json:"subscription"`
		Object       string `json:"object"`
		SessionID    string `json:"sessionId"`
	}{
		Subscription: subscriptionID,
		Object:       obj.ID,
		SessionID:    k.sessionID,
	}

	out, err := k.send(newRequest(`unsubscribe`, params))
	if err != nil {
		return err
	}

	select {
	case <-k.cctx.Done():
		return k.cctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-out:
		if resp.Error != nil {
			return resp.Err()
		}
	}

	return nil
}

func (k *kurentoClient) Close() error {
	k.cancel()
	return k.ws.Close()
}
