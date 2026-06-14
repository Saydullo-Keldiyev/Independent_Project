package websocket

import (
	"context"
	"encoding/json"
	"sync"

	"go.uber.org/zap"

	redisPkg "github.com/auction-system/notification-service/internal/redis"
)

// Hub manages all WebSocket clients. Supports multi-pod via Redis Pub/Sub.
type Hub struct {
	// clients maps userID → set of connections (user can have multiple tabs)
	clients map[string]map[*Client]bool
	mu      sync.RWMutex

	Register   chan *Client
	Unregister chan *Client
	broadcast  chan *UserMessage

	log *zap.Logger
}

// UserMessage targets a specific user
type UserMessage struct {
	UserID  string `json:"user_id"`
	Payload []byte `json:"payload"`
}

var H *Hub

func NewHub(log *zap.Logger) *Hub {
	h := &Hub{
		clients:    make(map[string]map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		broadcast:  make(chan *UserMessage, 512),
		log:        log,
	}
	H = h
	return h
}

// Run starts the hub event loop. Call in a goroutine.
func (h *Hub) Run(ctx context.Context) {
	// Subscribe to Redis Pub/Sub for cross-pod broadcasts
	go h.subscribeRedis(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case client := <-h.Register:
			h.mu.Lock()
			if _, ok := h.clients[client.UserID]; !ok {
				h.clients[client.UserID] = make(map[*Client]bool)
			}
			h.clients[client.UserID][client] = true
			h.mu.Unlock()

			redisPkg.SetOnline(client.UserID)
			h.log.Debug("ws client connected", zap.String("user_id", client.UserID))

		case client := <-h.Unregister:
			h.mu.Lock()
			if conns, ok := h.clients[client.UserID]; ok {
				if _, exists := conns[client]; exists {
					delete(conns, client)
					close(client.send)
					if len(conns) == 0 {
						delete(h.clients, client.UserID)
						redisPkg.SetOffline(client.UserID)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.deliverLocal(msg)
		}
	}
}

// SendToUser sends a notification to a specific user.
// Publishes via Redis so all pods deliver it.
func (h *Hub) SendToUser(userID string, payload []byte) {
	msg := &UserMessage{UserID: userID, Payload: payload}

	// Publish to Redis for cross-pod delivery
	data, _ := json.Marshal(msg)
	if err := redisPkg.PublishWSMessage(data); err != nil {
		// Fallback: deliver locally only
		h.deliverLocal(msg)
	}
}

// BroadcastAll sends to ALL connected users (use sparingly)
func (h *Hub) BroadcastAll(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conns := range h.clients {
		for client := range conns {
			select {
			case client.send <- payload:
			default:
				// buffer full — skip
			}
		}
	}
}

// deliverLocal sends to users connected to THIS pod only
func (h *Hub) deliverLocal(msg *UserMessage) {
	h.mu.RLock()
	conns, ok := h.clients[msg.UserID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	for client := range conns {
		select {
		case client.send <- msg.Payload:
		default:
			// buffer full — remove stale client
			h.mu.Lock()
			delete(h.clients[msg.UserID], client)
			close(client.send)
			h.mu.Unlock()
		}
	}
}

// subscribeRedis listens for messages from other pods
func (h *Hub) subscribeRedis(ctx context.Context) {
	sub := redisPkg.SubscribeWS(ctx)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case redisMsg, ok := <-ch:
			if !ok {
				return
			}
			var msg UserMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				continue
			}
			h.deliverLocal(&msg)
		}
	}
}

// OnlineCount returns the number of connected users on this pod
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
