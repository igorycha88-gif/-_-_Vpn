package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"smarttraffic/internal/models"
)

type TrafficRepository interface {
	Log(ctx context.Context, log *models.TrafficLog) error
	List(ctx context.Context, filter models.TrafficFilter) ([]*models.TrafficLog, error)
	GetTotalStats(ctx context.Context) (*models.TotalStats, error)
	GetPeerStats(ctx context.Context, peerID string) (*models.PeerStats, error)
	CleanupOld(ctx context.Context, retainDays int) (int64, error)
	InsertAlert(ctx context.Context, alert *models.Alert) error
	ListAlerts(ctx context.Context, limit int) ([]*models.Alert, error)
	GetPeerTrafficSummary(ctx context.Context) ([]*models.PeerTrafficSummary, error)
	DeleteByPeerID(ctx context.Context, peerID string) error
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

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM wg_peers WHERE last_seen IS NOT NULL AND last_seen >= datetime('now', '-2 minutes')").Scan(&stats.OnlinePeers)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetTotalStats online: %w", err)
	}

	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM routing_rules").Scan(&stats.RulesCount)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetTotalStats rules: %w", err)
	}

	return stats, nil
}

func (r *sqliteTrafficRepository) GetPeerStats(ctx context.Context, peerID string) (*models.PeerStats, error) {
	stats := &models.PeerStats{PeerID: peerID}
	var lastSeen sql.NullTime
	q := `SELECT total_rx, total_tx, is_active, last_seen FROM wg_peers WHERE id=?`
	err := r.db.QueryRowContext(ctx, q, peerID).Scan(&stats.TotalRx, &stats.TotalTx, new(bool), &lastSeen)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("traffic.GetPeerStats: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("traffic.GetPeerStats: %w", err)
	}

	if lastSeen.Valid && time.Since(lastSeen.Time) < 2*time.Minute {
		stats.Online = true
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

func (r *sqliteTrafficRepository) InsertAlert(ctx context.Context, alert *models.Alert) error {
	q := `INSERT OR IGNORE INTO alerts (id, type, message, severity, timestamp)
	      VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q, alert.ID, alert.Type, alert.Message, alert.Severity, alert.Timestamp)
	if err != nil {
		return fmt.Errorf("traffic.InsertAlert: %w", err)
	}
	return nil
}

func (r *sqliteTrafficRepository) ListAlerts(ctx context.Context, limit int) ([]*models.Alert, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, type, message, severity, timestamp FROM alerts ORDER BY timestamp DESC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("traffic.ListAlerts: %w", err)
	}
	defer rows.Close()

	var alerts []*models.Alert
	for rows.Next() {
		a := &models.Alert{}
		if err := rows.Scan(&a.ID, &a.Type, &a.Message, &a.Severity, &a.Timestamp); err != nil {
			return nil, fmt.Errorf("traffic.ListAlerts scan: %w", err)
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (r *sqliteTrafficRepository) GetPeerTrafficSummary(ctx context.Context) ([]*models.PeerTrafficSummary, error) {
	q := `SELECT
		p.id, p.name, p.is_active, p.last_seen,
		COALESCE(a.total_rx, 0) AS total_rx,
		COALESCE(a.total_tx, 0) AS total_tx,
		COALESCE(a.conn_count, 0) AS conn_count,
		a.top_domain
	FROM wg_peers p
	LEFT JOIN (
		SELECT
			peer_id,
			SUM(bytes_rx) AS total_rx,
			SUM(bytes_tx) AS total_tx,
			COUNT(*) AS conn_count,
			(SELECT domain FROM traffic_logs t2
			 WHERE t2.peer_id = t1.peer_id AND t2.domain != ''
			 GROUP BY domain ORDER BY SUM(bytes_rx + bytes_tx) DESC LIMIT 1) AS top_domain
		FROM traffic_logs t1
		GROUP BY peer_id
	) a ON p.id = a.peer_id
	ORDER BY COALESCE(a.total_rx, 0) + COALESCE(a.total_tx, 0) DESC`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("traffic.GetPeerTrafficSummary: %w", err)
	}
	defer rows.Close()

	var summaries []*models.PeerTrafficSummary
	for rows.Next() {
		s := &models.PeerTrafficSummary{}
		var lastSeen sql.NullTime
		var topDomain sql.NullString
		if err := rows.Scan(&s.PeerID, &s.PeerName, &s.IsActive, &lastSeen, &s.TotalRx, &s.TotalTx, &s.ConnCount, &topDomain); err != nil {
			return nil, fmt.Errorf("traffic.GetPeerTrafficSummary scan: %w", err)
		}
		if lastSeen.Valid {
			s.LastSeen = &lastSeen.Time
			s.Online = time.Since(lastSeen.Time) < 2*time.Minute
		}
		if topDomain.Valid {
			s.TopDomain = topDomain.String
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

func (r *sqliteTrafficRepository) DeleteByPeerID(ctx context.Context, peerID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM traffic_logs WHERE peer_id = ?", peerID)
	if err != nil {
		return fmt.Errorf("traffic.DeleteByPeerID: %w", err)
	}
	return nil
}
