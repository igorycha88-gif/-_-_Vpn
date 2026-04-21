package repository

import (
	"context"
	"database/sql"
	"fmt"

	"smarttraffic/internal/models"
)

type AuthRepository interface {
	GetUserByEmail(ctx context.Context, email string) (*models.AdminUser, error)
	GetUserByID(ctx context.Context, id string) (*models.AdminUser, error)
	StoreRefreshToken(ctx context.Context, userID, token string, expiresAt string) error
	GetRefreshToken(ctx context.Context, token string) (userID string, err error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteUserRefreshTokens(ctx context.Context, userID string) error
}

type sqliteAuthRepository struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) AuthRepository {
	return &sqliteAuthRepository{db: db}
}

func (r *sqliteAuthRepository) GetUserByEmail(ctx context.Context, email string) (*models.AdminUser, error) {
	q := `SELECT id, email, password_hash, created_at FROM admin_users WHERE email = ?`
	return r.scanUser(r.db.QueryRowContext(ctx, q, email))
}

func (r *sqliteAuthRepository) GetUserByID(ctx context.Context, id string) (*models.AdminUser, error) {
	q := `SELECT id, email, password_hash, created_at FROM admin_users WHERE id = ?`
	return r.scanUser(r.db.QueryRowContext(ctx, q, id))
}

func (r *sqliteAuthRepository) scanUser(row *sql.Row) (*models.AdminUser, error) {
	u := &models.AdminUser{}
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("auth.scanUser: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("auth.scanUser: %w", err)
	}
	return u, nil
}

func (r *sqliteAuthRepository) StoreRefreshToken(ctx context.Context, userID, token, expiresAt string) error {
	q := `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES (?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, userID, token, expiresAt)
	if err != nil {
		return fmt.Errorf("auth.StoreRefreshToken: %w", err)
	}
	return nil
}

func (r *sqliteAuthRepository) GetRefreshToken(ctx context.Context, token string) (string, error) {
	q := `SELECT user_id FROM refresh_tokens WHERE token = ? AND expires_at > datetime('now')`
	var userID string
	err := r.db.QueryRowContext(ctx, q, token).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("auth.GetRefreshToken: %w", ErrNotFound)
	}
	if err != nil {
		return "", fmt.Errorf("auth.GetRefreshToken: %w", err)
	}
	return userID, nil
}

func (r *sqliteAuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM refresh_tokens WHERE token = ?", token)
	if err != nil {
		return fmt.Errorf("auth.DeleteRefreshToken: %w", err)
	}
	return nil
}

func (r *sqliteAuthRepository) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM refresh_tokens WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("auth.DeleteUserRefreshTokens: %w", err)
	}
	return nil
}
