# SmartTraffic — Полный рефакторинг

> Дата: 2026-04-28
> Сложность: **СЛОЖНАЯ** (>30 файлов, затрагивает архитектуру, backend + frontend + инфраструктура)
> Маршрут конвейера: Аналитик → Архитектор → Разработчик → Тестировщик → DevOps

---

## Сводка проблем

| Блок | Критические | Высокие | Средние | Низкие | Всего |
|---|---|---|---|---|---|
| Безопасность | 2 | 3 | 2 | 0 | 7 |
| Backend (Go) | 0 | 4 | 8 | 2 | 14 |
| Frontend (React/TS) | 0 | 2 | 5 | 4 | 11 |
| Инфраструктура | 1 | 3 | 6 | 5 | 15 |
| **Итого** | **3** | **12** | **21** | **11** | **47** |

---

## БЛОК 1. Критические проблемы безопасности и CI/CD

### 1.1 [CRITICAL] Секреты в git-репозитории

**Файлы:**
- `deploy/server-ru/singbox/config.json` — приватный WG-ключ, IP зарубежного сервера, VLESS UUID, short_id
- `deploy/server-ru/wireguard/wg1.conf` — приватный WG-ключ, endpoint

**Проблема:** Файлы уже закоммичены в git. `.gitignore` добавлен позже, но не работает для уже отслеживаемых файлов.

**Решение:**
1. Заменить все секреты на placeholder-ы (`<PLACEHOLDER>`)
2. Выполнить `git rm --cached` для файлов с секретами
3. Обновить `.gitignore` чтобы файлы с реальными секретами никогда не попадали в git
4. Добавить шаблоны файлов (`.example`) для конфигураций

### 1.2 [CRITICAL] Баг CI/CD: environment всегда production-ru

**Файл:** `.github/workflows/deploy-ru.yml:109`

**Проблема:** Тернарный оператор `${{ github.ref == 'refs/heads/main' && 'production-ru' || 'production-ru' }}` всегда разрешается в `production-ru`.

**Решение:** Изменить на `'production-ru' || 'staging-ru'`

### 1.3 [CRITICAL] VITE_API_URL — runtime вместо build-time

**Файл:** `deploy/server-ru/docker-compose.prod.yml:96`

**Проблема:** `VITE_API_URL` указана как runtime-переменная в docker-compose, но Vite подставляет переменные окружения только при сборке. Значение не имеет эффекта в prod.

**Решение:** Передавать `VITE_API_URL` как build-arg в Dockerfile, либо использовать runtime-конфигурацию через `window.__CONFIG__`.

### 1.4 [HIGH] Docker socket в API-контейнере

**Файл:** `deploy/server-ru/docker-compose.prod.yml:69`

**Проблема:** `/var/run/docker.sock` примонтирован в API-контейнер — полный доступ к Docker демона хоста.

**Решение:** Убрать монтирование docker socket. API должен управлять sing-box через Clash API (HTTP), а не через Docker restart.

---

## БЛОК 2. Backend (Go) — архитектура и код

### 2.1 [HIGH] Нарушение слоистой архитектуры

**Файлы:** `handlers/dns.go`, `handlers/peers.go`, `handlers/presets.go`, `handlers/routes.go`

**Проблема:** Handlers импортируют `repository` напрямую для проверки `repository.ErrNotFound`. Нарушает правило: handler → service → repository.

**Решение:**
1. Создать `internal/errors/errors.go` с доменными ошибками: `ErrNotFound`, `ErrConflict`, `ErrValidation`
2. Services возвращают доменные ошибки вместо ошибок repository
3. Handlers проверяют доменные ошибки, а не `repository.ErrNotFound`
4. Убрать прямые импорты `repository` из handlers

### 2.2 [HIGH] Silent error swallowing

**Файлы:**
- `handlers/routes.go:55,104,125` — `_ = h.sbSvc.WriteConfigAndReload(r.Context())`
- `handlers/presets.go:56` — `_ = h.sbSvc.WriteConfigAndReload(r.Context())`
- `services/auth.go:103,117` — `_ = s.authRepo.DeleteRefreshToken(...)`, `_ = s.authRepo.StoreRefreshToken(...)`

