package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

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
		DeviceType: req.DeviceType,
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

	s.logger.Info("создан VLESS клиент", "id", peer.ID, "name", peer.Name, "device", peer.DeviceType, "uuid", peerUUID)
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
	deviceType := peer.DeviceType
	if deviceType == "" {
		deviceType = models.DeviceTypeIPhone
	}

	stack := "mixed"
	var routeRules []any

	baseRules := []any{
		map[string]any{"inbound": []string{"tun-in"}, "action": "sniff"},
		map[string]any{"protocol": "dns", "inbound": []string{"tun-in"}, "action": "hijack-dns"},
		map[string]any{"ip_is_private": true, "outbound": "direct-out"},
		map[string]any{
			"domain_suffix": []string{".ru", ".su", ".xn--p1ai"},
			"outbound":      "direct-out",
		},
		map[string]any{
			"domain_suffix": []string{
				"vk.com", "userapi.com", "vk-cdn.net",
				"yandex.com", "yandex.ru", "yandex.net", "yastatic.net",
				"ya.ru", "mail.ru", "rambler.ru",
				"gosuslugi.ru", "esia.gosuslugi.ru",
				"sberbank.ru", "tinkoff.ru",
				"ozon.ru", "wildberries.ru", "avito.ru",
				"habr.com", "kaspersky.com",
				"max.ru", "maxpatrol.ru", "positive-technologies.ru",
			},
			"outbound": "direct-out",
		},
	}

	proxyDomains := []any{
		map[string]any{
			"domain_suffix": []string{
				"youtube.com", "youtu.be", "googlevideo.com",
				"instagram.com", "cdninstagram.com",
				"facebook.com", "fbcdn.net", "meta.com",
				"telegram.org", "t.me",
				"twitter.com", "x.com", "twimg.com",
				"discord.com", "discordapp.com", "discord.gg",
				"chatgpt.com", "openai.com", "ai.com",
				"google.com", "googleapis.com", "gstatic.com",
				"github.com", "githubusercontent.com",
				"netflix.com", "nflxvideo.net", "nflximg.net",
				"tiktok.com", "tiktokcdn.com",
			},
			"outbound": "proxy",
		},
	}

	packageNameRules := []any{
		map[string]any{
			"package_name": []string{
				"com.google.android.projection.gearhead",
				"ru.yandex.weather",
			},
			"outbound": "direct-out",
		},
	}

	switch deviceType {
	case models.DeviceTypeAndroid:
		stack = "gvisor"
		routeRules = append(baseRules, packageNameRules...)
		routeRules = append(routeRules, proxyDomains...)
	default:
		stack = "system"
		routeRules = append(baseRules, proxyDomains...)
	}

	cfg := map[string]any{
		"log": map[string]any{
			"level":     "info",
			"timestamp": true,
		},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"tag": "dns-foreign", "address": "1.1.1.1", "detour": "proxy"},
				map[string]any{"tag": "dns-foreign-alt", "address": "8.8.8.8", "detour": "proxy"},
				map[string]any{"tag": "dns-ru", "address": "77.88.8.8", "detour": "direct-out"},
				map[string]any{"tag": "dns-ru-alt", "address": "77.88.8.1", "detour": "direct-out"},
			},
			"rules": []any{
				map[string]any{"domain_suffix": []string{".ru", ".su", ".xn--p1ai"}, "server": "dns-ru"},
				map[string]any{
					"domain_suffix": []string{
						"vk.com", "userapi.com", "vk-cdn.net",
						"yandex.com", "yandex.ru", "yandex.net", "yastatic.net",
						"ya.ru", "mail.ru", "rambler.ru",
						"gosuslugi.ru", "esia.gosuslugi.ru",
						"sberbank.ru", "tinkoff.ru",
						"ozon.ru", "wildberries.ru", "avito.ru",
						"habr.com", "kaspersky.com",
						"max.ru", "maxpatrol.ru", "positive-technologies.ru",
					},
					"server": "dns-ru",
				},
				map[string]any{"inbound": []string{"tun-in"}, "server": "dns-foreign"},
			},
			"final":    "dns-foreign",
			"strategy": "prefer_ipv4",
		},
		"inbounds": []any{
			map[string]any{
				"type":         "tun",
				"tag":          "tun-in",
				"address":      []string{"172.19.0.1/30"},
				"auto_route":   true,
				"strict_route": true,
				"stack":        stack,
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
			"rules":                 routeRules,
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

	online := false
	if peer.LastSeen != nil && time.Since(*peer.LastSeen) < 2*time.Minute {
		online = true
	}

	return &models.PeerStats{
		PeerID:  peer.ID,
		TotalRx: peer.TotalRx,
		TotalTx: peer.TotalTx,
		Online:  online,
	}, nil
}

func (s *WireGuardService) SyncAllPeers(ctx context.Context) error {
	s.logger.Info("синхронизация клиентов VLESS (управление через sing-box конфиг)")
	return nil
}
