package config

import (
	"os"
	"testing"
	"time"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func TestLoad_AllEnvVarsSet(t *testing.T) {
	setEnv(t, "APP_PORT", "9090")
	setEnv(t, "DB_PATH", "/tmp/test.db")
	setEnv(t, "JWT_SECRET", "super-secret-key-at-least-32-chars!")
	setEnv(t, "JWT_ACCESS_TTL", "30m")
	setEnv(t, "JWT_REFRESH_TTL", "336h")
	setEnv(t, "FOREIGN_TUNNEL_PRIVATE_KEY", "wg-priv-key")
	setEnv(t, "FOREIGN_TUNNEL_PEER_PUBLIC_KEY", "wg-peer-pub")
	setEnv(t, "FOREIGN_TUNNEL_LOCAL_ADDRESS", "10.30.0.2/30")
	setEnv(t, "WG_TUNNEL_INTERFACE", "wg2")
	setEnv(t, "VLESS_PRIVATE_KEY", "vless-priv")
	setEnv(t, "VLESS_PUBLIC_KEY", "vless-pub")
	setEnv(t, "VLESS_SHORT_ID", "abcdef12")
	setEnv(t, "VLESS_SERVER_NAME", "www.google.com")
	setEnv(t, "VLESS_PORT", "9443")
	setEnv(t, "VLESS_FLOW", "xtls-rprx-vision")
	setEnv(t, "VLESS_FINGERPRINT", "firefox")
	setEnv(t, "VLESS_SERVER_ENDPOINT", "5.6.7.8")
	setEnv(t, "FOREIGN_SERVER_IP", "5.6.7.8")
	setEnv(t, "FOREIGN_VLESS_UUID", "uuid-1234")
	setEnv(t, "FOREIGN_VLESS_REALITY_PUBLIC_KEY", "reality-pk")
	setEnv(t, "FOREIGN_VLESS_REALITY_SHORT_ID", "reality-sid")
	setEnv(t, "FOREIGN_VLESS_SERVER_NAME", "www.apple.com")
	setEnv(t, "SINGBOX_CONFIG_PATH", "/tmp/singbox.json")
	setEnv(t, "SINGBOX_CLASH_API_ADDR", "0.0.0.0:9091")
	setEnv(t, "SINGBOX_CLASH_API_SECRET", "clash-secret")
	setEnv(t, "CORS_ALLOWED_ORIGINS", "http://localhost:5173")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.App.Port != 9090 {
		t.Errorf("App.Port = %d, want 9090", cfg.App.Port)
	}
	if cfg.DB.Path != "/tmp/test.db" {
		t.Errorf("DB.Path = %q, want /tmp/test.db", cfg.DB.Path)
	}
	if cfg.JWT.Secret != "super-secret-key-at-least-32-chars!" {
		t.Errorf("JWT.Secret = %q, unexpected", cfg.JWT.Secret)
	}
	if cfg.JWT.AccessTTL != 30*time.Minute {
		t.Errorf("JWT.AccessTTL = %v, want 30m", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.RefreshTTL != 336*time.Hour {
		t.Errorf("JWT.RefreshTTL = %v, want 336h", cfg.JWT.RefreshTTL)
	}
	if cfg.WG.TunnelPrivateKey != "wg-priv-key" {
		t.Errorf("WG.TunnelPrivateKey = %q, unexpected", cfg.WG.TunnelPrivateKey)
	}
	if cfg.WG.TunnelPeerPublicKey != "wg-peer-pub" {
		t.Errorf("WG.TunnelPeerPublicKey = %q, unexpected", cfg.WG.TunnelPeerPublicKey)
	}
	if cfg.WG.TunnelLocalAddress != "10.30.0.2/30" {
		t.Errorf("WG.TunnelLocalAddress = %q, unexpected", cfg.WG.TunnelLocalAddress)
	}
	if cfg.WG.TunnelInterface != "wg2" {
		t.Errorf("WG.TunnelInterface = %q, unexpected", cfg.WG.TunnelInterface)
	}
	if cfg.VLESS.PrivateKey != "vless-priv" {
		t.Errorf("VLESS.PrivateKey = %q, unexpected", cfg.VLESS.PrivateKey)
	}
	if cfg.VLESS.PublicKey != "vless-pub" {
		t.Errorf("VLESS.PublicKey = %q, unexpected", cfg.VLESS.PublicKey)
	}
	if cfg.VLESS.ShortID != "abcdef12" {
		t.Errorf("VLESS.ShortID = %q, unexpected", cfg.VLESS.ShortID)
	}
	if cfg.VLESS.ServerName != "www.google.com" {
		t.Errorf("VLESS.ServerName = %q, unexpected", cfg.VLESS.ServerName)
	}
	if cfg.VLESS.Port != 9443 {
		t.Errorf("VLESS.Port = %d, want 9443", cfg.VLESS.Port)
	}
	if cfg.VLESS.Flow != "xtls-rprx-vision" {
		t.Errorf("VLESS.Flow = %q, unexpected", cfg.VLESS.Flow)
	}
	if cfg.VLESS.Fingerprint != "firefox" {
		t.Errorf("VLESS.Fingerprint = %q, unexpected", cfg.VLESS.Fingerprint)
	}
	if cfg.VLESS.ServerEndpoint != "5.6.7.8" {
		t.Errorf("VLESS.ServerEndpoint = %q, unexpected", cfg.VLESS.ServerEndpoint)
	}
	if cfg.Server.ForeignIP != "5.6.7.8" {
		t.Errorf("Server.ForeignIP = %q, unexpected", cfg.Server.ForeignIP)
	}
	if cfg.Server.ForeignVLESS.UUID != "uuid-1234" {
		t.Errorf("Server.ForeignVLESS.UUID = %q, unexpected", cfg.Server.ForeignVLESS.UUID)
	}
	if cfg.Server.ForeignVLESS.RealityPublicKey != "reality-pk" {
		t.Errorf("Server.ForeignVLESS.RealityPublicKey = %q, unexpected", cfg.Server.ForeignVLESS.RealityPublicKey)
	}
	if cfg.Server.ForeignVLESS.RealityShortID != "reality-sid" {
		t.Errorf("Server.ForeignVLESS.RealityShortID = %q, unexpected", cfg.Server.ForeignVLESS.RealityShortID)
	}
	if cfg.Server.ForeignVLESS.ServerName != "www.apple.com" {
		t.Errorf("Server.ForeignVLESS.ServerName = %q, unexpected", cfg.Server.ForeignVLESS.ServerName)
	}
	if cfg.SingBox.ConfigPath != "/tmp/singbox.json" {
		t.Errorf("SingBox.ConfigPath = %q, unexpected", cfg.SingBox.ConfigPath)
	}
	if cfg.SingBox.ClashAPIAddr != "0.0.0.0:9091" {
		t.Errorf("SingBox.ClashAPIAddr = %q, unexpected", cfg.SingBox.ClashAPIAddr)
	}
	if cfg.SingBox.ClashAPISecret != "clash-secret" {
		t.Errorf("SingBox.ClashAPISecret = %q, unexpected", cfg.SingBox.ClashAPISecret)
	}
	if cfg.CORS.AllowedOrigins != "http://localhost:5173" {
		t.Errorf("CORS.AllowedOrigins = %q, unexpected", cfg.CORS.AllowedOrigins)
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Clearenv()
	setEnv(t, "JWT_SECRET", "at-least-some-secret-value-here")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}

	if cfg.App.Port != 8080 {
		t.Errorf("App.Port default = %d, want 8080", cfg.App.Port)
	}
	if cfg.DB.Path != "/data/smarttraffic.db" {
		t.Errorf("DB.Path default = %q, want /data/smarttraffic.db", cfg.DB.Path)
	}
	if cfg.JWT.AccessTTL != 15*time.Minute {
		t.Errorf("JWT.AccessTTL default = %v, want 15m", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Errorf("JWT.RefreshTTL default = %v, want 168h", cfg.JWT.RefreshTTL)
	}
	if cfg.WG.TunnelInterface != "wg1" {
		t.Errorf("WG.TunnelInterface default = %q, want wg1", cfg.WG.TunnelInterface)
	}
	if cfg.WG.TunnelLocalAddress != "10.20.0.2/30" {
		t.Errorf("WG.TunnelLocalAddress default = %q, want 10.20.0.2/30", cfg.WG.TunnelLocalAddress)
	}
	if cfg.VLESS.ServerName != "www.microsoft.com" {
		t.Errorf("VLESS.ServerName default = %q, want www.microsoft.com", cfg.VLESS.ServerName)
	}
	if cfg.VLESS.Port != 8443 {
		t.Errorf("VLESS.Port default = %d, want 8443", cfg.VLESS.Port)
	}
	if cfg.VLESS.Flow != "xtls-rprx-vision" {
		t.Errorf("VLESS.Flow default = %q, unexpected", cfg.VLESS.Flow)
	}
	if cfg.VLESS.Fingerprint != "chrome" {
		t.Errorf("VLESS.Fingerprint default = %q, want chrome", cfg.VLESS.Fingerprint)
	}
	if cfg.Server.ForeignVLESS.ServerName != "www.microsoft.com" {
		t.Errorf("Server.ForeignVLESS.ServerName default = %q, want www.microsoft.com", cfg.Server.ForeignVLESS.ServerName)
	}
	if cfg.SingBox.ConfigPath != "/etc/singbox/config.json" {
		t.Errorf("SingBox.ConfigPath default = %q, unexpected", cfg.SingBox.ConfigPath)
	}
	if cfg.SingBox.ClashAPIAddr != "127.0.0.1:9090" {
		t.Errorf("SingBox.ClashAPIAddr default = %q, unexpected", cfg.SingBox.ClashAPIAddr)
	}
	if cfg.CORS.AllowedOrigins != "http://localhost:3000" {
		t.Errorf("CORS.AllowedOrigins default = %q, want http://localhost:3000", cfg.CORS.AllowedOrigins)
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	os.Clearenv()

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is missing")
	}
}

func TestLoad_EmptyJWTSecret(t *testing.T) {
	os.Clearenv()
	setEnv(t, "JWT_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is empty")
	}
}

func TestLoad_InvalidAccessTTL(t *testing.T) {
	os.Clearenv()
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "JWT_ACCESS_TTL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JWT_ACCESS_TTL")
	}
}

func TestLoad_InvalidRefreshTTL(t *testing.T) {
	os.Clearenv()
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "JWT_REFRESH_TTL", "bad")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid JWT_REFRESH_TTL")
	}
}

