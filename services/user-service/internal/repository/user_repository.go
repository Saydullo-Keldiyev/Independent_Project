package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/model"
)

var ErrUserNotFound = errors.New("user not found")
var ErrEmailExists = errors.New("email already registered")
var ErrUsernameExists = errors.New("username already taken")

type UserRepository struct{}

func NewUserRepository() *UserRepository { return &UserRepository{} }

func (r *UserRepository) Create(ctx context.Context, tx pgx.Tx, u *model.User) error {
	u.ID = uuid.NewString()
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	_, err := tx.Exec(ctx, `
		INSERT INTO users (id, username, email, password_hash, first_name, last_name, role, is_verified, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		u.ID, u.Username, u.Email, u.PasswordHash, u.FirstName, u.LastName, u.Role, u.IsVerified, u.IsActive, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrEmailExists
		}
		return err
	}
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	row := database.DB.QueryRow(ctx, `
		SELECT id, username, email, password_hash, first_name, last_name, role, is_verified, is_active, created_at, updated_at, deleted_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`, email)
	return scanUser(row)
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	row := database.DB.QueryRow(ctx, `
		SELECT id, username, email, password_hash, first_name, last_name, role, is_verified, is_active, created_at, updated_at, deleted_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanUser(row)
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id, firstName, lastName string) error {
	ct, err := database.DB.Exec(ctx, `
		UPDATE users SET first_name = $2, last_name = $3, updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`, id, firstName, lastName)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (*model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
		&u.Role, &u.IsVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &u, err
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique")
}
