// Package circuitbreaker provides a production-grade circuit breaker with Prometheus metrics,
// structured logging, configurable thresholds, cached/fallback responses, and proper state machine
// transitions (closed → open → half-open → closed/open).
package circuitbreaker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Errors returned by the circuit breaker.
var (
	ErrCircuitOpen    = errors.New("circuitbreaker: circuit is open")
	ErrCircuitReject  = errors.New("circuitbreaker: request rejected in half-open state")
	ErrInvalidConfig  = errors.New("circuitbreaker: invalid configuration")
)

// State represents the circuit breaker state.
type State int

const (
	StateClosed   State = 0
	StateOpen     State = 1
	StateHalfOpen State = 2
)

// String returns a human-readable state name.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config configures the circuit breaker behavior.
type Config struct {
	// FailureThreshold is the number of consecutive failures before opening the circuit.
	// Must be between 1 and 100. Default: 5.
	FailureThreshold int

	// ResetTimeout is the duration to wait in open state before transitioning to half-open.
	// Must be between 1s and 300s. Default: 30s.
	ResetTimeout time.Duration

	// SuccessThreshold is the number of consecutive successes in half-open state
	// before transitioning back to closed.
	// Must be between 1 and 10. Default: 1.
	SuccessThreshold int

	// CacheTTL is the maximum age of a cached response to return when the circuit is open.
	// Default: 60s.
	CacheTTL time.Duration

	// ServiceName is the name of the service protected by this circuit breaker.
	// Used for metrics labels and logging.
	ServiceName string
}

// DefaultConfig returns a Config with production defaults.
func DefaultConfig(serviceName string) Config {
	return Config{
		FailureThreshold: 5,
		ResetTimeout:     30 * time.Second,
		SuccessThreshold: 1,
		CacheTTL:         60 * time.Second,
		ServiceName:      serviceName,
	}
}

// validate checks configuration parameters and applies defaults.
func (c *Config) validate() error {
	if c.ServiceName == "" {
		return fmt.Errorf("%w: service name is required", ErrInvalidConfig)
	}
	if c.FailureThreshold < 1 || c.FailureThreshold > 100 {
		return fmt.Errorf("%w: failure threshold must be between 1 and 100, got %d", ErrInvalidConfig, c.FailureThreshold)
	}
	if c.ResetTimeout < time.Second || c.ResetTimeout > 300*time.Second {
		return fmt.Errorf("%w: reset timeout must be between 1s and 300s, got %s", ErrInvalidConfig, c.ResetTimeout)
	}
	if c.SuccessThreshold < 1 || c.SuccessThreshold > 10 {
		return fmt.Errorf("%w: success threshold must be between 1 and 10, got %d", ErrInvalidConfig, c.SuccessThreshold)
	}
	if c.CacheTTL <= 0 {
		c.CacheTTL = 60 * time.Second
	}
	return nil
}

// Metrics holds the current state metrics for the circuit breaker.
type Metrics struct {
	State            State
	ConsecutiveFails int
	SecondsSinceFail float64
	TotalRequests    int64
	TotalFailures    int64
}

// CircuitBreaker defines the interface for a circuit breaker.
type CircuitBreaker interface {
	// Execute runs the given function through the circuit breaker.
	// If the circuit is open, it returns a cached response or a degradation response.
	// If the circuit is half-open, only one probe request is allowed at a time.
	Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error)

	// GetState returns the current state of the circuit breaker.
	GetState() State

	// GetMetrics returns the current metrics of the circuit breaker.
	GetMetrics() Metrics
}

// cachedResponse stores a recent successful response.
type cachedResponse struct {
	value     interface{}
	timestamp time.Time
}

// circuitBreaker implements CircuitBreaker with full state machine,
// Prometheus metrics, structured logging, and cached/fallback responses.
type circuitBreaker struct {
	mu sync.Mutex

	config Config
	logger *zap.Logger

	state             State
	consecutiveFails  int
	consecutiveSucc   int
	lastFailureTime   time.Time
	lastStateChange   time.Time
	totalRequests     int64
	totalFailures     int64
	probeInFlight     bool

	cache *cachedResponse

	// Prometheus metrics
	stateGauge          prometheus.Gauge
	consecutiveFailGauge prometheus.Gauge
	secondsSinceFailGauge prometheus.Gauge

	// Clock function for testability
	now func() time.Time
}

// Option is a functional option for configuring the circuit breaker.
type Option func(*circuitBreaker)

// WithLogger sets a custom logger.
func WithLogger(logger *zap.Logger) Option {
	return func(cb *circuitBreaker) {
		cb.logger = logger
	}
}

