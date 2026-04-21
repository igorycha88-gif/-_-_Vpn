package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
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

	cfg := &config.WGConfig{
		ClientSubnet: "10.10.0.0", DNS: "1.1.1.1", MTU: 1280,
		ServerEndpoint: "1.2.3.4", ServerPubKey: "testpubkey", Port: 51820,
	}
	svc := NewWireGuardService(repository.NewPeerRepository(db), cfg, testLogger())

	peer, err := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Test Peer"})
	if err != nil {
		t.Fatalf("CreatePeer: %v", err)
	}
	if peer.ID == "" {
		t.Error("ID is empty")
	}
	if peer.PublicKey == "" {
		t.Error("PublicKey is empty")
	}
	if peer.Address == "" {
		t.Error("Address is empty")
	}
	if !peer.IsActive {
		t.Error("should be active")
	}
}

func TestWireGuardService_CreatePeer_ValidationError(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	cfg := &config.WGConfig{ClientSubnet: "10.10.0.0"}
	svc := NewWireGuardService(repository.NewPeerRepository(db), cfg, testLogger())

	_, err := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWireGuardService_ListPeers(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	cfg := &config.WGConfig{ClientSubnet: "10.10.0.0", DNS: "1.1.1.1", MTU: 1280}
	svc := NewWireGuardService(repository.NewPeerRepository(db), cfg, testLogger())

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

	cfg := &config.WGConfig{ClientSubnet: "10.10.0.0", DNS: "1.1.1.1", MTU: 1280}
	svc := NewWireGuardService(repository.NewPeerRepository(db), cfg, testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1"})
	if err := svc.DeletePeer(context.Background(), peer.ID); err != nil {
		t.Fatalf("DeletePeer: %v", err)
	}
}

func TestWireGuardService_GenerateClientConfig(t *testing.T) {
	cfg := &config.WGConfig{ServerEndpoint: "1.2.3.4", ServerPubKey: "serverpubkey", Port: 51820}
	svc := NewWireGuardService(nil, cfg, testLogger())

	config := svc.GenerateClientConfig(&models.Peer{
		PrivateKey: "clientprivkey", Address: "10.10.0.2", DNS: "1.1.1.1", MTU: 1280,
	})
	if config == "" {
		t.Fatal("config is empty")
	}
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
	cfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json"}
	wgCfg := &config.WGConfig{MTU: 1280, TunnelLocalAddress: "10.20.0.2/30"}
	srvCfg := &config.ServerConfig{ForeignIP: "1.2.3.4"}
	svc := NewSingBoxService(routeRepo, dnsRepo, cfg, wgCfg, srvCfg, testLogger())
	return svc, db
}

func TestSingBoxService_GenerateConfig_WithForeignIP(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	cfg, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}

	hasDirect := false
	hasBlock := false
	hasForeign := false
	for _, o := range cfg.Outbounds {
		switch o.Tag {
		case "direct-out":
			hasDirect = true
		case "block":
			hasBlock = true
		case "foreign-out":
			hasForeign = true
			if o.Server != "1.2.3.4" {
				t.Errorf("foreign Server = %q, want 1.2.3.4", o.Server)
			}
		}
	}
	if !hasDirect {
		t.Error("missing direct-out outbound")
	}
	if !hasBlock {
		t.Error("missing block outbound")
	}
	if !hasForeign {
		t.Error("missing foreign-out outbound")
	}
	if cfg.Route.Final != "foreign-out" {
		t.Errorf("Final = %q, want foreign-out", cfg.Route.Final)
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
	cfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json"}
	wgCfg := &config.WGConfig{MTU: 1280}
	srvCfg := &config.ServerConfig{ForeignIP: ""}
	svc := NewSingBoxService(routeRepo, dnsRepo, cfg, wgCfg, srvCfg, testLogger())

	result, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	if result.Route.Final != "direct-out" {
		t.Errorf("Final = %q, want direct-out", result.Route.Final)
	}
	for _, o := range result.Outbounds {
		if o.Tag == "foreign-out" {
			t.Error("foreign-out should not exist without ForeignIP")
		}
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
	if len(cfg.DNS.Rules) == 0 {
		t.Error("DNS rules empty")
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
		{"block", "block"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := svc.actionToOutbound(tt.action)
		if got != tt.expected {
			t.Errorf("actionToOutbound(%q) = %q, want %q", tt.action, got, tt.expected)
		}
	}
}
