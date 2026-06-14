package email

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

const maxRetries = 3

// SendWithRetry attempts to send an email with exponential backoff.
// Returns nil on success, error after all retries exhausted.
func (s *Sender) SendWithRetry(msg Message, log *zap.Logger) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := s.Send(msg); err != nil {
			lastErr = err
			backoff := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s, 4s

			log.Warn("email send failed, retrying",
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", maxRetries),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)

			time.Sleep(backoff)
			continue
		}

		// Success
		return nil
	}

	return fmt.Errorf("email send failed after %d retries: %w", maxRetries, lastErr)
}
