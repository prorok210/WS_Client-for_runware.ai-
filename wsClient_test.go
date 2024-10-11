package Pic_Generating

import (
	"fmt"
	"testing"
)

func TestWSClientSendMsg(t *testing.T) {
	wsClient := CreateWsClient("z1ilk4CqKMMMPSm3gynSdrsuoKsECcxK", 111)
	err := wsClient.Start()
	if err != nil {
		t.Error(err)
	}

	reqMsg := ReqMessage{
		PositivePrompt: "A beautiful landscape",
		Model:          "runware:100@1@1",
		Steps:          12,
		Width:          512,
		Height:         512,
		NumberResults:  1,
		OutputType:     []string{"URL"},
		TaskType:       "imageInference",
		TaskUUID:       GenerateUUID(),
	}

	wsClient.SendMsgChan <- reqMsg
	resp := <-wsClient.ReceiveMsgChan
	fmt.Println("Response: ", resp)

}
