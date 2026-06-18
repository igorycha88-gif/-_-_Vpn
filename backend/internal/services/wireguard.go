package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"smarttraffic/internal/apperrors"
	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"

	"github.com/google/uuid"
)

type WireGuardService struct {
	peerRepo    repository.PeerRepository
	trafficRepo repository.TrafficRepository
	vlessCfg    *config.VLESSConfig
	logger      *slog.Logger
}

func NewWireGuardService(peerRepo repository.PeerRepository, trafficRepo repository.TrafficRepository, vlessCfg *config.VLESSConfig, logger *slog.Logger) *WireGuardService {
	return &WireGuardService{
		peerRepo:    peerRepo,
		trafficRepo: trafficRepo,
		vlessCfg:    vlessCfg,
		logger:      logger,
	}
}

func (s *WireGuardService) mapErr(err error) error {
	if err == nil {
		return nil
	}
	if repository.IsNotFound(err) {
		return apperrors.ErrNotFound
	}
	return err
}

func (s *WireGuardService) CreatePeer(ctx context.Context, req *models.PeerCreateRequest) (*models.Peer, error) {
	if errs := req.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("service.wireguard.CreatePeer: невалидные данные: %v", errs)
	}

	peerUUID := uuid.New().String()

	configMode := req.ConfigMode
	if configMode == "" {
		configMode = models.ConfigModeTun
	}

	peer := &models.Peer{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Email:      req.Email,
		DeviceType: req.DeviceType,
		ConfigMode: configMode,
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

	s.logger.Info("создан VLESS клиент", "id", peer.ID, "name", peer.Name, "device", peer.DeviceType, "mode", peer.ConfigMode, "uuid", peerUUID)
	return peer, nil
}

func (s *WireGuardService) GetPeer(ctx context.Context, id string) (*models.Peer, error) {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapErr(fmt.Errorf("service.wireguard.GetPeer: %w", err))
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
		return s.mapErr(fmt.Errorf("service.wireguard.DeletePeer: %w", err))
	}

	if err := s.trafficRepo.DeleteByPeerID(ctx, id); err != nil {
		return fmt.Errorf("service.wireguard.DeletePeer traffic: %w", err)
	}

	if err := s.trafficRepo.DeleteSessionsByPeerID(ctx, id); err != nil {
		return fmt.Errorf("service.wireguard.DeletePeer sessions: %w", err)
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
		return s.mapErr(fmt.Errorf("service.wireguard.TogglePeer: %w", err))
	}
	peer.IsActive = active
	if err := s.peerRepo.Update(ctx, peer); err != nil {
		return fmt.Errorf("service.wireguard.TogglePeer update: %w", err)
	}

	s.logger.Info("изменён статус клиента", "id", id, "active", active)
	return nil
}

var androidAutoExcludePackages = []string{
	"com.google.android.projection.gearhead",
	"com.google.android.gms",
	"com.google.android.apps.auto",
}

var ruDomainSuffixes = []string{".ru", ".su", ".xn--p1ai"}

var ruDirectDomains = []string{
	"vk.com", "userapi.com", "vk-cdn.net",
	"yandex.com", "yandex.ru", "yandex.net", "yastatic.net",
	"ya.ru", "mail.ru", "rambler.ru",
	"gosuslugi.ru", "esia.gosuslugi.ru",
	"sberbank.ru", "tinkoff.ru",
	"ozon.ru", "wildberries.ru", "avito.ru", "avito.st", "avito.com",
	"habr.com", "kaspersky.com",
	"max.ru", "maxpatrol.ru", "positive-technologies.ru",
}

func (s *WireGuardService) buildClientConfigMap(peer *models.Peer) map[string]any {
	mode := peer.ConfigMode
	if mode == "" {
		mode = models.ConfigModeTun
	}
	if mode == models.ConfigModeProxy {
		return s.buildProxyConfigMap(peer)
	}
	return s.buildTunConfigMap(peer)
}

func (s *WireGuardService) buildTunConfigMap(peer *models.Peer) map[string]any {
	deviceType := peer.DeviceType
	if deviceType == "" {
		deviceType = models.DeviceTypeIPhone
	}

	stack := "mixed"
	var routeRules []any
	var excludePackages []string

	baseRules := s.buildBaseRules()
	proxyDomains := s.buildProxyDomains()

	switch deviceType {
	case models.DeviceTypeAndroid:
		stack = "gvisor"
		routeRules = append(baseRules, s.buildPackageNameRules()...)
		routeRules = append(routeRules, proxyDomains...)
		excludePackages = androidAutoExcludePackages
	default:
		routeRules = append(baseRules, proxyDomains...)
	}

	return map[string]any{
		"log":      map[string]any{"level": "info", "timestamp": true},
		"dns":      s.buildClientDNSConfig(),
		"inbounds": []any{s.buildTunInbound(stack, excludePackages)},
		"outbounds": []any{
			s.buildVlessOutbound(peer),
			map[string]any{"type": "direct", "tag": "direct-out"},
		},
		"route": map[string]any{
			"rules":                 routeRules,
			"final":                 "direct-out",
			"auto_detect_interface": true,
		},
	}
}

func (s *WireGuardService) buildBaseRules() []any {
	rules := []any{
		map[string]any{"inbound": []string{"tun-in"}, "action": "sniff"},
		map[string]any{"protocol": "dns", "inbound": []string{"tun-in"}, "action": "hijack-dns"},
		map[string]any{"ip_cidr": []string{s.vlessCfg.ServerEndpoint + "/32"}, "outbound": "direct-out"},
		map[string]any{"ip_is_private": true, "outbound": "direct-out"},
	}
	return append(rules, s.buildDirectDomainRules()...)
}

