package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
)

type PresetHandler struct {
	routingSvc *services.RoutingService
	presetSvc  *services.RoutingService
	sbSvc      *services.SingBoxService
	presetRepo repository.PresetRepository
	logger     *slog.Logger
}

func NewPresetHandler(routingSvc *services.RoutingService, sbSvc *services.SingBoxService, presetRepo repository.PresetRepository, logger *slog.Logger) *PresetHandler {
	return &PresetHandler{routingSvc: routingSvc, presetSvc: routingSvc, sbSvc: sbSvc, presetRepo: presetRepo, logger: logger}
}

func (h *PresetHandler) List(w http.ResponseWriter, r *http.Request) {
	presets, err := h.presetRepo.List(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения списка пресетов", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	if presets == nil {
		presets = []*models.Preset{}
	}
	JSON(w, http.StatusOK, presets)
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

	_ = h.sbSvc.WriteConfigAndReload(r.Context())

	JSON(w, http.StatusOK, result)
}
