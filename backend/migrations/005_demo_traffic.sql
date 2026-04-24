INSERT OR IGNORE INTO wg_peers (id, name, email, device_type, public_key, private_key, address, dns, mtu, is_active, total_rx, total_tx, last_seen)
VALUES
    ('demo-peer-001', 'iPhone (Демо)', 'demo@smarttraffic.local', 'iphone', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'demo-private-key-001', '10.10.0.2', '1.1.1.1,8.8.8.8', 1280, 1, 52428800, 31457280, datetime('now', '-2 minutes'));

INSERT OR IGNORE INTO wg_peers (id, name, email, device_type, public_key, private_key, address, dns, mtu, is_active, total_rx, total_tx, last_seen)
VALUES
    ('demo-peer-002', 'Android (Демо)', NULL, 'android', 'b2c3d4e5-f6a7-8901-bcde-f23456789012', 'demo-private-key-002', '10.10.0.3', '1.1.1.1,8.8.8.8', 1280, 1, 31457280, 20971520, datetime('now', '-1 minutes'));

INSERT OR IGNORE INTO wg_peers (id, name, email, device_type, public_key, private_key, address, dns, mtu, is_active, total_rx, total_tx, last_seen)
VALUES
    ('demo-peer-003', 'MacBook (Демо)', NULL, 'iphone', 'c3d4e5f6-a7b8-9012-cdef-345678901234', 'demo-private-key-003', '10.10.0.4', '1.1.1.1,8.8.8.8', 1280, 0, 10485760, 5242880, datetime('now', '-1 hour'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'youtube.com', '142.250.80.46', 443, 'proxy', 15728640, 1048576, datetime('now', '-5 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'googlevideo.com', '142.250.80.46', 443, 'proxy', 20971520, 524288, datetime('now', '-4 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'vk.com', '87.240.165.68', 443, 'direct', 5242880, 1048576, datetime('now', '-3 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'yandex.ru', '77.88.55.80', 443, 'direct', 3145728, 524288, datetime('now', '-2 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'instagram.com', '31.13.80.1', 443, 'proxy', 8388608, 2097152, datetime('now', '-1 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-002', 'telegram.org', '149.154.167.99', 443, 'proxy', 4194304, 1048576, datetime('now', '-10 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-002', 't.me', '149.154.167.99', 443, 'proxy', 2097152, 524288, datetime('now', '-9 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-002', 'habr.com', '178.248.233.32', 443, 'direct', 6291456, 3145728, datetime('now', '-8 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-002', 'github.com', '20.205.243.166', 443, 'proxy', 12582912, 5242880, datetime('now', '-7 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-002', 'youtube.com', '142.250.80.46', 443, 'proxy', 10485760, 2097152, datetime('now', '-6 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-003', 'chatgpt.com', '104.18.32.7', 443, 'proxy', 7340032, 2097152, datetime('now', '-30 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-003', 'gosuslugi.ru', '185.122.164.2', 443, 'direct', 1048576, 524288, datetime('now', '-25 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-003', 'discord.com', '162.159.128.233', 443, 'proxy', 3145728, 1048576, datetime('now', '-20 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    (NULL, NULL, '10.20.0.2', 0, 'tunnel_transfer', 104857600, 52428800, datetime('now', '-1 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    (NULL, NULL, '10.20.0.2', 0, 'tunnel_transfer', 78643200, 41943040, datetime('now', '-11 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'youtube.com', '142.250.80.46', 443, 'vless_transfer', 1048576, 262144, datetime('now', '-45 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-002', 'facebook.com', '31.13.80.1', 443, 'vless_transfer', 2097152, 524288, datetime('now', '-50 minutes'));

INSERT OR IGNORE INTO traffic_logs (peer_id, domain, dest_ip, dest_port, action, bytes_rx, bytes_tx, timestamp)
VALUES
    ('demo-peer-001', 'twitter.com', '104.244.42.1', 443, 'proxy', 4194304, 1048576, datetime('now', '-55 minutes'));

INSERT OR IGNORE INTO alerts (id, type, message, severity, timestamp)
VALUES
    ('demo-alert-001', 'system', 'Система запущена', 'info', datetime('now', '-1 hour'));

INSERT OR IGNORE INTO alerts (id, type, message, severity, timestamp)
VALUES
    ('demo-alert-002', 'peer', 'Клиент подключился: iPhone (Демо)', 'info', datetime('now', '-30 minutes'));

INSERT OR IGNORE INTO alerts (id, type, message, severity, timestamp)
VALUES
    ('demo-alert-003', 'peer', 'Клиент подключился: Android (Демо)', 'info', datetime('now', '-20 minutes'));

INSERT OR IGNORE INTO alerts (id, type, message, severity, timestamp)
VALUES
    ('demo-alert-004', 'tunnel', 'Межсерверный WG тоннель восстановлен', 'info', datetime('now', '-15 minutes'));

INSERT OR IGNORE INTO alerts (id, type, message, severity, timestamp)
VALUES
    ('demo-alert-005', 'peer', 'Клиент отключился: MacBook (Демо)', 'warning', datetime('now', '-1 hour'));
