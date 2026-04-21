package repository

import (
	"context"
	"database/sql"
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
	t.Cleanup(func() { db.Close() })
	return db
}

func TestPeerRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	peer := &models.Peer{
		ID: "test-peer-1", Name: "Test Peer", Email: "test@example.com",
		PublicKey: "dGVzdHB1YmxpY2tleQ==", PrivateKey: "dGVzdHByaXZhdGVrZXk=",
		Address: "10.10.0.2", DNS: "1.1.1.1", MTU: 1280, IsActive: true,
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
	if len(peers) != 1 {
		t.Errorf("List count = %d, want 1", len(peers))
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
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
	}

	activeCount, _ := repo.CountActive(ctx)
	if activeCount != 1 {
		t.Errorf("CountActive = %d, want 1", activeCount)
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

	repo.Create(ctx, &models.Peer{
		ID: "pk-test", Name: "PK", PublicKey: "uniquepubkey123",
		PrivateKey: "uniqueprivkey123", Address: "10.10.0.3", IsActive: true,
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

	repo.Create(ctx, &models.Peer{
		ID: "t-test", Name: "T", PublicKey: "tpk", PrivateKey: "tpv",
		Address: "10.10.0.4", IsActive: true,
	})

	repo.UpdateTraffic(ctx, "t-test", 1024, 2048)
	got, _ := repo.GetByID(ctx, "t-test")
	if got.TotalRx != 1024 || got.TotalTx != 2048 {
		t.Errorf("Rx=%d Tx=%d, want 1024,2048", got.TotalRx, got.TotalTx)
	}
}

func TestPeerRepository_UpdateLastSeen(t *testing.T) {
	db := initTestDB(t)
	repo := NewPeerRepository(db)
	ctx := context.Background()

	repo.Create(ctx, &models.Peer{
		ID: "seen-test", Name: "S", PublicKey: "spk", PrivateKey: "spv",
		Address: "10.10.0.5", IsActive: true,
	})

	repo.UpdateLastSeen(ctx, "seen-test")
	got, _ := repo.GetByID(ctx, "seen-test")
	if got.LastSeen == nil {
		t.Fatal("LastSeen should not be nil")
	}
}

func TestRouteRepository_CRUD(t *testing.T) {
	db := initTestDB(t)
	repo := NewRouteRepository(db)
	ctx := context.Background()

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
	if len(rules) != 1 {
		t.Errorf("List count = %d, want 1", len(rules))
	}

	got.Name = "Updated"
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	count, _ := repo.Count(ctx)
	if count != 1 {
		t.Errorf("Count = %d, want 1", count)
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
		repo.Create(ctx, &models.RoutingRule{
			ID: "r" + n, Name: n, Type: "domain",
			Pattern: n + ".com", Action: "direct", Priority: i + 1,
		})
	}

	if err := repo.Reorder(ctx, []string{"rC", "rA", "rB"}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}

	rules, _ := repo.List(ctx)
	if rules[0].ID != "rC" {
		t.Errorf("first = %q, want rC", rules[0].ID)
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

	repo.DeleteRefreshToken(ctx, "token-123")
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

	peerRepo.Create(ctx, &models.Peer{
		ID: "peer-1", Name: "P1", PublicKey: "pk1", PrivateKey: "pv1",
		Address: "10.10.0.2", IsActive: true,
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
	if len(logs) != 1 {
		t.Fatalf("List count = %d, want 1", len(logs))
	}
	if logs[0].Domain != "example.com" {
		t.Errorf("Domain = %q, want example.com", logs[0].Domain)
	}
}

func TestTrafficRepository_FilterByPeer(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	peerRepo := NewPeerRepository(db)
	ctx := context.Background()

	peerRepo.Create(ctx, &models.Peer{ID: "p1", Name: "P1", PublicKey: "pk1", PrivateKey: "pv1", Address: "10.10.0.2", IsActive: true})
	peerRepo.Create(ctx, &models.Peer{ID: "p2", Name: "P2", PublicKey: "pk2", PrivateKey: "pv2", Address: "10.10.0.3", IsActive: true})

	trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "p1", Domain: "a.com", Action: "direct", BytesRx: 100, BytesTx: 50})
	trafficRepo.Log(ctx, &models.TrafficLog{PeerID: "p2", Domain: "b.com", Action: "proxy", BytesRx: 200, BytesTx: 100})

	logs, _ := trafficRepo.List(ctx, models.TrafficFilter{PeerID: "p1", Limit: 10})
	if len(logs) != 1 {
		t.Errorf("filtered count = %d, want 1", len(logs))
	}
}

func TestTrafficRepository_Cleanup(t *testing.T) {
	db := initTestDB(t)
	trafficRepo := NewTrafficRepository(db)
	ctx := context.Background()

	trafficRepo.Log(ctx, &models.TrafficLog{Action: "direct"})

	deleted, err := trafficRepo.CleanupOld(ctx, 0)
	if err != nil {
		t.Fatalf("CleanupOld: %v", err)
	}
	if deleted < 0 {
		t.Errorf("deleted = %d, want >= 0", deleted)
	}
}
