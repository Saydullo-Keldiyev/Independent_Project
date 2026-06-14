package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/auction-system/notification-service/internal/consumer"
	"github.com/auction-system/notification-service/internal/email"
	"github.com/auction-system/notification-service/internal/model"
	redisPkg "github.com/auction-system/notification-service/internal/redis"
	"github.com/auction-system/notification-service/internal/repository"
	wsPkg "github.com/auction-system/notification-service/internal/websocket"
)

// NotificationService handles all notification dispatch logic.
// Implements consumer.NotificationHandler interface.
type NotificationService struct {
	emailSender *email.Sender
	log         *zap.Logger
}

func New(emailSender *email.Sender, log *zap.Logger) *NotificationService {
	return &NotificationService{
		emailSender: emailSender,
		log:         log,
	}
}

// HandleBidPlaced processes bid.placed events
func (s *NotificationService) HandleBidPlaced(ctx context.Context, event consumer.BidPlacedEvent) error {
	eventID := fmt.Sprintf("bid_placed:%s", event.BidID)

	// Idempotency check
	if redisPkg.IsProcessed(eventID) {
		s.log.Debug("event already processed, skipping", zap.String("event_id", eventID))
		return nil
	}

	// Persist notification
	notif := model.Notification{
		ID:        uuid.New().String(),
		UserID:    event.UserID,
		Type:      model.TypeBidPlaced,
		Title:     "Bid Placed Successfully",
		Message:   fmt.Sprintf("Your bid of $%.2f has been placed on auction %s", event.Amount, event.AuctionID),
		Metadata:  mustJSON(map[string]any{"auction_id": event.AuctionID, "amount": event.Amount, "bid_id": event.BidID}),
		EventID:   eventID,
		CreatedAt: time.Now().UTC(),
	}

	if err := repository.Create(ctx, notif); err != nil {
		s.log.Error("failed to persist notification", zap.Error(err))
		// Non-fatal — still try to deliver
	}

	// WebSocket — instant push
	wsPkg.NotifyUser(event.UserID, wsPkg.WSNotification{
		Type:      string(model.TypeBidPlaced),
		Title:     notif.Title,
		Message:   notif.Message,
		AuctionID: event.AuctionID,
		Amount:    event.Amount,
		Timestamp: event.Timestamp,
	})

	// Mark as processed
	redisPkg.MarkProcessed(eventID)

	s.log.Info("bid.placed notification dispatched",
		zap.String("user_id", event.UserID),
		zap.String("auction_id", event.AuctionID),
	)

	return nil
}

// HandleOutbid processes outbid events — notify the previous highest bidder
func (s *NotificationService) HandleOutbid(ctx context.Context, event consumer.OutbidEvent) error {
	eventID := fmt.Sprintf("outbid:%s:%s", event.AuctionID, event.OutbidUserID)

	if redisPkg.IsProcessed(eventID) {
		return nil
	}

	notif := model.Notification{
		ID:        uuid.New().String(),
		UserID:    event.OutbidUserID,
		Type:      model.TypeOutbid,
		Title:     "You've Been Outbid!",
		Message:   fmt.Sprintf("Someone placed a higher bid of $%.2f. Place a new bid to stay in the running!", event.NewAmount),
		Metadata:  mustJSON(map[string]any{"auction_id": event.AuctionID, "new_amount": event.NewAmount}),
		EventID:   eventID,
		CreatedAt: time.Now().UTC(),
	}

	repository.Create(ctx, notif)

	// WebSocket push
	wsPkg.NotifyUser(event.OutbidUserID, wsPkg.WSNotification{
		Type:      string(model.TypeOutbid),
		Title:     notif.Title,
		Message:   notif.Message,
		AuctionID: event.AuctionID,
		Amount:    event.NewAmount,
		Timestamp: time.Now(),
	})

	// Email (async with retry)
	if s.emailSender != nil && event.OutbidUserEmail != "" {
		go func() {
			msg := email.OutbidTemplate("Auction", event.NewAmount)
			msg.To = []string{event.OutbidUserEmail}
			if err := s.emailSender.SendWithRetry(msg, s.log); err != nil {
				s.log.Error("outbid email failed", zap.String("user_id", event.OutbidUserID), zap.Error(err))
			}
		}()
	}

	redisPkg.MarkProcessed(eventID)
	return nil
}

