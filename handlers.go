package Pic_Generating

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

func (ws *WSClient) handleConnLoop() {
	defer fmt.Println("handleConnLoop ended")
	fmt.Println("handleConnLoop started")
	for {
		if ws.reconn.Load() {
			fmt.Println("Continue in handleLoop")
			time.Sleep(time.Second)
			continue
		}
		select {
		case <-ws.Done:
			fmt.Println("Handle conn Channel done")
			return
		case req := <-ws.SendMsgChan:
			if err := ws.socket.SetWriteDeadline(time.Now().Add(WRITE_TIMEOUT)); err != nil {
				ws.ErrChan <- fmt.Errorf("set write deadline: %w", err)
				continue
			}

			if err := ws.send(req); err != nil {
				ws.ErrChan <- fmt.Errorf("send error: %w", err)
				return
			}

			if err := ws.socket.SetWriteDeadline(time.Time{}); err != nil {
				ws.ErrChan <- fmt.Errorf("clear write deadline: %w", err)
				continue
			}

			if err := ws.socket.SetReadDeadline(time.Now().Add(READ_TIMEOUT)); err != nil {
				ws.ErrChan <- fmt.Errorf("set read deadline: %w", err)
				continue
			}

			resp, err := ws.receive()
			if err != nil {
				ws.ErrChan <- fmt.Errorf("receive error: %w", err)
				continue
			}

			if err := ws.socket.SetReadDeadline(time.Time{}); err != nil {
				ws.ErrChan <- fmt.Errorf("clear read deadline: %w", err)
				continue
			}
			ws.ReceiveMsgChan <- resp
		}

	}
}

func (ws *WSClient) handleErrLoop() {
	defer fmt.Println("handleErrLoop ended")
	fmt.Println("handleErrLoop started")
	for {
		if ws.reconn.Load() {
			continue
		}
		select {
		case err := <-ws.ErrChan:
			fmt.Println(err)
			if err.Error() == "websocket: close 1006 (abnormal closure): unexpected EOF" {
				fmt.Println("Мы вошли в ошибку")
				ws.reconnecting()
				return
			}
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				ws.Close()
				return
			}
			ws.Close()
			return
		}

	}
}

func (ws *WSClient) send(msg ReqMessage) error {
	ws.socketMutex.Lock()
	defer ws.socketMutex.Unlock()
	if ws.socket == nil {
		return errors.New("socket is nil")
	}

	reqData := []ReqMessage{msg}

	if err := ws.socket.WriteJSON(reqData); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (ws *WSClient) receive() ([]RespData, error) {
	ws.socketMutex.Lock()
	defer ws.socketMutex.Unlock()

	if ws.socket == nil {
		return nil, errors.New("socket is nil")
	}

	_, resp, err := ws.socket.ReadMessage()
	if err != nil {
		return nil, err
	}
	response := new(RespMessage)
	if err := json.Unmarshal(resp, response); err != nil {
		return nil, err
	}
	fmt.Println("response: ", response)
	if len(response.Err) > 0 {
		return nil, fmt.Errorf("error response: %s", response.Err[0].Message)
	}
	return response.Data, nil
}

func (ws *WSClient) authentication() error {
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
	return nil
}

func (ws *WSClient) SendAndReceiveMsg(msg ReqMessage) ([]RespData, error) {
	if ws.socket == nil || ws.SendMsgChan == nil {
		if err := ws.Start(); err != nil {
			return nil, fmt.Errorf("failed to start connection: %w", err)
		}
	}

	ws.SendMsgChan <- msg

	select {
	case resp := <-ws.ReceiveMsgChan:
		return resp, nil
	case <-time.After(READ_TIMEOUT):
		return nil, errors.New("timeout waiting for response")
	}
}
