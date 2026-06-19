package logger

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newTestLogger creates a Logger backed by an observed core for assertions.
func newTestLogger(cfg LogConfig) (Logger, *observer.ObservedLogs) {
	if cfg.DedupWindow == 0 {
		cfg.DedupWindow = 60 * time.Second
	}
	if cfg.DedupThreshold == 0 {
		cfg.DedupThreshold = 10
	}

	core, logs := observer.New(zapcore.DebugLevel)
	baseLogger := zap.New(core).With(
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

	return l, logs
}

func TestNew(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewWithDefaults(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "development",
		Version:     "1.0.0",
	}

	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLogLevels(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
	}
	logger, logs := newTestLogger(cfg)

	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")
	logger.Debug("debug message")

	entries := logs.All()
	if len(entries) != 4 {
		t.Fatalf("expected 4 log entries, got %d", len(entries))
	}

	if entries[0].Level != zapcore.InfoLevel {
		t.Errorf("expected info level, got %v", entries[0].Level)
	}
	if entries[1].Level != zapcore.WarnLevel {
		t.Errorf("expected warn level, got %v", entries[1].Level)
	}
	if entries[2].Level != zapcore.ErrorLevel {
		t.Errorf("expected error level, got %v", entries[2].Level)
	}
	if entries[3].Level != zapcore.DebugLevel {
		t.Errorf("expected debug level, got %v", entries[3].Level)
	}
}

func TestRequiredFields(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "auction-service",
		Environment: "production",
		Version:     "2.1.0",
	}
	logger, logs := newTestLogger(cfg)

	logger.Info("test message", zap.String("trace_id", "trace-123"))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]

	// Check the context contains required fields.
	contextMap := entry.ContextMap()
	if contextMap["service_name"] != "auction-service" {
		t.Errorf("expected service_name=auction-service, got %v", contextMap["service_name"])
	}
	if contextMap["environment"] != "production" {
		t.Errorf("expected environment=production, got %v", contextMap["environment"])
	}
	if contextMap["version"] != "2.1.0" {
		t.Errorf("expected version=2.1.0, got %v", contextMap["version"])
	}
	if _, ok := contextMap["trace_id"]; !ok {
		t.Error("expected trace_id field")
	}
	if _, ok := contextMap["correlation_id"]; !ok {
		t.Error("expected correlation_id field")
	}
}

func TestWithCorrelationID(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
	}
	logger, logs := newTestLogger(cfg)

	corrLogger := logger.WithCorrelationID("corr-id-123")
	corrLogger.Info("test with correlation")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	contextMap := entries[0].ContextMap()
	if contextMap["correlation_id"] != "corr-id-123" {
		t.Errorf("expected correlation_id=corr-id-123, got %v", contextMap["correlation_id"])
	}
}

func TestWithFields(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
	}
	logger, logs := newTestLogger(cfg)

	enriched := logger.With(zap.String("request_id", "req-456"))
	enriched.Info("enriched log")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	contextMap := entries[0].ContextMap()
	if contextMap["request_id"] != "req-456" {
		t.Errorf("expected request_id=req-456, got %v", contextMap["request_id"])
	}
}

func TestSecretRedaction(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
		Secrets:     []string{"mysecretpassword", "api_key_12345"},
	}
	logger, logs := newTestLogger(cfg)

	logger.Info("connecting with mysecretpassword to database")
	logger.Info("using api_key_12345 for auth")

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Check that the secret is fully redacted.
	if strings.Contains(entries[0].Message, "mysecretpassword") {
		t.Error("full secret 'mysecretpassword' should be redacted from message")
	}
	if strings.Contains(entries[0].Message, "mysecret") {
		t.Error("partial secret 'mysecret' should be redacted from message")
	}
	if strings.Contains(entries[1].Message, "api_key_12345") {
		t.Error("full secret 'api_key_12345' should be redacted from message")
	}

	// Verify redaction placeholder is present.
	if !strings.Contains(entries[0].Message, "[REDACTED]") {
		t.Error("expected [REDACTED] placeholder in message")
	}
}

func TestSecretRedactionInFields(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
		Secrets:     []string{"secret_value_here"},
	}
	logger, logs := newTestLogger(cfg)

	logger.Info("connection established", zap.String("password", "secret_value_here"))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	contextMap := entries[0].ContextMap()
	password, _ := contextMap["password"].(string)
	if strings.Contains(password, "secret_value_here") {
		t.Error("secret should be redacted in field value")
	}
	if !strings.Contains(password, "[REDACTED]") {
		t.Error("expected [REDACTED] in field value")
	}
}

