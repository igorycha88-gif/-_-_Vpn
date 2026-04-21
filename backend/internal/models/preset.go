package models

import "time"

type Preset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Rules       string    `json:"rules"`
	IsBuiltin   bool      `json:"is_builtin"`
	CreatedAt   time.Time `json:"created_at"`
}

type PresetApplyResponse struct {
	AppliedRules int `json:"applied_rules"`
}
