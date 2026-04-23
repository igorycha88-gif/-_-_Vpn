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
			var tokenStr string

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
					tokenStr = parts[1]
				}
			}

			if tokenStr == "" {
				tokenStr = r.URL.Query().Get("token")
			}

			if tokenStr == "" {
				http.Error(w, `{"error":"отсутствует токен авторизации"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authSvc.ValidateAccessToken(tokenStr)
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
