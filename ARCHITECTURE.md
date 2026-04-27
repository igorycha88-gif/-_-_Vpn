# SmartTraffic — Архитектура системы управления сетевым трафиком

> Умное разделение трафика: рунет напрямую, остальной мир — через зарубежный прокси-сервер.

---

## 1. Обзор системы

SmartTraffic — это решение для управления сетевым трафиком между двумя серверами (РФ + зарубежный), позволяющее прозрачно маршрутизировать запросы: российские ресурсы обслуживаются напрямую, а доступ к зарубежным ресурсам идёт через VLESS relay на зарубежный сервер.

### Ключевые возможности

- VLESS+Reality (sing-box) для подключения клиентов с маскировкой под HTTPS
- VLESS relay между РФ и зарубежным сервером (RU sing-box → Foreign sing-box)
- WireGuard для резервного межсерверного тоннеля (РФ ↔ зарубежный)
- Автоматическое разделение трафика (рунет / зарубежный)
- Веб-панель администратора для управления правилами
- Маскировочный сайт-заглушка на основном домене
- Мониторинг и статистика в реальном времени
- Управление VLESS-клиентами (добавление/удаление, генерация sing-box JSON конфигов, QR-коды)

---

## 2. Высокоуровневая архитектура

```
┌──────────────┐            ┌──────────────── РОССИЙСКИЙ СЕРВЕР ────────────────┐
│              │            │                                                    │
│   Клиенты    │  VLESS+    │  ┌──────────────────────────────────────────────┐  │
│  (телефон,   │  Reality   │  │              Routing Engine                  │  │
│   ПК,        │  (sing-box │  │              (sing-box)                      │  │
│   планшет)   │  :443 TLS) │  │                                              │  │
│              │───────────▶│  │  VLESS Inbound ← клиенты (порт 443)         │  │
│              │            │  │                                              │  │
│              │            │  │  ┌─────────┐  ┌──────────────────────────┐  │  │
│              │            │  │  │ Direct  │  │  VLESS Relay (foreign)   │  │  │
│              │            │  │  │ (рунет) │  │  → Foreign :443          │  │  │
│              │            │  │  └────┬────┘  └───────────┬──────────────┘  │  │
│              │            │  └───────┼───────────────────┼─────────────────┘  │
│              │            │          │                   │                    │
│              │            │     напрямую            VLESS relay              │
│              │            │          │            (через интернет)           │
│              │            │          │                   │                    │
│              │            │  ┌───────────────────────┐   │                    │
│              │            │  │  Nginx (reverse proxy) │   │                    │
│              │            │  │  :80 HTTP              │   │                    │
│              │            │  │  ├─ / ──▶ Landing Page│   │                    │
│              │            │  │  ├─ /api ─▶ Go API    │   │                    │
│              │            │  │  └─ /admin ─▶ React   │   │                    │
│              │            │  └───────────────────────┘   │                    │
│              │            │                               │                    │
│              │            └───────────────────────────────┼────────────────────┘
│              │                                            │
└──────────────┘                                            ▼
                         ┌──────────────── ЗАРУБЕЖНЫЙ СЕРВЕР ────────────────┐
                         │                                                   │
                         │  ┌────────────────────────────────────────────┐   │
                         │  │  sing-box (systemd)                        │   │
                         │  │  VLESS+Reality inbound :443               │   │
                         │  │  → direct outbound → Global Internet      │   │
                         │  └────────────────────────────────────────────┘   │
                         │                                                   │
                         │  ┌──────────────┐                                 │
                         │  │  WireGuard   │                                 │
                         │  │  Server      │                                 │
                         │  │  (wg0 :51821)│  ← резервный тоннель от РФ    │
                         │  └──────────────┘                                 │
                         │                                                   │
                         │  iptables: NAT/Masquerade                         │
                         └───────────────────────────────────────────────────┘
```

---

## 3. Компоненты системы

### 3.1. VLESS+Reality (клиентские подключения)

