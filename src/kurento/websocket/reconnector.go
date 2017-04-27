// Реконнектор.
// Тонкая обертка над WS-соединением, инкапсулирует логику реконнекта).
// Данный уровень абстракции позволяет верхнему слою, работать с соединением вне зависимости от
// состояния этого соединения.
package websocket

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

var ErrConnClosed = errors.New(`Underlying ws-connection is closed`)
var ErrEmptyDialer = errors.New(`Underlying dialer is empty, reconnect is impossible`)

// Конструктор реконнектора.
//
// Оба параметра необязательны.
//  * conn - указатель на экземпляр WS-коннекта,
//  * dial - функция dialer, возвращающая новый WS-коннект.
//
// Если оба пустые тогда реконнектор будет
// по всем методам возаращать ErrConnClosed. А метод Reconnect() не будет выполнять реконнекта с
// ошибкой ErrEmptyDialer.
func NewReconn(conn WebSocketer, dial func() (WebSocketer, *http.Response, error)) *reconn {
	rconn := &reconn{dial: dial, conn: safeConn{statusCh: make(chan bool, 10), m: new(sync.RWMutex)}, pongHandler: pongHandler{m: new(sync.RWMutex)}}
	if conn != nil {
		rconn.conn.Set(conn)
	}
	return rconn
}

// Интерфейс реконнектора.
type reconnecter interface {
	WebSocketer
	// extended methods
	//
	// unary reconnection.
	Reconnect() (*http.Response, error)
	// inspects status of the active conn.
	IsActive() bool
	// realtime status change notifications.
	Status() <-chan bool
}

// Интерфейс WS-соединения.
type WebSocketer interface {
	// used *websocket.Conn methods
	//
	// Закрывает активное WS-соединение, проксирует ошибку в ответе.
	Close() error
	// Читает сообщение из активного WS-соединения.
	ReadMessage() (messageType int, p []byte, err error)
	// Регистрирует обработчик WS-понгов.
	SetPongHandler(h func(appData string) error)
	// Устанавливает дедлайн на запись для активного WS-соединения, проксирует ошибку в ответе.
	SetWriteDeadline(t time.Time) error
	// Устанавилвает дедлайн начтение для активного WS-соединения, проксирует ошибку в ответе.
	SetReadDeadline(t time.Time) error
	// Отправляет контрольное сообщение (исп. для пингов) по WS-соединению, проксирует ошибку в ответе.
	WriteControl(messageType int, data []byte, deadline time.Time) error
	// Отправляет сообщение по WS-соединению, проксирует ошибку в ответе.
	WriteMessage(messageType int, data []byte) error
}

// Тонкая оберка расширяющая websocket.Conn методом Reconnect() error.
// Реализует reconnecter интерфейс.
type reconn struct {
	conn        safeConn
	dial        func() (WebSocketer, *http.Response, error)
	pongHandler pongHandler
}

// Возвращает статус WS-соединения.
func (r *reconn) IsActive() bool {
	return r.conn.IsActive()
}

// Возвращает канал в который пишет изменения статуса WS-соединения.
// false - обрыв соединения
// true - соединение восстановлено
func (r *reconn) Status() <-chan bool {
	return r.conn.Status()
}

// Единичная попытка реконнекта.
// Если попытка успешная, то поле conn перезаписывается новым
// значением и метод возвращает nil, nil.
// Если попытка неуспешная, то поле conn перезаписывается nil-ом
// и метод возвращает не нулевую ошибку и ответ.
func (r *reconn) Reconnect() (resp *http.Response, err error) {
	if r.dial != nil {
		var conn WebSocketer
		conn, resp, err = r.dial()
		if err == nil && conn != nil {
			r.conn.Set(conn)
			if pongHandler := r.pongHandler.Get(); pongHandler != nil {
				conn.SetPongHandler(pongHandler)
			}
		} else {
			r.conn.Close()
		}
		return resp, err
	} else {
		return nil, ErrEmptyDialer
	}

	return resp, err
}

// Закрывает активное WS-соединение, проксирует ошибку в ответе.
func (r *reconn) Close() (err error) {
	return r.conn.Close()
}

// Читает сообщение из активного WS-соединения.
func (r *reconn) ReadMessage() (messageType int, p []byte, err error) {
	return r.conn.ReadMessage()
}

// Регистрирует обработчик WS-понгов.
func (r *reconn) SetPongHandler(h func(appData string) error) {
	r.pongHandler.Set(h)
	r.conn.SetPongHandler(h)
}

