package intrinio

import (
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var selfHealBackoffs [5]int = [5]int{10, 30, 60, 300, 600}

const (
	HEARTBEAT_INTERVAL       int = 20
	MAX_OPTIONS_QUEUE_DEPTH  int = 20000
	MAX_EQUITIES_QUEUE_DEPTH int = 10000
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func doBackoff(fn func() bool, isStopped *bool) {
	i := 0
	backoff := selfHealBackoffs[i]
	success := fn()
	for !success && !*isStopped {
		time.Sleep(time.Duration(backoff) * time.Second)
		if !*isStopped {
			i = min(i+1, len(selfHealBackoffs)-1)
			backoff = selfHealBackoffs[i]
			success = fn()
		}
	}
}

type Client struct {
	token           string
	tokenUpdateTime time.Time
	dataMsgCount    uint64
	txtMsgCount     uint32
	workerCount     int
	subscriptions   map[string]bool
	isStopped       bool
	isClosed        bool
	closeWg         sync.WaitGroup
	reconnected     chan bool
	readChannel     chan []byte
	writeChannel    chan []byte
	httpClient      *http.Client
	wsConn          *websocket.Conn
	heartbeat       *time.Ticker
	config          Config
	work            func()
	composeJoinMsg  func(string) []byte
	composeLeaveMsg func(string) []byte
}

func NewOptionsClient(
	c Config,
	onTrade func(OptionTrade),
	onQuote func(OptionQuote),
	onRefresh func(OptionRefresh),
	onUnusualActivity func(OptionUnusualActivity)) *Client {
	client := &Client{
		isStopped:     true,
		isClosed:      true,
		workerCount:   1,
		reconnected:   make(chan bool),
		readChannel:   make(chan []byte, MAX_OPTIONS_QUEUE_DEPTH),
		writeChannel:  make(chan []byte, 1000),
		subscriptions: make(map[string]bool),
		httpClient:    http.DefaultClient,
		config:        c,
	}
	if onTrade != nil {
		client.workerCount++
	}
	if onQuote != nil {
		client.workerCount += 8
	}
	client.work = func() {
		for {
			if len(client.readChannel) == 0 {
				if client.isClosed && client.isStopped {
					defer client.closeWg.Done()
					return
				} else {
					time.Sleep(time.Second)
				}
			}
			workOnOptions(
				client.readChannel,
				onTrade,
				onQuote,
				onRefresh,
				onUnusualActivity)
		}
	}
	client.composeJoinMsg = func(symbol string) []byte {
		return composeOptionJoinMsg(
			onTrade != nil,
			onQuote != nil,
			onRefresh != nil,
			onUnusualActivity != nil,
			symbol)
	}
	client.composeLeaveMsg = composeOptionLeaveMsg
	return client
}

func NewEquitiesClient(
	c Config,
	onTrade func(EquityTrade),
	onQuote func(EquityQuote)) *Client {
	client := &Client{
		isStopped:     true,
		isClosed:      true,
		workerCount:   2,
		reconnected:   make(chan bool),
		readChannel:   make(chan []byte, MAX_EQUITIES_QUEUE_DEPTH),
		writeChannel:  make(chan []byte, 1000),
		subscriptions: make(map[string]bool),
		httpClient:    http.DefaultClient,
		config:        c,
	}
	if onQuote != nil {
		client.workerCount += 2
	}
	client.work = func() {
		for {
			if len(client.readChannel) == 0 {
				if client.isClosed && client.isStopped {
					defer client.closeWg.Done()
					return
				} else {
					time.Sleep(time.Second)
				}
			}
			workOnEquities(
				client.readChannel,
				onTrade,
				onQuote)
		}
	}
	client.composeJoinMsg = func(symbol string) []byte {
		return composeEquityJoinMsg(
			onTrade != nil,
			onQuote != nil,
			symbol)
	}
	client.composeLeaveMsg = composeEquityLeaveMsg
	return client
}

func (client *Client) trySetToken() bool {
	log.Print("Client - Authorizing...")
	authUrl := client.config.getAuthUrl()
	req, httpNewReqErr := http.NewRequest("GET", authUrl, nil)
	if httpNewReqErr != nil {
		log.Printf("Client - Authorization Failure: %v\n", httpNewReqErr)
		return false
	}
	req.Header.Add("Client-Information", "IntrinioRealtimeOptionsGoSDKv2.3")
	resp, httpDoErr := client.httpClient.Do(req)
	if httpDoErr != nil {
		log.Printf("Client - Authorization Failure: %v\n", httpDoErr)
		return false
	}
	if resp.StatusCode != 200 {
		log.Printf("Client - Authorization Failure: %v\n", resp.Status)
		return false
	}
	defer resp.Body.Close()
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf("Client - Authorization Failure: %v\n", readErr)
		return false
	}
	client.token = string(body)
	client.tokenUpdateTime = time.Now()
	log.Print("Client - Authorization successful")
	return true
}

func (client *Client) getToken() string {
	if time.Since(client.tokenUpdateTime) < (24 * time.Hour) {
		return client.token
	}
	doBackoff(client.trySetToken, &client.isStopped)
	return client.token
}

func (client *Client) initWebSocket(token string) {
	log.Println("Client - Connecting...")
	wsUrl := client.config.getWSUrl(token)
	wsHeader := map[string][]string{"UseNewEquitiesFormat": {"v2"}, "Client-Information": {"IntrinioRealtimeOptionsGoSDKv2.3"}}
	dialer := websocket.Dialer{
		ReadBufferSize:  10240,
		WriteBufferSize: 128,
	}
	conn, resp, dialErr := dialer.Dial(wsUrl, wsHeader)
	if dialErr != nil {
		log.Printf("Client - Connection failure: %v\n", dialErr)
		return
	}
	log.Printf("Client - Status: %s\n", resp.Status)
	client.wsConn = conn
	if reflect.ValueOf(client.heartbeat).IsZero() {
		//log.Println("Client - Starting heartbeat")
		client.heartbeat = time.NewTicker(20 * time.Second)
	}
	client.isClosed = false
}

func (client *Client) tryResetWebSocket() bool {
	wsUrl := client.config.getWSUrl(client.token)
	wsHeader := map[string][]string{"UseNewEquitiesFormat": {"true"}}
	dialer := websocket.Dialer{
		ReadBufferSize:  10240,
		WriteBufferSize: 128,
	}
	conn, resp, dialErr := dialer.Dial(wsUrl, wsHeader)
	if dialErr != nil {
		return false
	}
	log.Printf("Client - Status: %s\n", resp.Status)
	client.wsConn = conn
	log.Printf("Client - Rejoining")
	for key := range client.subscriptions {
		client.writeChannel <- client.composeJoinMsg(key)
	}
	client.reconnected <- true
	client.isClosed = false
	return true
}

func (client *Client) reconnect() {
	client.wsConn.Close()
	time.Sleep(10 * time.Second)
	doBackoff(func() bool {
		log.Println("Client - Reconnecting...")
		if time.Since(client.tokenUpdateTime) < (24 * time.Hour) {
			return client.tryResetWebSocket()
		} else {
			if client.trySetToken() {
				return client.tryResetWebSocket()
			} else {
				return false
			}
		}
	}, &client.isStopped)
}

func (client *Client) write() {
	for {
		if client.isStopped {
			remainingWriteCount := len(client.writeChannel)
			for i := 0; i < remainingWriteCount; i++ {
				data := <-client.writeChannel
				client.wsConn.WriteMessage(websocket.BinaryMessage, data)
			}
			time.Sleep(500 * time.Millisecond)
			log.Println("Client - Sending close message")
			client.wsConn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second*2))
			return
		}
		if client.isClosed {
			time.Sleep(time.Second)
		} else {
			select {
			case <-client.heartbeat.C:
				client.wsConn.WriteMessage(websocket.BinaryMessage, []byte{})
				client.LogStats()
				if len(client.writeChannel) < 2 {
					time.Sleep(time.Duration(500) * time.Millisecond)
				}
			default:
				select {
				case data := <-client.writeChannel:
					client.wsConn.WriteMessage(websocket.BinaryMessage, data)
				default:
				}
				if len(client.writeChannel) < 2 {
					time.Sleep(time.Duration(500) * time.Millisecond)
				}
			}
		}
	}
}

