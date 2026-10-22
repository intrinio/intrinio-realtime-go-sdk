package intrinio

import (
	"context"
	"io"
	"log"
	"net"
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

type clientLifecycle uint8

const (
	clientStopped clientLifecycle = iota
	clientStarting
	clientStarted
	clientStopping
)

type clientRun struct {
	ctx       context.Context
	cancel    context.CancelFunc
	startDone chan struct{}
	done      chan struct{}
	startOnce sync.Once
	doneOnce  sync.Once
	wg        sync.WaitGroup
}

func newClientRun() *clientRun {
	ctx, cancel := context.WithCancel(context.Background())
	return &clientRun{
		ctx:       ctx,
		cancel:    cancel,
		startDone: make(chan struct{}),
		done:      make(chan struct{}),
	}
}

func (run *clientRun) signalStartDone() {
	run.startOnce.Do(func() { close(run.startDone) })
}

func (run *clientRun) signalDone() {
	run.doneOnce.Do(func() { close(run.done) })
}

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
				if recovered := recover(); recovered != nil {
					log.Printf("Client - Attempt failed: %v\n", recovered)
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
	lifecycle       clientLifecycle
	run             *clientRun
	readChannel     chan []byte
	priorityQueue   *priorityQueue
	writeChannel    chan []byte
	httpClient      *http.Client
	wsConn          *websocket.Conn
	connectingConn  *websocket.Conn
	config          Config
	work            func(context.Context)
	composeJoinMsg  func(string) []byte
	composeLeaveMsg func(string) []byte
	socketHeaders   http.Header
	connectTimeout  time.Duration
	reconnectDelay  time.Duration
	backoffs        []time.Duration
	heartbeat       time.Duration
}

func newClient(c Config, workerCount, readQueueDepth int) *Client {
	return &Client{
		workerCount:    workerCount,
		readChannel:    make(chan []byte, readQueueDepth),
		writeChannel:   make(chan []byte, 1000),
		subscriptions:  make(map[string]bool),
		httpClient:     http.DefaultClient,
		config:         c,
		lifecycle:      clientStopped,
		connectTimeout: defaultConnectTimeout,
		reconnectDelay: defaultReconnectDelay,
		backoffs:       append([]time.Duration(nil), selfHealBackoffs...),
		heartbeat:      time.Duration(HEARTBEAT_INTERVAL) * time.Second,
		socketHeaders: http.Header{
			"Client-Information": {"IntrinioRealtimeOptionsGoSDKv4.1"},
		},
	}
}

func NewOptionsClient(
	c Config,
	onTrade func(OptionTrade),
	onQuote func(OptionQuote),
	onRefresh func(OptionRefresh),
	onUnusualActivity func(OptionUnusualActivity)) *Client {
	client := newClient(c, 1, MAX_OPTIONS_QUEUE_DEPTH)
	client.priorityQueue = newPriorityQueue(4, MAX_OPTIONS_QUEUE_DEPTH)
	client.socketHeaders.Set("UseNewOptionsFormat", "v2")
	if c.Delayed {
		client.socketHeaders.Set("Delay", "true")
	}
	if onTrade != nil {
		client.workerCount++
	}
	if onQuote != nil {
		client.workerCount += 8
	}
	client.work = func(ctx context.Context) {
		for ctx.Err() == nil {
			didWork := workOnOptions(
				client.priorityQueue,
				onTrade,
				onQuote,
				onRefresh,
				onUnusualActivity)

			select {
			case <-ctx.Done():
				return
			case data := <-client.readChannel:
				didWork = true
				err := queueOptionMessages(data, client.priorityQueue, func() bool {
					return workOnOptions(
						client.priorityQueue,
						onTrade,
						onQuote,
						onRefresh,
						onUnusualActivity)
				})
				if err != nil {
					log.Printf("Option Client - Invalid data message: %v", err)
				}
			default:
			}

			if workOnOptions(
				client.priorityQueue,
				onTrade,
				onQuote,
				onRefresh,
				onUnusualActivity) {
				didWork = true
			}
			if !didWork && !waitFor(ctx, 10*time.Millisecond) {
				return
			}
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
	client.priorityQueue = newPriorityQueue(2, MAX_EQUITIES_QUEUE_DEPTH)
	client.socketHeaders.Set("UseNewEquitiesFormat", "v2")
	if onQuote != nil {
		client.workerCount += 2
	}
	client.work = func(ctx context.Context) {
		for ctx.Err() == nil {
			didWork := workOnEquities(
				client.priorityQueue,
				onTrade,
				onQuote)

			select {
			case <-ctx.Done():
				return
			case data := <-client.readChannel:
				didWork = true
				err := queueEquityMessages(data, client.priorityQueue, func() bool {
					return workOnEquities(
						client.priorityQueue,
						onTrade,
						onQuote)
				})
				if err != nil {
					log.Printf("Equity Client - Invalid data message: %v", err)
				}
			default:
			}

			if workOnEquities(
				client.priorityQueue,
				onTrade,
				onQuote) {
				didWork = true
			}
			if !didWork && !waitFor(ctx, 10*time.Millisecond) {
				return
			}
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

func (client *Client) timeout() time.Duration {
	client.stateMu.RLock()
	timeout := client.connectTimeout
	client.stateMu.RUnlock()
	if timeout <= 0 {
		return defaultConnectTimeout
	}
	return timeout
}

func (client *Client) currentRun() *clientRun {
	client.stateMu.RLock()
	run := client.run
	client.stateMu.RUnlock()
	return run
}

func (client *Client) operationContext() context.Context {
	run := client.currentRun()
	if run == nil {
		return context.Background()
	}
	return run.ctx
}

func (client *Client) trySetTokenForRun(run *clientRun) bool {
	log.Print("Client - Authorizing...")
	baseContext := context.Background()
	if run != nil {
		baseContext = run.ctx
	}
	ctx, cancel := context.WithTimeout(baseContext, client.timeout())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.config.getAuthUrl(), nil)
	if err != nil {
		log.Printf("Client - Authorization Failure: %v\n", err)
		return false
	}
	req.Header.Add("Client-Information", "IntrinioRealtimeOptionsGoSDKv4.1")
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

func (client *Client) trySetToken() bool {
	return client.trySetTokenForRun(client.currentRun())
}

func (client *Client) tokenValue() (string, bool) {
	client.stateMu.RLock()
	token := client.token
	fresh := token != "" && time.Since(client.tokenUpdateTime) < 24*time.Hour
	client.stateMu.RUnlock()
	return token, fresh
}

func (client *Client) settings() ([]time.Duration, time.Duration) {
	client.stateMu.RLock()
	backoffs := append([]time.Duration(nil), client.backoffs...)
	reconnectDelay := client.reconnectDelay
	client.stateMu.RUnlock()
	return backoffs, reconnectDelay
}

func (client *Client) dialWebSocket(
	run *clientRun,
	wsURL string,
	wsHeader http.Header) (*websocket.Conn, *http.Response, error) {
	timeout := client.timeout()
	dialer := newWebSocketDialer(timeout)
	baseContext := context.Background()
	if run != nil {
		baseContext = run.ctx
	}
	ctx, cancel := context.WithTimeout(baseContext, timeout)

	// Gorilla WebSocket uses the context deadline during the HTTP upgrade, but
	// cancellation alone does not interrupt a server that accepts the TCP
	// connection and never answers the upgrade request. Keep hold of the raw
	// connection so Stop can abort TLS and upgrade handshakes
	// promptly instead of waiting for the full connection timeout.
	var pendingMu sync.Mutex
	var pending net.Conn
	netDialer := &net.Dialer{}
	dialer.NetDialContext = func(dialCtx context.Context, network, address string) (net.Conn, error) {
		conn, err := netDialer.DialContext(dialCtx, network, address)
		if err != nil {
			return nil, err
		}
		pendingMu.Lock()
		if ctx.Err() != nil {
			pendingMu.Unlock()
			_ = conn.Close()
			return nil, ctx.Err()
		}
		pending = conn
		pendingMu.Unlock()
		return conn, nil
	}

	watcherDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			pendingMu.Lock()
			if pending != nil {
				_ = pending.Close()
			}
			pendingMu.Unlock()
		case <-watcherDone:
		}
	}()

	conn, resp, err := dialer.DialContext(ctx, wsURL, wsHeader)
	pendingMu.Lock()
	pending = nil
	pendingMu.Unlock()
	close(watcherDone)
	ctxErr := ctx.Err()
	cancel()
	if err == nil && ctxErr != nil {
		_ = conn.Close()
		return nil, resp, ctxErr
	}
	return conn, resp, err
}

func (client *Client) currentConnection() *websocket.Conn {
	client.stateMu.RLock()
	conn := client.wsConn
	client.stateMu.RUnlock()
	return conn
}

func (client *Client) currentConnectionForRun(run *clientRun) *websocket.Conn {
	client.stateMu.RLock()
	defer client.stateMu.RUnlock()
	if client.run != run || client.lifecycle == clientStopping || client.lifecycle == clientStopped {
		return nil
	}
	return client.wsConn
}

func (client *Client) isCurrentConnection(run *clientRun, conn *websocket.Conn) bool {
	client.stateMu.RLock()
	defer client.stateMu.RUnlock()
	return client.run == run && client.wsConn == conn
}

func (client *Client) trackConnectingConnection(run *clientRun, conn *websocket.Conn) bool {
	client.stateMu.Lock()
	defer client.stateMu.Unlock()
	if run != nil && (client.run != run || client.lifecycle == clientStopping ||
		client.lifecycle == clientStopped || run.ctx.Err() != nil) {
		return false
	}
	client.connectingConn = conn
	return true
}

func (client *Client) clearConnectingConnection(conn *websocket.Conn) {
	client.stateMu.Lock()
	if client.connectingConn == conn {
		client.connectingConn = nil
	}
	client.stateMu.Unlock()
}

func (client *Client) installConnection(run *clientRun, conn *websocket.Conn) bool {
	client.stateMu.Lock()
	if run != nil && (client.run != run || client.lifecycle == clientStopping ||
		client.lifecycle == clientStopped || run.ctx.Err() != nil) {
		if client.connectingConn == conn {
			client.connectingConn = nil
		}
		client.stateMu.Unlock()
		_ = conn.Close()
		return false
	}
	oldConn := client.wsConn
	client.wsConn = conn
	if client.connectingConn == conn {
		client.connectingConn = nil
	}
	client.stateMu.Unlock()
	if oldConn != nil && oldConn != conn {
		_ = oldConn.Close()
	}
	return true
}

func (client *Client) closeWebSocketForRun(run *clientRun) {
	client.stateMu.Lock()
	if run != nil && client.run != run {
		client.stateMu.Unlock()
		return
	}
	conn := client.wsConn
	connectingConn := client.connectingConn
	client.wsConn = nil
	client.connectingConn = nil
	client.stateMu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	if connectingConn != nil && connectingConn != conn {
		_ = connectingConn.Close()
	}
}

func (client *Client) closeWebSocket() {
	client.closeWebSocketForRun(nil)
}

func (client *Client) markConnectionFailed(conn *websocket.Conn) {
	client.stateMu.Lock()
	if client.wsConn == conn {
		client.wsConn = nil
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

func (client *Client) webSocketHeaders() http.Header {
	headers := make(http.Header, len(client.socketHeaders))
	for key, values := range client.socketHeaders {
		headers[key] = append([]string(nil), values...)
	}
	return headers
}

func (client *Client) connectAndRejoin(run *clientRun, token string) bool {
	conn, resp, err := client.dialWebSocket(run, client.config.getWSUrl(token), client.webSocketHeaders())
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if conn != nil {
			_ = conn.Close()
		}
		log.Printf("Client - Connection attempt failed: %v\n", err)
		return false
	}
	if resp != nil {
		log.Printf("Client - Status: %s\n", resp.Status)
	}
	if !client.trackConnectingConnection(run, conn) {
		_ = conn.Close()
		return false
	}

	client.subscriptionsMu.Lock()
	defer client.subscriptionsMu.Unlock()
	client.drainWriteChannel()
	if len(client.subscriptions) > 0 {
		log.Printf("Client - Rejoining %d subscriptions", len(client.subscriptions))
	}
	for symbol := range client.subscriptions {
		if run != nil && run.ctx.Err() != nil {
			client.clearConnectingConnection(conn)
			_ = conn.Close()
			return false
		}
		if err := client.writeMessage(conn, client.composeJoinMsg(symbol)); err != nil {
			log.Printf("Client - Rejoin failed: %v\n", err)
			client.clearConnectingConnection(conn)
			_ = conn.Close()
			return false
		}
	}
	return client.installConnection(run, conn)
}

func (client *Client) initWebSocket(run *clientRun, token string) bool {
	log.Println("Client - Connecting...")
	return client.connectAndRejoin(run, token)
}

func (client *Client) tryResetWebSocket() bool {
	token, _ := client.tokenValue()
	return client.connectAndRejoin(client.currentRun(), token)
}

func (client *Client) reconnect(run *clientRun) bool {
	client.closeWebSocketForRun(run)
	backoffs, reconnectDelay := client.settings()
	if !waitFor(run.ctx, reconnectDelay) {
		return false
	}

	return doBackoff(run.ctx, backoffs, func() bool {
		log.Println("Client - Reconnecting...")
		token, fresh := client.tokenValue()
		if !fresh {
			if !client.trySetTokenForRun(run) {
				return false
			}
			token, _ = client.tokenValue()
		}
		return client.connectAndRejoin(run, token)
	})
}

func (client *Client) write(run *clientRun) {
	interval := client.heartbeat
	if interval <= 0 {
		interval = time.Duration(HEARTBEAT_INTERVAL) * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for run.ctx.Err() == nil {
		conn := client.currentConnectionForRun(run)
		if conn == nil {
			if !waitFor(run.ctx, 10*time.Millisecond) {
				return
			}
			continue
		}

		var data []byte
		select {
		case <-run.ctx.Done():
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

func (client *Client) read(run *clientRun) {
	highWatermark := cap(client.readChannel) * 9 / 10
	queueFull := false
	for run.ctx.Err() == nil {
		conn := client.currentConnectionForRun(run)
		if conn == nil {
			if !client.reconnect(run) {
				return
			}
			continue
		}

		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if !client.isCurrentConnection(run, conn) {
				continue
			}
			client.markConnectionFailed(conn)
			log.Printf("Client - Received message '%v'\n", err)
			if run.ctx.Err() != nil || !client.reconnect(run) {
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

func (client *Client) drainSessionBuffers() {
	for {
		select {
		case <-client.readChannel:
		default:
			goto readDrained
		}
	}

readDrained:
	if client.priorityQueue != nil {
		client.priorityQueue.reset()
	}
	client.dataMsgCount.Store(0)
	client.txtMsgCount.Store(0)
}

func (client *Client) startRun(run *clientRun) {
	backoffs, _ := client.settings()
	connected := doBackoff(run.ctx, backoffs, func() bool {
		token, fresh := client.tokenValue()
		if !fresh {
			if !client.trySetTokenForRun(run) {
				return false
			}
			token, _ = client.tokenValue()
		}
		return client.initWebSocket(run, token)
	})
	if !connected {
		run.signalStartDone()
		return
	}

	client.stateMu.Lock()
	if client.run != run || client.lifecycle != clientStarting || run.ctx.Err() != nil {
		client.stateMu.Unlock()
		client.closeWebSocketForRun(run)
		run.signalStartDone()
		return
	}
	client.lifecycle = clientStarted
	run.wg.Add(client.workerCount + 2)
	client.stateMu.Unlock()

	for worker := 0; worker < client.workerCount; worker++ {
		go func() {
			defer run.wg.Done()
			client.work(run.ctx)
		}()
	}
	go func() {
		defer run.wg.Done()
		client.read(run)
	}()
	go func() {
		defer run.wg.Done()
		client.write(run)
	}()
	run.signalStartDone()
}

func (client *Client) Start() {
	for {
		client.stateMu.Lock()
		switch client.lifecycle {
		case clientStopped:
			run := newClientRun()
			client.run = run
			client.lifecycle = clientStarting
			client.stateMu.Unlock()
			client.drainSessionBuffers()
			client.startRun(run)
			return
		case clientStarting:
			run := client.run
			client.stateMu.Unlock()
			<-run.startDone
			client.stateMu.RLock()
			started := client.run == run && client.lifecycle == clientStarted
			client.stateMu.RUnlock()
			if started {
				return
			}
		case clientStarted:
			client.stateMu.Unlock()
			return
		case clientStopping:
			run := client.run
			client.stateMu.Unlock()
			<-run.done
		}
	}
}

func (client *Client) enqueueIfConnected(data []byte) {
	run := client.currentRun()
	if run == nil {
		return
	}
	for {
		if client.currentConnectionForRun(run) == nil {
			return
		}
		select {
		case client.writeChannel <- data:
			return
		case <-run.ctx.Done():
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
	switch client.lifecycle {
	case clientStopped:
		client.stateMu.Unlock()
		return
	case clientStopping:
		run := client.run
		client.stateMu.Unlock()
		<-run.done
		return
	case clientStarting, clientStarted:
		run := client.run
		client.lifecycle = clientStopping
		run.cancel()
		client.stateMu.Unlock()

		client.closeWebSocketForRun(run)
		<-run.startDone
		run.wg.Wait()
		client.drainSessionBuffers()
		client.drainWriteChannel()

		client.stateMu.Lock()
		if client.run == run {
			client.run = nil
			client.lifecycle = clientStopped
		}
		client.stateMu.Unlock()
		run.signalDone()
		log.Println("Client - Stopped")
	}
}

func (client *Client) LogStats() {
	priorityDepth := 0
	priorityDrops := uint64(0)
	if client.priorityQueue != nil {
		priorityDepth = client.priorityQueue.depth()
		priorityDrops = client.priorityQueue.dropCount()
	}
	log.Printf(
		"Client - Data Message Count: %d, Queue Depth: %d, Priority Queue Depth: %d, Priority Queue Drops: %d",
		client.dataMsgCount.Load(),
		len(client.readChannel),
		priorityDepth,
		priorityDrops)
}
