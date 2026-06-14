package model

// SearchRequest holds parsed query parameters
type SearchRequest struct {
	Query    string  `form:"q"`
	Category string  `form:"category"`
	State    string  `form:"state"`
	MinPrice float64 `form:"min_price"`
	MaxPrice float64 `form:"max_price"`
	SortBy   string  `form:"sort_by"` // price_asc, price_desc, newest, ending_soon, most_bids
	Page     int     `form:"page"`
	PageSize int     `form:"page_size"`
}

// SearchResponse is the API response envelope
type SearchResponse struct {
	Hits     []AuctionDocument `json:"hits"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Took     int64             `json:"took_ms"`
}

// SuggestResponse for autocomplete
type SuggestResponse struct {
	Suggestions []string `json:"suggestions"`
}

// TrendingResponse for trending searches/auctions
type TrendingResponse struct {
	Searches []TrendingItem `json:"searches"`
	Auctions []AuctionDocument `json:"auctions,omitempty"`
}

type TrendingItem struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}
