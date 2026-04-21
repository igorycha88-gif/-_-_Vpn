package repository

import (
	"context"
	"database/sql"
	"fmt"

	"smarttraffic/internal/models"
)

type PresetRepository interface {
	Create(ctx context.Context, preset *models.Preset) error
	GetByID(ctx context.Context, id string) (*models.Preset, error)
	List(ctx context.Context) ([]*models.Preset, error)
	Delete(ctx context.Context, id string) error
}

type sqlitePresetRepository struct {
	db *sql.DB
}

func NewPresetRepository(db *sql.DB) PresetRepository {
	return &sqlitePresetRepository{db: db}
}

func (r *sqlitePresetRepository) Create(ctx context.Context, preset *models.Preset) error {
	q := `INSERT INTO routing_presets (id, name, description, rules, is_builtin)
	      VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		preset.ID, preset.Name, preset.Description, preset.Rules, preset.IsBuiltin,
	)
	if err != nil {
		return fmt.Errorf("presets.Create: %w", err)
	}
	return nil
}

func (r *sqlitePresetRepository) GetByID(ctx context.Context, id string) (*models.Preset, error) {
	q := `SELECT id, name, description, rules, is_builtin, created_at
	      FROM routing_presets WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)

	p := &models.Preset{}
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Rules, &p.IsBuiltin, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("presets.GetByID: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("presets.GetByID: %w", err)
	}
	return p, nil
}

func (r *sqlitePresetRepository) List(ctx context.Context) ([]*models.Preset, error) {
	q := `SELECT id, name, description, rules, is_builtin, created_at
	      FROM routing_presets ORDER BY created_at ASC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("presets.List: %w", err)
	}
	defer rows.Close()

	var presets []*models.Preset
	for rows.Next() {
		p := &models.Preset{}
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Rules, &p.IsBuiltin, &p.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("presets.List scan: %w", err)
		}
		presets = append(presets, p)
	}
	return presets, rows.Err()
}

func (r *sqlitePresetRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM routing_presets WHERE id=? AND is_builtin=FALSE", id)
	if err != nil {
		return fmt.Errorf("presets.Delete: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("presets.Delete: %w", ErrNotFound)
	}
	return nil
}
