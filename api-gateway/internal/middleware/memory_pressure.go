package middleware

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

const (
	// Default pod memory limit: 512 MB if not configured.
	defaultPodMemoryLimitBytes uint64 = 512 * 1024 * 1024

	// highWatermark is the threshold (85%) at which we start rejecting requests.
	highWatermark = 0.85

	// lowWatermark is the threshold (80%) at which we resume accepting requests.
	lowWatermark = 0.80

	// defaultRetryAfterSeconds is the Retry-After value sent in 503 responses.
	defaultRetryAfterSeconds = 30

	// memCheckInterval is how often the background goroutine re-evaluates memory usage.
	memCheckInterval = 5 * time.Second
)

// Prometheus metrics for memory pressure middleware.
var (
	memoryPressureRejections = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "api_gateway",
		Name:      "memory_pressure_rejections_total",
		Help:      "Total number of requests rejected due to memory pressure.",
	})

	memoryUsageRatio = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "api_gateway",
		Name:      "memory_usage_ratio",
		Help:      "Current memory usage as a ratio of the pod memory limit (0.0–1.0).",
	})

	memoryPressureActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "api_gateway",
		Name:      "memory_pressure_active",
		Help:      "1 if memory pressure protection is active (rejecting requests), 0 otherwise.",
	})
)

// MemoryPressureConfig holds configuration for the memory pressure middleware.
type MemoryPressureConfig struct {
	// PodMemoryLimitBytes is the total memory available to the pod.
	PodMemoryLimitBytes uint64

	// HighWatermark is the ratio (0–1) above which requests are rejected. Default: 0.85.
	HighWatermark float64

	// LowWatermark is the ratio (0–1) below which requests are accepted again. Default: 0.80.
	LowWatermark float64

	// RetryAfterSeconds is the value of the Retry-After header. Default: 30.
	RetryAfterSeconds int

	// CheckInterval is how often the background goroutine checks memory. Default: 5s.
	CheckInterval time.Duration

	// Logger is the structured logger. If nil, a default production logger is used.
	Logger *zap.Logger
}

// MemoryPressureMiddleware monitors pod memory usage and rejects requests
// with HTTP 503 + Retry-After when memory exceeds the high watermark.
// It implements hysteresis: once triggered, it stays triggered until memory
// drops below the low watermark. This prevents flapping.
type MemoryPressureMiddleware struct {
	config   MemoryPressureConfig
	logger   *zap.Logger
	shedding atomic.Bool // true = currently rejecting requests

	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewMemoryPressureMiddleware creates a new memory pressure middleware.
// It reads the pod memory limit from the POD_MEMORY_LIMIT_BYTES environment variable.
// If not set, it falls back to the defaultPodMemoryLimitBytes constant.
func NewMemoryPressureMiddleware(logger *zap.Logger) *MemoryPressureMiddleware {
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	podLimit := readPodMemoryLimit()

	cfg := MemoryPressureConfig{
		PodMemoryLimitBytes: podLimit,
		HighWatermark:       highWatermark,
		LowWatermark:        lowWatermark,
		RetryAfterSeconds:   defaultRetryAfterSeconds,
		CheckInterval:       memCheckInterval,
		Logger:              logger,
	}

	return NewMemoryPressureMiddlewareWithConfig(cfg)
}

// NewMemoryPressureMiddlewareWithConfig creates a new memory pressure middleware
// with the provided configuration.
func NewMemoryPressureMiddlewareWithConfig(cfg MemoryPressureConfig) *MemoryPressureMiddleware {
	if cfg.Logger == nil {
		cfg.Logger, _ = zap.NewProduction()
	}
	if cfg.PodMemoryLimitBytes == 0 {
		cfg.PodMemoryLimitBytes = defaultPodMemoryLimitBytes
	}
	if cfg.HighWatermark == 0 {
		cfg.HighWatermark = highWatermark
	}
	if cfg.LowWatermark == 0 {
		cfg.LowWatermark = lowWatermark
	}
	if cfg.RetryAfterSeconds == 0 {
		cfg.RetryAfterSeconds = defaultRetryAfterSeconds
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = memCheckInterval
	}

	m := &MemoryPressureMiddleware{
		config: cfg,
		logger: cfg.Logger,
		stopCh: make(chan struct{}),
	}

	// Start background memory monitoring goroutine.
	go m.monitor()

	m.logger.Info("Memory pressure middleware initialized",
		zap.Uint64("pod_memory_limit_bytes", cfg.PodMemoryLimitBytes),
		zap.Float64("high_watermark", cfg.HighWatermark),
		zap.Float64("low_watermark", cfg.LowWatermark),
		zap.Int("retry_after_seconds", cfg.RetryAfterSeconds),
	)

	return m
}

// Middleware returns the Gin middleware handler that rejects requests when
// memory pressure is detected.
func (m *MemoryPressureMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if m.shedding.Load() {
			memoryPressureRejections.Inc()

			c.Header("Retry-After", strconv.Itoa(m.config.RetryAfterSeconds))
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "MEMORY_PRESSURE",
					"message": "Service is under memory pressure. Please retry later.",
				},
				"retry_after": m.config.RetryAfterSeconds,
			})
			return
		}

		c.Next()
	}
}

