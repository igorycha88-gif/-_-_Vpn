#!/usr/bin/env bash
set -euo pipefail

DEPLOY_BRANCH="${DEPLOY_BRANCH:-dev1}"
REMOTE_HOST="${REMOTE_HOST:-}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_PATH="${REMOTE_PATH:-/opt/smarttraffic}"
SSH_KEY="${SSH_KEY:-~/.ssh/id_rsa}"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -i ${SSH_KEY}"
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/api/v1/health}"
HEALTH_RETRIES="${HEALTH_RETRIES:-12}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"
BACKUP_KEEP="${BACKUP_KEEP:-5}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TAG="deploy_${TIMESTAMP}"
LOCK_FILE="/tmp/smarttraffic-deploy.lock"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${BLUE}[DEPLOY]${NC} $*"; }
ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; }

usage() {
    cat <<EOF
SmartTraffic — Safe Deploy Pipeline
====================================

Usage: $0 [OPTIONS]

Options:
  --host HOST         Remote VPS hostname/IP (required or set REMOTE_HOST)
  --user USER         SSH user (default: root)
  --path PATH         Remote project path (default: /opt/smarttraffic)
  --key KEY           SSH private key path (default: ~/.ssh/id_rsa)
  --branch BRANCH     Git branch to deploy (default: dev1)
  --skip-tests        Skip local tests before deploy
  --skip-backup       Skip DB backup before deploy
  --force             Force deploy even if lock exists
  --health-url URL    Health check URL (default: http://localhost:8080/api/v1/health)
  --dry-run           Show what would be done without executing
  -h, --help          Show this help

Examples:
  $0 --host 1.2.3.4
  $0 --host 1.2.3.4 --user deploy --branch dev1
  $0 --host 1.2.3.4 --skip-tests --dry-run

Environment variables (override options):
  REMOTE_HOST, REMOTE_USER, REMOTE_PATH, SSH_KEY,
  DEPLOY_BRANCH, HEALTH_URL, HEALTH_RETRIES, HEALTH_INTERVAL

EOF
    exit 0
}

SKIP_TESTS=false
SKIP_BACKUP=false
FORCE=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --host)        REMOTE_HOST="$2"; shift 2 ;;
        --user)        REMOTE_USER="$2"; shift 2 ;;
        --path)        REMOTE_PATH="$2"; shift 2 ;;
        --key)         SSH_KEY="$2"; shift 2 ;;
        --branch)      DEPLOY_BRANCH="$2"; shift 2 ;;
        --skip-tests)  SKIP_TESTS=true; shift ;;
        --skip-backup) SKIP_BACKUP=true; shift ;;
        --force)       FORCE=true; shift ;;
        --health-url)  HEALTH_URL="$2"; shift 2 ;;
        --dry-run)     DRY_RUN=true; shift ;;
        -h|--help)     usage ;;
        *)             err "Unknown option: $1"; usage ;;
    esac
done

ssh_cmd() {
    if $DRY_RUN; then
        echo -e "${YELLOW}[DRY-RUN SSH]${NC} ssh ${SSH_OPTS} ${REMOTE_USER}@${REMOTE_HOST} $*"
        return 0
    fi
    ssh ${SSH_OPTS} "${REMOTE_USER}@${REMOTE_HOST}" "$@"
}

scp_cmd() {
    if $DRY_RUN; then
        echo -e "${YELLOW}[DRY-RUN SCP]${NC} scp ${SSH_OPTS} $1 ${REMOTE_USER}@${REMOTE_HOST}:$2"
        return 0
    fi
    scp ${SSH_OPTS} "$1" "${REMOTE_USER}@${REMOTE_HOST}:$2"
}

step_header() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

cleanup_lock() {
    if ! $DRY_RUN; then
        ssh_cmd "rm -f ${LOCK_FILE}" 2>/dev/null || true
    fi
}

