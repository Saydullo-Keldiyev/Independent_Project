package kafka

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// ConsumerConfig holds configuration for the Kafka consumer.
type ConsumerConfig struct {
	Brokers        []string
	GroupID        string
	Topics         []string
	RetryCount     int             // default: 3
	RetryBackoff   []time.Duration // default: [1s, 2s, 4s]
	DLQTopic       string          // Dead Letter Queue topic
	IdempotencyTTL time.Duration   // TTL for processed events (default: 7 days)
}

// consumerGroup implements the Consumer interface using Sarama consumer groups.
type consumerGroup struct {
	client     sarama.ConsumerGroup
	config     ConsumerConfig
	dedupStore *DedupStore
	dlqProducer sarama.SyncProducer
	metrics    *Metrics
	logger     *zap.Logger
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	stopped    chan struct{}
}

// NewConsumer creates a new idempotent Kafka consumer with DLQ support.
func NewConsumer(cfg ConsumerConfig, dedupStore *DedupStore, metrics *Metrics, logger *zap.Logger) (Consumer, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Apply defaults.
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if len(cfg.RetryBackoff) == 0 {
		cfg.RetryBackoff = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	}
	if cfg.IdempotencyTTL == 0 {
		cfg.IdempotencyTTL = 7 * 24 * time.Hour
	}

	saramaCfg := sarama.NewConfig()
	saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	saramaCfg.Consumer.Return.Errors = true

	client, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, saramaCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Create DLQ producer if DLQ topic is configured.
	var dlqProducer sarama.SyncProducer
	if cfg.DLQTopic != "" {
		dlqCfg := sarama.NewConfig()
		dlqCfg.Producer.Return.Successes = true
		dlqCfg.Producer.RequiredAcks = sarama.WaitForAll

		dlqProducer, err = sarama.NewSyncProducer(cfg.Brokers, dlqCfg)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to create DLQ producer: %w", err)
		}
	}

	return &consumerGroup{
		client:      client,
		config:      cfg,
		dedupStore:  dedupStore,
		dlqProducer: dlqProducer,
		metrics:     metrics,
		logger:      logger,
		stopped:     make(chan struct{}),
	}, nil
}

// NewConsumerFromGroup creates a Consumer from an existing Sarama consumer group.
// Useful for testing with mock consumer groups.
func NewConsumerFromGroup(
	client sarama.ConsumerGroup,
	cfg ConsumerConfig,
	dedupStore *DedupStore,
	dlqProducer sarama.SyncProducer,
	metrics *Metrics,
	logger *zap.Logger,
) Consumer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = 3
	}
	if len(cfg.RetryBackoff) == 0 {
		cfg.RetryBackoff = []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	}
	if cfg.IdempotencyTTL == 0 {
		cfg.IdempotencyTTL = 7 * 24 * time.Hour
	}
	return &consumerGroup{
		client:      client,
		config:      cfg,
		dedupStore:  dedupStore,
		dlqProducer: dlqProducer,
		metrics:     metrics,
		logger:      logger,
		stopped:     make(chan struct{}),
	}
}

// Start begins consuming messages. It blocks until ctx is cancelled or Stop is called.
func (c *consumerGroup) Start(ctx context.Context, handler MessageHandler) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	cgHandler := &consumerGroupHandler{
		handler:    handler,
		config:     c.config,
		dedupStore: c.dedupStore,
		dlqProducer: c.dlqProducer,
		metrics:    c.metrics,
		logger:     c.logger,
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			// Consume returns when ctx is cancelled or rebalance occurs.
			err := c.client.Consume(ctx, c.config.Topics, cgHandler)
			if err != nil {
				c.logger.Error("consumer group error", zap.Error(err))
			}
			if ctx.Err() != nil {
				close(c.stopped)
				return
			}
		}
	}()

	// Monitor consumer errors.
	go func() {
		for err := range c.client.Errors() {
			c.logger.Error("consumer group async error", zap.Error(err))
		}
	}()

	<-c.stopped
	return nil
}