**Проблема:** Ошибки критических операций игнорируются. Правило маршрута сохраняется в БД, но sing-box не перезагружается — пользователь видит "success" при неработающем изменении.

**Решение:**
1. Обрабатывать ошибку от `WriteConfigAndReload` — возвращать ошибку клиенту если sing-box reload не удался
2. В auth — логировать ошибку и возвращать 500 при невозможности сохранить refresh token
3. Добавить retry-механизм для sing-box reload

### 2.3 [HIGH] God-методы

**Файлы:**
- `services/wireguard.go:107-252` — `buildClientConfigMap` (145 строк, 5+ уровней вложенности)
- `services/singbox.go` — `GenerateConfig` (126 строк с глубокой вложенностью map)

**Решение:**
1. Разбить `buildClientConfigMap` на подметоды: `buildInbounds()`, `buildOutbounds()`, `buildRouteRules()`, `buildDNSConfig()`
2. Создать типизированные структуры для sing-box конфигурации вместо `map[string]any`
3. Использовать Builder pattern для генерации конфигурации

### 2.4 [HIGH] Мёртвый код

**Файлы:**
- `pkg/wgcrypto/` — весь пакет не используется
- `services/wireguard.go:289-291` — `SyncAllPeers` — no-op
- `middleware/cors.go:26-29` — `Logging` — no-op middleware
- `handlers/presets.go:14` — `presetSvc` — неиспользуемое поле

**Решение:** Удалить мёртвый код.

### 2.5 [MEDIUM] Дублирование логики

**Проблемы:**
- Извлечение `:id` из URL дублируется 6+ раз (handlers/peers.go, monitoring.go) — использовать `getPathID` из routes.go
- `containsString` в `models/route.go:66` и `containsStr` в `services/routing.go:181` — одна функция в двух местах
- `splitList`, `splitComma`, `splitString` в `services/singbox.go` reimplement `strings.Split`
- Сканирование `lastSeen` дублируется в `repository/peers.go` (GetByID, List, GetByPublicKey)

**Решение:**
1. Вынести `getPathID` в общий helper
2. Удалить дубликаты `containsString` — оставить один в models
3. Заменить кастомные split на `strings.Split` + `strings.TrimSpace`
4. Создать `scanPeer` helper в repository

### 2.6 [MEDIUM] Несогласованность нейминга

**Проблемы:**
- `WireGuardService` управляет VLESS-клиентами — ввести `PeerService` или `ClientService`
- `PresetHandler.presetSvc` и `routingSvc` указывают на один и тот же объект
- Middleware: context value дублируется через string key и typed key

**Решение:**
1. Rename `WireGuardService` → `PeerService`
2. Удалить `presetSvc` из PresetHandler
3. Использовать только typed context key в middleware, убрать string key

### 2.7 [MEDIUM] Хардкод в конфигурации

**Проблемы (частичный список):**
- 50+ доменов захардкожены в `services/wireguard.go`
- DNS серверы: `"1.1.1.1,8.8.8.8"`, MTU: `1280`, TUN address: `"172.19.0.1/30"`
- Docker container name: `"smarttraffic-singbox"`
- Rate limiter params: `1, time.Second, 5`
- QR code size: `512`
- Cleanup interval: `24 * time.Hour`, retain days: `60`

**Решение:** Вынести все в `internal/config/config.go` с дефолтными значениями.

### 2.8 [MEDIUM] Несогласованная обработка ошибок в HTTP

**Проблема:** `middleware/auth.go` использует `http.Error` (plain text), handlers используют `ErrorJSON` (JSON). Разные форматы ответов.

**Решение:** Middleware auth должен использовать `ErrorJSON` для единообразия.

### 2.9 [MEDIUM] Транзакционность ApplyPreset

**Файл:** `services/routing.go:155-156`

**Проблема:** `ApplyPreset` удаляет все правила и создаёт новые без транзакции. При частичном сбое — данные теряются.

**Решение:** Обернуть в SQL-транзакцию.

### 2.10 [MEDIUM] Неэффективные SQL-запросы

**Файл:** `repository/traffic.go:95-124`

**Проблема:** `GetTotalStats` выполняет 5 отдельных SQL-запросов вместо одного.

**Решение:** Объединить в один запрос с подзапросами.

---

## БЛОК 3. Frontend (React + TypeScript) — код и компоненты

