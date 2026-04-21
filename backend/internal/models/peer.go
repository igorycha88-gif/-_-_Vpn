package models

import "time"

type Peer struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Email      string     `json:"email,omitempty"`
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
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	DNS   string `json:"dns,omitempty"`
	MTU   int    `json:"mtu,omitempty"`
}

func (r *PeerCreateRequest) Validate() map[string]string {
	errs := make(map[string]string)
	if r.Name == "" {
		errs["name"] = "обязательное поле"
	}
	if len(r.Name) > 255 {
		errs["name"] = "максимум 255 символов"
	}
	return errs
}

type PeerStats struct {
	PeerID   string `json:"peer_id"`
	TotalRx  int64  `json:"total_rx"`
	TotalTx  int64  `json:"total_tx"`
	Online   bool   `json:"online"`
}
