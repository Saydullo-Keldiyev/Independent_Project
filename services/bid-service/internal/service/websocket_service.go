package service

import (
	"net/http"

	"github.com/gorilla/websocket"

	wsPkg "github.com/auction-system/bid-service/internal/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in dev; restrict in production
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ServeWS upgrades the HTTP connection to WebSocket and registers the client
func ServeWS(w http.ResponseWriter, r *http.Request, auctionID, userID string) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	client := wsPkg.NewClient(wsPkg.H, conn, auctionID, userID)
	wsPkg.H.Register <- client

	// Run read/write pumps in separate goroutines
	go client.WritePump()
	go client.ReadPump()

	return nil
}
