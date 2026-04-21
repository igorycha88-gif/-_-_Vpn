package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
)

type RouteHandler struct {
	routingSvc *services.RoutingService
	sbSvc      *services.SingBoxService
	logger     *slog.Logger
}

func NewRouteHandler(routingSvc *services.RoutingService, sbSvc *services.SingBoxService, logger *slog.Logger) *RouteHandler {
	return &RouteHandler{routingSvc: routingSvc, sbSvc: sbSvc, logger: logger}
}

func (h *RouteHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.routingSvc.ListRules(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения списка правил", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	if rules == nil {
		rules = []*models.RoutingRule{}
	}
	JSON(w, http.StatusOK, rules)
}

func (h *RouteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.RoutingRuleCreateRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		JSON(w, http.StatusBadRequest, map[string]interface{}{"errors": errs})
		return
	}

	rule, err := h.routingSvc.CreateRule(r.Context(), &req)
	if err != nil {
		h.logger.Error("ошибка создания правила", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	_ = h.sbSvc.WriteConfig(r.Context())

	JSON(w, http.StatusCreated, rule)
}

func (h *RouteHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := getPathID(r)
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	rule, err := h.routingSvc.GetRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "правило не найдено")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, rule)
}

func (h *RouteHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := getPathID(r)
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	var req models.RoutingRuleUpdateRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	rule, err := h.routingSvc.UpdateRule(r.Context(), id, &req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "правило не найдено")
			return
		}
		h.logger.Error("ошибка обновления правила", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	_ = h.sbSvc.WriteConfig(r.Context())

	JSON(w, http.StatusOK, rule)
}

func (h *RouteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := getPathID(r)
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	if err := h.routingSvc.DeleteRule(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "правило не найдено")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	_ = h.sbSvc.WriteConfig(r.Context())

	JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *RouteHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req models.ReorderRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		JSON(w, http.StatusBadRequest, map[string]interface{}{"errors": errs})
		return
	}

	if err := h.routingSvc.ReorderRules(r.Context(), &req); err != nil {
		h.logger.Error("ошибка переупорядочивания правил", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	_ = h.sbSvc.WriteConfig(r.Context())

	JSON(w, http.StatusOK, map[string]string{"status": "reordered"})
}

func getPathID(r *http.Request) string {
	if id := r.PathValue("id"); id != "" {
		return id
	}
	return r.URL.Query().Get(":id")
}
