package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"smarttraffic/internal/models"
)

var ErrNotFound = errors.New("not found")

type RouteRepository interface {
	Create(ctx context.Context, rule *models.RoutingRule) error
	GetByID(ctx context.Context, id string) (*models.RoutingRule, error)
	List(ctx context.Context) ([]*models.RoutingRule, error)
	Update(ctx context.Context, rule *models.RoutingRule) error
	Delete(ctx context.Context, id string) error
	Reorder(ctx context.Context, ids []string) error
	Count(ctx context.Context) (int, error)
}

type sqliteRouteRepository struct {
	db *sql.DB
}

func NewRouteRepository(db *sql.DB) RouteRepository {
	return &sqliteRouteRepository{db: db}
}

func (r *sqliteRouteRepository) Create(ctx context.Context, rule *models.RoutingRule) error {
	q := `INSERT INTO routing_rules (id, name, type, pattern, action, priority, is_active)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, q,
		rule.ID, rule.Name, rule.Type, rule.Pattern, rule.Action, rule.Priority, rule.IsActive,
	)
	if err != nil {
		return fmt.Errorf("routes.Create: %w", err)
	}
	return nil
}

func (r *sqliteRouteRepository) GetByID(ctx context.Context, id string) (*models.RoutingRule, error) {
	q := `SELECT id, name, type, pattern, action, priority, is_active, created_at, updated_at
	      FROM routing_rules WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, id)

	rule := &models.RoutingRule{}
	err := row.Scan(
		&rule.ID, &rule.Name, &rule.Type, &rule.Pattern,
		&rule.Action, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("routes.GetByID: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("routes.GetByID: %w", err)
	}
	return rule, nil
}

func (r *sqliteRouteRepository) List(ctx context.Context) ([]*models.RoutingRule, error) {
	q := `SELECT id, name, type, pattern, action, priority, is_active, created_at, updated_at
	      FROM routing_rules ORDER BY priority ASC`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("routes.List: %w", err)
	}
	defer rows.Close()

	var rules []*models.RoutingRule
	for rows.Next() {
		rule := &models.RoutingRule{}
		err := rows.Scan(
			&rule.ID, &rule.Name, &rule.Type, &rule.Pattern,
			&rule.Action, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("routes.List scan: %w", err)
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *sqliteRouteRepository) Update(ctx context.Context, rule *models.RoutingRule) error {
	q := `UPDATE routing_rules SET name=?, type=?, pattern=?, action=?, priority=?, is_active=?, updated_at=CURRENT_TIMESTAMP
	      WHERE id=?`
	result, err := r.db.ExecContext(ctx, q,
		rule.Name, rule.Type, rule.Pattern, rule.Action, rule.Priority, rule.IsActive, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("routes.Update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("routes.Update: %w", ErrNotFound)
	}
	return nil
}

func (r *sqliteRouteRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM routing_rules WHERE id=?", id)
	if err != nil {
		return fmt.Errorf("routes.Delete: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("routes.Delete: %w", ErrNotFound)
	}
	return nil
}

func (r *sqliteRouteRepository) Reorder(ctx context.Context, ids []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("routes.Reorder: begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "UPDATE routing_rules SET priority=?, updated_at=CURRENT_TIMESTAMP WHERE id=?")
	if err != nil {
		return fmt.Errorf("routes.Reorder: prepare: %w", err)
	}
	defer stmt.Close()

	for i, id := range ids {
		_, err := stmt.ExecContext(ctx, i+1, id)
		if err != nil {
			return fmt.Errorf("routes.Reorder: exec: %w", err)
		}
	}

	return tx.Commit()
}

func (r *sqliteRouteRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM routing_rules").Scan(&count)
	return count, err
}
