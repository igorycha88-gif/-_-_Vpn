package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"time"

	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type SingBoxService struct {
	routeRepo repository.RouteRepository
	dnsRepo   repository.DNSRepository
	peerRepo  repository.PeerRepository
	cfg       *config.SingBoxConfig
	vlessCfg  *config.VLESSConfig
	wgConfig  *config.WGConfig
	srvConfig *config.ServerConfig
	logger    *slog.Logger
}

func NewSingBoxService(
	routeRepo repository.RouteRepository,
	dnsRepo repository.DNSRepository,
	peerRepo repository.PeerRepository,
	cfg *config.SingBoxConfig,
	vlessCfg *config.VLESSConfig,
	wgConfig *config.WGConfig,
	srvConfig *config.ServerConfig,
	logger *slog.Logger,
) *SingBoxService {
	return &SingBoxService{
		routeRepo: routeRepo,
		dnsRepo:   dnsRepo,
		peerRepo:  peerRepo,
		cfg:       cfg,
		vlessCfg:  vlessCfg,
		wgConfig:  wgConfig,
		srvConfig: srvConfig,
		logger:    logger,
	}
}

type singBoxConfig struct {
	Log          *singBoxLog          `json:"log,omitempty"`
	DNS          *singBoxDNS          `json:"dns,omitempty"`
	Inbounds     []any                `json:"inbounds"`
	Endpoints    []any                `json:"endpoints,omitempty"`
	Outbounds    []any                `json:"outbounds,omitempty"`
	Route        *singBoxRoute        `json:"route"`
	Experimental *singBoxExperimental `json:"experimental,omitempty"`
}

type singBoxLog struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp,omitempty"`
}

type singBoxDNS struct {
	Servers  []singBoxDNSServer `json:"servers"`
	Rules    []any              `json:"rules,omitempty"`
	Final    string             `json:"final,omitempty"`
	Strategy string             `json:"strategy,omitempty"`
}

type singBoxDNSServer struct {
	Tag    string `json:"tag"`
	Type   string `json:"type"`
	Server string `json:"server"`
	Detour string `json:"detour,omitempty"`
}

type singBoxRoute struct {
	Rules                 []any  `json:"rules"`
	Final                 string `json:"final"`
	AutoDetectInterface   bool   `json:"auto_detect_interface"`
	DefaultDomainResolver string `json:"default_domain_resolver,omitempty"`
}

type singBoxExperimental struct {
	ClashAPI *singBoxClashAPI `json:"clash_api,omitempty"`
}

type singBoxClashAPI struct {
	ExternalController string `json:"external_controller"`
	Secret             string `json:"secret,omitempty"`
}

