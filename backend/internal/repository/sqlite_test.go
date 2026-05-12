package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"smarttraffic/internal/models"
	"smarttraffic/migrations"
)

func initTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := InitDB(":memory:", migrations.Files)
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestPeerRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	peer := &models.Peer{
		ID: "test-peer-1", Name: "Test Peer", Email: "test@example.com",
		DeviceType: models.DeviceTypeIPhone,
		PublicKey: "dGVzdHB1YmxpY2tleQ==", PrivateKey: "dGVzdHByaXZhdGVrZXk=",
		Address: "10.99.0.2", DNS: "1.1.1.1", MTU: 1280, IsActive: true,
	}

	if err := repo.Create(ctx, peer); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "test-peer-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Test Peer" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Peer")
	}

	peers, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(peers) < 1 {
		t.Errorf("List count = %d, want >= 1", len(peers))
	}

	got.Name = "Updated Peer"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := repo.GetByID(ctx, "test-peer-1")
	if got2.Name != "Updated Peer" {
		t.Errorf("After update Name = %q, want Updated Peer", got2.Name)
	}

	count, _ := repo.Count(ctx)
	if count < 1 {
		t.Errorf("Count = %d, want >= 1", count)
	}

	activeCount, _ := repo.CountActive(ctx)
	if activeCount < 1 {
		t.Errorf("CountActive = %d, want >= 1", activeCount)
	}

	if err := repo.Delete(ctx, "test-peer-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = repo.GetByID(ctx, "test-peer-1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestPeerRepository_NotFound(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent peer")
	}

	if err := repo.Delete(ctx, "nonexistent"); err == nil {
		t.Fatal("expected error for deleting nonexistent")
	}
}

func TestPeerRepository_GetByPublicKey(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Peer{
		ID: "pk-test", Name: "PK", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "uniquepubkey123",
		PrivateKey: "uniqueprivkey123", Address: "10.99.0.3", IsActive: true,
	})

	got, err := repo.GetByPublicKey(ctx, "uniquepubkey123")
	if err != nil {
		t.Fatalf("GetByPublicKey: %v", err)
	}
	if got.ID != "pk-test" {
		t.Errorf("ID = %q, want pk-test", got.ID)
	}
}

func TestPeerRepository_UpdateTraffic(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Peer{
		ID: "t-test", Name: "T", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "tpk", PrivateKey: "tpv",
		Address: "10.99.0.4", IsActive: true,
	})

	_ = repo.UpdateTraffic(ctx, "t-test", 1024, 2048)
	got, _ := repo.GetByID(ctx, "t-test")
	if got.TotalRx != 1024 || got.TotalTx != 2048 {
		t.Errorf("Rx=%d Tx=%d, want 1024,2048", got.TotalRx, got.TotalTx)
	}
}

func TestPeerRepository_UpdateLastSeen(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	_ = repo.Create(ctx, &models.Peer{
		ID: "seen-test", Name: "S", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "spk", PrivateKey: "spv",
		Address: "10.99.0.5", IsActive: true,
	})

	_ = repo.UpdateLastSeen(ctx, "seen-test")
	got, _ := repo.GetByID(ctx, "seen-test")
	if got.LastSeen == nil {
		t.Fatal("LastSeen should not be nil")
	}
}

func TestRouteRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewRouteRepository(db)
	ctx := context.Background()

	initialCount, _ := repo.Count(ctx)

	rule := &models.RoutingRule{
		ID: "rule-1", Name: "Test", Type: "domain", Pattern: "example.com",
		Action: "direct", Priority: 1, IsActive: true,
	}

	if err := repo.Create(ctx, rule); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "rule-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Test" {
		t.Errorf("Name = %q, want Test", got.Name)
	}

	rules, _ := repo.List(ctx)
	if len(rules) != initialCount+1 {
		t.Errorf("List count = %d, want %d", len(rules), initialCount+1)
	}

	got.Name = "Updated"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	count, _ := repo.Count(ctx)
	if count != initialCount+1 {
		t.Errorf("Count = %d, want %d", count, initialCount+1)
	}

	if err := repo.Delete(ctx, "rule-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestRouteRepository_Reorder(t *testing.T) {
	db := initTestDB(t)
	repo := NewRouteRepository(db)
	ctx := context.Background()

	for i, n := range []string{"A", "B", "C"} {
		_ = repo.Create(ctx, &models.RoutingRule{
			ID: "r" + n, Name: n, Type: "domain",
			Pattern: n + ".com", Action: "direct", Priority: i + 10,
		})
	}

	if err := repo.Reorder(ctx, []string{"rC", "rA", "rB"}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	rules, _ := repo.List(ctx)
	var testRules []*models.RoutingRule
	for _, r := range rules {
		if r.ID == "rA" || r.ID == "rB" || r.ID == "rC" {
			testRules = append(testRules, r)
		}
	}
	if len(testRules) < 3 {
		t.Fatalf("expected at least 3 test rules, got %d", len(testRules))
	}
	if testRules[0].ID != "rC" {
		t.Errorf("first = %q, want rC", testRules[0].ID)
	}
}

func TestPresetRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewPresetRepository(db)
	ctx := context.Background()

	preset := &models.Preset{
		ID: "custom-preset", Name: "Custom", Description: "Test",
		Rules: `[{"type":"domain","pattern":"test.com","action":"proxy"}]`,
	}

	if err := repo.Create(ctx, preset); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "custom-preset")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Custom" {
		t.Errorf("Name = %q, want Custom", got.Name)
	}

	presets, _ := repo.List(ctx)
	if len(presets) < 1 {
		t.Errorf("List count = %d, want >= 1", len(presets))
	}

	if err := repo.Delete(ctx, "custom-preset"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestPresetRepository_CannotDeleteBuiltin(t *testing.T) {
	db := initTestDB(t)
	repo := NewPresetRepository(db)
	ctx := context.Background()

	err := repo.Delete(ctx, "preset-all-direct")
	if err == nil {
		t.Fatal("expected error deleting builtin preset")
	}
}

func TestDNSRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewDNSRepository(db)
	ctx := context.Background()

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UpstreamRU == "" {
		t.Error("UpstreamRU should not be empty")
	}

	got.BlockAds = true
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := repo.Get(ctx)
	if !got2.BlockAds {
		t.Error("BlockAds should be true")
	}
}

func TestAuthRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewAuthRepository(db)
	ctx := context.Background()

	user, err := repo.GetUserByEmail(ctx, "admin@smarttraffic.local")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.ID != "admin-001" {
		t.Errorf("ID = %q, want admin-001", user.ID)
	}

	user2, err := repo.GetUserByID(ctx, "admin-001")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user2.Email != "admin@smarttraffic.local" {
		t.Errorf("Email = %q, unexpected", user2.Email)
	}

	expiresAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	if err := repo.StoreRefreshToken(ctx, "admin-001", "token-123", expiresAt); err != nil {
		t.Fatalf("StoreRefreshToken: %v", err)
	}

	userID, err := repo.GetRefreshToken(ctx, "token-123")
	if err != nil {
		t.Fatalf("GetRefreshToken: %v", err)
	}
	if userID != "admin-001" {
		t.Errorf("UserID = %q, want admin-001", userID)
	}

	_, err = repo.GetRefreshToken(ctx, "bad-token")
	if err == nil {
		t.Fatal("expected error for bad token")
	}

	_ = repo.DeleteRefreshToken(ctx, "token-123")
	_, err = repo.GetRefreshToken(ctx, "token-123")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestAuthRepository_NotFound(t *testing.T) {
	db := initTestDB(t)
	repo := NewAuthRepository(db)
	ctx := context.Background()

	_, err := repo.GetUserByEmail(ctx, "no@no.com")
	if err == nil {
		t.Fatal("expected error for nonexistent email")
	}
}

func TestTrafficRepository_LogAndList(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "peer-1", Name: "P1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "pk1", PrivateKey: "pv1",
		Address: "10.99.0.6", IsActive: true,
	})

	if err := trafficRepo.Log(ctx, &models.TrafficLog{
		PeerID: "peer-1", Domain: "example.com", DestIP: "1.2.3.4",
		DestPort: 443, Action: "direct", BytesRx: 1000, BytesTx: 500,
	}); err != nil {
		t.Fatalf("Log: %v", err)
	}

	logs, err := trafficRepo.List(ctx, models.TrafficFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(logs) < 1 {
		t.Fatalf("List count = %d, want >= 1", len(logs))
	}
	found := false
	for _, l := range logs {
		if l.Domain == "example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find log with domain example.com")
	}
}

