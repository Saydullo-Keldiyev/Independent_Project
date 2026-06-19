// Package logger provides production-grade structured logging with deduplication
// and secret redaction. All services import this for consistent JSON logging.
package logger

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ctxKey is used for storing correlation ID in context.
type ctxKey string

const correlationIDKey ctxKey = "correlation_id"

// LogConfig contains configuration for the structured logger.
type LogConfig struct {
	ServiceName    string
	Environment    string
	Version        string
	DedupWindow    time.Duration // default: 60s
	DedupThreshold int           // default: 10 entries
	Secrets        []string      // secret values to redact
}

// Logger provides structured logging with deduplication and redaction.
type Logger interface {
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Debug(msg string, fields ...zap.Field)
	With(fields ...zap.Field) Logger
	WithCorrelationID(id string) Logger
	Sync() error
}

// zapLogger implements the Logger interface wrapping Zap.
type zapLogger struct {
	zap           *zap.Logger
	config        LogConfig
	dedup         *deduplicator
	redactor      *redactor
	correlationID string
}

// New creates a new Logger with the given configuration.
func New(cfg LogConfig) (Logger, error) {
	if cfg.DedupWindow == 0 {
		cfg.DedupWindow = 60 * time.Second
	}
	if cfg.DedupThreshold == 0 {
		cfg.DedupThreshold = 10
	}

	var zapCfg zap.Config
	if cfg.Environment == "production" {
		zapCfg = zap.NewProductionConfig()
		zapCfg.EncoderConfig.TimeKey = "timestamp"
		zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapCfg.EncoderConfig.LevelKey = "level"
		zapCfg.EncoderConfig.MessageKey = "message"
		zapCfg.Encoding = "json"
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	baseLogger, err := zapCfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		return nil, err
	}

	// Add base fields that appear on every log entry.
	baseLogger = baseLogger.With(
		zap.String("service_name", cfg.ServiceName),
		zap.String("environment", cfg.Environment),
		zap.String("version", cfg.Version),
	)

	l := &zapLogger{
		zap:      baseLogger,
		config:   cfg,
		dedup:    newDeduplicator(cfg.DedupWindow, cfg.DedupThreshold),
		redactor: newRedactor(cfg.Secrets),
	}

	return l, nil
}

func (l *zapLogger) Info(msg string, fields ...zap.Field) {
	l.log(zapcore.InfoLevel, msg, fields...)
}

func (l *zapLogger) Warn(msg string, fields ...zap.Field) {
	l.log(zapcore.WarnLevel, msg, fields...)
}

func (l *zapLogger) Error(msg string, fields ...zap.Field) {
	l.log(zapcore.ErrorLevel, msg, fields...)
}

func (l *zapLogger) Debug(msg string, fields ...zap.Field) {
	l.log(zapcore.DebugLevel, msg, fields...)
}

func (l *zapLogger) With(fields ...zap.Field) Logger {
	return &zapLogger{
		zap:           l.zap.With(fields...),
		config:        l.config,
		dedup:         l.dedup,
		redactor:      l.redactor,
		correlationID: l.correlationID,
	}
}

func (l *zapLogger) WithCorrelationID(id string) Logger {
	return &zapLogger{
		zap:           l.zap.With(zap.String("correlation_id", id)),
		config:        l.config,
		dedup:         l.dedup,
		redactor:      l.redactor,
		correlationID: id,
	}
}

func (l *zapLogger) Sync() error {
	// Flush dedup summaries before syncing.
	l.dedup.flushAll(l.emitSummary)
	return l.zap.Sync()
}

// log handles redaction, deduplication, and emission.
func (l *zapLogger) log(level zapcore.Level, msg string, fields ...zap.Field) {
	// Redact secrets from the message.
	msg = l.redactor.Redact(msg)

	// Redact secrets from string fields.
	fields = l.redactor.RedactFields(fields)

	// Check deduplication.
	if l.dedup.shouldSuppress(level, msg) {
		return
	}

	// Build context fields.
	contextFields := l.buildContextFields(fields)

	switch level {
	case zapcore.InfoLevel:
		l.zap.Info(msg, contextFields...)
	case zapcore.WarnLevel:
		l.zap.Warn(msg, contextFields...)
	case zapcore.ErrorLevel:
		l.zap.Error(msg, contextFields...)
	case zapcore.DebugLevel:
		l.zap.Debug(msg, contextFields...)
	}
}

// buildContextFields adds trace_id and correlation_id if not already present.
func (l *zapLogger) buildContextFields(fields []zap.Field) []zap.Field {
	hasTraceID := false
	hasCorrelationID := false

	for _, f := range fields {
		if f.Key == "trace_id" {
			hasTraceID = true
		}
		if f.Key == "correlation_id" {
			hasCorrelationID = true
		}
	}

	if !hasTraceID {
		fields = append(fields, zap.String("trace_id", ""))
	}
	if !hasCorrelationID && l.correlationID != "" {
		fields = append(fields, zap.String("correlation_id", l.correlationID))
	} else if !hasCorrelationID {
		fields = append(fields, zap.String("correlation_id", ""))
	}

	return fields
}