func (s *SingBoxService) GenerateConfig(ctx context.Context) (*singBoxConfig, error) {
	rules, err := s.routeRepo.List(ctx)
	if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("service.singbox.GenerateConfig: %w", err)
	}

	dnsSettings, err := s.dnsRepo.Get(ctx)
	if err != nil {
		s.logger.Warn("не удалось получить DNS настройки, используются умолчания")
		dnsSettings = &models.DNSSettings{
			UpstreamRU:      "77.88.8.8,77.88.8.1",
			UpstreamForeign: "1.1.1.1,8.8.8.8",
		}
	}

	peers, err := s.peerRepo.List(ctx)
	if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("service.singbox.GenerateConfig list peers: %w", err)
	}

	var users []map[string]any
	for _, p := range peers {
		if p.IsActive {
			users = append(users, map[string]any{
				"uuid": p.PublicKey,
				"flow": s.vlessCfg.Flow,
			})
		}
	}

	vlessInbound := map[string]any{
		"type":        "vless",
		"tag":         "vless-in",
		"listen":      "::",
		"listen_port": s.vlessCfg.Port,
		"users":       users,
		"tls": map[string]any{
			"enabled":     true,
			"server_name": s.vlessCfg.ServerName,
			"reality": map[string]any{
				"enabled": true,
				"handshake": map[string]any{
					"server":      s.vlessCfg.ServerName,
					"server_port": s.vlessCfg.Port,
				},
				"private_key": s.vlessCfg.PrivateKey,
				"short_id":    []string{s.vlessCfg.ShortID},
			},
		},
	}

	directOutbound := map[string]any{"type": "direct", "tag": "direct-out"}

	cfg := &singBoxConfig{
		Log:       &singBoxLog{Level: "info", Timestamp: true},
		Inbounds:  []any{vlessInbound},
		Outbounds: []any{directOutbound},
		Route: &singBoxRoute{
			Rules: []any{
				map[string]any{"action": "sniff"},
				map[string]any{"protocol": "dns", "action": "hijack-dns"},
			},
			Final:                 "direct-out",
			AutoDetectInterface:   true,
			DefaultDomainResolver: "dns-foreign-1.1.1.1",
		},
	}

	cfg.DNS = s.buildDNSConfig(dnsSettings)

	clashAPI := &singBoxClashAPI{
		ExternalController: s.cfg.ClashAPIAddr,
	}
	if s.cfg.ClashAPISecret != "" {
		clashAPI.Secret = s.cfg.ClashAPISecret
	}
	cfg.Experimental = &singBoxExperimental{ClashAPI: clashAPI}

	if s.srvConfig.ForeignIP != "" && s.srvConfig.ForeignVLESS.UUID != "" {
		vlessOutbound := map[string]any{
			"type":        "vless",
			"tag":         "foreign-out",
			"server":      s.srvConfig.ForeignIP,
			"server_port": 443,
			"uuid":        s.srvConfig.ForeignVLESS.UUID,
			"flow":        "xtls-rprx-vision",
			"tls": map[string]any{
				"enabled":     true,
				"server_name": s.srvConfig.ForeignVLESS.ServerName,
				"utls": map[string]any{
					"enabled":     true,
					"fingerprint": "chrome",
				},
				"reality": map[string]any{
					"enabled":     true,
					"public_key":  s.srvConfig.ForeignVLESS.RealityPublicKey,
					"short_id":    s.srvConfig.ForeignVLESS.RealityShortID,
				},
			},
		}
		cfg.Outbounds = append(cfg.Outbounds, vlessOutbound)
		cfg.Route.Final = "foreign-out"
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		if rule.Action == "block" {
			routeRule := map[string]any{"action": "reject"}
			s.populateRouteRuleFields(routeRule, rule)
			cfg.Route.Rules = append(cfg.Route.Rules, routeRule)
			continue
		}

		outbound := s.actionToOutbound(rule.Action)
		if outbound == "" {
			continue
		}

		routeRule := map[string]any{"outbound": outbound}
		s.populateRouteRuleFields(routeRule, rule)
		cfg.Route.Rules = append(cfg.Route.Rules, routeRule)
	}

	return cfg, nil
}

func (s *SingBoxService) populateRouteRuleFields(routeRule map[string]any, rule *models.RoutingRule) {
	switch rule.Type {
	case "domain":
		routeRule["domain"] = []string{rule.Pattern}
	case "domain_suffix":
		routeRule["domain_suffix"] = []string{rule.Pattern}
	case "domain_keyword":
		routeRule["domain_keyword"] = []string{rule.Pattern}
	case "ip":
		routeRule["ip_cidr"] = []string{rule.Pattern}
	case "geoip":
		s.logger.Warn("geoip правило пропущено — не поддерживается в sing-box 1.12+", "pattern", rule.Pattern)
		return
	case "port":
		var port int
		fmt.Sscanf(rule.Pattern, "%d", &port)
		if port > 0 {
			routeRule["port"] = []int{port}
		}
	case "regex":
		routeRule["domain"] = []string{"regexp:" + rule.Pattern}
	}
}