func TestTrafficRepository_FilterByPeer(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{ID: "p1", Name: "P1", DeviceType: models.DeviceTypeIPhone, PublicKey: "pk1", PrivateKey: "pv1", Address: "10.99.0.7", IsActive: true})
	_ = peerRepo.Create(ctx, &models.Peer{ID: "p2", Name: "P2", DeviceType: models.DeviceTypeAndroid, PublicKey: "pk2", PrivateKey: "pv2", Address: "10.99.0.8", IsActive: true})

	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "p1", Domain: "a.com", Action: "direct", BytesRx: 100, BytesTx: 50})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "p2", Domain: "b.com", Action: "proxy", BytesRx: 200, BytesTx: 100})

	logs, _ := trafficRepo.List(ctx, models.TrafficFilter{PeerID: "p1", Limit: 10})
	if len(logs) != 1 {
		t.Errorf("filtered count = %d, want 1", len(logs))
	}
}

func TestTrafficRepository_Cleanup(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_ = trafficRepo.Log(ctx, &models.TrafficLog{Action: "direct"})

	deleted, err := trafficRepo.CleanupOld(ctx, 0)
	if err != nil {
		t.Fatalf("CleanupOld: %v", err)
	}
	if deleted < 0 {
		t.Errorf("deleted = %d, want >= 0", deleted)
	}
}

func TestTrafficRepository_DeleteSessionsByPeerID(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "sess-peer", Name: "SP", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "spk1", PrivateKey: "spv1",
		Address: "10.99.0.9", IsActive: true,
	})

	sid1, err := trafficRepo.CreateSession(ctx, "sess-peer")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sid2, err := trafficRepo.CreateSession(ctx, "sess-peer")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	_ = trafficRepo.CloseSession(ctx, sid1, 100, 200, 3)

	active, err := trafficRepo.GetActiveSession(ctx, "sess-peer")
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active == nil || active.ID != sid2 {
		t.Fatal("expected active session to exist")
	}

	if err := trafficRepo.DeleteSessionsByPeerID(ctx, "sess-peer"); err != nil {
		t.Fatalf("DeleteSessionsByPeerID: %v", err)
	}

	active, err = trafficRepo.GetActiveSession(ctx, "sess-peer")
	if err != nil {
		t.Fatalf("GetActiveSession after delete: %v", err)
	}
	if active != nil {
		t.Fatal("expected nil session after DeleteSessionsByPeerID")
	}
}

func TestTrafficRepository_GetTotalStats(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "stats-p1", Name: "SP1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "statspk1", PrivateKey: "statspv1",
		Address: "10.99.1.1", IsActive: true,
	})
	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "stats-p2", Name: "SP2", DeviceType: models.DeviceTypeAndroid,
		PublicKey: "statspk2", PrivateKey: "statspv2",
		Address: "10.99.1.2", IsActive: false,
	})

	_ = peerRepo.UpdateTraffic(ctx, "stats-p1", 5000, 3000)
	_ = peerRepo.UpdateTraffic(ctx, "stats-p2", 1000, 2000)

	stats, err := trafficRepo.GetTotalStats(ctx)
	if err != nil {
		t.Fatalf("GetTotalStats: %v", err)
	}
	if stats.TotalRx < 6000 {
		t.Errorf("TotalRx = %d, want >= 6000", stats.TotalRx)
	}
	if stats.TotalTx < 5000 {
		t.Errorf("TotalTx = %d, want >= 5000", stats.TotalTx)
	}
	if stats.TotalPeers < 2 {
		t.Errorf("TotalPeers = %d, want >= 2", stats.TotalPeers)
	}
	if stats.ActivePeers < 1 {
		t.Errorf("ActivePeers = %d, want >= 1", stats.ActivePeers)
	}
	if stats.RulesCount <= 0 {
		t.Errorf("RulesCount = %d, want > 0", stats.RulesCount)
	}
}

func TestTrafficRepository_GetPeerStats(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "ps-peer", Name: "PS", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "pspk1", PrivateKey: "pspv1",
		Address: "10.99.2.1", IsActive: true,
	})
	_ = peerRepo.UpdateTraffic(ctx, "ps-peer", 1024, 2048)

	stats, err := trafficRepo.GetPeerStats(ctx, "ps-peer")
	if err != nil {
		t.Fatalf("GetPeerStats: %v", err)
	}
	if stats.TotalRx != 1024 {
		t.Errorf("TotalRx = %d, want 1024", stats.TotalRx)
	}
	if stats.TotalTx != 2048 {
		t.Errorf("TotalTx = %d, want 2048", stats.TotalTx)
	}
	if stats.PeerID != "ps-peer" {
		t.Errorf("PeerID = %q, want ps-peer", stats.PeerID)
	}
}

