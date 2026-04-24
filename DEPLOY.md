# SmartTraffic — Конвейер безопасного деплоя

> Безопасный деплой на два сервера (РФ + зарубежный) с автоматическим откатом,
> canary-проверками, мануальным approval и zero-downtime стратегией.

---

## 1. Визуальная схема конвейера

```
                          PUSH / MERGE
                               │
                    ┌──────────┴──────────┐
                    │                     │
              branch == main        branch == dev1
                    │                     │
                    ▼                     ▼
            ┌──────────────┐      ┌──────────────┐
            │   CI Check   │      │   CI Check   │
            │  (lint+test  │      │  (lint+test  │
            │   +build)    │      │   +build)    │
            └──────┬───────┘      └──────┬───────┘
                   │                     │
                   ▼                     ▼
            ┌──────────────┐      ┌──────────────┐
            │   Security   │      │   Security   │
            │    Scan      │      │    Scan      │
            └──────┬───────┘      └──────┬───────┘
                   │                     │
                   ▼                     ▼
            ┌──────────────┐      ┌──────────────┐
            │   Manual     │      │   Auto-deploy│
            │  Approval    │      │   (dev1)     │
            │  (prod)      │      └──────┬───────┘
            └──────┬───────┘             │
                   │                     ▼
                   ▼              ┌──────────────┐
            ┌──────────────┐     │  Deploy RU   │
            │  Deploy RU   │     │  (staging)   │
            │  (prod)      │     └──────┬───────┘
            └──────┬───────┘             │
                   │                     ▼
                   ▼              ┌──────────────┐
            ┌──────────────┐     │  Health +    │
            │  Pre-deploy  │     │  Smoke test  │
            │  Snapshot    │     └──────┬───────┘
            └──────┬───────┘             │
                   │               ┌─────┴─────┐
                   ▼               │           │
            ┌──────────────┐    OK ▼        FAIL│
            │  Deploy RU   │  ┌────────┐  ┌──────────┐
            │  Containers  │  │  Done  │  │ Rollback │
            └──────┬───────┘  └────────┘  └──────────┘
                   │
                   ▼
            ┌──────────────┐
            │  Canary      │
            │  Health Check│
            │  (30 сек)    │
            └──────┬───────┘
                   │
              ┌────┴────┐
              │         │
           OK ▼      FAIL│
         ┌────────┐ ┌──────────┐
         │  Done  │ │ Rollback │
         └───┬────┘ │  auto    │
             │      └──────────┘
             ▼
      ┌──────────────┐
      │  Deploy      │
      │  Foreign WG  │
      │  (manual)    │
      └──────┬───────┘
             │
             ▼
      ┌──────────────┐
      │  Verify      │
      │  Tunnel      │
      └──────┬───────┘
             │
        ┌────┴────┐
        │         │
     OK ▼      FAIL│
   ┌────────┐ ┌──────────┐
   │  Done  │ │ WG Reset │
   └────────┘ │ + Notify │
              └──────────┘
```

---

## 2. Стратегия деплоя

### 2.1. Российский сервер (RU)

| Параметр | Значение |
|---|---|
| **Стратегия** | Rolling update с health-gate |
| **Downtime** | ~30 сек (пересборка + рестарт контейнеров) |
| **Откат** | Автоматический при failed health check |
| **Approval** | Требуется для `main` (production) |
| **Триггер** | `push` в `main` / `dev1`, или `workflow_dispatch` |

### 2.2. Зарубежный сервер (Foreign)

| Параметр | Значение |
|---|---|
| **Стратегия** | Blue-Green для WireGuard |
| **Downtime** | ~5 сек (restart WireGuard) |
| **Откат** | Автоматический (предыдущий конфиг) |
| **Approval** | Всегда ручной (`workflow_dispatch`) |
| **Триггер** | Только `workflow_dispatch` |

---

## 3. GitHub Environments и Protection Rules

### 3.1. Настройка Environments

В репозитории **Settings → Environments** создать:

#### `production-ru` (РФ-сервер, продакшен)

```
Protection rules:
  ☑ Required reviewers         — 1 (тимлид / DevOps)
  ☑ Wait timer                 — 2 минуты (остыл перед деплоем)
  ☑ Branch                     — main
  ☐ Deployment branch          — all branches (для dev1)

Secrets:
  RU_SERVER_HOST               — IP или домен РФ-сервера
  RU_SERVER_USER               — SSH пользователь (deploy)
  RU_SERVER_PATH               — /opt/smarttraffic
  RU_SSH_PRIVATE_KEY           — SSH ключ (ed25519)
  RU_SSH_HOST_KEY              — ssh-keyscan ключ
  RU_PRODUCTION_ENV            — содержимое .env (base64)
```

