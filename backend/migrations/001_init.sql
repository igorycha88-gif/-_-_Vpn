CREATE TABLE IF NOT EXISTS admin_users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES admin_users(id)
);

CREATE TABLE IF NOT EXISTS wg_peers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT,
    public_key TEXT NOT NULL UNIQUE,
    private_key TEXT NOT NULL,
    address TEXT NOT NULL UNIQUE,
    dns TEXT DEFAULT '1.1.1.1,8.8.8.8',
    mtu INTEGER DEFAULT 1280,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_rx INTEGER DEFAULT 0,
    total_tx INTEGER DEFAULT 0,
    last_seen DATETIME
);

CREATE TABLE IF NOT EXISTS routing_rules (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('domain', 'ip', 'geoip', 'port', 'regex', 'domain_suffix', 'domain_keyword')),
    pattern TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('direct', 'proxy', 'block')),
    priority INTEGER NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS routing_presets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    rules TEXT NOT NULL,
    is_builtin BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS traffic_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    peer_id TEXT,
    domain TEXT,
    dest_ip TEXT,
    dest_port INTEGER,
    action TEXT,
    bytes_rx INTEGER,
    bytes_tx INTEGER,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (peer_id) REFERENCES wg_peers(id)
);

CREATE TABLE IF NOT EXISTS dns_settings (
    id INTEGER PRIMARY KEY CHECK(id = 1),
    upstream_ru TEXT DEFAULT '77.88.8.8,77.88.8.1',
    upstream_foreign TEXT DEFAULT '1.1.1.1,8.8.8.8',
    block_ads BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_wg_peers_public_key ON wg_peers(public_key);
CREATE INDEX idx_wg_peers_is_active ON wg_peers(is_active);
CREATE INDEX idx_routing_rules_priority ON routing_rules(priority);
CREATE INDEX idx_routing_rules_type ON routing_rules(type);
CREATE INDEX idx_traffic_logs_peer_id ON traffic_logs(peer_id);
CREATE INDEX idx_traffic_logs_timestamp ON traffic_logs(timestamp);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
