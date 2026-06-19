package shutdown

import (
	"context"

	"github.com/IBM/sarama"
)

// SaramaFlusher wraps a sarama.SyncProducer to implement the KafkaFlusher interface.
// Since SyncProducer sends messages synchronously (blocking until ack), flushing
// means closing the producer gracefully which delivers any buffered messages.
type SaramaFlusher struct {
	producer sarama.SyncProducer
}

// NewSaramaFlusher creates a KafkaFlusher from a Sarama SyncProducer.
func NewSaramaFlusher(producer sarama.SyncProducer) *SaramaFlusher {
	return &SaramaFlusher{producer: producer}
}

// Flush closes the Sarama producer, which flushes any buffered messages.
// Returns 0 unflushed messages on success (SyncProducer delivers synchronously).
// Respects context cancellation for timeout enforcement.
func (f *SaramaFlusher) Flush(ctx context.Context) (unflushed int, err error) {
	done := make(chan error, 1)
	go func() {
		done <- f.producer.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			return 0, err
		}
		return 0, nil
	case <-ctx.Done():
		// Timeout exceeded — we can't determine unflushed count from Sarama's SyncProducer,
		// but we report at least 1 to indicate incomplete flush.
		return 1, ctx.Err()
	}
}

// SaramaAsyncFlusher wraps a sarama.AsyncProducer to implement the KafkaFlusher interface.
// AsyncProducer has a more meaningful flush operation since messages are buffered.
type SaramaAsyncFlusher struct {
	producer sarama.AsyncProducer
}

// NewSaramaAsyncFlusher creates a KafkaFlusher from a Sarama AsyncProducer.
func NewSaramaAsyncFlusher(producer sarama.AsyncProducer) *SaramaAsyncFlusher {
	return &SaramaAsyncFlusher{producer: producer}
}

// Flush closes the AsyncProducer and waits for pending messages to be delivered.
// The AsyncClose method will attempt to deliver all pending messages before returning.
func (f *SaramaAsyncFlusher) Flush(ctx context.Context) (unflushed int, err error) {
	done := make(chan struct{})
	go func() {
		f.producer.AsyncClose()
		// Drain successes and errors channels
		for range f.producer.Successes() {
		}
		for range f.producer.Errors() {
			unflushed++
		}
		close(done)
	}()

	select {
	case <-done:
		return unflushed, nil
	case <-ctx.Done():
		return unflushed, ctx.Err()
	}
}
