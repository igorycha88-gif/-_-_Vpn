package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"

	"github.com/google/uuid"
)

type WireGuardService struct {
	peerRepo repository.PeerRepository
	vlessCfg *config.VLESSConfig
	logger   *slog.Logger
}

func NewWireGuardService(peerRepo repository.PeerRepository, vlessCfg *config.VLESSConfig, logger *slog.Logger) *WireGuardService {
	return &WireGuardService{
		peerRepo: peerRepo,
		vlessCfg: vlessCfg,
		logger:   logger,
	}
}

func (s *WireGuardService) CreatePeer(ctx context.Context, req *models.PeerCreateRequest) (*models.Peer, error) {
	if errs := req.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("service.wireguard.CreatePeer: невалидные данные: %v", errs)
	}

	peerUUID := uuid.New().String()

	peer := &models.Peer{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Email:      req.Email,
		PublicKey:  peerUUID,
		PrivateKey: "",
		Address:    peerUUID,
		DNS:        "1.1.1.1,8.8.8.8",
		MTU:        1280,
		IsActive:   true,
	}

	if err := s.peerRepo.Create(ctx, peer); err != nil {
		return nil, fmt.Errorf("service.wireguard.CreatePeer save: %w", err)
	}

	s.logger.Info("создан VLESS клиент", "id", peer.ID, "name", peer.Name, "uuid", peerUUID)
	return peer, nil
}

func (s *WireGuardService) GetPeer(ctx context.Context, id string) (*models.Peer, error) {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.GetPeer: %w", err)
	}
	return peer, nil
}

func (s *WireGuardService) ListPeers(ctx context.Context) ([]*models.Peer, error) {
	peers, err := s.peerRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.ListPeers: %w", err)
	}
	return peers, nil
}

func (s *WireGuardService) DeletePeer(ctx context.Context, id string) error {
	if _, err := s.peerRepo.GetByID(ctx, id); err != nil {
		return fmt.Errorf("service.wireguard.DeletePeer: %w", err)
	}

	if err := s.peerRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("service.wireguard.DeletePeer: %w", err)
	}
	s.logger.Info("удалён VLESS клиент", "id", id)
	return nil
}

func (s *WireGuardService) TogglePeer(ctx context.Context, id string, active bool) error {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("service.wireguard.TogglePeer: %w", err)
	}
	peer.IsActive = active
	if err := s.peerRepo.Update(ctx, peer); err != nil {
		return fmt.Errorf("service.wireguard.TogglePeer update: %w", err)
	}

	s.logger.Info("изменён статус клиента", "id", id, "active", active)
	return nil
}

func (s *WireGuardService) GenerateClientConfig(peer *models.Peer) string {
	cfg := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"tag": "remote", "address": "1.1.1.1"},
				map[string]any{"tag": "local", "address": "77.88.8.8", "detour": "direct-out"},
			},
			"rules":    []any{map[string]any{"server": "local"}},
			"final":    "remote",
			"strategy": "prefer_ipv4",
		},
		"inbounds": []any{
			map[string]any{
				"type":         "tun",
				"tag":          "tun-in",
				"address":      []string{"172.19.0.1/30"},
				"auto_route":   true,
				"strict_route": true,
				"stack":        "mixed",
			},
		},
		"outbounds": []any{
			map[string]any{
				"type":        "vless",
				"tag":         "proxy",
				"server":      s.vlessCfg.ServerEndpoint,
				"server_port": s.vlessCfg.Port,
				"uuid":        peer.PublicKey,
				"flow":        s.vlessCfg.Flow,
				"tls": map[string]any{
					"enabled":     true,
					"server_name": s.vlessCfg.ServerName,
					"utls": map[string]any{
						"enabled":     true,
						"fingerprint": s.vlessCfg.Fingerprint,
					},
					"reality": map[string]any{
						"enabled":    true,
						"public_key": s.vlessCfg.PublicKey,
						"short_id":   s.vlessCfg.ShortID,
					},
				},
			},
			map[string]any{"type": "direct", "tag": "direct-out"},
		},
		"route": map[string]any{
			"rules": []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
				map[string]any{"ip_is_private": true, "outbound": "direct-out"},
				map[string]any{
					"domain_suffix": []string{"max.ru", "maxpatrol.ru", "positive-technologies.ru"},
					"outbound":      "direct-out",
				},
				map[string]any{
					"domain_suffix": []string{"gosuslugi.ru", "esia.gosuslugi.ru"},
					"outbound":      "direct-out",
				},
			},
			"final":                 "proxy",
			"auto_detect_interface": true,
		},
	}

	data, _ := json.MarshalIndent(cfg, "", "  ")
	return string(data)
}

func (s *WireGuardService) GetPeerStats(ctx context.Context, id string) (*models.PeerStats, error) {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.GetPeerStats: %w", err)
	}
	return &models.PeerStats{
		PeerID:  peer.ID,
		TotalRx: peer.TotalRx,
		TotalTx: peer.TotalTx,
		Online:  peer.IsActive,
	}, nil
}

func (s *WireGuardService) SyncAllPeers(ctx context.Context) error {
	s.logger.Info("синхронизация клиентов VLESS (управление через sing-box конфиг)")
	return nil
}
