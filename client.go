package intrinio

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

var selfHealBackoffs = []time.Duration{
	10 * time.Second,
	30 * time.Second,
	60 * time.Second,
	300 * time.Second,
	600 * time.Second,
}

const (
	HEARTBEAT_INTERVAL       int = 20
	MAX_OPTIONS_QUEUE_DEPTH  int = 20000
	MAX_EQUITIES_QUEUE_DEPTH int = 10000
	defaultConnectTimeout        = 30 * time.Second
	defaultReconnectDelay        = 10 * time.Second
)

func waitFor(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func doBackoff(ctx context.Context, backoffs []time.Duration, fn func() bool) bool {
	if len(backoffs) == 0 {
		backoffs = []time.Duration{0}
	}

	for attempt := 0; ctx.Err() == nil; attempt++ {
		success := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Client - Attempt failed: %v\n", r)
				}
			}()
			success = fn()
		}()
		if success {
			return true
		}
		delayIndex := attempt
		if delayIndex >= len(backoffs) {
			delayIndex = len(backoffs) - 1
		}
		if !waitFor(ctx, backoffs[delayIndex]) {
			return false
		}
	}
	return false
}

func newWebSocketDialer(timeout time.Duration) websocket.Dialer {
	return websocket.Dialer{
		ReadBufferSize:   10240,
		WriteBufferSize:  128,
		HandshakeTimeout: timeout,
	}
}

type Client struct {
	stateMu         sync.RWMutex
	subscriptionsMu sync.Mutex
	token           string
	tokenUpdateTime time.Time
	dataMsgCount    atomic.Uint64
	txtMsgCount     atomic.Uint32
	workerCount     int
	subscriptions   map[string]bool
	isStopped       bool
	isClosed        bool
	closeWg         sync.WaitGroup
	runWg           sync.WaitGroup
	readChannel     chan []byte
	writeChannel    chan []byte
	httpClient      *http.Client
	wsConn          *websocket.Conn
	config          Config
	work            func()
	composeJoinMsg  func(string) []byte
	composeLeaveMsg func(string) []byte
	ctx             context.Context
	cancel          context.CancelFunc
	connectTimeout  time.Duration
	reconnectDelay  time.Duration
	backoffs        []time.Duration
	heartbeat       time.Duration
}

func newClient(c Config, workerCount, readQueueDepth int) *Client {
	return &Client{
		isStopped:      true,
		isClosed:       true,
		workerCount:    workerCount,
		readChannel:    make(chan []byte, readQueueDepth),
		writeChannel:   make(chan []byte, 1000),
		subscriptions:  make(map[string]bool),
		httpClient:     http.DefaultClient,
		config:         c,
		connectTimeout: defaultConnectTimeout,
		reconnectDelay: defaultReconnectDelay,
		backoffs:       append([]time.Duration(nil), selfHealBackoffs...),
		heartbeat:      time.Duration(HEARTBEAT_INTERVAL) * time.Second,
	}
}

