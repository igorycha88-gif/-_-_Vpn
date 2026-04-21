package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
)

type PresetHandler struct {
	routingSvc *services.RoutingService
	sbSvc      *services.SingBoxService
	presetSvc  *services.RoutingService
	logger     *slog.Logger
}

func NewPresetHandler(routingSvc *services.RoutingService, sbSvc *services.SingBoxService, logger *slog.Logger) *PresetHandler {
	return &PresetHandler{routingSvc: routingSvc, sbSvc: sbSvc, presetSvc: routingSvc, logger: logger}
}

func (h *PresetHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.routingSvc.ListRules(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения списка пресетов", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	_ = rules
	JSON(w, http.StatusOK, []interface{}{})
}

func (h *PresetHandler) Apply(w http.ResponseWriter, r *http.Request) {
	id := getPathID(r)
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id пресета не указан")
		return
	}

	result, err := h.routingSvc.ApplyPreset(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "пресет не найден")
			return
		}
		h.logger.Error("ошибка применения пресета", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	_ = h.sbSvc.WriteConfig(r.Context())

	JSON(w, http.StatusOK, result)
}
