package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/auction-system/bid-service/internal/dto"
	kafkaPkg "github.com/auction-system/bid-service/internal/kafka"
	"github.com/auction-system/bid-service/internal/lock"
	"github.com/auction-system/bid-service/internal/model"
	"github.com/auction-system/bid-service/internal/observability"
	redisPkg "github.com/auction-system/bid-service/internal/redis"
	"github.com/auction-system/bid-service/internal/repository"
	walletPkg "github.com/auction-system/bid-service/internal/wallet"
	wsPkg "github.com/auction-system/bid-service/internal/websocket"
)

var (
	ErrAuctionNotFound  = errors.New("auction not found")
	ErrAuctionNotActive = errors.New("auction is not active")
	ErrBidTooLow        = errors.New("bid amount must be higher than current highest bid")
	ErrAuctionLocked    = errors.New("auction is busy, please try again")
	ErrSellerCannotBid  = errors.New("seller cannot bid on their own auction")
)

// PlaceBid executes the full bid placement flow with full observability:
// 1. Start trace span
// 2. Acquire distributed lock
// 3. Validate auction status
// 4. Check bid amount vs current highest
// 5. Save bid to DB
// 6. Update Redis cache
// 7. Publish Kafka event
// 8. Broadcast via WebSocket
// 9. Release lock + record metrics
func PlaceBid(ctx context.Context, userID string, req dto.CreateBidRequest) (*dto.BidResponse, error) {
	// ── Tracing ───────────────────────────────────────────────────────────
	ctx, span := observability.StartSpan(ctx, "service.PlaceBid",
		observability.BidAttributes(req.AuctionID, userID, req.Amount)...,
	)
	defer span.End()

	logger := observability.FromCtx(ctx).With(
		zap.String("auction_id", req.AuctionID),
		zap.String("user_id", userID),
		zap.Float64("amount", req.Amount),
	)

	// ── Metrics timer ─────────────────────────────────────────────────────
	bidStart := time.Now()

	// ── Step 1: Acquire distributed lock ─────────────────────────────────
	lockCtx, lockSpan := observability.StartSpan(ctx, "lock.Acquire",
		attribute.String("auction_id", req.AuctionID),
	)
	distLock := lock.NewLock(req.AuctionID, 5*time.Second)
	acquired, err := distLock.Acquire(lockCtx)
	lockSpan.End()

	if err != nil {
		observability.LockAcquireTotal.WithLabelValues("error").Inc()
		observability.RecordError(span, err)
		return nil, fmt.Errorf("lock error: %w", err)
	}
	if !acquired {
		observability.LockAcquireTotal.WithLabelValues("failed").Inc()
		logger.Warn("failed to acquire auction lock")
		return nil, ErrAuctionLocked
	}
	observability.LockAcquireTotal.WithLabelValues("acquired").Inc()
	defer distLock.Release(ctx)

	// ── Step 2: Fetch and validate auction ────────────────────────────────
	_, auctionSpan := observability.StartSpan(ctx, "repository.GetAuctionByID")
	auction, err := repository.GetAuctionByID(ctx, req.AuctionID)
	auctionSpan.End()

	if err != nil {
		observability.RecordError(span, err)
		return nil, ErrAuctionNotFound
	}

	if auction.Status != model.AuctionStatusActive {
		return nil, ErrAuctionNotActive
	}

	if auction.SellerID == userID {
		return nil, ErrSellerCannotBid
	}

	// ── Step 3: Check current highest bid (Redis → DB fallback) ──────────
	redisStart := time.Now()
	highestBid, err := redisPkg.GetHighestBid(req.AuctionID)
	observability.RedisOperationDuration.WithLabelValues("get").Observe(time.Since(redisStart).Seconds())

	if err != nil || highestBid == 0 {
		// Cache miss — fetch from DB
		_, dbSpan := observability.StartSpan(ctx, "repository.GetHighestBid")
		highestBid, err = repository.GetHighestBid(ctx, req.AuctionID)
		dbSpan.End()
		if err != nil {
			observability.RecordError(span, err)
			return nil, fmt.Errorf("failed to get highest bid: %w", err)
		}
	}

	floor := auction.CurrentPrice
	if highestBid > floor {
		floor = highestBid
	}

	if req.Amount <= floor {
		observability.BidRequestsTotal.WithLabelValues("rejected").Inc()
		return nil, fmt.Errorf("%w: current highest is %.2f", ErrBidTooLow, floor)
	}

	// ── Step 3.5: Hold funds from bidder's wallet ─────────────────────────
	holdRef := fmt.Sprintf("auction:%s:bid", req.AuctionID)
	if err := walletPkg.Hold(ctx, userID, req.Amount, holdRef); err != nil {
		// If wallet service is unavailable, log warning but allow bid
		// In production with strict mode, return error instead
		logger.Warn("wallet hold failed — proceeding without hold",
			zap.String("user_id", userID),
			zap.Float64("amount", req.Amount),
			zap.Error(err),
		)
	}

	// ── Step 4: Save bid to database ──────────────────────────────────────
	bid := model.Bid{
		ID:        uuid.New().String(),
		AuctionID: req.AuctionID,
		UserID:    userID,
		Amount:    req.Amount,
		CreatedAt: time.Now().UTC(),
	}

	dbStart := time.Now()
	_, createSpan := observability.StartSpan(ctx, "repository.CreateBid")
	err = repository.CreateBid(ctx, bid)
	createSpan.End()
	observability.DBQueryDuration.WithLabelValues("create_bid").Observe(time.Since(dbStart).Seconds())

	if err != nil {
		observability.DBErrorsTotal.WithLabelValues("create_bid").Inc()
		observability.BidRequestsTotal.WithLabelValues("error").Inc()
		observability.RecordError(span, err)
		return nil, fmt.Errorf("failed to save bid: %w", err)
	}

	// ── Step 5: Update Redis cache ────────────────────────────────────────
	redisSetStart := time.Now()
	if err := redisPkg.SetHighestBid(req.AuctionID, req.Amount, 2*time.Hour); err != nil {
		logger.Warn("failed to update redis cache", zap.Error(err))
		observability.RedisErrorsTotal.WithLabelValues("set").Inc()
	}
	observability.RedisOperationDuration.WithLabelValues("set").Observe(time.Since(redisSetStart).Seconds())

	// ── Step 6: Update auction current price in DB ────────────────────────
	if err := repository.UpdateCurrentPrice(ctx, req.AuctionID, req.Amount); err != nil {
		logger.Warn("failed to update auction price", zap.Error(err))
	}

	// ── Step 7: Publish Kafka event ───────────────────────────────────────
	event := kafkaPkg.BidPlacedEvent{
		EventType: kafkaPkg.EventBidPlaced,
		BidID:     bid.ID,
		AuctionID: bid.AuctionID,
		UserID:    bid.UserID,
		Amount:    bid.Amount,
		Timestamp: bid.CreatedAt,
	}
	if err := kafkaPkg.PublishEvent(kafkaPkg.EventBidPlaced, event); err != nil {
		logger.Warn("failed to publish kafka event", zap.Error(err))
		observability.KafkaPublishTotal.WithLabelValues(kafkaPkg.EventBidPlaced, "error").Inc()
	} else {
		observability.KafkaPublishTotal.WithLabelValues(kafkaPkg.EventBidPlaced, "success").Inc()
	}

	// ── Step 8: Broadcast to WebSocket clients ────────────────────────────
	if err := wsPkg.BroadcastNewBid(bid.AuctionID, bid.ID, bid.UserID, bid.Amount); err != nil {
		logger.Warn("failed to broadcast websocket", zap.Error(err))
	} else {
		observability.WebSocketMessagesTotal.WithLabelValues(bid.AuctionID).Inc()
	}

	// ── Step 8.5: Release previous bidder's held funds ────────────────────
	if highestBid > 0 {
		// Find the previous highest bidder
		prevBidderID, err := repository.GetHighestBidder(ctx, req.AuctionID, bid.ID)
		if err == nil && prevBidderID != "" {
			releaseRef := fmt.Sprintf("auction:%s:bid", req.AuctionID)
			if err := walletPkg.Release(ctx, prevBidderID, highestBid, releaseRef); err != nil {
				logger.Warn("failed to release previous bidder funds",
					zap.String("prev_bidder", prevBidderID),
					zap.Float64("amount", highestBid),
					zap.Error(err),
				)
			}
		}
	}

	// ── Step 9: Record success metrics ───────────────────────────────────
	observability.BidRequestsTotal.WithLabelValues("success").Inc()
	observability.BidLatency.WithLabelValues(req.AuctionID).Observe(time.Since(bidStart).Seconds())

	logger.Info("bid placed successfully",
		zap.String("bid_id", bid.ID),
		zap.Float64("amount", bid.Amount),
		zap.Duration("duration", time.Since(bidStart)),
	)

	return &dto.BidResponse{
		ID:        bid.ID,
		AuctionID: bid.AuctionID,
		UserID:    bid.UserID,
		Amount:    bid.Amount,
		CreatedAt: bid.CreatedAt,
	}, nil
}

