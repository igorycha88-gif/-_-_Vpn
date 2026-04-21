package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"smarttraffic/internal/services"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	EmailKey  contextKey = "email"
	RoleKey   contextKey = "role"
)

func AuthMiddleware(authSvc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"отсутствует токен авторизации"}`, http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				http.Error(w, `{"error":"неверный формат токена"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authSvc.ValidateAccessToken(parts[1])
			if err != nil {
				if errors.Is(err, services.ErrInvalidToken) {
					http.Error(w, `{"error":"неверный или просроченный токен"}`, http.StatusUnauthorized)
					return
				}
				http.Error(w, `{"error":"ошибка авторизации"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, EmailKey, claims.Email)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)
			ctx = context.WithValue(ctx, "user_id", claims.UserID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
