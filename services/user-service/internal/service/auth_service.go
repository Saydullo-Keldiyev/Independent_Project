package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/auction-system/user-service/internal/auth"
	"github.com/auction-system/user-service/internal/config"
	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/dto"
	"github.com/auction-system/user-service/internal/kafka"
	"github.com/auction-system/user-service/internal/model"
	"github.com/auction-system/user-service/internal/observability"
	"github.com/auction-system/user-service/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveUser       = errors.New("account is inactive")
	ErrInvalidRefresh     = errors.New("invalid or expired refresh token")
)

type AuthService struct {
	users    *repository.UserRepository
	wallets  *repository.WalletRepository
	sessions *repository.SessionRepository
	refresh  *repository.RefreshRepository
	audit    *repository.AuditRepository
	jwt      *auth.JWTManager
	cfg      *config.Config
}

func NewAuthService(cfg *config.Config) *AuthService {
	return &AuthService{
		users:    repository.NewUserRepository(),
		wallets:  repository.NewWalletRepository(),
		sessions: repository.NewSessionRepository(),
		refresh:  repository.NewRefreshRepository(),
		audit:    repository.NewAuditRepository(),
		jwt:      auth.NewJWTManager(cfg.JWT.Secret, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL),
		cfg:      cfg,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest, ip, ua string) (*dto.AuthResponse, error) {
	role := model.RoleBidder
	if req.Role == string(model.RoleSeller) {
		role = model.RoleSeller
	}

	hash, err := auth.HashPassword(req.Password, s.cfg.Security.BcryptCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Role:         role,
		IsVerified:   false,
		IsActive:     true,
	}

	tx, err := database.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := s.users.Create(ctx, tx, user); err != nil {
		if errors.Is(err, repository.ErrEmailExists) {
			return nil, err
		}
		return nil, err
	}

	wallet, err := s.wallets.Create(ctx, tx, user.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, user.ID, "USER_REGISTERED", map[string]any{"email": user.Email, "role": user.Role})
	_ = kafka.PublishEvent(kafka.UserEvent{
		Type: kafka.EventUserRegistered, UserID: user.ID, Email: user.Email, Role: string(user.Role),
	})
	_ = kafka.PublishEvent(kafka.UserEvent{
		Type: kafka.EventWalletCreated, UserID: user.ID, WalletID: wallet.ID,
	})

	return s.issueTokens(ctx, user, ip, ua)
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, ip, ua string) (*dto.AuthResponse, error) {
	observability.LoginAttemptsTotal.WithLabelValues("attempt").Inc()
	user, err := s.users.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			observability.LoginAttemptsTotal.WithLabelValues("failed").Inc()
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrInactiveUser
	}
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		observability.LoginAttemptsTotal.WithLabelValues("failed").Inc()
		return nil, ErrInvalidCredentials
	}
	observability.LoginAttemptsTotal.WithLabelValues("success").Inc()
	_ = s.audit.Log(ctx, user.ID, "USER_LOGGED_IN", map[string]any{"ip": ip})
	_ = kafka.PublishEvent(kafka.UserEvent{Type: kafka.EventUserLoggedIn, UserID: user.ID, Email: user.Email})
	return s.issueTokens(ctx, user, ip, ua)
}

func (s *AuthService) Refresh(ctx context.Context, plainRefresh string) (*dto.AuthResponse, error) {
	hash := auth.HashToken(plainRefresh)
	rt, err := s.refresh.FindValid(ctx, hash)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	if err := s.refresh.Revoke(ctx, rt.ID); err != nil {
		return nil, err
	}
	user, err := s.users.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, ErrInvalidRefresh
	}
	return s.issueTokens(ctx, user, "", "")
}

func (s *AuthService) Logout(ctx context.Context, accessJTI string, accessTTL time.Duration, plainRefresh string) error {
	if accessJTI != "" {
		_ = auth.BlacklistJTI(ctx, accessJTI, accessTTL)
	}
	if plainRefresh != "" {
		hash := auth.HashToken(plainRefresh)
		if rt, err := s.refresh.FindValid(ctx, hash); err == nil {
			_ = s.refresh.Revoke(ctx, rt.ID)
			_ = s.audit.Log(ctx, rt.UserID, "USER_LOGGED_OUT", nil)
		}
	}
	return nil
}

func (s *AuthService) issueTokens(ctx context.Context, user *model.User, ip, ua string) (*dto.AuthResponse, error) {
	if ip != "" {
		_ = s.sessions.EnsureSessionLimit(ctx, user.ID, s.cfg.Security.MaxSessions)
		_ = s.sessions.Create(ctx, &model.Session{
			UserID: user.ID, IPAddress: ip, UserAgent: ua, DeviceInfo: parseDevice(ua),
		})
	}

	access, _, _, err := s.jwt.IssueAccessToken(user)
	if err != nil {
		return nil, err
	}

	plain, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	if _, err := s.refresh.Create(ctx, user.ID, hash, s.jwt.RefreshExpiresAt()); err != nil {
		return nil, err
	}

	return &dto.AuthResponse{
		AccessToken:  access,
		RefreshToken: plain,
		ExpiresIn:    int64(s.cfg.JWT.AccessTokenTTL * 60),
		TokenType:    "Bearer",
		User:         toUserResponse(user),
	}, nil
}

func (s *AuthService) ParseToken(token string) (*auth.Claims, error) {
	claims, err := s.jwt.ParseAccessToken(token)
	if err != nil {
		observability.JWTValidationErrors.Inc()
		return nil, err
	}
	blacklisted, err := auth.IsBlacklisted(context.Background(), claims.ID)
	if err != nil {
		observability.Log.Warn("blacklist check failed", zap.Error(err))
	}
	if blacklisted {
		observability.JWTValidationErrors.Inc()
		return nil, fmt.Errorf("token revoked")
	}
	return claims, nil
}

func parseDevice(ua string) string {
	if len(ua) > 120 {
		return ua[:120]
	}
	return ua
}

func toUserResponse(u *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID: u.ID, Username: u.Username, Email: u.Email,
		FirstName: u.FirstName, LastName: u.LastName, Role: string(u.Role),
	}
}

// BeginTx helper for wallet operations from other services
func BeginTx(ctx context.Context) (pgx.Tx, error) {
	return database.DB.Begin(ctx)
}
