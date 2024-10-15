package Pic_Generating

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	URL                   = "wss://ws-api.runware.ai/v1"
	SEND_CHAN_MAX_SIZE    = 128
	RECEIVE_CHAN_MAX_SIZE = 128
	CONNECTION_TIMEOUT    = 120 * time.Second
	RECONNECTED_DELAY     = 1 * time.Second
	MAX_RETRIES           = 3
	WRITE_TIMEOUT         = 20 * time.Second
	READ_TIMEOUT          = 80 * time.Second
)

func CreateWsClient(apiKey string, userID uint) *WSClient {
	return &WSClient{
		User: struct {
			ID   uint
			UUID string
		}{ID: userID, UUID: ""},
		url:         URL,
		apiKey:      apiKey,
		socketMutex: sync.Mutex{},
		reconn:      atomic.Bool{},
	}
}

func (ws *WSClient) Start(numberResults int) error {
	if ws.socket == nil {
		if err := ws.connect(numberResults); err != nil {
			return err
		}
	} else {
		if err := ws.reconnecting(numberResults); err != nil {
			return err
		}
	}

	return nil
}

func (ws *WSClient) connect(numberResults int) error {
	socket, _, err := websocket.DefaultDialer.Dial(ws.url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	ws.socket = socket

	err = ws.authentication()
	if err != nil {
		return fmt.Errorf("Connect error %w", err)
	}

	ws.sendMsgChan = make(chan ReqMessage, SEND_CHAN_MAX_SIZE)
	ws.receiveMsgChan = make(chan RespMessage, RECEIVE_CHAN_MAX_SIZE)
	ws.errChan = make(chan error)
	ws.Done = make(chan struct{})

	ws.wg.Add(2)
	go ws.handleConnLoop(numberResults)
	go ws.handleErrLoop(numberResults)

	return nil
}

func (ws *WSClient) reconnecting(numberResults int) error {
	if !ws.reconn.CompareAndSwap(false, true) {
		return errors.New("reconnection already in progress")
	}

	go ws.Close()

	retryCount := 0
	backoffDuration := RECONNECTED_DELAY

	for {
		if retryCount >= MAX_RETRIES {
			return errors.New("max retries reached")
		}
		err := ws.connect(numberResults)
		if err == nil {
			log.Println("reconnect successful")
			return nil
		}
		time.Sleep(backoffDuration)
		retryCount++
		backoffDuration *= 2
		log.Printf("Reconnection attempt %d failed: %v. Next attempt in %v", retryCount, err, backoffDuration)
	}
}

func (ws *WSClient) Close() {
	ws.socketMutex.Lock()
	defer ws.socketMutex.Unlock()
	if ws.socket != nil {
		ws.socket.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second),
		)

		if err := ws.socket.Close(); err != nil {
			log.Printf("Error closing WebSocket connection: %v", err)
		}
	}

	ws.socket = nil

	close(ws.Done)

	ws.wg.Wait()
	safeClose(ws.sendMsgChan)
	safeClose(ws.receiveMsgChan)
	safeClose(ws.errChan)
}

func safeClose[T any](ch chan T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic while closing channel: %v", r)
		}
	}()

	select {
	case _, ok := <-ch:
		if ok {
			close(ch)
		}
	default:
		close(ch)
	}

}

func GenerateUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
