package Pic_Generating

import (
	"encoding/json"
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
	TIMEOUT               = 15 * time.Second
	RECONNECTED_DELAY     = 1 * time.Second
	RECONNECT_TIMEOUT     = 10 * time.Second
	RECONNECT_MAX_TRIES   = 3
	MAX_RETRIES           = 3
	WRITE_TIMEOUT         = 10 * time.Second
	READ_TIMEOUT          = 30 * time.Second
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
	if ws.socket == nil {
		ws.SendMsgChan = make(chan ReqMessage, SEND_CHAN_MAX_SIZE)
		ws.ReceiveMsgChan = make(chan []RespData, RECEIVE_CHAN_MAX_SIZE)
		ws.ErrChan = make(chan error)
		ws.Done = make(chan struct{})
		if err := ws.connect(); err != nil {
			return err
		}
	} else {
		if err := ws.reconnecting(); err != nil {
			return err
		}
	}
	go ws.handleConnLoop()
	return nil
}

func (ws *WSClient) connect() error {
	if ws.socket != nil {
		return errors.New("socket already exists")
	}

	socket, _, err := websocket.DefaultDialer.Dial(ws.url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	ws.socket = socket

	authCredentials := ReqMessage{
		TaskType: "authentication",
		ApiKey:   ws.apiKey,
	}
	formattedReq := []ReqMessage{authCredentials}

	jsonStr, err := json.Marshal(formattedReq)
	if err != nil {
		return fmt.Errorf("failed to marshal auth request: %w", err)
	}
	fmt.Println(string(jsonStr))

	err = ws.socket.WriteMessage(websocket.TextMessage, jsonStr)
	if err != nil {
		return fmt.Errorf("failed to send auth request: %w", err)
	}
	_, resp, err := ws.socket.ReadMessage()
	if err != nil {
		return fmt.Errorf("failed to read auth response: %w", err)
	}
	response := new(RespMessage)
	if err := json.Unmarshal(resp, response); err != nil {
		return fmt.Errorf("failed to unmarshal auth response: %w", err)
	}
	if len(response.Err) > 0 {
		return fmt.Errorf("error response: %s", response.Err[0].Message)
	}
	if len(response.Data) == 0 {
		return errors.New("empty data in auth response")
	}
	ws.User.UUID = response.Data[0].ConnectionSessionUUID

	log.Println("Auth response:", string(resp))

	return nil
}

func (ws *WSClient) reconnecting() error {
	if !ws.reconn.CompareAndSwap(false, true) {
		return errors.New("reconnection already in progress")
	}
	err := ws.Close()
	if err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}
	defer ws.reconn.Store(false)
	defer log.Println("Reconnection finished")

	retryCount := 0
	backoffDuration := RECONNECTED_DELAY

	for {
		select {
		case <-time.After(RECONNECT_TIMEOUT):
			return errors.New("reconnection timeout")
		default:
			if retryCount >= MAX_RETRIES {
				return errors.New("max retries reached")
			}
			err := ws.connect()
			if err == nil {
				return nil
			}

			time.Sleep(backoffDuration)
			retryCount++
			backoffDuration *= 2
			log.Printf("Reconnection attempt %d failed: %v. Next attempt in %v", retryCount, err, backoffDuration)

		}

	}
}

func (ws *WSClient) Close() error {
	var closeErr error = nil
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
				closeErr = err
			}
		}

		if ws.Done != nil {
			close(ws.Done)
		}

		safeClose(ws.SendMsgChan)
		safeClose(ws.ReceiveMsgChan)
		safeClose(ws.ErrChan)

	})
	return closeErr
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
