package Pic_Generating

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

func (ws *WSClient) handleConnLoop() {
	defer ws.wg.Done()

	timer := time.NewTimer(CONNECTION_TIMEOUT)
	defer timer.Stop()
	for {
		if ws.reconn.Load() {
			time.Sleep(time.Second)
			continue
		}
		select {
		case <-ws.Done:
			return
		case req := <-ws.SendMsgChan:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(CONNECTION_TIMEOUT)

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
			ws.ReceiveMsgChan <- *resp
		case <-timer.C:
			ws.ErrChan <- errors.New("Connection timeout")
			return
		}
	}
}

func (ws *WSClient) handleErrLoop() {
	defer ws.wg.Done()
	for {
		if ws.reconn.Load() {
			continue
		}
		select {
		case err := <-ws.ErrChan:
			log.Printf("Error: %v", err)
			switch {
			case websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure):
				if reconnErr := ws.reconnecting(); reconnErr != nil {
					errResp := errResp
					errResp.Err[0].Message = err.Error()
					ws.ReceiveMsgChan <- errResp
					go ws.Close()
					return
				}

			case err.Error() == "Connection timeout":
				errResp := errResp
				errResp.Err[0].Message = err.Error()
				ws.ReceiveMsgChan <- errResp
				go ws.Close()
				return

			case errors.Is(err, net.ErrClosed):
				return
			default:
				if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
					if reconnErr := ws.reconnecting(); reconnErr != nil {
						errResp := errResp
						errResp.Err[0].Message = err.Error()
						ws.ReceiveMsgChan <- errResp
						go ws.Close()
						return
					}
				} else {
					errResp := errResp
					errResp.Err[0].Message = err.Error()
					ws.ReceiveMsgChan <- errResp
					go ws.Close()
					return
				}
			}
		case <-ws.Done:
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
		return err
	}
	return nil
}

func (ws *WSClient) receive() (*RespMessage, error) {
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
	if len(response.Err) > 0 {
		return nil, fmt.Errorf("error response: %s", response.Err[0].Message)
	}
	return response, nil
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
	log.Printf("UserID: %d authenticated with UUID:%s", ws.User.ID, ws.User.UUID)
	return nil
}

func (ws *WSClient) SendAndReceiveMsg(msg ReqMessage) (RespMessage, error) {
	if ws.socket == nil || ws.SendMsgChan == nil {
		if err := ws.Start(); err != nil {
			go ws.Close()
			return emptyResp, fmt.Errorf("failed to start connection: %w", err)
		}
	}

	ws.SendMsgChan <- msg

	timeout := time.NewTimer(READ_TIMEOUT + WRITE_TIMEOUT)
	defer timeout.Stop()

	select {
	case resp := <-ws.ReceiveMsgChan:
		return resp, nil
	case <-timeout.C:
		return emptyResp, errors.New("timeout waiting for response")
	}
}
