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

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type SingBoxStatsCollector struct {
	peerRepo    repository.PeerRepository
	trafficRepo repository.TrafficRepository
	alertSvc    *TrafficService
	logger      *slog.Logger
	apiURL      string
	apiSecret   string
	client      *http.Client
	interval    time.Duration

	mu          sync.Mutex
	connState   map[string]*connBytes
	onlinePeers map[string]bool
	apiReachable bool
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
	User        string `json:"user"`
	Host        string `json:"host"`
	Destination string `json:"destination"`
	DstPort     int    `json:"destination_port"`
	Network     string `json:"network"`
}

type userDelta struct {
	rx          int64
	tx          int64
	connections []userConnection
}

type userConnection struct {
	host        string
	destination string
	dstPort     int
	rx          int64
	tx          int64
}

func NewSingBoxStatsCollector(
	peerRepo repository.PeerRepository,
	trafficRepo repository.TrafficRepository,
	alertSvc *TrafficService,
	apiAddr string,
	apiSecret string,
	logger *slog.Logger,
) *SingBoxStatsCollector {
	return &SingBoxStatsCollector{
		peerRepo:     peerRepo,
		trafficRepo:  trafficRepo,
		alertSvc:     alertSvc,
		logger:       logger,
		apiURL:       "http://" + apiAddr,
		apiSecret:    apiSecret,
		client:       &http.Client{Timeout: 5 * time.Second},
		interval:     10 * time.Second,
		connState:    make(map[string]*connBytes),
		onlinePeers:  make(map[string]bool),
		apiReachable: false,
	}
}

func (c *SingBoxStatsCollector) addAlert(alert *models.Alert) {
	if c.alertSvc != nil {
		c.addAlert(alert)
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
		if c.apiReachable {
			c.logger.Error("sing-box Clash API недоступен", "api", c.apiURL, "error", err)
			c.addAlert(&models.Alert{
				ID:        fmt.Sprintf("clash-api-down-%d", time.Now().Unix()),
				Type:      "system",
				Message:   "sing-box Clash API недоступен: " + err.Error(),
				Severity:  "error",
				Timestamp: time.Now(),
			})
			c.apiReachable = false
		}
		return
	}

	if !c.apiReachable {
		c.logger.Info("sing-box Clash API снова доступен", "api", c.apiURL)
		c.addAlert(&models.Alert{
			ID:        fmt.Sprintf("clash-api-up-%d", time.Now().Unix()),
			Type:      "system",
			Message:   "sing-box Clash API снова доступен",
			Severity:  "info",
			Timestamp: time.Now(),
		})
		c.apiReachable = true
	}

	deltas := c.computeDeltas(resp.Connections)

	currentOnline := make(map[string]bool)

	for uuid, delta := range deltas {
		if delta.rx == 0 && delta.tx == 0 {
			continue
		}

		peer, err := c.peerRepo.GetByPublicKey(ctx, uuid)
		if err != nil {
			continue
		}

		currentOnline[peer.ID] = true

		if err := c.peerRepo.UpdateTraffic(ctx, peer.ID, delta.rx, delta.tx); err != nil {
			c.logger.Error("ошибка обновления трафика клиента", "uuid", uuid, "error", err)
			continue
		}

		if err := c.peerRepo.UpdateLastSeen(ctx, peer.ID); err != nil {
			c.logger.Error("ошибка обновления last_seen клиента", "uuid", uuid, "error", err)
		}

		for _, conn := range delta.connections {
			if conn.rx == 0 && conn.tx == 0 {
				continue
			}
			trafficLog := &models.TrafficLog{
				PeerID:  peer.ID,
				BytesRx: conn.rx,
				BytesTx: conn.tx,
				Action:  "vless_transfer",
			}
			if conn.host != "" {
				trafficLog.Domain = conn.host
			} else if conn.destination != "" {
				trafficLog.DestIP = conn.destination
			}
			if conn.dstPort > 0 {
				trafficLog.DestPort = conn.dstPort
			}
			if err := c.trafficRepo.Log(ctx, trafficLog); err != nil {
				c.logger.Error("ошибка логирования трафика клиента в traffic_logs", "uuid", uuid, "error", err)
			}
		}
	}

	for peerID := range currentOnline {
		if !c.onlinePeers[peerID] {
			peer, err := c.peerRepo.GetByID(ctx, peerID)
			if err == nil {
				c.addAlert(&models.Alert{
					ID:        fmt.Sprintf("peer-online-%s-%d", peerID, time.Now().Unix()),
					Type:      "peer",
					Message:   "Клиент подключился: " + peer.Name,
					Severity:  "info",
					Timestamp: time.Now(),
				})
			}
		}
	}
	for peerID := range c.onlinePeers {
		if !currentOnline[peerID] {
			peer, err := c.peerRepo.GetByID(ctx, peerID)
			if err == nil {
				c.addAlert(&models.Alert{
					ID:        fmt.Sprintf("peer-offline-%s-%d", peerID, time.Now().Unix()),
					Type:      "peer",
					Message:   "Клиент отключился: " + peer.Name,
					Severity:  "warning",
					Timestamp: time.Now(),
				})
			}
		}
	}
	c.onlinePeers = currentOnline

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
			d.connections = append(d.connections, userConnection{
				host:        conn.Metadata.Host,
				destination: conn.Metadata.Destination,
				dstPort:     conn.Metadata.DstPort,
				rx:          drx,
				tx:          dtx,
			})
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
