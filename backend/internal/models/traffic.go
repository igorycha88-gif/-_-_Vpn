package models

import "time"

type TrafficLog struct {
	ID        int64     `json:"id"`
	PeerID    string    `json:"peer_id,omitempty"`
	Domain    string    `json:"domain,omitempty"`
	DestIP    string    `json:"dest_ip,omitempty"`
	DestPort  int       `json:"dest_port,omitempty"`
	Action    string    `json:"action"`
	BytesRx   int64     `json:"bytes_rx"`
	BytesTx   int64     `json:"bytes_tx"`
	Timestamp time.Time `json:"timestamp"`
}

type TrafficFilter struct {
	PeerID    string
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

type TotalStats struct {
	TotalRx       int64 `json:"total_rx"`
	TotalTx       int64 `json:"total_tx"`
	ActivePeers   int   `json:"active_peers"`
	TotalPeers    int   `json:"total_peers"`
	RulesCount    int   `json:"rules_count"`
}

type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

type PeerTrafficSummary struct {
	PeerID    string    `json:"peer_id"`
	PeerName  string    `json:"peer_name"`
	TotalRx   int64     `json:"total_rx"`
	TotalTx   int64     `json:"total_tx"`
	Online    bool      `json:"online"`
	IsActive  bool      `json:"is_active"`
	LastSeen  *time.Time `json:"last_seen,omitempty"`
	ConnCount int       `json:"conn_count"`
	TopDomain string    `json:"top_domain,omitempty"`
}
