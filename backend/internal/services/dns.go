package services

import (
	"context"
	"fmt"
	"log/slog"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type DNSService struct {
	dnsRepo repository.DNSRepository
	logger  *slog.Logger
}

func NewDNSService(dnsRepo repository.DNSRepository, logger *slog.Logger) *DNSService {
	return &DNSService{
		dnsRepo: dnsRepo,
		logger:  logger,
	}
}

func (s *DNSService) GetSettings(ctx context.Context) (*models.DNSSettings, error) {
	settings, err := s.dnsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.dns.GetSettings: %w", err)
	}
	return settings, nil
}

func (s *DNSService) UpdateSettings(ctx context.Context, req *models.DNSSettingsUpdateRequest) (*models.DNSSettings, error) {
	current, err := s.dnsRepo.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.dns.UpdateSettings get: %w", err)
	}

	if req.UpstreamRU != nil {
		current.UpstreamRU = *req.UpstreamRU
	}
	if req.UpstreamForeign != nil {
		current.UpstreamForeign = *req.UpstreamForeign
	}
	if req.BlockAds != nil {
		current.BlockAds = *req.BlockAds
	}

	if err := s.dnsRepo.Update(ctx, current); err != nil {
		return nil, fmt.Errorf("service.dns.UpdateSettings save: %w", err)
	}

	s.logger.Info("DNS настройки обновлены")
	return current, nil
}

func (s *DNSService) GetPresets() []models.DNSPreset {
	return []models.DNSPreset{
		{ID: "auto", Name: "Автоматически", Servers: ""},
		{ID: "yandex", Name: "Яндекс DNS (77.88.8.8, 77.88.8.1)", Servers: "77.88.8.8,77.88.8.1"},
		{ID: "cloudflare", Name: "Cloudflare (1.1.1.1, 1.0.0.1)", Servers: "1.1.1.1,1.0.0.1"},
		{ID: "google", Name: "Google (8.8.8.8, 8.8.4.4)", Servers: "8.8.8.8,8.8.4.4"},
		{ID: "adguard", Name: "AdGuard (94.140.14.14, 94.140.15.15)", Servers: "94.140.14.14,94.140.15.15"},
	}
}