func TestGetEnv_Fallback(t *testing.T) {
	os.Clearenv()
	result := getEnv("NONEXISTENT_KEY_XYZ", "fallback_val")
	if result != "fallback_val" {
		t.Errorf("getEnv with unset key = %q, want fallback_val", result)
	}
}

func TestGetEnv_SetValue(t *testing.T) {
	setEnv(t, "TEST_GETENV_KEY", "myval")
	result := getEnv("TEST_GETENV_KEY", "default")
	if result != "myval" {
		t.Errorf("getEnv = %q, want myval", result)
	}
}

func TestGetEnv_EmptyValueUsesFallback(t *testing.T) {
	os.Clearenv()
	os.Setenv("TEST_EMPTY_KEY", "")
	result := getEnv("TEST_EMPTY_KEY", "fallback")
	if result != "fallback" {
		t.Errorf("getEnv with empty value = %q, want fallback", result)
	}
}

func TestGetEnvInt_Fallback(t *testing.T) {
	os.Clearenv()
	result := getEnvInt("NONEXISTENT_INT_KEY_XYZ", 42)
	if result != 42 {
		t.Errorf("getEnvInt with unset key = %d, want 42", result)
	}
}

func TestGetEnvInt_SetValue(t *testing.T) {
	setEnv(t, "TEST_GETENVINT_KEY", "123")
	result := getEnvInt("TEST_GETENVINT_KEY", 0)
	if result != 123 {
		t.Errorf("getEnvInt = %d, want 123", result)
	}
}