- **Протокол**: VLESS + XTLS-Reality через sing-box
- **Порт**: 443/TLS (маскировка под HTTPS)
- **Аутентификация**: UUID клиента + Reality TLS handshake
- **Конфигурация**: генерируется через API админ-панели (sing-box JSON)
- **Клиенты**: sing-box (мобильные, десктоп)

### 3.2. VLESS Relay (межсерверный транспорт)

Трафик между РФ и зарубежным сервером передаётся через VLESS relay:

- **RU sing-box**: VLESS outbound → Foreign:443
- **Foreign sing-box**: VLESS inbound на :443 → direct outbound → интернет
- **Преимущества**: не требует отдельного тоннеля, маскируется под TLS, простая конфигурация

### 3.3. WireGuard (резервный межсерверный тоннель)

- **Порт**: 51821/UDP (на обоих серверах)
- **Интерфейс**: `wg1` (РФ), `wg0` (зарубежный)
- **Назначение**: резервный транспорт, мониторинг
- **IP-сеть**: 10.20.0.0/30 (point-to-point)

### 3.4. Routing Engine — sing-box (РФ-сервер)

sing-box — универсальная платформа проксирования с мощным движком маршрутизации.

Принимает VLESS+Reality подключения клиентов и маршрутизирует трафик:

```
Клиентский трафик (VLESS+Reality :443)
    │
    ▼
sing-box routing rules
    ├── Домен *.ru, .su, .xn--p1ai → direct outbound
    ├── Российские сервисы (vk, yandex, sber и т.д.) → direct outbound
    ├── Зарубежные сервисы (youtube, instagram, telegram и т.д.) → foreign-out
    └── Всё остальное → foreign-out (VLESS relay)
```

**Реальная конфигурация sing-box (РФ-сервер):**

```json
{
  "inbounds": [
    {
      "type": "vless",
      "tag": "vless-in",
      "listen": "::",
      "listen_port": 443,
      "users": [{"uuid": "<client-uuid>", "flow": "xtls-rprx-vision"}],
      "tls": {
        "enabled": true,
        "server_name": "www.microsoft.com",
        "reality": {
          "enabled": true,
          "handshake": {"server": "www.microsoft.com", "server_port": 443},
          "private_key": "<reality-private-key>",
          "short_id": ["<short-id>"]
        }
      }
    }
  ],
  "outbounds": [
    {"type": "direct", "tag": "direct-out"},
    {
      "type": "vless",
      "tag": "foreign-out",
      "server": "<foreign-ip>",
      "server_port": 443,
      "uuid": "<foreign-vless-uuid>",
      "flow": "xtls-rprx-vision",
      "tls": {
        "enabled": true,
        "server_name": "www.microsoft.com",
        "utls": {"enabled": true, "fingerprint": "chrome"},
        "reality": {
          "enabled": true,
          "public_key": "<foreign-reality-public-key>",
          "short_id": "<foreign-reality-short-id>"
        }
      }
    }
  ],
  "route": {
    "rules": [
      {"action": "sniff"},
      {"protocol": "dns", "action": "hijack-dns"},
      {"domain_suffix": [".ru"], "outbound": "direct-out"},
      {"domain_suffix": ["youtube.com"], "outbound": "foreign-out"}
    ],
    "final": "foreign-out"
  }
}
```

### 3.5. sing-box (зарубежный сервер)

Принимает VLESS relay трафик от РФ-сервера и отправляет напрямую в интернет:

```json
{
  "inbounds": [
    {
      "type": "vless",
      "tag": "vless-in",
      "listen": "::",
      "listen_port": 443,
      "users": [{"uuid": "<vless-uuid>", "flow": "xtls-rprx-vision"}],
      "tls": {
        "enabled": true,
        "server_name": "www.microsoft.com",
        "reality": {
          "enabled": true,
          "handshake": {"server": "www.microsoft.com", "server_port": 443},
          "private_key": "<reality-private-key>",
          "short_id": ["<short-id>"]
        }
      }
    }
  ],
  "outbounds": [
    {"type": "direct", "tag": "direct-out"}
  ]
}
```