func TestTrafficRepository_GetPeerStats_NotFound(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_, err := trafficRepo.GetPeerStats(ctx, "nonexistent-peer")
	if err == nil {
		t.Fatal("expected error for nonexistent peer")
	}
}

func TestTrafficRepository_InsertAlert_ListAlerts(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_ = trafficRepo.DeleteAllAlerts(ctx)

	alert := &models.Alert{
		ID: "alert-1", Type: "traffic", Message: "High traffic detected",
		Severity: "warning", Timestamp: time.Now(),
	}
	if err := trafficRepo.InsertAlert(ctx, alert); err != nil {
		t.Fatalf("InsertAlert: %v", err)
	}

	alert2 := &models.Alert{
		ID: "alert-2", Type: "security", Message: "Suspicious activity",
		Severity: "critical", Timestamp: time.Now().Add(-1 * time.Hour),
	}
	_ = trafficRepo.InsertAlert(ctx, alert2)

	alerts, err := trafficRepo.ListAlerts(ctx, 10)
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("alerts count = %d, want 2", len(alerts))
	}

	found1, found2 := false, false
	for _, a := range alerts {
		if a.ID == "alert-1" {
			found1 = true
		}
		if a.ID == "alert-2" && a.Severity == "critical" {
			found2 = true
		}
	}
	if !found1 {
		t.Error("expected to find alert-1 in results")
	}
	if !found2 {
		t.Error("expected alert-2 with severity critical")
	}
}

func TestTrafficRepository_ListAlerts_DefaultLimit(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_, err := trafficRepo.ListAlerts(ctx, 0)
	if err != nil {
		t.Fatalf("ListAlerts with limit 0: %v", err)
	}

	_, err = trafficRepo.ListAlerts(ctx, 600)
	if err != nil {
		t.Fatalf("ListAlerts with limit 600: %v", err)
	}
}

func TestTrafficRepository_DeleteAlert(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_ = trafficRepo.InsertAlert(ctx, &models.Alert{
		ID: "del-alert-1", Type: "test", Message: "to delete",
		Severity: "info", Timestamp: time.Now(),
	})
	_ = trafficRepo.InsertAlert(ctx, &models.Alert{
		ID: "del-alert-2", Type: "test", Message: "to keep",
		Severity: "info", Timestamp: time.Now(),
	})

	if err := trafficRepo.DeleteAlert(ctx, "del-alert-1"); err != nil {
		t.Fatalf("DeleteAlert: %v", err)
	}

	alerts, _ := trafficRepo.ListAlerts(ctx, 100)
	for _, a := range alerts {
		if a.ID == "del-alert-1" {
			t.Error("alert del-alert-1 should be deleted")
		}
	}
	found := false
	for _, a := range alerts {
		if a.ID == "del-alert-2" {
			found = true
		}
	}
	if !found {
		t.Error("alert del-alert-2 should still exist")
	}
}

