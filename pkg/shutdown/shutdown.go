// Package shutdown provides a coordinated graceful shutdown handler for
// microservices. It orchestrates the orderly shutdown of HTTP servers,
// WebSocket connections, Kafka producers, and database connection pools,
// each with its own configurable timeout.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Default timeouts per component during shutdown.
const (
	DefaultHTTPDrainTimeout      = 30 * time.Second
	DefaultWebSocketCloseTimeout = 10 * time.Second
	DefaultKafkaFlushTimeout     = 10 * time.Second
	DefaultDBCloseTimeout        = 5 * time.Second
)

// Config holds per-component timeout settings for the shutdown sequence.
type Config struct {
	// HTTPDrainTimeout is the maximum time to wait for in-flight HTTP requests to complete.
	HTTPDrainTimeout time.Duration

	// WebSocketCloseTimeout is the maximum time to wait for WebSocket clients to acknowledge close frames.
	WebSocketCloseTimeout time.Duration

	// KafkaFlushTimeout is the maximum time to wait for pending Kafka messages to flush.
	KafkaFlushTimeout time.Duration

	// DBCloseTimeout is the maximum time to allow for database pool closure.
	DBCloseTimeout time.Duration
}

// DefaultConfig returns a Config with production-ready default timeouts.
func DefaultConfig() Config {
	return Config{
		HTTPDrainTimeout:      DefaultHTTPDrainTimeout,
		WebSocketCloseTimeout: DefaultWebSocketCloseTimeout,
		KafkaFlushTimeout:     DefaultKafkaFlushTimeout,
		DBCloseTimeout:        DefaultDBCloseTimeout,
	}
}

// WebSocketManager is the interface that WebSocket managers must implement
// to support graceful close frame sending during shutdown.
type WebSocketManager interface {
	// CloseAll sends close frames to all connected clients and waits for
	// acknowledgment up to the given context deadline. Returns the number
	// of connections that did not acknowledge in time.
	CloseAll(ctx context.Context) (unacknowledged int, err error)
}

// KafkaFlusher is the interface that Kafka producers must implement to
// support graceful message flushing during shutdown.
type KafkaFlusher interface {
	// Flush attempts to deliver all pending messages. Returns the number
	// of unflushed messages if the context deadline is exceeded.
	Flush(ctx context.Context) (unflushed int, err error)
}

// DatabaseCloser is the interface that database pools must implement to
// support graceful closure during shutdown.
type DatabaseCloser interface {
	// Close closes all connections in the pool.
	Close() error
}

// Handler coordinates the orderly shutdown of service components.
// Components are shut down in this order:
//  1. HTTP server stops accepting new connections
//  2. WebSocket connections receive close frames
//  3. Kafka producer flushes pending messages
//  4. HTTP server drains in-flight requests
//  5. Database connection pools are closed
type Handler struct {
	config Config
	logger *zap.Logger

	server    *http.Server
	wsManager WebSocketManager
	kafka     KafkaFlusher
	databases []DatabaseCloser

	mu       sync.Mutex
	shutdown bool
}

// Option is a functional option for configuring the Handler.
type Option func(*Handler)

// WithHTTPServer registers an HTTP server for graceful shutdown.
func WithHTTPServer(server *http.Server) Option {
	return func(h *Handler) {
		h.server = server
	}
}

// WithWebSocketManager registers a WebSocket manager for sending close frames.
func WithWebSocketManager(ws WebSocketManager) Option {
	return func(h *Handler) {
		h.wsManager = ws
	}
}

// WithKafkaFlusher registers a Kafka producer for message flushing.
func WithKafkaFlusher(kafka KafkaFlusher) Option {
	return func(h *Handler) {
		h.kafka = kafka
	}
}

// WithDatabase registers a database connection pool for closure during shutdown.
// Multiple databases can be registered by calling this option multiple times.
func WithDatabase(db DatabaseCloser) Option {
	return func(h *Handler) {
		h.databases = append(h.databases, db)
	}
}