### 3.6. Nginx Reverse Proxy (маскировка + доступ к API)

Схема маршрутизации HTTP (порт 80):

```
:80 (HTTP)
  ├── /               → Landing Page (статический сайт-заглушка)
  ├── /api/v1/*       → Go Backend API (:8080)
  ├── /admin/*        → React SPA (:3000)
  ├── /admin/api/*    → Go Backend API (:8080)
  └── /health         → Go Backend API (:8080)
```

> Порт 443 занят sing-box (VLESS+Reality), поэтому Nginx работает только на HTTP :80.

### 3.7. Маскировочный сайт-заглушка

**Цель**: при проверке домена/IP должен выглядеть как обычный легитимный ресурс.

**Рекомендуемые варианты сайта-заглушки:**

| Вариант | Описание | Почему не вызывает подозрений |
|---|---|---|
| **IT-консалтинг** | Сайт компании «услуги IT-аутсорсинга» | Совпадает с профилем: сервер, домен, SSL — всё логично |
| **Веб-студия** | Портфолио, услуги разработки сайтов | Объясняет наличие технической инфраструктуры |
| **Облачные сервисы** | «Провайдер облачных решений» | Объясняет трафик и серверную инфраструктуру |

**Содержимое сайта-заглушки:**

- Главная страница: описание компании, услуги
- Страница «О компании»: реквизиты (вымышленные), контакты
- Страница «Услуги»: IT-аутсорсинг, администрирование, DevOps
- Страница «Контакты»: форма обратной связи
- Favicon, мета-теги, Open Graph — всё как у настоящего сайта
- robots.txt, sitemap.xml — для правдоподобности

### 3.8. Backend API (Go)

REST API для управления всей системой.

**Технологии:**
- Go 1.22+
- Chi Router (легковесный HTTP-роутер)
- SQLite (через modernc.org/sqlite)
- sing-box — управление через генерацию конфигурации и hot-reload

**Основные эндпоинты:**

```
# Аутентификация
POST   /api/v1/auth/login
POST   /api/v1/auth/refresh
GET    /api/v1/auth/session
POST   /api/v1/auth/logout
POST   /api/v1/auth/logout-all

# VLESS клиенты
GET    /api/v1/wg/peers
POST   /api/v1/wg/peers
POST   /api/v1/wg/peers/sync
GET    /api/v1/wg/peers/{id}
DELETE /api/v1/wg/peers/{id}
GET    /api/v1/wg/peers/{id}/config
GET    /api/v1/wg/peers/{id}/qr
GET    /api/v1/wg/peers/{id}/stats
PUT    /api/v1/wg/peers/{id}/toggle

# Правила маршрутизации
GET    /api/v1/routes
POST   /api/v1/routes
PUT    /api/v1/routes/reorder
GET    /api/v1/routes/{id}
PUT    /api/v1/routes/{id}
DELETE /api/v1/routes/{id}

# Пресеты маршрутизации
GET    /api/v1/presets
POST   /api/v1/presets/{id}/apply

# DNS
GET    /api/v1/dns/settings
PUT    /api/v1/dns/settings
GET    /api/v1/dns/presets

# Серверы
GET    /api/v1/servers/status
GET    /api/v1/servers/ru/stats
GET    /api/v1/servers/foreign/stats

# Мониторинг
GET    /api/v1/monitoring/traffic
GET    /api/v1/monitoring/traffic/aggregate
GET    /api/v1/monitoring/logs
GET    /api/v1/monitoring/alerts
GET    /api/v1/monitoring/stats
GET    /api/v1/monitoring/peer/{id}
GET    /api/v1/monitoring/peers-stats
```

**Модели данных:**

