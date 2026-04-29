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

## КОНВЕЙЕР РАЗРАБОТКИ — ОБЯЗАТЕЛЕН ДЛЯ КАЖДОЙ ЗАДАЧИ

**Каждая задача, поступающая от пользователя, ОБЯЗАНА пройти полный конвейер.**

```
Аналитик → [Архитектор] → Разработчик → Тестировщик → [цикл баг-фикс] → DevOps
```

### Требование: Конвейер всегда запускается

1. **Любая задача** от пользователя автоматически запускает конвейер
2. **Пропуск этапов ЗАПРЕЩЁН** — каждый этап выполняется полностью
3. **Хардгейты** — каждый этап имеет автоматические проверки-ворота; непрохождение = остановка
4. **Эскалация** — если что-то не получается или нужно отклониться от конвейера → ОБЯЗАТЕЛЬНО спросить пользователя перед действием

### Требование: Тесты — всегда, для всего нового функционала

1. **TDD-first** — тесты пишутся ДО реализации кода
2. **Покрытие ≥ 80%** — для нового кода, проверяется автоматически
3. **Все тесты проходят** — `make verify` = 0 ошибок, 0 failing тестов
4. **Баг-фикс = failing test** — каждый баг сначала воспроизводится тестом, потом фиксится

### Требование: Пересборка и запуск локально

1. **DevOps этап** — полная пересборка Docker: `docker compose down && docker compose build --no-cache && docker compose up -d`
2. **Все тесты в Docker** — после сборки прогоняются все тесты
3. **Smoke-проверка** — все контейнеры Up, логи без ошибок, API отвечает

---

## Навыки (обязательны к исполнению):

- `.opencode/skills/analyst.md` — Аналитик
- `.opencode/skills/architect.md` — Архитектор (для средних/сложных задач)
- `.opencode/skills/developer.md` — Разработчик
- `.opencode/skills/tester.md` — Тестировщик
- `.opencode/skills/devops.md` — DevOps

---

## Правила работы

1. **Конвейер обязателен** — каждая задача проходит полный конвейер, пропуск этапов запрещён
2. **Все навыки обязательны** — каждая роль следует всем практикам из своего файла навыка
3. **Читать навык перед работой** — перед выполнением роли прочитать соответствующий навык
4. **Контекст проекта** — читать TZ.md, ARCHITECTURE.md, PIPELINE.md, DEPLOY.md перед началом
5. **Язык** — вся коммуникация на русском языке
6. **Без комментариев** — никаких комментариев в коде (кроме явного запроса)
7. **Без секретов** — никогда не коммитить пароли, ключи, токены
8. **Тесты обязательны** — для нового кода: TDD, покрытие ≥ 80%, все тесты зелёные
9. **Эскалация к пользователю** — если:
   - Что-то не получается на любом этапе конвейера
   - Нужно отклониться от требований конвейера
   - Требования противоречивы или неполны
   - Есть несколько вариантов решения с разными компромиссами
   → **ОСТАНОВИТЬСЯ и спросить пользователя** прежде чем действовать

---

## Единая команда проверки: `make verify`

Запускается на каждом этапе передачи между ролями:

```bash
make verify
```

Включает:
- Backend: `go vet` + `golangci-lint run` + `go test -race -coverprofile=coverage.out ./...` + покрытие ≥ 80%
- Frontend: `npm run lint` + `npm run typecheck` + `npm run test -- --coverage --passWithNoTests`
- Build: `go build ./...` + `npm run build`

**Если `make verify` падает — задача НЕ передаётся дальше.**

---

## Команды проверки (детально)

### Backend (Go)
```bash
go vet ./...
golangci-lint run
go test -race -count=1 -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total
go build ./...
```

### Frontend (React + TypeScript)
```bash
npm run lint
npm run typecheck
npm run test -- --coverage --passWithNoTests
npm run build
```

### Docker (DevOps этап)
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
CI (lint+test+build+security+coverage gate)
    → Pre-flight (disk, lock, connectivity)
    → Snapshot (backup DB, save current commit)
    → Deploy (build images, restart containers)
    → Canary (30 сек наблюдение, health check)
    → Smoke (containers up, no errors)
    → Done / Auto-rollback
```

### CI/CD Workflows:
- `.github/workflows/ci.yml` — CI проверки (включая coverage gate)
- `.github/workflows/deploy-ru.yml` — деплой на РФ-сервер
- `.github/workflows/deploy-foreign.yml` — деплой на зарубежный сервер
- `.github/workflows/rollback.yml` — ручной откат

### Команды деплоя (ручные):
```bash
gh workflow run deploy-ru.yml
gh workflow run deploy-foreign.yml
gh workflow run rollback.yml -f server=ru -f target=previous
gh workflow run rollback.yml -f server=foreign
```