// GetBidsByAuction returns all bids for an auction
func GetBidsByAuction(ctx context.Context, auctionID string) ([]dto.BidResponse, error) {
	ctx, span := observability.StartSpan(ctx, "service.GetBidsByAuction",
		attribute.String("auction_id", auctionID),
	)
	defer span.End()

	bids, err := repository.GetBidsByAuction(ctx, auctionID)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}

	resp := make([]dto.BidResponse, 0, len(bids))
	for _, b := range bids {
		resp = append(resp, dto.BidResponse{
			ID:        b.ID,
			AuctionID: b.AuctionID,
			UserID:    b.UserID,
			Amount:    b.Amount,
			CreatedAt: b.CreatedAt,
		})
	}
	return resp, nil
}

// GetBidsByUser returns all bids placed by a user
func GetBidsByUser(ctx context.Context, userID string) ([]dto.BidResponse, error) {
	ctx, span := observability.StartSpan(ctx, "service.GetBidsByUser",
		attribute.String("user_id", userID),
	)
	defer span.End()

	bids, err := repository.GetBidsByUser(ctx, userID)
	if err != nil {
		observability.RecordError(span, err)
		return nil, err
	}

	resp := make([]dto.BidResponse, 0, len(bids))
	for _, b := range bids {
		resp = append(resp, dto.BidResponse{
			ID:        b.ID,
			AuctionID: b.AuctionID,
			UserID:    b.UserID,
			Amount:    b.Amount,
			CreatedAt: b.CreatedAt,
		})
	}
	return resp, nil
}
