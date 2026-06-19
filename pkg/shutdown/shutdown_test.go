package shutdown

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// --- Mock implementations ---

type mockWebSocketManager struct {
	closeAllFn func(ctx context.Context) (int, error)
	called     atomic.Bool
}

func (m *mockWebSocketManager) CloseAll(ctx context.Context) (int, error) {
	m.called.Store(true)
	if m.closeAllFn != nil {
		return m.closeAllFn(ctx)
	}
	return 0, nil
}

type mockKafkaFlusher struct {
	flushFn func(ctx context.Context) (int, error)
	called  atomic.Bool
}

func (m *mockKafkaFlusher) Flush(ctx context.Context) (int, error) {
	m.called.Store(true)
	if m.flushFn != nil {
		return m.flushFn(ctx)
	}
	return 0, nil
}

type mockDBCloser struct {
	closeFn func() error
	called  atomic.Bool
}

func (m *mockDBCloser) Close() error {
	m.called.Store(true)
	if m.closeFn != nil {
		return m.closeFn()
	}
	return nil
}

// --- Tests ---

func TestShutdown_AllComponentsCleanSuccess(t *testing.T) {
	ws := &mockWebSocketManager{}
	kafka := &mockKafkaFlusher{}
	db1 := &mockDBCloser{}
	db2 := &mockDBCloser{}

	// Create a test HTTP server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go server.Serve(listener)

	handler := New(
		DefaultConfig(),
		WithHTTPServer(server),
		WithWebSocketManager(ws),
		WithKafkaFlusher(kafka),
		WithDatabase(db1),
		WithDatabase(db2),
		WithLogger(zaptest.NewLogger(t)),
	)

	err = handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected clean shutdown, got error: %v", err)
	}

	if !ws.called.Load() {
		t.Error("expected WebSocket CloseAll to be called")
	}
	if !kafka.called.Load() {
		t.Error("expected Kafka Flush to be called")
	}
	if !db1.called.Load() {
		t.Error("expected database 1 Close to be called")
	}
	if !db2.called.Load() {
		t.Error("expected database 2 Close to be called")
	}
}

func TestShutdown_HTTPDrainWithInFlightRequests(t *testing.T) {
	reqStarted := make(chan struct{})
	reqRelease := make(chan struct{})

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqStarted)
		<-reqRelease
		w.WriteHeader(http.StatusOK)
	})}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)

	// Start an in-flight request
	go func() {
		http.Get(fmt.Sprintf("http://%s/test", listener.Addr().String()))
	}()

	// Wait for request to reach the handler
	<-reqStarted

	handler := New(
		Config{
			HTTPDrainTimeout:      2 * time.Second,
			WebSocketCloseTimeout: 1 * time.Second,
			KafkaFlushTimeout:     1 * time.Second,
			DBCloseTimeout:        1 * time.Second,
		},
		WithHTTPServer(server),
		WithLogger(zaptest.NewLogger(t)),
	)

	// Shutdown in background, let the request complete before timeout
	var shutdownErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		shutdownErr = handler.Shutdown(context.Background())
	}()

	// Allow the in-flight request to complete
	time.Sleep(100 * time.Millisecond)
	close(reqRelease)

	wg.Wait()
	if shutdownErr != nil {
		t.Errorf("expected clean shutdown with drained request, got: %v", shutdownErr)
	}
}

func TestShutdown_HTTPDrainTimeout(t *testing.T) {
	reqStarted := make(chan struct{})

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(reqStarted)
		// Simulate a very slow request that exceeds drain timeout
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	})}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)

	// Start an in-flight request
	go func() {
		http.Get(fmt.Sprintf("http://%s/test", listener.Addr().String()))
	}()
	<-reqStarted

	handler := New(
		Config{
			HTTPDrainTimeout:      200 * time.Millisecond,
			WebSocketCloseTimeout: 100 * time.Millisecond,
			KafkaFlushTimeout:     100 * time.Millisecond,
			DBCloseTimeout:        100 * time.Millisecond,
		},
		WithHTTPServer(server),
		WithLogger(zaptest.NewLogger(t)),
	)

	err = handler.Shutdown(context.Background())
	if err == nil {
		t.Error("expected timeout error during HTTP drain")
	}
}

func TestShutdown_WebSocketCloseTimeout(t *testing.T) {
	ws := &mockWebSocketManager{
		closeAllFn: func(ctx context.Context) (int, error) {
			// Simulate slow close that exceeds the timeout
			select {
			case <-ctx.Done():
				return 5, ctx.Err()
			case <-time.After(5 * time.Second):
				return 0, nil
			}
		},
	}

	handler := New(
		Config{
			HTTPDrainTimeout:      1 * time.Second,
			WebSocketCloseTimeout: 200 * time.Millisecond,
			KafkaFlushTimeout:     1 * time.Second,
			DBCloseTimeout:        1 * time.Second,
		},
		WithWebSocketManager(ws),
		WithLogger(zaptest.NewLogger(t)),
	)

	err := handler.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error from WebSocket close timeout")
	}
}

func TestShutdown_KafkaFlushReportsUnflushed(t *testing.T) {
	kafka := &mockKafkaFlusher{
		flushFn: func(ctx context.Context) (int, error) {
			return 3, nil
		},
	}

	handler := New(
		DefaultConfig(),
		WithKafkaFlusher(kafka),
		WithLogger(zaptest.NewLogger(t)),
	)

	// Shutdown should succeed (unflushed is logged but not an error)
	err := handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected no error (unflushed is a warning), got: %v", err)
	}
}