func TestTrafficRepository_DeleteAllAlerts(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_ = trafficRepo.InsertAlert(ctx, &models.Alert{ID: "aa-1", Type: "test", Message: "a", Severity: "info", Timestamp: time.Now()})
	_ = trafficRepo.InsertAlert(ctx, &models.Alert{ID: "aa-2", Type: "test", Message: "b", Severity: "info", Timestamp: time.Now()})
	_ = trafficRepo.InsertAlert(ctx, &models.Alert{ID: "aa-3", Type: "test", Message: "c", Severity: "info", Timestamp: time.Now()})

	if err := trafficRepo.DeleteAllAlerts(ctx); err != nil {
		t.Fatalf("DeleteAllAlerts: %v", err)
	}

	alerts, err := trafficRepo.ListAlerts(ctx, 100)
	if err != nil {
		t.Fatalf("ListAlerts: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("alerts count = %d, want 0", len(alerts))
	}
}

func TestTrafficRepository_CleanupOldAlerts(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_ = trafficRepo.DeleteAllAlerts(ctx)

	_, _ = db.Exec("INSERT INTO alerts (id, type, message, severity, timestamp) VALUES (?, ?, ?, ?, datetime('now', '-60 days'))",
		"old-alert", "test", "old", "info")
	_, _ = db.Exec("INSERT INTO alerts (id, type, message, severity, timestamp) VALUES (?, ?, ?, ?, datetime('now'))",
		"new-alert", "test", "new", "info")

	deleted, err := trafficRepo.CleanupOldAlerts(ctx, 30)
	if err != nil {
		t.Fatalf("CleanupOldAlerts: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	alerts, _ := trafficRepo.ListAlerts(ctx, 100)
	if len(alerts) != 1 {
		t.Fatalf("alerts count = %d, want 1", len(alerts))
	}
	if alerts[0].ID != "new-alert" {
		t.Errorf("expected new-alert, got %q", alerts[0].ID)
	}
}

func TestTrafficRepository_CleanupOldAlerts_DefaultRetain(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_, _ = db.Exec("INSERT INTO alerts (id, type, message, severity, timestamp) VALUES (?, ?, ?, ?, datetime('now'))",
		"recent", "test", "recent", "info")

	deleted, err := trafficRepo.CleanupOldAlerts(ctx, 0)
	if err != nil {
		t.Fatalf("CleanupOldAlerts with 0: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0", deleted)
	}
}

func TestTrafficRepository_CreateSession_CloseSession(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "cs-peer", Name: "CS", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "cspk", PrivateKey: "cspv",
		Address: "10.99.3.1", IsActive: true,
	})

	sid, err := trafficRepo.CreateSession(ctx, "cs-peer")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sid <= 0 {
		t.Errorf("session ID = %d, want > 0", sid)
	}

	if err := trafficRepo.CloseSession(ctx, sid, 500, 300, 7); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	sessions, err := trafficRepo.ListSessions(ctx, "cs-peer", 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("sessions count = %d, want 1", len(sessions))
	}
	s := sessions[0]
	if s.BytesRx != 500 {
		t.Errorf("BytesRx = %d, want 500", s.BytesRx)
	}
	if s.BytesTx != 300 {
		t.Errorf("BytesTx = %d, want 300", s.BytesTx)
	}
	if s.ConnectionsCount != 7 {
		t.Errorf("ConnectionsCount = %d, want 7", s.ConnectionsCount)
	}
	if s.DisconnectedAt == nil {
		t.Error("DisconnectedAt should not be nil after CloseSession")
	}
}

func TestTrafficRepository_GetActiveSession(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "ga-peer", Name: "GA", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "gapk", PrivateKey: "gapv",
		Address: "10.99.4.1", IsActive: true,
	})

	active, err := trafficRepo.GetActiveSession(ctx, "ga-peer")
	if err != nil {
		t.Fatalf("GetActiveSession no session: %v", err)
	}
	if active != nil {
		t.Error("expected nil when no sessions exist")
	}

	sid, _ := trafficRepo.CreateSession(ctx, "ga-peer")

	active, err = trafficRepo.GetActiveSession(ctx, "ga-peer")
	if err != nil {
		t.Fatalf("GetActiveSession: %v", err)
	}
	if active == nil {
		t.Fatal("expected active session")
	}
	if active.ID != sid {
		t.Errorf("ID = %d, want %d", active.ID, sid)
	}
	if active.PeerID != "ga-peer" {
		t.Errorf("PeerID = %q, want ga-peer", active.PeerID)
	}

	_ = trafficRepo.CloseSession(ctx, sid, 100, 50, 1)

	active, err = trafficRepo.GetActiveSession(ctx, "ga-peer")
	if err != nil {
		t.Fatalf("GetActiveSession after close: %v", err)
	}
	if active != nil {
		t.Error("expected nil after session closed")
	}
}

func TestTrafficRepository_ListSessions(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "ls-peer", Name: "LS", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "lspk", PrivateKey: "lspv",
		Address: "10.99.5.1", IsActive: true,
	})

	sid1, _ := trafficRepo.CreateSession(ctx, "ls-peer")
	_ = trafficRepo.CloseSession(ctx, sid1, 100, 200, 2)

	sid2, _ := trafficRepo.CreateSession(ctx, "ls-peer")
	_ = trafficRepo.CloseSession(ctx, sid2, 300, 400, 5)

	sid3, _ := trafficRepo.CreateSession(ctx, "ls-peer")

	sessions, err := trafficRepo.ListSessions(ctx, "ls-peer", 10)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("sessions count = %d, want 3", len(sessions))
	}

	if sessions[0].ID != sid3 {
		t.Errorf("first session ID = %d, want %d (most recent)", sessions[0].ID, sid3)
	}

	if sessions[0].DisconnectedAt != nil {
		t.Error("most recent session should still be active (nil DisconnectedAt)")
	}
	if sessions[1].DisconnectedAt == nil {
		t.Error("second session should be closed")
	}
}

