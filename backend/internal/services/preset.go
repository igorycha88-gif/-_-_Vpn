package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"smarttraffic/internal/apperrors"
	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"

	"github.com/google/uuid"
)

type PresetService struct {
	presetRepo repository.PresetRepository
	routeRepo  repository.RouteRepository
	logger     *slog.Logger
}

func NewPresetService(presetRepo repository.PresetRepository, routeRepo repository.RouteRepository, logger *slog.Logger) *PresetService {
	return &PresetService{
		presetRepo: presetRepo,
		routeRepo:  routeRepo,
		logger:     logger,
	}
}

func (s *PresetService) List(ctx context.Context) ([]*models.Preset, error) {
	presets, err := s.presetRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.preset.List: %w", err)
	}
	return presets, nil
}

func (s *PresetService) GetByID(ctx context.Context, id string) (*models.Preset, error) {
	preset, err := s.presetRepo.GetByID(ctx, id)
	if err != nil {
		return nil, s.mapErr(fmt.Errorf("service.preset.GetByID: %w", err))
	}
	return preset, nil
}

func (s *PresetService) ApplyPreset(ctx context.Context, presetID string) (*models.PresetApplyResponse, error) {
	preset, err := s.presetRepo.GetByID(ctx, presetID)
	if err != nil {
		return nil, s.mapErr(fmt.Errorf("service.preset.ApplyPreset: %w", err))
	}

	var presetRules []struct {
		Type    string `json:"type"`
		Pattern string `json:"pattern"`
		Action  string `json:"action"`
	}
	if err := json.Unmarshal([]byte(preset.Rules), &presetRules); err != nil {
		return nil, fmt.Errorf("service.preset.ApplyPreset parse rules: %w", err)
	}

	existingRules, err := s.routeRepo.List(ctx)
	if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("service.preset.ApplyPreset list: %w", err)
	}
	for _, r := range existingRules {
		if err := s.routeRepo.Delete(ctx, r.ID); err != nil {
			s.logger.Error("ошибка удаления правила при применении пресета", "id", r.ID, "error", err)
		}
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

func (s *PresetService) mapErr(err error) error {
	if err == nil {
		return nil
	}
	if repository.IsNotFound(err) {
		return fmt.Errorf("%w: %v", apperrors.ErrNotFound, err)
	}
	return err
}
