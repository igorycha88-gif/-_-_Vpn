# SmartTraffic — CI/CD Pipeline

> Автоматизированный деплой на РФ-сервер и зарубежный сервер через GitHub Actions.

---

## Архитектура CI/CD

```
┌─────────────────────────────────────────────────────────────┐
│                    GITHUB ACTIONS                           │
│                                                             │
│  Push/PR ──▶ ci.yml                                        │
│              ├── Backend: go vet, test, build               │
│              ├── Frontend: lint, typecheck, test, build     │
│              ├── Landing: build                             │
│              ├── Docker: validate compose, build images     │
│              └── Security: hardcoded secrets check          │
│                          │                                  │
│  Push to main/dev1 ──────┼──▶ deploy-ru.yml                │
│                          │    ├── CI Check (ci.yml)         │
│                          │    ├── Git pull on server        │
│                          │    ├── Write .env from secrets   │
│                          │    ├── Remote deploy script      │
│                          │    │   ├── Backup DB             │
│                          │    │   ├── Build images          │
│                          │    │   ├── Deploy containers     │
│                          │    │   ├── Health check          │
│                          │    │   └── Auto-rollback on fail │
│                          │    └── Post-deploy verification  │
│                          │                                  │
│  Manual trigger ─────────┼──▶ deploy-foreign.yml           │
│                          │    ├── Generate WG config        │
│                          │    ├── Upload to server          │
│                          │    ├── Restart WireGuard         │
│                          │    └── Verify tunnel             │
│                          │                                  │
└─────────────────────────────────────────────────────────────┘
```

---

## Workflow-ы

### 1. CI (`ci.yml`)

**Триггер:** push в любую ветку, pull_request в main/dev1

**Jobs:**
| Job | Что проверяет |
|---|---|
| Backend | `go vet`, `go test`, `go build` |
| Frontend | `npm lint`, `typecheck`, `test`, `build` |
| Landing | `npm build` |
| Docker | Валидация compose-файлов, сборка образов |
| Security | Поиск захардкоженных секретов, проверка `.gitignore` |

### 2. Deploy RU (`deploy-ru.yml`)

**Триггер:** push в main/dev1 (после успешного CI), ручной запуск

**Environment:** `production-ru` (требует подтверждения если настроено)

**Процесс:**
1. Ожидает успешного прохождения CI
2. Подключается по SSH к РФ-серверу
3. Обновляет код (`git fetch` + `git reset`)
4. Записывает production `.env` из GitHub Secrets
5. Запускает `remote-deploy-ru.sh` на сервере
6. Проверяет здоровье после деплоя

**Автоматический откат:** при провале health check система автоматически:
- Восстанавливает БД из последнего бэкапа
- Откатывает код к предыдущему коммиту
- Пересобирает и запускает предыдущую версию

### 3. Deploy Foreign (`deploy-foreign.yml`)

**Триггер:** только ручной запуск (workflow_dispatch)

**Процесс:**
1. Генерирует `wg0.conf` из GitHub Secrets (ключи не хранятся в репо)
2. Бэкапит текущий конфиг на сервере
3. Загружает новый конфиг через SCP
4. Перезапускает WireGuard
5. Проверяет тоннель (ping до РФ-сервера)

---

## Настройка GitHub Secrets

### Обязательные секреты для РФ-сервера

| Secret | Описание | Пример |
|---|---|---|
| `RU_SERVER_HOST` | IP или домен РФ-сервера | `203.0.113.10` |
| `RU_SERVER_USER` | SSH пользователь | `root` или `deploy` |
| `RU_SERVER_PATH` | Путь к проекту на сервере | `/opt/smarttraffic` |
| `RU_SSH_PRIVATE_KEY` | SSH приватный ключ | `-----BEGIN OPENSSH PRIVATE KEY-----...` |
| `RU_SSH_HOST_KEY` | (Опционально) Хост-ключ сервера | `server ssh-ed25519 AAAA...` |
| `RU_PRODUCTION_ENV` | Полное содержимое `.env` для production | Содержимое файла `.env` |

### Обязательные секреты для зарубежного сервера

| Secret | Описание |
|---|---|
| `FOREIGN_SERVER_HOST` | IP зарубежного сервера |
| `FOREIGN_SERVER_USER` | SSH пользователь |
| `FOREIGN_SSH_PRIVATE_KEY` | SSH приватный ключ |
| `FOREIGN_SSH_HOST_KEY` | (Опционально) Хост-ключ |
| `FOREIGN_WG_PRIVATE_KEY` | Приватный ключ WireGuard |
| `FOREIGN_WG_ADDRESS` | Адрес WG интерфейса (`10.20.0.2/30`) |
| `FOREIGN_WG_LISTEN_PORT` | Порт WG (`51821`) |
| `RU_WG_PUBLIC_KEY` | Публичный ключ РФ-сервера для тоннеля |
| `SINGBOX_WG_PUBLIC_KEY` | Публичный ключ sing-box outbound |
| `RU_TUNNEL_IP` | IP тоннеля РФ-сервера (`10.20.0.1`) |

