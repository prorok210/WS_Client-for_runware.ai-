package Pic_Generating

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
)

func (ws *WSClient) handleConnLoop() {
	for {
		select {
		case <-ws.Done:
			return
		case req := <-ws.SendMsgChan:
			if err := ws.send(req); err != nil {
				ws.ErrChan <- err
			}
			resp, err := ws.receive()
			if err != nil {
				ws.ErrChan <- err
			}
			ws.ReceiveMsgChan <- resp
		case err := <-ws.ErrChan:
			ws.handleErr(err)
		}
	}
}

func (ws *WSClient) send(msg ReqMessage) error {
	if ws.socket == nil {
		return errors.New("socket is nil")
	}
	reqData := []ReqMessage{msg}
	if err := ws.socket.WriteJSON(reqData); err != nil {
		return err
	}
	return nil
}

func (ws *WSClient) receive() ([]RespData, error) {
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
	return response.Data, nil
}

func (ws *WSClient) handleErr(err error) {
	fmt.Println(err)
}

func (ws *WSClient) SendMsg(msg ReqMessage) {
	if ws.socket == nil {
		err := ws.Start()
		if err != nil {
			log.Println(err)
			return
		}
	}
	ws.SendMsgChan <- msg
}

func (ws *WSClient) ReceiveMsg() []RespData {
	return <-ws.ReceiveMsgChan
}
