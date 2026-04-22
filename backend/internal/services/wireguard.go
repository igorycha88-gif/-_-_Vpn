package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
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

	if err := s.addPeerToInterface(peer.PublicKey, peer.Address); err != nil {
		s.logger.Error("не удалось добавить peer в WG интерфейс", "error", err)
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
	peer, err := s.peerRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("service.wireguard.DeletePeer: %w", err)
	}

	if err := s.removePeerFromInterface(peer.PublicKey); err != nil {
		s.logger.Error("не удалось удалить peer из WG интерфейса", "error", err)
	}

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

	if active {
		if err := s.addPeerToInterface(peer.PublicKey, peer.Address); err != nil {
			s.logger.Error("не удалось добавить peer в WG интерфейс", "error", err)
		}
	} else {
		if err := s.removePeerFromInterface(peer.PublicKey); err != nil {
			s.logger.Error("не удалось удалить peer из WG интерфейса", "error", err)
		}
	}

	s.logger.Info("изменён статус WG клиента", "id", id, "active", active)
	return nil
}

func (s *WireGuardService) GenerateClientConfig(peer *models.Peer) string {
	var sb strings.Builder
	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))

	subnet := strings.TrimSuffix(s.cfg.ClientSubnet, ".0/24")
	sb.WriteString(fmt.Sprintf("Address = %s/24\n", peer.Address))

	sb.WriteString(fmt.Sprintf("DNS = %s.1\n", subnet))
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

func (s *WireGuardService) SyncAllPeers(ctx context.Context) error {
	peers, err := s.peerRepo.List(ctx)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("service.wireguard.SyncAllPeers list: %w", err)
	}

	for _, peer := range peers {
		if peer.IsActive {
			if err := s.addPeerToInterface(peer.PublicKey, peer.Address); err != nil {
				s.logger.Error("sync: не удалось добавить peer", "id", peer.ID, "error", err)
			} else {
				s.logger.Info("sync: peer добавлен", "name", peer.Name, "address", peer.Address)
			}
		}
	}

	s.logger.Info("синхронизация пиров завершена", "total", len(peers))
	return nil
}

func (s *WireGuardService) addPeerToInterface(publicKey, address string) error {
	iface := s.cfg.Interface
	args := []string{"set", iface, "peer", publicKey, "allowed-ips", address + "/32"}

	cmd := exec.Command("wg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg set %s peer %s… allowed-ips %s/32: %w: %s", iface, publicKey[:8], address, err, string(output))
	}
	return nil
}

func (s *WireGuardService) removePeerFromInterface(publicKey string) error {
	iface := s.cfg.Interface
	args := []string{"set", iface, "peer", publicKey, "remove"}

	cmd := exec.Command("wg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg set %s peer %s… remove: %w: %s", iface, publicKey[:8], err, string(output))
	}
	return nil
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
