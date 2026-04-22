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
	c.logger.Info("запуск сборщика WG статистики", "interface", c.iface, "interval", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("остановка сборщика WG статистики")
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
			c.logger.Warn("WG интерфейс недоступен", "interface", c.iface, "error", err)
		}
		return
	}

	c.mu.Lock()
	c.wgActive = true
	c.mu.Unlock()

	handshakeOutput, _ := c.runWG("latest-handshakes")

	transferMap := c.parseTransfer(transferOutput)
	handshakeMap := c.parseHandshakes(handshakeOutput)

	peers, err := c.peerRepo.List(ctx)
	if err != nil {
		c.logger.Error("ошибка получения списка пиров", "error", err)
		return
	}

	for _, peer := range peers {
		if !peer.IsActive {
			continue
		}

		current, hasTransfer := transferMap[peer.PublicKey]
		handshakeAge, hasHandshake := handshakeMap[peer.PublicKey]

		isOnline := hasHandshake && handshakeAge <= 120

		c.checkAlert(peer.ID, peer.Name, isOnline)

		if hasTransfer {
			prev := c.prev[peer.PublicKey]
			deltaRx := current.rx - prev.rx
			deltaTx := current.tx - prev.tx

			if deltaRx < 0 {
				deltaRx = current.rx
			}
			if deltaTx < 0 {
				deltaTx = current.tx
			}

			if deltaRx > 0 || deltaTx > 0 {
				if err := c.peerRepo.UpdateTraffic(ctx, peer.ID, deltaRx, deltaTx); err != nil {
					c.logger.Error("ошибка обновления трафика пира", "id", peer.ID, "error", err)
				}

				if err := c.trafficRepo.Log(ctx, &models.TrafficLog{
					PeerID:  peer.ID,
					Action:  "transfer",
					BytesRx: deltaRx,
					BytesTx: deltaTx,
				}); err != nil {
					c.logger.Error("ошибка логирования трафика", "id", peer.ID, "error", err)
				}
			}

			c.mu.Lock()
			c.prev[peer.PublicKey] = current
			c.mu.Unlock()
		}

		if isOnline {
			if err := c.peerRepo.UpdateLastSeen(ctx, peer.ID); err != nil {
				c.logger.Error("ошибка обновления last_seen", "id", peer.ID, "error", err)
			}
		}
	}
}

func (c *WGStatsCollector) checkAlert(peerID, peerName string, isOnline bool) {
	c.mu.Lock()
	prevOnline, existed := c.online[peerID]
	c.online[peerID] = isOnline
	c.mu.Unlock()

	if !existed {
		return
	}

	if prevOnline && !isOnline {
		c.alertSvc.AddAlert(&models.Alert{
			ID:        fmt.Sprintf("peer-offline-%s-%d", peerID, time.Now().Unix()),
			Type:      "peer_offline",
			Message:   fmt.Sprintf("Клиент \"%s\" отключился", peerName),
			Severity:  "warning",
			Timestamp: time.Now(),
		})
		c.logger.Info("клиент отключился", "name", peerName, "id", peerID)
	}

	if !prevOnline && isOnline {
		c.alertSvc.AddAlert(&models.Alert{
			ID:        fmt.Sprintf("peer-online-%s-%d", peerID, time.Now().Unix()),
			Type:      "peer_online",
			Message:   fmt.Sprintf("Клиент \"%s\" подключился", peerName),
			Severity:  "info",
			Timestamp: time.Now(),
		})
		c.logger.Info("клиент подключился", "name", peerName, "id", peerID)
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

func (c *WGStatsCollector) parseHandshakes(output string) map[string]int64 {
	result := make(map[string]int64)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ts, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		if ts == 0 {
			result[parts[0]] = 999999
			continue
		}
		age := time.Now().Unix() - ts
		result[parts[0]] = age
	}
	return result
}