---

## Настройка GitHub Environments

Создайте в Settings → Environments:

### `production-ru`
- **Protection rules:**
  - Required reviewers: добавить себя/команду (для подтверждения деплоя)
  - Deployment branch: `main` и `dev1`
- **Environment secrets:** все `RU_*` секреты

### `production-foreign`
- **Protection rules:**
  - Required reviewers: добавить себя/команду
- **Environment secrets:** все `FOREIGN_*` секреты

---

## Генерация SSH-ключей для деплоя

```bash
ssh-keygen -t ed25519 -C "github-deploy-ru" -f ~/.ssh/deploy-ru

# Публичный ключ → на сервер в ~/.ssh/authorized_keys
ssh-copy-id -i ~/.ssh/deploy-ru.pub root@<RU_SERVER_IP>

# Приватный ключ → в GitHub Secrets (RU_SSH_PRIVATE_KEY)
cat ~/.ssh/deploy-ru

# Хост-ключ → в GitHub Secrets (RU_SSH_HOST_KEY)
ssh-keyscan -H <RU_SERVER_IP>
```

Повторить для зарубежного сервера.

---

## Подготовка серверов к CI/CD

### РФ-сервер

```bash
# 1. Первоначальная настройка
bash scripts/setup-ru.sh

# 2. Клонирование репозитория
git clone <repo-url> /opt/smarttraffic
cd /opt/smarttraffic

# 3. Создание директорий
mkdir -p data backups

# 4. Первый деплой вручную
cp deploy/server-ru/.env.deploy.example deploy/server-ru/.env
# Заполните .env реальными значениями
docker compose -f deploy/server-ru/docker-compose.prod.yml up -d
```

### Зарубежный сервер

```bash
# 1. Первоначальная настройка
bash scripts/setup-foreign.sh <RU_SERVER_IP>

# 2. WireGuard настроится через CI/CD
# Или вручную: скопируйте wg0.conf в /etc/wireguard/
```

---

## Ручной деплой (альтернатива CI/CD)

```bash
# Деплой на РФ-сервер
make deploy-ru REMOTE_HOST=1.2.3.4 DEPLOY_BRANCH=dev1

# Или напрямую скриптом
bash scripts/deploy.sh --host 1.2.3.4 --branch dev1

# Откат
make rollback-ru REMOTE_HOST=1.2.3.4

# Деплой на зарубежный сервер
bash scripts/deploy.sh --host <FOREIGN_IP> --skip-tests
```

---

## Безопасность

### Меры защиты

1. **Секреты только в GitHub Secrets** — ключи, пароли, токены никогда не попадают в код
2. **WireGuard конфиг генерируется на лету** — ключи не хранятся в репозитории
3. **Environment protection** — требуется подтверждение для production деплоя
4. **SSH с верификацией хоста** — защита от MITM
5. **Автоматический откат** — при провале health check система откатывается
6. **Lock файл** — предотвращает параллельные деплои
7. **Security check в CI** — автоматический поиск захардкоженных секретов
8. **Минимальные права** — deploy-пользователь с ограниченными правами

### Рекомендации по SSH-ключам

- Использовать **ed25519** (современный, быстрый, безопасный)
- Один ключ на сервер (не переиспользовать)
- Добавить passphrase к ключу (опционально для CI/CD)
- Регулярно ротировать ключи (каждые 90 дней)

### Рекомендации по серверу

- Создать отдельного пользователя `deploy` с минимальными правами
- Добавить в группу `docker` для управления контейнерами
- Настроить `sudoers` только для необходимых команд
- Отключить SSH вход по паролю (`PasswordAuthentication no`)

---

## Мониторинг деплоев

### История деплоев

На РФ-сервере:
```bash
cat /opt/smarttraffic/.deploy-history
cat /opt/smarttraffic/.deploy-current-tag
cat /opt/smarttraffic/.deploy-previous-tag
```

### Бэкапы БД

```bash
ls -lh /opt/smarttraffic/backups/
```

### Логи GitHub Actions

Все деплои логируются в GitHub Actions → Actions tab.
Каждый запуск содержит полный лог: от CI до verification.