#### `production-foreign` (Зарубежный сервер)

```
Protection rules:
  ☑ Required reviewers         — 1 (тимлид / DevOps)
  ☑ Wait timer                 — 1 минута

Secrets:
  FOREIGN_SERVER_HOST          — IP зарубежного сервера
  FOREIGN_SERVER_USER          — SSH пользователь (root)
  FOREIGN_SSH_PRIVATE_KEY      — SSH ключ
  FOREIGN_SSH_HOST_KEY         — ssh-keyscan ключ
  FOREIGN_WG_PRIVATE_KEY       — Приватный ключ WG
  FOREIGN_WG_ADDRESS           — 10.20.0.2/30
  FOREIGN_WG_LISTEN_PORT       — 51821
  RU_WG_PUBLIC_KEY             — Публичный ключ WG (РФ сторона)
  RU_SERVER_HOST               — IP РФ-сервера
  SINGBOX_WG_PUBLIC_KEY        — Публичный ключ sing-box WG
  RU_TUNNEL_IP                 — 10.20.0.1
```

#### `staging-ru` (РФ-сервер, staging/dev1)

```
Protection rules:
  ☐ Required reviewers         — не требуется (автодеплой)

Secrets:
  (те же что и production-ru, но指向 staging)
```

---

## 4. Workflow: Деплой на РФ-сервер

### 4.1. Триггеры

```yaml
on:
  push:
    branches: [main, dev1]
    paths-ignore: ['docs/**', '**.md', '.opencode/**']
  workflow_dispatch:
    inputs:
      skip_smoke:
        description: 'Пропустить smoke-тесты'
        type: boolean
        default: false
      force_rollback:
        description: 'Принудительный откат к предыдущей версии'
        type: boolean
        default: false
```

### 4.2. Этапы (Jobs)

```
ci (lint+test+build+security)
        │
        ▼
pre-deploy (snapshot DB, lock, pre-flight checks)
        │
        ▼
deploy (build images, restart containers)
        │
        ▼
canary (health check, smoke tests, 30s observation)
        │
   ┌────┴────┐
   │         │
OK ▼      FAIL│
 done     rollback → notify
```

### 4.3. Rollback — автоматический

Условия автоматического отката:
1. Health check не пройден за N попыток (по умолчанию 15 × 5 сек = 75 сек)
2. Smoke-тесты вернули критическую ошибку
3. Контейнер упал (restart loop) в течение canary-периода
4. Ручной триггер `force_rollback: true`

### 4.4. Rollback — ручной

Отдельный workflow `rollback.yml` для ручного отката:

```yaml
on:
  workflow_dispatch:
    inputs:
      target:
        description: 'Номер деплоя или "previous"'
        required: false
        default: 'previous'
      server:
        description: 'Сервер'
        type: choice
        options: [ru, foreign]
```

---

## 5. Workflow: Деплой на зарубежный сервер

### 5.1. Триггеры

Только `workflow_dispatch` — **всегда ручной**.

### 5.2. Blue-Green стратегия для WireGuard

```
Текущий: /etc/wireguard/wg0.conf (ACTIVE)
Новый:   /etc/wireguard/wg0.conf.new (PENDING)

1. Бэкап wg0.conf → wg0_YYYYMMDD_HHMMSS.conf.bak
2. Загрузить wg0.conf.new
3. Валидация: формат + нет плейсхолдеров
4. wg0.conf.new → wg0.conf (atomic replace)
5. wg-quick down wg0 && wg-quick up wg0
6. Проверка: ping до RU-сервера через тоннель
7. FAIL → восстановить wg0.conf.bak → wg-quick up wg0
```

---

## 6. Безопасность конвейера

### 6.1. Минимум привилегий

| Мера | Описание |
|---|---|
| SSH deploy key | ed25519, только для деплоя, без passphrase в CI |
| SSH host key | Предварительно сохранён в secrets (MITM защита) |
| Cleanup | SSH ключ удаляется в `if: always()` |
| No secrets in logs | `echo "::add-mask::${SECRET}"` |
| Timeout | Все SSH команды с `-o ConnectTimeout=15` |

### 6.2. Protection Rules

| Правило | RU Prod | RU Staging | Foreign |
|---|---|---|---|
| Required reviewers | 1 | Нет | 1 |
| Wait timer | 2 мин | 0 | 1 мин |
| Branch restriction | main | dev1 | — |

### 6.3. Аудит

Каждый деплой логирует в `.deploy-history`:

```
deploy_20260424_153000|abc1234|2026-04-24_15:30:00
deploy_20260424_160000|def5678|2026-04-24_16:00:00
```

---

## 7. Rollback — процедуры

