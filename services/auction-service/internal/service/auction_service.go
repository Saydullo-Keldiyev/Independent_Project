package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/auction-system/auction-service/internal/dto"
	"github.com/auction-system/auction-service/internal/kafka"
	"github.com/auction-system/auction-service/internal/model"
	redisPkg "github.com/auction-system/auction-service/internal/redis"
	"github.com/auction-system/auction-service/internal/repository"
)

var (
	ErrNotFound       = errors.New("auction not found")
	ErrForbidden      = errors.New("you do not own this auction")
	ErrInvalidState   = errors.New("invalid state transition")
	ErrInvalidTime    = errors.New("end time must be after start time")
	ErrAlreadyStarted = errors.New("cannot modify active auction")
)

type AuctionService struct {
	log *zap.Logger
}

func New(log *zap.Logger) *AuctionService {
	return &AuctionService{log: log}
}

// Create creates a new auction in draft/scheduled state
func (s *AuctionService) Create(ctx context.Context, sellerID string, req dto.CreateAuctionRequest) (*dto.AuctionResponse, error) {
	startTime, _ := time.Parse(time.RFC3339, req.StartTime)
	endTime, _ := time.Parse(time.RFC3339, req.EndTime)

	if endTime.Before(startTime) || endTime.Before(time.Now()) {
		return nil, ErrInvalidTime
	}

	// Determine initial state
	state := model.StateScheduled
	if startTime.Before(time.Now()) {
		state = model.StateActive
	}

	categoryID := &req.CategoryID
	if req.CategoryID == "" {
		categoryID = nil
	}

	auction := model.Auction{
		ID:            uuid.New().String(),
		SellerID:      sellerID,
		Title:         req.Title,
		Description:   req.Description,
		CategoryID:    categoryID,
		StartingPrice: req.StartingPrice,
		ReservePrice:  req.ReservePrice,
		CurrentPrice:  req.StartingPrice,
		State:         state,
		StartTime:     startTime,
		EndTime:       endTime,
		TotalBids:     0,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := repository.Create(ctx, auction); err != nil {
		return nil, fmt.Errorf("create auction: %w", err)
	}

	// Publish event
	kafka.PublishEvent(kafka.EventAuctionCreated, kafka.AuctionEvent{
		EventType: kafka.EventAuctionCreated,
		AuctionID: auction.ID,
		SellerID:  sellerID,
		Title:     auction.Title,
		Timestamp: time.Now(),
	})

	// Cache
	redisPkg.CacheAuction(auction.ID, auction)

	s.log.Info("auction created",
		zap.String("id", auction.ID),
		zap.String("state", string(state)),
		zap.String("seller_id", sellerID),
	)

	return toResponse(&auction), nil
}

// GetByID returns a single auction
func (s *AuctionService) GetByID(ctx context.Context, id string) (*dto.AuctionResponse, error) {
	auction, err := repository.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return toResponse(auction), nil
}

// Update modifies an auction (only owner, only draft/scheduled)
func (s *AuctionService) Update(ctx context.Context, id, userID string, req dto.UpdateAuctionRequest) (*dto.AuctionResponse, error) {
	auction, err := repository.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}

	if auction.SellerID != userID {
		return nil, ErrForbidden
	}

	if auction.State != model.StateDraft && auction.State != model.StateScheduled {
		return nil, ErrAlreadyStarted
	}

	if req.Title != "" {
		auction.Title = req.Title
	}
	if req.Description != "" {
		auction.Description = req.Description
	}
	if req.ReservePrice > 0 {
		auction.ReservePrice = req.ReservePrice
	}

	if err := repository.Update(ctx, *auction); err != nil {
		return nil, err
	}

	redisPkg.InvalidateAuction(id)
	return toResponse(auction), nil
}

// Delete soft-deletes an auction (owner or admin)
func (s *AuctionService) Delete(ctx context.Context, id, userID, role string) error {
	auction, err := repository.GetByID(ctx, id)
	if err != nil {
		return ErrNotFound
	}

	if auction.SellerID != userID && role != "admin" {
		return ErrForbidden
	}

	if err := repository.SoftDelete(ctx, id); err != nil {
		return err
	}

	redisPkg.InvalidateAuction(id)

	kafka.PublishEvent(kafka.EventAuctionDeleted, kafka.AuctionEvent{
		EventType: kafka.EventAuctionDeleted,
		AuctionID: id,
		Timestamp: time.Now(),
	})

	return nil
}

// List returns paginated auctions
func (s *AuctionService) List(ctx context.Context, state string, page, pageSize int) (*dto.AuctionListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	auctions, total, err := repository.List(ctx, state, pageSize, offset)
	if err != nil {
		return nil, err
	}

	resp := &dto.AuctionListResponse{
		Auctions: make([]dto.AuctionResponse, 0, len(auctions)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, a := range auctions {
		resp.Auctions = append(resp.Auctions, *toResponse(&a))
	}
	return resp, nil
}

// GetBySeller returns all auctions for a seller
func (s *AuctionService) GetBySeller(ctx context.Context, sellerID string) ([]dto.AuctionResponse, error) {
	auctions, err := repository.GetBySeller(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	resp := make([]dto.AuctionResponse, 0, len(auctions))
	for _, a := range auctions {
		resp = append(resp, *toResponse(&a))
	}
	return resp, nil
}

func toResponse(a *model.Auction) *dto.AuctionResponse {
	return &dto.AuctionResponse{
		ID:            a.ID,
		SellerID:      a.SellerID,
		Title:         a.Title,
		Description:   a.Description,
		CategoryID:    a.CategoryID,
		StartingPrice: a.StartingPrice,
		CurrentPrice:  a.CurrentPrice,
		State:         string(a.State),
		StartTime:     a.StartTime,
		EndTime:       a.EndTime,
		WinnerID:      a.WinnerID,
		TotalBids:     a.TotalBids,
		CreatedAt:     a.CreatedAt,
	}
}
