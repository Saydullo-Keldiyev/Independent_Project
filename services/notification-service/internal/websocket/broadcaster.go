package websocket

import (
	"encoding/json"
	"time"
)

// WSNotification is the payload sent to WebSocket clients
type WSNotification struct {
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	AuctionID string    `json:"auction_id,omitempty"`
	Amount    float64   `json:"amount,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// NotifyUser sends a real-time notification to a specific user via WebSocket
func NotifyUser(userID string, notif WSNotification) error {
	if H == nil {
		return nil
	}

	payload, err := json.Marshal(notif)
	if err != nil {
		return err
	}

	H.SendToUser(userID, payload)
	return nil
}
