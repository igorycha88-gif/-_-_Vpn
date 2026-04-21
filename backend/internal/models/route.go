package models

import "time"

type RoutingRule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Pattern   string    `json:"pattern"`
	Action    string    `json:"action"`
	Priority  int       `json:"priority"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoutingRuleCreateRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Pattern  string `json:"pattern"`
	Action   string `json:"action"`
	Priority int    `json:"priority,omitempty"`
}

type RoutingRuleUpdateRequest struct {
	Name     *string `json:"name,omitempty"`
	Type     *string `json:"type,omitempty"`
	Pattern  *string `json:"pattern,omitempty"`
	Action   *string `json:"action,omitempty"`
	Priority *int    `json:"priority,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

var ValidRuleTypes = []string{"domain", "ip", "geoip", "port", "regex", "domain_suffix", "domain_keyword"}
var ValidRuleActions = []string{"direct", "proxy", "block"}

func (r *RoutingRuleCreateRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Name == "" {
		errs["name"] = "обязательное поле"
	}
	if !containsString(ValidRuleTypes, r.Type) {
		errs["type"] = "недопустимый тип"
	}
	if r.Pattern == "" {
		errs["pattern"] = "обязательное поле"
	}
	if !containsString(ValidRuleActions, r.Action) {
		errs["action"] = "недопустимое действие"
	}
	return errs
}

type ReorderRequest struct {
	IDs []string `json:"ids"`
}

func (r *ReorderRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if len(r.IDs) == 0 {
		errs["ids"] = "список не может быть пустым"
	}
	return errs
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