rollback_and_exit() {
    err "ДЕПЛОЙ ЗАВЕРШИЛСЯ С ОШИБКОЙ. Запускаю откат..."
    echo ""

    if ! $DRY_RUN; then
        LATEST_BACKUP=$(ssh_cmd "ls -1t ${REMOTE_PATH}/backups/db_*.sqlite.bak 2>/dev/null | head -1" 2>/dev/null || echo "")
        if [[ -n "${LATEST_BACKUP}" ]]; then
            log "Восстанавливаю БД из: ${LATEST_BACKUP}"
            ssh_cmd "cp ${LATEST_BACKUP} ${REMOTE_PATH}/data/smarttraffic.db" || true
        fi

        PREVIOUS_TAG=$(ssh_cmd "cat ${REMOTE_PATH}/.deploy-previous-tag 2>/dev/null" || echo "")
        if [[ -n "${PREVIOUS_TAG}" ]]; then
            log "Переключаюсь на предыдущий релиз: ${PREVIOUS_TAG}"
            ssh_cmd "cd ${REMOTE_PATH} && git checkout ${PREVIOUS_TAG}" || true
        fi

        log "Перезапускаю сервисы из предыдущей версии..."
        ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml down && docker compose -f docker-compose.prod.yml up -d" || true

        log "Проверяю здоровье после отката..."
        sleep 5
        if ssh_cmd "curl -sf ${HEALTH_URL} > /dev/null 2>&1"; then
            warn "Откат выполнен. Сервисы работают из предыдущей версии."
        else
            err "ОТКАТ ТАКЖЕ ЗАВЕРШИЛСЯ С ОШИБКОЙ! Требуется ручное вмешательство!"
        fi
    else
        warn "[DRY-RUN] Был бы запущен откат к предыдущей версии"
    fi

    cleanup_lock
    exit 1
}

# ─────────────────────────────────────────────
# VALIDATION
# ─────────────────────────────────────────────

step_header "ШАГ 0: Валидация параметров"

if [[ -z "${REMOTE_HOST}" ]]; then
    err "REMOTE_HOST не указан. Используйте --host или установите REMOTE_HOST"
    exit 1
fi
ok "Remote host: ${REMOTE_HOST}"

if [[ ! -f "${SSH_KEY}" ]]; then
    err "SSH ключ не найден: ${SSH_KEY}"
    exit 1
fi
ok "SSH key: ${SSH_KEY}"

log "Проверяю SSH подключение..."
if ! ssh_cmd "echo 'SSH OK'" > /dev/null 2>&1; then
    err "Не удалось подключиться по SSH к ${REMOTE_USER}@${REMOTE_HOST}"
    exit 1
fi
ok "SSH подключение установлено"

if ! $FORCE; then
    log "Проверяю блокировку деплоя..."
    LOCKED=$(ssh_cmd "cat ${LOCK_FILE} 2>/dev/null" || echo "")
    if [[ -n "${LOCKED}" ]]; then
        err "Деплой заблокирован. PID: ${LOCKED}"
        err "Используйте --force или удалите ${LOCK_FILE} на сервере"
        exit 1
    fi
fi

ssh_cmd "echo \$\$ > ${LOCK_FILE}"
trap cleanup_lock EXIT

# ─────────────────────────────────────────────
# STEP 1: LOCAL PRE-DEPLOY CHECKS
# ─────────────────────────────────────────────

step_header "ШАГ 1: Локальные проверки (pre-deploy)"

CURRENT_BRANCH=$(git branch --show-current)
if [[ "${CURRENT_BRANCH}" != "${DEPLOY_BRANCH}" ]]; then
    err "Текущая ветка: ${CURRENT_BRANCH}, ожидается: ${DEPLOY_BRANCH}"
    err "Переключитесь: git checkout ${DEPLOY_BRANCH}"
    exit 1
fi
ok "Текущая ветка: ${DEPLOY_BRANCH}"

UNCOMMITTED=$(git status --porcelain)
if [[ -n "${UNCOMMITTED}" ]]; then
    err "Есть незакоммиченные изменения:"
    echo "${UNCOMMITTED}"
    err "Закоммитьте или спрячьте изменения перед деплоем"
    exit 1
fi
ok "Нет незакоммиченных изменений"

LOCAL_AHEAD=$(git log --oneline origin/${DEPLOY_BRANCH}..HEAD 2>/dev/null | wc -l | tr -d ' ')
if [[ "${LOCAL_AHEAD}" -gt 0 ]]; then
    log "Пушу ${LOCAL_AHEAD} коммитов в origin/${DEPLOY_BRANCH}..."
    if $DRY_RUN; then
        echo -e "${YELLOW}[DRY-RUN]${NC} git push origin ${DEPLOY_BRANCH}"
    else
        git push origin "${DEPLOY_BRANCH}"
    fi
    ok "Изменения запушены"
