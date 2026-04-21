package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"smarttraffic/internal/config"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"

	wgcrypto "smarttraffic/pkg/wgcrypto"

	"github.com/google/uuid"
)

type WireGuardService struct {
	peerRepo repository.PeerRepository
	cfg      *config.WGConfig
	logger   *slog.Logger
}

func NewWireGuardService(peerRepo repository.PeerRepository, cfg *config.WGConfig, logger *slog.Logger) *WireGuardService {
	return &WireGuardService{
		peerRepo: peerRepo,
		cfg:      cfg,
		logger:   logger,
	}
}

func (s *WireGuardService) CreatePeer(ctx context.Context, req *models.PeerCreateRequest) (*models.Peer, error) {
	if errs := req.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("service.wireguard.CreatePeer: невалидные данные: %v", errs)
	}

	privateKey, publicKey, err := wgcrypto.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.CreatePeer generate keys: %w", err)
	}

	address, err := s.allocateAddress(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.CreatePeer allocate address: %w", err)
	}

	dns := req.DNS
	if dns == "" {
		dns = s.cfg.DNS
	}

	mtu := req.MTU
	if mtu == 0 {
		mtu = s.cfg.MTU
	}

	peer := &models.Peer{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Email:      req.Email,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Address:    address,
		DNS:        dns,
		MTU:        mtu,
		IsActive:   true,
	}

	if err := s.peerRepo.Create(ctx, peer); err != nil {
		return nil, fmt.Errorf("service.wireguard.CreatePeer save: %w", err)
	}

	s.logger.Info("создан WG клиент", "id", peer.ID, "name", peer.Name, "address", peer.Address)
	return peer, nil
}

func (s *WireGuardService) GetPeer(ctx context.Context, id string) (*models.Peer, error) {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.GetPeer: %w", err)
	}
	return peer, nil
}

func (s *WireGuardService) ListPeers(ctx context.Context) ([]*models.Peer, error) {
	peers, err := s.peerRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.ListPeers: %w", err)
	}
	return peers, nil
}

func (s *WireGuardService) DeletePeer(ctx context.Context, id string) error {
	if err := s.peerRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("service.wireguard.DeletePeer: %w", err)
	}
	s.logger.Info("удалён WG клиент", "id", id)
	return nil
}

func (s *WireGuardService) TogglePeer(ctx context.Context, id string, active bool) error {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("service.wireguard.TogglePeer: %w", err)
	}
	peer.IsActive = active
	if err := s.peerRepo.Update(ctx, peer); err != nil {
		return fmt.Errorf("service.wireguard.TogglePeer update: %w", err)
	}
	s.logger.Info("изменён статус WG клиента", "id", id, "active", active)
	return nil
}

func (s *WireGuardService) GenerateClientConfig(peer *models.Peer) string {
	var sb strings.Builder
	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s/24\n", peer.Address))
	sb.WriteString(fmt.Sprintf("DNS = %s\n", peer.DNS))
	sb.WriteString(fmt.Sprintf("MTU = %d\n", peer.MTU))
	sb.WriteString("\n[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", s.cfg.ServerPubKey))
	sb.WriteString(fmt.Sprintf("Endpoint = %s:%d\n", s.cfg.ServerEndpoint, s.cfg.Port))
	sb.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	sb.WriteString("PersistentKeepalive = 25\n")
	return sb.String()
}

func (s *WireGuardService) GetPeerStats(ctx context.Context, id string) (*models.PeerStats, error) {
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service.wireguard.GetPeerStats: %w", err)
	}
	return &models.PeerStats{
		PeerID:  peer.ID,
		TotalRx: peer.TotalRx,
		TotalTx: peer.TotalTx,
		Online:  peer.IsActive,
	}, nil
}

func (s *WireGuardService) allocateAddress(ctx context.Context) (string, error) {
	peers, err := s.peerRepo.List(ctx)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return "", err
	}

	subnet := strings.TrimSuffix(s.cfg.ClientSubnet, ".0/24")

	used := make(map[string]bool)
	for _, p := range peers {
		parts := strings.Split(p.Address, ".")
		if len(parts) == 4 {
			used[parts[3]] = true
		}
	}

	for i := 2; i <= 254; i++ {
		host := fmt.Sprintf("%d", i)
		if !used[host] {
			return fmt.Sprintf("%s.%s", subnet, host), nil
		}
	}

	return "", fmt.Errorf("нет свободных адресов в пуле %s", s.cfg.ClientSubnet)
}