func NewOptionsClient(
	c Config,
	onTrade func(OptionTrade),
	onQuote func(OptionQuote),
	onRefresh func(OptionRefresh),
	onUnusualActivity func(OptionUnusualActivity)) *Client {
	client := newClient(c, 1, MAX_OPTIONS_QUEUE_DEPTH)
	if onTrade != nil {
		client.workerCount++
	}
	if onQuote != nil {
		client.workerCount += 8
	}
	client.work = func() {
		defer client.closeWg.Done()
		for {
			if len(client.readChannel) == 0 {
				if client.stoppedAndClosed() {
					return
				}
				time.Sleep(time.Second)
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
	client := newClient(c, 2, MAX_EQUITIES_QUEUE_DEPTH)
	if onQuote != nil {
		client.workerCount += 2
	}
	client.work = func() {
		defer client.closeWg.Done()
		for {
			if len(client.readChannel) == 0 {
				if client.stoppedAndClosed() {
					return
				}
				time.Sleep(time.Second)
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

func (client *Client) operationContext() context.Context {
	client.stateMu.RLock()
	ctx := client.ctx
	client.stateMu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (client *Client) timeout() time.Duration {
	client.stateMu.RLock()
	timeout := client.connectTimeout
	client.stateMu.RUnlock()
	if timeout <= 0 {
		return defaultConnectTimeout
	}
	return timeout
}

func (client *Client) stoppedAndClosed() bool {
	client.stateMu.RLock()
	result := client.isStopped && client.isClosed
	client.stateMu.RUnlock()
	return result
}

func (client *Client) stopped() bool {
	client.stateMu.RLock()
	result := client.isStopped
	client.stateMu.RUnlock()
	return result
}

func (client *Client) trySetToken() bool {
	log.Print("Client - Authorizing...")
	ctx, cancel := context.WithTimeout(client.operationContext(), client.timeout())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.config.getAuthUrl(), nil)
	if err != nil {
		log.Printf("Client - Authorization Failure: %v\n", err)
		return false
	}
	req.Header.Add("Client-Information", "IntrinioRealtimeOptionsGoSDKv4.0")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		log.Printf("Client - Authorization Failure: %v\n", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("Client - Authorization Failure: %v\n", resp.Status)
		return false
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Client - Authorization Failure: %v\n", err)
		return false
	}

	client.stateMu.Lock()
	client.token = string(body)
	client.tokenUpdateTime = time.Now()
	client.stateMu.Unlock()
	log.Print("Client - Authorization successful")
	return true
}

func (client *Client) tokenValue() (string, bool) {
	client.stateMu.RLock()
	token := client.token
	fresh := time.Since(client.tokenUpdateTime) < 24*time.Hour
	client.stateMu.RUnlock()
	return token, fresh
}

func (client *Client) getToken() string {
	if token, fresh := client.tokenValue(); fresh {
		return token
	}
	ctx := client.operationContext()
	client.stateMu.RLock()
	backoffs := append([]time.Duration(nil), client.backoffs...)
	client.stateMu.RUnlock()
	doBackoff(ctx, backoffs, client.trySetToken)
	token, _ := client.tokenValue()
	return token
}

func (client *Client) dialWebSocket(wsURL string, wsHeader http.Header) (*websocket.Conn, *http.Response, error) {
	timeout := client.timeout()
	dialer := newWebSocketDialer(timeout)
	ctx, cancel := context.WithTimeout(client.operationContext(), timeout)
	defer cancel()
	return dialer.DialContext(ctx, wsURL, wsHeader)
}

func (client *Client) currentConnection() *websocket.Conn {
	client.stateMu.RLock()
	conn := client.wsConn
	client.stateMu.RUnlock()
	return conn
}

func (client *Client) installConnection(conn *websocket.Conn) bool {
	client.stateMu.Lock()
	if client.ctx != nil && (client.isStopped || client.ctx.Err() != nil) {
		client.stateMu.Unlock()
		_ = conn.Close()
		return false
	}
	oldConn := client.wsConn
	client.wsConn = conn
	client.isClosed = false
	client.stateMu.Unlock()
	if oldConn != nil && oldConn != conn {
		_ = oldConn.Close()
	}
	return true
}

func (client *Client) closeWebSocket() {
	client.stateMu.Lock()
	conn := client.wsConn
	client.wsConn = nil
	client.isClosed = true
	client.stateMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (client *Client) markConnectionFailed(conn *websocket.Conn) {
	client.stateMu.Lock()
	if client.wsConn == conn {
		client.wsConn = nil
		client.isClosed = true
	}
	client.stateMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
}

func (client *Client) writeMessage(conn *websocket.Conn, data []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(client.timeout())); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.BinaryMessage, data)
}

func (client *Client) drainWriteChannel() {
	for {
		select {
		case <-client.writeChannel:
		default:
			return
		}
	}
}

func (client *Client) connectAndRejoin(token string, headers http.Header) bool {
	conn, resp, err := client.dialWebSocket(client.config.getWSUrl(token), headers)
	if err != nil {
		log.Printf("Client - Connection attempt failed: %v\n", err)
		if conn != nil {
			_ = conn.Close()
		}
		return false
	}
	if resp != nil {
		log.Printf("Client - Status: %s\n", resp.Status)
	}

	client.subscriptionsMu.Lock()
	defer client.subscriptionsMu.Unlock()
	client.drainWriteChannel()
	if len(client.subscriptions) > 0 {
		log.Printf("Client - Rejoining %d subscriptions", len(client.subscriptions))
	}
	for symbol := range client.subscriptions {
		if err := client.writeMessage(conn, client.composeJoinMsg(symbol)); err != nil {
			log.Printf("Client - Rejoin failed: %v\n", err)
			_ = conn.Close()
			return false
		}
	}
	return client.installConnection(conn)
}

func (client *Client) initWebSocket(token string) bool {
	log.Println("Client - Connecting...")
	headers := http.Header{
		"UseNewEquitiesFormat": {"v2"},
		"Client-Information":   {"IntrinioRealtimeOptionsGoSDKv4.0"},
	}
	return client.connectAndRejoin(token, headers)
}

func (client *Client) tryResetWebSocket() bool {
	token, _ := client.tokenValue()
	return client.connectAndRejoin(token, http.Header{"UseNewEquitiesFormat": {"true"}})
}

func (client *Client) reconnect() bool {
	client.closeWebSocket()
	ctx := client.operationContext()
	client.stateMu.RLock()
	delay := client.reconnectDelay
	backoffs := append([]time.Duration(nil), client.backoffs...)
	client.stateMu.RUnlock()
	if !waitFor(ctx, delay) {
		return false
	}

	return doBackoff(ctx, backoffs, func() bool {
		log.Println("Client - Reconnecting...")
		if _, fresh := client.tokenValue(); !fresh && !client.trySetToken() {
			return false
		}
		return client.tryResetWebSocket()
	})
}

func (client *Client) write() {
	defer client.runWg.Done()
	interval := client.heartbeat
	if interval <= 0 {
		interval = time.Duration(HEARTBEAT_INTERVAL) * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ctx := client.operationContext()

	for {
		if ctx.Err() != nil || client.stopped() {
			return
		}
		conn := client.currentConnection()
		if conn == nil {
			if !waitFor(ctx, 10*time.Millisecond) {
				return
			}
			continue
		}

		var data []byte
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data = []byte{}
			client.LogStats()
		case data = <-client.writeChannel:
		}
		if err := client.writeMessage(conn, data); err != nil {
			log.Printf("Client - Write failed: %v\n", err)
			client.markConnectionFailed(conn)
		}
	}
}

func (client *Client) read() {
	defer client.runWg.Done()
	highWatermark := cap(client.readChannel) * 9 / 10
	queueFull := false
	for {
		if client.stopped() {
			return
		}
		conn := client.currentConnection()
		if conn == nil {
			if !client.reconnect() {
				return
			}
			continue
		}

		msgType, data, err := conn.ReadMessage()
		if err != nil {
			client.markConnectionFailed(conn)
			log.Printf("Client - Received message '%v'\n", err)
			if client.stopped() || !client.reconnect() {
				return
			}
			log.Println("Client - Reconnected")
			continue
		}

		switch msgType {
		case websocket.BinaryMessage:
			client.dataMsgCount.Add(1)
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
		case websocket.TextMessage:
			client.txtMsgCount.Add(1)
			log.Printf("Client - %s\n", string(data))
		}
	}
}

func (client *Client) Start() {
	client.stateMu.Lock()
	if !client.isStopped {
		client.stateMu.Unlock()
		return
	}
	client.ctx, client.cancel = context.WithCancel(context.Background())
	client.isStopped = false
	client.isClosed = true
	ctx := client.ctx
	backoffs := append([]time.Duration(nil), client.backoffs...)
	client.stateMu.Unlock()

	token := client.getToken()
	if ctx.Err() != nil {
		return
	}
	if !doBackoff(ctx, backoffs, func() bool { return client.initWebSocket(token) }) {
		return
	}

	client.stateMu.Lock()
	if client.isStopped {
		client.stateMu.Unlock()
		client.closeWebSocket()
		return
	}
	client.closeWg.Add(client.workerCount)
	client.runWg.Add(2)
	client.stateMu.Unlock()
	for worker := 0; worker < client.workerCount; worker++ {
		go client.work()
	}
	go client.read()
	go client.write()
}

func (client *Client) enqueueIfConnected(data []byte) {
	ctx := client.operationContext()
	for {
		if client.currentConnection() == nil || client.stopped() {
			return
		}
		select {
		case client.writeChannel <- data:
			return
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func (client *Client) Join(symbol string) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return
	}
	client.subscriptionsMu.Lock()
	defer client.subscriptionsMu.Unlock()
	if client.subscriptions[symbol] {
		return
	}
	client.subscriptions[symbol] = true
	client.enqueueIfConnected(client.composeJoinMsg(symbol))
}

func (client *Client) JoinMany(symbols []string) {
	for _, symbol := range symbols {
		client.Join(symbol)
	}
}

func (client *Client) JoinLobby() {
	client.Join("$FIREHOSE")
}

func (client *Client) LeaveAll() {
	client.subscriptionsMu.Lock()
	defer client.subscriptionsMu.Unlock()
	for symbol := range client.subscriptions {
		delete(client.subscriptions, symbol)
		client.enqueueIfConnected(client.composeLeaveMsg(symbol))
	}
}

func (client *Client) Leave(symbol string) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return
	}
	client.subscriptionsMu.Lock()
	defer client.subscriptionsMu.Unlock()
	if !client.subscriptions[symbol] {
		return
	}
	delete(client.subscriptions, symbol)
	client.enqueueIfConnected(client.composeLeaveMsg(symbol))
}

func (client *Client) LeaveMany(symbols []string) {
	for _, symbol := range symbols {
		client.Leave(symbol)
	}
}

func (client *Client) LeaveLobby(composeLeave func(string)) {
	client.Leave("$FIREHOSE")
}

func (client *Client) Stop() {
	log.Println("Client - Stopping...")
	client.stateMu.Lock()
	if client.isStopped {
		client.stateMu.Unlock()
		return
	}
	client.isStopped = true
	cancel := client.cancel
	client.stateMu.Unlock()
	if cancel != nil {
		cancel()
	}
	client.closeWebSocket()
	client.LeaveAll()
	client.closeWg.Wait()
	client.runWg.Wait()
	log.Println("Client - Stopped")
}

func (client *Client) LogStats() {
	log.Printf("Client - Data Message Count: %d, Queue Depth: %d", client.dataMsgCount.Load(), len(client.readChannel))
}
