package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
)

type DNSHandler struct {
	dnsSvc *services.DNSService
	logger *slog.Logger
}

func NewDNSHandler(dnsSvc *services.DNSService, logger *slog.Logger) *DNSHandler {
	return &DNSHandler{dnsSvc: dnsSvc, logger: logger}
}

func (h *DNSHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.dnsSvc.GetSettings(r.Context())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "настройки DNS не найдены")
			return
		}
		h.logger.Error("ошибка получения DNS настроек", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, settings)
}

func (h *DNSHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req models.DNSSettingsUpdateRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	settings, err := h.dnsSvc.UpdateSettings(r.Context(), &req)
	if err != nil {
		h.logger.Error("ошибка обновления DNS настроек", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, settings)
}
