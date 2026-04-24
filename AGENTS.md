# SmartTraffic — Инструкции для AI-агентов

> Этот файл описывает как AI-агенты (opencode и подобные) должны работать с проектом.

---

## Контекст проекта

Система управления сетевым трафиком: рунет напрямую, остальной мир — через зарубежный прокси.

### Ключевые документы (обязательно к прочтению):
- `TZ.md` — техническое задание, этапы работ
- `ARCHITECTURE.md` — архитектура системы, компоненты, схемы
- `PIPELINE.md` — конвейер разработки, роли, правила передачи
- `DEPLOY.md` — конвейер безопасного деплоя, откаты, CI/CD

### Стек технологий:
- **Backend**: Go 1.22+, Chi Router, SQLite
- **Frontend**: React 18, TypeScript, Ant Design, Vite, React Query
- **Routing**: sing-box
- **VPN**: WireGuard
- **Reverse Proxy**: Nginx + Let's Encrypt
- **Контейнеризация**: Docker + Docker Compose
- **ОС серверов**: Ubuntu 22.04/24.04 LTS

---

## Конвейер разработки

Задачи проходят через обязательный конвейер:

```
Аналитик → [Архитектор] → Разработчик → Тестировщик → [цикл баг-фикс] → DevOps
```

**Навыки (обязательны к исполнению):**
- `.opencode/skills/analyst.md` — Аналитик
- `.opencode/skills/architect.md` — Архитектор (для средних/сложных задач)
- `.opencode/skills/developer.md` — Разработчик
- `.opencode/skills/tester.md` — Тестировщик
- `.opencode/skills/devops.md` — DevOps

---

## Правила работы

1. **Все навыки обязательны** — каждая роль следует всем практикам из своего файла навыка
2. **Читать навык перед работой** — перед выполнением роли прочитать соответствующий навык
3. **Контекст проекта** — читать TZ.md, ARCHITECTURE.md, PIPELINE.md, DEPLOY.md перед началом
4. **Язык** — вся коммуникация на русском языке
5. **Без комментариев** — никаких комментариев в коде (кроме явного запроса)
6. **Без секретов** — никогда не коммитить пароли, ключи, токены
7. **Линтинг и типизация** — обязательно запускать перед передачей следующей роли
8. **Тесты** — покрытие ≥ 80% для нового кода

---

## Команды проверки

### Backend (Go)
```bash
go vet ./...
golangci-lint run
go test ./...
go build ./...
```

### Frontend (React + TypeScript)
```bash
npm run lint
npm run typecheck
npm run test
npm run build
```

### Docker
```bash
docker compose down
docker compose build --no-cache
docker compose up -d
docker compose ps
docker compose logs --tail=100 [service]
```

---

## Определение сложности задачи

| Критерий | Простая | Средняя | Сложная |
|---|---|---|---|
| Количество файлов | 1–3 | 4–10 | >10 |
| Новая функциональность | Нет | Частично | Да |
| Затрагивает архитектуру | Нет | Косвенно | Да |

**Маршрутизация:**
- Простая → Аналитик → Разработчик
- Средняя/Сложная → Аналитик → Архитектор → Разработчик

---

## Конвейер деплоя

Полное описание в `DEPLOY.md`. Краткая схема:

```
CI (lint+test+build+security)
    → Pre-flight (disk, lock, connectivity)
    → Snapshot (backup DB, save current commit)
    → Deploy (build images, restart containers)
    → Canary (30 сек наблюдение, health check)
    → Smoke (containers up, no errors)
    → Done / Auto-rollback
```

### CI/CD Workflows:
- `.github/workflows/ci.yml` — CI проверки
- `.github/workflows/deploy-ru.yml` — деплой на РФ-сервер
- `.github/workflows/deploy-foreign.yml` — деплой на зарубежный сервер
- `.github/workflows/rollback.yml` — ручной откат

### Команды деплоя (ручные):
```bash
# Деплой на RU (через GitHub Actions):
gh workflow run deploy-ru.yml

# Деплой на Foreign:
gh workflow run deploy-foreign.yml

# Откат RU:
gh workflow run rollback.yml -f server=ru -f target=previous

# Откат Foreign:
gh workflow run rollback.yml -f server=foreign
```
