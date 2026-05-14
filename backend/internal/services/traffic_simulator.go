package services

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type TrafficSimulator struct {
	peerRepo    repository.PeerRepository
	trafficRepo repository.TrafficRepository
	alertSvc    *TrafficService
	logger      *slog.Logger
	interval    time.Duration

	mu           sync.Mutex
	peerRealtime map[string]*models.PeerRealtimeStats
	apiReachable bool
}

func NewTrafficSimulator(
	peerRepo repository.PeerRepository,
	trafficRepo repository.TrafficRepository,
	alertSvc *TrafficService,
	logger *slog.Logger,
) *TrafficSimulator {
	return &TrafficSimulator{
		peerRepo:     peerRepo,
		trafficRepo:  trafficRepo,
		alertSvc:     alertSvc,
		logger:       logger,
		interval:     10 * time.Second,
		peerRealtime: make(map[string]*models.PeerRealtimeStats),
		apiReachable: true,
	}
}

func (s *TrafficSimulator) GetRealtimeStats() map[string]*models.PeerRealtimeStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[string]*models.PeerRealtimeStats, len(s.peerRealtime))
	for k, v := range s.peerRealtime {
		cp := *v
		result[k] = &cp
	}
	return result
}

func (s *TrafficSimulator) IsAPIReachable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.apiReachable
}

var simulatedDomains = []struct {
	domain string
	ip     string
	port   int
	action string
	weight int
}{
	{"youtube.com", "142.250.80.46", 443, "proxy", 30},
	{"googlevideo.com", "142.250.80.46", 443, "proxy", 20},
	{"vk.com", "87.240.165.68", 443, "direct", 15},
	{"instagram.com", "31.13.80.1", 443, "proxy", 10},
	{"telegram.org", "149.154.167.99", 443, "proxy", 12},
	{"yandex.ru", "77.88.55.80", 443, "direct", 10},
	{"github.com", "20.205.243.166", 443, "proxy", 8},
	{"habr.com", "178.248.233.32", 443, "direct", 5},
	{"discord.com", "162.159.128.233", 443, "proxy", 7},
	{"twitter.com", "104.244.42.1", 443, "proxy", 6},
	{"chatgpt.com", "104.18.32.7", 443, "proxy", 8},
	{"facebook.com", "31.13.80.1", 443, "proxy", 4},
	{"t.me", "149.154.167.99", 443, "proxy", 5},
	{"gosuslugi.ru", "185.122.164.2", 443, "direct", 3},
}

func (s *TrafficSimulator) Start(ctx context.Context) {
	s.logger.Info("запуск симулятора трафика (dev mode)", "interval", s.interval)

	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("PANIC в TrafficSimulator", "error", r)
		}
	}()

	s.addAlert(ctx, &models.Alert{
		ID:        "simulator-started",
		Type:      "system",
		Message:   "Симулятор трафика запущен (dev mode)",
		Severity:  "info",
		Timestamp: time.Now(),
	})

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.simulate(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("остановка симулятора трафика")
			return
		case <-ticker.C:
			s.simulate(ctx)
		}
	}
}

func (s *TrafficSimulator) simulate(ctx context.Context) {
	peers, err := s.peerRepo.List(ctx)
	if err != nil {
		s.logger.Error("ошибка получения списка клиентов для симуляции", "error", err)
		return
	}

	if len(peers) == 0 {
		return
	}

	intervalSec := s.interval.Seconds()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		if !peer.IsActive {
			delete(s.peerRealtime, peer.ID)
			continue
		}

		if rand.Float64() < 0.1 {
			delete(s.peerRealtime, peer.ID)
			continue
		}

		numDomains := rand.Intn(3) + 1
		var totalRx, totalTx int64

		for i := 0; i < numDomains; i++ {
			entry := simulatedDomains[rand.Intn(len(simulatedDomains))]

			rx := rand.Int63n(2*1024*1024) + 10*1024
			tx := rand.Int63n(512*1024) + 5*1024

			totalRx += rx
			totalTx += tx

			trafficLog := &models.TrafficLog{
				PeerID:  peer.ID,
				Domain:  entry.domain,
				DestIP:  entry.ip,
				DestPort: entry.port,
				Action:  entry.action,
				BytesRx: rx,
				BytesTx: tx,
			}
			if logErr := s.trafficRepo.Log(ctx, trafficLog); logErr != nil {
				s.logger.Error("ошибка логирования симулированного трафика", "error", logErr)
			}
		}

		if totalRx > 0 || totalTx > 0 {
			if updateErr := s.peerRepo.UpdateTraffic(ctx, peer.ID, totalRx, totalTx); updateErr != nil {
				s.logger.Error("ошибка обновления трафика клиента (sim)", "id", peer.ID, "error", updateErr)
			}
		}

		if updateErr := s.peerRepo.UpdateLastSeen(ctx, peer.ID); updateErr != nil {
			s.logger.Error("ошибка обновления last_seen (sim)", "id", peer.ID, "error", updateErr)
		}

		stats, ok := s.peerRealtime[peer.ID]
		if !ok {
			now := time.Now()
			stats = &models.PeerRealtimeStats{
				ConnectedAt: &now,
			}
			s.peerRealtime[peer.ID] = stats
		}

		stats.ActiveConnections = rand.Intn(15) + 1
		stats.BandwidthRx = totalRx
		stats.BandwidthTx = totalTx
		stats.BandwidthRateRx = float64(totalRx) / intervalSec
		stats.BandwidthRateTx = float64(totalTx) / intervalSec
		stats.SessionRx += totalRx
		stats.SessionTx += totalTx
	}
}

func (s *TrafficSimulator) addAlert(ctx context.Context, alert *models.Alert) {
	if s.alertSvc != nil {
		s.alertSvc.AddAlert(ctx, alert)
	}
}
