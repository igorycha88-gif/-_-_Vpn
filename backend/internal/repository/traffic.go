package repository

import (
	"context"
	"database/sql"
	"fmt"

	"smarttraffic/internal/models"
)

type TrafficRepository interface {
	Log(ctx context.Context, log *models.TrafficLog) error
	List(ctx context.Context, filter models.TrafficFilter) ([]*models.TrafficLog, error)
	GetTotalStats(ctx context.Context) (*models.TotalStats, error)
	GetPeerStats(ctx context.Context, peerID string) (*models.PeerStats, error)
	CleanupOld(ctx context.Context, retainDays int) (int64, error)
}

type sqliteTrafficRepository struct {
	db *sql.DB
}

func NewTrafficRepository(db *sql.DB) TrafficRepository {
	return &sqliteTrafficRepository{db: db}
}

func (r *sqliteTrafficRepository) Log(ctx context.Context, log *models.TrafficLog) error {
	q := `INSERT INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		log.PeerID, log.Domain, log.DestIP, log.DestPort,
		log.Action, log.BytesRx, log.BytesTx,
	)
	if err != nil {
		return fmt.Errorf("traffic.Log: %w", err)
	}
	return nil
}

func (r *sqliteTrafficRepository) List(ctx context.Context, filter models.TrafficFilter) ([]*models.TrafficLog, error) {
	q := `SELECT id, peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp
	      FROM traffic_logs WHERE 1=1`
	args := []interface{}{}

	if filter.PeerID != "" {
		q += " AND peer_id = ?"
		args = append(args, filter.PeerID)
	}
	if filter.StartTime != nil {
		q += " AND timestamp >= ?"
		args = append(args, filter.StartTime)
	}
	if filter.EndTime != nil {
		q += " AND timestamp <= ?"
		args = append(args, filter.EndTime)
	}

	q += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		q += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("traffic.List: %w", err)
	}
	defer rows.Close()

	var logs []*models.TrafficLog
	for rows.Next() {
		l := &models.TrafficLog{}
		err := rows.Scan(
			&l.ID, &l.PeerID, &l.Domain, &l.DestIP, &l.DestPort,
			&l.Action, &l.BytesRx, &l.BytesTx, &l.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("traffic.List scan: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func (r *sqliteTrafficRepository) GetTotalStats(ctx context.Context) (*models.TotalStats, error) {
	stats := &models.TotalStats{}

	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(total_rx),0), COALESCE(SUM(total_tx),0) FROM wg_peers").Scan(&stats.TotalRx, &stats.TotalTx)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetTotalStats: %w", err)
	}

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wg_peers").Scan(&stats.TotalPeers)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetTotalStats peers: %w", err)
	}

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wg_peers WHERE is_active=TRUE").Scan(&stats.ActivePeers)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetTotalStats active: %w", err)
	}

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM routing_rules").Scan(&stats.RulesCount)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetTotalStats rules: %w", err)
	}

	return stats, nil
}

func (r *sqliteTrafficRepository) GetPeerStats(ctx context.Context, peerID string) (*models.PeerStats, error) {
	stats := &models.PeerStats{PeerID: peerID}
	q := `SELECT total_rx, total_tx, is_active FROM wg_peers WHERE id=?`
	err := r.db.QueryRowContext(ctx, q, peerID).Scan(&stats.TotalRx, &stats.TotalTx, &stats.Online)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("traffic.GetPeerStats: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("traffic.GetPeerStats: %w", err)
	}
	return stats, nil
}

func (r *sqliteTrafficRepository) CleanupOld(ctx context.Context, retainDays int) (int64, error) {
	q := `DELETE FROM traffic_logs WHERE timestamp < datetime('now', printf('-%d days', ?))`
	result, err := r.db.ExecContext(ctx, q, retainDays)
	if err != nil {
		return 0, fmt.Errorf("traffic.CleanupOld: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}
