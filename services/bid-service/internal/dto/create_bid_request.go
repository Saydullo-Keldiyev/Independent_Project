package dto

type CreateBidRequest struct {
	AuctionID string  `json:"auction_id" binding:"required,uuid"`
	Amount    float64 `json:"amount"     binding:"required,gt=0"`
}
