package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

const aggregateVLESSKey = "__aggregate_vless__"

type RealtimeStatsProvider interface {
	GetRealtimeStats() map[string]*models.PeerRealtimeStats
}

type SingBoxStatsCollector struct {
	peerRepo    repository.PeerRepository
	trafficRepo repository.TrafficRepository
	alertSvc    *TrafficService
	logger      *slog.Logger
	apiURL      string
	apiSecret   string
	client      *http.Client
	interval    time.Duration

	mu               sync.Mutex
	connState        map[string]*connBytes
	onlinePeers      map[string]bool
	apiReachable     bool
	peerRealtime     map[string]*models.PeerRealtimeStats
	peerSessions     map[string]*peerSessionInfo
}

type peerSessionInfo struct {
	sessionID  int64
	startTime  time.Time
	sessionRx  int64
	sessionTx  int64
	connCount  int
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
	User          string `json:"user"`
	Host          string `json:"host"`
	Destination   string `json:"destination"`
	DestinationIP string `json:"destinationIP"`
	DstPort       string `json:"destinationPort"`
	Network       string `json:"network"`
	SourceIP      string `json:"sourceIP"`
	SourcePort    string `json:"sourcePort"`
	Type          string `json:"type"`
}

type userDelta struct {
	rx          int64
	tx          int64
	connCount   int
	connections []userConnection
}

type userConnection struct {
	host        string
	destination string
	dstPort     string
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
		peerRealtime: make(map[string]*models.PeerRealtimeStats),
		peerSessions: make(map[string]*peerSessionInfo),
		apiReachable: false,
	}
}

func (c *SingBoxStatsCollector) GetRealtimeStats() map[string]*models.PeerRealtimeStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[string]*models.PeerRealtimeStats, len(c.peerRealtime))
	for k, v := range c.peerRealtime {
		cp := *v
		result[k] = &cp
	}
	return result
}

func (c *SingBoxStatsCollector) addAlert(ctx context.Context, alert *models.Alert) {
	if c.alertSvc != nil {
		c.alertSvc.AddAlert(ctx, alert)
	}
}