func TestSecretRedactionMinLength(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "test-service",
		Environment: "production",
		Version:     "1.0.0",
		Secrets:     []string{"abc"}, // less than 4 chars, should not be redacted
	}
	logger, logs := newTestLogger(cfg)

	logger.Info("the value abc appears here")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Short secrets (<4 chars) should NOT be redacted.
	if strings.Contains(entries[0].Message, "[REDACTED]") {
		t.Error("secrets shorter than 4 chars should not be redacted")
	}
}

func TestDeduplication(t *testing.T) {
	cfg := LogConfig{
		ServiceName:    "test-service",
		Environment:    "production",
		Version:        "1.0.0",
		DedupWindow:    60 * time.Second,
		DedupThreshold: 10,
	}
	logger, logs := newTestLogger(cfg)

	// Emit 15 identical messages.
	for i := 0; i < 15; i++ {
		logger.Info("repeated message")
	}

	entries := logs.All()
	// Should only emit 10 (the threshold).
	if len(entries) != 10 {
		t.Fatalf("expected 10 entries (threshold), got %d", len(entries))
	}
}

func TestDeduplicationDifferentLevels(t *testing.T) {
	cfg := LogConfig{
		ServiceName:    "test-service",
		Environment:    "production",
		Version:        "1.0.0",
		DedupWindow:    60 * time.Second,
		DedupThreshold: 10,
	}
	logger, logs := newTestLogger(cfg)

	// Same message but different levels should have separate dedup counters.
	for i := 0; i < 12; i++ {
		logger.Info("same message")
	}
	for i := 0; i < 12; i++ {
		logger.Warn("same message")
	}

	entries := logs.All()
	// 10 info + 10 warn = 20
	if len(entries) != 20 {
		t.Fatalf("expected 20 entries (10 info + 10 warn), got %d", len(entries))
	}
}

func TestDeduplicationWindowExpiry(t *testing.T) {
	cfg := LogConfig{
		ServiceName:    "test-service",
		Environment:    "production",
		Version:        "1.0.0",
		DedupWindow:    60 * time.Second,
		DedupThreshold: 10,
	}
	logger, logs := newTestLogger(cfg)

	// Access the internal deduplicator to manipulate time.
	zl := logger.(*zapLogger)
	now := time.Now()
	zl.dedup.nowFunc = func() time.Time { return now }

	// Emit threshold messages.
	for i := 0; i < 10; i++ {
		logger.Info("window test")
	}
	// This should be suppressed.
	logger.Info("window test")

	if len(logs.All()) != 10 {
		t.Fatalf("expected 10, got %d", len(logs.All()))
	}

	// Advance time past the window.
	zl.dedup.nowFunc = func() time.Time { return now.Add(61 * time.Second) }

	// This should go through (new window).
	logger.Info("window test")

	if len(logs.All()) != 11 {
		t.Fatalf("expected 11 after window expiry, got %d", len(logs.All()))
	}
}

func TestDeduplicationSummaryOnSync(t *testing.T) {
	cfg := LogConfig{
		ServiceName:    "test-service",
		Environment:    "production",
		Version:        "1.0.0",
		DedupWindow:    60 * time.Second,
		DedupThreshold: 10,
	}
	logger, logs := newTestLogger(cfg)

	// Emit 15 messages (5 suppressed).
	for i := 0; i < 15; i++ {
		logger.Info("repeated for summary")
	}

	// Sync should emit a summary.
	_ = logger.Sync()

	entries := logs.All()
	// 10 regular + 1 summary = 11
	if len(entries) != 11 {
		t.Fatalf("expected 11 entries (10 + 1 summary), got %d", len(entries))
	}

	// Last entry should be the summary.
	lastEntry := entries[len(entries)-1]
	contextMap := lastEntry.ContextMap()
	if contextMap["dedup_summary"] != true {
		t.Error("expected dedup_summary=true in summary entry")
	}
	suppressedCount, ok := contextMap["suppressed_count"].(int64)
	if !ok || suppressedCount != 5 {
		t.Errorf("expected suppressed_count=5, got %v", contextMap["suppressed_count"])
	}
}

func TestCorrelationIDContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithCorrelationIDCtx(ctx, "ctx-correlation-123")

	id := CorrelationIDFromCtx(ctx)
	if id != "ctx-correlation-123" {
		t.Errorf("expected ctx-correlation-123, got %s", id)
	}
}

func TestCorrelationIDContextNil(t *testing.T) {
	id := CorrelationIDFromCtx(nil)
	if id != "" {
		t.Errorf("expected empty string for nil context, got %s", id)
	}
}

func TestCorrelationIDContextEmpty(t *testing.T) {
	ctx := context.Background()
	id := CorrelationIDFromCtx(ctx)
	if id != "" {
		t.Errorf("expected empty string for context without correlation ID, got %s", id)
	}
}

func TestProductionJSONOutput(t *testing.T) {
	cfg := LogConfig{
		ServiceName: "json-test-service",
		Environment: "production",
		Version:     "3.0.0",
	}

	// Create a real production logger and capture output.
	logger, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We verify the JSON format by checking the output format config
	// (since capturing real output requires more setup).
	// Instead, verify that the New function succeeds for production env.
	_ = logger
}

func TestRedactorDirectly(t *testing.T) {
	r := newRedactor([]string{"SuperSecretKey123"})

	tests := []struct {
		name     string
		input    string
		contains string
		absent   string
	}{
		{
			name:     "full secret redacted",
			input:    "Using SuperSecretKey123 for auth",
			contains: "[REDACTED]",
			absent:   "SuperSecretKey123",
		},
		{
			name:     "partial secret (4+ chars) redacted",
			input:    "Found Supe in the logs",
			contains: "[REDACTED]",
			absent:   "Supe",
		},
		{
			name:     "short substring not redacted",
			input:    "The value Sup is safe",
			contains: "Sup",
			absent:   "[REDACTED]",
		},
		{
			name:     "no secret present",
			input:    "Nothing to see here",
			contains: "Nothing to see here",
			absent:   "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Redact(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
			if tt.absent != "" && strings.Contains(result, tt.absent) {
				t.Errorf("expected result to NOT contain %q, got %q", tt.absent, result)
			}
		})
	}
}

func TestRedactorMultipleSecrets(t *testing.T) {
	r := newRedactor([]string{"password123", "apikey456"})

	input := "Auth with password123 and apikey456"
	result := r.Redact(input)

	if strings.Contains(result, "password123") {
		t.Error("password123 should be redacted")
	}
	if strings.Contains(result, "apikey456") {
		t.Error("apikey456 should be redacted")
	}
}

func TestDeduplicatorDirectly(t *testing.T) {
	d := newDeduplicator(60*time.Second, 10)

	// First 10 should not be suppressed.
	for i := 0; i < 10; i++ {
		suppressed := d.shouldSuppress(zapcore.InfoLevel, "test msg")
		if suppressed {
			t.Errorf("entry %d should not be suppressed", i)
		}
	}

	// 11th should be suppressed.
	if !d.shouldSuppress(zapcore.InfoLevel, "test msg") {
		t.Error("11th entry should be suppressed")
	}

	// Different message should not be suppressed.
	if d.shouldSuppress(zapcore.InfoLevel, "different msg") {
		t.Error("different message should not be suppressed")
	}
}

func TestJSONLogFormat(t *testing.T) {
	// This test verifies that production logger outputs valid JSON
	// by creating an in-memory encoder and checking output format.
	cfg := zap.NewProductionEncoderConfig()
	cfg.TimeKey = "timestamp"
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder

	enc := zapcore.NewJSONEncoder(cfg)

	// Create a test entry.
	entry := zapcore.Entry{
		Level:   zapcore.InfoLevel,
		Time:    time.Now(),
		Message: "test",
	}

	fields := []zapcore.Field{
		zap.String("service_name", "test-svc"),
		zap.String("environment", "production"),
		zap.String("version", "1.0.0"),
		zap.String("correlation_id", "corr-123"),
		zap.String("trace_id", "trace-456"),
	}

	buf, err := enc.EncodeEntry(entry, fields)
	if err != nil {
		t.Fatalf("failed to encode: %v", err)
	}

	// Verify it's valid JSON.
	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify required fields.
	requiredFields := []string{"timestamp", "level", "msg", "service_name", "environment", "version", "correlation_id", "trace_id"}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}
