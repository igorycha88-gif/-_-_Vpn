package handlers

import (
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/services"
)

type MonitoringHandler struct {
	trafficSvc *services.TrafficService
	logger     *slog.Logger
}

func NewMonitoringHandler(trafficSvc *services.TrafficService, logger *slog.Logger) *MonitoringHandler {
	return &MonitoringHandler{trafficSvc: trafficSvc, logger: logger}
}

func (h *MonitoringHandler) Traffic(w http.ResponseWriter, r *http.Request) {
	filter := models.TrafficFilter{
		PeerID: r.URL.Query().Get("peer_id"),
		Limit:  100,
	}

	logs, err := h.trafficSvc.GetTrafficLogs(r.Context(), filter)
	if err != nil {
		h.logger.Error("ошибка получения логов трафика", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if logs == nil {
		logs = []*models.TrafficLog{}
	}

	JSON(w, http.StatusOK, logs)
}

func (h *MonitoringHandler) Logs(w http.ResponseWriter, r *http.Request) {
	filter := models.TrafficFilter{
		PeerID: r.URL.Query().Get("peer_id"),
		Limit:  200,
	}

	logs, err := h.trafficSvc.GetTrafficLogs(r.Context(), filter)
	if err != nil {
		h.logger.Error("ошибка получения логов маршрутизации", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if logs == nil {
		logs = []*models.TrafficLog{}
	}

	JSON(w, http.StatusOK, logs)
}

func (h *MonitoringHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := h.trafficSvc.GetAlerts(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения алертов", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if alerts == nil {
		alerts = []*models.Alert{}
	}

	JSON(w, http.StatusOK, alerts)
}

func (h *MonitoringHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.trafficSvc.GetTotalStats(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения общей статистики", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, stats)
}
