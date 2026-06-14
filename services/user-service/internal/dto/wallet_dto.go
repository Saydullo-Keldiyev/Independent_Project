package dto

type WalletResponse struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

type DepositRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
}

type WalletTransactionResponse struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	Amount        float64 `json:"amount"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	Description   string  `json:"description"`
	CreatedAt     string  `json:"created_at"`
}

type WalletHistoryResponse struct {
	Transactions []WalletTransactionResponse `json:"transactions"`
	Total        int                         `json:"total"`
}
