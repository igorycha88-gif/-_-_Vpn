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
	OnlinePeers   int   `json:"online_peers"`
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
	PeerID           string     `json:"peer_id"`
	PeerName         string     `json:"peer_name"`
	TotalRx          int64      `json:"total_rx"`
	TotalTx          int64      `json:"total_tx"`
	Online           bool       `json:"online"`
	IsActive         bool       `json:"is_active"`
	LastSeen         *time.Time `json:"last_seen,omitempty"`
	ConnCount        int        `json:"conn_count"`
	TopDomain        string     `json:"top_domain,omitempty"`
	ActiveConns      int        `json:"active_conns"`
	BandwidthRateRx  float64    `json:"bandwidth_rate_rx"`
	BandwidthRateTx  float64    `json:"bandwidth_rate_tx"`
	ConnectedAt      *time.Time `json:"connected_at,omitempty"`
	SessionRx        int64      `json:"session_rx"`
	SessionTx        int64      `json:"session_tx"`
}

type PeerSession struct {
	ID               int64      `json:"id"`
	PeerID           string     `json:"peer_id"`
	ConnectedAt      time.Time  `json:"connected_at"`
	DisconnectedAt   *time.Time `json:"disconnected_at,omitempty"`
	BytesRx          int64      `json:"bytes_rx"`
	BytesTx          int64      `json:"bytes_tx"`
	ConnectionsCount int        `json:"connections_count"`
}

type PeerRealtimeStats struct {
	ActiveConnections int        `json:"active_connections"`
	BandwidthRx       int64      `json:"bandwidth_rx"`
	BandwidthTx       int64      `json:"bandwidth_tx"`
	BandwidthRateRx   float64    `json:"bandwidth_rate_rx"`
	BandwidthRateTx   float64    `json:"bandwidth_rate_tx"`
	ConnectedAt       *time.Time `json:"connected_at,omitempty"`
	SessionRx         int64      `json:"session_rx"`
	SessionTx         int64      `json:"session_tx"`
}

type TrafficAggregateItem struct {
	Domain string `json:"domain"`
	RX     int64  `json:"rx"`
	TX     int64  `json:"tx"`
	Count  int    `json:"count"`
}