func TestGetEnvInt_InvalidInteger(t *testing.T) {
	setEnv(t, "TEST_GETENVINT_BAD", "not-a-number")
	result := getEnvInt("TEST_GETENVINT_BAD", 99)
	if result != 99 {
		t.Errorf("getEnvInt with invalid value = %d, want fallback 99", result)
	}
}

func TestGetEnvInt_EmptyValue(t *testing.T) {
	os.Clearenv()
	os.Setenv("TEST_INT_EMPTY", "")
	result := getEnvInt("TEST_INT_EMPTY", 77)
	if result != 77 {
		t.Errorf("getEnvInt with empty value = %d, want 77", result)
	}
}

func TestLoad_TTLParsing(t *testing.T) {
	tests := []struct {
		name      string
		accessTTL string
		refreshTTL string
		wantAccess  time.Duration
		wantRefresh time.Duration
	}{
		{"hours", "1h", "48h", 1 * time.Hour, 48 * time.Hour},
		{"seconds", "30s", "60s", 30 * time.Second, 60 * time.Second},
		{"composite", "1h30m", "2h15m30s", 90 * time.Minute, 2*time.Hour + 15*time.Minute + 30*time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			setEnv(t, "JWT_SECRET", "secret")
			setEnv(t, "JWT_ACCESS_TTL", tt.accessTTL)
			setEnv(t, "JWT_REFRESH_TTL", tt.refreshTTL)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load(): %v", err)
			}
			if cfg.JWT.AccessTTL != tt.wantAccess {
				t.Errorf("AccessTTL = %v, want %v", cfg.JWT.AccessTTL, tt.wantAccess)
			}
			if cfg.JWT.RefreshTTL != tt.wantRefresh {
				t.Errorf("RefreshTTL = %v, want %v", cfg.JWT.RefreshTTL, tt.wantRefresh)
			}
		})
	}
}

func TestLoad_ServerPortCustom(t *testing.T) {
	os.Clearenv()
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "APP_PORT", "3000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.App.Port != 3000 {
		t.Errorf("App.Port = %d, want 3000", cfg.App.Port)
	}
}

func TestLoad_VLESSPortInvalid(t *testing.T) {
	os.Clearenv()
	setEnv(t, "JWT_SECRET", "secret")
	setEnv(t, "VLESS_PORT", "abc")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load(): %v", err)
	}
	if cfg.VLESS.Port != 8443 {
		t.Errorf("VLESS.Port with invalid value = %d, want default 8443", cfg.VLESS.Port)
	}
}
