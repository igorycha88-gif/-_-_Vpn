package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"smarttraffic/internal/config"
	"smarttraffic/internal/middleware"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"
	"smarttraffic/migrations"
)

type testDeps struct {
	db          *sql.DB
	authSvc     *services.AuthService
	authHandler *AuthHandler
	peerHandler *PeerHandler
	routeHandler *RouteHandler
	dnsHandler  *DNSHandler
	presetHandler *PresetHandler
	monitoringHandler *MonitoringHandler
	serverHandler *ServerHandler
	sbSvc       *services.SingBoxService
	wgSvc       *services.WireGuardService
	trafficSvc  *services.TrafficService
	collector   *services.WGStatsCollector
	logger      *slog.Logger
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()
	db, err := repository.InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	jwtCfg := &config.JWTConfig{Secret: "test-secret-key-at-least-32-chars!", AccessTTL: 15 * time.Minute, RefreshTTL: 168 * time.Hour}
	vlessCfg := &config.VLESSConfig{PrivateKey: "pk", PublicKey: "pub", ShortID: "sid", ServerName: "www.microsoft.com", Port: 443, Flow: "xtls-rprx-vision", Fingerprint: "chrome", ServerEndpoint: "1.2.3.4"}
	sbCfg := &config.SingBoxConfig{ConfigPath: t.TempDir() + "/config.json", ClashAPIAddr: "127.0.0.1:9090"}
	wgCfg := &config.WGConfig{TunnelLocalAddress: "10.20.0.2/30", TunnelPrivateKey: "testkey", TunnelPeerPublicKey: "testpeerkey"}
	srvCfg := &config.ServerConfig{ForeignIP: "1.2.3.4", ForeignVLESS: config.ForeignVLESSConfig{UUID: "test-uuid", ServerName: "www.microsoft.com", RealityPublicKey: "test-pk", RealityShortID: "test-sid"}}

	peerRepo := repository.NewPeerRepository(db)
	routeRepo := repository.NewRouteRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	authRepo := repository.NewAuthRepository(db)
	dnsRepo := repository.NewDNSRepository(db)
	presetRepo := repository.NewPresetRepository(db)

	authSvc := services.NewAuthService(authRepo, jwtCfg, logger)
	wgSvc := services.NewWireGuardService(peerRepo, trafficRepo, vlessCfg, logger)
	sbSvc := services.NewSingBoxService(routeRepo, dnsRepo, peerRepo, sbCfg, vlessCfg, wgCfg, srvCfg, logger)
	routingSvc := services.NewRoutingService(routeRepo, logger)
	dnsSvc := services.NewDNSService(dnsRepo, logger)
	presetSvc := services.NewPresetService(presetRepo, routeRepo, logger)
	trafficSvc := services.NewTrafficService(trafficRepo, peerRepo, logger)
	collector := services.NewWGStatsCollector(peerRepo, trafficRepo, nil, "wg0", logger)

	return &testDeps{
		db: db, authSvc: authSvc, logger: logger,
		authHandler: NewAuthHandler(authSvc, logger),
		peerHandler: NewPeerHandler(wgSvc, sbSvc, logger),
		routeHandler: NewRouteHandler(routingSvc, sbSvc, logger),
		dnsHandler: NewDNSHandler(dnsSvc, logger),
		presetHandler: NewPresetHandler(routingSvc, sbSvc, presetSvc, logger),
		monitoringHandler: NewMonitoringHandler(trafficSvc, wgSvc, nil, logger),
		serverHandler: NewServerHandler(trafficSvc, collector, logger),
		sbSvc: sbSvc, wgSvc: wgSvc, trafficSvc: trafficSvc, collector: collector,
	}
}

func (d *testDeps) authenticatedRequest(method, path, body string) *http.Request {
	tokens, _ := d.authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, "admin-001")
	ctx = context.WithValue(ctx, middleware.EmailKey, "admin@smarttraffic.local")
	ctx = context.WithValue(ctx, middleware.RoleKey, "admin")
	return r.WithContext(ctx)
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func readBody(resp *http.Response) map[string]interface{} {
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result
}

func readBodySlice(resp *http.Response) []interface{} {
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	var result []interface{}
	json.Unmarshal(body, &result)
	return result
}