func (c *SingBoxStatsCollector) Start(ctx context.Context) {
	c.logger.Info("запуск сборщика статистики VLESS-клиентов", "api", c.apiURL, "interval", c.interval)

	defer func() {
		if r := recover(); r != nil {
			c.logger.Error("PANIC в SingBoxStatsCollector", "error", r)
		}
	}()

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
		c.logger.Error("sing-box Clash API ошибка", "api", c.apiURL, "error", err, "was_reachable", c.apiReachable)
		if c.apiReachable {
			c.addAlert(ctx, &models.Alert{
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
		c.logger.Info("sing-box Clash API снова доступен", "api", c.apiURL, "connections", len(resp.Connections))
		c.addAlert(ctx, &models.Alert{
			ID:        fmt.Sprintf("clash-api-up-%d", time.Now().Unix()),
			Type:      "system",
			Message:   "sing-box Clash API снова доступен",
			Severity:  "info",
			Timestamp: time.Now(),
		})
		c.apiReachable = true
	}

	c.logger.Debug("получены соединения от Clash API", "count", len(resp.Connections))

	deltas := c.computeDeltas(resp.Connections)
	peerConnCounts := c.countConnectionsPerPeer(resp.Connections)

	currentOnline := make(map[string]bool)

	if aggDelta, ok := deltas[aggregateVLESSKey]; ok {
		delete(deltas, aggregateVLESSKey)
		c.handleAggregateVLESS(ctx, aggDelta, currentOnline, peerConnCounts)
	}

	intervalSec := c.interval.Seconds()

	for uuid, delta := range deltas {
		peer, err := c.peerRepo.GetByPublicKey(ctx, uuid)
		if err != nil {
			c.logger.Warn("UUID из Clash API не найден в БД", "uuid", uuid, "error", err)
			continue
		}

		currentOnline[peer.ID] = true

		if delta.rx > 0 || delta.tx > 0 {
			if err := c.peerRepo.UpdateTraffic(ctx, peer.ID, delta.rx, delta.tx); err != nil {
				c.logger.Error("ошибка обновления трафика клиента", "uuid", uuid, "error", err)
				continue
			}
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
			if conn.dstPort != "" {
				if p, err := strconv.Atoi(conn.dstPort); err == nil {
					trafficLog.DestPort = p
				}
			}
			if err := c.trafficRepo.Log(ctx, trafficLog); err != nil {
				c.logger.Error("ошибка логирования трафика клиента в traffic_logs", "uuid", uuid, "error", err)
			}
		}

		activeConns := peerConnCounts[peer.ID]
		c.updatePeerRealtime(peer.ID, delta, activeConns, intervalSec, currentOnline[peer.ID])
	}

	for peerID := range currentOnline {
		if !c.onlinePeers[peerID] {
			peer, err := c.peerRepo.GetByID(ctx, peerID)
			if err == nil {
				c.addAlert(ctx, &models.Alert{
					ID:        fmt.Sprintf("peer-online-%s-%d", peerID, time.Now().Unix()),
					Type:      "peer",
					Message:   "Клиент подключился: " + peer.Name,
					Severity:  "info",
					Timestamp: time.Now(),
				})
			}
			c.startSession(ctx, peerID)
		}
	}
	for peerID := range c.onlinePeers {
		if !currentOnline[peerID] {
			peer, err := c.peerRepo.GetByID(ctx, peerID)
			if err == nil {
				c.addAlert(ctx, &models.Alert{
					ID:        fmt.Sprintf("peer-offline-%s-%d", peerID, time.Now().Unix()),
					Type:      "peer",
					Message:   "Клиент отключился: " + peer.Name,
					Severity:  "warning",
					Timestamp: time.Now(),
				})
			}
			c.endSession(ctx, peerID)
			c.clearPeerRealtime(peerID)
		}
	}
	c.onlinePeers = currentOnline

	c.cleanupStaleConnections(resp.Connections)
}

func (c *SingBoxStatsCollector) countConnectionsPerPeer(connections []clashConnection) map[string]int {
	uuidToPeerID := make(map[string]string)
	counts := make(map[string]int)

	for _, conn := range connections {
		if conn.Metadata.User == "" {
			continue
		}
		peerID, ok := uuidToPeerID[conn.Metadata.User]
		if !ok {
			ctx := context.Background()
			peer, err := c.peerRepo.GetByPublicKey(ctx, conn.Metadata.User)
			if err != nil {
				continue
			}
			peerID = peer.ID
			uuidToPeerID[conn.Metadata.User] = peerID
		}
		counts[peerID]++
	}

	return counts
}

func (c *SingBoxStatsCollector) updatePeerRealtime(peerID string, delta *userDelta, activeConns int, intervalSec float64, online bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	stats, ok := c.peerRealtime[peerID]
	if !ok {
		stats = &models.PeerRealtimeStats{}
		c.peerRealtime[peerID] = stats
	}

	stats.ActiveConnections = activeConns
	stats.BandwidthRx = delta.rx
	stats.BandwidthTx = delta.tx
	if intervalSec > 0 {
		stats.BandwidthRateRx = float64(delta.rx) / intervalSec
		stats.BandwidthRateTx = float64(delta.tx) / intervalSec
	}
	stats.SessionRx += delta.rx
	stats.SessionTx += delta.tx

	sess, ok := c.peerSessions[peerID]
	if ok && sess != nil {
		stats.ConnectedAt = &sess.startTime
		sess.connCount = activeConns
	}

	_ = online
	_ = now
}

func (c *SingBoxStatsCollector) clearPeerRealtime(peerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.peerRealtime, peerID)
}

func (c *SingBoxStatsCollector) startSession(ctx context.Context, peerID string) {
	sessionID, err := c.trafficRepo.CreateSession(ctx, peerID)
	if err != nil {
		c.logger.Error("ошибка создания сессии", "peer_id", peerID, "error", err)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.peerSessions[peerID] = &peerSessionInfo{
		sessionID: sessionID,
		startTime: time.Now(),
	}
}

func (c *SingBoxStatsCollector) endSession(ctx context.Context, peerID string) {
	c.mu.Lock()
	sess, ok := c.peerSessions[peerID]
	if ok {
		delete(c.peerSessions, peerID)
	}
	c.mu.Unlock()

	if !ok || sess == nil {
		return
	}

	rt, hasRt := c.peerRealtime[peerID]
	var rx, tx int64
	if hasRt {
		rx = rt.SessionRx
		tx = rt.SessionTx
	}

	if err := c.trafficRepo.CloseSession(ctx, sess.sessionID, rx, tx, sess.connCount); err != nil {
		c.logger.Error("ошибка закрытия сессии", "peer_id", peerID, "session_id", sess.sessionID, "error", err)
	}
}

func (c *SingBoxStatsCollector) handleAggregateVLESS(ctx context.Context, delta *userDelta, currentOnline map[string]bool, peerConnCounts map[string]int) {
	totalRx := delta.rx
	totalTx := delta.tx

	c.logger.Info("обработка агрегатного VLESS трафика",
		"total_rx", totalRx, "total_tx", totalTx,
		"connections", len(delta.connections))

	peers, err := c.peerRepo.List(ctx)
	if err != nil {
		c.logger.Error("ошибка получения списка пиров для агрегатного трафика", "error", err)
		return
	}

	var activePeers []*models.Peer
	for _, p := range peers {
		if p.IsActive {
			activePeers = append(activePeers, p)
		}
	}

	if len(activePeers) == 0 {
		c.logger.Warn("нет активных клиентов для распределения VLESS трафика")
		return
	}

	hasTraffic := totalRx > 0 || totalTx > 0

	if hasTraffic {
		perPeerRx := totalRx / int64(len(activePeers))
		perPeerTx := totalTx / int64(len(activePeers))
		remainderRx := totalRx % int64(len(activePeers))
		remainderTx := totalTx % int64(len(activePeers))

		for i, peer := range activePeers {
			rx := perPeerRx
			tx := perPeerTx
			if int64(i) < remainderRx {
				rx++
			}
			if int64(i) < remainderTx {
				tx++
			}

			if rx > 0 || tx > 0 {
				if err := c.peerRepo.UpdateTraffic(ctx, peer.ID, rx, tx); err != nil {
					c.logger.Error("ошибка обновления агрегатного трафика клиента", "id", peer.ID, "error", err)
				}
			}
		}
	}

	for _, peer := range activePeers {
		currentOnline[peer.ID] = true
		if err := c.peerRepo.UpdateLastSeen(ctx, peer.ID); err != nil {
			c.logger.Error("ошибка обновления last_seen клиента", "id", peer.ID, "error", err)
		}
	}

	for i, conn := range delta.connections {
		if conn.rx == 0 && conn.tx == 0 {
			continue
		}
		peer := activePeers[i%len(activePeers)]
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
		if conn.dstPort != "" {
			if p, err := strconv.Atoi(conn.dstPort); err == nil {
				trafficLog.DestPort = p
			}
		}
		if err := c.trafficRepo.Log(ctx, trafficLog); err != nil {
			c.logger.Error("ошибка логирования агрегатного трафика", "error", err)
		}
	}

	intervalSec := c.interval.Seconds()
	for _, peer := range activePeers {
		perPeerRx := totalRx / int64(len(activePeers))
		perPeerTx := totalTx / int64(len(activePeers))
		activeConns := peerConnCounts[peer.ID]

		c.mu.Lock()
		stats, ok := c.peerRealtime[peer.ID]
		if !ok {
			stats = &models.PeerRealtimeStats{}
			c.peerRealtime[peer.ID] = stats
		}
		stats.ActiveConnections = activeConns
		stats.BandwidthRx = perPeerRx
		stats.BandwidthTx = perPeerTx
		if intervalSec > 0 {
			stats.BandwidthRateRx = float64(perPeerRx) / intervalSec
			stats.BandwidthRateTx = float64(perPeerTx) / intervalSec
		}
		stats.SessionRx += perPeerRx
		stats.SessionTx += perPeerTx
		sess, hasSess := c.peerSessions[peer.ID]
		if hasSess && sess != nil {
			stats.ConnectedAt = &sess.startTime
		}
		c.mu.Unlock()
	}

	if hasTraffic {
		c.logger.Info("агрегатный VLESS трафик распределён",
			"total_rx", totalRx, "total_tx", totalTx,
			"active_peers", len(activePeers))
	}
}

func (c *SingBoxStatsCollector) computeDeltas(connections []clashConnection) map[string]*userDelta {
	deltas := make(map[string]*userDelta)

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, conn := range connections {
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

		userKey := conn.Metadata.User
		if userKey == "" && isVLESSInbound(conn.Metadata.Type) {
			userKey = aggregateVLESSKey
		}

		if userKey == "" {
			c.connState[conn.ID] = &connBytes{upload: conn.Upload, download: conn.Download}
			continue
		}

		d, ok := deltas[userKey]
		if !ok {
			d = &userDelta{}
			deltas[userKey] = d
		}
		d.rx += drx
		d.tx += dtx
		d.connCount++
		if drx > 0 || dtx > 0 {
			dest := conn.Metadata.DestinationIP
			if dest == "" {
				dest = conn.Metadata.Destination
			}
			d.connections = append(d.connections, userConnection{
				host:        conn.Metadata.Host,
				destination: dest,
				dstPort:     conn.Metadata.DstPort,
				rx:          drx,
				tx:          dtx,
			})
		}

		c.connState[conn.ID] = &connBytes{upload: conn.Upload, download: conn.Download}
	}

	return deltas
}

func isVLESSInbound(connType string) bool {
	return strings.Contains(connType, "vless")
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
