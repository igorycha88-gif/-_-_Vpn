package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

	peer, err := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Test Peer", DeviceType: models.DeviceTypeIPhone})
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
	if peer.DeviceType != models.DeviceTypeIPhone {
		t.Errorf("DeviceType = %q, want %q", peer.DeviceType, models.DeviceTypeIPhone)
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

	svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1", DeviceType: models.DeviceTypeIPhone})
	peers, _ = svc.ListPeers(context.Background())
	if len(peers) != 1 {
		t.Errorf("count = %d, want 1", len(peers))
	}
}

func TestWireGuardService_DeletePeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	svc := NewWireGuardService(repository.NewPeerRepository(db), testVLESSConfig(), testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1", DeviceType: models.DeviceTypeIPhone})
	if err := svc.DeletePeer(context.Background(), peer.ID); err != nil {
		t.Fatalf("DeletePeer: %v", err)
	}
}

func TestWireGuardService_GenerateClientConfig(t *testing.T) {
	svc := NewWireGuardService(nil, testVLESSConfig(), testLogger())

	peer := &models.Peer{
		PublicKey:  "7f2105d9-3962-4dd3-80d5-6ac86d271855",
		DeviceType: models.DeviceTypeIPhone,
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
	if !contains(config, "yandex.net") {
		t.Error("config should contain yandex.net in direct rules")
	}
	if contains(config, "package_name") {
		t.Error("iPhone config should NOT contain package_name rules")
	}
	if !contains(config, `"stack": "mixed"`) {
		t.Error("iPhone config should use stack mixed")
	}
	if !contains(config, "youtube.com") {
		t.Error("config should contain youtube.com in proxy rules")
	}
	if !contains(config, "telegram.org") {
		t.Error("config should contain telegram.org in proxy rules")
	}
	if !contains(config, "vk.com") {
		t.Error("config should contain vk.com in direct rules")
	}
	if !contains(config, ".ru") {
		t.Error("config should contain .ru domain suffix in direct rules")
	}
}

func TestWireGuardService_GenerateClientConfig_Android(t *testing.T) {
	svc := NewWireGuardService(nil, testVLESSConfig(), testLogger())

	peer := &models.Peer{
		PublicKey:  "test-uuid-android",
		DeviceType: models.DeviceTypeAndroid,
	}
	config := svc.GenerateClientConfig(peer)
	if config == "" {
		t.Fatal("config is empty")
	}
	if !contains(config, `"stack": "gvisor"`) {
		t.Error("Android config should use stack gvisor")
	}
	if !contains(config, "youtube.com") {
		t.Error("Android config should contain youtube.com in proxy rules")
	}
	if !contains(config, "vk.com") {
		t.Error("Android config should contain vk.com in direct rules")
	}
	if !contains(config, "yandex.net") {
		t.Error("Android config should contain yandex.net in direct rules")
	}
	if !contains(config, `"package_name"`) {
		t.Error("Android config should contain package_name rules")
	}
	if !contains(config, "com.google.android.projection.gearhead") {
		t.Error("Android config should contain Android Auto package name")
	}
	if !contains(config, "ru.yandex.weather") {
		t.Error("Android config should contain Yandex Weather package name")
	}
}

func TestWireGuardService_GenerateClientConfig_DefaultFallback(t *testing.T) {
	svc := NewWireGuardService(nil, testVLESSConfig(), testLogger())

	peer := &models.Peer{
		PublicKey:  "test-uuid-empty",
		DeviceType: "",
	}
	config := svc.GenerateClientConfig(peer)
	if !contains(config, `"stack": "mixed"`) {
		t.Error("Empty device_type should fallback to iPhone (stack mixed)")
	}
}

func TestPeerCreateRequest_Validate_DeviceType(t *testing.T) {
	tests := []struct {
		name       string
		req        models.PeerCreateRequest
		wantErr    bool
		errField   string
	}{
		{"valid iphone", models.PeerCreateRequest{Name: "Test", DeviceType: "iphone"}, false, ""},
		{"valid android", models.PeerCreateRequest{Name: "Test", DeviceType: "android"}, false, ""},
		{"empty device_type", models.PeerCreateRequest{Name: "Test", DeviceType: ""}, true, "device_type"},
		{"invalid device_type", models.PeerCreateRequest{Name: "Test", DeviceType: "windows"}, true, "device_type"},
		{"empty name", models.PeerCreateRequest{Name: "", DeviceType: "iphone"}, true, "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.req.Validate()
			if tt.wantErr {
				if len(errs) == 0 {
					t.Error("expected validation error")
				}
				if tt.errField != "" {
					if _, ok := errs[tt.errField]; !ok {
						t.Errorf("expected error for field %q, got errors: %v", tt.errField, errs)
					}
				}
			} else {
				if len(errs) > 0 {
					t.Errorf("unexpected errors: %v", errs)
				}
			}
		})
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
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json", ClashAPIAddr: "127.0.0.1:9090"}
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
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json", ClashAPIAddr: "127.0.0.1:9090"}
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

func TestSingBoxService_GenerateConfig_ExperimentalClashAPI(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	cfg, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}

	if cfg.Experimental == nil {
		t.Fatal("Experimental section is nil")
	}
	if cfg.Experimental.ClashAPI == nil {
		t.Fatal("ClashAPI section is nil")
	}
	if cfg.Experimental.ClashAPI.ExternalController != "127.0.0.1:9090" {
		t.Errorf("ExternalController = %q, want 127.0.0.1:9090", cfg.Experimental.ClashAPI.ExternalController)
	}
}

func TestSingBoxStatsCollector_ComputeDeltas_NewConnection(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: make(map[string]*connBytes),
	}

	connections := []clashConnection{
		{ID: "conn1", Upload: 100, Download: 500, Metadata: clashMetadata{User: "user-uuid-1"}},
	}

	deltas := collector.computeDeltas(connections)

	d, ok := deltas["user-uuid-1"]
	if !ok {
		t.Fatal("expected delta for user-uuid-1")
	}
	if d.tx != 100 {
		t.Errorf("tx = %d, want 100", d.tx)
	}
	if d.rx != 500 {
		t.Errorf("rx = %d, want 500", d.rx)
	}
}

