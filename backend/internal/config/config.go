package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	App     AppConfig
	DB      DBConfig
	JWT     JWTConfig
	WG      WGConfig
	VLESS   VLESSConfig
	Server  ServerConfig
	SingBox SingBoxConfig
	CORS    CORSConfig
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
	Interface           string
	Port                int
	TunnelInterface     string
	TunnelPrivateKey    string
	TunnelPeerPublicKey string
	TunnelLocalAddress  string
	MTU                 int
}

type VLESSConfig struct {
	PrivateKey     string
	PublicKey      string
	ShortID        string
	ServerName     string
	Port           int
	Flow           string
	Fingerprint    string
	ServerEndpoint string
}

type ForeignVLESSConfig struct {
	UUID             string
	RealityPublicKey string
	RealityShortID   string
	ServerName       string
}

type ServerConfig struct {
	ForeignIP    string
	ForeignVLESS ForeignVLESSConfig
}

type SingBoxConfig struct {
	ConfigPath     string
	ClashAPIAddr   string
	ClashAPISecret string
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
	cfg.WG.TunnelPrivateKey = getEnv("FOREIGN_TUNNEL_PRIVATE_KEY", "")
	cfg.WG.TunnelPeerPublicKey = getEnv("FOREIGN_TUNNEL_PEER_PUBLIC_KEY", "")
	cfg.WG.TunnelLocalAddress = getEnv("FOREIGN_TUNNEL_LOCAL_ADDRESS", "10.20.0.2/30")
	cfg.WG.MTU = getEnvInt("WG_MTU", 1280)
	cfg.WG.TunnelInterface = getEnv("WG_TUNNEL_INTERFACE", "wg1")

	cfg.VLESS.PrivateKey = getEnv("VLESS_PRIVATE_KEY", "")
	cfg.VLESS.PublicKey = getEnv("VLESS_PUBLIC_KEY", "")
	cfg.VLESS.ShortID = getEnv("VLESS_SHORT_ID", "")
	cfg.VLESS.ServerName = getEnv("VLESS_SERVER_NAME", "www.microsoft.com")
	cfg.VLESS.Port = getEnvInt("VLESS_PORT", 443)
	cfg.VLESS.Flow = getEnv("VLESS_FLOW", "xtls-rprx-vision")
	cfg.VLESS.Fingerprint = getEnv("VLESS_FINGERPRINT", "chrome")
	cfg.VLESS.ServerEndpoint = getEnv("VLESS_SERVER_ENDPOINT", "")

	cfg.Server.ForeignIP = getEnv("FOREIGN_SERVER_IP", "")

	cfg.Server.ForeignVLESS.UUID = getEnv("FOREIGN_VLESS_UUID", "")
	cfg.Server.ForeignVLESS.RealityPublicKey = getEnv("FOREIGN_VLESS_REALITY_PUBLIC_KEY", "")
	cfg.Server.ForeignVLESS.RealityShortID = getEnv("FOREIGN_VLESS_REALITY_SHORT_ID", "")
	cfg.Server.ForeignVLESS.ServerName = getEnv("FOREIGN_VLESS_SERVER_NAME", "www.microsoft.com")

	cfg.SingBox.ConfigPath = getEnv("SINGBOX_CONFIG_PATH", "/etc/singbox/config.json")
	cfg.SingBox.ClashAPIAddr = getEnv("SINGBOX_CLASH_API_ADDR", "127.0.0.1:9090")
	cfg.SingBox.ClashAPISecret = getEnv("SINGBOX_CLASH_API_SECRET", "")

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