func (s *WireGuardService) buildDirectDomainRules() []any {
	return []any{
		map[string]any{"domain_suffix": ruDomainSuffixes, "outbound": "direct-out"},
		map[string]any{"domain_suffix": ruDirectDomains, "outbound": "direct-out"},
	}
}

func (s *WireGuardService) buildProxyDomains() []any {
	return []any{
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
}

func (s *WireGuardService) buildPackageNameRules() []any {
	return []any{
		map[string]any{
			"package_name": []string{
				"com.google.android.projection.gearhead",
				"ru.yandex.weather",
				"ru.sberbankmobile",
			},
			"outbound": "direct-out",
		},
	}
}

func (s *WireGuardService) buildClientDNSConfig() map[string]any {
	return map[string]any{
		"servers": []any{
			map[string]any{"tag": "dns-foreign", "address": "1.1.1.1", "detour": "proxy"},
			map[string]any{"tag": "dns-foreign-alt", "address": "8.8.8.8", "detour": "proxy"},
			map[string]any{"tag": "dns-ru", "address": "77.88.8.8", "detour": "direct-out"},
			map[string]any{"tag": "dns-ru-alt", "address": "77.88.8.1", "detour": "direct-out"},
		},
		"rules": []any{
			map[string]any{"domain_suffix": ruDomainSuffixes, "server": "dns-ru"},
			map[string]any{"domain_suffix": ruDirectDomains, "server": "dns-ru"},
			map[string]any{"inbound": []string{"tun-in"}, "server": "dns-foreign"},
		},
		"final":    "dns-foreign",
		"strategy": "prefer_ipv4",
	}
}

func (s *WireGuardService) buildProxyConfigMap(peer *models.Peer) map[string]any {
	rules := []any{
		map[string]any{"inbound": []string{"mixed-in"}, "action": "sniff"},
		map[string]any{"ip_cidr": []string{s.vlessCfg.ServerEndpoint + "/32"}, "outbound": "direct-out"},
		map[string]any{"ip_is_private": true, "outbound": "direct-out"},
	}
	rules = append(rules, s.buildDirectDomainRules()...)
	rules = append(rules, s.buildProxyDomains()...)

	return map[string]any{
		"log":      map[string]any{"level": "info", "timestamp": true},
		"dns":      s.buildProxyDNSConfig(),
		"inbounds": []any{s.buildMixedInbound()},
		"outbounds": []any{
			s.buildVlessOutbound(peer),
			map[string]any{"type": "direct", "tag": "direct-out"},
		},
		"route": map[string]any{
			"rules":                 rules,
			"final":                 "proxy",
			"auto_detect_interface": true,
		},
	}
}

func (s *WireGuardService) buildMixedInbound() map[string]any {
	return map[string]any{
		"type":        "mixed",
		"tag":         "mixed-in",
		"listen":      "127.0.0.1",
		"listen_port": 2080,
	}
}

func (s *WireGuardService) buildProxyDNSConfig() map[string]any {
	return map[string]any{
		"servers": []any{
			map[string]any{"tag": "dns-foreign", "address": "1.1.1.1", "detour": "proxy"},
			map[string]any{"tag": "dns-foreign-alt", "address": "8.8.8.8", "detour": "proxy"},
			map[string]any{"tag": "dns-ru", "address": "77.88.8.8", "detour": "direct-out"},
			map[string]any{"tag": "dns-ru-alt", "address": "77.88.8.1", "detour": "direct-out"},
		},
		"rules": []any{
			map[string]any{"domain_suffix": ruDomainSuffixes, "server": "dns-ru"},
			map[string]any{"domain_suffix": ruDirectDomains, "server": "dns-ru"},
		},
		"final":    "dns-foreign",
		"strategy": "prefer_ipv4",
	}
}

func (s *WireGuardService) buildTunInbound(stack string, excludePackages []string) map[string]any {
	inbound := map[string]any{
		"type":         "tun",
		"tag":          "tun-in",
		"address":      []string{"172.19.0.1/30"},
		"auto_route":   true,
		"strict_route": true,
		"stack":        stack,
	}
	if len(excludePackages) > 0 {
		inbound["exclude_package"] = excludePackages
	}
	return inbound
}

func (s *WireGuardService) buildVlessOutbound(peer *models.Peer) map[string]any {
	return map[string]any{
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
	}
}

func (s *WireGuardService) GenerateClientConfig(peer *models.Peer) string {
	cfg := s.buildClientConfigMap(peer)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		s.logger.Error("ошибка генерации конфига", "error", err)
		return "{}"
	}
	return string(data)
}

func (s *WireGuardService) GenerateClientConfigCompact(peer *models.Peer) (string, error) {
	cfg := s.buildClientConfigMap(peer)
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("service.wireguard.GenerateClientConfigCompact: %w", err)
	}
	return string(data), nil
}

func (s *WireGuardService) GetPeerStats(ctx context.Context, id string) (*models.PeerStats, error) {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapErr(fmt.Errorf("service.wireguard.GetPeerStats: %w", err))
	}

	online := peer.LastSeen != nil && time.Since(*peer.LastSeen) < 2*time.Minute

	return &models.PeerStats{
		PeerID:  peer.ID,
		TotalRx: peer.TotalRx,
		TotalTx: peer.TotalTx,
		Online:  online,
	}, nil
}
