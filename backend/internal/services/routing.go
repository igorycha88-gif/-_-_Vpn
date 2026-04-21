package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"

	"github.com/google/uuid"
)

type RoutingService struct {
	routeRepo  repository.RouteRepository
	presetRepo repository.PresetRepository
	logger     *slog.Logger
}

func NewRoutingService(routeRepo repository.RouteRepository, presetRepo repository.PresetRepository, logger *slog.Logger) *RoutingService {
	return &RoutingService{
		routeRepo:  routeRepo,
		presetRepo: presetRepo,
		logger:     logger,
	}
}

func (s *RoutingService) CreateRule(ctx context.Context, req *models.RoutingRuleCreateRequest) (*models.RoutingRule, error) {
	if errs := req.Validate(); len(errs) > 0 {
		return nil, fmt.Errorf("service.routing.CreateRule: невалидные данные: %v", errs)
	}

	priority := req.Priority
	if priority == 0 {
		count, err := s.routeRepo.Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("service.routing.CreateRule count: %w", err)
		}
		priority = count + 1
	}

	rule := &models.RoutingRule{
		ID:       uuid.New().String(),
		Name:     req.Name,
		Type:     req.Type,
		Pattern:  req.Pattern,
		Action:   req.Action,
		Priority: priority,
		IsActive: true,
	}

	if err := s.routeRepo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("service.routing.CreateRule save: %w", err)
	}

	s.logger.Info("создано правило маршрутизации", "id", rule.ID, "name", rule.Name)
	return rule, nil
}

func (s *RoutingService) GetRule(ctx context.Context, id string) (*models.RoutingRule, error) {
	rule, err := s.routeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service.routing.GetRule: %w", err)
	}
	return rule, nil
}

func (s *RoutingService) ListRules(ctx context.Context) ([]*models.RoutingRule, error) {
	rules, err := s.routeRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.routing.ListRules: %w", err)
	}
	return rules, nil
}

func (s *RoutingService) UpdateRule(ctx context.Context, id string, req *models.RoutingRuleUpdateRequest) (*models.RoutingRule, error) {
	rule, err := s.routeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service.routing.UpdateRule get: %w", err)
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Type != nil {
		if !containsStr(models.ValidRuleTypes, *req.Type) {
			return nil, fmt.Errorf("service.routing.UpdateRule: недопустимый тип: %s", *req.Type)
		}
		rule.Type = *req.Type
	}
	if req.Pattern != nil {
		rule.Pattern = *req.Pattern
	}
	if req.Action != nil {
		if !containsStr(models.ValidRuleActions, *req.Action) {
			return nil, fmt.Errorf("service.routing.UpdateRule: недопустимое действие: %s", *req.Action)
		}
		rule.Action = *req.Action
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.IsActive != nil {
		rule.IsActive = *req.IsActive
	}

	if err := s.routeRepo.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("service.routing.UpdateRule save: %w", err)
	}

	s.logger.Info("обновлено правило маршрутизации", "id", id)
	return rule, nil
}

func (s *RoutingService) DeleteRule(ctx context.Context, id string) error {
	if err := s.routeRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("service.routing.DeleteRule: %w", err)
	}
	s.logger.Info("удалено правило маршрутизации", "id", id)
	return nil
}

func (s *RoutingService) ReorderRules(ctx context.Context, req *models.ReorderRequest) error {
	if errs := req.Validate(); len(errs) > 0 {
		return fmt.Errorf("service.routing.ReorderRules: невалидные данные: %v", errs)
	}
	if err := s.routeRepo.Reorder(ctx, req.IDs); err != nil {
		return fmt.Errorf("service.routing.ReorderRules: %w", err)
	}
	s.logger.Info("правила маршрутизации переупорядочены", "count", len(req.IDs))
	return nil
}

func (s *RoutingService) ApplyPreset(ctx context.Context, presetID string) (*models.PresetApplyResponse, error) {
	preset, err := s.presetRepo.GetByID(ctx, presetID)
	if err != nil {
		return nil, fmt.Errorf("service.routing.ApplyPreset: %w", err)
	}

	var presetRules []struct {
		Type    string `json:"type"`
		Pattern string `json:"pattern"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal([]byte(preset.Rules), &presetRules); err != nil {
		return nil, fmt.Errorf("service.routing.ApplyPreset parse rules: %w", err)
	}

	existingRules, err := s.routeRepo.List(ctx)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("service.routing.ApplyPreset list: %w", err)
	}
	for _, r := range existingRules {
		_ = s.routeRepo.Delete(ctx, r.ID)
	}

	applied := 0
	for i, pr := range presetRules {
		rule := &models.RoutingRule{
			ID:       uuid.New().String(),
			Name:     fmt.Sprintf("preset:%s:rule:%d", preset.Name, i+1),
			Type:     pr.Type,
			Pattern:  pr.Pattern,
			Action:   pr.Action,
			Priority: i + 1,
			IsActive: true,
		}
		if err := s.routeRepo.Create(ctx, rule); err != nil {
			s.logger.Error("ошибка применения правила пресета", "error", err)
			continue
		}
		applied++
	}

	s.logger.Info("применён пресет", "preset", preset.Name, "applied", applied)
	return &models.PresetApplyResponse{AppliedRules: applied}, nil
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
