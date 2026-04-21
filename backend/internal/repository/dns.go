package repository

import (
	"context"
	"database/sql"
	"fmt"

	"smarttraffic/internal/models"
)

type DNSRepository interface {
	Get(ctx context.Context) (*models.DNSSettings, error)
	Update(ctx context.Context, settings *models.DNSSettings) error
}

type sqliteDNSRepository struct {
	db *sql.DB
}

func NewDNSRepository(db *sql.DB) DNSRepository {
	return &sqliteDNSRepository{db: db}
}

func (r *sqliteDNSRepository) Get(ctx context.Context) (*models.DNSSettings, error) {
	q := `SELECT id, upstream_ru, upstream_foreign, block_ads FROM dns_settings WHERE id = 1`
	row := r.db.QueryRowContext(ctx, q)

	s := &models.DNSSettings{}
	err := row.Scan(&s.ID, &s.UpstreamRU, &s.UpstreamForeign, &s.BlockAds)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dns.Get: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("dns.Get: %w", err)
	}
	return s, nil
}

func (r *sqliteDNSRepository) Update(ctx context.Context, settings *models.DNSSettings) error {
	q := `UPDATE dns_settings SET upstream_ru=?, upstream_foreign=?, block_ads=? WHERE id=1`
	result, err := r.db.ExecContext(ctx, q, settings.UpstreamRU, settings.UpstreamForeign, settings.BlockAds)
	if err != nil {
		return fmt.Errorf("dns.Update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("dns.Update: %w", ErrNotFound)
	}
	return nil
}