// Stop gracefully stops the consumer.
func (c *consumerGroup) Stop() error {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()

	var errs []error
	if err := c.client.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close consumer group: %w", err))
	}
	if c.dlqProducer != nil {
		if err := c.dlqProducer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close DLQ producer: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during consumer stop: %v", errs)
	}
	return nil
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler.
type consumerGroupHandler struct {
	handler     MessageHandler
	config      ConsumerConfig
	dedupStore  *DedupStore
	dlqProducer sarama.SyncProducer
	metrics     *Metrics
	logger      *zap.Logger
}

func (h *consumerGroupHandler) Setup(session sarama.ConsumerGroupSession) error {
	h.logger.Info("consumer group session started",
		zap.String("group_id", h.config.GroupID),
		zap.Int32s("claims", claimedPartitions(session)),
	)
	return nil
}

func (h *consumerGroupHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	h.logger.Info("consumer group session ended",
		zap.String("group_id", h.config.GroupID),
	)
	return nil
}

func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processMessage(session, claim, msg)
	}
	return nil
}

// processMessage handles a single Kafka message with idempotency, retry, and DLQ.
func (h *consumerGroupHandler) processMessage(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim, saramaMsg *sarama.ConsumerMessage) {
	ctx := session.Context()

	// Parse message into our domain model.
	msg := h.parseMessage(saramaMsg)

	// Propagate correlation ID into context.
	if msg.CorrelationID != "" {
		ctx = WithCorrelationID(ctx, msg.CorrelationID)
	}

	// Update consumer lag metric using the claim's high water mark.
	if h.metrics != nil {
		highWaterMark := claim.HighWaterMarkOffset()
		lag := highWaterMark - saramaMsg.Offset - 1
		if lag < 0 {
			lag = 0
		}
		h.metrics.ConsumerLag.WithLabelValues(
			h.config.GroupID,
			saramaMsg.Topic,
			strconv.Itoa(int(saramaMsg.Partition)),
		).Set(float64(lag))
	}

	// Check for duplicate event_id.
	if h.dedupStore != nil && msg.EventID != "" {
		isDup, _ := h.dedupStore.IsDuplicate(ctx, msg.EventID)
		if isDup {
			h.logger.Debug("duplicate event skipped",
				zap.String("event_id", msg.EventID),
				zap.String("topic", saramaMsg.Topic),
			)
			if h.metrics != nil {
				h.metrics.DuplicateEventsTotal.WithLabelValues(h.config.GroupID, saramaMsg.Topic).Inc()
			}
			// Acknowledge the duplicate message.
			session.MarkMessage(saramaMsg, "")
			return
		}
	}

	// Process the message with retry logic.
	start := time.Now()
	var lastErr error

	for attempt := 0; attempt <= h.config.RetryCount; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay.
			backoffIdx := attempt - 1
			if backoffIdx >= len(h.config.RetryBackoff) {
				backoffIdx = len(h.config.RetryBackoff) - 1
			}
			delay := h.config.RetryBackoff[backoffIdx]

			h.logger.Warn("retrying message processing",
				zap.String("event_id", msg.EventID),
				zap.String("topic", saramaMsg.Topic),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", delay),
			)

			if h.metrics != nil {
				h.metrics.RetryTotal.WithLabelValues(h.config.GroupID, saramaMsg.Topic).Inc()
			}

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		lastErr = h.handler(ctx, msg)
		if lastErr == nil {
			break
		}

		h.logger.Error("message processing failed",
			zap.String("event_id", msg.EventID),
			zap.String("topic", saramaMsg.Topic),
			zap.Int("attempt", attempt+1),
			zap.Error(lastErr),
		)
	}

	duration := time.Since(start)
	if h.metrics != nil {
		h.metrics.ProcessingDuration.WithLabelValues(h.config.GroupID, saramaMsg.Topic).Observe(duration.Seconds())
		h.metrics.MessagesConsumed.WithLabelValues(h.config.GroupID, saramaMsg.Topic).Inc()
	}

	if lastErr != nil {
		// All retries exhausted — move to DLQ.
		h.sendToDLQ(ctx, saramaMsg, msg, lastErr)
	} else {
		// Mark as processed in the dedup store.
		if h.dedupStore != nil && msg.EventID != "" {
			_ = h.dedupStore.MarkProcessed(ctx, msg.EventID)
		}
	}

	// Acknowledge the message regardless of success/failure (DLQ handles failures).
	session.MarkMessage(saramaMsg, "")
}

