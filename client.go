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
	HEARTBEAT_INTERVAL int = 20
	MAX_SYMBOL_SIZE    int = 21
	TRADE_MSG_SIZE     int = 72
	QUOTE_MSG_SIZE     int = 52
	REFRESH_MSG_SIZE   int = 52
	UA_MSG_SIZE        int = 74
	MAX_QUEUE_DEPTH    int = 1000
)

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
	token             string
	tokenUpdateTime   time.Time
	dataMsgCount      uint64
	txtMsgCount       uint32
	workerCount       int
	subscriptions     map[string]bool
	isStopped         bool
	isClosed          bool
	closeWg           sync.WaitGroup
	reconnected       chan bool
	readChannel       chan []byte
	writeChannel      chan []byte
	httpClient        *http.Client
	wsConn            *websocket.Conn
	heartbeat         *time.Ticker
	config            Config
	OnTrade           func(Trade)
	OnQuote           func(Quote)
	OnRefresh         func(Refresh)
	OnUnusualActivity func(UnusualActivity)
}

func NewClient(c Config, onTrade func(Trade), onQuote func(Quote), onRefresh func(Refresh), onUnusualActivity func(UnusualActivity)) *Client {
	client := &Client{
		isStopped:     true,
		isClosed:      true,
		reconnected:   make(chan bool),
		readChannel:   make(chan []byte, MAX_QUEUE_DEPTH),
		writeChannel:  make(chan []byte, 1000),
		httpClient:    http.DefaultClient,
		config:        c,
		subscriptions: make(map[string]bool),
	}
	var workerCount int = 0
	if onTrade != nil {
		client.OnTrade = onTrade
		workerCount++
	}
	if onQuote != nil {
		client.OnQuote = onQuote
		workerCount += 3
	}
	if onRefresh != nil {
		client.OnRefresh = onRefresh
	}
	if onUnusualActivity != nil {
		client.OnUnusualActivity = onUnusualActivity
	}
	client.workerCount = workerCount
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
	req.Header.Add("Client-Information", "IntrinioRealtimeOptionsGoSDKv1.0")
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
	dialer := websocket.Dialer{
		ReadBufferSize:  10240,
		WriteBufferSize: 128,
	}
	conn, resp, dialErr := dialer.Dial(wsUrl, nil)
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
	dialer := websocket.Dialer{
		ReadBufferSize:  10240,
		WriteBufferSize: 128,
	}
	conn, resp, dialErr := dialer.Dial(wsUrl, nil)
	if dialErr != nil {
		return false
	}
	log.Printf("Client - Status: %s\n", resp.Status)
	client.wsConn = conn
	log.Printf("Client - Rejoining")
	for key := range client.subscriptions {
		client.join(key)
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
				time.Now().Add(time.Second))
			return
		}
		if client.isClosed {
			time.Sleep(time.Second)
		} else {
			select {
			case <-client.heartbeat.C:
				client.wsConn.WriteMessage(websocket.BinaryMessage, []byte{})
				//client.LogStats()
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
	var highWatermark int = MAX_QUEUE_DEPTH * 9 / 10
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
					log.Println("Client - Quote channel full")
					queueFull = true
				}
			}
		} else if msgType == websocket.TextMessage {
			client.txtMsgCount++
			log.Printf("Client - %s\n", string(data))
		}
	}
}