### 3.1 [HIGH] Массовое дублирование formatBytes

**Файлы:** `Dashboard.tsx:12`, `Monitoring.tsx:11`, `Peers.tsx:29`, `TrafficChart.tsx:8`

**Решение:** Создать `src/utils/format.ts` с единственной `formatBytes`.

### 3.2 [HIGH] Дублирование извлечения токена из localStorage

**Файлы:** `QrModal.tsx:19-28,45-53`, `Peers.tsx:53-62`

**Проблема:** `localStorage.getItem('smarttraffic_tokens')` + JSON.parse дублируется, минуя `store/auth.ts` абстракцию.

**Решение:** Создать `src/utils/download.ts` с функцией `downloadWithAuth(url, filename)` которая использует `getAccessToken()` из store.

### 3.3 [MEDIUM] Monitoring.tsx — слишком большой компонент (423 строки)

**Решение:** Разбить на подкомпоненты:
- `MonitoringStats.tsx` — статистика
- `PeerCards.tsx` — карточки пиров
- `LogTable.tsx` — таблица логов
- `AlertsTable.tsx` — таблица алертов
- Вынести `logColumns`, `peerStatsColumns` за пределы компонента (memoize)

### 3.4 [MEDIUM] Баг Settings.tsx — handleLogoutAll

**Файл:** `Settings.tsx:12`

**Проблема:** Функция называется `handleLogoutAll` и кнопка подписана "Завершить все сессии", но вызывается `logout()` (одна сессия).

**Решение:** Исправить на `logoutAll()`.

### 3.5 [MEDIUM] Несогласованность нейминга навигации

**Проблема:** Меню: "WireGuard клиенты" (AppLayout.tsx), страница: "VLESS клиенты" (Peers.tsx).

**Решение:** Унифицировать на "VLESS клиенты".

### 3.6 [MEDIUM] Все стили inline

**Решение:** Вынести повторяющиеся стили в CSS модули или общий stylesheet.

### 3.7 [LOW] Мёртвый код API hooks

**Неиспользуемые экспорты:** `usePeer`, `useRuStats`, `useForeignStats`, `useDnsPresets`, `useTrafficLogs`, `getPeerStats`, `getRule`, `logoutAll`

**Решение:** Удалить или пометить как будущие функции.

---

## БЛОК 4. Инфраструктура — CI/CD, nginx, scripts

### 4.1 [HIGH] Несогласованность health-check URL

**Проблема:** Два разных пути:
- `/api/v1/health` — в `scripts/deploy.sh`, `scripts/rollback.sh`
- `/health` — в CI/CD workflows, prod docker-compose, remote-deploy scripts, nginx config

**Решение:** Унифицировать на `/api/v1/health`. Обновить: CI/CD workflows, docker-compose.prod.yml, remote-deploy scripts, nginx config.

### 4.2 [HIGH] Heredoc с отступами в deploy-foreign.yml

**Файл:** `.github/workflows/deploy-foreign.yml:74-88`

**Проблема:** WireGuard конфиг генерируется через heredoc с пробельными отступами — ведущие пробелы попадают в конфиг, WireGuard может отклонить.

**Решение:** Использовать `<<-EOF` с tab-отступами или убрать отступы.

### 4.3 [MEDIUM] Опечатка в домене sing-box

**Файл:** `deploy/server-ru/singbox/config.json:115`

**Проблема:** `positive-techologies.ru` → `positive-technologies.ru` (пропущена `n`).

**Решение:** Исправить опечатку.

### 4.4 [MEDIUM] Landing Dockerfile — nginx конфиг через echo

**Файл:** `landing/Dockerfile:16-33`

**Проблема:** 18 последовательных `echo` команд для генерации nginx конфига — нечитаемо и хрупко.

**Решение:** Создать `landing/nginx.conf` и `COPY` его в Dockerfile.

### 4.5 [MEDIUM] Duplicate mirror URLs в download-geodata.sh

**Файл:** `scripts/download-geodata.sh:6-14,7-21`

**Проблема:** Primary URL дублируется в массиве mirrors.

**Решение:** Убрать дубли.

### 4.6 [MEDIUM] DEPLOY.md — опечатка с китайскими символами

**Файл:** `DEPLOY.md:169`

