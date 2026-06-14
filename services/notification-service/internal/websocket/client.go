package websocket

import (
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	redisPkg "github.com/auction-system/notification-service/internal/redis"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

// Client represents a single WebSocket connection for a user
type Client struct {
	UserID string
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	log    *zap.Logger
}

func NewClient(hub *Hub, conn *websocket.Conn, userID string, log *zap.Logger) *Client {
	return &Client{
		UserID: userID,
		conn:   conn,
		send:   make(chan []byte, 256),
		hub:    hub,
		log:    log,
	}
}

// ReadPump reads messages from the client (mostly pong responses)
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		// Refresh online status on each pong
		redisPkg.RefreshOnline(c.UserID)
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Warn("ws read error", zap.String("user_id", c.UserID), zap.Error(err))
			}
			break
		}
	}
}

// WritePump sends messages to the client
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Flush queued messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
