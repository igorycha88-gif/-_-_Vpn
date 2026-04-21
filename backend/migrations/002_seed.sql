INSERT OR IGNORE INTO admin_users (id, email, password_hash)
VALUES (
    'admin-001',
    'admin@smarttraffic.local',
    '$2a$12$enBheIEIdeGdBIneLdDYkOOXmi4d5bz5CFn5IypbPq8WnbuU37O/i'
);

INSERT OR IGNORE INTO routing_presets (id, name, description, rules, is_builtin)
VALUES (
    'preset-all-direct',
    'Всё напрямую',
    'Весь трафик идёт напрямую, без прокси',
    '[{"type":"regex","pattern":".*","action":"direct"}]',
    TRUE
);

INSERT OR IGNORE INTO routing_presets (id, name, description, rules, is_builtin)
VALUES (
    'preset-all-proxy',
    'Всё через прокси',
    'Весь трафик идёт через зарубежный прокси',
    '[{"type":"regex","pattern":".*","action":"proxy"}]',
    TRUE
);

INSERT OR IGNORE INTO routing_presets (id, name, description, rules, is_builtin)
VALUES (
    'preset-auto-russia',
    'Авто-рунет',
    'Российские ресурсы напрямую, остальное через прокси',
    '[{"type":"domain_suffix","pattern":".ru","action":"direct"},{"type":"geoip","pattern":"ru","action":"direct"}]',
    TRUE
);

INSERT OR IGNORE INTO dns_settings (id, upstream_ru, upstream_foreign, block_ads)
VALUES (1, '77.88.8.8,77.88.8.1', '1.1.1.1,8.8.8.8', FALSE);