```sql
-- VLESS клиенты
CREATE TABLE wg_peers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT,
    device_type TEXT DEFAULT 'iphone',
    public_key  TEXT NOT NULL UNIQUE,
    private_key TEXT NOT NULL,
    address     TEXT NOT NULL UNIQUE,
    dns         TEXT DEFAULT '1.1.1.1,8.8.8.8',
    mtu         INTEGER DEFAULT 1280,
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    total_rx    INTEGER DEFAULT 0,
    total_tx    INTEGER DEFAULT 0,
    last_seen   DATETIME
);

-- Правила маршрутизации
CREATE TABLE routing_rules (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    pattern     TEXT NOT NULL,
    action      TEXT NOT NULL,
    priority    INTEGER NOT NULL,
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Пресеты маршрутизации
CREATE TABLE routing_presets (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    rules       TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Логи трафика
CREATE TABLE traffic_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    peer_id     TEXT,
    domain      TEXT,
    dest_ip     TEXT,
    dest_port   INTEGER,
    action      TEXT,
    bytes_rx    INTEGER,
    bytes_tx    INTEGER,
    timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- DNS настройки
CREATE TABLE dns_settings (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    upstream_ru TEXT DEFAULT '77.88.8.8,77.88.8.1',
    upstream_foreign TEXT DEFAULT '1.1.1.1,8.8.8.8',
    block_ads   BOOLEAN DEFAULT FALSE
);

-- Алерты
CREATE TABLE alerts (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    type        TEXT NOT NULL,
    message     TEXT NOT NULL,
    severity    TEXT DEFAULT 'warning',
    is_read     BOOLEAN DEFAULT FALSE,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### 3.9. Frontend Admin Panel (React + TypeScript)

**Технологии:**
- React 18 + TypeScript
- Ant Design — UI-компоненты
- React Query — управление серверным состоянием
- Vite — сборка

**Страницы админ-панели:**

| Страница | Описание |
|---|---|
| **Dashboard** | Обзор системы: онлайн-клиенты, загрузка каналов, статус серверов |
| **VLESS → Клиенты** | Список клиентов, добавление/удаление, генерация sing-box JSON конфигов и QR-кодов |
| **Маршрутизация → Правила** | CRUD правил, drag-and-drop для приоритетов, тестирование правила |
| **Маршрутизация → Пресеты** | Готовые профили: «Всё напрямую», «Всё через прокси», «Авто-рунет» |
| **Маршрутизация → Списки** | Управление community-списками (antifilter, zapret-info) |
| **DNS** | Настройка DNS для разных зон |
| **Мониторинг** | Графики трафика, логи, алерты |
| **Настройки** | Общие настройки системы, бэкап/восстановление |

### 3.10. Зарубежный сервер — компоненты

| Компонент | Порт | Назначение |
|---|---|---|
| sing-box (systemd) | 443/TCP | VLESS+Reality — приём relay трафика от РФ-сервера |
| WireGuard (wg0) | 51821/UDP | Резервный межсерверный тоннель от РФ-сервера |
| xray (systemd) | 8443/TCP | Сторонний сервис (не входит в SmartTraffic) |

**Поток трафика через зарубежный сервер:**

```
РФ-сервер (sing-box VLESS outbound)
    │
    ▼ VLESS+Reality (порт 443, через интернет)
Зарубежный сервер
    │
    ▼
sing-box VLESS inbound (port 443)
    │
    ├─ DNS-резолв через зарубежные DNS (1.1.1.1, 8.8.8.8)
    │
    ▼