// WithPrometheusRegisterer sets a custom Prometheus registerer.
func WithPrometheusRegisterer(reg prometheus.Registerer) Option {
	return func(cb *circuitBreaker) {
		cb.registerMetrics(reg)
	}
}

// WithClock sets a custom clock function (for testing).
func WithClock(now func() time.Time) Option {
	return func(cb *circuitBreaker) {
		cb.now = now
	}
}

// New creates a new CircuitBreaker with the given configuration.
func New(cfg Config, opts ...Option) (CircuitBreaker, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cb := &circuitBreaker{
		config:        cfg,
		state:         StateClosed,
		lastStateChange: time.Now(),
		now:           time.Now,
	}

	// Apply options
	for _, opt := range opts {
		opt(cb)
	}

	// Set default logger if not provided
	if cb.logger == nil {
		cb.logger, _ = zap.NewProduction()
	}

	// Register default Prometheus metrics if not already registered via option
	if cb.stateGauge == nil {
		cb.registerMetrics(prometheus.DefaultRegisterer)
	}

	// Set initial metric values
	cb.stateGauge.Set(float64(StateClosed))
	cb.consecutiveFailGauge.Set(0)
	cb.secondsSinceFailGauge.Set(0)

	return cb, nil
}

// registerMetrics registers Prometheus metrics with the given registerer.
func (cb *circuitBreaker) registerMetrics(reg prometheus.Registerer) {
	cb.stateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "circuit_breaker_state",
		Help:        "Current circuit breaker state (0=closed, 1=open, 2=half-open)",
		ConstLabels: prometheus.Labels{"service": cb.config.ServiceName},
	})

	cb.consecutiveFailGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "circuit_breaker_consecutive_failures",
		Help:        "Number of consecutive failures",
		ConstLabels: prometheus.Labels{"service": cb.config.ServiceName},
	})

	cb.secondsSinceFailGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "circuit_breaker_seconds_since_failure",
		Help:        "Seconds since last failure",
		ConstLabels: prometheus.Labels{"service": cb.config.ServiceName},
	})

	// Register metrics, ignoring already registered errors for reuse
	reg.Register(cb.stateGauge)
	reg.Register(cb.consecutiveFailGauge)
	reg.Register(cb.secondsSinceFailGauge)
}

// Execute runs the given function through the circuit breaker state machine.
func (cb *circuitBreaker) Execute(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	cb.totalRequests++
	now := cb.now()

	switch cb.state {
	case StateClosed:
		cb.mu.Unlock()
		return cb.executeClosed(ctx, fn)

	case StateOpen:
		// Check if reset timeout has elapsed → transition to half-open
		if now.Sub(cb.lastStateChange) >= cb.config.ResetTimeout {
			cb.transition(StateHalfOpen)
			// Allow this request as the probe
			cb.probeInFlight = true
			cb.mu.Unlock()
			return cb.executeProbe(ctx, fn)
		}
		// Circuit is open — return cached or fallback
		cached := cb.getCachedResponse(now)
		cb.mu.Unlock()
		if cached != nil {
			return cached, nil
		}
		return cb.gracefulDegradation(), ErrCircuitOpen

	case StateHalfOpen:
		// Only allow one probe at a time
		if cb.probeInFlight {
			// Reject — another probe is in progress
			cached := cb.getCachedResponse(now)
			cb.mu.Unlock()
			if cached != nil {
				return cached, nil
			}
			return cb.gracefulDegradation(), ErrCircuitReject
		}
		// Allow this request as the probe
		cb.probeInFlight = true
		cb.mu.Unlock()
		return cb.executeProbe(ctx, fn)

	default:
		cb.mu.Unlock()
		return nil, fmt.Errorf("circuitbreaker: unknown state %d", cb.state)
	}
}

// executeClosed runs the function in closed state. On failure, increments counter.
func (cb *circuitBreaker) executeClosed(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	result, err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.isFailure(err) {
		cb.consecutiveFails++
		cb.totalFailures++
		cb.lastFailureTime = cb.now()
		cb.consecutiveFailGauge.Set(float64(cb.consecutiveFails))
		cb.secondsSinceFailGauge.Set(0)

		if cb.consecutiveFails >= cb.config.FailureThreshold {
			cb.transition(StateOpen)
		}
		return result, err
	}

	// Success in closed state: reset failure counter, cache the result
	cb.consecutiveFails = 0
	cb.consecutiveFailGauge.Set(0)
	cb.updateSecondsSinceFail()
	if result != nil {
		cb.cache = &cachedResponse{value: result, timestamp: cb.now()}
	}
	return result, nil
}