else
    ok "Ветка актуальна относительно remote"
fi

if [[ "${SKIP_TESTS}" == "false" ]]; then
    log "Запускаю линтинг backend..."
    if ! $DRY_RUN; then
        (cd backend && go vet ./...) || { err "go vet не прошёл"; exit 1; }
    fi
    ok "Backend линтинг пройден"

    log "Запускаю линтинг frontend..."
    if ! $DRY_RUN; then
        (cd frontend && npm run lint 2>&1) || { err "frontend lint не прошёл"; exit 1; }
        (cd frontend && npm run typecheck 2>&1) || { err "frontend typecheck не прошёл"; exit 1; }
    fi
    ok "Frontend линтинг + типизация пройдены"

    log "Запускаю тесты backend..."
    if ! $DRY_RUN; then
        (cd backend && go test ./... 2>&1) || { err "backend тесты не прошли"; exit 1; }
    fi
    ok "Backend тесты пройдены"

    log "Запускаю тесты frontend..."
    if ! $DRY_RUN; then
        (cd frontend && npm run test 2>&1) || { err "frontend тесты не прошли"; exit 1; }
    fi
    ok "Frontend тесты пройдены"

    log "Проверяю сборку backend..."
    if ! $DRY_RUN; then
        (cd backend && go build ./... ) || { err "backend build не прошёл"; exit 1; }
    fi
    ok "Backend сборка пройдена"

    log "Проверяю сборку frontend..."
    if ! $DRY_RUN; then
        (cd frontend && npm run build 2>&1) || { err "frontend build не прошёл"; exit 1; }
    fi
    ok "Frontend сборка пройдена"
else
    warn "Тесты и линтинг пропущены (--skip-tests)"
fi

COMMIT_SHA=$(git rev-parse --short HEAD)
COMMIT_MSG=$(git log -1 --format='%s')
ok "Deploying commit: ${COMMIT_SHA} — ${COMMIT_MSG}"

# ─────────────────────────────────────────────
# STEP 2: BACKUP ON REMOTE
# ─────────────────────────────────────────────

step_header "ШАГ 2: Бэкап на прод-сервере"

if [[ "${SKIP_BACKUP}" == "false" ]]; then
    log "Создаю директорию для бэкапов..."
    ssh_cmd "mkdir -p ${REMOTE_PATH}/backups"

    DB_EXISTS=$(ssh_cmd "test -f ${REMOTE_PATH}/data/smarttraffic.db && echo YES || echo NO")
    if [[ "${DB_EXISTS}" == *"YES"* ]]; then
        log "Бэкаплю SQLite базу данных..."
        ssh_cmd "cp ${REMOTE_PATH}/data/smarttraffic.db ${REMOTE_PATH}/backups/db_${TIMESTAMP}.sqlite.bak"
        ok "БД забэкаплена: backups/db_${TIMESTAMP}.sqlite.bak"
    else
        warn "Файл БД не найден — пропускаю бэкап БД"
    fi

    log "Сохраняю текущий git-тег для отката..."
    PREV_COMMIT=$(ssh_cmd "cd ${REMOTE_PATH} && git rev-parse --short HEAD 2>/dev/null" || echo "unknown")
    ssh_cmd "echo '${PREV_COMMIT}' > ${REMOTE_PATH}/.deploy-previous-tag"
    ssh_cmd "echo '${PREV_COMMIT}' > ${REMOTE_PATH}/.deploy-previous-image-tag"
    ok "Previous tag: ${PREV_COMMIT}"

    PREV_DEPLOY=$(ssh_cmd "cat ${REMOTE_PATH}/.deploy-current-tag 2>/dev/null" || echo "none")
    if [[ "${PREV_DEPLOY}" != "none" ]]; then
        ok "Предыдущий деплой: ${PREV_DEPLOY}"
    fi

    log "Очищаю старые бэкапы (оставляю ${BACKUP_KEEP})..."
    ssh_cmd "cd ${REMOTE_PATH}/backups && ls -1t db_*.sqlite.bak 2>/dev/null | tail -n +\$((${BACKUP_KEEP}+1)) | xargs -r rm -f"
    ok "Старые бэкапы очищены"
else
    warn "Бэкап пропущен (--skip-backup)"
fi

