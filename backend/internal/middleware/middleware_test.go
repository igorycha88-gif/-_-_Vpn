package middleware

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"smarttraffic/internal/config"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
	"smarttraffic/migrations"
)

func newTestAuthSvc(t *testing.T) *services.AuthService {
	t.Helper()
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return services.NewAuthService(
		repository.NewAuthRepository(db),
		&config.JWTConfig{Secret: "test-secret-key-at-least-32-chars!", AccessTTL: 15 * time.Minute, RefreshTTL: 168 * time.Hour},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	authSvc := newTestAuthSvc(t)
	tokens, _ := authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")

	called := false
	handler := AuthMiddleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		userID, _ := r.Context().Value(UserIDKey).(string)
		if userID == "" {
			t.Error("user_id not in context")
		}
		email, _ := r.Context().Value(EmailKey).(string)
		if email != "admin@smarttraffic.local" {
			t.Errorf("email = %q, want admin@smarttraffic.local", email)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("next handler was not called")
	}
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_NoToken(t *testing.T) {
	authSvc := newTestAuthSvc(t)

	handler := AuthMiddleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach next handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	authSvc := newTestAuthSvc(t)

	handler := AuthMiddleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach next handler")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_QueryToken(t *testing.T) {
	authSvc := newTestAuthSvc(t)
	tokens, _ := authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")

	called := false
	handler := AuthMiddleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test?token="+tokens.AccessToken, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("next handler was not called")
	}
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Result().StatusCode)
	}
}

func TestAuthMiddleware_MalformedAuthHeader(t *testing.T) {
	authSvc := newTestAuthSvc(t)

	handler := AuthMiddleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach next handler")
	}))

	tests := []string{
		"Basic dXNlcjpwYXNz",
		"Bearer",
		"bearer ",
		"SomeOtherScheme token123",
	}
	for _, authHeader := range tests {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", authHeader)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusUnauthorized {
			t.Errorf("Authorization %q: status = %d, want 401", authHeader, w.Result().StatusCode)
		}
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()
	authSvc := services.NewAuthService(
		repository.NewAuthRepository(db),
		&config.JWTConfig{Secret: "test-secret-key-at-least-32-chars!", AccessTTL: 1 * time.Nanosecond, RefreshTTL: 168 * time.Hour},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	tokens, _ := authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")

	time.Sleep(10 * time.Millisecond)

	handler := AuthMiddleware(authSvc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach next handler with expired token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for expired token", w.Result().StatusCode)
	}
}

func TestRateLimiter_AllowsBurst(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, 5)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, w.Result().StatusCode)
		}
	}
}

func TestRateLimiter_BlocksAfterBurst(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, 2)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "5.6.7.8:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "5.6.7.8:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Result().StatusCode)
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := NewRateLimiter(1, time.Second, 1)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = strings.Repeat("1.", i) + "1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("different IP request %d: status = %d, want 200", i+1, w.Result().StatusCode)
		}
	}
}

func TestCORS_Middleware(t *testing.T) {
	handler := CORS("http://localhost:3000,http://localhost:5173")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestLogging_Middleware(t *testing.T) {
	called := false
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Logging middleware should pass through to next handler")
	}
	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Result().StatusCode)
	}
}

func init() {
	_ = (*sql.DB)(nil)
}
