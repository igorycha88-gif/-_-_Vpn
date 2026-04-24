package handlers

import (
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/services"
)

type MonitoringHandler struct {
	trafficSvc *services.TrafficService
	wgSvc      *services.WireGuardService
	logger     *slog.Logger
}

func NewMonitoringHandler(trafficSvc *services.TrafficService, wgSvc *services.WireGuardService, logger *slog.Logger) *MonitoringHandler {
	return &MonitoringHandler{trafficSvc: trafficSvc, wgSvc: wgSvc, logger: logger}
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

type trafficAggregate struct {
	Domain string `json:"domain"`
	RX     int64  `json:"rx"`
	TX     int64  `json:"tx"`
	Count  int    `json:"count"`
}

func (h *MonitoringHandler) TrafficAggregate(w http.ResponseWriter, r *http.Request) {
	filter := models.TrafficFilter{
		PeerID: r.URL.Query().Get("peer_id"),
		Limit:  1000,
	}

	logs, err := h.trafficSvc.GetTrafficLogs(r.Context(), filter)
	if err != nil {
		h.logger.Error("ошибка получения агрегации трафика", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	aggMap := make(map[string]*trafficAggregate)
	for _, l := range logs {
		key := l.Domain
		if key == "" {
			key = l.DestIP
		}
		if key == "" {
			key = "unknown"
		}
		a, ok := aggMap[key]
		if !ok {
			a = &trafficAggregate{Domain: key}
			aggMap[key] = a
		}
		a.RX += l.BytesRx
		a.TX += l.BytesTx
		a.Count++
	}

	result := make([]*trafficAggregate, 0, len(aggMap))
	for _, a := range aggMap {
		result = append(result, a)
	}

	JSON(w, http.StatusOK, result)
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

func (h *MonitoringHandler) PeerMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	peer, err := h.wgSvc.GetPeer(r.Context(), id)
	if err != nil {
		h.logger.Error("ошибка получения пира", "id", id, "error", err)
		ErrorJSON(w, http.StatusNotFound, "клиент не найден")
		return
	}

	filter := models.TrafficFilter{
		PeerID: id,
		Limit:  50,
	}
	logs, err := h.trafficSvc.GetTrafficLogs(r.Context(), filter)
	if err != nil {
		h.logger.Error("ошибка получения логов пира", "id", id, "error", err)
		logs = []*models.TrafficLog{}
	}

	result := map[string]interface{}{
		"peer":         peer,
		"traffic_logs": logs,
	}

	JSON(w, http.StatusOK, result)
}

func (h *MonitoringHandler) PeersStats(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.trafficSvc.GetAllPeerStats(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения статистики по клиентам", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, summaries)
}
