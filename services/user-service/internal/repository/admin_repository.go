package repository

import (
	"context"
	"fmt"

	"github.com/auction-system/user-service/internal/database"
	"github.com/auction-system/user-service/internal/model"
)

type AdminRepository struct{}

func NewAdminRepository() *AdminRepository { return &AdminRepository{} }

// ListUsers returns paginated users with optional filters
func (r *AdminRepository) ListUsers(ctx context.Context, page, limit int, role, search string) ([]model.User, int, error) {
	offset := (page - 1) * limit

	// Build WHERE clause
	where := "deleted_at IS NULL"
	args := []any{}
	argIdx := 1

	if role != "" {
		where += fmt.Sprintf(" AND role = $%d", argIdx)
		args = append(args, role)
		argIdx++
	}
	if search != "" {
		where += fmt.Sprintf(" AND (username ILIKE $%d OR email ILIKE $%d OR first_name ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	// Count
	var total int
	countQuery := "SELECT COUNT(*) FROM users WHERE " + where
	if err := database.DB.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch
	query := fmt.Sprintf(`
		SELECT id, username, email, password_hash, first_name, last_name, role, is_verified, is_active, created_at, updated_at, deleted_at
		FROM users WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1)

	args = append(args, limit, offset)
	rows, err := database.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.FirstName, &u.LastName,
			&u.Role, &u.IsVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt, &u.DeletedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	return users, total, rows.Err()
}

// UpdateRole changes a user's role
func (r *AdminRepository) UpdateRole(ctx context.Context, userID string, role model.Role) error {
	ct, err := database.DB.Exec(ctx, `
		UPDATE users SET role = $2, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, userID, role)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SetActive bans or unbans a user
func (r *AdminRepository) SetActive(ctx context.Context, userID string, active bool) error {
	ct, err := database.DB.Exec(ctx, `
		UPDATE users SET is_active = $2, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, userID, active)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// SoftDelete marks a user as deleted
func (r *AdminRepository) SoftDelete(ctx context.Context, userID string) error {
	ct, err := database.DB.Exec(ctx, `
		UPDATE users SET deleted_at = NOW(), is_active = false, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}
