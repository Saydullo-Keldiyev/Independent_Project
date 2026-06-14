package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/auction-system/auction-service/internal/kafka"
	"github.com/auction-system/auction-service/internal/model"
	redisPkg "github.com/auction-system/auction-service/internal/redis"
	"github.com/auction-system/auction-service/internal/repository"
)

// Scheduler runs periodic tasks: activate scheduled auctions, end expired ones.
// Uses Redis distributed lock to prevent duplicate processing across pods.
type Scheduler struct {
	interval time.Duration
	lockTTL  time.Duration
	log      *zap.Logger
}

func New(intervalSec, lockTTLSec int, log *zap.Logger) *Scheduler {
	return &Scheduler{
		interval: time.Duration(intervalSec) * time.Second,
		lockTTL:  time.Duration(lockTTLSec) * time.Second,
		log:      log,
	}
}

// Run starts the scheduler loop. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.log.Info("auction scheduler started", zap.Duration("interval", s.interval))

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	// Acquire distributed lock — only one pod processes at a time
	acquired, err := redisPkg.AcquireSchedulerLock("auction_scheduler", s.lockTTL)
	if err != nil || !acquired {
		return // another pod is handling it
	}
	defer redisPkg.ReleaseSchedulerLock("auction_scheduler")

	s.activateScheduled(ctx)
	s.endExpired(ctx)
}

// activateScheduled transitions scheduled → active when start_time is reached
func (s *Scheduler) activateScheduled(ctx context.Context) {
	auctions, err := repository.GetScheduledToActivate(ctx)
	if err != nil {
		s.log.Error("get scheduled auctions", zap.Error(err))
		return
	}

	for _, a := range auctions {
		if err := repository.UpdateState(ctx, a.ID, model.StateActive); err != nil {
			s.log.Error("activate auction", zap.String("id", a.ID), zap.Error(err))
			continue
		}

		kafka.PublishEvent(kafka.EventAuctionStarted, kafka.AuctionEvent{
			EventType: kafka.EventAuctionStarted,
			AuctionID: a.ID,
			SellerID:  a.SellerID,
			Title:     a.Title,
			Timestamp: time.Now(),
		})

		redisPkg.InvalidateAuction(a.ID)
		s.log.Info("auction activated", zap.String("id", a.ID), zap.String("title", a.Title))
	}
}

// endExpired transitions active → ended when end_time is reached, selects winner
func (s *Scheduler) endExpired(ctx context.Context) {
	auctions, err := repository.GetExpiredActive(ctx)
	if err != nil {
		s.log.Error("get expired auctions", zap.Error(err))
		return
	}

	for _, a := range auctions {
		s.endAuction(ctx, a)
	}
}

func (s *Scheduler) endAuction(ctx context.Context, a model.Auction) {
	// Find highest bidder (winner)
	winnerID, highestBid, err := repository.GetHighestBid(ctx, a.ID)
	if err != nil {
		s.log.Error("get highest bid", zap.String("auction_id", a.ID), zap.Error(err))
	}

	if winnerID != "" && highestBid >= a.ReservePrice {
		// Winner found — meets reserve price
		if err := repository.SetWinner(ctx, a.ID, winnerID, highestBid); err != nil {
			s.log.Error("set winner", zap.String("auction_id", a.ID), zap.Error(err))
			return
		}

		kafka.PublishEvent(kafka.EventAuctionEnded, kafka.AuctionEvent{
			EventType: kafka.EventAuctionEnded,
			AuctionID: a.ID,
			SellerID:  a.SellerID,
			Title:     a.Title,
			WinnerID:  winnerID,
			Amount:    highestBid,
			Timestamp: time.Now(),
		})

		s.log.Info("auction ended with winner",
			zap.String("auction_id", a.ID),
			zap.String("winner_id", winnerID),
			zap.Float64("amount", highestBid),
		)
	} else {
		// No winner (no bids or reserve not met)
		repository.UpdateState(ctx, a.ID, model.StateEnded)

		kafka.PublishEvent(kafka.EventAuctionEnded, kafka.AuctionEvent{
			EventType: kafka.EventAuctionEnded,
			AuctionID: a.ID,
			SellerID:  a.SellerID,
			Title:     a.Title,
			Timestamp: time.Now(),
		})

		s.log.Info("auction ended without winner", zap.String("auction_id", a.ID))
	}

	redisPkg.InvalidateAuction(a.ID)
}
