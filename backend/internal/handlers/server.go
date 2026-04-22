package handlers

import (
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"smarttraffic/internal/models"
	"smarttraffic/internal/services"
)

type ServerHandler struct {
	trafficSvc *services.TrafficService
	collector  *services.WGStatsCollector
	logger     *slog.Logger
}

func NewServerHandler(trafficSvc *services.TrafficService, collector *services.WGStatsCollector, logger *slog.Logger) *ServerHandler {
	return &ServerHandler{trafficSvc: trafficSvc, collector: collector, logger: logger}
}

func (h *ServerHandler) Status(w http.ResponseWriter, r *http.Request) {
	wgActive := h.collector != nil && h.collector.IsWGActive()

	status := &models.ServerStatus{
		RU: models.ServerInfo{
			Online: true,
		},
		Foreign: models.ServerInfo{
			Online: wgActive,
		},
	}

	JSON(w, http.StatusOK, status)
}

func (h *ServerHandler) RUStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.trafficSvc.GetTotalStats(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения статистики РФ-сервера", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	wgStatus := "stopped"
	if h.collector != nil && h.collector.IsWGActive() {
		wgStatus = "running"
	}

	serverStats := &models.ServerStats{
		TotalRx:       stats.TotalRx,
		TotalTx:       stats.TotalTx,
		ActivePeers:   stats.ActivePeers,
		TotalPeers:    stats.TotalPeers,
		WGStatus:      wgStatus,
		SingboxStatus: "running",
	}

	JSON(w, http.StatusOK, serverStats)
}

func (h *ServerHandler) ForeignStats(w http.ResponseWriter, r *http.Request) {
	JSON(w, http.StatusOK, map[string]interface{}{
		"online": true,
		"uptime": time.Now().Format(time.RFC3339),
		"cpu":    "0%",
		"memory": "0%",
	})
}

func (h *ServerHandler) Health(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	JSON(w, http.StatusOK, map[string]interface{}{
		"status":          "ok",
		"memory_alloc_mb": m.Alloc / 1024 / 1024,
		"goroutines":      runtime.NumGoroutine(),
	})
}