func TestTrafficRepository_ListSessions_DefaultLimit(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "lsl-peer", Name: "LSL", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "lslpk", PrivateKey: "lslpv",
		Address: "10.99.5.2", IsActive: true,
	})
	_, _ = trafficRepo.CreateSession(ctx, "lsl-peer")

	sessions, err := trafficRepo.ListSessions(ctx, "lsl-peer", 0)
	if err != nil {
		t.Fatalf("ListSessions with limit 0: %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("sessions count = %d, want 1", len(sessions))
	}
}

func TestTrafficRepository_GetPeerTrafficSummary(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "pts-p1", Name: "PTS1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "ptspk1", PrivateKey: "ptspv1",
		Address: "10.99.6.1", IsActive: true,
	})
	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "pts-p2", Name: "PTS2", DeviceType: models.DeviceTypeAndroid,
		PublicKey: "ptspk2", PrivateKey: "ptspv2",
		Address: "10.99.6.2", IsActive: true,
	})

	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "pts-p1", Domain: "google.com", Action: "proxy", BytesRx: 1000, BytesTx: 500})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "pts-p1", Domain: "vk.com", Action: "direct", BytesRx: 2000, BytesTx: 1000})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "pts-p2", Domain: "youtube.com", Action: "proxy", BytesRx: 5000, BytesTx: 3000})
	_ = peerRepo.UpdateTraffic(ctx, "pts-p1", 3000, 1500)
	_ = peerRepo.UpdateTraffic(ctx, "pts-p2", 5000, 3000)

	summaries, err := trafficRepo.GetPeerTrafficSummary(ctx)
	if err != nil {
		t.Fatalf("GetPeerTrafficSummary: %v", err)
	}

	var s1, s2 *models.PeerTrafficSummary
	for _, s := range summaries {
		if s.PeerID == "pts-p1" {
			s1 = s
		}
		if s.PeerID == "pts-p2" {
			s2 = s
		}
	}
	if s1 == nil || s2 == nil {
		t.Fatal("expected to find both pts-p1 and pts-p2 in summaries")
	}
	if s1.TotalRx != 3000 || s1.TotalTx != 1500 {
		t.Errorf("pts-p1: Rx=%d Tx=%d, want 3000,1500", s1.TotalRx, s1.TotalTx)
	}
	if s1.ConnCount != 2 {
		t.Errorf("pts-p1 ConnCount = %d, want 2", s1.ConnCount)
	}
	if s1.TopDomain != "vk.com" {
		t.Errorf("pts-p1 TopDomain = %q, want vk.com", s1.TopDomain)
	}
	if s2.TotalRx != 5000 || s2.TotalTx != 3000 {
		t.Errorf("pts-p2: Rx=%d Tx=%d, want 5000,3000", s2.TotalRx, s2.TotalTx)
	}
	if s2.ConnCount != 1 {
		t.Errorf("pts-p2 ConnCount = %d, want 1", s2.ConnCount)
	}
	if s2.TopDomain != "youtube.com" {
		t.Errorf("pts-p2 TopDomain = %q, want youtube.com", s2.TopDomain)
	}
}

func TestTrafficRepository_GetPeerTrafficSummary_NoTraffic(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "empty-peer", Name: "Empty", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "empk", PrivateKey: "empv",
		Address: "10.99.6.3", IsActive: true,
	})

	summaries, err := trafficRepo.GetPeerTrafficSummary(ctx)
	if err != nil {
		t.Fatalf("GetPeerTrafficSummary: %v", err)
	}
	var found *models.PeerTrafficSummary
	for _, s := range summaries {
		if s.PeerID == "empty-peer" {
			found = s
		}
	}
	if found == nil {
		t.Fatal("expected to find empty-peer in summaries")
	}
	if found.TotalRx != 0 || found.TotalTx != 0 {
		t.Errorf("expected zero traffic, got Rx=%d Tx=%d", found.TotalRx, found.TotalTx)
	}
	if found.ConnCount != 0 {
		t.Errorf("ConnCount = %d, want 0", found.ConnCount)
	}
}