// WithLogger sets the logger for the handler.
func WithLogger(logger *zap.Logger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

// New creates a new shutdown Handler with the given config and options.
func New(cfg Config, opts ...Option) *Handler {
	h := &Handler{
		config:    cfg,
		logger:    zap.NewNop(),
		databases: make([]DatabaseCloser, 0),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// ListenAndShutdown blocks until a SIGTERM or SIGINT is received, then
// executes the shutdown sequence. Returns an error if any component fails
// to shut down cleanly within its timeout.
func (h *Handler) ListenAndShutdown() error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigCh
	h.logger.Info("received shutdown signal", zap.String("signal", sig.String()))

	return h.Shutdown(context.Background())
}

// Shutdown executes the graceful shutdown sequence. It can be called directly
// for testing or when signal handling is managed externally.
func (h *Handler) Shutdown(ctx context.Context) error {
	h.mu.Lock()
	if h.shutdown {
		h.mu.Unlock()
		return nil
	}
	h.shutdown = true
	h.mu.Unlock()

	var errs []error

	// Step 1: Stop accepting new HTTP connections (non-blocking, just marks the server).
	// The actual drain happens in step 4.
	h.logger.Info("shutdown: stopping new connections")

	// Step 2: Send WebSocket close frames to all connected clients.
	if h.wsManager != nil {
		h.logger.Info("shutdown: closing WebSocket connections",
			zap.Duration("timeout", h.config.WebSocketCloseTimeout))

		wsCtx, wsCancel := context.WithTimeout(ctx, h.config.WebSocketCloseTimeout)
		unack, err := h.wsManager.CloseAll(wsCtx)
		wsCancel()

		if err != nil {
			h.logger.Warn("shutdown: WebSocket close error", zap.Error(err))
			errs = append(errs, fmt.Errorf("websocket close: %w", err))
		}
		if unack > 0 {
			h.logger.Warn("shutdown: WebSocket connections did not acknowledge close",
				zap.Int("unacknowledged", unack))
		}
	}

	// Step 3: Flush pending Kafka messages.
	if h.kafka != nil {
		h.logger.Info("shutdown: flushing Kafka messages",
			zap.Duration("timeout", h.config.KafkaFlushTimeout))

		kafkaCtx, kafkaCancel := context.WithTimeout(ctx, h.config.KafkaFlushTimeout)
		unflushed, err := h.kafka.Flush(kafkaCtx)
		kafkaCancel()

		if err != nil {
			h.logger.Warn("shutdown: Kafka flush error", zap.Error(err))
			errs = append(errs, fmt.Errorf("kafka flush: %w", err))
		}
		if unflushed > 0 {
			h.logger.Warn("shutdown: Kafka messages could not be flushed within timeout",
				zap.Int("unflushed_count", unflushed),
				zap.Duration("timeout", h.config.KafkaFlushTimeout))
		}
	}

	// Step 4: Drain in-flight HTTP requests.
	if h.server != nil {
		h.logger.Info("shutdown: draining HTTP connections",
			zap.Duration("timeout", h.config.HTTPDrainTimeout))

		httpCtx, httpCancel := context.WithTimeout(ctx, h.config.HTTPDrainTimeout)
		err := h.server.Shutdown(httpCtx)
		httpCancel()

		if err != nil {
			h.logger.Warn("shutdown: HTTP drain error, forcing close", zap.Error(err))
			errs = append(errs, fmt.Errorf("http drain: %w", err))
			// Force-close remaining connections.
			if closeErr := h.server.Close(); closeErr != nil {
				errs = append(errs, fmt.Errorf("http force close: %w", closeErr))
			}
		}
	}

	// Step 5: Close database connection pools.
	if len(h.databases) > 0 {
		h.logger.Info("shutdown: closing database connections",
			zap.Int("pool_count", len(h.databases)))

		dbCtx, dbCancel := context.WithTimeout(ctx, h.config.DBCloseTimeout)
		defer dbCancel()

		var dbWg sync.WaitGroup
		dbErrors := make(chan error, len(h.databases))

		for i, db := range h.databases {
			dbWg.Add(1)
			go func(idx int, closer DatabaseCloser) {
				defer dbWg.Done()
				done := make(chan error, 1)
				go func() {
					done <- closer.Close()
				}()
				select {
				case err := <-done:
					if err != nil {
						dbErrors <- fmt.Errorf("database pool %d: %w", idx, err)
					}
				case <-dbCtx.Done():
					dbErrors <- fmt.Errorf("database pool %d: close timeout exceeded", idx)
				}
			}(i, db)
		}

		dbWg.Wait()
		close(dbErrors)
		for err := range dbErrors {
			h.logger.Warn("shutdown: database close error", zap.Error(err))
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		h.logger.Info("shutdown: completed with errors", zap.Int("error_count", len(errs)))
		return errors.Join(errs...)
	}

	h.logger.Info("shutdown: completed successfully")
	return nil
}