func TestShutdown_KafkaFlushTimeout(t *testing.T) {
	kafka := &mockKafkaFlusher{
		flushFn: func(ctx context.Context) (int, error) {
			select {
			case <-ctx.Done():
				return 7, ctx.Err()
			case <-time.After(5 * time.Second):
				return 0, nil
			}
		},
	}

	handler := New(
		Config{
			HTTPDrainTimeout:      1 * time.Second,
			WebSocketCloseTimeout: 1 * time.Second,
			KafkaFlushTimeout:     200 * time.Millisecond,
			DBCloseTimeout:        1 * time.Second,
		},
		WithKafkaFlusher(kafka),
		WithLogger(zaptest.NewLogger(t)),
	)

	err := handler.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error from Kafka flush timeout")
	}
}

func TestShutdown_DatabaseCloseError(t *testing.T) {
	db := &mockDBCloser{
		closeFn: func() error {
			return errors.New("connection pool close failed")
		},
	}

	handler := New(
		DefaultConfig(),
		WithDatabase(db),
		WithLogger(zaptest.NewLogger(t)),
	)

	err := handler.Shutdown(context.Background())
	if err == nil {
		t.Error("expected error from database close failure")
	}
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just verify we get a meaningful error
		if err.Error() == "" {
			t.Error("expected non-empty error message")
		}
	}
}

func TestShutdown_MultipleDatabasePools(t *testing.T) {
	var callOrder []string
	var mu sync.Mutex

	db1 := &mockDBCloser{
		closeFn: func() error {
			mu.Lock()
			callOrder = append(callOrder, "db1")
			mu.Unlock()
			return nil
		},
	}
	db2 := &mockDBCloser{
		closeFn: func() error {
			mu.Lock()
			callOrder = append(callOrder, "db2")
			mu.Unlock()
			return nil
		},
	}

	handler := New(
		DefaultConfig(),
		WithDatabase(db1),
		WithDatabase(db2),
		WithLogger(zaptest.NewLogger(t)),
	)

	err := handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected clean shutdown, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(callOrder) != 2 {
		t.Errorf("expected both databases closed, got %d", len(callOrder))
	}
}

func TestShutdown_IdempotentCalls(t *testing.T) {
	var closeCount atomic.Int32
	db := &mockDBCloser{
		closeFn: func() error {
			closeCount.Add(1)
			return nil
		},
	}

	handler := New(
		DefaultConfig(),
		WithDatabase(db),
		WithLogger(zaptest.NewLogger(t)),
	)

	// Call shutdown multiple times
	_ = handler.Shutdown(context.Background())
	_ = handler.Shutdown(context.Background())
	_ = handler.Shutdown(context.Background())

	if closeCount.Load() != 1 {
		t.Errorf("expected database Close called exactly once, got %d", closeCount.Load())
	}
}

func TestShutdown_NilComponents(t *testing.T) {
	// Handler with no components should shut down cleanly
	handler := New(DefaultConfig(), WithLogger(zaptest.NewLogger(t)))

	err := handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("expected clean shutdown with no components, got: %v", err)
	}
}

func TestShutdown_ExecutionOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	ws := &mockWebSocketManager{
		closeAllFn: func(ctx context.Context) (int, error) {
			mu.Lock()
			order = append(order, "ws")
			mu.Unlock()
			return 0, nil
		},
	}

	kafka := &mockKafkaFlusher{
		flushFn: func(ctx context.Context) (int, error) {
			mu.Lock()
			order = append(order, "kafka")
			mu.Unlock()
			return 0, nil
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go server.Serve(listener)

	db := &mockDBCloser{
		closeFn: func() error {
			mu.Lock()
			order = append(order, "db")
			mu.Unlock()
			return nil
		},
	}

	handler := New(
		DefaultConfig(),
		WithHTTPServer(server),
		WithWebSocketManager(ws),
		WithKafkaFlusher(kafka),
		WithDatabase(db),
		WithLogger(zaptest.NewLogger(t)),
	)

	err = handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify order: ws -> kafka -> (http drain is implicit) -> db
	if len(order) < 3 {
		t.Fatalf("expected at least 3 steps, got %d: %v", len(order), order)
	}
	if order[0] != "ws" {
		t.Errorf("expected ws first, got %s", order[0])
	}
	if order[1] != "kafka" {
		t.Errorf("expected kafka second, got %s", order[1])
	}
	if order[2] != "db" {
		t.Errorf("expected db third, got %s", order[2])
	}
}

func TestCloserFunc(t *testing.T) {
	called := false
	closer := CloserFunc(func() error {
		called = true
		return nil
	})

	err := closer.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected CloserFunc to be called")
	}
}

func TestNoOpCloser(t *testing.T) {
	closer := NoOpCloser()
	err := closer.Close()
	if err != nil {
		t.Errorf("unexpected error from NoOpCloser: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.HTTPDrainTimeout != 30*time.Second {
		t.Errorf("expected 30s HTTP drain, got %v", cfg.HTTPDrainTimeout)
	}
	if cfg.WebSocketCloseTimeout != 10*time.Second {
		t.Errorf("expected 10s WebSocket close, got %v", cfg.WebSocketCloseTimeout)
	}
	if cfg.KafkaFlushTimeout != 10*time.Second {
		t.Errorf("expected 10s Kafka flush, got %v", cfg.KafkaFlushTimeout)
	}
	if cfg.DBCloseTimeout != 5*time.Second {
		t.Errorf("expected 5s DB close, got %v", cfg.DBCloseTimeout)
	}
}

func TestShutdown_WithNopLogger(t *testing.T) {
	// Verify the handler works with a nop logger (default)
	handler := New(DefaultConfig())
	err := handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShutdown_WithCustomLogger(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handler := New(DefaultConfig(), WithLogger(logger))
	err := handler.Shutdown(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