# ─────────────────────────────────────────────
# STEP 3: GIT PULL ON REMOTE
# ─────────────────────────────────────────────

step_header "ШАГ 3: Обновление кода на прод-сервере"

log "Проверяю репозиторий на сервере..."
REPO_EXISTS=$(ssh_cmd "test -d ${REMOTE_PATH}/.git && echo YES || echo NO")
if [[ "${REPO_EXISTS}" != *"YES"* ]]; then
    err "Репозиторий не найден на ${REMOTE_PATH}. Выполните первоначальную настройку."
    exit 1
fi

log "Подтягиваю изменения из origin/${DEPLOY_BRANCH}..."
ssh_cmd "cd ${REMOTE_PATH} && git fetch origin ${DEPLOY_BRANCH} && git reset --hard origin/${DEPLOY_BRANCH}"
REMOTE_SHA=$(ssh_cmd "cd ${REMOTE_PATH} && git rev-parse --short HEAD")
ok "Код обновлён: ${REMOTE_SHA}"

log "Обновляю submodules (если есть)..."
ssh_cmd "cd ${REMOTE_PATH} && git submodule update --init --recursive 2>/dev/null || true"

ssh_cmd "echo '${TAG}|${COMMIT_SHA}|${TIMESTAMP}' >> ${REMOTE_PATH}/.deploy-history"
ssh_cmd "echo '${TAG}' > ${REMOTE_PATH}/.deploy-current-tag"
ok "Тег деплоя: ${TAG}"

# ─────────────────────────────────────────────
# STEP 4: BUILD ON REMOTE
# ─────────────────────────────────────────────

step_header "ШАГ 4: Сборка Docker-образов на прод-сервере"

log "Собираю образы (no-cache)..."
if ! $DRY_RUN; then
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml build --no-cache" 2>&1 | tail -20
else
    echo -e "${YELLOW}[DRY-RUN]${NC} docker compose -f docker-compose.prod.yml build --no-cache"
fi
ok "Образы собраны"