func TestSingBoxStatsCollector_ComputeDeltas_ExistingConnection(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: map[string]*connBytes{
			"conn1": {upload: 100, download: 500},
		},
	}

	connections := []clashConnection{
		{ID: "conn1", Upload: 250, Download: 1200, Metadata: clashMetadata{User: "user-uuid-1"}},
	}

	deltas := collector.computeDeltas(connections)

	d, ok := deltas["user-uuid-1"]
	if !ok {
		t.Fatal("expected delta for user-uuid-1")
	}
	if d.tx != 150 {
		t.Errorf("tx = %d, want 150", d.tx)
	}
	if d.rx != 700 {
		t.Errorf("rx = %d, want 700", d.rx)
	}
}

func TestSingBoxStatsCollector_ComputeDeltas_MultipleUsers(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: make(map[string]*connBytes),
	}

	connections := []clashConnection{
		{ID: "conn1", Upload: 100, Download: 200, Metadata: clashMetadata{User: "uuid-1"}},
		{ID: "conn2", Upload: 300, Download: 400, Metadata: clashMetadata{User: "uuid-2"}},
		{ID: "conn3", Upload: 50, Download: 60, Metadata: clashMetadata{User: "uuid-1"}},
	}

	deltas := collector.computeDeltas(connections)

	d1, ok := deltas["uuid-1"]
	if !ok {
		t.Fatal("expected delta for uuid-1")
	}
	if d1.tx != 150 {
		t.Errorf("uuid-1 tx = %d, want 150", d1.tx)
	}
	if d1.rx != 260 {
		t.Errorf("uuid-1 rx = %d, want 260", d1.rx)
	}

	d2, ok := deltas["uuid-2"]
	if !ok {
		t.Fatal("expected delta for uuid-2")
	}
	if d2.tx != 300 {
		t.Errorf("uuid-2 tx = %d, want 300", d2.tx)
	}
	if d2.rx != 400 {
		t.Errorf("uuid-2 rx = %d, want 400", d2.rx)
	}
}

