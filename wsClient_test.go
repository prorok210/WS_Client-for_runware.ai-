package Pic_Generating

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestWSClientSendMsg(t *testing.T) {
	err := godotenv.Load("./.env")
	if err != nil {
		t.Error(err)
	}

	wsClient := CreateWsClient(os.Getenv("API_KEY2"), 111)

	var data []ReqMessage

	for i := 0; i < 50; i++ {
		promt := fmt.Sprintf("A beautiful landscape %d", i)
		reqMsg := ReqMessage{
			PositivePrompt: promt,
			Model:          "runware:100@1@1",
			Steps:          12,
			Width:          512,
			Height:         512,
			NumberResults:  1,
			OutputType:     []string{"URL"},
			TaskType:       "imageInference",
			TaskUUID:       GenerateUUID(),
		}
		data = append(data, reqMsg)
	}
	go func() {
		fmt.Println("Start testing")
		for i, reqMsg := range data {
			resp, err := wsClient.SendAndReceiveMsg(reqMsg)
			if err != nil {
				fmt.Println("err connectin", err)
			}
			fmt.Println(i, "Sent: ", reqMsg)
			fmt.Println(i, "Response: ", resp)
			if i == 10 || i == 20 || i == 30 || i == 40 {
				go func() {
					for j := 0; j < i/10*60; j++ {
						fmt.Println(j)
						time.Sleep(1 * time.Second)
					}
				}()
				sleepDuration := time.Duration((i / 10) * 60)
				time.Sleep(sleepDuration * time.Second)
			}
		}
	}()
	select {}
}
