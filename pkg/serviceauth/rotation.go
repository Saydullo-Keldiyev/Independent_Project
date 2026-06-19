package serviceauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"go.uber.org/zap"
)

// KeyGeneratorFunc generates a new signing key. The default generates a 32-byte random hex string.
type KeyGeneratorFunc func() (string, error)

// RotationConfig configures the automatic key rotation.
type RotationConfig struct {
	// RotationInterval is how often to rotate keys (default 7 days).
	RotationInterval time.Duration

	// OverlapPeriod is how long old keys remain valid (default 24h).
	OverlapPeriod time.Duration

	// CheckInterval is how often the scheduler checks if rotation is needed.
	// Defaults to 1 minute. Can be lowered for testing.
	CheckInterval time.Duration

	// KeyGenerator is a custom key generator. If nil, uses default random generator.
	KeyGenerator KeyGeneratorFunc

	// Logger for rotation events.
	Logger *zap.Logger
}

// RotationScheduler handles automatic credential rotation every 7 days
// with a 24h overlap period where both old and new credentials are accepted.
type RotationScheduler struct {
	keyManager       *KeyManager
	interval         time.Duration
	overlapPeriod    time.Duration
	checkInterval    time.Duration
	keyGenerator     KeyGeneratorFunc
	logger           *zap.Logger
	cancelFunc       context.CancelFunc
	mu               sync.Mutex
	lastRotation     time.Time
	nextRotation     time.Time
}

// NewRotationScheduler creates a scheduler that auto-rotates keys.
func NewRotationScheduler(keyManager *KeyManager, cfg *RotationConfig) *RotationScheduler {
	interval := cfg.RotationInterval
	if interval == 0 {
		interval = DefaultRotationInterval
	}

	overlapPeriod := cfg.OverlapPeriod
	if overlapPeriod == 0 {
		overlapPeriod = DefaultOverlapPeriod
	}

	keyGen := cfg.KeyGenerator
	if keyGen == nil {
		keyGen = defaultKeyGenerator
	}

	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	now := time.Now()
	return &RotationScheduler{
		keyManager:    keyManager,
		interval:      interval,
		overlapPeriod: overlapPeriod,
		checkInterval: cfg.CheckInterval,
		keyGenerator:  keyGen,
		logger:        logger,
		lastRotation:  now,
		nextRotation:  now.Add(interval),
	}
}

// Start begins the automatic key rotation schedule.
func (rs *RotationScheduler) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	rs.cancelFunc = cancel

	go rs.rotationLoop(ctx)

	rs.logger.Info("key rotation scheduler started",
		zap.Duration("interval", rs.interval),
		zap.Duration("overlap_period", rs.overlapPeriod),
		zap.Time("next_rotation", rs.nextRotation),
	)
}

// Stop stops the rotation scheduler.
func (rs *RotationScheduler) Stop() {
	if rs.cancelFunc != nil {
		rs.cancelFunc()
	}
}

// NextRotation returns the time of the next scheduled rotation.
func (rs *RotationScheduler) NextRotation() time.Time {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.nextRotation
}

// LastRotation returns the time of the last rotation.
func (rs *RotationScheduler) LastRotation() time.Time {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.lastRotation
}

// ForceRotate triggers an immediate key rotation.
func (rs *RotationScheduler) ForceRotate() error {
	return rs.rotate()
}

func (rs *RotationScheduler) rotationLoop(ctx context.Context) {
	checkInterval := rs.checkInterval
	if checkInterval == 0 {
		checkInterval = 1 * time.Minute
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			rs.logger.Info("key rotation scheduler stopped")
			return
		case now := <-ticker.C:
			rs.mu.Lock()
			shouldRotate := now.After(rs.nextRotation) || now.Equal(rs.nextRotation)
			rs.mu.Unlock()

			if shouldRotate {
				if err := rs.rotate(); err != nil {
					rs.logger.Error("key rotation failed",
						zap.Error(err),
						zap.Time("attempted_at", now),
					)
				}
			}
		}
	}
}

func (rs *RotationScheduler) rotate() error {
	newKey, err := rs.keyGenerator()
	if err != nil {
		return err
	}

	rs.keyManager.RotateKey(newKey)

	now := time.Now()
	rs.mu.Lock()
	rs.lastRotation = now
	rs.nextRotation = now.Add(rs.interval)
	rs.mu.Unlock()

	rs.logger.Info("service auth key rotated successfully",
		zap.Time("rotated_at", now),
		zap.Time("next_rotation", now.Add(rs.interval)),
		zap.Duration("overlap_period", rs.overlapPeriod),
	)

	return nil
}

// defaultKeyGenerator generates a cryptographically secure 32-byte hex key.
func defaultKeyGenerator() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
