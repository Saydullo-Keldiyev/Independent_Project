package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var (
	baseURL    string
	httpClient = &http.Client{Timeout: 5 * time.Second}
)

// Init sets the user-service base URL for wallet operations
func Init(userServiceURL string) {
	baseURL = userServiceURL
}

// Hold reserves funds from a user's wallet
func Hold(ctx context.Context, userID string, amount float64, ref string) error {
	return callWalletAPI(ctx, "/api/v1/internal/wallet/hold", map[string]any{
		"user_id": userID,
		"amount":  amount,
		"ref":     ref,
	})
}

// Release returns held funds to a user's wallet
func Release(ctx context.Context, userID string, amount float64, ref string) error {
	return callWalletAPI(ctx, "/api/v1/internal/wallet/release", map[string]any{
		"user_id": userID,
		"amount":  amount,
		"ref":     ref,
	})
}

// Settle finalizes winner's hold into a charge
func Settle(ctx context.Context, userID string, amount float64, auctionID string) error {
	return callWalletAPI(ctx, "/api/v1/internal/wallet/settle", map[string]any{
		"user_id":    userID,
		"amount":     amount,
		"auction_id": auctionID,
	})
}

// Credit adds funds to seller's wallet
func Credit(ctx context.Context, userID string, amount float64, auctionID string) error {
	return callWalletAPI(ctx, "/api/v1/internal/wallet/credit", map[string]any{
		"user_id":    userID,
		"amount":     amount,
		"auction_id": auctionID,
	})
}

func callWalletAPI(ctx context.Context, path string, body map[string]any) error {
	if baseURL == "" {
		return nil // wallet integration disabled — skip silently
	}

	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("wallet service unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		if errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("wallet service returned %d", resp.StatusCode)
	}

	return nil
}