// Устанавливает дедлайн на запись для активного WS-соединения, проксирует ошибку в ответе.
func (r *reconn) SetWriteDeadline(t time.Time) error {
	return r.conn.SetWriteDeadline(t)
}

// Устанавливает дедлайн на чтение для активного WS-соединения, проксирует ошибку в ответе.
func (r *reconn) SetReadDeadline(t time.Time) error {
	return r.conn.SetReadDeadline(t)
}

// Отправляет контрольное сообщение (исп. для пингов) по WS-соединению, проксирует ошибку в ответе.
func (r *reconn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return r.conn.WriteControl(messageType, data, deadline)
}

// Отправляет сообщение по WS-соединению, проксирует ошибку в ответе.
func (r *reconn) WriteMessage(messageType int, data []byte) error {
	return r.conn.WriteMessage(messageType, data)
}

type pongHandler struct {
	m       *sync.RWMutex
	handler func(string) error
}

func (p *pongHandler) Get() func(string) error {
	p.m.RLock()
	defer p.m.RUnlock()
	return p.handler
}

func (p *pongHandler) Set(handler func(string) error) {
	p.m.Lock()
	p.handler = handler
	p.m.Unlock()
}

type safeConn struct {
	webSocket WebSocketer
	m         *sync.RWMutex
	statusCh  chan bool
}

func (sc *safeConn) IsActive() bool {
	sc.m.RLock()
	defer sc.m.RUnlock()
	return sc.webSocket != nil
}

func (sc *safeConn) Status() <-chan bool {
	if sc.statusCh == nil {
		ch := make(chan bool)
		close(ch)
		return ch
	}
	return sc.statusCh
}

func (sc *safeConn) Close() (err error) {
	sc.m.Lock()

	if sc.webSocket != nil {
		err = sc.webSocket.Close()
		sc.webSocket = nil
	} else {
		err = ErrConnClosed
	}
	sc.m.Unlock()

	if sc.statusCh != nil {
		sc.statusCh <- false
	}

	return err
}

func (sc *safeConn) ReadMessage() (messageType int, p []byte, err error) {
	sc.m.RLock()

	if sc.webSocket == nil {
		sc.m.RUnlock()
		return messageType, p, ErrConnClosed
	}

	messageType, p, err = sc.webSocket.ReadMessage()
	sc.m.RUnlock()
	if err != nil {
		sc.Close()
	}
	return messageType, p, err
}

func (sc *safeConn) SetPongHandler(h func(appData string) error) {
	sc.m.RLock()
	defer sc.m.RUnlock()

	if sc.webSocket == nil {
		return
	}

	sc.webSocket.SetPongHandler(h)
}

func (sc *safeConn) SetWriteDeadline(t time.Time) error {
	sc.m.RLock()

	if sc.webSocket == nil {
		sc.m.RUnlock()
		return ErrConnClosed
	}

	err := sc.webSocket.SetWriteDeadline(t)
	sc.m.RUnlock()

	if err != nil {
		sc.Close()
	}
	return err
}

func (sc *safeConn) SetReadDeadline(t time.Time) error {
	sc.m.RLock()

	if sc.webSocket == nil {
		sc.m.RUnlock()
		return ErrConnClosed
	}

	err := sc.webSocket.SetReadDeadline(t)
	sc.m.RUnlock()

	if err != nil {
		sc.Close()
	}

	return err
}

func (sc *safeConn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	sc.m.RLock()

	if sc.webSocket == nil {
		sc.m.RUnlock()
		return ErrConnClosed
	}

	err := sc.webSocket.WriteControl(messageType, data, deadline)
	sc.m.RUnlock()
	if err != nil {
		sc.Close()
	}

	return err
}

func (sc *safeConn) WriteMessage(messageType int, data []byte) error {
	sc.m.RLock()
	if sc.webSocket == nil {
		sc.m.RUnlock()
		return ErrConnClosed
	}

	err := sc.webSocket.WriteMessage(messageType, data)
	sc.m.RUnlock()
	if err != nil {
		sc.Close()
	}

	return err
}

func (sc *safeConn) Set(conn WebSocketer) {
	sc.m.Lock()
	defer sc.m.Unlock()
	sc.webSocket = conn

	if sc.statusCh != nil {
		sc.statusCh <- (conn != nil)
	}
}