Прямое подключение к глобальному интернету (direct outbound)
```

---

## 4. Сетевая схема

```
                     Интернет
                        │
            ┌───────────┴───────────┐
            │                       │
    ┌───────┴───────┐       ┌──────┴──────┐
    │  РУНЕТ        │       │  ЗАРУБЕЖНЫЙ │
    │  (прямые IP)  │       │  ИНТЕРНЕТ   │
    └───────┬───────┘       └──────┬──────┘
            │                      │
    ┌───────┴──────────────────────┴──────┐
    │          РФ СЕРВЕР                  │
    │                                     │
    │  eth0: <public-ip>                  │
    │  sing-box: VLESS :443 (клиенты)    │
    │  sing-box: Clash API :9090 (local) │
    │  nginx: :80 (HTTP)                 │
    │  api: :8080 (Go backend)           │
    │  wg1: 10.20.0.1/30 → зарубежный   │
    │                                     │
    │  sing-box:                          │
    │    VLESS inbound → direct ИЛИ       │
    │    VLESS relay → Foreign :443       │
    │                                     │
    │  Nginx :80 → Landing + Admin        │
    └──────────────────┬──────────────────┘
                       │ VLESS relay :443 (через интернет)
                       │ + WG tunnel :51821 (резерв)
    ┌──────────────────┴──────────────────┐
    │       ЗАРУБЕЖНЫЙ СЕРВЕР             │
    │                                     │
    │  eth0: <public-ip>                  │
    │  sing-box: VLESS+Reality :443      │
    │  wg0:  10.20.0.2/30 ← РФ сервер    │
    │  xray: VLESS :8443 (сторонний)     │
    │                                     │
    │  iptables: NAT/Masquerade           │
    └─────────────────────────────────────┘
```

---

## 5. iptables — правила на РФ-сервере

```bash
sysctl -w net.ipv4.ip_forward=1

iptables -t nat -A POSTROUTING -o wg1 -j MASQUERADE

iptables -A FORWARD -i wg1 -j ACCEPT
iptables -A FORWARD -o wg1 -j ACCEPT
iptables -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT

iptables -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -o eth0 -j TCPMSS --clamp-mss-to-pmtu
```

---

## 6. Безопасность

### 6.1. Маскировка

| Мера | Описание |
|---|---|
| **Landing Page** | Легитимный сайт на :80 |
| **VLESS на :443** | Выглядит как обычный HTTPS-трафик (Reality TLS) |
| **API за аутентификацией** | Админка и API доступны только по JWT-токену |
| **Rate Limiting** | Защита от брутфорса на /api/v1/auth/login |

### 6.2. Аутентификация

- JWT-токены с коротким TTL (15 минут access + refresh token)
- Хеширование паролей через bcrypt

### 6.3. Изоляция

- sing-box запускается в отдельном контейнере
- Backend API не слушает на публичном интерфейсе — только через Nginx
- WG тоннель ограничен point-to-point (10.20.0.0/30)

---

## 7. Порты — сводная таблица (production)

### 7.1. РФ-сервер

| Порт | Протокол | Процесс | Назначение |
|---|---|---|---|
| **22** | TCP | sshd | SSH-доступ |
| **80** | TCP | nginx | HTTP (Landing + API + Admin) |
| **443** | TCP | sing-box | VLESS+Reality (клиентские подключения) |
| **8080** | TCP | api (Go) | Backend REST API (только localhost) |
| **9090** | TCP | sing-box | Clash API (только localhost, мониторинг) |
| **51821** | UDP | WireGuard | Межсерверный тоннель РФ ↔ зарубежный (wg1) |

### 7.2. Зарубежный сервер

| Порт | Протокол | Процесс | Назначение |
|---|---|---|---|
| **22** | TCP | sshd | SSH-доступ |
| **443** | TCP | sing-box | VLESS+Reality (приём relay от РФ-сервера) |
| **51821** | UDP | WireGuard | Резервный межсерверный тоннель от РФ-сервера (wg0) |
| **8443** | TCP | xray | Сторонний сервис (не входит в SmartTraffic) |

---

## 8. Docker Compose — деплой (РФ-сервер)

```yaml
services:
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx/conf.d:/etc/nginx/conf.d
      - certbot-data:/etc/letsencrypt:ro
    depends_on:
      - api
      - frontend
      - landing

  certbot:
    image: certbot/certbot
    volumes:
      - certbot-data:/etc/letsencrypt
      - ./certbot/www:/var/www/certbot

  api:
    build: ./backend
    environment:
      - APP_PORT=8080
      - DB_PATH=/data/smarttraffic.db
      - JWT_SECRET=${JWT_SECRET}
      - VLESS_PRIVATE_KEY=${VLESS_PRIVATE_KEY}
      - VLESS_PUBLIC_KEY=${VLESS_PUBLIC_KEY}
      - VLESS_SHORT_ID=${VLESS_SHORT_ID}
      - VLESS_SERVER_NAME=${VLESS_SERVER_NAME}
      - VLESS_PORT=${VLESS_PORT}
      - VLESS_FLOW=${VLESS_FLOW}
      - VLESS_FINGERPRINT=${VLESS_FINGERPRINT}
      - VLESS_SERVER_ENDPOINT=${VLESS_SERVER_ENDPOINT}
      - WG_TUNNEL_INTERFACE=${WG_TUNNEL_INTERFACE}
      - SINGBOX_CONFIG_PATH=/etc/singbox/config.json
      - SINGBOX_CLASH_API_ADDR=${SINGBOX_CLASH_API_ADDR}
      - FOREIGN_SERVER_IP=${FOREIGN_SERVER_IP}
      - FOREIGN_VLESS_UUID=${FOREIGN_VLESS_UUID}
      - FOREIGN_VLESS_REALITY_PUBLIC_KEY=${FOREIGN_VLESS_REALITY_PUBLIC_KEY}
      - FOREIGN_VLESS_REALITY_SHORT_ID=${FOREIGN_VLESS_REALITY_SHORT_ID}
    volumes:
      - api-data:/data
      - ./singbox:/etc/singbox
    network_mode: host
    cap_add:
      - NET_ADMIN

  singbox:
    image: ghcr.io/sagernet/sing-box
    volumes:
      - ./singbox:/etc/singbox
    network_mode: host
    cap_add:
      - NET_ADMIN

  frontend:
    build: ./frontend

  landing:
    build: ./landing