**Проблема:** Смешанный русский/китайский текст: `指向 staging`.

**Решение:** Заменить на русский текст.

### 4.7 [LOW] Landing — захардкоженный год, домен

**Файлы:** Все HTML файлы landing: `&copy; 2024`, OG URL `https://infotech-solutions.ru/`

**Решение:** Использовать Vite переменные для домена, JS для года.

### 4.8 [LOW] .gitignore — ненужные Next.js записи

**Проблема:** `landing/.next/` и `landing/out/` — проект использует Vite, не Next.js.

**Решение:** Удалить.

---

## План декомпозиции (порядок выполнения)

```
Блок 1 (Критические)
├── 1.1 Секреты → placeholder + git rm --cached
├── 1.2 CI/CD bug fix
├── 1.3 VITE_API_URL fix
└── 1.4 Docker socket removal

Блок 2 (Backend)
├── 2.1 Доменные ошибки + убрать repository из handlers
├── 2.2 Обработка ошибок (sinbox reload, auth tokens)
├── 2.3 Refactor god-методов (buildClientConfig, GenerateConfig)
├── 2.4 Удаление мёртвого кода
├── 2.5 Устранение дублирования
├── 2.6 Нейминг (WireGuardService → PeerService)
├── 2.7 Конфигурация (вынести хардкод)
├── 2.8 Единообразие HTTP-ошибок
├── 2.9 Транзакционность ApplyPreset
└── 2.10 Оптимизация SQL-запросов

Блок 3 (Frontend)
├── 3.1 Утилита formatBytes
├── 3.2 Утилита downloadWithAuth
├── 3.3 Декомпозиция Monitoring.tsx
├── 3.4 Баг Settings.tsx logoutAll
├── 3.5 Унификация нейминга навигации
├── 3.6 Стили (частичный вынос)
└── 3.7 Удаление мёртвого кода API

Блок 4 (Инфраструктура)
├── 4.1 Health-check URL унификация
├── 4.2 Heredoc fix в CI/CD
├── 4.3 Опечатка в домене
├── 4.4 Landing Dockerfile refactor
├── 4.5 Duplicate mirrors fix
├── 4.6 DEPLOY.md typo fix
├── 4.7 Landing hardcoded values
└── 4.8 .gitignore cleanup
```

---

## Критерии приёмки

### Функциональные:
- [ ] Все существующие API-эндпоинты продолжают работать
- [ ] Frontend корректно взаимодействует с Backend
- [ ] Авторизация (JWT) работает без изменений
- [ ] CRUD пиров, правил, пресетов работает
- [ ] Конфигурация sing-box генерируется корректно
- [ ] Health-check доступен по единому URL

### Нефункциональные:
- [ ] В git нет секретов (ключей, паролей, IP)
- [ ] Все ошибки обрабатываются (нет `_ = err`)
- [ ] Нет дублированного кода (DRY)
- [ ] Функции не длиннее 50 строк
- [ ] Handlers не импортируют repository
- [ ] Нет мёртвого кода

### Безопасность:
- [ ] Docker socket не примонтирован
- [ ] CI/CD корректно выбирает environment
- [ ] Токены не передаются в URL query params

### Сборка:
- [ ] `go vet ./...` — пройден
- [ ] `go build ./...` — успешна
- [ ] `go test ./...` — все тесты пройдены
- [ ] `npm run lint` — пройден
- [ ] `npm run typecheck` — пройден
- [ ] `npm run build` — успешна

---

## Риски и митигация

| Риск | Вероятность | Влияние | Митигация |
|---|---|---|---|
| Рефакторинг god-методов ломает генерацию конфига | Средняя | Высокое | Полное тестирование существующими тестами + ручная проверка JSON |
| Rename WireGuardService ломает импорты | Низкая | Среднее | Поэтапный rename через IDE с компиляцией после каждого шага |
| Удаление docker socket ломает restart sing-box | Средняя | Высокое | Заменить на Clash API reload (уже есть метод `reloadViaClashAPI`) |
| Удаление мёртвого кода ломает что-то неожиданное | Низкая | Низкое | grep по всей кодовой базе перед удалением |
| Frontend декомпозиция ломает рендеринг | Средняя | Среднее | Тесты на каждый компонент + ручная проверка |