// sendToDLQ sends a failed message to the Dead Letter Queue.
func (h *consumerGroupHandler) sendToDLQ(ctx context.Context, original *sarama.ConsumerMessage, msg *Message, lastErr error) {
	if h.dlqProducer == nil || h.config.DLQTopic == "" {
		h.logger.Error("message exhausted retries but no DLQ configured",
			zap.String("event_id", msg.EventID),
			zap.String("topic", original.Topic),
			zap.Error(lastErr),
		)
		return
	}

	// Build DLQ message with original headers plus error metadata.
	headers := []sarama.RecordHeader{
		{Key: []byte("X-Original-Topic"), Value: []byte(original.Topic)},
		{Key: []byte("X-Original-Partition"), Value: []byte(strconv.Itoa(int(original.Partition)))},
		{Key: []byte("X-Original-Offset"), Value: []byte(strconv.FormatInt(original.Offset, 10))},
		{Key: []byte("X-Error-Message"), Value: []byte(lastErr.Error())},
		{Key: []byte("X-Retry-Count"), Value: []byte(strconv.Itoa(h.config.RetryCount))},
	}

	// Carry over original headers.
	for _, h := range original.Headers {
		if h != nil {
			headers = append(headers, sarama.RecordHeader{Key: h.Key, Value: h.Value})
		}
	}

	dlqMsg := &sarama.ProducerMessage{
		Topic:   h.config.DLQTopic,
		Key:     sarama.ByteEncoder(original.Key),
		Value:   sarama.ByteEncoder(original.Value),
		Headers: headers,
	}

	_, _, err := h.dlqProducer.SendMessage(dlqMsg)
	if err != nil {
		h.logger.Error("failed to send message to DLQ",
			zap.String("event_id", msg.EventID),
			zap.String("dlq_topic", h.config.DLQTopic),
			zap.Error(err),
		)
		return
	}

	if h.metrics != nil {
		h.metrics.DLQSize.WithLabelValues(h.config.GroupID, original.Topic).Inc()
	}

	h.logger.Warn("message sent to DLQ after retries exhausted",
		zap.String("event_id", msg.EventID),
		zap.String("topic", original.Topic),
		zap.String("dlq_topic", h.config.DLQTopic),
		zap.Error(lastErr),
	)
}

// parseMessage converts a Sarama consumer message to our domain Message type.
func (h *consumerGroupHandler) parseMessage(saramaMsg *sarama.ConsumerMessage) *Message {
	msg := &Message{
		Topic:     saramaMsg.Topic,
		Key:       string(saramaMsg.Key),
		Payload:   saramaMsg.Value,
		Headers:   make(map[string]string),
		Timestamp: saramaMsg.Timestamp,
	}

	for _, header := range saramaMsg.Headers {
		if header == nil {
			continue
		}
		key := string(header.Key)
		value := string(header.Value)

		switch key {
		case HeaderEventID:
			msg.EventID = value
		case HeaderCorrelationID:
			msg.CorrelationID = value
		default:
			msg.Headers[key] = value
		}
	}

	return msg
}

// claimedPartitions extracts partition numbers from a session for logging.
func claimedPartitions(session sarama.ConsumerGroupSession) []int32 {
	var partitions []int32
	for _, parts := range session.Claims() {
		partitions = append(partitions, parts...)
	}
	return partitions
}