volumes:
  certbot-data:
  api-data:
```

---

## 9. Пошаговый план развёртывания

### Этап 1: Подготовка серверов

1. Арендовать VPS в РФ (минимум 2 vCPU, 2 GB RAM, Ubuntu 22.04/24.04)
2. Арендовать VPS за рубежом (минимум 1 vCPU, 1 GB RAM, Ubuntu 22.04/24.04)
3. Зарегистрировать домен и привязать A-запись к IP РФ-сервера

### Этап 2: Настройка зарубежного сервера

1. Запустить `scripts/setup-foreign.sh <RU_IP>`
2. Настроить sing-box с VLESS+Reality на порту 443
3. Настроить WireGuard wg0 для межсерверного тоннеля
4. Настроить NAT/форвардинг

### Этап 3: Настройка РФ-сервера

1. Запустить `scripts/setup-ru.sh`
2. Настроить WireGuard wg1 (межсерверный тоннель)
3. Установить Docker и Docker Compose

### Этап 4: Развёртывание сервисов на РФ-сервере

1. Клонировать репозиторий
2. Заполнить `.env`
3. Сгенерировать ключи: `scripts/generate-keys.sh vless`
4. Запустить `docker compose -f deploy/server-ru/docker-compose.prod.yml up -d`

### Этап 5: Настройка и тестирование

1. Добавить первого VLESS-клиента через API
2. Подключиться клиентом, проверить разделение трафика
3. Настроить начальные правила маршрутизации

---

## 10. Community-списки для маршрутизации

Автоматически обновляемые списки российских ресурсов:

| Список | Источник | Описание |
|---|---|---|
| **antifilter-community** | https://community.antifilter.download/ | Список заблокированных доменов (для прокси) |
| **antifilter-russia** | https://antifilter.download/ | IP-списки |
| **GeoIP Ru** | MaxMind / DB-IP | IP-адреса российских сетей |
| **Geosite Ru** | v2fly/domain-list-community | Российские домены |

---

## 11. Структура проекта

```
smarttraffic/
├── ARCHITECTURE.md
├── docker-compose.yml
├── .env.example
├── Makefile
│
├── deploy/
│   ├── server-ru/
│   │   ├── docker-compose.prod.yml
│   │   ├── .env.deploy.example
│   │   ├── wireguard/
│   │   │   └── wg1.conf              # Межсерверный тоннель
│   │   ├── singbox/
│   │   │   ├── config.json           # Конфиг sing-box (генерируется)
│   │   │   └── rules/
│   │   ├── nginx/
│   │   │   └── conf.d/
│   │   │       └── default.conf
│   │   ├── iptables/
│   │   │   └── rules.sh
│   │   └── scripts/
│   │       └── remote-deploy-ru.sh
│   │
│   └── server-foreign/
│       ├── wireguard/
│       │   └── wg0.conf              # Конфиг приёма тоннеля
│       ├── iptables/
│       │   └── rules.sh
│       └── scripts/
│           └── remote-deploy-foreign.sh
│
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── cmd/api/main.go
│   ├── internal/
│   │   ├── config/config.go
│   │   ├── models/
│   │   ├── handlers/
│   │   ├── services/
│   │   ├── repository/
│   │   └── middleware/
│   ├── migrations/
│   └── pkg/
│
├── frontend/
│   ├── Dockerfile
│   ├── src/
│   │   ├── pages/
│   │   ├── components/
│   │   ├── api/
│   │   ├── hooks/
│   │   └── types/
│   └── package.json
│
├── landing/
│   ├── Dockerfile
│   ├── index.html
│   ├── about.html
│   ├── services.html
│   └── contacts.html
│
├── scripts/
│   ├── setup-ru.sh
│   ├── setup-foreign.sh
│   └── generate-keys.sh
│
└── .github/workflows/
    ├── ci.yml
    ├── deploy-ru.yml
    ├── deploy-foreign.yml
    └── rollback.yml
