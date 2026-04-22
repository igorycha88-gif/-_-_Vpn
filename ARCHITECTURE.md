# SmartTraffic — Архитектура системы управления сетевым трафиком

> Умное разделение трафика: рунет напрямую, остальной мир — через зарубежный прокси-сервер.

---

## 1. Обзор системы

SmartTraffic — это решение для управления сетевым трафиком между двумя серверами (РФ + зарубежный), позволяющее прозрачно маршрутизировать запросы: российские ресурсы обслуживаются напрямую, а доступ к заблокированным ресурсам идёт через зарубежный прокси.

### Ключевые возможности

- VLESS+Reality (sing-box) для подключения клиентов с маскировкой под HTTPS
- WireGuard для межсерверного тоннеля (РФ ↔ зарубежный)
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
│              │            │  │  ┌─────────┐  ┌──────────┐  ┌───────────┐  │  │
│              │            │  │  │ Direct  │  │  Foreign  │  │  WG Tunnel│  │  │
│              │            │  │  │ (рунет) │  │  Proxy    │  │  (wg1)    │  │  │
│              │            │  │  └────┬────┘  └─────┬────┘  └─────┬─────┘  │  │
│              │            │  └───────┼─────────────┼─────────────┼─────────┘  │
│              │            │          │             │             │            │
│              │            │     напрямую      через WG       WG tunnel       │
│              │            │          │        endpoint        (wg1)          │
│              │            │          │             │             │            │
│              │            │  ┌───────────────────────┐         │             │
│              │            │  │  Nginx (reverse proxy) │         │             │
│              │            │  │  :80 → :443 SSL        │         │             │
│              │            │  │  ├─ / ──▶ Landing Page│         │             │
│              │            │  │  ├─ /api ─▶ Go API    │         │             │
│              │            │  │  └─ /admin ─▶ React   │         │             │
│              │            │  └───────────────────────┘         │             │
│              │            │                                    │             │
│              │            └────────────────────────────────────┼─────────────┘
│              │                                                 │
│              │                                         WG tunnel (wg1)
│              │                                                 │
└──────────────┘                                                 ▼
                         ┌──────────────── ЗАРУБЕЖНЫЙ СЕРВЕР ────────────────┐
                         │                                                   │
                         │  ┌──────────────┐    ┌────────────────────────┐   │
                         │  │  WireGuard   │───▶│  NAT / Masquerade      │   │
                         │  │  Server      │    │  (iptables SNAT)       │   │
                         │  │  (wg0)       │    │  → Global Internet     │   │
                         │  └──────────────┘    └────────────────────────┘   │
                         │                                                   │
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

### 3.2. WireGuard тоннель РФ ↔ Зарубежный (межсерверный)

- **Порт**: 51821/UDP (на зарубежном сервере)
- **Интерфейс**: `wg1` (на обоих серверах)
- **Назначение**: транспорт для зарубежного трафика
- **IP-сеть**: 10.20.0.0/30 (point-to-point)

### 3.3. Routing Engine — sing-box

**sing-box** — универсальная платформа проксирования с мощным движком маршрутизации.

Располагается на РФ-сервере, принимает VLESS+Reality подключения клиентов и маршрутизирует трафик:

```
Клиентский трафик (VLESS+Reality :443)
    │
    ▼
sing-box routing rules
    ├── Домен *.ru, российские IP → direct outbound
    ├── GeoIP:ru → direct outbound
    ├── Community списки (antifilter) → direct outbound
    └── Всё остальное → wireguard outbound (wg1 → зарубежный)
```