log "Тегирую образы для отката..."
if ! $DRY_RUN; then
    for svc in api frontend landing; do
        IMG=$(ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml images -q ${svc} 2>/dev/null" | head -1)
        if [[ -n "${IMG}" ]]; then
            ssh_cmd "docker tag \$(docker inspect --format='{{.RepoTags}}' ${IMG} 2>/dev/null | grep -o '[^ ]*' | head -1) smarttraffic-${svc}:${TAG} 2>/dev/null || docker tag ${IMG} smarttraffic-${svc}:${TAG} 2>/dev/null || true"
        fi
    done
fi
ok "Образы тегированы: ${TAG}"

# ─────────────────────────────────────────────
# STEP 5: DEPLOY (BLUE-GREEN STRATEGY)
# ─────────────────────────────────────────────

step_header "ШАГ 5: Развёртывание на прод"

log "Останавливаю текущие сервисы..."
if ! $DRY_RUN; then
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml down --timeout 30" || true
fi
ok "Сервисы остановлены"

log "Копирую .env если отсутствует..."
if ! $DRY_RUN; then
    ssh_cmd "test -f ${REMOTE_PATH}/deploy/server-ru/.env || (cp ${REMOTE_PATH}/deploy/server-ru/.env.deploy.example ${REMOTE_PATH}/deploy/server-ru/.env 2>/dev/null; echo '.env создан из примера') || true"
fi

log "Запускаю новые сервисы..."
if ! $DRY_RUN; then
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml up -d --remove-orphans" 2>&1
else
    echo -e "${YELLOW}[DRY-RUN]${NC} docker compose -f docker-compose.prod.yml up -d"
fi
ok "Сервисы запущены"

# ─────────────────────────────────────────────
# STEP 6: HEALTH CHECK
# ─────────────────────────────────────────────

step_header "ШАГ 6: Проверка здоровья (health check)"

log "Ожидаю ответа от API (попыток: ${HEALTH_RETRIES}, интервал: ${HEALTH_INTERVAL}s)..."

HEALTHY=false
for i in $(seq 1 "${HEALTH_RETRIES}"); do
    if ! $DRY_RUN; then
        RESPONSE=$(ssh_cmd "curl -sf -o /dev/null -w '%{http_code}' ${HEALTH_URL} 2>/dev/null" || echo "000")
        if [[ "${RESPONSE}" == "200" ]]; then
            HEALTHY=true
            ok "Health check пройден (попытка ${i}/${HEALTH_RETRIES}) — HTTP ${RESPONSE}"
            break
        else
            log "Попытка ${i}/${HEALTH_RETRIES} — HTTP ${RESPONSE}"
            sleep "${HEALTH_INTERVAL}"
        fi
    else
        HEALTHY=true
        ok "[DRY-RUN] Health check был бы пройден"
        break
    fi
done

if [[ "${HEALTHY}" == "false" ]]; then
    err "Health check НЕ пройден после ${HEALTH_RETRIES} попыток!"
    log "Логи сервисов:"
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml logs --tail=30" 2>&1 || true
    rollback_and_exit
fi

# ─────────────────────────────────────────────
# STEP 7: SMOKE TESTS
# ─────────────────────────────────────────────

step_header "ШАГ 7: Smoke-тесты"

log "Проверяю статус контейнеров..."
if ! $DRY_RUN; then
    CONTAINERS=$(ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml ps --format '{{.Name}} {{.Status}}'")
    echo "${CONTAINERS}"

    UNHEALTHY=$(echo "${CONTAINERS}" | grep -v "Up" | grep -v "^$" || true)
    if [[ -n "${UNHEALTHY}" ]]; then
        warn "Некоторые контейнеры не запущены:"
        echo "${UNHEALTHY}"
    fi
fi
ok "Статус контейнеров проверен"

log "Проверяю Nginx → Backend связку..."
if ! $DRY_RUN; then
    API_STATUS=$(ssh_cmd "curl -sf -o /dev/null -w '%{http_code}' http://localhost/api/v1/health 2>/dev/null" || echo "000")
    if [[ "${API_STATUS}" == "200" ]]; then
        ok "Nginx → Backend: HTTP ${API_STATUS}"
    else
        warn "Nginx → Backend: HTTP ${API_STATUS} (может быть нормой если Nginx на :80 без SSL)"
    fi
fi

log "Проверяю Landing Page..."
if ! $DRY_RUN; then
    LANDING_STATUS=$(ssh_cmd "curl -sf -o /dev/null -w '%{http_code}' http://localhost/ 2>/dev/null" || echo "000")
    ok "Landing: HTTP ${LANDING_STATUS}"
fi

log "Проверяю логи на ошибки..."
if ! $DRY_RUN; then
    ERR_COUNT=$(ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml logs --tail=50 2>&1 | grep -ci 'error\|fatal\|panic'" || echo "0")
    if [[ "${ERR_COUNT}" -gt 0 ]]; then
        warn "Найдено ${ERR_COUNT} строк с error/fatal/panic в логах"
    else
        ok "Критических ошибок в логах не обнаружено"
    fi
fi

# ─────────────────────────────────────────────
# STEP 8: CLEANUP
# ─────────────────────────────────────────────

step_header "ШАГ 8: Очистка"

log "Удаляю неиспользуемые Docker-образы..."
if ! $DRY_RUN; then
    ssh_cmd "docker image prune -f 2>/dev/null || true"
fi
ok "Неиспользуемые образы удалены"

log "Очищаю Docker build cache..."
if ! $DRY_RUN; then
    ssh_cmd "docker builder prune -f --filter 'until=24h' 2>/dev/null || true"
fi
ok "Build cache очищен"

# ─────────────────────────────────────────────
# SUMMARY
# ─────────────────────────────────────────────

step_header "ДЕПЛОЙ УСПЕШНО ЗАВЕРШЁН"

echo -e "${GREEN}"
cat <<EOF
  ┌──────────────────────────────────────────────────┐
  │                                                  │
  │   SMARTTRAFFIC DEPLOYED SUCCESSFULLY            │
  │                                                  │
  │   Branch:  ${DEPLOY_BRANCH}
  │   Commit:  ${COMMIT_SHA} — ${COMMIT_MSG}
  │   Tag:     ${TAG}
  │   Host:    ${REMOTE_USER}@${REMOTE_HOST}
  │   Path:    ${REMOTE_PATH}
  │                                                  │
  │   Rollback: make rollback                        │
  │             OR: scripts/rollback.sh --host ${REMOTE_HOST}
  │                                                  │
  └──────────────────────────────────────────────────┘
EOF
echo -e "${NC}"

cleanup_lock
trap - EXIT
