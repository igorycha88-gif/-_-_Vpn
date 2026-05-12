package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"smarttraffic/internal/apperrors"
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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	_, err := svc.Login(context.Background(), "admin@smarttraffic.local", "wrongpassword")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_Login_WrongEmail(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	_, err := svc.Login(context.Background(), "no@no.com", "admin123")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthService_ValidateAccessToken(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())

	_, err := svc.ValidateAccessToken("invalid.token.here")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestAuthService_GetSession(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

	svc := NewAuthService(repository.NewAuthRepository(db), testJWTConfig(), testLogger())
	tokens, _ := svc.Login(context.Background(), "admin@smarttraffic.local", "admin123")

	err := svc.Logout(context.Background(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}
}

func TestWireGuardService_CreatePeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewWireGuardService(repository.NewPeerRepository(db), repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

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
	defer func() { _ = db.Close() }()

	svc := NewWireGuardService(repository.NewPeerRepository(db), repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

	_, err := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestWireGuardService_ListPeers(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewWireGuardService(repository.NewPeerRepository(db), repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

	peers, err := svc.ListPeers(context.Background())
	if err != nil {
		t.Fatalf("ListPeers: %v", err)
	}
	initialCount := len(peers)

	_, _ = svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1", DeviceType: models.DeviceTypeIPhone})
	peers, _ = svc.ListPeers(context.Background())
	if len(peers) != initialCount+1 {
		t.Errorf("count = %d, want %d", len(peers), initialCount+1)
	}
}

func TestWireGuardService_DeletePeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	trafficRepo := repository.NewTrafficRepository(db)
	svc := NewWireGuardService(repository.NewPeerRepository(db), trafficRepo, testVLESSConfig(), testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1", DeviceType: models.DeviceTypeIPhone})

	_, err := trafficRepo.CreateSession(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := svc.DeletePeer(context.Background(), peer.ID); err != nil {
		t.Fatalf("DeletePeer: %v", err)
	}

	_, err = svc.GetPeer(context.Background(), peer.ID)
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}

	session, err := trafficRepo.GetActiveSession(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if session != nil {
		t.Fatal("expected nil session after peer delete")
	}
}

func TestWireGuardService_GenerateClientConfig(t *testing.T) {
	svc := NewWireGuardService(nil, nil, testVLESSConfig(), testLogger())

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
	if !contains(config, "avito.st") {
		t.Error("config should contain avito.st in direct rules")
	}
	if !contains(config, "avito.com") {
		t.Error("config should contain avito.com in direct rules")
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
	if !contains(config, `"detour": "proxy"`) {
		t.Error("iPhone config should have DNS foreign with proxy detour")
	}
	if !contains(config, `"detour": "direct-out"`) {
		t.Error("iPhone config should have DNS RU with direct-out detour")
	}
	if !contains(config, "dns-foreign") {
		t.Error("iPhone config should have dns-foreign server")
	}
	if !contains(config, "dns-ru") {
		t.Error("iPhone config should have dns-ru server")
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
	if !contains(config, "sberbank.ru") {
		t.Error("config should contain sberbank.ru in direct rules")
	}
	if !contains(config, ".ru") {
		t.Error("config should contain .ru domain suffix in direct rules")
	}
}

func TestWireGuardService_GenerateClientConfig_Android(t *testing.T) {
	svc := NewWireGuardService(nil, nil, testVLESSConfig(), testLogger())

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
	if !contains(config, "ru.sberbankmobile") {
		t.Error("Android config should contain Sberbank package name")
	}
	if !contains(config, `"exclude_package"`) {
		t.Error("Android config should contain exclude_package in tun inbound")
	}
	if !contains(config, "com.google.android.gms") {
		t.Error("Android config should exclude Google Play Services for Android Auto")
	}
	if !contains(config, "com.google.android.apps.auto") {
		t.Error("Android config should exclude Android Auto companion app")
	}
}

func TestWireGuardService_GenerateClientConfig_iPhone_NoExcludePackage(t *testing.T) {
	svc := NewWireGuardService(nil, nil, testVLESSConfig(), testLogger())

	peer := &models.Peer{
		PublicKey:  "test-uuid-iphone",
		DeviceType: models.DeviceTypeIPhone,
	}
	config := svc.GenerateClientConfig(peer)
	if contains(config, `"exclude_package"`) {
		t.Error("iPhone config should NOT contain exclude_package")
	}
}

func TestWireGuardService_GenerateClientConfig_FinalIsDirectOut(t *testing.T) {
	svc := NewWireGuardService(nil, nil, testVLESSConfig(), testLogger())

	peer := &models.Peer{
		PublicKey:  "test-uuid-final",
		DeviceType: models.DeviceTypeIPhone,
	}
	config := svc.GenerateClientConfig(peer)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		t.Fatalf("config should be valid JSON: %v", err)
	}

	route, ok := parsed["route"].(map[string]any)
	if !ok {
		t.Fatal("config should have route object")
	}
	final, ok := route["final"].(string)
	if !ok {
		t.Fatal("route should have final string")
	}
	if final != "direct-out" {
		t.Errorf("expected final to be 'direct-out', got '%s'", final)
	}
}

func TestWireGuardService_GenerateClientConfig_DefaultFallback(t *testing.T) {
	svc := NewWireGuardService(nil, nil, testVLESSConfig(), testLogger())

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
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
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
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
		testLogger(),
	)

	_, err := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRoutingService_UpdateRule(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
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
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(
		repository.NewRouteRepository(db),
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
	defer func() { _ = db.Close() }()

	presetSvc := NewPresetService(
		repository.NewPresetRepository(db),
		repository.NewRouteRepository(db),
		testLogger(),
	)

	result, err := presetSvc.ApplyPreset(context.Background(), "preset-all-direct")
	if err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if result.AppliedRules < 1 {
		t.Errorf("AppliedRules = %d, want >= 1", result.AppliedRules)
	}
}

func TestDNSService_GetSettings(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

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
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	logs, err := svc.GetTrafficLogs(context.Background(), models.TrafficFilter{Limit: 50})
	if err != nil {
		t.Fatalf("GetTrafficLogs: %v", err)
	}
	if logs == nil {
		t.Error("expected non-nil logs")
	}
}

func TestTrafficService_Alerts(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

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
	t.Cleanup(func() { _ = db.Close() })

	routeRepo := repository.NewRouteRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	peerRepo := repository.NewPeerRepository(db)
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json", ClashAPIAddr: "127.0.0.1:9090"}
	vlessCfg := testVLESSConfig()
	wgCfg := &config.WGConfig{TunnelLocalAddress: "10.20.0.2/30", TunnelPrivateKey: "testkey", TunnelPeerPublicKey: "testpeerkey"}
	srvCfg := &config.ServerConfig{ForeignIP: "1.2.3.4", ForeignVLESS: config.ForeignVLESSConfig{UUID: "test-uuid", ServerName: "www.microsoft.com", RealityPublicKey: "test-pk", RealityShortID: "test-sid"}}
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
	defer func() { _ = db.Close() }()

	routeRepo := repository.NewRouteRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	peerRepo := repository.NewPeerRepository(db)
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json", ClashAPIAddr: "127.0.0.1:9090"}
	vlessCfg := testVLESSConfig()
	wgCfg := &config.WGConfig{}
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

func TestSingBoxService_GenerateConfig_OnlyActivePeersInUsers(t *testing.T) {
	svc, db := newTestSingBoxService(t)
	peerRepo := repository.NewPeerRepository(db)

	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "p1", Name: "Active", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-active-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "p2", Name: "Inactive", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-inactive-2", PrivateKey: "pk", Address: "addr2",
		DNS: "1.1.1.1", MTU: 1280, IsActive: false,
	})

	cfg, err := svc.GenerateConfig(context.Background())
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}

	data, _ := json.Marshal(cfg.Inbounds[0])
	cfgStr := string(data)

	if !contains(cfgStr, "uuid-active-1") {
		t.Error("active peer UUID should be in inbound users")
	}
	if contains(cfgStr, "uuid-inactive-2") {
		t.Error("inactive peer UUID should NOT be in inbound users")
	}
}

func TestWireGuardService_TogglePeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewWireGuardService(repository.NewPeerRepository(db), repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{
		Name: "Toggle", DeviceType: models.DeviceTypeIPhone,
	})

	if err := svc.TogglePeer(context.Background(), peer.ID, false); err != nil {
		t.Fatalf("TogglePeer off: %v", err)
	}

	updated, _ := svc.GetPeer(context.Background(), peer.ID)
	if updated.IsActive {
		t.Error("peer should be inactive after toggle off")
	}

	if err := svc.TogglePeer(context.Background(), peer.ID, true); err != nil {
		t.Fatalf("TogglePeer on: %v", err)
	}

	updated, _ = svc.GetPeer(context.Background(), peer.ID)
	if !updated.IsActive {
		t.Error("peer should be active after toggle on")
	}
}

func TestWireGuardService_TogglePeer_NotFound(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewWireGuardService(repository.NewPeerRepository(db), repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

	err := svc.TogglePeer(context.Background(), "nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent peer")
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

	d, ok := deltas["uuid-1"]
	if !ok {
		t.Fatal("expected delta entry for uuid-1 even with zero change")
	}
	if d.rx != 0 {
		t.Errorf("rx = %d, want 0", d.rx)
	}
	if d.tx != 0 {
		t.Errorf("tx = %d, want 0", d.tx)
	}
	if len(d.connections) != 0 {
		t.Errorf("expected 0 connections for zero delta, got %d", len(d.connections))
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
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(nil, nil, nil, addr, "", testLogger())

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
		_ = json.NewEncoder(w).Encode(clashConnectionsResponse{})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(nil, nil, nil, addr, "my-secret", testLogger())

	_, err := collector.fetchConnections()
	if err != nil {
		t.Fatalf("fetchConnections with secret: %v", err)
	}
}

func TestSingBoxStatsCollector_FetchConnections_ServerDown(t *testing.T) {
	collector := NewSingBoxStatsCollector(nil, nil, nil, "127.0.0.1:1", "", testLogger())

	_, err := collector.fetchConnections()
	if err == nil {
		t.Error("expected error when server is down")
	}
}

func TestSingBoxStatsCollector_AggregateUsesSourceIP(t *testing.T) {
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)

	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "peer-1", Name: "Peer 1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "peer-2", Name: "Peer 2", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-2", PrivateKey: "pk", Address: "addr2",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			resp := clashConnectionsResponse{
				Connections: []clashConnection{
					{ID: "c1", Upload: 1000, Download: 5000, Metadata: clashMetadata{User: "uuid-1", SourceIP: "10.0.0.1", Type: "vless"}},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			resp := clashConnectionsResponse{
				Connections: []clashConnection{
					{ID: "c2", Upload: 2000, Download: 8000, Metadata: clashMetadata{User: "", SourceIP: "10.0.0.1", Type: "vless"}},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, addr, "", testLogger())
	collector.connState = make(map[string]*connBytes)

	collector.collect(context.Background())

	p1, _ := peerRepo.GetByID(context.Background(), "peer-1")
	p2, _ := peerRepo.GetByID(context.Background(), "peer-2")

	if p1.TotalRx != 5000 {
		t.Errorf("peer-1 TotalRx after first collect = %d, want 5000", p1.TotalRx)
	}
	if p2.TotalRx != 0 {
		t.Errorf("peer-2 TotalRx after first collect = %d, want 0", p2.TotalRx)
	}

	collector.collect(context.Background())

	p1, _ = peerRepo.GetByID(context.Background(), "peer-1")
	p2, _ = peerRepo.GetByID(context.Background(), "peer-2")

	if p2.TotalRx != 0 {
		t.Errorf("peer-2 TotalRx should be 0, got %d — aggregate was incorrectly distributed", p2.TotalRx)
	}
	if p1.TotalRx != 13000 {
		t.Errorf("peer-1 TotalRx = %d, want 13000 (5000 first + 8000 aggregate)", p1.TotalRx)
	}
}

func TestSingBoxStatsCollector_AggregateNoDistributionWithoutMapping(t *testing.T) {
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)

	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "peer-a", Name: "Peer A", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-a", PrivateKey: "pk", Address: "addr-a",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "peer-b", Name: "Peer B", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-b", PrivateKey: "pk", Address: "addr-b",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := clashConnectionsResponse{
			Connections: []clashConnection{
				{ID: "c1", Upload: 1000, Download: 5000, Metadata: clashMetadata{User: "", SourceIP: "10.0.0.99", Type: "vless"}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, addr, "", testLogger())
	collector.connState = make(map[string]*connBytes)

	collector.collect(context.Background())

	pA, _ := peerRepo.GetByID(context.Background(), "peer-a")
	pB, _ := peerRepo.GetByID(context.Background(), "peer-b")

	if pA.TotalRx != 0 {
		t.Errorf("peer-a TotalRx should be 0 when no source IP mapping exists, got %d", pA.TotalRx)
	}
	if pB.TotalRx != 0 {
		t.Errorf("peer-b TotalRx should be 0 when no source IP mapping exists, got %d", pB.TotalRx)
	}
}

func TestSingBoxStatsCollector_Collect_Integration(t *testing.T) {
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

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
		_ = json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, addr, "", testLogger())
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
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	svc := NewWireGuardService(peerRepo, repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

	peer, _ := svc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "P1", DeviceType: models.DeviceTypeIPhone})

	stats, err := svc.GetPeerStats(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("GetPeerStats: %v", err)
	}
	if stats.Online {
		t.Error("new peer without last_seen should not be online")
	}
}

func TestTrafficService_LogTraffic(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "log-peer-1", Name: "LogTest", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "log-uuid-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		peerRepo,
		testLogger(),
	)

	err := svc.LogTraffic(context.Background(), &models.TrafficLog{
		PeerID:  "log-peer-1",
		Domain:  "example.com",
		Action:  "test_action",
		BytesRx: 1024,
		BytesTx: 512,
	})
	if err != nil {
		t.Fatalf("LogTraffic: %v", err)
	}

	logs, err := svc.GetTrafficLogs(context.Background(), models.TrafficFilter{PeerID: "log-peer-1", Limit: 50})
	if err != nil {
		t.Fatalf("GetTrafficLogs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].PeerID != "log-peer-1" {
		t.Errorf("PeerID = %q, want log-peer-1", logs[0].PeerID)
	}
	if logs[0].Domain != "example.com" {
		t.Errorf("Domain = %q, want example.com", logs[0].Domain)
	}
	if logs[0].BytesRx != 1024 {
		t.Errorf("BytesRx = %d, want 1024", logs[0].BytesRx)
	}
	if logs[0].BytesTx != 512 {
		t.Errorf("BytesTx = %d, want 512", logs[0].BytesTx)
	}
}

func TestTrafficService_CleanupOldLogs(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(`INSERT INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
		VALUES ('demo-peer-001', '', '', 0, 'test', 100, 200, datetime('now', '-60 days'))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
		VALUES ('demo-peer-001', '', '', 0, 'test', 300, 400, datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	deleted, err := svc.CleanupOldLogs(context.Background(), 30)
	if err != nil {
		t.Fatalf("CleanupOldLogs: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	logs, _ := svc.GetTrafficLogs(context.Background(), models.TrafficFilter{PeerID: "demo-peer-001", Limit: 50})
	for _, l := range logs {
		if l.Action == "test" && l.BytesRx == 100 {
			t.Error("old log with bytes_rx=100 should have been deleted")
		}
	}
}

func TestTrafficService_CleanupOldLogs_DefaultRetain(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	_, err := db.Exec(`INSERT INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
		VALUES ('demo-peer-001', '', '', 0, 'test', 1, 1, datetime('now', '-31 days'))`)
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := svc.CleanupOldLogs(context.Background(), 0)
	if err != nil {
		t.Fatalf("CleanupOldLogs: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1 with default retain_days=30", deleted)
	}
}

func TestTrafficService_AddAlert_DeleteAlert(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	alert := &models.Alert{
		ID:        "alert-test-1",
		Type:      "system",
		Message:   "Test alert message",
		Severity:  "warning",
		Timestamp: time.Now(),
	}

	svc.AddAlert(context.Background(), alert)

	alerts, err := svc.GetAlerts(context.Background())
	if err != nil {
		t.Fatalf("GetAlerts: %v", err)
	}
	found := false
	for _, a := range alerts {
		if a.ID == "alert-test-1" {
			found = true
			if a.Message != "Test alert message" {
				t.Errorf("Message = %q, want Test alert message", a.Message)
			}
			break
		}
	}
	if !found {
		t.Fatal("alert-test-1 should be present after AddAlert")
	}

	err = svc.DeleteAlert(context.Background(), "alert-test-1")
	if err != nil {
		t.Fatalf("DeleteAlert: %v", err)
	}

	alerts, _ = svc.GetAlerts(context.Background())
	for _, a := range alerts {
		if a.ID == "alert-test-1" {
			t.Fatal("alert-test-1 should be deleted")
		}
	}
}

func TestTrafficService_ClearAllAlerts(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	_ = svc.ClearAllAlerts(context.Background())

	svc.AddAlert(context.Background(), &models.Alert{ID: "c1", Type: "system", Message: "A1", Severity: "info", Timestamp: time.Now()})
	svc.AddAlert(context.Background(), &models.Alert{ID: "c2", Type: "system", Message: "A2", Severity: "warning", Timestamp: time.Now()})
	svc.AddAlert(context.Background(), &models.Alert{ID: "c3", Type: "system", Message: "A3", Severity: "error", Timestamp: time.Now()})

	alerts, _ := svc.GetAlerts(context.Background())
	if len(alerts) != 3 {
		t.Fatalf("expected 3 alerts, got %d", len(alerts))
	}

	err := svc.ClearAllAlerts(context.Background())
	if err != nil {
		t.Fatalf("ClearAllAlerts: %v", err)
	}

	alerts, _ = svc.GetAlerts(context.Background())
	if len(alerts) != 0 {
		t.Errorf("expected 0 alerts after clear, got %d", len(alerts))
	}
}

func TestTrafficService_CleanupOldAlerts(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	trafficRepo := repository.NewTrafficRepository(db)
	svc := NewTrafficService(trafficRepo, repository.NewPeerRepository(db), testLogger())

	_ = svc.ClearAllAlerts(context.Background())

	_, err := db.Exec(`INSERT INTO alerts (id, type, message, severity, timestamp) VALUES ('old-a', 'system', 'Old', 'info', datetime('now', '-60 days'))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO alerts (id, type, message, severity, timestamp) VALUES ('new-a', 'system', 'New', 'info', datetime('now'))`)
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := svc.CleanupOldAlerts(context.Background(), 30)
	if err != nil {
		t.Fatalf("CleanupOldAlerts: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	alerts, _ := svc.GetAlerts(context.Background())
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].ID != "new-a" {
		t.Errorf("remaining alert ID = %q, want new-a", alerts[0].ID)
	}
}

func TestTrafficService_GetAllPeerStats(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	summaries, err := svc.GetAllPeerStats(context.Background())
	if err != nil {
		t.Fatalf("GetAllPeerStats: %v", err)
	}
	if summaries == nil {
		t.Fatal("summaries should not be nil")
	}
}

func TestTrafficService_GetAllPeerStats_WithPeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	svc := NewTrafficService(trafficRepo, peerRepo, testLogger())

	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "sp1", Name: "StatsPeer", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-sp1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})
	_ = trafficRepo.Log(context.Background(), &models.TrafficLog{
		PeerID: "sp1", Domain: "test.com", Action: "test", BytesRx: 500, BytesTx: 300,
	})
	_ = peerRepo.UpdateTraffic(context.Background(), "sp1", 500, 300)

	summaries, err := svc.GetAllPeerStats(context.Background())
	if err != nil {
		t.Fatalf("GetAllPeerStats: %v", err)
	}

	var found *models.PeerTrafficSummary
	for _, s := range summaries {
		if s.PeerID == "sp1" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find peer sp1 in summaries")
	}
	if found.TotalRx != 500 {
		t.Errorf("TotalRx = %d, want 500", found.TotalRx)
	}
}

func TestTrafficService_GetTrafficAggregate(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "agg-p1", Name: "AggPeer1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-agg-1", PrivateKey: "pk", Address: "addr-a1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		peerRepo,
		testLogger(),
	)

	_ = svc.LogTraffic(context.Background(), &models.TrafficLog{PeerID: "agg-p1", Domain: "example.com", Action: "test", BytesRx: 1000, BytesTx: 500})
	_ = svc.LogTraffic(context.Background(), &models.TrafficLog{PeerID: "agg-p1", Domain: "example.com", Action: "test", BytesRx: 200, BytesTx: 100})
	_ = svc.LogTraffic(context.Background(), &models.TrafficLog{PeerID: "agg-p1", Domain: "other.com", Action: "test", BytesRx: 300, BytesTx: 150})

	items, err := svc.GetTrafficAggregate(context.Background(), "agg-p1", 10)
	if err != nil {
		t.Fatalf("GetTrafficAggregate: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Domain != "example.com" {
		t.Errorf("first Domain = %q, want example.com", items[0].Domain)
	}
	if items[0].RX != 1200 {
		t.Errorf("first RX = %d, want 1200", items[0].RX)
	}
	if items[0].Count != 2 {
		t.Errorf("first Count = %d, want 2", items[0].Count)
	}
}

func TestTrafficService_GetTrafficAggregate_ByPeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "agg-p2", Name: "AggPeer2", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-agg-2", PrivateKey: "pk", Address: "addr-a2",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		peerRepo,
		testLogger(),
	)

	_ = svc.LogTraffic(context.Background(), &models.TrafficLog{PeerID: "agg-p2", Domain: "a.com", Action: "test", BytesRx: 100, BytesTx: 50})
	_ = svc.LogTraffic(context.Background(), &models.TrafficLog{PeerID: "agg-p2", Domain: "b.com", Action: "test", BytesRx: 200, BytesTx: 100})

	items, err := svc.GetTrafficAggregate(context.Background(), "agg-p2", 10)
	if err != nil {
		t.Fatalf("GetTrafficAggregate: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items for peer agg-p2, got %d", len(items))
	}
}

func TestTrafficService_GetTrafficAggregate_DefaultLimit(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	items, err := svc.GetTrafficAggregate(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("GetTrafficAggregate: %v", err)
	}
	if items == nil {
		t.Error("expected non-nil result")
	}
}

func TestTrafficService_GetPeerSessions(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "sess-peer-1", Name: "SessPeer", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-sess-1", PrivateKey: "pk", Address: "addr-s1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	trafficRepo := repository.NewTrafficRepository(db)
	svc := NewTrafficService(trafficRepo, peerRepo, testLogger())

	_, _ = trafficRepo.CreateSession(context.Background(), "sess-peer-1")
	_, _ = trafficRepo.CreateSession(context.Background(), "sess-peer-1")

	sessions, err := svc.GetPeerSessions(context.Background(), "sess-peer-1", 10)
	if err != nil {
		t.Fatalf("GetPeerSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
	if sessions[0].PeerID != "sess-peer-1" {
		t.Errorf("PeerID = %q, want sess-peer-1", sessions[0].PeerID)
	}
}

func TestTrafficService_GetPeerSessions_DefaultLimit(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewTrafficService(
		repository.NewTrafficRepository(db),
		repository.NewPeerRepository(db),
		testLogger(),
	)

	sessions, err := svc.GetPeerSessions(context.Background(), "nonexistent", 0)
	if err != nil {
		t.Fatalf("GetPeerSessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for nonexistent peer, got %d", len(sessions))
	}
}

func TestRoutingService_GetRule(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(repository.NewRouteRepository(db), testLogger())

	created, err := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{
		Name: "GetTest", Type: "domain", Pattern: "example.com", Action: "direct",
	})
	if err != nil {
		t.Fatalf("CreateRule: %v", err)
	}

	rule, err := svc.GetRule(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if rule.ID != created.ID {
		t.Errorf("ID = %q, want %q", rule.ID, created.ID)
	}
	if rule.Name != "GetTest" {
		t.Errorf("Name = %q, want GetTest", rule.Name)
	}
	if rule.Type != "domain" {
		t.Errorf("Type = %q, want domain", rule.Type)
	}
	if rule.Pattern != "example.com" {
		t.Errorf("Pattern = %q, want example.com", rule.Pattern)
	}
	if rule.Action != "direct" {
		t.Errorf("Action = %q, want direct", rule.Action)
	}
	if !rule.IsActive {
		t.Error("rule should be active by default")
	}
}

func TestRoutingService_GetRule_NotFound(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(repository.NewRouteRepository(db), testLogger())

	_, err := svc.GetRule(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent rule")
	}
}

func TestRoutingService_ListRules(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(repository.NewRouteRepository(db), testLogger())

	rulesBefore, _ := svc.ListRules(context.Background())
	initialCount := len(rulesBefore)

	_, _ = svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "L1", Type: "domain", Pattern: "a.com", Action: "direct"})
	_, _ = svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "L2", Type: "domain", Pattern: "b.com", Action: "proxy"})
	_, _ = svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "L3", Type: "domain", Pattern: "c.com", Action: "block"})

	rules, err := svc.ListRules(context.Background())
	if err != nil {
		t.Fatalf("ListRules: %v", err)
	}
	if len(rules) != initialCount+3 {
		t.Errorf("expected %d rules, got %d", initialCount+3, len(rules))
	}
}

func TestRoutingService_ReorderRules(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(repository.NewRouteRepository(db), testLogger())

	r1, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "R1", Type: "domain", Pattern: "a.com", Action: "direct"})
	r2, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "R2", Type: "domain", Pattern: "b.com", Action: "proxy"})
	r3, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "R3", Type: "domain", Pattern: "c.com", Action: "block"})

	err := svc.ReorderRules(context.Background(), &models.ReorderRequest{IDs: []string{r3.ID, r1.ID, r2.ID}})
	if err != nil {
		t.Fatalf("ReorderRules: %v", err)
	}

	rules, _ := svc.ListRules(context.Background())
	if len(rules) < 3 {
		t.Fatalf("expected at least 3 rules, got %d", len(rules))
	}

	newRules := make(map[string]int)
	for _, r := range rules {
		if r.ID == r1.ID || r.ID == r2.ID || r.ID == r3.ID {
			newRules[r.ID] = r.Priority
		}
	}
	if newRules[r3.ID] != 1 {
		t.Errorf("r3 priority = %d, want 1", newRules[r3.ID])
	}
	if newRules[r1.ID] != 2 {
		t.Errorf("r1 priority = %d, want 2", newRules[r1.ID])
	}
	if newRules[r2.ID] != 3 {
		t.Errorf("r2 priority = %d, want 3", newRules[r2.ID])
	}
}

func TestRoutingService_ReorderRules_Validation(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewRoutingService(repository.NewRouteRepository(db), testLogger())

	err := svc.ReorderRules(context.Background(), &models.ReorderRequest{IDs: []string{}})
	if err == nil {
		t.Fatal("expected validation error for empty IDs")
	}
}

func TestPresetService_List(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewPresetService(
		repository.NewPresetRepository(db),
		repository.NewRouteRepository(db),
		testLogger(),
	)

	presets, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(presets) == 0 {
		t.Fatal("expected at least one preset from seed data")
	}

	names := make(map[string]bool)
	for _, p := range presets {
		names[p.Name] = true
	}
	if !names["Всё напрямую"] {
		t.Error("expected 'Всё напрямую' preset")
	}
}

func TestPresetService_GetByID(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewPresetService(
		repository.NewPresetRepository(db),
		repository.NewRouteRepository(db),
		testLogger(),
	)

	preset, err := svc.GetByID(context.Background(), "preset-all-direct")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if preset.Name != "Всё напрямую" {
		t.Errorf("Name = %q, want Всё напрямую", preset.Name)
	}
	if preset.Rules == "" {
		t.Error("Rules should not be empty")
	}
	if !preset.IsBuiltin {
		t.Error("preset-all-direct should be builtin")
	}
}

func TestPresetService_GetByID_NotFound(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewPresetService(
		repository.NewPresetRepository(db),
		repository.NewRouteRepository(db),
		testLogger(),
	)

	_, err := svc.GetByID(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent preset")
	}
}

func TestDNSService_GetPresets(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewDNSService(repository.NewDNSRepository(db), testLogger())

	presets := svc.GetPresets()
	if len(presets) == 0 {
		t.Fatal("expected DNS presets")
	}

	found := false
	for _, p := range presets {
		if p.ID == "cloudflare" {
			found = true
			if !contains(p.Servers, "1.1.1.1") {
				t.Error("cloudflare preset should contain 1.1.1.1")
			}
		}
	}
	if !found {
		t.Error("cloudflare preset not found")
	}

	foundYandex := false
	for _, p := range presets {
		if p.ID == "yandex" {
			foundYandex = true
		}
	}
	if !foundYandex {
		t.Error("yandex preset not found")
	}
}

func TestSingBoxService_populateRouteRuleFields(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	t.Run("domain", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "domain", Pattern: "example.com"})
		arr, ok := rr["domain"].([]string)
		if !ok || len(arr) != 1 || arr[0] != "example.com" {
			t.Errorf("got %v", rr["domain"])
		}
	})

	t.Run("domain_suffix", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "domain_suffix", Pattern: ".com"})
		arr, ok := rr["domain_suffix"].([]string)
		if !ok || len(arr) != 1 || arr[0] != ".com" {
			t.Errorf("got %v", rr["domain_suffix"])
		}
	})

	t.Run("domain_keyword", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "domain_keyword", Pattern: "google"})
		arr, ok := rr["domain_keyword"].([]string)
		if !ok || len(arr) != 1 || arr[0] != "google" {
			t.Errorf("got %v", rr["domain_keyword"])
		}
	})

	t.Run("ip", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "ip", Pattern: "1.2.3.4/32"})
		arr, ok := rr["ip_cidr"].([]string)
		if !ok || len(arr) != 1 || arr[0] != "1.2.3.4/32" {
			t.Errorf("got %v", rr["ip_cidr"])
		}
	})

	t.Run("port", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "port", Pattern: "443"})
		arr, ok := rr["port"].([]int)
		if !ok || len(arr) != 1 || arr[0] != 443 {
			t.Errorf("got %v", rr["port"])
		}
	})

	t.Run("regex", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "regex", Pattern: ".*\\.com"})
		arr, ok := rr["domain"].([]string)
		if !ok || len(arr) != 1 || arr[0] != "regexp:.*\\.com" {
			t.Errorf("got %v", rr["domain"])
		}
	})

	t.Run("geoip_skipped", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "geoip", Pattern: "RU"})
		if len(rr) != 0 {
			t.Errorf("geoip should be skipped, got fields: %v", rr)
		}
	})

	t.Run("port_invalid", func(t *testing.T) {
		rr := map[string]any{}
		svc.populateRouteRuleFields(rr, &models.RoutingRule{Type: "port", Pattern: "notanumber"})
		if _, ok := rr["port"]; ok {
			t.Error("invalid port should not set port field")
		}
	})
}

func TestSingBoxService_buildDNSConfig(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	dns := svc.buildDNSConfig(&models.DNSSettings{
		UpstreamRU:      "77.88.8.8,77.88.8.1",
		UpstreamForeign: "1.1.1.1,8.8.8.8",
	})

	if len(dns.Servers) != 4 {
		t.Errorf("expected 4 DNS servers, got %d", len(dns.Servers))
	}
	if dns.Strategy != "prefer_ipv4" {
		t.Errorf("Strategy = %q, want prefer_ipv4", dns.Strategy)
	}
	if dns.Final == "" {
		t.Error("Final should not be empty")
	}
	if len(dns.Rules) < 2 {
		t.Errorf("expected at least 2 DNS rules, got %d", len(dns.Rules))
	}

	hasRU := false
	hasForeign := false
	for _, s := range dns.Servers {
		if contains(s.Tag, "dns-ru-") {
			hasRU = true
			if s.Type != "udp" {
				t.Errorf("server type = %q, want udp", s.Type)
			}
		}
		if contains(s.Tag, "dns-foreign-") {
			hasForeign = true
			if s.Detour != "foreign-out" {
				t.Errorf("foreign server detour = %q, want foreign-out", s.Detour)
			}
		}
	}
	if !hasRU {
		t.Error("expected RU DNS server")
	}
	if !hasForeign {
		t.Error("expected foreign DNS server")
	}
}

func TestSingBoxService_buildDNSConfig_EmptyForeign(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	routeRepo := repository.NewRouteRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	peerRepo := repository.NewPeerRepository(db)
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json", ClashAPIAddr: "127.0.0.1:9090"}
	vlessCfg := testVLESSConfig()
	wgCfg := &config.WGConfig{}
	srvCfg := &config.ServerConfig{ForeignIP: ""}
	svc := NewSingBoxService(routeRepo, dnsRepo, peerRepo, sbCfg, vlessCfg, wgCfg, srvCfg, testLogger())

	dns := svc.buildDNSConfig(&models.DNSSettings{
		UpstreamRU:      "77.88.8.8",
		UpstreamForeign: "1.1.1.1",
	})

	for _, s := range dns.Servers {
		if contains(s.Tag, "dns-foreign-") && s.Detour != "" {
			t.Errorf("foreign server should have no detour without foreign-out, got %q", s.Detour)
		}
	}
}

func TestSingBoxService_Restart(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	err := svc.Restart()
	if err != nil {
		if !contains(err.Error(), "docker") {
			t.Logf("Restart returned expected docker error: %v", err)
		}
	}
}

func TestSingBoxService_WriteConfigAndRestart(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	err := svc.WriteConfigAndRestart(context.Background())
	_ = err

	data, readErr := os.ReadFile(svc.cfg.ConfigPath)
	if readErr != nil {
		t.Fatalf("config file should exist after WriteConfigAndRestart: %v", readErr)
	}
	if len(data) == 0 {
		t.Error("config file should not be empty")
	}
}

func TestSingBoxService_WriteConfigAndReload(t *testing.T) {
	svc, _ := newTestSingBoxService(t)

	err := svc.WriteConfigAndReload(context.Background())
	_ = err

	data, readErr := os.ReadFile(svc.cfg.ConfigPath)
	if readErr != nil {
		t.Fatalf("config file should exist after WriteConfigAndReload: %v", readErr)
	}
	if len(data) == 0 {
		t.Error("config file should not be empty")
	}
}

func TestSingBoxService_splitCommaList(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"", nil},
		{"single", []string{"single"}},
		{",,,", nil},
		{" a ,, b ", []string{"a", "b"}},
	}
	for _, tt := range tests {
		got := splitCommaList(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitCommaList(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitCommaList(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestWGStatsCollector_parseTransfer(t *testing.T) {
	collector := NewWGStatsCollector(nil, nil, nil, "wg0", testLogger())

	output := "pubkey1\t1000\t2000\t1234567890\npubkey2\t3000\t4000\t1234567891\n"
	result := collector.parseTransfer(output)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result["pubkey1"].rx != 1000 {
		t.Errorf("pubkey1 rx = %d, want 1000", result["pubkey1"].rx)
	}
	if result["pubkey1"].tx != 2000 {
		t.Errorf("pubkey1 tx = %d, want 2000", result["pubkey1"].tx)
	}
	if result["pubkey2"].rx != 3000 {
		t.Errorf("pubkey2 rx = %d, want 3000", result["pubkey2"].rx)
	}
	if result["pubkey2"].tx != 4000 {
		t.Errorf("pubkey2 tx = %d, want 4000", result["pubkey2"].tx)
	}
}

func TestWGStatsCollector_parseTransfer_Empty(t *testing.T) {
	collector := NewWGStatsCollector(nil, nil, nil, "wg0", testLogger())

	result := collector.parseTransfer("")
	if len(result) != 0 {
		t.Errorf("expected 0 entries for empty input, got %d", len(result))
	}
}

func TestWGStatsCollector_parseTransfer_InvalidLine(t *testing.T) {
	collector := NewWGStatsCollector(nil, nil, nil, "wg0", testLogger())

	output := "pubkey1\t100\t200\t123\nshort\npubkey2\tabc\tdef\t123\n"
	result := collector.parseTransfer(output)

	if len(result) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(result))
	}
	if result["pubkey1"].rx != 100 {
		t.Errorf("pubkey1 rx = %d, want 100", result["pubkey1"].rx)
	}
}

func TestWGStatsCollector_IsWGActive(t *testing.T) {
	collector := NewWGStatsCollector(nil, nil, nil, "wg0", testLogger())

	if collector.IsWGActive() {
		t.Error("new collector should not be active")
	}
}

func TestWGStatsCollector_NewCollector(t *testing.T) {
	collector := NewWGStatsCollector(nil, nil, nil, "wg1", testLogger())

	if collector.iface != "wg1" {
		t.Errorf("iface = %q, want wg1", collector.iface)
	}
	if collector.interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s", collector.interval)
	}
}

func TestSingBoxStatsCollector_countConnectionsPerPeer(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "cp-1", Name: "CP1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-cp-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "cp-2", Name: "CP2", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-cp-2", PrivateKey: "pk", Address: "addr2",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	collector := NewSingBoxStatsCollector(peerRepo, nil, nil, "127.0.0.1:1", "", testLogger())

	connections := []clashConnection{
		{ID: "c1", Metadata: clashMetadata{User: "uuid-cp-1"}},
		{ID: "c2", Metadata: clashMetadata{User: "uuid-cp-1"}},
		{ID: "c3", Metadata: clashMetadata{User: "uuid-cp-2"}},
		{ID: "c4", Metadata: clashMetadata{User: ""}},
	}

	counts := collector.countConnectionsPerPeer(connections)

	if counts["cp-1"] != 2 {
		t.Errorf("cp-1 count = %d, want 2", counts["cp-1"])
	}
	if counts["cp-2"] != 1 {
		t.Errorf("cp-2 count = %d, want 1", counts["cp-2"])
	}
	if _, ok := counts[""]; ok {
		t.Error("empty user should not appear in counts")
	}
}

func TestSingBoxStatsCollector_updatePeerRealtime(t *testing.T) {
	collector := NewSingBoxStatsCollector(nil, nil, nil, "127.0.0.1:1", "", testLogger())
	collector.peerRealtime = make(map[string]*models.PeerRealtimeStats)
	collector.peerSessions = make(map[string]*peerSessionInfo)

	startTime := time.Now().Add(-5 * time.Minute)
	collector.peerSessions["peer-rt-1"] = &peerSessionInfo{
		sessionID: 42,
		startTime: startTime,
	}

	delta := &userDelta{rx: 1000, tx: 500}
	collector.updatePeerRealtime("peer-rt-1", delta, 3, 10.0, true)

	stats := collector.GetRealtimeStats()
	s, ok := stats["peer-rt-1"]
	if !ok {
		t.Fatal("expected realtime stats for peer-rt-1")
	}
	if s.ActiveConnections != 3 {
		t.Errorf("ActiveConnections = %d, want 3", s.ActiveConnections)
	}
	if s.BandwidthRx != 1000 {
		t.Errorf("BandwidthRx = %d, want 1000", s.BandwidthRx)
	}
	if s.BandwidthTx != 500 {
		t.Errorf("BandwidthTx = %d, want 500", s.BandwidthTx)
	}
	if s.BandwidthRateRx != 100.0 {
		t.Errorf("BandwidthRateRx = %f, want 100.0", s.BandwidthRateRx)
	}
	if s.BandwidthRateTx != 50.0 {
		t.Errorf("BandwidthRateTx = %f, want 50.0", s.BandwidthRateTx)
	}
	if s.SessionRx != 1000 {
		t.Errorf("SessionRx = %d, want 1000", s.SessionRx)
	}
	if s.SessionTx != 500 {
		t.Errorf("SessionTx = %d, want 500", s.SessionTx)
	}
	if s.ConnectedAt == nil {
		t.Error("ConnectedAt should be set from session")
	}

	collector.updatePeerRealtime("peer-rt-1", &userDelta{rx: 200, tx: 100}, 5, 10.0, true)
	stats = collector.GetRealtimeStats()
	s = stats["peer-rt-1"]
	if s.SessionRx != 1200 {
		t.Errorf("SessionRx after second update = %d, want 1200", s.SessionRx)
	}
	if s.SessionTx != 600 {
		t.Errorf("SessionTx after second update = %d, want 600", s.SessionTx)
	}
}

func TestSingBoxStatsCollector_startSession(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "sess-rt-1", Name: "SessRT", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-sess-rt-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, "127.0.0.1:1", "", testLogger())
	collector.peerSessions = make(map[string]*peerSessionInfo)

	collector.startSession(context.Background(), "sess-rt-1")

	collector.mu.Lock()
	sess, ok := collector.peerSessions["sess-rt-1"]
	collector.mu.Unlock()

	if !ok {
		t.Fatal("expected session for sess-rt-1")
	}
	if sess.sessionID == 0 {
		t.Error("sessionID should be non-zero")
	}
	if sess.startTime.IsZero() {
		t.Error("startTime should be set")
	}
}

func TestSingBoxStatsCollector_endSession(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "sess-end-1", Name: "SessEnd", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-sess-end-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, "127.0.0.1:1", "", testLogger())
	collector.peerSessions = make(map[string]*peerSessionInfo)
	collector.peerRealtime = make(map[string]*models.PeerRealtimeStats)

	sessionID, _ := trafficRepo.CreateSession(context.Background(), "sess-end-1")
	collector.mu.Lock()
	collector.peerSessions["sess-end-1"] = &peerSessionInfo{
		sessionID: sessionID,
		startTime: time.Now().Add(-10 * time.Minute),
		connCount: 3,
	}
	collector.peerRealtime["sess-end-1"] = &models.PeerRealtimeStats{
		SessionRx: 5000,
		SessionTx: 3000,
	}
	collector.mu.Unlock()

	collector.endSession(context.Background(), "sess-end-1")

	collector.mu.Lock()
	_, exists := collector.peerSessions["sess-end-1"]
	collector.mu.Unlock()

	if exists {
		t.Error("session should be removed after endSession")
	}

	sessions, _ := trafficRepo.ListSessions(context.Background(), "sess-end-1", 10)
	found := false
	for _, s := range sessions {
		if s.ID == sessionID && s.DisconnectedAt != nil {
			found = true
			if s.BytesRx != 5000 {
				t.Errorf("BytesRx = %d, want 5000", s.BytesRx)
			}
			if s.BytesTx != 3000 {
				t.Errorf("BytesTx = %d, want 3000", s.BytesTx)
			}
		}
	}
	if !found {
		t.Error("closed session should have DisconnectedAt set with correct bytes")
	}
}

func TestSingBoxStatsCollector_endSession_NoActiveSession(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	collector := NewSingBoxStatsCollector(nil, repository.NewTrafficRepository(db), nil, "127.0.0.1:1", "", testLogger())
	collector.peerSessions = make(map[string]*peerSessionInfo)

	collector.endSession(context.Background(), "nonexistent-peer")
}

func deactivateDemoPeers(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec("UPDATE wg_peers SET is_active = 0 WHERE id LIKE 'demo-%'")
	if err != nil {
		t.Fatalf("failed to deactivate demo peers: %v", err)
	}
}

func TestSingBoxStatsCollector_handleAggregateVLESS_Basic(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()
	deactivateDemoPeers(t, db)

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "agg-vl-1", Name: "AggVL1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-agg-vl-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true, TotalRx: 0, TotalTx: 0,
	})
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "agg-vl-2", Name: "AggVL2", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-agg-vl-2", PrivateKey: "pk", Address: "addr2",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true, TotalRx: 0, TotalTx: 0,
	})

	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, "127.0.0.1:1", "", testLogger())
	collector.peerRealtime = make(map[string]*models.PeerRealtimeStats)
	collector.peerSessions = make(map[string]*peerSessionInfo)

	delta := &userDelta{
		rx: 1000,
		tx: 500,
		connections: []userConnection{
			{sourceIP: "10.0.0.1", host: "example.com", rx: 600, tx: 300},
			{sourceIP: "10.0.0.2", host: "test.com", rx: 400, tx: 200},
		},
	}
	currentOnline := make(map[string]bool)
	peerConnCounts := map[string]int{"agg-vl-1": 2}

	collector.mu.Lock()
	collector.sourceIPToPeerID = map[string]string{
		"10.0.0.1": "agg-vl-1",
		"10.0.0.2": "agg-vl-2",
	}
	collector.mu.Unlock()

	collector.handleAggregateVLESS(context.Background(), delta, currentOnline, peerConnCounts)

	if len(currentOnline) != 2 {
		t.Errorf("currentOnline count = %d, want 2", len(currentOnline))
	}
	if !currentOnline["agg-vl-1"] {
		t.Error("agg-vl-1 should be online")
	}
	if !currentOnline["agg-vl-2"] {
		t.Error("agg-vl-2 should be online")
	}

	p1, _ := peerRepo.GetByID(context.Background(), "agg-vl-1")
	p2, _ := peerRepo.GetByID(context.Background(), "agg-vl-2")
	totalDistributed := p1.TotalRx + p2.TotalRx
	if totalDistributed != 1000 {
		t.Errorf("total distributed rx = %d, want 1000", totalDistributed)
	}
	totalTx := p1.TotalTx + p2.TotalTx
	if totalTx != 500 {
		t.Errorf("total distributed tx = %d, want 500", totalTx)
	}

	if p1.LastSeen == nil {
		t.Error("agg-vl-1 LastSeen should be set")
	}
	if p2.LastSeen == nil {
		t.Error("agg-vl-2 LastSeen should be set")
	}

	stats := collector.GetRealtimeStats()
	if s, ok := stats["agg-vl-1"]; !ok {
		t.Error("expected realtime stats for agg-vl-1")
	} else {
		if s.ActiveConnections != 2 {
			t.Errorf("agg-vl-1 ActiveConnections = %d, want 2", s.ActiveConnections)
		}
	}
}

func TestSingBoxStatsCollector_handleAggregateVLESS_NoData(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "agg-no-1", Name: "AggNo", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-agg-no-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, "127.0.0.1:1", "", testLogger())
	collector.peerRealtime = make(map[string]*models.PeerRealtimeStats)
	collector.peerSessions = make(map[string]*peerSessionInfo)

	delta := &userDelta{
		rx: 0, tx: 0,
		connections: []userConnection{
			{sourceIP: "10.0.0.1", host: "", rx: 0, tx: 0},
		},
	}
	currentOnline := make(map[string]bool)

	collector.mu.Lock()
	collector.sourceIPToPeerID = map[string]string{
		"10.0.0.1": "agg-no-1",
	}
	collector.mu.Unlock()

	collector.handleAggregateVLESS(context.Background(), delta, currentOnline, nil)

	if !currentOnline["agg-no-1"] {
		t.Error("peer should be marked online even with no traffic")
	}

	p, _ := peerRepo.GetByID(context.Background(), "agg-no-1")
	if p.TotalRx != 0 {
		t.Errorf("TotalRx = %d, want 0", p.TotalRx)
	}
	if p.LastSeen == nil {
		t.Error("LastSeen should still be updated")
	}
}

func TestSingBoxStatsCollector_handleAggregateVLESS_NoActivePeers(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()
	deactivateDemoPeers(t, db)

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "agg-inactive-1", Name: "Inactive", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-inactive", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: false,
	})

	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, "127.0.0.1:1", "", testLogger())
	collector.peerRealtime = make(map[string]*models.PeerRealtimeStats)

	delta := &userDelta{rx: 500, tx: 200}
	currentOnline := make(map[string]bool)

	collector.handleAggregateVLESS(context.Background(), delta, currentOnline, nil)

	if len(currentOnline) != 0 {
		t.Errorf("currentOnline should be empty with no active peers, got %d", len(currentOnline))
	}
}

func TestWireGuardService_GenerateClientConfigCompact(t *testing.T) {
	svc := NewWireGuardService(nil, nil, testVLESSConfig(), testLogger())

	t.Run("iphone", func(t *testing.T) {
		peer := &models.Peer{
			PublicKey:  "compact-uuid-iphone",
			DeviceType: models.DeviceTypeIPhone,
		}
		config, err := svc.GenerateClientConfigCompact(peer)
		if err != nil {
			t.Fatalf("GenerateClientConfigCompact: %v", err)
		}
		if config == "" {
			t.Fatal("config is empty")
		}
		if !contains(config, "vless") {
			t.Error("config should contain vless type")
		}
		if !contains(config, "compact-uuid-iphone") {
			t.Error("config should contain UUID")
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(config), &parsed); err != nil {
			t.Fatalf("config should be valid JSON: %v", err)
		}
		if contains(config, "\n") {
			t.Error("compact config should not contain newlines")
		}
	})

	t.Run("android", func(t *testing.T) {
		peer := &models.Peer{
			PublicKey:  "compact-uuid-android",
			DeviceType: models.DeviceTypeAndroid,
		}
		config, err := svc.GenerateClientConfigCompact(peer)
		if err != nil {
			t.Fatalf("GenerateClientConfigCompact: %v", err)
		}
		if !contains(config, "gvisor") {
			t.Error("Android compact config should use gvisor stack")
		}
		if !contains(config, "package_name") {
			t.Error("Android compact config should contain package_name")
		}
	})

	t.Run("empty_device_type", func(t *testing.T) {
		peer := &models.Peer{
			PublicKey:  "compact-uuid-default",
			DeviceType: "",
		}
		config, err := svc.GenerateClientConfigCompact(peer)
		if err != nil {
			t.Fatalf("GenerateClientConfigCompact: %v", err)
		}
		if !contains(config, "mixed") {
			t.Error("empty device type should fallback to mixed stack")
		}
	})
}

func TestWireGuardService_GetPeerStats_NotFound(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	svc := NewWireGuardService(repository.NewPeerRepository(db), repository.NewTrafficRepository(db), testVLESSConfig(), testLogger())

	_, err := svc.GetPeerStats(context.Background(), "nonexistent-peer-id")
	if err == nil {
		t.Fatal("expected error for nonexistent peer")
	}
}

func TestTrafficService_GetPeerStats_WithData(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)

	_ = peerRepo.Create(context.Background(), &models.Peer{
		ID: "tps-1", Name: "TPeer", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uuid-tps-1", PrivateKey: "pk", Address: "addr1",
		DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	})

	svc := NewTrafficService(trafficRepo, peerRepo, testLogger())

	_ = svc.LogTraffic(context.Background(), &models.TrafficLog{
		PeerID:  "tps-1",
		Domain:  "example.com",
		Action:  "test",
		BytesRx: 2048,
		BytesTx: 1024,
	})

	stats, err := svc.GetPeerStats(context.Background(), "tps-1")
	if err != nil {
		t.Fatalf("GetPeerStats: %v", err)
	}
	if stats == nil {
		t.Fatal("stats should not be nil")
	}
	if stats.PeerID != "tps-1" {
		t.Errorf("PeerID = %q, want tps-1", stats.PeerID)
	}
}

func TestAuthService_RefreshToken_Success(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	authRepo := repository.NewAuthRepository(db)
	cfg := testJWTConfig()
	svc := NewAuthService(authRepo, cfg, testLogger())

	tp, err := svc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	newPair, err := svc.RefreshToken(context.Background(), tp.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if newPair.AccessToken == "" {
		t.Error("access token should not be empty")
	}
	if newPair.RefreshToken == "" {
		t.Error("refresh token should not be empty")
	}
	if newPair.RefreshToken == tp.RefreshToken {
		t.Error("new refresh token should differ from old")
	}
}

func TestAuthService_RefreshToken_InvalidToken(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	authRepo := repository.NewAuthRepository(db)
	cfg := testJWTConfig()
	svc := NewAuthService(authRepo, cfg, testLogger())

	_, err := svc.RefreshToken(context.Background(), "nonexistent-token")
	if err == nil {
		t.Error("expected error for invalid refresh token")
	}
}

func TestAuthService_LogoutAll(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	authRepo := repository.NewAuthRepository(db)
	cfg := testJWTConfig()
	svc := NewAuthService(authRepo, cfg, testLogger())

	tp2, err := svc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	_ = tp2

	err = svc.LogoutAll(context.Background(), "admin-001")
	if err != nil {
		t.Errorf("LogoutAll: %v", err)
	}
}

func TestMapRepoError(t *testing.T) {
	if mapRepoError(repository.ErrNotFound) != apperrors.ErrNotFound {
		t.Error("expected ErrNotFound mapping")
	}
	if mapRepoError(errors.New("other")) == apperrors.ErrNotFound {
		t.Error("unexpected mapping for generic error")
	}
}

func TestSingBoxStatsCollector_clearPeerRealtime(t *testing.T) {
	collector := NewSingBoxStatsCollector(nil, nil, nil, "127.0.0.1:1", "", testLogger())
	collector.peerRealtime = map[string]*models.PeerRealtimeStats{
		"p1": {ActiveConnections: 1},
		"p2": {ActiveConnections: 2},
	}
	collector.clearPeerRealtime("p1")
	if _, ok := collector.peerRealtime["p1"]; ok {
		t.Error("p1 should be removed")
	}
	if _, ok := collector.peerRealtime["p2"]; !ok {
		t.Error("p2 should still exist")
	}
}

func TestSingBoxStatsCollector_Start_Stop(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	collector := NewSingBoxStatsCollector(peerRepo, trafficRepo, nil, "127.0.0.1:1", "", testLogger())
	collector.interval = 50 * time.Millisecond

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"inbounds":[],"outbounds":[]}`)
	}))
	defer ts.Close()
	collector.apiURL = ts.URL

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		collector.Start(ctx)
		close(done)
	}()

	<-done
}

func TestIsVLESSInbound_CaseInsensitive(t *testing.T) {
	tests := []struct {
		connType string
		want     bool
	}{
		{"vless", true},
		{"VLESS", true},
		{"Vless", true},
		{"VLESS-IN", true},
		{"vless-in", true},
		{"tcp", false},
		{"", false},
		{"http", false},
	}
	for _, tt := range tests {
		got := isVLESSInbound(tt.connType)
		if got != tt.want {
			t.Errorf("isVLESSInbound(%q) = %v, want %v", tt.connType, got, tt.want)
		}
	}
}

func TestSingBoxStatsCollector_ComputeDeltas_VLESSUpperCaseType(t *testing.T) {
	collector := &SingBoxStatsCollector{
		connState: make(map[string]*connBytes),
	}

	connections := []clashConnection{
		{ID: "conn1", Upload: 100, Download: 500, Metadata: clashMetadata{User: "", Type: "VLESS"}},
	}

	deltas := collector.computeDeltas(connections)

	_, ok := deltas[aggregateVLESSKey]
	if !ok {
		t.Fatal("expected delta for aggregateVLESSKey when Type is uppercase VLESS")
	}
}

func TestWGStatsCollector_runWG(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	collector := NewWGStatsCollector(nil, nil, nil, "wg0", testLogger())

	t.Run("invalid_interface", func(t *testing.T) {
		if _, err := exec.LookPath("wg"); err != nil {
			t.Skip("wg command not available")
		}

		collector.iface = "nonexistent-wg0"
		_, err := collector.runWG("transfer")
		if err == nil {
			t.Error("expected error for nonexistent interface")
		}
	})
}

func TestWGStatsCollector_collect(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	alertSvc := NewTrafficService(trafficRepo, peerRepo, testLogger())

	collector := NewWGStatsCollector(peerRepo, trafficRepo, alertSvc, "wg0", testLogger())
	collector.interval = 50 * time.Millisecond

	t.Run("error_case", func(t *testing.T) {
		collector.iface = "nonexistent-wg0"

		ctx := context.Background()
		collector.collect(ctx)

		if collector.IsWGActive() {
			t.Error("WG should not be active after error")
		}
	})
}

func TestWGStatsCollector_Start(t *testing.T) {
	db, _ := repository.InitDB(":memory:", migrations.Files)
	defer func() { _ = db.Close() }()

	peerRepo := repository.NewPeerRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	alertSvc := NewTrafficService(trafficRepo, peerRepo, testLogger())

	collector := NewWGStatsCollector(peerRepo, trafficRepo, alertSvc, "wg0", testLogger())
	collector.interval = 50 * time.Millisecond

	t.Run("start_and_stop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()

		done := make(chan struct{})
		go func() {
			collector.Start(ctx)
			close(done)
		}()

		<-done
	})
}
