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
	routeRepo repository.RouteRepository
	dnsRepo   repository.DNSRepository
	cfg       *config.SingBoxConfig
	wgConfig  *config.WGConfig
	srvConfig *config.ServerConfig
	logger    *slog.Logger
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
	Outbounds []SingBoxOutbound `json:"outbounds"`
	Route     *SingBoxRoute     `json:"route"`
}

type SingBoxLog struct {
	Level string `json:"level"`
}

type SingBoxDNS struct {
	Servers []SingBoxDNSServer `json:"servers"`
	Rules   []SingBoxDNSRule   `json:"rules,omitempty"`
}

type SingBoxDNSServer struct {
	Tag     string `json:"tag"`
	Address string `json:"address"`
}

type SingBoxDNSRule struct {
	Server  string   `json:"server"`
	Outbound []string `json:"outbound,omitempty"`
}

type SingBoxInbound struct {
	Type        string `json:"type"`
	Tag         string `json:"tag"`
	Listen      string `json:"listen"`
	ListenPort  int    `json:"listen_port,omitempty"`
	Sniff       bool   `json:"sniff,omitempty"`
	SniffOverrideDestination bool `json:"sniff_override_destination,omitempty"`
}

type SingBoxOutbound struct {
	Type          string   `json:"type"`
	Tag           string   `json:"tag"`
	Server        string   `json:"server,omitempty"`
	ServerPort    int      `json:"server_port,omitempty"`
	LocalAddress  []string `json:"local_address,omitempty"`
	PrivateKey    string   `json:"private_key,omitempty"`
	PeerPublicKey string   `json:"peer_public_key,omitempty"`
	Reserved      []int    `json:"reserved,omitempty"`
	MTU           int      `json:"mtu,omitempty"`
}

type SingBoxRoute struct {
	Rules          []SingBoxRouteRule `json:"rules"`
	Final          string             `json:"final"`
	AutoDetectInterface bool          `json:"auto_detect_interface"`
}

type SingBoxRouteRule struct {
	DomainSuffix  []string `json:"domain_suffix,omitempty"`
	Domain        []string `json:"domain,omitempty"`
	DomainKeyword []string `json:"domain_keyword,omitempty"`
	IPCIDR        []string `json:"ip_cidr,omitempty"`
	GeoIP         []string `json:"geoip,omitempty"`
	Port          []int    `json:"port,omitempty"`
	Outbound      string   `json:"outbound"`
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
			UpstreamRU:     "77.88.8.8,77.88.8.1",
			UpstreamForeign: "1.1.1.1,8.8.8.8",
		}
	}

	cfg := &SingBoxConfig{
		Log: &SingBoxLog{Level: "info"},
		Inbounds: []SingBoxInbound{
			{
				Type:       "tproxy",
				Tag:        "tproxy-in",
				Listen:     "::",
				ListenPort: 12345,
				Sniff:      true,
				SniffOverrideDestination: true,
			},
		},
		Outbounds: []SingBoxOutbound{
			{Type: "direct", Tag: "direct-out"},
		},
		Route: &SingBoxRoute{
			Rules:                []SingBoxRouteRule{},
			Final:                "direct-out",
			AutoDetectInterface:  true,
		},
	}

	cfg.DNS = s.buildDNSConfig(dnsSettings)

	if s.srvConfig.ForeignIP != "" {
		cfg.Outbounds = append(cfg.Outbounds, SingBoxOutbound{
			Type:          "wireguard",
			Tag:           "foreign-out",
			Server:        s.srvConfig.ForeignIP,
			ServerPort:    51821,
			LocalAddress:  []string{"10.20.0.2/30"},
			PrivateKey:    "",
			PeerPublicKey: "",
			Reserved:      []int{0, 0, 0},
			MTU:           1280,
		})
		cfg.Route.Final = "foreign-out"
	}

	for _, rule := range rules {
		if !rule.IsActive {
			continue
		}
		outbound := s.actionToOutbound(rule.Action)
		if outbound == "" {
			continue
		}

		routeRule := SingBoxRouteRule{Outbound: outbound}
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
		cfg.Route.Rules = append(cfg.Route.Rules, routeRule)
	}

	return cfg, nil
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

func (s *SingBoxService) buildDNSConfig(settings *models.DNSSettings) *SingBoxDNS {
	var servers []SingBoxDNSServer
	for _, addr := range splitList(settings.UpstreamRU) {
		servers = append(servers, SingBoxDNSServer{Tag: "dns-ru-" + addr, Address: addr})
	}
	for _, addr := range splitList(settings.UpstreamForeign) {
		servers = append(servers, SingBoxDNSServer{Tag: "dns-foreign-" + addr, Address: addr})
	}
	return &SingBoxDNS{Servers: servers}
}

func (s *SingBoxService) actionToOutbound(action string) string {
	switch action {
	case "direct":
		return "direct-out"
	case "proxy":
		return "foreign-out"
	case "block":
		return "block"
	}
	return ""
}

func splitList(s string) []string {
	var result []string
	for _, v := range []string{} {
		_ = v
	}
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
