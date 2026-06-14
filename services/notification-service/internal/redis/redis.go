package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Client *redis.Client
var Ctx = context.Background()

type Options struct {
	Addr     string
	Password string
	DB       int
}

func Connect(opts Options) error {
	Client = redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		PoolSize:     15,
		MinIdleConns: 3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(Ctx, 5*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to redis: %w", err)
	}
	return nil
}

func Close() {
	if Client != nil {
		Client.Close()
	}
}

// ── Online/Offline presence tracking ──────────────────────────────────────────

const onlinePrefix = "online:user:"
const onlineTTL = 5 * time.Minute

// SetOnline marks a user as online (call on WS connect)
func SetOnline(userID string) error {
	return Client.Set(Ctx, onlinePrefix+userID, "1", onlineTTL).Err()
}

// SetOffline removes online status (call on WS disconnect)
func SetOffline(userID string) error {
	return Client.Del(Ctx, onlinePrefix+userID).Err()
}

// IsOnline checks if a user is currently connected
func IsOnline(userID string) bool {
	val, err := Client.Exists(Ctx, onlinePrefix+userID).Result()
	return err == nil && val > 0
}

// RefreshOnline extends the TTL (call periodically from WS ping)
func RefreshOnline(userID string) error {
	return Client.Expire(Ctx, onlinePrefix+userID, onlineTTL).Err()
}

// ── Idempotency check ─────────────────────────────────────────────────────────

const processedPrefix = "notif:processed:"
const processedTTL = 24 * time.Hour

// MarkProcessed marks an event as processed (idempotency)
func MarkProcessed(eventID string) error {
	return Client.Set(Ctx, processedPrefix+eventID, "1", processedTTL).Err()
}

// IsProcessed checks if an event was already processed
func IsProcessed(eventID string) bool {
	val, err := Client.Exists(Ctx, processedPrefix+eventID).Result()
	return err == nil && val > 0
}

// ── Pub/Sub for WebSocket scaling across pods ─────────────────────────────────

const wsBroadcastChannel = "ws:notifications"

// PublishWSMessage publishes a notification to all pods via Redis Pub/Sub
func PublishWSMessage(payload []byte) error {
	return Client.Publish(Ctx, wsBroadcastChannel, payload).Err()
}

// SubscribeWS returns a Pub/Sub subscription for WebSocket broadcasts
func SubscribeWS(ctx context.Context) *redis.PubSub {
	return Client.Subscribe(ctx, wsBroadcastChannel)
}
