// Обзёрвер WS-соединения.
// Абстракция поверх реконнектора, инкапсулирует логику по работе с WS-соединением (чтение, запись, heartbeat),
// делегирует реконнект - реконнектору. Позволяет внешнему коду работать с WS-соединением на более высоком уровне
// не заботясь о реконнекте и поддержке соединения в активном состоянии.
package websocket

import (
	"errors"
	"io/ioutil"
	"sync"
	"time"

	"context"
	"github.com/gorilla/websocket"
	"log"
)

const (
	CLOSE_STMT = "server is going to shutdown"

	// Time allowed to write a message to the peer.
	WRITE_TIMEOUT = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	PONG_TIMEOUT = 2 * time.Second

	// Send pings to peer with this period. Should be less then PONG_TIMEOUT.
	PING_RATE = time.Second

	// Maximum message size allowed from peer. For future use.
	// MESSAGE_MAX_SIZE = 512
)

var (
	ErrNilContext    = errors.New(`nil context is a bad habbit`)
	ErrNilConnection = errors.New(`nil connection seriously?`)
)

// Конструктор обзёрвера пользовательских WS-соединений.
// Если переданный коннект не активный, перед конструированием
// обзёрвера производит попытку подключения и, вне зависимости
// от результата последней, возвращает новый инстанс обзёрвера
// предварительно запустив две рутины одна для записи, другая для чтения.
func NewWsListener(ctx context.Context, conn reconnecter) (*WsListener, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	if conn == nil {
		return nil, ErrNilConnection
	}
	wsListener := &WsListener{
		conn:    conn,
		wg:      new(sync.WaitGroup),
		done:    make(chan struct{}),
		readCh:  make(chan []byte, 10),
		writeCh: make(chan []byte, 10),
		ticker:  time.NewTicker(PING_RATE),
	}

	conn.SetPongHandler(wsListener.pongHandler)

	// try to reconnect if connection is inactive
	wsListener.Reconnect()

	wsListener.wg.Add(2)
	go wsListener.startReader()
	go wsListener.startWriter()

	return wsListener, nil
}

// Обзёрвер пользовательских WS-соединений.
type WsListener struct {
	conn    reconnecter
	wg      *sync.WaitGroup
	done    chan struct{}
	readCh  chan []byte
	writeCh chan []byte
	ticker  *time.Ticker
}

const (
	// размер буфера WS на чтение
	READ_BUF = 1024
	// размер буфера WS на запись
	WRITE_BUF = 1024
	// таймаут реконнекта WS соединения
	RECONNECT_TIMEOUT = 250 * time.Millisecond
)

// Читатель из WS соединения.
// В случае ошибки соединения логирует ее
// ждет таймаут == времени реконнекта (чтобы не спамить логи при реконнекте)
// заверашет работу только при закрытии канала done.
func (l *WsListener) startReader() {
	defer func() {
		l.wg.Done()
	}()

	for {
		select {
		case <-l.done:
			return
		default:
			msgType, data, err := l.conn.ReadMessage()
			if err != nil {
				time.Sleep(RECONNECT_TIMEOUT / 10)

			}

			if msgType == websocket.TextMessage {
				l.readCh <- data
			}
		}
	}
}

// Писатель в WS соединение.
// В случае ошибки соединения логирует ее
// ждет таймаут == времени реконнекта (чтобы не спамить логи при реконнекте),
// шлет пинги в открытое соединение,
// заверашет работу только при закрытии канала done.
func (l *WsListener) startWriter() {
	defer func() {
		l.ticker.Stop()

		l.wg.Done()
	}()

	for {
		select {
		case <-l.done:
			return
		case msg := <-l.writeCh:
			err := l.conn.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT))
			if err != nil {

				time.Sleep(RECONNECT_TIMEOUT / 10)
				continue
			}

			err = l.conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {

				time.Sleep(RECONNECT_TIMEOUT / 10)
			}
		case <-l.ticker.C:

			if err := l.conn.WriteControl(
				websocket.PingMessage,
				[]byte{},
				time.Now().Add(time.Second),
			); err != nil {

				time.Sleep(RECONNECT_TIMEOUT / 10)
			}
		}
	}
}

// Возвращает канал в который пишет изменения статуса WS-соединения.
// false - обрыв соединения
// true - соединение восстановлено
func (l *WsListener) Status() <-chan bool {
	return l.conn.Status()
}

// Возвращает канал с сообщениями из WS-соединения,
// сообщения отфильтрованы по типу websocket.TextMessage.
func (l *WsListener) Read() <-chan []byte {
	return l.readCh
}

// Отправляет байты в канал на отправку по WS.
func (l *WsListener) Write(msg []byte) {
	l.writeCh <- msg
}

// Разовый реконнект WS-соединения.
// Логирует все, что пошло не так.
func (l *WsListener) Reconnect() {
	if !l.conn.IsActive() {
		resp, err := l.conn.Reconnect()
		if err != nil {
			log.Printf(`Reconnect: %s`, err)
		}
		if resp != nil {
			_, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				log.Printf(`Reconnect: %s`, err)
			}

		}
	}
}

// Завершение работы обзервера. Останавливает опорные рутины и закрывает соотв. WS коннект.
func (l *WsListener) Close() error {
	// отправляем сообщение о закрытии WS-соединения другой стороне
	l.conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

	if err := l.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, CLOSE_STMT),
		time.Now().Add(time.Second),
	); err != nil {
		return err
	}

	close(l.done)  // закрываем сигнальный канал
	l.conn.Close() // выходя, закрываем WS коннект

	l.wg.Wait() // дожидаемся завершения рутин чтения и записи

	close(l.writeCh) // закрываем канал на запись
	close(l.readCh)  // закрываем канал на чтение

	return nil
}

func (l *WsListener) pongHandler(string) error {
	return l.conn.SetReadDeadline(time.Now().Add(PONG_TIMEOUT))
}