func (client *Client) read() {
	var highWatermark int = cap(client.readChannel) * 9 / 10
	var queueFull bool = false
	for {
		msgType, data, err := client.wsConn.ReadMessage()
		if err != nil {
			client.isClosed = true
			log.Printf("Client - Received message '%v'\n", err)
			if client.isStopped {
				return
			}
			go client.reconnect()
			<-client.reconnected
			log.Println("Client - Reconnected")
		} else if msgType == websocket.BinaryMessage {
			client.dataMsgCount++
			select {
			case client.readChannel <- data:
				if queueFull && len(client.readChannel) < highWatermark {
					queueFull = false
					log.Println("Client - read channel draining")
				}
			default:
				if !queueFull {
					log.Println("Client - read channel full")
					queueFull = true
				}
			}
		} else if msgType == websocket.TextMessage {
			client.txtMsgCount++
			log.Printf("Client - %s\n", string(data))
		}
	}
}

func (client *Client) Start() {
	client.isStopped = false
	token := client.getToken()
	client.initWebSocket(token)
	for w := 0; w < client.workerCount; w++ {
		client.closeWg.Add(1)
		go client.work()
	}
	go client.read()
	go client.write()
}

func (client *Client) Join(symbol string) {
	s := strings.TrimSpace(symbol)
	if s != "" {
		for client.isClosed {
			time.Sleep(time.Second)
		}
		if !client.subscriptions[symbol] {
			client.subscriptions[symbol] = true
			client.writeChannel <- client.composeJoinMsg(symbol)
		}
	}
}

