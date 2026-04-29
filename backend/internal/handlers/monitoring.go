package handlers

import (
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/services"
)

type MonitoringHandler struct {
	trafficSvc  *services.TrafficService
	wgSvc       *services.WireGuardService
	rtProvider  services.RealtimeStatsProvider
	logger      *slog.Logger
}

func NewMonitoringHandler(
	trafficSvc *services.TrafficService,
	wgSvc *services.WireGuardService,
	rtProvider services.RealtimeStatsProvider,
	logger *slog.Logger,
) *MonitoringHandler {
	return &MonitoringHandler{
		trafficSvc:  trafficSvc,
		wgSvc:       wgSvc,
		rtProvider:  rtProvider,
		logger:      logger,
	}
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

func (h *MonitoringHandler) TrafficAggregate(w http.ResponseWriter, r *http.Request) {
	peerID := r.URL.Query().Get("peer_id")

	items, err := h.trafficSvc.GetTrafficAggregate(r.Context(), peerID, 30)
	if err != nil {
		h.logger.Error("ошибка получения агрегации трафика", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, items)
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

	if h.rtProvider != nil {
		rtStats := h.rtProvider.GetRealtimeStats()
		stats.OnlinePeers = len(rtStats)
	}

	h.logger.Debug("monitoring stats", "online", stats.OnlinePeers, "total", stats.TotalPeers, "rx", stats.TotalRx, "tx", stats.TotalTx)

	JSON(w, http.StatusOK, stats)
}

func (h *MonitoringHandler) PeerMonitor(w http.ResponseWriter, r *http.Request) {
	id := getPathID(r)
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

	if h.rtProvider != nil {
		if rt, ok := h.rtProvider.GetRealtimeStats()[id]; ok {
			result["realtime"] = rt
		}
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

	if h.rtProvider != nil {
		rtStats := h.rtProvider.GetRealtimeStats()
		for _, s := range summaries {
			if rt, ok := rtStats[s.PeerID]; ok {
				s.Online = true
				s.ActiveConns = rt.ActiveConnections
				s.BandwidthRateRx = rt.BandwidthRateRx
				s.BandwidthRateTx = rt.BandwidthRateTx
				s.ConnectedAt = rt.ConnectedAt
				s.SessionRx = rt.SessionRx
				s.SessionTx = rt.SessionTx
			} else {
				s.Online = false
			}
		}
	}

	JSON(w, http.StatusOK, summaries)
}
