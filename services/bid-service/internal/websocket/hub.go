package websocket

import (
	"log"
	"sync"

	"github.com/auction-system/bid-service/internal/observability"
)

// Hub maintains the set of active clients and broadcasts messages.
// It is safe for concurrent use.
type Hub struct {
	// rooms maps auctionID -> set of clients
	rooms map[string]map[*Client]bool

	// broadcast receives messages to send to a specific auction room
	broadcast chan *RoomMessage

	// Register receives new clients (exported for external use)
	Register chan *Client

	// Unregister receives clients to remove (exported for external use)
	Unregister chan *Client

	mu sync.RWMutex
}

type RoomMessage struct {
	AuctionID string
	Payload   []byte
}

var H = NewHub()

func NewHub() *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]bool),
		broadcast:  make(chan *RoomMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// Run starts the hub event loop. Must be called in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			if _, ok := h.rooms[client.AuctionID]; !ok {
				h.rooms[client.AuctionID] = make(map[*Client]bool)
			}
			h.rooms[client.AuctionID][client] = true
			h.mu.Unlock()
			observability.WebSocketConnections.Inc()
			log.Printf("Client registered for auction %s. Total: %d", client.AuctionID, len(h.rooms[client.AuctionID]))

		case client := <-h.Unregister:
			h.mu.Lock()
			if room, ok := h.rooms[client.AuctionID]; ok {
				if _, ok := room[client]; ok {
					delete(room, client)
					close(client.send)
					if len(room) == 0 {
						delete(h.rooms, client.AuctionID)
					}
					observability.WebSocketConnections.Dec()
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			room, ok := h.rooms[msg.AuctionID]
			h.mu.RUnlock()

			if ok {
				for client := range room {
					select {
					case client.send <- msg.Payload:
					default:
						// Client send buffer full — remove it
						h.mu.Lock()
						delete(h.rooms[msg.AuctionID], client)
						close(client.send)
						h.mu.Unlock()
					}
				}
			}
		}
	}
}

// BroadcastToAuction sends a message to all clients watching an auction
func (h *Hub) BroadcastToAuction(auctionID string, payload []byte) {
	h.broadcast <- &RoomMessage{
		AuctionID: auctionID,
		Payload:   payload,
	}
}
