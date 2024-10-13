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
	CONNECTION_TIMEOUT    = 60 * time.Second
	RECONNECTED_DELAY     = 1 * time.Second
	MAX_RETRIES           = 3
	WRITE_TIMEOUT         = 5 * time.Second
	READ_TIMEOUT          = 15 * time.Second
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
		closeOnce:   sync.Once{},
		closed:      atomic.Bool{},
	}
}

func (ws *WSClient) Start() error {
	fmt.Println("Starting")
	if ws.socket == nil {
		if err := ws.connect(); err != nil {
			return err
		}
	} else {
		if err := ws.reconnecting(); err != nil {
			return err
		}
	}

	return nil
}

func (ws *WSClient) connect() error {
	socket, _, err := websocket.DefaultDialer.Dial(ws.url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	ws.socket = socket

	err = ws.authentication()

	log.Println("Auth:", string(ws.User.UUID))

	ws.SendMsgChan = make(chan ReqMessage, SEND_CHAN_MAX_SIZE)
	ws.ReceiveMsgChan = make(chan []RespData, RECEIVE_CHAN_MAX_SIZE)
	ws.ErrChan = make(chan error)
	ws.Done = make(chan struct{})

	ws.wg.Add(2)
	go ws.handleConnLoop()
	go ws.handleErrLoop()

	return nil
}

func (ws *WSClient) reconnecting() error {
	fmt.Println("reconnecting...")
	defer fmt.Println("reconnecting done")
	if !ws.reconn.CompareAndSwap(false, true) {
		return errors.New("reconnection already in progress")
	}
	defer ws.reconn.Store(false)

	go ws.Close()

	retryCount := 0
	backoffDuration := RECONNECTED_DELAY

	for {
		if retryCount >= MAX_RETRIES {
			return errors.New("max retries reached")
		}
		err := ws.connect()
		if err == nil {
			fmt.Println("recon successful")
			return nil
		}
		fmt.Println("Ошибка в reconnecting", err)
		time.Sleep(backoffDuration)
		retryCount++
		backoffDuration *= 2
		log.Printf("Reconnection attempt %d failed: %v. Next attempt in %v", retryCount, err, backoffDuration)
	}
}

func (ws *WSClient) Close() {
	fmt.Println("Close")
	defer fmt.Println("close done")
	ws.closeOnce.Do(func() {
		ws.closed.Store(true)

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
			ws.socket = nil
		}

		if ws.Done != nil {
			close(ws.Done)
		}
		ws.wg.Wait()
		safeClose(ws.SendMsgChan)
		safeClose(ws.ReceiveMsgChan)
		safeClose(ws.ErrChan)
	})
}

func safeClose[T any](ch chan T) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic while closing channel: %v", r)
		}
		fmt.Println("Chanel closed", ch)
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
