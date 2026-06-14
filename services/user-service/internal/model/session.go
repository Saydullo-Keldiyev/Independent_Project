package model

import "time"

type Session struct {
	ID           string    `db:"id"`
	UserID       string    `db:"user_id"`
	IPAddress    string    `db:"ip_address"`
	UserAgent    string    `db:"user_agent"`
	DeviceInfo   string    `db:"device_info"`
	LastActivity time.Time `db:"last_activity"`
	CreatedAt    time.Time `db:"created_at"`
}

type RefreshToken struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	Revoked   bool      `db:"revoked"`
	CreatedAt time.Time `db:"created_at"`
}

type AuditLog struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	Action    string    `db:"action"`
	Metadata  string    `db:"metadata"` // JSON
	CreatedAt time.Time `db:"created_at"`
}