**Пример конфигурации sing-box (серверная часть):**

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
    {
      "type": "direct",
      "tag": "direct-out"
    },
    {
      "type": "wireguard",
      "tag": "foreign-out",
      "address": ["10.20.0.2/30"],
      "private_key": "<key>",
      "peers": [{"address": "<foreign-ip>", "port": 51821, "public_key": "<key>", "allowed_ips": ["0.0.0.0/0"]}]
    }
  ],
  "route": {
    "rules": [
      {"domain_suffix": [".ru"], "outbound": "direct-out"},
      {"geoip": ["ru"], "outbound": "direct-out"}
    ],
    "final": "foreign-out"
  }
}
```

### 3.4. Nginx Reverse Proxy (маскировка + доступ к API)

Схема маршрутизации HTTP/HTTPS:

```
:443 (SSL, Let's Encrypt)
  ├── /               → Landing Page (статический сайт-заглушка)
  ├── /api/v1/*       → Go Backend API (:8080)
  ├── /admin/*        → React SPA (:3000)
  └── /admin/api/*    → Go Backend API (:8080)
```

### 3.5. Маскировочный сайт-заглушка

**Цель**: при проверке домена/IP должен выглядеть как обычный легитимный ресурс.

**Рекомендуемые варианты сайта-заглушки:**

| Вариант | Описание | Почему не вызывает подозрений |
|---|---|---|
| **IT-консалтинг** | Сайт компании «услуги IT-аутсорсинга» | Совпадает с профилем: сервер, домен, SSL — всё логично |
| **Веб-студия** | Портфолио, услуги разработки сайтов | Объясняет наличие технической инфраструктуры |
| **Облачные сервисы** | «Провайдер облачных решений» | Объясняет трафик и серверную инфраструктуру |
| **VPN-сервис (легальный)** | Корпоративный VPN для бизнеса | Самый честный вариант, но привлекает внимание |

**Рекомендация**: **IT-консалтинг** или **веб-студия** — наиболее нейтральные варианты.

**Содержимое сайта-заглушки:**

```
- Главная страница: описание компании, услуги
- Страница «О компании»: реквизиты (вымышленные), контакты
- Страница «Услуги»: IT-аутсорсинг, администрирование, DevOps
- Страница «Контакты»: форма обратной связи (можно нерабочая)
- Favicon, мета-теги, Open Graph — всё как у настоящего сайта
- Роботс.txt, sitemap.xml — для правдоподобности
```

**Технические требования к заглушке:**

- Статический HTML/CSS (Next.js SSG или чистый HTML)
- Адаптивный дизайн
- SSL-сертификат Let's Encrypt (автообновление через certbot)
- Корректные HTTP-заголовки
- Никаких упоминаний VPN/прокси в коде

### 3.6. Backend API (Go)

REST API для управления всей системой.

**Технологии:**
- Go 1.22+
- Chi Router (легковесный HTTP-роутер)
- SQLite (через go-sqlite3 или modernc.org/sqlite)
- sing-box — управление через генерацию конфигурации и hot-reload

**Основные эндпоинты:**

```
# Аутентификация
POST   /api/v1/auth/login
POST   /api/v1/auth/logout
GET    /api/v1/auth/session

# VLESS клиенты
GET    /api/v1/wg/peers
POST   /api/v1/wg/peers              # Добавить клиента (генерирует UUID)
DELETE /api/v1/wg/peers/{id}         # Удалить клиента
GET    /api/v1/wg/peers/{id}/config  # Скачать sing-box JSON конфиг
GET    /api/v1/wg/peers/{id}/qr      # QR-код для подключения (sing-box)
GET    /api/v1/wg/peers/{id}/stats   # Статистика трафика

# Правила маршрутизации
GET    /api/v1/routes
POST   /api/v1/routes                # Создать правило
PUT    /api/v1/routes/{id}           # Обновить правило
DELETE /api/v1/routes/{id}           # Удалить правило
PUT    /api/v1/routes/reorder        # Изменить порядок правил

# Пресеты маршрутизации
GET    /api/v1/presets
POST   /api/v1/presets/{id}/apply    # Применить пресет

# Серверы
GET    /api/v1/servers/status        # Статус обоих серверов
GET    /api/v1/servers/ru/stats      # Статистика РФ-сервера
GET    /api/v1/servers/foreign/stats # Статистика зарубежного

# DNS
GET    /api/v1/dns/settings
PUT    /api/v1/dns/settings

# Мониторинг
GET    /api/v1/monitoring/traffic    # Трафик в реальном времени
GET    /api/v1/monitoring/logs       # Логи маршрутизации
GET    /api/v1/monitoring/alerts     # Алерты

# Community-списки
GET    /api/v1/lists
POST   /api/v1/lists/sync            # Синхронизация с источниками
```

**Модели данных:**

```sql
-- VLESS клиенты (таблица wg_peers для обратной совместимости)
CREATE TABLE wg_peers (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    email       TEXT,
    public_key  TEXT NOT NULL UNIQUE,  -- UUID клиента VLESS
    private_key TEXT NOT NULL,
    address     TEXT NOT NULL UNIQUE,  -- UUID клиента VLESS
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
    type        TEXT NOT NULL, -- 'domain', 'ip', 'geoip', 'port', 'regex'
    pattern     TEXT NOT NULL, -- домен, IP/CIDR, код страны, порт, regex
    action      TEXT NOT NULL, -- 'direct', 'proxy', 'block'
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
    rules       TEXT NOT NULL, -- JSON массив правил
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Логи трафика
CREATE TABLE traffic_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    peer_id     TEXT,
    domain      TEXT,
    dest_ip     TEXT,
    dest_port   INTEGER,
    action      TEXT, -- 'direct' или 'proxy'
    bytes_rx    INTEGER,
    bytes_tx    INTEGER,
    timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- DNS настройки
CREATE TABLE dns_settings (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    upstream_ru TEXT DEFAULT '77.88.8.8,77.88.8.1',     -- Яндекс DNS
    upstream_foreign TEXT DEFAULT '1.1.1.1,8.8.8.8',    -- Cloudflare/Google
    block_ads   BOOLEAN DEFAULT FALSE
);
```

### 3.7. Frontend Admin Panel (React + TypeScript)

**Технологии:**
- React 18 + TypeScript
- Ant Design — UI-компоненты (богатая библиотека для админок)
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

### 3.8. Зарубежный сервер

Минимальная конфигурация:

- WireGuard сервер на порту 51821/UDP (интерфейс `wg0`)
- NAT через iptables:
  ```bash
  iptables -t nat -A POSTROUTING -s 10.20.0.0/30 -o eth0 -j MASQUERADE
  iptables -A FORWARD -i wg0 -j ACCEPT
  iptables -A FORWARD -o wg0 -j ACCEPT
  ```
- Sysctl:
  ```
  net.ipv4.ip_forward = 1
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
    │  wg1:  10.20.0.1/30 → зарубежный   │
    │                                     │
    │  sing-box:                          │
    │    VLESS inbound → direct ИЛИ wg1   │
    │                                     │
    │  Nginx :80→:443 → Landing + Admin   │
    └──────────────────┬──────────────────┘
                       │ WG tunnel (wg1)
                       │ 10.20.0.0/30
    ┌──────────────────┴──────────────────┐
    │       ЗАРУБЕЖНЫЙ СЕРВЕР             │
    │                                     │
    │  eth0: <public-ip>                  │
    │  wg0:  10.20.0.2/30 ← РФ сервер    │
    │                                     │
    │  iptables: NAT/MASQUERADE           │
    └─────────────────────────────────────┘
```

---

## 5. iptables — правила на РФ-сервере

```bash
# Включаем форвардинг
sysctl -w net.ipv4.ip_forward=1

# NAT для трафика через зарубежный туннель
iptables -t nat -A POSTROUTING -o wg1 -j MASQUERADE

# NAT для прямого трафика клиентов (рунет)
iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE

# Разрешаем форвардинг
iptables -A FORWARD -i wg1 -j ACCEPT
iptables -A FORWARD -o wg1 -j ACCEPT
iptables -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT
```

---

## 6. Безопасность

### 6.1. Маскировка

| Мера | Описание |
|---|---|
| **Landing Page** | Легитимный сайт на :443, SSL Let's Encrypt |
| **VLESS на :443** | Выглядит как обычный HTTPS-трафик (Reality TLS) |
| **API за аутентификацией** | Админка и API доступны только по JWT-токену |
| **Rate Limiting** | Защита от брутфорса на /api/v1/auth/login |
| **Нет открытых портов** | Только :443 (HTTPS/VLESS) и :51821/UDP (WG тоннель) |

### 6.2. Аутентификация

- JWT-токены с коротким TTL (15 минут access + refresh token)
- Хеширование паролей через bcrypt (cost factor 12)
- Двухфакторная аутентификация (опционально, TOTP)

### 6.3. Изоляция

- sing-box запускается в отдельном контейнере/отдельным пользователем
- WireGuard работает на уровне ядра (максимальная производительность)
- Backend API не слушает на публичном интерфейсе — только через Nginx

---

## 7. Docker Compose — деплой

```yaml
# docker-compose.yml (РФ-сервер)

version: "3.9"

services:
  nginx:
    image: nginx:alpine
    ports:
      - "443:443"
      - "80:80"
    volumes:
      - ./nginx/conf.d:/etc/nginx/conf.d
      - ./landing/dist:/usr/share/nginx/html
      - certbot-data:/etc/letsencrypt
    depends_on:
      - api
      - frontend

  certbot:
    image: certbot/certbot
    volumes:
      - certbot-data:/etc/letsencrypt
      - ./certbot/www:/var/www/certbot
    entrypoint: "/bin/sh -c 'trap exit TERM; while :; do certbot renew; sleep 12h & wait $${!}; done;'"

  api:
    build: ./backend
    environment:
      - WG_INTERFACE=wg0
      - SINGBOX_CONFIG=/etc/singbox/config.json
      - DB_PATH=/data/smarttraffic.db
      - JWT_SECRET=${JWT_SECRET}
    volumes:
      - api-data:/data
      - /etc/wireguard:/etc/wireguard
      - ./singbox:/etc/singbox
    cap_add:
      - NET_ADMIN
    network_mode: host

  singbox:
    image: ghcr.io/sagernet/sing-box
    container_name: singbox
    volumes:
      - ./singbox:/etc/singbox
    ports:
      - "12345:12345"
      - "5353:5353/udp"
      - "5353:5353/tcp"
    cap_add:
      - NET_ADMIN
    network_mode: host
    restart: unless-stopped

  frontend:
    build: ./frontend
    ports:
      - "3000:3000"

  landing:
    build: ./landing
    # Статический сайт, отдаётся через Nginx

volumes:
  certbot-data:
  api-data:
```

---

## 8. Пошаговый план развёртывания

### Этап 1: Подготовка серверов

1. Арендовать VPS в РФ (минимум 2 vCPU, 2 GB RAM, Ubuntu 22.04/24.04)
2. Арендовать VPS за рубежом (минимум 1 vCPU, 1 GB RAM, Ubuntu 22.04/24.04)
3. Зарегистрировать домен и привязать A-запись к IP РФ-сервера
4. Получить SSL-сертификат Let's Encrypt

### Этап 2: Настройка зарубежного сервера

1. Установить WireGuard
2. Настроить wg0 для межсерверного тоннеля
3. Настроить NAT/форвардинг
4. Настроить фаервол (только порт 51821/UDP от IP РФ-сервера)

### Этап 3: Настройка РФ-сервера — базовая инфраструктура

1. Установить WireGuard
2. Настроить wg0 (клиентский VPN) и wg1 (межсерверный тоннель)
3. Настроить iptables для transparent proxy
4. Установить Docker и Docker Compose

### Этап 4: Развёртывание сервисов на РФ-сервере

1. Установить sing-box (Docker)
2. Развернуть Backend API (Docker)
3. Развернуть Frontend Admin Panel (Docker)
4. Настроить Nginx reverse proxy
5. Развернуть Landing Page
6. Получить SSL-сертификат

### Этап 5: Настройка и тестирование

1. Добавить первый WireGuard-клиент через CLI
2. Подключиться клиентом, проверить разделение трафика
3. Настроить начальные правила маршрутизации
4. Протестировать доступ к рунет-ресурсам (напрямую)
5. Протестировать доступ к зарубежным ресурсам (через прокси)
6. Проверить маскировочный сайт

### Этап 6: Эксплуатация

1. Настроить мониторинг и алерты
2. Настроить автообновление community-списков
3. Настроить бэкап БД
4. Добавить клиентов через админ-панель

---

## 9. Community-списки для маршрутизации

Автоматически обновляемые списки российских ресурсов:

| Список | Источник | Описание |
|---|---|---|
| **antifilter-community** | https://community.antifilter.download/ | Список заблокированных доменов (для прокси) |
| **antifilter-russia** | https://antifilter.download/ | IP-списки |
| **zapret-info** | GitHub mirror реестра РКН | Списки блокировок |
| **GeoIP Ru** | MaxMind / DB-IP | IP-адреса российских сетей |
| **Geosite Ru** | v2fly/domain-list-community | Российские домены |

**Логика**: всё, что в списках рунета → напрямую. Всё остальное → через зарубежный прокси. Можно инвертировать: всё из списков заблокированного → через прокси, остальное → напрямую.

---

## 10. Структура проекта

```
smarttraffic/
├── ARCHITECTURE.md                    # Этот документ
├── docker-compose.yml                 # Оркестрация сервисов (РФ)
├── .env.example                       # Пример переменных окружения
├── Makefile                           # Команды для сборки и деплоя
│
├── deploy/
│   ├── server-ru/
│   │   ├── setup.sh                   # Скрипт первоначальной настройки
│   │   ├── wireguard/
│   │   │   ├── wg0.conf              # Конфиг клиентского VPN
│   │   │   └── wg1.conf              # Конфиг межсерверного тоннеля
│   │   ├── singbox/
│   │   │   ├── config.json           # Конфиг sing-box
│   │   │   └── rules/                # Кастомные правила
│   │   ├── nginx/
│   │   │   └── conf.d/
│   │   │       └── default.conf      # Конфиг Nginx
│   │   ├── iptables/
│   │   │   └── rules.sh             # Скрипт правил iptables
│   │   └── scripts/
│   │       ├── sync-lists.sh         # Синхронизация community-списков
│   │       └── backup.sh             # Бэкап конфигурации
│   │
│   └── server-foreign/
│       ├── setup.sh                   # Скрипт настройки зарубежного сервера
│       ├── wireguard/
│       │   └── wg0.conf              # Конфиг приёма тоннеля
│       └── iptables/
│           └── rules.sh             # NAT правила
│
├── backend/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/
│   │   └── api/
│   │       └── main.go              # Точка входа
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go            # Конфигурация приложения
│   │   ├── models/
│   │   │   ├── peer.go              # Модель WireGuard клиента
│   │   │   ├── route.go             # Модель правила маршрутизации
│   │   │   ├── preset.go            # Модель пресета
│   │   │   └── traffic.go           # Модель статистики
│   │   ├── handlers/
│   │   │   ├── auth.go              # Аутентификация
│   │   │   ├── peers.go             # Управление VLESS клиентами
│   │   │   ├── routes.go            # Управление правилами
│   │   │   ├── presets.go           # Управление пресетами
│   │   │   ├── monitoring.go        # Мониторинг и статистика
│   │   │   ├── dns.go               # DNS настройки
│   │   │   └── server.go            # Статус серверов
│   │   ├── services/
│   │   │   ├── wireguard.go         # Логика управления VLESS клиентами
│   │   │   ├── singbox.go           # Генерация конфигов sing-box (сервер + клиент)
│   │   │   ├── routing.go           # Логика маршрутизации
│   │   │   ├── dns.go               # DNS сервис
│   │   │   └── traffic.go           # Сбор статистики
│   │   ├── repository/
│   │   │   ├── sqlite.go            # Подключение к SQLite
│   │   │   ├── peers.go             # Репозиторий клиентов
│   │   │   ├── routes.go            # Репозиторий правил
│   │   │   └── traffic.go           # Репозиторий статистики
│   │   └── middleware/
│   │       ├── auth.go              # JWT middleware
│   │       ├── cors.go              # CORS
│   │       └── ratelimit.go         # Rate limiting
│   ├── migrations/
│   │   ├── 001_init.sql             # Начальная схема БД
│   │   └── 002_seed.sql             # Начальные данные (пресеты)
│   └── pkg/
│       ├── wgcrypto/
│       │   └── keys.go              # Генерация ключей WireGuard (межсерверный тоннель)
│       └── qrcode/
│           └── qrcode.go            # Генерация QR-кодов (sing-box JSON конфиг)
│
├── frontend/
│   ├── Dockerfile
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── index.html
│   ├── public/
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── api/
│       │   ├── client.ts            # HTTP клиент
│       │   ├── auth.ts              # API аутентификации
│       │   ├── peers.ts             # API клиентов WG
│       │   ├── routes.ts            # API правил маршрутизации
│       │   └── monitoring.ts        # API мониторинга
│       ├── pages/
│       │   ├── Login.tsx
│       │   ├── Dashboard.tsx
│       │   ├── Peers.tsx            # Управление VLESS клиентами
│       │   ├── RoutingRules.tsx     # Правила маршрутизации
│       │   ├── Presets.tsx          # Пресеты
│       │   ├── DnsSettings.tsx      # DNS настройки
│       │   ├── Monitoring.tsx       # Мониторинг
│       │   └── Settings.tsx         # Настройки системы
│       ├── components/
│       │   ├── Layout.tsx
│       │   ├── PeerCard.tsx
│       │   ├── RuleEditor.tsx
│       │   ├── TrafficChart.tsx
│       │   ├── ServerStatus.tsx
│       │   └── QrModal.tsx
│       ├── hooks/
│       │   ├── useAuth.ts
│       │   ├── usePeers.ts
│       │   └── useRoutes.ts
│       ├── store/
│       │   └── index.ts
│       └── types/
│           └── index.ts             # TypeScript типы
│
├── landing/
│   ├── Dockerfile
│   ├── package.json
│   ├── src/
│   │   ├── pages/
│   │   │   ├── Home.tsx             # Главная
│   │   │   ├── About.tsx            # О компании
│   │   │   ├── Services.tsx         # Услуги
│   │   │   └── Contacts.tsx         # Контакты
│   │   ├── components/
│   │   └── styles/
│   └── dist/                        # Собранный статический сайт
│
└── scripts/
    ├── setup-ru.sh                  # Автоматическая настройка РФ-сервера
    ├── setup-foreign.sh             # Автоматическая настройка зарубежного
    ├── generate-keys.sh             # Генерация ключей WireGuard
    └── sync-community-lists.sh      # Обновление списков маршрутизации
```

---

## 11. Рекомендации по серверам

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

## 12. Дальнейшее развитие (Roadmap)

### v1.0 — MVP
- [x] VLESS+Reality для клиентских подключений (sing-box)
- [x] WireGuard межсерверный тоннель (РФ ↔ зарубежный)
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
