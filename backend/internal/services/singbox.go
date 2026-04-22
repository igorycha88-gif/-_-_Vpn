package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
)

type SingBoxService struct {
	routeRepo  repository.RouteRepository
	dnsRepo    repository.DNSRepository
	cfg        *config.SingBoxConfig
	wgConfig   *config.WGConfig
	srvConfig  *config.ServerConfig
	logger     *slog.Logger
}

func NewSingBoxService(
	routeRepo repository.RouteRepository,
	dnsRepo repository.DNSRepository,
	cfg *config.SingBoxConfig,
	wgConfig *config.WGConfig,
	srvConfig *config.ServerConfig,
	logger *slog.Logger,
) *SingBoxService {
	return &SingBoxService{
		routeRepo: routeRepo,
		dnsRepo:   dnsRepo,
		cfg:       cfg,
		wgConfig:  wgConfig,
		srvConfig: srvConfig,
		logger:    logger,
	}
}

type SingBoxConfig struct {
	Log       *SingBoxLog       `json:"log,omitempty"`
	DNS       *SingBoxDNS       `json:"dns,omitempty"`
	Inbounds  []SingBoxInbound  `json:"inbounds"`
	Endpoints []SingBoxEndpoint `json:"endpoints,omitempty"`
	Outbounds []SingBoxOutbound `json:"outbounds"`
	Route     *SingBoxRoute     `json:"route"`
}

type SingBoxLog struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp,omitempty"`
}

type SingBoxDNS struct {
	Servers  []SingBoxDNSServer `json:"servers"`
	Rules    []SingBoxDNSRule   `json:"rules,omitempty"`
	Final    string             `json:"final,omitempty"`
	Strategy string             `json:"strategy,omitempty"`
}

type SingBoxDNSServer struct {
	Tag    string `json:"tag"`
	Type   string `json:"type"`
	Server string `json:"server"`
}

type SingBoxDNSRule struct {
	Server   string   `json:"server"`
	Outbound []string `json:"outbound,omitempty"`
}

type SingBoxInbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listen_port,omitempty"`
}

type SingBoxEndpoint struct {
	Type        string              `json:"type"`
	Tag         string              `json:"tag"`
	Address     []string            `json:"address"`
	PrivateKey  string              `json:"private_key"`
	MTU         int                 `json:"mtu,omitempty"`
	Peers       []SingBoxWGPeer     `json:"peers"`
}

type SingBoxWGPeer struct {
	Address    string `json:"address"`
	Port       int    `json:"port"`
	PublicKey  string `json:"public_key"`
	AllowedIPs []string `json:"allowed_ips"`
	Reserved   []int  `json:"reserved,omitempty"`
}

type SingBoxOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

type SingBoxRoute struct {
	Rules               []SingBoxRouteRule `json:"rules"`
	Final               string             `json:"final"`
	AutoDetectInterface bool               `json:"auto_detect_interface"`
}

