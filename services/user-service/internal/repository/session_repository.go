package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/model"
)

type SessionRepository struct{}

func NewSessionRepository() *SessionRepository { return &SessionRepository{} }

func (r *SessionRepository) Create(ctx context.Context, s *model.Session) error {
	s.ID = uuid.NewString()
	s.CreatedAt = time.Now()
	s.LastActivity = time.Now()
	_, err := database.DB.Exec(ctx, `
		INSERT INTO user_sessions (id, user_id, ip_address, user_agent, device_info, last_activity, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		s.ID, s.UserID, s.IPAddress, s.UserAgent, s.DeviceInfo, s.LastActivity, s.CreatedAt)
	return err
}

func (r *SessionRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := database.DB.QueryRow(ctx, `SELECT COUNT(*) FROM user_sessions WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

func (r *SessionRepository) DeleteOldest(ctx context.Context, userID string) error {
	_, err := database.DB.Exec(ctx, `
		DELETE FROM user_sessions WHERE id = (
			SELECT id FROM user_sessions WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1
		)`, userID)
	return err
}

func (r *SessionRepository) Touch(ctx context.Context, sessionID string) error {
	_, err := database.DB.Exec(ctx, `UPDATE user_sessions SET last_activity = NOW() WHERE id = $1`, sessionID)
	return err
}

func (r *SessionRepository) ActiveCount(ctx context.Context) (int, error) {
	var n int
	err := database.DB.QueryRow(ctx, `
		SELECT COUNT(*) FROM user_sessions WHERE last_activity > NOW() - INTERVAL '24 hours'`).Scan(&n)
	return n, err
}

// EnsureSessionLimit removes oldest session if user exceeds maxSessions
func (r *SessionRepository) EnsureSessionLimit(ctx context.Context, userID string, maxSessions int) error {
	n, err := r.CountByUser(ctx, userID)
	if err != nil {
		return err
	}
	for n >= maxSessions {
		if err := r.DeleteOldest(ctx, userID); err != nil {
			return err
		}
		n--
	}
	return nil
}
