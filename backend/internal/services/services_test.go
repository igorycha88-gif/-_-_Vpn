package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
	"smarttraffic/migrations"

	"golang.org/x/crypto/bcrypt"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func testJWTConfig() *config.JWTConfig {
	return &config.JWTConfig{
		Secret:     "test-secret-key-at-least-32-chars!",
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 168 * time.Hour,
	}
}

func testVLESSConfig() *config.VLESSConfig {
	return &config.VLESSConfig{
		PrivateKey:     "test-private-key",
		PublicKey:      "test-public-key",
		ShortID:        "test-short-id",
		ServerName:     "www.microsoft.com",
		Port:           443,
		Flow:           "xtls-rprx-vision",
		Fingerprint:    "chrome",
		ServerEndpoint: "1.2.3.4",
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	tokens, err := svc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("AccessToken is empty")
	}
	if tokens.RefreshToken == "" {
		t.Error("RefreshToken is empty")
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	_, err := svc.Login(context.Background(), "admin@smarttraffic.local", "wrongpassword")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_WrongEmail(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	_, err := svc.Login(context.Background(), "no@no.com", "admin123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_ValidateAccessToken(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())
	tokens, err := svc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	claims, err := svc.ValidateAccessToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.Email != "admin@smarttraffic.local" {
		t.Errorf("Email = %q, want admin@smarttraffic.local", claims.Email)
	}
	if claims.Role != "admin" {
		t.Errorf("Role = %q, want admin", claims.Role)
	}
}

func TestAuthService_ValidateAccessToken_Invalid(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	_, err := svc.ValidateAccessToken("invalid.token.here")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestAuthService_GetSession(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	session, err := svc.GetSession(context.Background(), "admin-001")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if session.Email != "admin@smarttraffic.local" {
		t.Errorf("Email = %q, unexpected", session.Email)
	}
}

func TestAuthService_Logout(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())
	tokens, _ := svc.Login(context.Background(), "admin@smarttraffic.local", "admin123")

	err := svc.Logout(context.Background(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestWireGuardService_CreatePeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewWireGuardService(repository.NewPeerRepository(db), testVLESSConfig(), testLogger())

	peer, err := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Test Peer"})
	if err != nil {
		t.Fatalf("CreatePeer: %v", err)
	}
	if peer.ID == "" {
		t.Error("ID is empty")
	}
	if peer.PublicKey == "" {
		t.Error("PublicKey (UUID) is empty")
	}
	if peer.Address == "" {
		t.Error("Address is empty")
	}
	if !peer.IsActive {
		t.Error("should be active")
	}
	if peer.MTU != 1280 {
		t.Errorf("MTU = %d, want 1280", peer.MTU)
	}
}

func TestWireGuardService_CreatePeer_ValidationError(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewWireGuardService(repository.NewPeerRepository(db), testVLESSConfig(), testLogger())

	_, err := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWireGuardService_ListPeers(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewWireGuardService(repository.NewPeerRepository(db), testVLESSConfig(), testLogger())

	peers, err := svc.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("count = %d, want 0", len(peers))
	}

	svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1"})
	peers, _ = svc.ListPeers(context.Background())
	if len(peers) != 1 {
		t.Errorf("count = %d, want 1", len(peers))
	}
}

func TestWireGuardService_DeletePeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewWireGuardService(repository.NewPeerRepository(db), testVLESSConfig(), testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1"})
	if err := svc.DeletePeer(context.Background(), peer.ID); err != nil {
		t.Fatalf("DeletePeer: %v", err)
	}
}

func TestWireGuardService_GenerateClientConfig(t *testing.T) {
	svc := NewWireGuardService(nil, testVLESSConfig(), testLogger())

	peer := &models.Peer{
		PublicKey: "7f2105d9-3962-4dd3-80d5-6ac86d271855",
	}
	config := svc.GenerateClientConfig(peer)
	if config == "" {
		t.Fatal("config is empty")
	}
	if !contains(config, "vless") {
		t.Error("config should contain vless type")
	}
	if !contains(config, "7f2105d9-3962-4dd3-80d5-6ac86d271855") {
		t.Error("config should contain UUID")
	}
	if !contains(config, "max.ru") {
		t.Error("config should contain max.ru in direct rules")
	}
	if !contains(config, "gosuslugi.ru") {
		t.Error("config should contain gosuslugi.ru in direct rules")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestRoutingService_CreateRule(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
		repository.NewPresetRepository(db),
		testLogger(),
	)

	rule, err := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{
		Name: "Test", Type: "domain", Pattern: "example.com", Action: "direct",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	if rule.ID == "" {
		t.Error("ID is empty")
	}
}

func TestRoutingService_CreateRule_Validation(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
		repository.NewPresetRepository(db),
		testLogger(),
	)

	_, err := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRoutingService_UpdateRule(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
		repository.NewPresetRepository(db),
		testLogger(),
	)

	rule, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{
		Name: "Test", Type: "domain", Pattern: "example.com", Action: "direct",
	})

	newName := "Updated"
	updated, err := svc.UpdateRule(context.Background(), rule.ID, &models.RoutingRuleUpdateRequest{
		Name: &newName,
	})
	if err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("Name = %q, want Updated", updated.Name)
	}
}

func TestRoutingService_DeleteRule(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
		repository.NewPresetRepository(db),
		testLogger(),
	)

	rule, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{
		Name: "Test", Type: "domain", Pattern: "example.com", Action: "direct",
	})

	if err := svc.DeleteRule(context.Background(), rule.ID); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
}

func TestRoutingService_ApplyPreset(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
		repository.NewPresetRepository(db),
		testLogger(),
	)

	result, err := svc.ApplyPreset(context.Background(), "preset-all-direct")
	if err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if result.AppliedRules < 1 {
		t.Errorf("AppliedRules = %d, want >= 1", result.AppliedRules)
	}
}

func TestDNSService_GetSettings(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewDNSService(repository.NewDNSRepository(db), testLogger())

	settings, err := svc.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if settings.UpstreamRU == "" {
		t.Error("UpstreamRU should not be empty")
	}
}

func TestDNSService_UpdateSettings(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewDNSService(repository.NewDNSRepository(db), testLogger())

	newRU := "8.8.8.8"
	settings, err := svc.UpdateSettings(context.Background(), &models.DNSSettingsUpdateRequest{
		UpstreamRU: &newRU,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if settings.UpstreamRU != "8.8.8.8" {
		t.Errorf("UpstreamRU = %q, want 8.8.8.8", settings.UpstreamRU)
	}
}

func TestTrafficService_GetTotalStats(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	stats, err := svc.GetTotalStats(context.Background())
	if err != nil {
		t.Fatalf("GetTotalStats: %v", err)
	}
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
}

func TestTrafficService_GetTrafficLogs(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	logs, err := svc.GetTrafficLogs(context.Background(), models.TrafficFilter{Limit: 50})
	if err != nil {
		t.Fatalf("GetTrafficLogs: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected empty logs, got %d", len(logs))
	}
}

func TestTrafficService_Alerts(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	alerts, err := svc.GetAlerts(context.Background())
	if err != nil {
		t.Fatalf("GetAlerts: %v", err)
	}
	if alerts == nil {
		t.Fatal("alerts should not be nil")
	}
}

func TestBcryptPassword(t *testing.T) {
	password := "admin123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		t.Fatalf("bcrypt.Generate: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Fatalf("bcrypt.Compare: %v", err)
	}
}

func newTestSingBoxService(t *testing.T) (*SingBoxService, *sql.DB) {
	t.Helper()
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	routeRepo := repository.NewRouteRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	peerRepo := repository.NewPeerRepository(db)
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json"}
	vlessCfg := testVLESSConfig()
	wgCfg := &config.WGConfig{MTU: 1280, TunnelLocalAddress: "10.20.0.2/30", TunnelPrivateKey: "testkey", TunnelPeerPublicKey: "testpeerkey"}
	srvCfg := &config.ServerConfig{ForeignIP: "1.2.3.4"}
	svc := NewSingBoxService(routeRepo, dnsRepo, peerRepo, sbCfg, vlessCfg, wgCfg, srvCfg, testLogger())
	return svc, db
}

func TestSingBoxService_GenerateConfig_WithVLESSInbound(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	cfg, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}

	if cfg.Route.Final != "foreign-out" {
		t.Errorf("Final = %q, want foreign-out", cfg.Route.Final)
	}
	if len(cfg.Inbounds) == 0 {
		t.Error("expected at least one inbound")
	}
}

func TestSingBoxService_GenerateConfig_NoForeignIP(t *testing.T) {
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	routeRepo := repository.NewRouteRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	peerRepo := repository.NewPeerRepository(db)
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json"}
	vlessCfg := testVLESSConfig()
	wgCfg := &config.WGConfig{MTU: 1280}
	srvCfg := &config.ServerConfig{ForeignIP: ""}
	svc := NewSingBoxService(routeRepo, dnsRepo, peerRepo, sbCfg, vlessCfg, wgCfg, srvCfg, testLogger())

	result, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	if result.Route.Final != "direct-out" {
		t.Errorf("Final = %q, want direct-out", result.Route.Final)
	}
}

func TestSingBoxService_GenerateConfig_DNSRules(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	cfg, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	if cfg.DNS == nil {
		t.Fatal("DNS config is nil")
	}
	if len(cfg.DNS.Servers) == 0 {
		t.Error("DNS servers empty")
	}
}

func TestSingBoxService_WriteConfig(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	if err := svc.WriteConfig(context.Background()); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
}

func TestSingBoxService_ActionToOutbound(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	tests := []struct {
		action   string
		expected string
	}{
		{"direct", "direct-out"},
		{"proxy", "foreign-out"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := svc.actionToOutbound(tt.action)
		if got != tt.expected {
			t.Errorf("actionToOutbound(%q) = %q, want %q", tt.action, got, tt.expected)
		}
	}
}