// executeProbe runs the function as a probe in half-open state.
func (cb *circuitBreaker) executeProbe(ctx context.Context, fn func() (interface{}, error)) (interface{}, error) {
	result, err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.probeInFlight = false

	if cb.isFailure(err) {
		// Probe failed: back to open
		cb.consecutiveFails++
		cb.totalFailures++
		cb.consecutiveSucc = 0
		cb.lastFailureTime = cb.now()
		cb.consecutiveFailGauge.Set(float64(cb.consecutiveFails))
		cb.secondsSinceFailGauge.Set(0)
		cb.transition(StateOpen)
		return result, err
	}

	// Probe succeeded
	cb.consecutiveSucc++
	if cb.consecutiveSucc >= cb.config.SuccessThreshold {
		// Enough consecutive successes: close the circuit
		cb.consecutiveFails = 0
		cb.consecutiveSucc = 0
		cb.consecutiveFailGauge.Set(0)
		cb.updateSecondsSinceFail()
		cb.transition(StateClosed)
	}
	if result != nil {
		cb.cache = &cachedResponse{value: result, timestamp: cb.now()}
	}
	return result, nil
}

// transition changes the circuit state and logs the transition.
func (cb *circuitBreaker) transition(newState State) {
	prevState := cb.state
	if prevState == newState {
		return
	}

	cb.state = newState
	cb.lastStateChange = cb.now()

	// Reset probe flag on state transitions
	if newState == StateOpen {
		cb.probeInFlight = false
		cb.consecutiveSucc = 0
	}

	// Update Prometheus state gauge
	cb.stateGauge.Set(float64(newState))

	// Emit structured WARN log
	cb.logger.Warn("circuit breaker state transition",
		zap.String("service", cb.config.ServiceName),
		zap.String("previous_state", prevState.String()),
		zap.String("new_state", newState.String()),
		zap.Int("failure_count", cb.consecutiveFails),
		zap.Time("timestamp", cb.now()),
	)
}

// isFailure determines if an error should be counted as a circuit breaker failure.
// Counts: HTTP 5xx, connection timeout, connection refused.
func (cb *circuitBreaker) isFailure(err error) bool {
	if err == nil {
		return false
	}

	// Check for HTTP status error
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 500
	}

	// Check for connection timeout
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check for connection refused
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			if sysErr.Syscall == "connectex" || sysErr.Syscall == "connect" {
				return true
			}
		}
		return true
	}

	// Check for context deadline exceeded (timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// getCachedResponse returns the cached response if it's within CacheTTL.
func (cb *circuitBreaker) getCachedResponse(now time.Time) interface{} {
	if cb.cache == nil {
		return nil
	}
	if now.Sub(cb.cache.timestamp) > cb.config.CacheTTL {
		return nil
	}
	return cb.cache.value
}

// gracefulDegradation returns a default response when the circuit is open and no cache is available.
func (cb *circuitBreaker) gracefulDegradation() interface{} {
	return &DegradationResponse{
		ServiceName: cb.config.ServiceName,
		Message:     "service temporarily unavailable",
		StatusCode:  http.StatusServiceUnavailable,
	}
}

// updateSecondsSinceFail updates the Prometheus gauge for seconds since last failure.
func (cb *circuitBreaker) updateSecondsSinceFail() {
	if cb.lastFailureTime.IsZero() {
		cb.secondsSinceFailGauge.Set(0)
		return
	}
	cb.secondsSinceFailGauge.Set(cb.now().Sub(cb.lastFailureTime).Seconds())
}

// GetState returns the current state.
func (cb *circuitBreaker) GetState() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := cb.now()
	// Check if open state should transition to half-open (for external state queries)
	if cb.state == StateOpen && now.Sub(cb.lastStateChange) >= cb.config.ResetTimeout {
		cb.transition(StateHalfOpen)
	}
	return cb.state
}

// GetMetrics returns current metrics snapshot.
func (cb *circuitBreaker) GetMetrics() Metrics {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	var secondsSinceFail float64
	if !cb.lastFailureTime.IsZero() {
		secondsSinceFail = cb.now().Sub(cb.lastFailureTime).Seconds()
	}

	return Metrics{
		State:            cb.state,
		ConsecutiveFails: cb.consecutiveFails,
		SecondsSinceFail: secondsSinceFail,
		TotalRequests:    cb.totalRequests,
		TotalFailures:    cb.totalFailures,
	}
}

// HTTPError represents an HTTP error with a status code.
// Used to signal HTTP 5xx errors to the circuit breaker.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// NewHTTPError creates a new HTTPError.
func NewHTTPError(statusCode int, message string) *HTTPError {
	return &HTTPError{StatusCode: statusCode, Message: message}
}

// DegradationResponse is returned when the circuit is open and no cached response is available.
type DegradationResponse struct {
	ServiceName string `json:"service_name"`
	Message     string `json:"message"`
	StatusCode  int    `json:"status_code"`
}
