package models

type ServerStatus struct {
	RU     ServerInfo `json:"ru"`
	Foreign ServerInfo `json:"foreign"`
}

type ServerInfo struct {
	Online    bool   `json:"online"`
	IP        string `json:"ip,omitempty"`
	Uptime    string `json:"uptime,omitempty"`
	CPUUsage  string `json:"cpu_usage,omitempty"`
	RAMUsage  string `json:"ram_usage,omitempty"`
	DiskUsage string `json:"disk_usage,omitempty"`
}

type ServerStats struct {
	TotalRx      int64  `json:"total_rx"`
	TotalTx      int64  `json:"total_tx"`
	ActivePeers  int    `json:"active_peers"`
	TotalPeers   int    `json:"total_peers"`
	WGStatus     string `json:"wg_status"`
	SingboxStatus string `json:"singbox_status"`
}