// HandleAuctionWon processes auction.won events
func (s *NotificationService) HandleAuctionWon(ctx context.Context, event consumer.AuctionWonEvent) error {
	eventID := fmt.Sprintf("auction_won:%s:%s", event.AuctionID, event.WinnerID)

	if redisPkg.IsProcessed(eventID) {
		return nil
	}

	notif := model.Notification{
		ID:        uuid.New().String(),
		UserID:    event.WinnerID,
		Type:      model.TypeAuctionWon,
		Title:     "🎉 You Won the Auction!",
		Message:   fmt.Sprintf("Congratulations! You won with a bid of $%.2f", event.Amount),
		Metadata:  mustJSON(map[string]any{"auction_id": event.AuctionID, "amount": event.Amount}),
		EventID:   eventID,
		CreatedAt: time.Now().UTC(),
	}

	repository.Create(ctx, notif)

	// WebSocket
	wsPkg.NotifyUser(event.WinnerID, wsPkg.WSNotification{
		Type:      string(model.TypeAuctionWon),
		Title:     notif.Title,
		Message:   notif.Message,
		AuctionID: event.AuctionID,
		Amount:    event.Amount,
		Timestamp: event.Timestamp,
	})

	// Email
	if s.emailSender != nil && event.WinnerEmail != "" {
		go func() {
			msg := email.AuctionWonEmail("Auction Item", event.Amount)
			msg.To = []string{event.WinnerEmail}
			if err := s.emailSender.SendWithRetry(msg, s.log); err != nil {
				s.log.Error("auction won email failed", zap.String("winner_id", event.WinnerID), zap.Error(err))
			}
		}()
	}

	redisPkg.MarkProcessed(eventID)
	return nil
}

// HandleAuctionEnded processes auction.ended events — notify seller and winner
func (s *NotificationService) HandleAuctionEnded(ctx context.Context, event consumer.AuctionEndedEvent) error {
	eventID := fmt.Sprintf("auction_ended:%s", event.AuctionID)

	if redisPkg.IsProcessed(eventID) {
		return nil
	}

	// ── Notify Seller ─────────────────────────────────────────────────────
	if event.SellerID != "" {
		var sellerTitle, sellerMsg string
		if event.WinnerID != "" {
			sellerTitle = "🎉 Your Auction Sold!"
			sellerMsg = fmt.Sprintf("Your auction \"%s\" ended! Winner: %s, Final Price: $%.2f", event.Title, event.WinnerID[:8], event.Amount)
		} else {
			sellerTitle = "🏁 Auction Ended"
			sellerMsg = fmt.Sprintf("Your auction \"%s\" ended without a winning bid.", event.Title)
		}

		sellerNotif := model.Notification{
			ID:        uuid.New().String(),
			UserID:    event.SellerID,
			Type:      model.TypeAuctionEnded,
			Title:     sellerTitle,
			Message:   sellerMsg,
			Metadata:  mustJSON(map[string]any{"auction_id": event.AuctionID, "winner_id": event.WinnerID, "amount": event.Amount}),
			EventID:   fmt.Sprintf("auction_ended_seller:%s", event.AuctionID),
			CreatedAt: time.Now().UTC(),
		}

		if err := repository.Create(ctx, sellerNotif); err != nil {
			s.log.Error("failed to persist seller notification", zap.Error(err))
		}

		wsPkg.NotifyUser(event.SellerID, wsPkg.WSNotification{
			Type:      string(model.TypeAuctionEnded),
			Title:     sellerTitle,
			Message:   sellerMsg,
			AuctionID: event.AuctionID,
			Amount:    event.Amount,
			Timestamp: event.Timestamp,
		})
	}

	// ── Notify Winner ─────────────────────────────────────────────────────
	if event.WinnerID != "" {
		winnerTitle := "🏆 You Won the Auction!"
		winnerMsg := fmt.Sprintf("Congratulations! You won \"%s\" with a bid of $%.2f", event.Title, event.Amount)

		winnerNotif := model.Notification{
			ID:        uuid.New().String(),
			UserID:    event.WinnerID,
			Type:      model.TypeAuctionWon,
			Title:     winnerTitle,
			Message:   winnerMsg,
			Metadata:  mustJSON(map[string]any{"auction_id": event.AuctionID, "amount": event.Amount}),
			EventID:   fmt.Sprintf("auction_ended_winner:%s", event.AuctionID),
			CreatedAt: time.Now().UTC(),
		}

		if err := repository.Create(ctx, winnerNotif); err != nil {
			s.log.Error("failed to persist winner notification", zap.Error(err))
		}

		wsPkg.NotifyUser(event.WinnerID, wsPkg.WSNotification{
			Type:      string(model.TypeAuctionWon),
			Title:     winnerTitle,
			Message:   winnerMsg,
			AuctionID: event.AuctionID,
			Amount:    event.Amount,
			Timestamp: event.Timestamp,
		})
	}

	s.log.Info("auction.ended notifications dispatched",
		zap.String("auction_id", event.AuctionID),
		zap.String("seller_id", event.SellerID),
		zap.String("winner_id", event.WinnerID),
	)

	redisPkg.MarkProcessed(eventID)
	return nil
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
