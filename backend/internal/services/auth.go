package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("неверный email или пароль")
	ErrInvalidToken       = errors.New("неверный или просроченный токен")
)

type AuthService struct {
	authRepo repository.AuthRepository
	cfg      *config.JWTConfig
	logger   *slog.Logger
}

func NewAuthService(authRepo repository.AuthRepository, cfg *config.JWTConfig, logger *slog.Logger) *AuthService {
	return &AuthService{
		authRepo: authRepo,
		cfg:      cfg,
		logger:   logger,
	}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*models.TokenPair, error) {
	user, err := s.authRepo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("service.auth.Login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	accessToken, err := s.generateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("service.auth.Login access: %w", err)
	}

	refreshToken := uuid.New().String()
	expiresAt := time.Now().Add(s.cfg.RefreshTTL).Format(time.RFC3339)

	if err := s.authRepo.StoreRefreshToken(ctx, user.ID, refreshToken, expiresAt); err != nil {
		return nil, fmt.Errorf("service.auth.Login store refresh: %w", err)
	}

	s.logger.Info("пользователь авторизован", "email", email)

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
	}, nil
}

func (s *AuthService) ValidateAccessToken(tokenStr string) (*models.Claims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	c := &models.Claims{
		UserID: claims["user_id"].(string),
		Email:  claims["email"].(string),
		Role:   claims["role"].(string),
	}

	return c, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*models.TokenPair, error) {
	userID, err := s.authRepo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	_ = s.authRepo.DeleteRefreshToken(ctx, refreshToken)

	user, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	accessToken, err := s.generateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("service.auth.RefreshToken access: %w", err)
	}

	newRefreshToken := uuid.New().String()
	expiresAt := time.Now().Add(s.cfg.RefreshTTL).Format(time.RFC3339)
	_ = s.authRepo.StoreRefreshToken(ctx, user.ID, newRefreshToken, expiresAt)

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(s.cfg.AccessTTL.Seconds()),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	err := s.authRepo.DeleteRefreshToken(ctx, refreshToken)
	if err != nil {
		return fmt.Errorf("service.auth.Logout: %w", err)
	}
	return nil
}

func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	err := s.authRepo.DeleteUserRefreshTokens(ctx, userID)
	if err != nil {
		return fmt.Errorf("service.auth.LogoutAll: %w", err)
	}
	return nil
}

func (s *AuthService) GetSession(ctx context.Context, userID string) (*models.SessionResponse, error) {
	user, err := s.authRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("service.auth.GetSession: %w", err)
	}
	return &models.SessionResponse{
		UserID: user.ID,
		Email:  user.Email,
		Role:   "admin",
	}, nil
}

func (s *AuthService) generateAccessToken(userID, email string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    "admin",
		"iat":     now.Unix(),
		"exp":     now.Add(s.cfg.AccessTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Secret))
}