func TestTrafficRepository_GetTrafficAggregate(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "agg-p0", Name: "A0", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "aggpk0", PrivateKey: "aggpv0",
		Address: "10.99.7.0", IsActive: true,
	})

	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "agg-p0", Domain: "google.com", Action: "proxy", BytesRx: 1000, BytesTx: 500})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "agg-p0", Domain: "google.com", Action: "proxy", BytesRx: 2000, BytesTx: 1000})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "agg-p0", Domain: "vk.com", Action: "direct", BytesRx: 500, BytesTx: 200})

	items, err := trafficRepo.GetTrafficAggregate(ctx, "agg-p0", 10)
	if err != nil {
		t.Fatalf("GetTrafficAggregate: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items count = %d, want 2", len(items))
	}

	if items[0].Domain != "google.com" {
		t.Errorf("first domain = %q, want google.com", items[0].Domain)
	}
	if items[0].RX != 3000 || items[0].TX != 1500 {
		t.Errorf("google.com: RX=%d TX=%d, want 3000,1500", items[0].RX, items[0].TX)
	}
	if items[0].Count != 2 {
		t.Errorf("google.com Count = %d, want 2", items[0].Count)
	}

	if items[1].Domain != "vk.com" {
		t.Errorf("second domain = %q, want vk.com", items[1].Domain)
	}
	if items[1].Count != 1 {
		t.Errorf("vk.com Count = %d, want 1", items[1].Count)
	}
}

func TestTrafficRepository_GetTrafficAggregate_ByPeer(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "agg-p1", Name: "A1", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "aggpk1", PrivateKey: "aggpv1",
		Address: "10.99.7.1", IsActive: true,
	})
	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "agg-p2", Name: "A2", DeviceType: models.DeviceTypeAndroid,
		PublicKey: "aggpk2", PrivateKey: "aggpv2",
		Address: "10.99.7.2", IsActive: true,
	})

	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "agg-p1", Domain: "google.com", Action: "proxy", BytesRx: 100, BytesTx: 50})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "agg-p2", Domain: "youtube.com", Action: "proxy", BytesRx: 999, BytesTx: 999})

	items, err := trafficRepo.GetTrafficAggregate(ctx, "agg-p1", 10)
	if err != nil {
		t.Fatalf("GetTrafficAggregate: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	if items[0].Domain != "google.com" {
		t.Errorf("domain = %q, want google.com", items[0].Domain)
	}
}

func TestTrafficRepository_GetTrafficAggregate_DefaultLimit(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	_, err := trafficRepo.GetTrafficAggregate(ctx, "", 0)
	if err != nil {
		t.Fatalf("GetTrafficAggregate with limit 0: %v", err)
	}

	_, err = trafficRepo.GetTrafficAggregate(ctx, "", 600)
	if err != nil {
		t.Fatalf("GetTrafficAggregate with limit 600: %v", err)
	}
}

func TestTrafficRepository_GetTrafficAggregate_FallbackToIP(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "fb-peer", Name: "FB", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "fbpk", PrivateKey: "fbpv",
		Address: "10.99.7.3", IsActive: true,
	})

	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "fb-peer", Domain: "", DestIP: "1.2.3.4", Action: "direct", BytesRx: 100, BytesTx: 50})

	items, err := trafficRepo.GetTrafficAggregate(ctx, "fb-peer", 10)
	if err != nil {
		t.Fatalf("GetTrafficAggregate: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items count = %d, want 1", len(items))
	}
	if items[0].Domain != "1.2.3.4" {
		t.Errorf("domain = %q, want 1.2.3.4 (IP fallback)", items[0].Domain)
	}
}

func TestTrafficRepository_DeleteByPeerID(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "del-peer", Name: "DP", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "dpk1", PrivateKey: "dpv1",
		Address: "10.99.0.10", IsActive: true,
	})

	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "del-peer", Domain: "test.com", Action: "direct", BytesRx: 50, BytesTx: 30})
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "del-peer", Domain: "other.com", Action: "proxy", BytesRx: 70, BytesTx: 40})

	if err := trafficRepo.DeleteByPeerID(ctx, "del-peer"); err != nil {
		t.Fatalf("DeleteByPeerID: %v", err)
	}

	logs, err := trafficRepo.List(ctx, models.TrafficFilter{PeerID: "del-peer", Limit: 100})
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("expected 0 logs after delete, got %d", len(logs))
	}
}

