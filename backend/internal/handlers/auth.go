package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/services"
)

type AuthHandler struct {
	svc    *services.AuthService
	logger *slog.Logger
}

func NewAuthHandler(svc *services.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, logger: logger}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		JSON(w, http.StatusBadRequest, map[string]interface{}{"errors": errs})
		return
	}

	tokens, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			ErrorJSON(w, http.StatusUnauthorized, "неверный email или пароль")
			return
		}
		h.logger.Error("ошибка авторизации", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshTokenRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		h.logger.Error("ошибка logout", "error", err)
	}

	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "не авторизован")
		return
	}

	if err := h.svc.LogoutAll(r.Context(), userID); err != nil {
		h.logger.Error("ошибка logout all", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value("user_id").(string)
	if userID == "" {
		ErrorJSON(w, http.StatusUnauthorized, "не авторизован")
		return
	}

	session, err := h.svc.GetSession(r.Context(), userID)
	if err != nil {
		ErrorJSON(w, http.StatusUnauthorized, "сессия не найдена")
		return
	}

	JSON(w, http.StatusOK, session)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshTokenRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	tokens, err := h.svc.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, services.ErrInvalidToken) {
			ErrorJSON(w, http.StatusUnauthorized, "неверный или просроченный refresh token")
			return
		}
		h.logger.Error("ошибка refresh token", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, tokens)
}
