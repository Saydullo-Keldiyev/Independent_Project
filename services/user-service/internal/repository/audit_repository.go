package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/auction-system/user-service/internal/database"
)

type AuditRepository struct{}

func NewAuditRepository() *AuditRepository { return &AuditRepository{} }

func (r *AuditRepository) Log(ctx context.Context, userID, action string, metadata map[string]any) error {
	meta, err := json.Marshal(metadata)
	if err != nil {
		meta = []byte("{}")
	}
	_, err = database.DB.Exec(ctx, `
		INSERT INTO audit_logs (id, user_id, action, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5)`,
		uuid.NewString(), nullUUID(userID), action, meta, time.Now())
	return err
}

func nullUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