func TestSingBoxStatsCollector_ComputeDeltas_NoUser(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: make(map[string]*connBytes),
	}

	connections := []clashConnection{
		{ID: "conn1", Upload: 100, Download: 200, Metadata: clashMetadata{User: ""}},
	}

	deltas := collector.computeDeltas(connections)

	if len(deltas) != 0 {
		t.Errorf("expected 0 deltas, got %d", len(deltas))
	}
}

func TestSingBoxStatsCollector_ComputeDeltas_ZeroDelta(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: map[string]*connBytes{
			"conn1": {upload: 100, download: 200},
		},
	}

	connections := []clashConnection{
		{ID: "conn1", Upload: 100, Download: 200, Metadata: clashMetadata{User: "uuid-1"}},
	}

	deltas := collector.computeDeltas(connections)

	if len(deltas) != 0 {
		t.Errorf("expected 0 deltas for zero change, got %d", len(deltas))
	}
}

func TestSingBoxStatsCollector_CleanupStaleConnections(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: map[string]*connBytes{
			"conn1": {upload: 100, download: 200},
			"conn2": {upload: 300, download: 400},
		},
	}

	connections := []clashConnection{
		{ID: "conn1"},
	}

	collector.cleanupStaleConnections(connections)

	if _, exists := collector.connState["conn1"]; !exists {
		t.Error("conn1 should still exist")
	}
	if _, exists := collector.connState["conn2"]; exists {
		t.Error("conn2 should have been cleaned up")
	}
}

func TestSingBoxStatsCollector_FetchConnections(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			t.Errorf("path = %q, want /connections", r.URL.Path)
		}
		resp := clashConnectionsResponse{
			Connections: []clashConnection{
				{ID: "c1", Upload: 100, Download: 200, Metadata: clashMetadata{User: "uuid-1"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(nil, nil, addr, "", testLogger())

	resp, err := collector.fetchConnections()
	if err != nil {
		t.Fatalf("fetchConnections: %v", err)
	}
	if len(resp.Connections) != 1 {
		t.Fatalf("connections = %d, want 1", len(resp.Connections))
	}
	if resp.Connections[0].Metadata.User != "uuid-1" {
		t.Errorf("user = %q, want uuid-1", resp.Connections[0].Metadata.User)
	}
}

func TestSingBoxStatsCollector_FetchConnections_WithSecret(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(clashConnectionsResponse{})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(nil, nil, addr, "my-secret", testLogger())

	_, err := collector.fetchConnections()
	if err != nil {
		t.Fatalf("fetchConnections with secret: %v", err)
	}
}

func TestSingBoxStatsCollector_FetchConnections_ServerDown(t *testing.T) {
	collector := NewSingBoxStatsCollector(nil, nil, "127.0.0.1:1", "", testLogger())

	_, err := collector.fetchConnections()
	if err == nil {
		t.Error("expected error when server is down")
	}
}

func TestSingBoxStatsCollector_Collect_Integration(t *testing.T) {
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)

	err = peerRepo.Create(context.Background(), &models.Peer{
		ID: "peer-1", Name: "Test", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "test-uuid-1", PrivateKey: "pk",
		Address: "test-uuid-1", DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := clashConnectionsResponse{
			Connections: []clashConnection{
				{ID: "c1", Upload: 1000, Download: 5000, Metadata: clashMetadata{User: "test-uuid-1"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, addr, "", testLogger())
	collector.connState = make(map[string]*connBytes)

	collector.collect(context.Background())

	updated, err := peerRepo.GetByID(context.Background(), "peer-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.TotalRx != 5000 {
		t.Errorf("TotalRx = %d, want 5000", updated.TotalRx)
	}
	if updated.TotalTx != 1000 {
		t.Errorf("TotalTx = %d, want 1000", updated.TotalTx)
	}
	if updated.LastSeen == nil {
		t.Error("LastSeen should be set")
	}
}

func TestWireGuardService_GetPeerStats_OnlineByLastSeen(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer db.Close()

	peerRepo := repository.NewPeerRepository(db)
	svc := NewWireGuardService(peerRepo, testVLESSConfig(), testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1", DeviceType: models.DeviceTypeIPhone})

	stats, err := svc.GetPeerStats(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("GetPeerStats: %v", err)
	}
	if stats.Online {
		t.Error("new peer without last_seen should not be online")
	}
}