func (client *Client) JoinMany(symbols []string) {
	for client.isClosed {
		time.Sleep(time.Second)
	}
	for i := 0; i < len(symbols); i++ {
		s := strings.TrimSpace(symbols[i])
		if s != "" && !client.subscriptions[symbols[i]] {
			client.subscriptions[symbols[i]] = true
			client.writeChannel <- client.composeJoinMsg(symbols[i])
		}
	}
}

func (client *Client) JoinLobby() {
	for client.isClosed {
		time.Sleep(time.Second)
	}
	if !client.subscriptions["$FIREHOSE"] {
		client.subscriptions["$FIREHOSE"] = true
		client.writeChannel <- client.composeJoinMsg("$FIREHOSE")
	} else {
		log.Print("Client - lobby channel already joined")
	}
}

func (client *Client) LeaveAll() {
	for key := range client.subscriptions {
		client.writeChannel <- client.composeLeaveMsg(key)
		delete(client.subscriptions, key)
	}
}

func (client *Client) Leave(symbol string) {
	s := strings.TrimSpace(symbol)
	if s != "" {
		if client.subscriptions[symbol] {
			client.writeChannel <- client.composeLeaveMsg(symbol)
			delete(client.subscriptions, symbol)
		}
	}
}

func (client *Client) LeaveMany(symbols []string) {
	for i := 0; i < len(symbols); i++ {
		client.Leave(symbols[i])
	}
}

func (client *Client) LeaveLobby(composeLeave func(string)) {
	if client.subscriptions["$FIREHOSE"] {
		client.writeChannel <- client.composeLeaveMsg("$FIREHOSE")
		delete(client.subscriptions, "$FIREHOSE")
	}
}

func (client *Client) Stop() {
	log.Println("Client - Stopping...")
	client.LeaveAll()
	client.isStopped = true
	client.closeWg.Wait()
	//client.LogStats()
	log.Println("Client - Stopped")
}

func (client *Client) LogStats() {
	log.Printf("Client - Data Message Count: %d, Queue Depth: %d", client.dataMsgCount, len(client.readChannel))
}