```

---

## 12. Рекомендации по серверам

### РФ-сервер (минимум)

| Параметр | Значение |
|---|---|
| CPU | 2 vCPU |
| RAM | 2 GB |
| Диск | 30 GB SSD |
| Сеть | 100 Mbit/s+, безлимит |
| ОС | Ubuntu 22.04 / 24.04 LTS |
| Расположение | Москва / Санкт-Петербург |

### Зарубежный сервер (минимум)

| Параметр | Значение |
|---|---|
| CPU | 1 vCPU |
| RAM | 1 GB |
| Диск | 15 GB SSD |
| Сеть | 100 Mbit/s+, безлимит |
| ОС | Ubuntu 22.04 / 24.04 LTS |
| Расположение | Нидерланды / Финляндия / Германия |

---

## 13. Дальнейшее развитие (Roadmap)

### v1.0 — MVP ✅

- [x] VLESS+Reality для клиентских подключений (sing-box)
- [x] VLESS relay между серверами (RU → Foreign)
- [x] WireGuard межсерверный тоннель (резервный)
- [x] sing-box с маршрутизацией (рунет / мир)
- [x] REST API для управления
- [x] Админ-панель (клиенты, правила)
- [x] Маскировочный сайт

### v1.1 — Улучшения

- [ ] Автообновление community-списков по расписанию
- [ ] Мониторинг трафика в реальном времени (WebSocket)
- [ ] Экспорт/импорт конфигурации
- [ ] Бэкап БД по расписанию

### v2.0 — Продвинутые функции

- [ ] Поддержка нескольких зарубежных серверов
- [ ] Балансировка нагрузки между прокси
- [ ] OAuth2 / SSO аутентификация
- [ ] Мобильное приложение для админки
- [ ] API для интеграции с внешними системами
- [ ] Поддержка IPv6
