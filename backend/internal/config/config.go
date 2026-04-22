package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App    AppConfig
	DB     DBConfig
	JWT    JWTConfig
	WG     WGConfig
	Server ServerConfig
	SingBox SingBoxConfig
	CORS   CORSConfig
}

type AppConfig struct {
	Port int
}

type DBConfig struct {
	Path string
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type WGConfig struct {
	Interface            string
	Port                 int
	ServerEndpoint       string
	ServerPubKey         string
	ClientSubnet         string
	TunnelSubnet         string
	MTU                  int
	DNS                  string
	TunnelPrivateKey     string
	TunnelPeerPublicKey  string
	TunnelLocalAddress   string
}

type ServerConfig struct {
	ForeignIP string
}

type SingBoxConfig struct {
	ConfigPath string
}

type CORSConfig struct {
	AllowedOrigins string
}

func Load() (*Config, error) {
	cfg := &Config{}

	cfg.App.Port = getEnvInt("APP_PORT", 8080)

	cfg.DB.Path = getEnv("DB_PATH", "/data/smarttraffic.db")

	cfg.JWT.Secret = getEnv("JWT_SECRET", "")
	if cfg.JWT.Secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	accessTTL, err := time.ParseDuration(getEnv("JWT_ACCESS_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TTL: %w", err)
	}
	cfg.JWT.AccessTTL = accessTTL

	refreshTTL, err := time.ParseDuration(getEnv("JWT_REFRESH_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TTL: %w", err)
	}
	cfg.JWT.RefreshTTL = refreshTTL

	cfg.WG.Interface = getEnv("WG_INTERFACE", "wg0")
	cfg.WG.Port = getEnvInt("WG_PORT", 51820)
	cfg.WG.ServerEndpoint = getEnv("WG_SERVER_ENDPOINT", "")
	cfg.WG.ServerPubKey = getEnv("WG_SERVER_PUBLIC_KEY", "")
	cfg.WG.ClientSubnet = getEnv("WG_CLIENT_SUBNET", "10.10.0.0/24")
	cfg.WG.TunnelSubnet = getEnv("WG_TUNNEL_SUBNET", "10.20.0.0/30")
	cfg.WG.MTU = getEnvInt("WG_MTU", 1280)
	cfg.WG.DNS = getEnv("WG_DNS", "10.10.0.1")
	cfg.WG.TunnelPrivateKey = getEnv("FOREIGN_TUNNEL_PRIVATE_KEY", "")
	cfg.WG.TunnelPeerPublicKey = getEnv("FOREIGN_TUNNEL_PEER_PUBLIC_KEY", "")
	cfg.WG.TunnelLocalAddress = getEnv("FOREIGN_TUNNEL_LOCAL_ADDRESS", "10.20.0.2/30")

	cfg.Server.ForeignIP = getEnv("FOREIGN_SERVER_IP", "")

	cfg.SingBox.ConfigPath = getEnv("SINGBOX_CONFIG_PATH", "/etc/singbox/config.json")

	cfg.CORS.AllowedOrigins = getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err == nil {
			return i
		}
	}
	return fallback
}
