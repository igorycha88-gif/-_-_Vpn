package repository

import (
	"context"
	"database/sql"
	"fmt"

	"smarttraffic/internal/models"
)

type PeerRepository interface {
	Create(ctx context.Context, p *models.Peer) error
	GetByID(ctx context.Context, id string) (*models.Peer, error)
	List(ctx context.Context) ([]*models.Peer, error)
	Update(ctx context.Context, p *models.Peer) error
	Delete(ctx context.Context, id string) error
	GetByPublicKey(ctx context.Context, publicKey string) (*models.Peer, error)
	UpdateTraffic(ctx context.Context, id string, rx, tx int64) error
	UpdateLastSeen(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
	CountActive(ctx context.Context) (int, error)
}

type sqlitePeerRepository struct {
	db *sql.DB
}

func NewPeerRepository(db *sql.DB) PeerRepository {
	return &sqlitePeerRepository{db: db}
}

func (r *sqlitePeerRepository) Create(ctx context.Context, p *models.Peer) error {
	q := `INSERT INTO wg_peers (id, name, email, public_key, private_key, address, dns, mtu, is_active)
	      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		p.ID, p.Name, p.Email, p.PublicKey, p.PrivateKey,
		p.Address, p.DNS, p.MTU, p.IsActive,
	)
	if err != nil {
		return fmt.Errorf("peers.Create: %w", err)
	}
	return nil
}

func (r *sqlitePeerRepository) GetByID(ctx context.Context, id string) (*models.Peer, error) {
	q := `SELECT id, name, email, public_key, private_key, address, dns, mtu,
	             is_active, created_at, updated_at, total_rx, total_tx, last_seen
	      FROM wg_peers WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)

	p := &models.Peer{}
	var lastSeen sql.NullTime

	err := row.Scan(
		&p.ID, &p.Name, &p.Email, &p.PublicKey, &p.PrivateKey,
		&p.Address, &p.DNS, &p.MTU, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt, &p.TotalRx, &p.TotalTx, &lastSeen,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("peers.GetByID: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("peers.GetByID: %w", err)
	}

	if lastSeen.Valid {
		p.LastSeen = &lastSeen.Time
	}
	return p, nil
}

func (r *sqlitePeerRepository) List(ctx context.Context) ([]*models.Peer, error) {
	q := `SELECT id, name, email, public_key, '', address, dns, mtu,
	             is_active, created_at, updated_at, total_rx, total_tx, last_seen
	      FROM wg_peers ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("peers.List: %w", err)
	}
	defer rows.Close()

	var peers []*models.Peer
	for rows.Next() {
		p := &models.Peer{}
		var lastSeen sql.NullTime
		err := rows.Scan(
			&p.ID, &p.Name, &p.Email, &p.PublicKey, &p.PrivateKey,
			&p.Address, &p.DNS, &p.MTU, &p.IsActive,
			&p.CreatedAt, &p.UpdatedAt, &p.TotalRx, &p.TotalTx, &lastSeen,
		)
		if err != nil {
			return nil, fmt.Errorf("peers.List scan: %w", err)
		}
		if lastSeen.Valid {
			p.LastSeen = &lastSeen.Time
		}
		peers = append(peers, p)
	}
	return peers, rows.Err()
}

func (r *sqlitePeerRepository) Update(ctx context.Context, p *models.Peer) error {
	q := `UPDATE wg_peers SET name=?, email=?, dns=?, mtu=?, is_active=?, updated_at=CURRENT_TIMESTAMP
	      WHERE id=?`
	result, err := r.db.ExecContext(ctx, q, p.Name, p.Email, p.DNS, p.MTU, p.IsActive, p.ID)
	if err != nil {
		return fmt.Errorf("peers.Update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("peers.Update: %w", ErrNotFound)
	}
	return nil
}

func (r *sqlitePeerRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM wg_peers WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("peers.Delete: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("peers.Delete: %w", ErrNotFound)
	}
	return nil
}

func (r *sqlitePeerRepository) GetByPublicKey(ctx context.Context, publicKey string) (*models.Peer, error) {
	q := `SELECT id, name, email, public_key, private_key, address, dns, mtu,
	             is_active, created_at, updated_at, total_rx, total_tx, last_seen
	      FROM wg_peers WHERE public_key = ?`
	row := r.db.QueryRowContext(ctx, q, publicKey)

	p := &models.Peer{}
	var lastSeen sql.NullTime

	err := row.Scan(
		&p.ID, &p.Name, &p.Email, &p.PublicKey, &p.PrivateKey,
		&p.Address, &p.DNS, &p.MTU, &p.IsActive,
		&p.CreatedAt, &p.UpdatedAt, &p.TotalRx, &p.TotalTx, &lastSeen,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("peers.GetByPublicKey: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("peers.GetByPublicKey: %w", err)
	}

	if lastSeen.Valid {
		p.LastSeen = &lastSeen.Time
	}
	return p, nil
}

func (r *sqlitePeerRepository) UpdateTraffic(ctx context.Context, id string, rx, tx int64) error {
	q := `UPDATE wg_peers SET total_rx=total_rx+?, total_tx=total_tx+?, updated_at=CURRENT_TIMESTAMP
	      WHERE id=?`
	_, err := r.db.ExecContext(ctx, q, rx, tx, id)
	if err != nil {
		return fmt.Errorf("peers.UpdateTraffic: %w", err)
	}
	return nil
}

func (r *sqlitePeerRepository) UpdateLastSeen(ctx context.Context, id string) error {
	q := `UPDATE wg_peers SET last_seen=CURRENT_TIMESTAMP WHERE id=?`
	_, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("peers.UpdateLastSeen: %w", err)
	}
	return nil
}

func (r *sqlitePeerRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wg_peers").Scan(&count)
	return count, err
}

func (r *sqlitePeerRepository) CountActive(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wg_peers WHERE is_active=TRUE").Scan(&count)
	return count, err
}
