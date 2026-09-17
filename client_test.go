package intrinio

import (
	"context"
	"fmt"
	"io"
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