func (l *zapLogger) emitSummary(level zapcore.Level, msg string, suppressed int) {
	l.zap.Log(level, msg,
		zap.Int("suppressed_count", suppressed),
		zap.Bool("dedup_summary", true),
	)
}

// WithCorrelationIDCtx stores a correlation ID in context.
func WithCorrelationIDCtx(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromCtx retrieves the correlation ID from context.
func CorrelationIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(correlationIDKey).(string)
	return id
}

// ---- Deduplication ----

// dedupKey uniquely identifies a (level, message) pair for deduplication.
type dedupKey struct {
	level zapcore.Level
	msg   string
}

// dedupEntry tracks the count and window timing for a dedup key.
type dedupEntry struct {
	count     int
	windowEnd time.Time
}

// deduplicator suppresses repeated log entries.
type deduplicator struct {
	mu        sync.Mutex
	entries   map[dedupKey]*dedupEntry
	window    time.Duration
	threshold int
	nowFunc   func() time.Time // for testing
}

func newDeduplicator(window time.Duration, threshold int) *deduplicator {
	return &deduplicator{
		entries:   make(map[dedupKey]*dedupEntry),
		window:    window,
		threshold: threshold,
		nowFunc:   time.Now,
	}
}

// shouldSuppress returns true if this log entry should be suppressed.
// It also handles emitting summaries when windows expire.
func (d *deduplicator) shouldSuppress(level zapcore.Level, msg string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := dedupKey{level: level, msg: msg}
	now := d.nowFunc()

	entry, exists := d.entries[key]
	if !exists {
		// First entry: start a new window.
		d.entries[key] = &dedupEntry{
			count:     1,
			windowEnd: now.Add(d.window),
		}
		return false
	}

	// Check if the window has expired.
	if now.After(entry.windowEnd) {
		// Window expired, reset.
		entry.count = 1
		entry.windowEnd = now.Add(d.window)
		return false
	}

	// Within window: increment count.
	entry.count++

	// Allow up to threshold entries.
	if entry.count <= d.threshold {
		return false
	}

	// Suppress beyond threshold.
	return true
}

// flushAll emits summaries for any entries that have suppressed messages.
func (d *deduplicator) flushAll(emitFn func(zapcore.Level, string, int)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, entry := range d.entries {
		suppressed := entry.count - d.threshold
		if suppressed > 0 {
			emitFn(key.level, key.msg, suppressed)
		}
	}
	d.entries = make(map[dedupKey]*dedupEntry)
}

// GetSuppressedCount returns the number of suppressed entries for a given key.
// Useful for testing.
func (d *deduplicator) GetSuppressedCount(level zapcore.Level, msg string) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := dedupKey{level: level, msg: msg}
	entry, exists := d.entries[key]
	if !exists {
		return 0
	}
	suppressed := entry.count - d.threshold
	if suppressed < 0 {
		return 0
	}
	return suppressed
}

// ---- Redaction ----

// redactor replaces secret substrings in log messages and fields.
type redactor struct {
	secrets []string
}

func newRedactor(secrets []string) *redactor {
	return &redactor{secrets: secrets}
}

// Redact replaces any 4+ character substring of configured secrets with [REDACTED].
func (r *redactor) Redact(input string) string {
	if len(r.secrets) == 0 {
		return input
	}

	result := input
	for _, secret := range r.secrets {
		if len(secret) < 4 {
			continue
		}
		// Generate all substrings of length 4+ and replace them.
		result = r.redactSecret(result, secret)
	}
	return result
}

// redactSecret replaces all substrings of 4+ chars from the secret found in input.
func (r *redactor) redactSecret(input, secret string) string {
	// Start from the longest substring to the shortest (min 4 chars).
	// This ensures we replace the largest matches first.
	for length := len(secret); length >= 4; length-- {
		for start := 0; start+length <= len(secret); start++ {
			substr := secret[start : start+length]
			if strings.Contains(input, substr) {
				input = strings.ReplaceAll(input, substr, "[REDACTED]")
			}
		}
	}
	return input
}

// RedactFields redacts string values in zap fields.
func (r *redactor) RedactFields(fields []zap.Field) []zap.Field {
	if len(r.secrets) == 0 {
		return fields
	}

	result := make([]zap.Field, len(fields))
	for i, f := range fields {
		if f.Type == zapcore.StringType {
			f.String = r.Redact(f.String)
		}
		result[i] = f
	}
	return result
}
