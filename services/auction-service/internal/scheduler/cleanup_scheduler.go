package scheduler

import (
	"context"
	"time"

	"github.com/auction-system/auction-service/internal/database"
)

// RunCleanup archives ended auctions older than retention (daily).
func RunCleanup(ctx context.Context, retentionDays int) {
	if retentionDays <= 0 {
		retentionDays = 90
	}
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = database.DB.Exec(ctx, `
				UPDATE auctions SET state = 'archived', status = 'ended', updated_at = NOW()
				WHERE state = 'ended' AND end_time < NOW() - make_interval(days => $1)
				AND deleted_at IS NULL`, retentionDays)
		}
	}
}