// Stop terminates the background monitoring goroutine.
func (m *MemoryPressureMiddleware) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// IsShedding returns true if the middleware is currently rejecting requests.
func (m *MemoryPressureMiddleware) IsShedding() bool {
	return m.shedding.Load()
}

// monitor runs in a background goroutine, periodically checking memory usage
// and toggling the shedding state with hysteresis.
func (m *MemoryPressureMiddleware) monitor() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkMemory()
		}
	}
}

// checkMemory reads the current memory stats and updates the shedding state.
func (m *MemoryPressureMiddleware) checkMemory() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Use Sys as the total memory obtained from the OS by the Go runtime.
	// This represents the actual memory footprint of the process.
	currentUsage := memStats.Sys
	ratio := float64(currentUsage) / float64(m.config.PodMemoryLimitBytes)

	// Update Prometheus gauge.
	memoryUsageRatio.Set(ratio)

	wasShedding := m.shedding.Load()

	if !wasShedding && ratio >= m.config.HighWatermark {
		// Transition: accepting → rejecting
		m.shedding.Store(true)
		memoryPressureActive.Set(1)
		m.logger.Warn("Memory pressure: activating load shedding",
			zap.Float64("usage_ratio", ratio),
			zap.Uint64("current_bytes", currentUsage),
			zap.Uint64("limit_bytes", m.config.PodMemoryLimitBytes),
			zap.Float64("high_watermark", m.config.HighWatermark),
		)
	} else if wasShedding && ratio < m.config.LowWatermark {
		// Transition: rejecting → accepting (hysteresis)
		m.shedding.Store(false)
		memoryPressureActive.Set(0)
		m.logger.Info("Memory pressure: deactivating load shedding",
			zap.Float64("usage_ratio", ratio),
			zap.Uint64("current_bytes", currentUsage),
			zap.Uint64("limit_bytes", m.config.PodMemoryLimitBytes),
			zap.Float64("low_watermark", m.config.LowWatermark),
		)
	}
}

// readPodMemoryLimit reads the pod memory limit from the environment.
// It checks POD_MEMORY_LIMIT_BYTES first, then attempts to read from
// the cgroups v2 memory limit file (/sys/fs/cgroup/memory.max),
// falling back to cgroups v1 (/sys/fs/cgroup/memory/memory.limit_in_bytes).
// If none are available, it returns the default limit.
func readPodMemoryLimit() uint64 {
	// Priority 1: Explicit environment variable.
	if envVal := os.Getenv("POD_MEMORY_LIMIT_BYTES"); envVal != "" {
		if limit, err := strconv.ParseUint(envVal, 10, 64); err == nil && limit > 0 {
			return limit
		}
	}

	// Priority 2: cgroups v2 memory limit.
	if data, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		val := trimNewline(string(data))
		if val != "max" {
			if limit, err := strconv.ParseUint(val, 10, 64); err == nil && limit > 0 {
				return limit
			}
		}
	}

	// Priority 3: cgroups v1 memory limit.
	if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		val := trimNewline(string(data))
		if limit, err := strconv.ParseUint(val, 10, 64); err == nil && limit > 0 {
			// cgroups v1 returns a very large number when unlimited; ignore it.
			if limit < 1<<62 {
				return limit
			}
		}
	}

	return defaultPodMemoryLimitBytes
}

// trimNewline removes trailing newline characters from a string.
func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
