package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"smarttraffic/internal/repository"
)

type SingBoxStatsCollector struct {
	peerRepo    repository.PeerRepository
	trafficRepo repository.TrafficRepository
	logger      *slog.Logger
	apiURL      string
	apiSecret   string
	client      *http.Client
	interval    time.Duration

	mu        sync.Mutex
	connState map[string]*connBytes
}

type connBytes struct {
	upload   int64
	download int64
}

type clashConnectionsResponse struct {
	Connections []clashConnection `json:"connections"`
}

type clashConnection struct {
	ID       string        `json:"id"`
	Upload   int64         `json:"upload"`
	Download int64         `json:"download"`
	Metadata clashMetadata `json:"metadata"`
}

type clashMetadata struct {
	User string `json:"user"`
}

type userDelta struct {
	rx int64
	tx int64
}

func NewSingBoxStatsCollector(
	peerRepo repository.PeerRepository,
	trafficRepo repository.TrafficRepository,
	apiAddr string,
	apiSecret string,
	logger *slog.Logger,
) *SingBoxStatsCollector {
	return &SingBoxStatsCollector{
		peerRepo:    peerRepo,
		trafficRepo: trafficRepo,
		logger:      logger,
		apiURL:      "http://" + apiAddr,
		apiSecret:   apiSecret,
		client:      &http.Client{Timeout: 5 * time.Second},
		interval:    10 * time.Second,
		connState:   make(map[string]*connBytes),
	}
}

func (c *SingBoxStatsCollector) Start(ctx context.Context) {
	c.logger.Info("запуск сборщика статистики VLESS-клиентов", "api", c.apiURL, "interval", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("остановка сборщика статистики VLESS-клиентов")
			return
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

func (c *SingBoxStatsCollector) collect(ctx context.Context) {
	resp, err := c.fetchConnections()
	if err != nil {
		c.logger.Warn("не удалось получить соединения от sing-box Clash API", "error", err)
		return
	}

	deltas := c.computeDeltas(resp.Connections)

	for uuid, delta := range deltas {
		if delta.rx == 0 && delta.tx == 0 {
			continue
		}

		peer, err := c.peerRepo.GetByPublicKey(ctx, uuid)
		if err != nil {
			continue
		}

		if err := c.peerRepo.UpdateTraffic(ctx, peer.ID, delta.rx, delta.tx); err != nil {
			c.logger.Error("ошибка обновления трафика клиента", "uuid", uuid, "error", err)
			continue
		}

		if err := c.peerRepo.UpdateLastSeen(ctx, peer.ID); err != nil {
			c.logger.Error("ошибка обновления last_seen клиента", "uuid", uuid, "error", err)
		}
	}

	c.cleanupStaleConnections(resp.Connections)
}

func (c *SingBoxStatsCollector) computeDeltas(connections []clashConnection) map[string]*userDelta {
	deltas := make(map[string]*userDelta)

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, conn := range connections {
		if conn.Metadata.User == "" {
			c.connState[conn.ID] = &connBytes{upload: conn.Upload, download: conn.Download}
			continue
		}

		prev, exists := c.connState[conn.ID]

		var drx, dtx int64
		if exists {
			drx = conn.Download - prev.download
			dtx = conn.Upload - prev.upload
		} else {
			drx = conn.Download
			dtx = conn.Upload
		}

		if drx < 0 {
			drx = 0
		}
		if dtx < 0 {
			dtx = 0
		}

		if drx > 0 || dtx > 0 {
			d, ok := deltas[conn.Metadata.User]
			if !ok {
				d = &userDelta{}
				deltas[conn.Metadata.User] = d
			}
			d.rx += drx
			d.tx += dtx
		}

		c.connState[conn.ID] = &connBytes{upload: conn.Upload, download: conn.Download}
	}

	return deltas
}

func (c *SingBoxStatsCollector) cleanupStaleConnections(connections []clashConnection) {
	activeConns := make(map[string]bool, len(connections))
	for _, conn := range connections {
		activeConns[conn.ID] = true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for id := range c.connState {
		if !activeConns[id] {
			delete(c.connState, id)
		}
	}
}

func (c *SingBoxStatsCollector) fetchConnections() (*clashConnectionsResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, c.apiURL+"/connections", nil)
	if err != nil {
		return nil, fmt.Errorf("создание запроса: %w", err)
	}

	if c.apiSecret != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiSecret)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("выполнение запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("статус %d от sing-box Clash API", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("чтение ответа: %w", err)
	}

	var result clashConnectionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("парсинг JSON: %w", err)
	}

	return &result, nil
}