### 7.1. Автоматический откат (RU)

```
1. Восстановить БД из бэкапа
2. git checkout к предыдущему коммиту
3. Пересобрать Docker-образы
4. docker compose up -d
5. Health check
6. Если откат тоже провален → ALERT (ручное вмешательство)
```

### 7.2. Ручной откат (RU)

```bash
# На сервере:
cd /opt/smarttraffic
cat .deploy-history          # посмотреть историю
cat .deploy-previous-tag     # предыдущий тег

# Откат к конкретному коммиту:
git checkout <commit-sha>
docker compose -f deploy/server-ru/docker-compose.prod.yml down
docker compose -f deploy/server-ru/docker-compose.prod.yml build --no-cache
docker compose -f deploy/server-ru/docker-compose.prod.yml up -d
```

### 7.3. Откат зарубежного сервера

```bash
# На зарубежном сервере:
ls -lt /etc/wireguard/backups/    # список бэкапов
cp /etc/wireguard/backups/wg0_YYYYMMDD_HHMMSS.conf.bak /etc/wireguard/wg0.conf
wg-quick down wg0 && wg-quick up wg0
wg show wg0
```

### 7.4. Emergency: полный откат обоих серверов

1. Откатить RU: `gh workflow run rollback.yml -f server=ru -f target=previous`
2. Откатить Foreign: `gh workflow run rollback.yml -f server=foreign`
3. Проверить тоннель: `ssh ru-server "ping -c 3 10.20.0.2"`
4. Проверить клиентские подключения

---

## 8. Мониторинг после диплоя

### 8.1. Автоматические проверки (post-deploy)

| Проверка | Таймаут | Критичность |
|---|---|---|
| API `/health` → 200 | 75 сек (15 × 5) | Critical — триггерит откат |
| Все контейнеры `Up` | 30 сек | Critical — триггерит откат |
| Нет panic/fatal в логах | 10 сек | Warning |
| WireGuard handshake | 15 сек | Warning |
| Landing page → 200 | 10 сек | Warning |

### 8.2. Ручные проверки (опционально)

```bash
# Проверить разделение трафика:
curl --vless-client <client-config> https://ya.ru    # → direct
curl --vless-client <client-config> https://google.com  # → proxy

# Проверить WG тоннель:
ssh ru-server "ping -c 3 10.20.0.2"

# Проверить SSL:
curl -vI https://your-domain.com
```

---

## 9. Структура файлов диплоя

```
.github/workflows/
├── ci.yml                    # CI: lint, test, build, security
├── deploy-ru.yml             # Деплой на РФ-сервер (v2)
├── deploy-foreign.yml        # Деплой на зарубежный сервер (v2)
└── rollback.yml              # Ручной откат (RU или Foreign)

deploy/
├── server-ru/
│   ├── docker-compose.prod.yml
│   ├── .env.deploy.example
│   ├── scripts/
│   │   └── remote-deploy-ru.sh    # Деплой-скрипт (v2)
│   ├── nginx/conf.d/
│   ├── singbox/
│   └── wireguard/
└── server-foreign/
    └── scripts/
        └── remote-deploy-foreign.sh  # Деплой-скрипт (v2)

scripts/
├── setup-ru.sh
├── setup-foreign.sh
└── generate-keys.sh
```

---

## 10. Чеклист перед первым деплоем

### На стороне GitHub

- [ ] Создать Environment `production-ru` с protection rules
- [ ] Создать Environment `production-foreign` с protection rules
- [ ] Добавить все секреты (SSH ключи, IP, ключи WG)
- [ ] Проверить `RU_PRODUCTION_ENV` — закодированное содержимое .env

### На стороне серверов

- [ ] RU: установлен Docker, Docker Compose, Git, WireGuard
- [ ] RU: `/opt/smarttraffic` — клонирован репозиторий
- [ ] RU: `deploy/server-ru/.env` — заполнен
- [ ] RU: iptables правила настроены
- [ ] Foreign: установлен WireGuard, `net.ipv4.ip_forward=1`
- [ ] Foreign: `/etc/wireguard/` — директория существует
- [ ] SSH: deploy ключ добавлен в `authorized_keys` на обоих серверах

### Smoke-тест перед первым автодеплоем

```bash
# Мануально проверить SSH подключение:
ssh -i ~/.ssh/deploy_key deploy@<ru-server> "echo OK"
ssh -i ~/.ssh/deploy_key root@<foreign-server> "echo OK"

# Мануально проверить деплой-скрипт:
ssh deploy@<ru-server> "cd /opt/smarttraffic && \
  DEPLOY_PATH=/opt/smarttraffic \
  bash deploy/server-ru/scripts/remote-deploy-ru.sh"
```