func work(
	isClosed *bool,
	isStopped *bool,
	closeWg *sync.WaitGroup,
	readChannel <-chan []byte,
	onTrade func(Trade),
	onQuote func(Quote),
	onRefresh func(Refresh),
	onUA func(UnusualActivity)) {
	for {
		if len(readChannel) == 0 {
			if *isClosed && *isStopped {
				defer closeWg.Done()
				return
			} else {
				time.Sleep(time.Second)
			}
		}
		select {
		case data := <-readChannel:
			count := data[0]
			startIndex := 1
			for i := 0; i < int(count); i++ {
				msgType := data[startIndex+1+MAX_SYMBOL_SIZE]
				if msgType == 1 {
					quote := parseQuote(data[startIndex:(startIndex + QUOTE_MSG_SIZE)])
					startIndex = startIndex + QUOTE_MSG_SIZE
					if onQuote != nil {
						onQuote(quote)
					}
				} else if msgType == 0 {
					trade := parseTrade(data[startIndex:(startIndex + TRADE_MSG_SIZE)])
					startIndex = startIndex + TRADE_MSG_SIZE
					if onTrade != nil {
						onTrade(trade)
					}
				} else if msgType > 2 {
					ua := parseUA(data[startIndex:(startIndex + UA_MSG_SIZE)])
					startIndex = startIndex + UA_MSG_SIZE
					if onUA != nil {
						onUA(ua)
					}
				} else if msgType == 2 {
					refresh := parseRefresh(data[startIndex:(startIndex + REFRESH_MSG_SIZE)])
					startIndex = startIndex + REFRESH_MSG_SIZE
					if onRefresh != nil {
						onRefresh(refresh)
					}
				} else {
					log.Printf("Client - Invalid message type: %d", msgType)
				}
			}
		default:
		}
	}
}

func (client *Client) Start() {
	client.isStopped = false
	token := client.getToken()
	client.initWebSocket(token)
	for w := 0; w < client.workerCount; w++ {
		client.closeWg.Add(1)
		go work(
			&client.isClosed,
			&client.isStopped,
			&client.closeWg,
			client.readChannel,
			client.OnTrade,
			client.OnQuote,
			client.OnRefresh,
			client.OnUnusualActivity)
	}
	go client.read()
	go client.write()
}

func (client *Client) join(symbol string) {
	newSymbol := convertOldContractIdToNew(symbol)
	var mask uint8 = 0
	if client.OnTrade != nil {
		mask = mask | 1
	}
	if client.OnQuote != nil {
		mask = mask | 2
	}
	if client.OnRefresh != nil {
		mask = mask | 4
	}
	if client.OnUnusualActivity != nil {
		mask = mask | 8
	}
	message := make([]byte, 0, len(newSymbol)+2)
	message = append(message, 74, mask)
	message = append(message, []byte(newSymbol)...)
	log.Printf("Client - Joining channel %s\n", newSymbol)
	client.writeChannel <- message
}

func (client *Client) leave(symbol string) {
	if client.subscriptions[symbol] {
		delete(client.subscriptions, symbol)
		newSymbol := convertOldContractIdToNew(symbol)
		message := make([]byte, 0, len(newSymbol)+2)
		message = append(message, 76, 0)
		message = append(message, []byte(newSymbol)...)
		log.Printf("Client - Leaving channel %s\n", newSymbol)
		client.writeChannel <- message
	}
}

func (client *Client) Join(symbol string) {
	s := strings.TrimSpace(symbol)
	if s != "" {
		for client.isClosed {
			time.Sleep(time.Second)
		}
		if !client.subscriptions[symbol] {
			client.subscriptions[symbol] = true
			client.join(symbol)
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
			client.join(symbols[i])
		}
	}
}

func (client *Client) JoinLobby() {
	for client.isClosed {
		time.Sleep(time.Second)
	}
	if !client.subscriptions["$FIREHOSE"] {
		client.subscriptions["$FIREHOSE"] = true
		client.join("$FIREHOSE")
	} else {
		log.Print("Client - lobby channel already joined")
	}
}

func (client *Client) LeaveAll() {
	for key := range client.subscriptions {
		client.leave(key)
	}
}

func (client *Client) Leave(symbol string) {
	s := strings.TrimSpace(symbol)
	if s != "" {
		client.leave(s)
	}
}

func (client *Client) LeaveMany(symbols []string) {
	for i := 0; i < len(symbols); i++ {
		client.Leave(symbols[i])
	}
}

func (client *Client) LeaveLobby() {
	client.leave("$FIREHOSE")
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