func TestAuthRepository_DeleteUserRefreshTokens(t *testing.T) {
	db := initTestDB(t)
	authRepo := NewAuthRepository(db)
	ctx := context.Background()

	userID := "user-del-tokens"

	_, _ = db.ExecContext(ctx, "INSERT INTO admin_users (id, email, password_hash) VALUES (?, ?, ?)",
		userID, "del@test.com", "hash")
	_, _ = db.ExecContext(ctx, "INSERT INTO admin_users (id, email, password_hash) VALUES (?, ?, ?)",
		"other-user", "other@test.com", "hash")

	token1 := "token-del-1"
	token2 := "token-del-2"
	token3 := "token-del-3"

	if err := authRepo.StoreRefreshToken(ctx, userID, token1, time.Now().Add(1*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("StoreRefreshToken token1: %v", err)
	}
	if err := authRepo.StoreRefreshToken(ctx, userID, token2, time.Now().Add(2*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("StoreRefreshToken token2: %v", err)
	}
	if err := authRepo.StoreRefreshToken(ctx, "other-user", token3, time.Now().Add(1*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatalf("StoreRefreshToken token3: %v", err)
	}

	if err := authRepo.DeleteUserRefreshTokens(ctx, userID); err != nil {
		t.Fatalf("DeleteUserRefreshTokens: %v", err)
	}

	_, err1 := authRepo.GetRefreshToken(ctx, token1)
	_, err2 := authRepo.GetRefreshToken(ctx, token2)
	_, err3 := authRepo.GetRefreshToken(ctx, token3)

	if err1 == nil || err2 == nil {
		t.Error("expected error for deleted tokens")
	}
	if err3 != nil {
		t.Errorf("expected other user token to exist: %v", err3)
	}
}

func TestTrafficRepository_GetPeerTrafficSummary_UsesWgPeersTotals(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "wgts-p1", Name: "WGTS", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "wgtsk1", PrivateKey: "wgtsv1",
		Address: "10.99.8.1", IsActive: true,
	})

	_ = peerRepo.UpdateTraffic(ctx, "wgts-p1", 9999, 8888)

	summaries, err := trafficRepo.GetPeerTrafficSummary(ctx)
	if err != nil {
		t.Fatalf("GetPeerTrafficSummary: %v", err)
	}
	var found *models.PeerTrafficSummary
	for _, s := range summaries {
		if s.PeerID == "wgts-p1" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find wgts-p1 in summaries")
	}
	if found.TotalRx != 9999 {
		t.Errorf("TotalRx = %d, want 9999 (from wg_peers)", found.TotalRx)
	}
	if found.TotalTx != 8888 {
		t.Errorf("TotalTx = %d, want 8888 (from wg_peers)", found.TotalTx)
	}
}

func TestTrafficRepository_GetPeerTrafficSummary_WgPeersOverTrafficLogs(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	_ = peerRepo.Create(ctx, &models.Peer{
		ID: "wgot-p1", Name: "WGOT", DeviceType: models.DeviceTypeIPhone,
		PublicKey: "wgotk1", PrivateKey: "wgotv1",
		Address: "10.99.9.1", IsActive: true,
	})

	_ = peerRepo.UpdateTraffic(ctx, "wgot-p1", 5000, 3000)
	_ = trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "wgot-p1", Domain: "a.com", Action: "test", BytesRx: 100, BytesTx: 50})

	summaries, err := trafficRepo.GetPeerTrafficSummary(ctx)
	if err != nil {
		t.Fatalf("GetPeerTrafficSummary: %v", err)
	}
	var found *models.PeerTrafficSummary
	for _, s := range summaries {
		if s.PeerID == "wgot-p1" {
			found = s
			break
		}
	}
	if found == nil {
		t.Fatal("expected to find wgot-p1")
	}
	if found.TotalRx != 5000 {
		t.Errorf("TotalRx = %d, want 5000 (wg_peers value, not 100 from traffic_logs)", found.TotalRx)
	}
	if found.TotalTx != 3000 {
		t.Errorf("TotalTx = %d, want 3000 (wg_peers value, not 50 from traffic_logs)", found.TotalTx)
	}
	if found.ConnCount != 1 {
		t.Errorf("ConnCount = %d, want 1 (from traffic_logs)", found.ConnCount)
	}
	if found.TopDomain != "a.com" {
		t.Errorf("TopDomain = %q, want a.com (from traffic_logs)", found.TopDomain)
	}
}

func TestIsNotFound(t *testing.T) {
	if !IsNotFound(ErrNotFound) {
		t.Error("IsNotFound should return true for ErrNotFound")
	}
	if IsNotFound(errors.New("other error")) {
		t.Error("IsNotFound should return false for other errors")
	}
	if IsNotFound(nil) {
		t.Error("IsNotFound should return false for nil")
	}
}
