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

INSERT OR IGNORE INTO routing_presets (id, name, description, rules, is_builtin)
VALUES (
    'preset-russia-blocked',
    'Рунет + Заблокированные',
    'Российские ресурсы (Госуслуги, VK, Макс, Яндекс и др.) напрямую, заблокированные сервисы (YouTube, Instagram, Facebook, Telegram) через прокси',
    '[{"type":"domain_suffix","pattern":".ru","action":"direct"},{"type":"domain_suffix","pattern":".su","action":"direct"},{"type":"domain_suffix","pattern":".xn--p1ai","action":"direct"},{"type":"geoip","pattern":"ru","action":"direct"},{"type":"domain_suffix","pattern":"vk.com","action":"direct"},{"type":"domain_suffix","pattern":"userapi.com","action":"direct"},{"type":"domain_suffix","pattern":"vk-cdn.net","action":"direct"},{"type":"domain_suffix","pattern":"vkuservideo.net","action":"direct"},{"type":"domain_suffix","pattern":"yandex.com","action":"direct"},{"type":"domain_suffix","pattern":"yastatic.net","action":"direct"},{"type":"domain_suffix","pattern":"habr.com","action":"direct"},{"type":"domain_suffix","pattern":"kaspersky.com","action":"direct"},{"type":"domain_suffix","pattern":"youtube.com","action":"proxy"},{"type":"domain_suffix","pattern":"youtu.be","action":"proxy"},{"type":"domain_suffix","pattern":"googlevideo.com","action":"proxy"},{"type":"domain_suffix","pattern":"ytimg.com","action":"proxy"},{"type":"domain_suffix","pattern":"yt3.ggpht.com","action":"proxy"},{"type":"domain_suffix","pattern":"instagram.com","action":"proxy"},{"type":"domain_suffix","pattern":"cdninstagram.com","action":"proxy"},{"type":"domain_suffix","pattern":"fbcdn.net","action":"proxy"},{"type":"domain_suffix","pattern":"facebook.com","action":"proxy"},{"type":"domain_suffix","pattern":"meta.com","action":"proxy"},{"type":"domain_suffix","pattern":"telegram.org","action":"proxy"},{"type":"domain_suffix","pattern":"t.me","action":"proxy"},{"type":"domain_suffix","pattern":"twitter.com","action":"proxy"},{"type":"domain_suffix","pattern":"x.com","action":"proxy"},{"type":"domain_suffix","pattern":"t.co","action":"proxy"},{"type":"domain_suffix","pattern":"twimg.com","action":"proxy"},{"type":"domain_suffix","pattern":"discord.com","action":"proxy"},{"type":"domain_suffix","pattern":"discordapp.com","action":"proxy"},{"type":"domain_suffix","pattern":"discord.gg","action":"proxy"},{"type":"domain_suffix","pattern":"netflix.com","action":"proxy"},{"type":"domain_suffix","pattern":"spotify.com","action":"proxy"},{"type":"domain_suffix","pattern":"tiktok.com","action":"proxy"},{"type":"domain_suffix","pattern":"openai.com","action":"proxy"},{"type":"domain_suffix","pattern":"chatgpt.com","action":"proxy"}]',
    TRUE
);

INSERT OR IGNORE INTO dns_settings (id, upstream_ru, upstream_foreign, block_ads)
VALUES (1, '77.88.8.8,77.88.8.1', '1.1.1.1,8.8.8.8', FALSE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-ru-domains', 'Российские домены (.ru)', 'domain_suffix', '.ru', 'direct', 1, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-ru-geoip', 'Российские IP (GeoIP)', 'geoip', 'ru', 'direct', 2, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-ru-su', 'Домены .su (Россия)', 'domain_suffix', '.su', 'direct', 3, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-ru-rf', 'Домены .рф (IDN)', 'domain_suffix', '.xn--p1ai', 'direct', 4, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-vk', 'VK (социальная сеть)', 'domain_suffix', 'vk.com', 'direct', 5, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-vk-api', 'VK API / CDN', 'domain_suffix', 'userapi.com', 'direct', 6, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-vk-cdn', 'VK CDN', 'domain_suffix', 'vk-cdn.net', 'direct', 7, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-yandex', 'Яндекс (международный)', 'domain_suffix', 'yandex.com', 'direct', 8, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-yastatic', 'Яндекс Static', 'domain_suffix', 'yastatic.net', 'direct', 9, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-habr', 'Habr', 'domain_suffix', 'habr.com', 'direct', 10, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-kaspersky', 'Kaspersky', 'domain_suffix', 'kaspersky.com', 'direct', 11, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-youtube', 'YouTube', 'domain_suffix', 'youtube.com', 'proxy', 12, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-youtu-be', 'YouTube (короткие ссылки)', 'domain_suffix', 'youtu.be', 'proxy', 13, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-googlevideo', 'Google Video CDN', 'domain_suffix', 'googlevideo.com', 'proxy', 14, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-instagram', 'Instagram', 'domain_suffix', 'instagram.com', 'proxy', 15, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-instagram-cdn', 'Instagram CDN', 'domain_suffix', 'cdninstagram.com', 'proxy', 16, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-facebook', 'Facebook', 'domain_suffix', 'facebook.com', 'proxy', 17, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-meta', 'Meta', 'domain_suffix', 'meta.com', 'proxy', 18, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-telegram', 'Telegram', 'domain_suffix', 'telegram.org', 'proxy', 19, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-t-me', 'Telegram ссылки', 'domain_suffix', 't.me', 'proxy', 20, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-twitter', 'Twitter / X', 'domain_suffix', 'twitter.com', 'proxy', 21, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-x-com', 'X.com', 'domain_suffix', 'x.com', 'proxy', 22, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-discord', 'Discord', 'domain_suffix', 'discord.com', 'proxy', 23, TRUE);

INSERT OR IGNORE INTO routing_rules (id, name, type, pattern, action, priority, is_active)
VALUES ('rule-chatgpt', 'ChatGPT / OpenAI', 'domain_suffix', 'chatgpt.com', 'proxy', 24, TRUE);