type SingBoxRouteRule struct {
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	Domain        []string `json:"domain,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	IPCIDR        []string `json:"ip_cidr,omitempty"`
	GeoIP         []string `json:"geoip,omitempty"`
	Port          []int    `json:"port,omitempty"`
	Protocol      string   `json:"protocol,omitempty"`
	Action        string   `json:"action,omitempty"`
	Outbound      string   `json:"outbound,omitempty"`
}

func (s *SingBoxService) GenerateConfig(ctx context.Context) (*SingBoxConfig, error) {
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

	cfg := &SingBoxConfig{
		Log: &SingBoxLog{Level: "info", Timestamp: true},
		Inbounds: []SingBoxInbound{
			{
				Type:       "tproxy",
				Tag:        "tproxy-in",
				Listen:     "::",
				ListenPort: 12345,
			},
		},
		Outbounds: []SingBoxOutbound{
			{Type: "direct", Tag: "direct-out"},
		},
		Route: &SingBoxRoute{
			Rules: []SingBoxRouteRule{
				{Action: "sniff"},
				{Protocol: "dns", Action: "hijack-dns"},
			},
			Final:               "direct-out",
			AutoDetectInterface: true,
		},
	}

	cfg.DNS = s.buildDNSConfig(dnsSettings)

	if s.srvConfig.ForeignIP != "" {
		cfg.Endpoints = []SingBoxEndpoint{
			{
				Type:       "wireguard",
				Tag:        "foreign-out",
				Address:    []string{s.wgConfig.TunnelLocalAddress},
				PrivateKey: s.wgConfig.TunnelPrivateKey,
				MTU:        s.wgConfig.MTU,
				Peers: []SingBoxWGPeer{
					{
						Address:    s.srvConfig.ForeignIP,
						Port:       51821,
						PublicKey:  s.wgConfig.TunnelPeerPublicKey,
						AllowedIPs: []string{"0.0.0.0/0"},
						Reserved:   []int{0, 0, 0},
					},
				},
			},
		}
		cfg.Route.Final = "foreign-out"
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}

		if rule.Action == "block" {
			routeRule := SingBoxRouteRule{Action: "reject"}
			s.populateRouteRuleFields(&routeRule, rule)
			cfg.Route.Rules = append(cfg.Route.Rules, routeRule)
			continue
		}

		outbound := s.actionToOutbound(rule.Action)
		if outbound == "" {
			continue
		}

		routeRule := SingBoxRouteRule{Outbound: outbound}
		s.populateRouteRuleFields(&routeRule, rule)
		cfg.Route.Rules = append(cfg.Route.Rules, routeRule)
	}

	return cfg, nil
}

func (s *SingBoxService) populateRouteRuleFields(routeRule *SingBoxRouteRule, rule *models.Route) {
	switch rule.Type {
	case "domain":
		routeRule.Domain = []string{rule.Pattern}
	case "domain_suffix":
		routeRule.DomainSuffix = []string{rule.Pattern}
	case "domain_keyword":
		routeRule.DomainKeyword = []string{rule.Pattern}
	case "ip":
		routeRule.IPCIDR = []string{rule.Pattern}
	case "geoip":
		routeRule.GeoIP = []string{rule.Pattern}
	case "port":
		var port int
		fmt.Sscanf(rule.Pattern, "%d", &port)
		if port > 0 {
			routeRule.Port = []int{port}
		}
	case "regex":
		routeRule.Domain = []string{"regexp:" + rule.Pattern}
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
	cmd := exec.Command("docker", "kill", "-s", "SIGHUP", "smarttraffic-singbox")
	if err := cmd.Run(); err != nil {
		s.logger.Warn("не удалось отправить SIGHUP sing-box, попытка restart")
		cmd = exec.Command("docker", "restart", "smarttraffic-singbox")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("service.singbox.Reload: %w", err)
		}
	}
	s.logger.Info("sing-box перезагружен")
	return nil
}

func (s *SingBoxService) WriteConfigAndReload(ctx context.Context) error {
	if err := s.WriteConfig(ctx); err != nil {
		return err
	}

	go func() {
		if err := s.Reload(); err != nil {
			s.logger.Error("ошибка hot-reload sing-box", "error", err)
		}
	}()

	return nil
}

func (s *SingBoxService) buildDNSConfig(settings *models.DNSSettings) *SingBoxDNS {
	var servers []SingBoxDNSServer
	ruTags := []string{}
	foreignTags := []string{}
	for _, addr := range splitList(settings.UpstreamRU) {
		tag := "dns-ru-" + addr
		servers = append(servers, SingBoxDNSServer{Tag: tag, Type: "udp", Server: addr})
		ruTags = append(ruTags, tag)
	}
	for _, addr := range splitList(settings.UpstreamForeign) {
		tag := "dns-foreign-" + addr
		servers = append(servers, SingBoxDNSServer{Tag: tag, Type: "udp", Server: addr})
		foreignTags = append(foreignTags, tag)
	}

	var rules []SingBoxDNSRule
	if len(ruTags) > 0 {
		rules = append(rules, SingBoxDNSRule{
			Server:   ruTags[0],
			Outbound: []string{"direct-out"},
		})
	}
	if len(foreignTags) > 0 {
		rules = append(rules, SingBoxDNSRule{
			Server:   foreignTags[0],
			Outbound: []string{"foreign-out"},
		})
	}

	finalTag := ""
	if len(foreignTags) > 0 {
		finalTag = foreignTags[0]
	} else if len(ruTags) > 0 {
		finalTag = ruTags[0]
	}

	return &SingBoxDNS{
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