func TestAuthHandler_Login_Success(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.LoginRequest{Email: "admin@smarttraffic.local", Password: "admin123"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Login(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["access_token"] == nil {
		t.Error("expected access_token in response")
	}
	if data["refresh_token"] == nil {
		t.Error("expected refresh_token in response")
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Login(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAuthHandler_Login_EmptyFields(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.LoginRequest{Email: "", Password: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Login(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAuthHandler_Login_WrongCredentials(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.LoginRequest{Email: "admin@smarttraffic.local", Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Login(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthHandler_Login_SQLInjection(t *testing.T) {
	d := newTestDeps(t)
	body := `{"email": "admin@smarttraffic.local' OR '1'='1", "password": "anything"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Login(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("SQL injection should fail with 401, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_Refresh_Success(t *testing.T) {
	d := newTestDeps(t)
	tokens, _ := d.authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	body := toJSON(models.RefreshTokenRequest{RefreshToken: tokens.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Refresh(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAuthHandler_Refresh_InvalidToken(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.RefreshTokenRequest{RefreshToken: "invalid"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Refresh(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestAuthHandler_Session_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/auth/session", "")
	w := httptest.NewRecorder()
	d.authHandler.Session(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["email"] != "admin@smarttraffic.local" {
		t.Errorf("email = %v, want admin@smarttraffic.local", data["email"])
	}
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	d := newTestDeps(t)
	tokens, _ := d.authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	body := toJSON(models.RefreshTokenRequest{RefreshToken: tokens.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.Logout(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAuthHandler_LogoutAll_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/auth/logout-all", "")
	w := httptest.NewRecorder()
	d.authHandler.LogoutAll(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestAuthHandler_LogoutAll_NoAuth(t *testing.T) {
	d := newTestDeps(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout-all", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.authHandler.LogoutAll(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestPeerHandler_List_ReturnsSeedPeers(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/wg/peers", "")
	w := httptest.NewRecorder()
	d.peerHandler.List(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBodySlice(resp)
	if len(data) == 0 {
		t.Errorf("expected seed peers, got 0 items")
	}
}

func TestPeerHandler_Create_Success(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.PeerCreateRequest{Name: "Test iPhone", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/wg/peers", body)
	w := httptest.NewRecorder()
	d.peerHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		data := readBody(resp)
		if resp.StatusCode == http.StatusInternalServerError && data["error"] == "клиент создан, но не удалось перезапустить sing-box" {
			t.Skip("sing-box restart unavailable in test environment")
		}
		t.Fatalf("status = %d, want 201, body: %v", resp.StatusCode, data)
	}
	data := readBody(resp)
	if data["name"] != "Test iPhone" {
		t.Errorf("name = %v, want Test iPhone", data["name"])
	}
	if data["public_key"] == nil {
		t.Error("expected public_key in response")
	}
}

func TestPeerHandler_Create_InvalidDeviceType(t *testing.T) {
	d := newTestDeps(t)
	body := `{"name":"Test","device_type":"windows"}`
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/wg/peers", body)
	w := httptest.NewRecorder()
	d.peerHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPeerHandler_Create_EmptyName(t *testing.T) {
	d := newTestDeps(t)
	body := `{"name":"","device_type":"iphone"}`
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/wg/peers", body)
	w := httptest.NewRecorder()
	d.peerHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPeerHandler_Create_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/wg/peers", "bad json")
	w := httptest.NewRecorder()
	d.peerHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPeerHandler_Get_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Test", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/wg/peers/"+peer.ID, "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.Get(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if pk, exists := data["private_key"]; exists && pk != "" {
		t.Error("private_key should be stripped from response")
	}
}

func TestPeerHandler_Get_NotFound(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/wg/peers/nonexistent", "")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	d.peerHandler.Get(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPeerHandler_Delete_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Test", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodDelete, "/api/v1/wg/peers/"+peer.ID, "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.Delete(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPeerHandler_Delete_WithActiveSession(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "SessionTest", DeviceType: models.DeviceTypeIPhone})

	trafficRepo := repository.NewTrafficRepository(d.db)
	_, err := trafficRepo.CreateSession(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	trafficRepo.Log(context.Background(), &models.TrafficLog{
		PeerID: peer.ID, Domain: "youtube.com", Action: "proxy", BytesRx: 5000, BytesTx: 3000,
	})

	req := d.authenticatedRequest(http.MethodDelete, "/api/v1/wg/peers/"+peer.ID, "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.Delete(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		data := readBody(resp)
		t.Fatalf("status = %d, want 200, body: %v", resp.StatusCode, data)
	}

	_, err = d.wgSvc.GetPeer(context.Background(), peer.ID)
	if err == nil {
		t.Fatal("expected error after delete, peer should not exist")
	}
}

func TestPeerHandler_Delete_WithClosedSession(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "ClosedSess", DeviceType: models.DeviceTypeIPhone})

	trafficRepo := repository.NewTrafficRepository(d.db)
	sid, err := trafficRepo.CreateSession(context.Background(), peer.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	trafficRepo.CloseSession(context.Background(), sid, 1024, 2048, 5)

	req := d.authenticatedRequest(http.MethodDelete, "/api/v1/wg/peers/"+peer.ID, "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.Delete(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPeerHandler_Delete_NotFound(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodDelete, "/api/v1/wg/peers/nonexistent", "")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	d.peerHandler.Delete(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPeerHandler_DownloadConfig_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "ConfigTest", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/wg/peers/"+peer.ID+"/config", "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.DownloadConfig(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}
}

func TestPeerHandler_GetQRCode_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "QRTest", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/wg/peers/"+peer.ID+"/qr", "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.GetQRCode(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
}

func TestPeerHandler_Toggle_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Toggle", DeviceType: models.DeviceTypeIPhone})
	body := `{"active":false}`
	req := d.authenticatedRequest(http.MethodPut, "/api/v1/wg/peers/"+peer.ID+"/toggle", body)
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.Toggle(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPeerHandler_GetStats_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Stats", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/wg/peers/"+peer.ID+"/stats", "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.peerHandler.GetStats(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRouteHandler_List_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/routes", "")
	w := httptest.NewRecorder()
	d.routeHandler.List(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRouteHandler_Create_Success(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.RoutingRuleCreateRequest{Name: "Block YouTube", Type: "domain", Pattern: "youtube.com", Action: "proxy"})
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/routes", body)
	w := httptest.NewRecorder()
	d.routeHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	data := readBody(resp)
	if data["name"] != "Block YouTube" {
		t.Errorf("name = %v, want Block YouTube", data["name"])
	}
}

func TestRouteHandler_Create_InvalidType(t *testing.T) {
	d := newTestDeps(t)
	body := `{"name":"Bad","type":"invalid","pattern":"test","action":"direct"}`
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/routes", body)
	w := httptest.NewRecorder()
	d.routeHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRouteHandler_Create_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/routes", "bad")
	w := httptest.NewRecorder()
	d.routeHandler.Create(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestRouteHandler_Get_Success(t *testing.T) {
	d := newTestDeps(t)
	svc := services.NewRoutingService(repository.NewRouteRepository(d.db), d.logger)
	rule, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "Get", Type: "domain", Pattern: "test.com", Action: "direct"})
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/routes/"+rule.ID, "")
	req.SetPathValue("id", rule.ID)
	w := httptest.NewRecorder()
	d.routeHandler.Get(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRouteHandler_Get_NotFound(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/routes/nonexistent", "")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	d.routeHandler.Get(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRouteHandler_Update_Success(t *testing.T) {
	d := newTestDeps(t)
	svc := services.NewRoutingService(repository.NewRouteRepository(d.db), d.logger)
	rule, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "Upd", Type: "domain", Pattern: "test.com", Action: "direct"})
	newName := "Updated"
	body := toJSON(models.RoutingRuleUpdateRequest{Name: &newName})
	req := d.authenticatedRequest(http.MethodPut, "/api/v1/routes/"+rule.ID, body)
	req.SetPathValue("id", rule.ID)
	w := httptest.NewRecorder()
	d.routeHandler.Update(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["name"] != "Updated" {
		t.Errorf("name = %v, want Updated", data["name"])
	}
}

func TestRouteHandler_Delete_Success(t *testing.T) {
	d := newTestDeps(t)
	svc := services.NewRoutingService(repository.NewRouteRepository(d.db), d.logger)
	rule, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "Del", Type: "domain", Pattern: "test.com", Action: "direct"})
	req := d.authenticatedRequest(http.MethodDelete, "/api/v1/routes/"+rule.ID, "")
	req.SetPathValue("id", rule.ID)
	w := httptest.NewRecorder()
	d.routeHandler.Delete(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRouteHandler_Reorder_Success(t *testing.T) {
	d := newTestDeps(t)
	svc := services.NewRoutingService(repository.NewRouteRepository(d.db), d.logger)
	r1, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "A", Type: "domain", Pattern: "a.com", Action: "direct"})
	r2, _ := svc.CreateRule(context.Background(), &models.RoutingRuleCreateRequest{Name: "B", Type: "domain", Pattern: "b.com", Action: "proxy"})
	body := toJSON(models.ReorderRequest{IDs: []string{r2.ID, r1.ID}})
	req := d.authenticatedRequest(http.MethodPut, "/api/v1/routes/reorder", body)
	w := httptest.NewRecorder()
	d.routeHandler.Reorder(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRouteHandler_Reorder_EmptyIDs(t *testing.T) {
	d := newTestDeps(t)
	body := toJSON(models.ReorderRequest{IDs: []string{}})
	req := d.authenticatedRequest(http.MethodPut, "/api/v1/routes/reorder", body)
	w := httptest.NewRecorder()
	d.routeHandler.Reorder(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDNSHandler_Get_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/dns/settings", "")
	w := httptest.NewRecorder()
	d.dnsHandler.Get(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["upstream_ru"] == nil {
		t.Error("expected upstream_ru in response")
	}
}

func TestDNSHandler_Update_Success(t *testing.T) {
	d := newTestDeps(t)
	newRU := "8.8.4.4"
	body := toJSON(models.DNSSettingsUpdateRequest{UpstreamRU: &newRU})
	req := d.authenticatedRequest(http.MethodPut, "/api/v1/dns/settings", body)
	w := httptest.NewRecorder()
	d.dnsHandler.Update(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["upstream_ru"] != "8.8.4.4" {
		t.Errorf("upstream_ru = %v, want 8.8.4.4", data["upstream_ru"])
	}
}

func TestDNSHandler_Update_InvalidJSON(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPut, "/api/v1/dns/settings", "bad json")
	w := httptest.NewRecorder()
	d.dnsHandler.Update(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDNSHandler_ListPresets_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/dns/presets", "")
	w := httptest.NewRecorder()
	d.dnsHandler.ListPresets(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPresetHandler_List_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/presets", "")
	w := httptest.NewRecorder()
	d.presetHandler.List(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestPresetHandler_Apply_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/presets/preset-all-direct/apply", "")
	req.SetPathValue("id", "preset-all-direct")
	w := httptest.NewRecorder()
	d.presetHandler.Apply(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %v", resp.StatusCode, readBody(resp))
	}
	data := readBody(resp)
	if data["applied_rules"] == nil {
		t.Error("expected applied_rules in response")
	}
}

func TestPresetHandler_Apply_NotFound(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/presets/nonexistent/apply", "")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	d.presetHandler.Apply(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMonitoringHandler_Stats_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/monitoring/stats", "")
	w := httptest.NewRecorder()
	d.monitoringHandler.Stats(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["total_peers"] == nil {
		t.Error("expected total_peers in response")
	}
}

func TestMonitoringHandler_Traffic_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/monitoring/traffic", "")
	w := httptest.NewRecorder()
	d.monitoringHandler.Traffic(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMonitoringHandler_Alerts_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/monitoring/alerts", "")
	w := httptest.NewRecorder()
	d.monitoringHandler.Alerts(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMonitoringHandler_PeersStats_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/monitoring/peers-stats", "")
	w := httptest.NewRecorder()
	d.monitoringHandler.PeersStats(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestMonitoringHandler_PeerMonitor_Success(t *testing.T) {
	d := newTestDeps(t)
	peer, _ := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Mon", DeviceType: models.DeviceTypeIPhone})
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/monitoring/peer/"+peer.ID, "")
	req.SetPathValue("id", peer.ID)
	w := httptest.NewRecorder()
	d.monitoringHandler.PeerMonitor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["peer"] == nil {
		t.Error("expected peer in response")
	}
}

func TestMonitoringHandler_PeerMonitor_NotFound(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/monitoring/peer/nonexistent", "")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	d.monitoringHandler.PeerMonitor(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServerHandler_Status_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/servers/status", "")
	w := httptest.NewRecorder()
	d.serverHandler.Status(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	ruStatus, ok := data["ru"].(map[string]interface{})
	if !ok || ruStatus["online"] != true {
		t.Error("expected ru.online = true")
	}
}

func TestServerHandler_RUStats_Success(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodGet, "/api/v1/servers/ru/stats", "")
	w := httptest.NewRecorder()
	d.serverHandler.RUStats(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestServerHandler_Health_Success(t *testing.T) {
	d := newTestDeps(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	d.serverHandler.Health(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	data := readBody(resp)
	if data["status"] != "ok" {
		t.Errorf("status = %v, want ok", data["status"])
	}
	if data["goroutines"] == nil {
		t.Error("expected goroutines in health response")
	}
}

func TestResponseHelpers_JSON(t *testing.T) {
	w := httptest.NewRecorder()
	JSON(w, http.StatusOK, map[string]string{"test": "value"})
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want json", ct)
	}
}

func TestResponseHelpers_ErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()
	ErrorJSON(w, http.StatusBadRequest, "test error")
	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	data := readBody(resp)
	if data["error"] != "test error" {
		t.Errorf("error = %v, want 'test error'", data["error"])
	}
}

func TestResponseHelpers_DecodeJSON(t *testing.T) {
	body := `{"name":"test","count":42}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	var result struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := DecodeJSON(req, &result); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("Name = %q, want test", result.Name)
	}
	if result.Count != 42 {
		t.Errorf("Count = %d, want 42", result.Count)
	}
}

func TestResponseHelpers_DecodeJSON_DisallowsUnknownFields(t *testing.T) {
	body := `{"unknown_field":123}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	var result struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(req, &result); err == nil {
		t.Error("expected error for unknown fields")
	}
}

func TestPeerHandler_Sync_SingBoxUnavailable(t *testing.T) {
	d := newTestDeps(t)
	req := d.authenticatedRequest(http.MethodPost, "/api/v1/wg/peers/sync", "")
	w := httptest.NewRecorder()
	d.peerHandler.Sync(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("sing-box reload unavailable in test environment (status=%d)", resp.StatusCode)
	}
}

func TestFullAPIWorkflow(t *testing.T) {
	d := newTestDeps(t)

	tokens, err := d.authSvc.Login(context.Background(), "admin@smarttraffic.local", "admin123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	auth := func(req *http.Request) *http.Request {
		req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
		return req
	}

	peerBody := toJSON(models.PeerCreateRequest{Name: "Workflow", DeviceType: models.DeviceTypeIPhone})
	req := auth(httptest.NewRequest(http.MethodPost, "/api/v1/wg/peers", strings.NewReader(peerBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	d.peerHandler.Create(w, req)
	createResp := w.Result()
	createBody, _ := io.ReadAll(createResp.Body)
	var peerResp map[string]interface{}
	json.Unmarshal(createBody, &peerResp)

	if createResp.StatusCode == http.StatusInternalServerError {
		if peerResp["error"] == "клиент создан, но не удалось перезапустить sing-box" {
			peer, err := d.wgSvc.CreatePeer(context.Background(), &models.PeerCreateRequest{Name: "Workflow", DeviceType: models.DeviceTypeIPhone})
			if err != nil {
				t.Fatalf("CreatePeer via service: %v", err)
			}
			peerResp = map[string]interface{}{"id": peer.ID}
		} else {
			t.Fatalf("Create peer: %d, body: %s", createResp.StatusCode, string(createBody))
		}
	} else if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("Create peer: %d", createResp.StatusCode)
	}
	peerID := peerResp["id"].(string)

	ruleBody := toJSON(models.RoutingRuleCreateRequest{Name: "Route YouTube", Type: "domain", Pattern: "youtube.com", Action: "proxy"})
	req = auth(httptest.NewRequest(http.MethodPost, "/api/v1/routes", strings.NewReader(ruleBody)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	d.routeHandler.Create(w, req)
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("Create rule: %d", w.Result().StatusCode)
	}

	req = auth(httptest.NewRequest(http.MethodGet, "/api/v1/monitoring/stats", nil))
	w = httptest.NewRecorder()
	d.monitoringHandler.Stats(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Stats: %d", w.Result().StatusCode)
	}

	req = auth(httptest.NewRequest(http.MethodGet, "/api/v1/wg/peers/"+peerID, nil))
	req.SetPathValue("id", peerID)
	w = httptest.NewRecorder()
	d.peerHandler.Get(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Get peer: %d", w.Result().StatusCode)
	}

	req = auth(httptest.NewRequest(http.MethodDelete, "/api/v1/wg/peers/"+peerID, nil))
	req.SetPathValue("id", peerID)
	w = httptest.NewRecorder()
	d.peerHandler.Delete(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("Delete peer: %d", w.Result().StatusCode)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func init() {
	_ = bytes.NewReader(nil)
}
