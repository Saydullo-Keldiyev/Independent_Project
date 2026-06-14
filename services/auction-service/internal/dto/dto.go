package dto

import "time"

type CreateAuctionRequest struct {
	Title         string  `json:"title"          binding:"required,min=3,max=255"`
	Description   string  `json:"description"    binding:"required,min=10"`
	CategoryID    string  `json:"category_id"    binding:"omitempty,uuid"`
	StartingPrice float64 `json:"starting_price" binding:"required,gt=0"`
	ReservePrice  float64 `json:"reserve_price"  binding:"omitempty,gte=0"`
	StartTime     string  `json:"start_time"     binding:"required"`
	EndTime       string  `json:"end_time"       binding:"required"`
}

type UpdateAuctionRequest struct {
	Title       string  `json:"title"       binding:"omitempty,min=3,max=255"`
	Description string  `json:"description" binding:"omitempty,min=10"`
	ReservePrice float64 `json:"reserve_price" binding:"omitempty,gte=0"`
}

type AuctionResponse struct {
	ID            string    `json:"id"`
	SellerID      string    `json:"seller_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	CategoryID    *string   `json:"category_id,omitempty"`
	StartingPrice float64   `json:"starting_price"`
	CurrentPrice  float64   `json:"current_price"`
	State         string    `json:"state"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	WinnerID      *string   `json:"winner_id,omitempty"`
	TotalBids     int       `json:"total_bids"`
	CreatedAt     time.Time `json:"created_at"`
}

type AuctionListResponse struct {
	Auctions []AuctionResponse `json:"auctions"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type SearchParams struct {
	Query    string  `form:"q"`
	Category string  `form:"category"`
	State    string  `form:"state"`
	MinPrice float64 `form:"min_price"`
	MaxPrice float64 `form:"max_price"`
	SortBy   string  `form:"sort_by"`
	Page     int     `form:"page"`
	PageSize int     `form:"page_size"`
}
