package intrinio

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newConnectionTestClient(serverURL string) *Client {
	client := newClient(Config{
		ApiKey:    "test-key",
		Provider:  MANUAL,
		IPAddress: strings.TrimPrefix(serverURL, "http://"),
	}, 0, 16)
	client.connectTimeout = 50 * time.Millisecond
	client.reconnectDelay = time.Millisecond
	client.backoffs = []time.Duration{time.Millisecond, 2 * time.Millisecond}
	client.heartbeat = time.Hour
	client.composeJoinMsg = func(symbol string) []byte { return []byte("J" + symbol) }
	client.composeLeaveMsg = func(symbol string) []byte { return []byte("L" + symbol) }
	return client
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", message)
}

func authHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "test-token")
}

func TestStartRetriesFailedInitialWebSocketConnection(t *testing.T) {
	var attempts atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	if attempts.Load() < 2 {
		t.Fatalf("expected initial connection retry, got %d attempt(s)", attempts.Load())
	}
	if client.currentConnection() == nil {
		t.Fatal("Start returned without a WebSocket connection")
	}
	client.Stop()
}

func TestAuthorizationTimeoutBacksOffAndCanBeStopped(t *testing.T) {
	var attempts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(_ http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = 15 * time.Millisecond
	done := make(chan struct{})
	go func() {
		client.Start()
		close(done)
	}()

	waitForCondition(t, 500*time.Millisecond, func() bool { return attempts.Load() >= 2 }, "authorization retry")
	client.Stop()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Start did not stop after canceling a stalled authorization call")
	}
}

func TestWebSocketHandshakeTimeoutBacksOffAndCanBeStopped(t *testing.T) {
	var attempts atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(_ http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = 15 * time.Millisecond
	done := make(chan struct{})
	go func() {
		client.Start()
		close(done)
	}()

	waitForCondition(t, 500*time.Millisecond, func() bool { return attempts.Load() >= 2 }, "WebSocket handshake retry")
	client.Stop()
	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Start did not stop after canceling a stalled WebSocket handshake")
	}
}

func TestStopDuringHungInitialHandshakeCompletes(t *testing.T) {
	handshakeStarted := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-handshakeStarted:
		default:
			close(handshakeStarted)
		}
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = time.Second
	startDone := make(chan struct{})
	go func() {
		client.Start()
		close(startDone)
	}()
	select {
	case <-handshakeStarted:
	case <-time.After(time.Second):
		t.Fatal("initial handshake did not start")
	}

	stopDone := make(chan struct{})
	go func() {
		client.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Stop did not cancel the hung initial handshake")
	}
	select {
	case <-startDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Start did not return after the hung handshake was canceled")
	}
}

func TestStopDuringHungAuthorizationCompletes(t *testing.T) {
	authStarted := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-authStarted:
		default:
			close(authStarted)
		}
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = time.Second
	startDone := make(chan struct{})
	go func() {
		client.Start()
		close(startDone)
	}()
	select {
	case <-authStarted:
	case <-time.After(time.Second):
		t.Fatal("authorization request did not start")
	}

	stopDone := make(chan struct{})
	go func() {
		client.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Stop did not cancel the hung authorization request")
	}
	select {
	case <-startDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Start did not return after authorization was canceled")
	}
}