func (s *SingBoxService) WriteConfig(ctx context.Context) error {
	cfg, err := s.GenerateConfig(ctx)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("service.singbox.WriteConfig marshal: %w", err)
	}

	if err := os.WriteFile(s.cfg.ConfigPath, data, 0644); err != nil {
		return fmt.Errorf("service.singbox.WriteConfig write: %w", err)
	}

	s.logger.Info("конфиг sing-box записан", "path", s.cfg.ConfigPath)
	return nil
}

func (s *SingBoxService) Reload() error {
	if err := s.reloadViaClashAPI(); err == nil {
		s.logger.Info("sing-box перезагружен через Clash API")
		return nil
	}

	s.logger.Warn("Clash API reload не удался, fallback на docker restart")
	cmd := exec.Command("docker", "restart", "smarttraffic-singbox")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("service.singbox.Reload docker restart: %w", err)
	}
	s.logger.Info("sing-box перезагружен через docker restart")
	return nil
}

func (s *SingBoxService) reloadViaClashAPI() error {
	body := fmt.Sprintf(`{"path":"%s"}`, s.cfg.ConfigPath)
	req, err := http.NewRequest(http.MethodPut, "http://"+s.cfg.ClashAPIAddr+"/configs", bytes.NewReader([]byte(body)))
	if err != nil {
		return fmt.Errorf("создание запроса: %w", err)
	}
	if s.cfg.ClashAPISecret != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.ClashAPISecret)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("выполнение запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("статус %d от Clash API /configs", resp.StatusCode)
	}
	return nil
}

func (s *SingBoxService) WriteConfigAndReload(ctx context.Context) error {
	if err := s.WriteConfig(ctx); err != nil {
		return err
	}

	if err := s.Reload(); err != nil {
		s.logger.Error("ошибка перезагрузки sing-box", "error", err)
		return err
	}

	return nil
}

func (s *SingBoxService) buildDNSConfig(settings *models.DNSSettings) *singBoxDNS {
	var servers []singBoxDNSServer
	var rules []any
	ruTags := []string{}
	foreignTags := []string{}
	for _, addr := range splitList(settings.UpstreamRU) {
		tag := "dns-ru-" + addr
		servers = append(servers, singBoxDNSServer{Tag: tag, Type: "udp", Server: addr})
		ruTags = append(ruTags, tag)
	}
	hasForeignOut := s.srvConfig.ForeignIP != "" && s.srvConfig.ForeignVLESS.UUID != ""
	for _, addr := range splitList(settings.UpstreamForeign) {
		tag := "dns-foreign-" + addr
		srv := singBoxDNSServer{Tag: tag, Type: "udp", Server: addr}
		if hasForeignOut {
			srv.Detour = "foreign-out"
		}
		servers = append(servers, srv)
		foreignTags = append(foreignTags, tag)
	}

	if len(ruTags) > 0 {
		rules = append(rules, map[string]any{"server": ruTags[0]})
	}
	if len(foreignTags) > 0 {
		rules = append(rules, map[string]any{"server": foreignTags[0]})
	}

	finalTag := ""
	if len(foreignTags) > 0 {
		finalTag = foreignTags[0]
	} else if len(ruTags) > 0 {
		finalTag = ruTags[0]
	}

	return &singBoxDNS{
		Servers:  servers,
		Rules:    rules,
		Final:    finalTag,
		Strategy: "prefer_ipv4",
	}
}

func (s *SingBoxService) actionToOutbound(action string) string {
	switch action {
	case "direct":
		return "direct-out"
	case "proxy":
		return "foreign-out"
	}
	return ""
}

func splitList(s string) []string {
	var result []string
	for _, item := range splitComma(s) {
		result = append(result, item)
	}
	return result
}

func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	return splitString(s, ",")
}

func splitString(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			part := s[start:i]
			if part != "" {
				result = append(result, part)
			}
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
