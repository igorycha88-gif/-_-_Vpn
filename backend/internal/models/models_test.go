package models

import (
	"strings"
	"testing"
)

func TestPeerCreateRequest_Validate_Valid(t *testing.T) {
	req := PeerCreateRequest{Name: "Test Peer", DeviceType: DeviceTypeIPhone}
	errs := req.Validate()
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestPeerCreateRequest_Validate_EmptyName(t *testing.T) {
	req := PeerCreateRequest{Name: "", DeviceType: DeviceTypeIPhone}
	errs := req.Validate()
	if _, ok := errs["name"]; !ok {
		t.Error("expected error for name")
	}
}

func TestPeerCreateRequest_Validate_EmptyDeviceType(t *testing.T) {
	req := PeerCreateRequest{Name: "Test", DeviceType: ""}
	errs := req.Validate()
	if _, ok := errs["device_type"]; !ok {
		t.Error("expected error for device_type")
	}
}

func TestPeerCreateRequest_Validate_InvalidDeviceType(t *testing.T) {
	req := PeerCreateRequest{Name: "Test", DeviceType: "windows"}
	errs := req.Validate()
	if _, ok := errs["device_type"]; !ok {
		t.Error("expected error for device_type")
	}
}

func TestPeerCreateRequest_Validate_NameTooLong(t *testing.T) {
	req := PeerCreateRequest{Name: strings.Repeat("a", 256), DeviceType: DeviceTypeIPhone}
	errs := req.Validate()
	if _, ok := errs["name"]; !ok {
		t.Error("expected error for name too long")
	}
}

func TestRoutingRuleCreateRequest_Validate_Valid(t *testing.T) {
	req := RoutingRuleCreateRequest{
		Name:    "Test Rule",
		Type:    "domain",
		Pattern: "example.com",
		Action:  "direct",
	}
	errs := req.Validate()
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestRoutingRuleCreateRequest_Validate_EmptyName(t *testing.T) {
	req := RoutingRuleCreateRequest{
		Name:    "",
		Type:    "domain",
		Pattern: "example.com",
		Action:  "direct",
	}
	errs := req.Validate()
	if _, ok := errs["name"]; !ok {
		t.Error("expected error for name")
	}
}

func TestRoutingRuleCreateRequest_Validate_InvalidType(t *testing.T) {
	req := RoutingRuleCreateRequest{
		Name:    "Test",
		Type:    "invalid",
		Pattern: "example.com",
		Action:  "direct",
	}
	errs := req.Validate()
	if _, ok := errs["type"]; !ok {
		t.Error("expected error for type")
	}
}

func TestRoutingRuleCreateRequest_Validate_EmptyPattern(t *testing.T) {
	req := RoutingRuleCreateRequest{
		Name:    "Test",
		Type:    "domain",
		Pattern: "",
		Action:  "direct",
	}
	errs := req.Validate()
	if _, ok := errs["pattern"]; !ok {
		t.Error("expected error for pattern")
	}
}

func TestRoutingRuleCreateRequest_Validate_InvalidAction(t *testing.T) {
	req := RoutingRuleCreateRequest{
		Name:    "Test",
		Type:    "domain",
		Pattern: "example.com",
		Action:  "invalid",
	}
	errs := req.Validate()
	if _, ok := errs["action"]; !ok {
		t.Error("expected error for action")
	}
}

func TestReorderRequest_Validate_Valid(t *testing.T) {
	req := ReorderRequest{IDs: []string{"id1", "id2", "id3"}}
	errs := req.Validate()
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestReorderRequest_Validate_EmptyIDs(t *testing.T) {
	req := ReorderRequest{IDs: []string{}}
	errs := req.Validate()
	if _, ok := errs["ids"]; !ok {
		t.Error("expected error for ids")
	}
}

func TestReorderRequest_Validate_NilIDs(t *testing.T) {
	req := ReorderRequest{IDs: nil}
	errs := req.Validate()
	if _, ok := errs["ids"]; !ok {
		t.Error("expected error for ids")
	}
}

func TestContainsString(t *testing.T) {
	slice := []string{"domain", "ip", "geoip", "port"}
	if !ContainsString(slice, "domain") {
		t.Error("expected true for domain")
	}
	if !ContainsString(slice, "ip") {
		t.Error("expected true for ip")
	}
	if ContainsString(slice, "missing") {
		t.Error("expected false for missing")
	}
	if ContainsString(nil, "anything") {
		t.Error("expected false for nil slice")
	}
	if ContainsString([]string{}, "anything") {
		t.Error("expected false for empty slice")
	}
}

func TestLoginRequest_Validate_Valid(t *testing.T) {
	req := LoginRequest{Email: "admin@example.com", Password: "secret123"}
	errs := req.Validate()
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}

func TestLoginRequest_Validate_EmptyEmail(t *testing.T) {
	req := LoginRequest{Email: "", Password: "secret123"}
	errs := req.Validate()
	if _, ok := errs["email"]; !ok {
		t.Error("expected error for email")
	}
}

func TestLoginRequest_Validate_EmptyPassword(t *testing.T) {
	req := LoginRequest{Email: "admin@example.com", Password: ""}
	errs := req.Validate()
	if _, ok := errs["password"]; !ok {
		t.Error("expected error for password")
	}
}

func TestLoginRequest_Validate_BothEmpty(t *testing.T) {
	req := LoginRequest{}
	errs := req.Validate()
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
}

func TestRoutingRuleCreateRequest_Validate_AllValidTypes(t *testing.T) {
	for _, typ := range ValidRuleTypes {
		req := RoutingRuleCreateRequest{
			Name:    "Test",
			Type:    typ,
			Pattern: "example.com",
			Action:  "direct",
		}
		errs := req.Validate()
		if _, ok := errs["type"]; ok {
			t.Errorf("type %q should be valid", typ)
		}
	}
}

func TestRoutingRuleCreateRequest_Validate_AllValidActions(t *testing.T) {
	for _, action := range ValidRuleActions {
		req := RoutingRuleCreateRequest{
			Name:    "Test",
			Type:    "domain",
			Pattern: "example.com",
			Action:  action,
		}
		errs := req.Validate()
		if _, ok := errs["action"]; ok {
			t.Errorf("action %q should be valid", action)
		}
	}
}

func TestPeerCreateRequest_Validate_BothDeviceTypes(t *testing.T) {
	for _, dt := range []string{DeviceTypeIPhone, DeviceTypeAndroid} {
		req := PeerCreateRequest{Name: "Test", DeviceType: dt}
		errs := req.Validate()
		if _, ok := errs["device_type"]; ok {
			t.Errorf("device_type %q should be valid", dt)
		}
	}
}

func TestPeerCreateRequest_Validate_NameAtMaxLength(t *testing.T) {
	req := PeerCreateRequest{Name: strings.Repeat("a", 255), DeviceType: DeviceTypeIPhone}
	errs := req.Validate()
	if _, ok := errs["name"]; ok {
		t.Error("name at 255 chars should be valid")
	}
}
