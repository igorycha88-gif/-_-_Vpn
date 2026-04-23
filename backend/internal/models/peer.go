package models

import "time"

const (
	DeviceTypeIPhone = "iphone"
	DeviceTypeAndroid = "android"
)

var ValidDeviceTypes = map[string]bool{
	DeviceTypeIPhone:  true,
	DeviceTypeAndroid: true,
}

type Peer struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email,omitempty"`
	DeviceType string     `json:"device_type"`
	PublicKey  string     `json:"public_key"`
	PrivateKey string     `json:"private_key,omitempty"`
	Address    string     `json:"address"`
	DNS        string     `json:"dns"`
	MTU        int        `json:"mtu"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	TotalRx    int64      `json:"total_rx"`
	TotalTx    int64      `json:"total_tx"`
	LastSeen   *time.Time `json:"last_seen,omitempty"`
}

type PeerCreateRequest struct {
	Name       string `json:"name"`
	Email      string `json:"email,omitempty"`
	DeviceType string `json:"device_type"`
}

func (r *PeerCreateRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Name == "" {
		errs["name"] = "обязательное поле"
	}
	if len(r.Name) > 255 {
		errs["name"] = "максимум 255 символов"
	}
	if r.DeviceType == "" {
		errs["device_type"] = "обязательное поле"
	} else if !ValidDeviceTypes[r.DeviceType] {
		errs["device_type"] = "допустимые значения: iphone, android"
	}
	return errs
}

type PeerStats struct {
	PeerID  string `json:"peer_id"`
	TotalRx int64  `json:"total_rx"`
	TotalTx int64  `json:"total_tx"`
	Online  bool   `json:"online"`
}
