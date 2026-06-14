package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/model"
)

var ErrRefreshNotFound = errors.New("refresh token not found or revoked")

type RefreshRepository struct{}

func NewRefreshRepository() *RefreshRepository { return &RefreshRepository{} }

func (r *RefreshRepository) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (*model.RefreshToken, error) {
	rt := &model.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}
	_, err := database.DB.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt, rt.Revoked, rt.CreatedAt)
	return rt, err
}

func (r *RefreshRepository) FindValid(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	row := database.DB.QueryRow(ctx, `
		SELECT id, user_id, token_hash, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked = FALSE AND expires_at > NOW()`, tokenHash)
	var rt model.RefreshToken
	err := row.Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.Revoked, &rt.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRefreshNotFound
	}
	return &rt, err
}

func (r *RefreshRepository) Revoke(ctx context.Context, id string) error {
	_, err := database.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1`, id)
	return err
}

func (r *RefreshRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := database.DB.Exec(ctx, `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1 AND revoked = FALSE`, userID)
	return err
}
