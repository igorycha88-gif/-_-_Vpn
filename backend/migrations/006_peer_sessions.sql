CREATE TABLE IF NOT EXISTS peer_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    peer_id TEXT NOT NULL,
    connected_at DATETIME NOT NULL,
    disconnected_at DATETIME,
    bytes_rx INTEGER DEFAULT 0,
    bytes_tx INTEGER DEFAULT 0,
    connections_count INTEGER DEFAULT 0,
    FOREIGN KEY (peer_id) REFERENCES wg_peers(id)
);

CREATE INDEX IF NOT EXISTS idx_peer_sessions_peer_id ON peer_sessions(peer_id);
CREATE INDEX IF NOT EXISTS idx_peer_sessions_connected_at ON peer_sessions(connected_at DESC);
