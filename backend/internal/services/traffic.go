package services

import (
	"context"
	"fmt"
	"log/slog"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type TrafficService struct {
	trafficRepo repository.TrafficRepository
	peerRepo    repository.PeerRepository
	logger      *slog.Logger
}

func NewTrafficService(trafficRepo repository.TrafficRepository, peerRepo repository.PeerRepository, logger *slog.Logger) *TrafficService {
	return &TrafficService{
		trafficRepo: trafficRepo,
		peerRepo:    peerRepo,
		logger:      logger,
	}
}

func (s *TrafficService) GetTrafficLogs(ctx context.Context, filter models.TrafficFilter) ([]*models.TrafficLog, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	logs, err := s.trafficRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("service.traffic.GetTrafficLogs: %w", err)
	}
	return logs, nil
}

func (s *TrafficService) GetTotalStats(ctx context.Context) (*models.TotalStats, error) {
	stats, err := s.trafficRepo.GetTotalStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.traffic.GetTotalStats: %w", err)
	}
	return stats, nil
}

func (s *TrafficService) GetPeerStats(ctx context.Context, peerID string) (*models.PeerStats, error) {
	stats, err := s.trafficRepo.GetPeerStats(ctx, peerID)
	if err != nil {
		return nil, fmt.Errorf("service.traffic.GetPeerStats: %w", err)
	}
	return stats, nil
}

func (s *TrafficService) LogTraffic(ctx context.Context, log *models.TrafficLog) error {
	if err := s.trafficRepo.Log(ctx, log); err != nil {
		return fmt.Errorf("service.traffic.LogTraffic: %w", err)
	}
	return nil
}

func (s *TrafficService) CleanupOldLogs(ctx context.Context, retainDays int) (int64, error) {
	if retainDays <= 0 {
		retainDays = 30
	}
	deleted, err := s.trafficRepo.CleanupOld(ctx, retainDays)
	if err != nil {
		return 0, fmt.Errorf("service.traffic.CleanupOldLogs: %w", err)
	}
	s.logger.Info("очищены старые логи трафика", "deleted", deleted, "retain_days", retainDays)
	return deleted, nil
}

func (s *TrafficService) GetAlerts(ctx context.Context) ([]*models.Alert, error) {
	return []*models.Alert{}, nil
}