func TestReconnectBacksOffAfterFailureAndRejoinsSubscriptions(t *testing.T) {
	var connections atomic.Int32
	firstJoin := make(chan string, 1)
	rejoined := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		if connectionNumber == 2 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if connectionNumber == 1 {
			firstJoin <- string(data)
			return
		}
		if connectionNumber == 3 {
			rejoined <- string(data)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	client.Join(" AAPL ")
	select {
	case message := <-firstJoin:
		if message != "JAAPL" {
			t.Fatalf("unexpected initial join %q", message)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("initial join was not sent")
	}
	select {
	case message := <-rejoined:
		if message != "JAAPL" {
			t.Fatalf("unexpected rejoin %q", message)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("subscription was not rejoined after reconnect")
	}
	if connections.Load() < 3 {
		t.Fatalf("expected a failed reconnect attempt followed by success, got %d connections", connections.Load())
	}
	client.Stop()
}

func TestRejoinMoreSubscriptionsThanWriteQueueCapacity(t *testing.T) {
	const subscriptionCount = 1500
	received := make(chan int, 1)
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		count := 0
		for count < subscriptionCount {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			count++
		}
		received <- count
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = time.Second
	client.stateMu.Lock()
	client.token = "test-token"
	client.tokenUpdateTime = time.Now()
	client.stateMu.Unlock()
	for i := 0; i < subscriptionCount; i++ {
		client.subscriptions[fmt.Sprintf("SYM%04d", i)] = true
	}
	for i := 0; i < cap(client.writeChannel); i++ {
		client.writeChannel <- []byte("stale")
	}

	done := make(chan bool, 1)
	go func() { done <- client.tryResetWebSocket() }()
	select {
	case success := <-done:
		if !success {
			t.Fatal("reconnect failed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("rejoin blocked on the bounded write queue")
	}
	select {
	case count := <-received:
		if count != subscriptionCount {
			t.Fatalf("received %d rejoin messages, want %d", count, subscriptionCount)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive all rejoin messages")
	}
	client.closeWebSocket()
}

func TestConcurrentCloseAndWriteDoesNotPanicOrRace(t *testing.T) {
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			client.Join(fmt.Sprintf("SYM%d", i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			conn := client.currentConnection()
			if conn != nil {
				client.markConnectionFailed(conn)
			}
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()
	waitForCondition(t, time.Second, func() bool { return client.currentConnection() != nil }, "connection recovery")
	client.Stop()
}

func TestJoinNormalizesAndDeduplicatesSymbols(t *testing.T) {
	client := newConnectionTestClient("http://127.0.0.1:1")
	client.Join(" AAPL ")
	client.Join("AAPL")
	client.Join("   ")
	client.JoinMany([]string{" MSFT", "MSFT ", ""})

	client.subscriptionsMu.Lock()
	defer client.subscriptionsMu.Unlock()
	if len(client.subscriptions) != 2 || !client.subscriptions["AAPL"] || !client.subscriptions["MSFT"] {
		t.Fatalf("unexpected normalized subscriptions: %#v", client.subscriptions)
	}
}

func TestDoBackoffRecoversPanicsAndHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	attempts := 0
	if !doBackoff(ctx, []time.Duration{time.Millisecond}, func() bool {
		attempts++
		if attempts < 3 {
			panic("transient failure")
		}
		return true
	}) {
		t.Fatal("backoff did not recover and retry")
	}
	if attempts != 3 {
		t.Fatalf("got %d attempts, want 3", attempts)
	}

	canceled, stop := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan bool, 1)
	go func() {
		done <- doBackoff(canceled, []time.Duration{time.Hour}, func() bool {
			close(started)
			return false
		})
	}()
	<-started
	stop()
	select {
	case success := <-done:
		if success {
			t.Fatal("canceled backoff reported success")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("cancellation did not interrupt backoff")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type trackingBody struct {
	closed atomic.Bool
}

func (body *trackingBody) Read(_ []byte) (int, error) { return 0, io.EOF }
func (body *trackingBody) Close() error {
	body.closed.Store(true)
	return nil
}

func TestAuthorizationClosesNonSuccessResponseBody(t *testing.T) {
	body := &trackingBody{}
	client := newConnectionTestClient("http://127.0.0.1:1")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Body:       body,
			Header:     make(http.Header),
		}, nil
	})}
	if client.trySetToken() {
		t.Fatal("authorization unexpectedly succeeded")
	}
	if !body.closed.Load() {
		t.Fatal("authorization response body was not closed")
	}
}

func TestStartCalledTwiceDoesNotReconnect(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	client.Start()
	if connections.Load() != 1 {
		t.Fatalf("Start connected %d times, want 1", connections.Load())
	}
	client.Stop()
}

func TestStopBeforeStartAndConcurrentStopsComplete(t *testing.T) {
	client := newConnectionTestClient("http://127.0.0.1:1")
	client.Stop()

	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client = newConnectionTestClient(server.URL)
	client.Start()
	done := make(chan struct{}, 2)
	for i := 0; i < 2; i++ {
		go func() {
			client.Stop()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("concurrent Stop did not complete")
		}
	}
}

func TestStartAfterStopRestartsAndRejoins(t *testing.T) {
	type joinMessage struct {
		connection int32
		message    string
	}
	var connections atomic.Int32
	joins := make(chan joinMessage, 4)
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			joins <- joinMessage{connection: connectionNumber, message: string(data)}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	client.Join("AAPL")
	first := <-joins
	if first.connection != 1 || first.message != "JAAPL" {
		t.Fatalf("first join = %#v", first)
	}
	client.Stop()

	client.Start()
	select {
	case second := <-joins:
		if second.connection != 2 || second.message != "JAAPL" {
			t.Fatalf("restart join = %#v", second)
		}
	case <-time.After(time.Second):
		t.Fatal("restart did not rejoin the existing subscription")
	}
	client.Stop()
}

func TestStartAfterStopDoesNotReplayQueuedData(t *testing.T) {
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	client.Stop()
	client.readChannel <- []byte{1, 2, 3}
	client.Start()
	if depth := len(client.readChannel); depth != 0 {
		t.Fatalf("restart retained %d stale network message(s)", depth)
	}
	client.Stop()
}

func TestInitialAuthorizationFailureRetriesUntilSuccess(t *testing.T) {
	var authAttempts atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, _ *http.Request) {
		if authAttempts.Add(1) < 3 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		authHandler(w, nil)
	})
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	if authAttempts.Load() != 3 {
		t.Fatalf("authorization attempts = %d, want 3", authAttempts.Load())
	}
	client.Stop()
}

func TestReconnectAuthorizationFailureRetriesUntilSuccess(t *testing.T) {
	var authAttempts atomic.Int32
	var connections atomic.Int32
	closeFirstConnection := make(chan struct{})
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, _ *http.Request) {
		attempt := authAttempts.Add(1)
		if attempt == 2 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		authHandler(w, nil)
	})
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if connectionNumber == 1 {
			<-closeFirstConnection
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	client.stateMu.Lock()
	client.tokenUpdateTime = time.Now().Add(-25 * time.Hour)
	client.stateMu.Unlock()
	close(closeFirstConnection)

	waitForCondition(t, time.Second, func() bool {
		return authAttempts.Load() >= 3 && connections.Load() >= 2 &&
			client.currentConnection() != nil
	}, "reconnect after authorization failure")
	client.Stop()
}

func TestStartWaitsForInProgressStopBeforeStartingNewSession(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.workerCount = 1
	firstWorkerStarted := make(chan struct{})
	releaseFirstWorker := make(chan struct{})
	var workerRuns atomic.Int32
	client.work = func(ctx context.Context) {
		if workerRuns.Add(1) == 1 {
			close(firstWorkerStarted)
			<-releaseFirstWorker
			return
		}
		<-ctx.Done()
	}
	client.Start()
	<-firstWorkerStarted

	stopDone := make(chan struct{})
	go func() {
		client.Stop()
		close(stopDone)
	}()
	waitForCondition(t, time.Second, func() bool {
		client.stateMu.RLock()
		defer client.stateMu.RUnlock()
		return client.lifecycle == clientStopping
	}, "client to begin stopping")

	startDone := make(chan struct{})
	go func() {
		client.Start()
		close(startDone)
	}()
	select {
	case <-startDone:
		t.Fatal("Start completed before the previous Stop")
	case <-time.After(20 * time.Millisecond):
	}
	if connections.Load() != 1 {
		t.Fatalf("new session connected before Stop completed; connections = %d", connections.Load())
	}

	close(releaseFirstWorker)
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("Stop did not complete after the worker exited")
	}
	select {
	case <-startDone:
	case <-time.After(time.Second):
		t.Fatal("Start did not create a new session after Stop completed")
	}
	if connections.Load() != 2 {
		t.Fatalf("connections = %d, want 2", connections.Load())
	}
	client.Stop()
}

func TestFirstConnectionClosingImmediatelyRecovers(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if connectionNumber == 1 {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	waitForCondition(t, time.Second, func() bool {
		return connections.Load() >= 2 && client.currentConnection() != nil
	}, "recovery after the first connection closed")
	client.Stop()
}

func TestTextMessageDoesNotReconnect(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte("warning")); err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	waitForCondition(t, time.Second, func() bool {
		return client.txtMsgCount.Load() == 1
	}, "text message")
	time.Sleep(25 * time.Millisecond)
	if connections.Load() != 1 {
		t.Fatalf("text message caused %d connections, want 1", connections.Load())
	}
	client.Stop()
}

func TestMultipleDisconnectReconnectCyclesReceiveData(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.BinaryMessage, []byte{0}); err != nil {
			return
		}
		if connectionNumber < 3 {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	waitForCondition(t, time.Second, func() bool {
		return connections.Load() >= 3 && client.dataMsgCount.Load() >= 3 &&
			client.currentConnection() != nil
	}, "multiple reconnect cycles and data")
	client.Stop()
}

func TestJoinWhileDisconnectedIsSentAfterReconnect(t *testing.T) {
	var connections atomic.Int32
	rejoined := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if connectionNumber == 1 {
			return
		}
		_, data, err := conn.ReadMessage()
		if err == nil {
			rejoined <- string(data)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.reconnectDelay = 50 * time.Millisecond
	client.Start()
	waitForCondition(t, time.Second, func() bool {
		return client.currentConnection() == nil
	}, "first connection to close")
	client.Join("MSFT")
	select {
	case message := <-rejoined:
		if message != "JMSFT" {
			t.Fatalf("rejoin message = %q, want JMSFT", message)
		}
	case <-time.After(time.Second):
		t.Fatal("join made while disconnected was not sent after reconnect")
	}
	client.Stop()
}

func TestReconnectHandshakeTimeoutRetriesUntilSuccess(t *testing.T) {
	var attempts atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		if attempt == 2 {
			<-r.Context().Done()
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if attempt == 1 {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = 15 * time.Millisecond
	client.Start()
	waitForCondition(t, time.Second, func() bool {
		return attempts.Load() >= 3 && client.currentConnection() != nil
	}, "reconnect after a hung handshake")
	client.Stop()
}

func TestStopDuringHungReconnectHandshakeCompletes(t *testing.T) {
	hungHandshake := make(chan struct{})
	var attempts atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		select {
		case <-hungHandshake:
		default:
			close(hungHandshake)
		}
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.connectTimeout = time.Second
	client.Start()
	select {
	case <-hungHandshake:
	case <-time.After(time.Second):
		t.Fatal("reconnect did not enter the hung handshake")
	}
	stopDone := make(chan struct{})
	go func() {
		client.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Stop did not cancel the hung reconnect handshake")
	}
}

func TestReconnectAfterRejoinWriteFailure(t *testing.T) {
	var connections atomic.Int32
	firstJoin := make(chan struct{})
	finalJoin := make(chan string, 1)
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if connectionNumber == 2 {
			if tcpConn, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
				_ = tcpConn.SetLinger(0)
			}
			_ = conn.Close()
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if connectionNumber == 1 {
			close(firstJoin)
			return
		}
		finalJoin <- string(data)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.Start()
	client.Join("AAPL")
	select {
	case <-firstJoin:
	case <-time.After(time.Second):
		t.Fatal("initial join was not received")
	}
	select {
	case message := <-finalJoin:
		if message != "JAAPL" {
			t.Fatalf("final rejoin = %q, want JAAPL", message)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not recover after the rejoin write failed")
	}
	if connections.Load() < 3 {
		t.Fatalf("connections = %d, want at least 3", connections.Load())
	}
	client.Stop()
}

func TestStopDuringReconnectBackoffCompletes(t *testing.T) {
	backoffStarted := make(chan struct{})
	var attempts atomic.Int32
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		select {
		case <-backoffStarted:
		default:
			close(backoffStarted)
		}
		http.Error(w, "try again", http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client := newConnectionTestClient(server.URL)
	client.backoffs = []time.Duration{time.Hour}
	client.Start()
	select {
	case <-backoffStarted:
	case <-time.After(time.Second):
		t.Fatal("reconnect did not enter backoff")
	}
	stopDone := make(chan struct{})
	go func() {
		client.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Stop did not cancel reconnect backoff")
	}
}

func TestOptionsClientReconnectsWithSameProtocolHeaders(t *testing.T) {
	var connections atomic.Int32
	headerFailures := make(chan string, 2)
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/socket/websocket", func(w http.ResponseWriter, r *http.Request) {
		connectionNumber := connections.Add(1)
		if got := r.Header.Get("UseNewOptionsFormat"); got != "v2" {
			headerFailures <- got
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if connectionNumber == 1 {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	config := Config{
		ApiKey:    "test-key",
		Provider:  MANUAL,
		IPAddress: strings.TrimPrefix(server.URL, "http://"),
	}
	client := NewOptionsClient(config, nil, nil, nil, nil)
	client.connectTimeout = 50 * time.Millisecond
	client.reconnectDelay = time.Millisecond
	client.backoffs = []time.Duration{time.Millisecond}
	client.heartbeat = time.Hour
	client.Start()
	waitForCondition(t, time.Second, func() bool {
		return connections.Load() >= 2 && client.currentConnection() != nil
	}, "options reconnect")
	select {
	case got := <-headerFailures:
		t.Fatalf("UseNewOptionsFormat header = %q, want v2", got)
	default:
	}
	client.Stop()
}
