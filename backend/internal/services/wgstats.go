package services

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type WGStatsCollector struct {
	peerRepo    repository.PeerRepository
	trafficRepo repository.TrafficRepository
	alertSvc    *TrafficService
	logger      *slog.Logger
	iface       string
	interval    time.Duration

	mu       sync.Mutex
	prev     map[string]peerTransfer
	online   map[string]bool
	wgActive bool
}

type peerTransfer struct {
	rx int64
	tx int64
}

func NewWGStatsCollector(
	peerRepo repository.PeerRepository,
	trafficRepo repository.TrafficRepository,
	alertSvc *TrafficService,
	iface string,
	logger *slog.Logger,
) *WGStatsCollector {
	return &WGStatsCollector{
		peerRepo:    peerRepo,
		trafficRepo: trafficRepo,
		alertSvc:    alertSvc,
		logger:      logger,
		iface:       iface,
		interval:    10 * time.Second,
		prev:        make(map[string]peerTransfer),
		online:      make(map[string]bool),
		wgActive:    false,
	}
}

func (c *WGStatsCollector) Start(ctx context.Context) {
	c.logger.Info("запуск сборщика статистики межсерверного тоннеля", "interface", c.iface, "interval", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("остановка сборщика статистики межсерверного тоннеля")
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *WGStatsCollector) IsWGActive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wgActive
}

func (c *WGStatsCollector) collect(ctx context.Context) {
	transferOutput, err := c.runWG("transfer")
	if err != nil {
		c.mu.Lock()
		wasActive := c.wgActive
		c.wgActive = false
		c.mu.Unlock()
		if wasActive {
			c.logger.Warn("межсерверный WG тоннель недоступен", "interface", c.iface, "error", err)
			c.alertSvc.AddAlert(ctx, &models.Alert{
				ID:        fmt.Sprintf("wg-tunnel-down-%d", time.Now().Unix()),
				Type:      "tunnel",
				Message:   "Межсерверный WG тоннель недоступен: " + err.Error(),
				Severity:  "error",
				Timestamp: time.Now(),
			})
		}
		return
	}

	c.mu.Lock()
	if !c.wgActive {
		c.alertSvc.AddAlert(ctx, &models.Alert{
			ID:        fmt.Sprintf("wg-tunnel-up-%d", time.Now().Unix()),
			Type:      "tunnel",
			Message:   "Межсерверный WG тоннель восстановлен",
			Severity:  "info",
			Timestamp: time.Now(),
		})
	}
	c.wgActive = true
	c.mu.Unlock()

	transferMap := c.parseTransfer(transferOutput)

	for pubKey, current := range transferMap {
		prev := c.prev[pubKey]
		deltaRx := current.rx - prev.rx
		deltaTx := current.tx - prev.tx

		if deltaRx < 0 {
			deltaRx = current.rx
		}
		if deltaTx < 0 {
			deltaTx = current.tx
		}

		if deltaRx > 0 || deltaTx > 0 {
			if err := c.trafficRepo.Log(ctx, &models.TrafficLog{
				Action:  "tunnel_transfer",
				BytesRx: deltaRx,
				BytesTx: deltaTx,
			}); err != nil {
				c.logger.Error("ошибка логирования трафика тоннеля", "error", err)
			}
		}

		c.mu.Lock()
		c.prev[pubKey] = current
		c.mu.Unlock()
	}
}

func (c *WGStatsCollector) runWG(mode string) (string, error) {
	cmd := exec.Command("wg", "show", c.iface, mode)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("wg show %s %s: %w: %s", c.iface, mode, err, string(output))
	}
	return string(output), nil
}

func (c *WGStatsCollector) parseTransfer(output string) map[string]peerTransfer {
	result := make(map[string]peerTransfer)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		rx, errRx := strconv.ParseInt(parts[1], 10, 64)
		tx, errTx := strconv.ParseInt(parts[2], 10, 64)
		if errRx != nil || errTx != nil {
			continue
		}
		result[parts[0]] = peerTransfer{rx: rx, tx: tx}
	}
	return result
}
