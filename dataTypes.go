package Pic_Generating

import (
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type ReqMessage struct {
	TaskType       string   `json:"taskType,omitempty"`
	TaskUUID       string   `json:"taskUUID,omitempty"`
	OutputType     []string `json:"outputType,omitempty"`
	OutputFormat   string   `json:"outputFormat,omitempty"`
	PositivePrompt string   `json:"positivePrompt,omitempty"`
	NegativePrompt string   `json:"negativePrompt,omitempty"`
	Height         int      `json:"height,omitempty"`
	Width          int      `json:"width,omitempty"`
	Model          string   `json:"model,omitempty"`
	Steps          int      `json:"steps,omitempty"`
	CFGScale       float64  `json:"CFGScale,omitempty"`
	NumberResults  int      `json:"numberResults,omitempty"`
	Scheduler      string   `json:"scheduler,omitempty"`
	Seed           int      `json:"seed,omitempty"`
	ApiKey         string   `json:"apiKey,omitempty"`
}

type RespData struct {
	TaskType              string `json:"taskType"`
	TaskUUID              string `json:"taskUUID"`
	ImageUUID             string `json:"imageUUID"`
	NSFWContent           bool   `json:"NSFWContent"`
	ConnectionSessionUUID string `json:"connectionSessionUUID"`
	ImageURL              string `json:"imageURL"`
}

type RespError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Parameter string `json:"parameter"`
	Type      string `json:"type"`
	TaskUUID  string `json:"taskUUID"`
}

type RespMessage struct {
	Data []RespData  `json:"data"`
	Err  []RespError `json:"errors"`
}

type WSClient struct {
	User struct {
		ID   uint
		UUID string
	}
	url            string
	apiKey         string
	socket         *websocket.Conn
	sendMsgChan    chan ReqMessage
	receiveMsgChan chan RespMessage
	errChan        chan error
	Done           chan struct{}
	CloseChan      chan struct{}
	wg             sync.WaitGroup
	socketMutex    sync.Mutex
	reconn         atomic.Bool
	dataInChannel  atomic.Bool
}

var (
	errResp   = RespMessage{Data: []RespData{}, Err: []RespError{{Code: "500", Message: "Internal server error"}}}
	emptyResp = []RespMessage{
		{
			Data: []RespData{},  // Пустой слайс
			Err:  []RespError{}, // Пустой слайс
		},
	}
)
