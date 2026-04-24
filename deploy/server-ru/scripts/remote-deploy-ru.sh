#!/usr/bin/env bash
set -euo pipefail

DEPLOY_PATH="${DEPLOY_PATH:-/opt/smarttraffic}"
COMPOSE_FILE="deploy/server-ru/docker-compose.prod.yml"
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/health}"
HEALTH_RETRIES="${HEALTH_RETRIES:-15}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"
BACKUP_KEEP="${BACKUP_KEEP:-5}"
SKIP_HEALTH_CHECK="${SKIP_HEALTH_CHECK:-false}"
LOCK_FILE="/tmp/smarttraffic-deploy.lock"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
TAG="deploy_${TIMESTAMP}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${BLUE}[DEPLOY]${NC} $(date '+%H:%M:%S') $*"; }
ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; }

step() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

cleanup_lock() {
    rm -f "${LOCK_FILE}" 2>/dev/null || true
}

rollback_and_exit() {
    err "ДЕПЛОЙ ПРОВАЛЕН. Запускаю откат..."
    echo ""

    LATEST_BACKUP=$(ls -1t "${DEPLOY_PATH}/backups/db_"*.sqlite.bak 2>/dev/null | head -1 || true)
    if [[ -n "${LATEST_BACKUP}" ]]; then
        log "Восстанавливаю БД из: ${LATEST_BACKUP}"
        cp "${LATEST_BACKUP}" "${DEPLOY_PATH}/data/smarttraffic.db" 2>/dev/null || true
    fi

    PREVIOUS_TAG=$(cat "${DEPLOY_PATH}/.deploy-previous-tag" 2>/dev/null || true)
    if [[ -n "${PREVIOUS_TAG}" ]]; then
        log "Откат к коммиту: ${PREVIOUS_TAG}"
        cd "${DEPLOY_PATH}"
        git checkout "${PREVIOUS_TAG}" 2>/dev/null || true
    fi

    log "Пересборка и запуск предыдущей версии..."
    cd "${DEPLOY_PATH}"
    docker compose -f "${COMPOSE_FILE}" down --timeout 30 2>/dev/null || true
    docker compose -f "${COMPOSE_FILE}" build --no-cache 2>&1 | tail -5
    docker compose -f "${COMPOSE_FILE}" up -d --remove-orphans 2>&1

    sleep 5
    HTTP=$(curl -sf -o /dev/null -w '%{http_code}' "${HEALTH_URL}" 2>/dev/null || echo "000")
    if [[ "${HTTP}" == "200" ]]; then
        warn "Откат выполнен. Сервисы работают из предыдущей версии."
    else
        err "ОТКАТ ТАКЖЕ ПРОВАЛЕН! Требуется ручное вмешательство!"
    fi

    cleanup_lock
    exit 1
}

# ═══════════════════════════════════
# LOCK
# ═══════════════════════════════════

if [[ -f "${LOCK_FILE}" ]]; then
    LOCK_PID=$(cat "${LOCK_FILE}" 2>/dev/null || echo "")
    if [[ -n "${LOCK_PID}" ]] && kill -0 "${LOCK_PID}" 2>/dev/null; then
        err "Деплой заблокирован другим процессом (PID: ${LOCK_PID})"
        exit 1
    fi
    warn "Устаревшая блокировка удалена"
    rm -f "${LOCK_FILE}"
fi

echo $$ > "${LOCK_FILE}"
trap cleanup_lock EXIT

cd "${DEPLOY_PATH}"

COMMIT_SHA=$(git rev-parse --short HEAD)
COMMIT_MSG=$(git log -1 --format='%s')
log "Коммит: ${COMMIT_SHA} — ${COMMIT_MSG}"

# ═══════════════════════════════════
# STEP 1: BACKUP
# ═══════════════════════════════════

step "ШАГ 1: Бэкап"

mkdir -p "${DEPLOY_PATH}/backups"
mkdir -p "${DEPLOY_PATH}/data"

if [[ -f "${DEPLOY_PATH}/data/smarttraffic.db" ]]; then
    cp "${DEPLOY_PATH}/data/smarttraffic.db" "${DEPLOY_PATH}/backups/db_${TIMESTAMP}.sqlite.bak"
    ok "БД забэкаплена: backups/db_${TIMESTAMP}.sqlite.bak"
else
    warn "Файл БД не найден — пропускаю бэкап"
fi

PREV_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
echo "${PREV_COMMIT}" > "${DEPLOY_PATH}/.deploy-previous-tag"
ok "Previous commit: ${PREV_COMMIT}"

ls -1t "${DEPLOY_PATH}/backups/db_"*.sqlite.bak 2>/dev/null | tail -n +"$((BACKUP_KEEP+1))" | xargs -r rm -f
ok "Старые бэкапы очищены (оставляю ${BACKUP_KEEP})"

# ═══════════════════════════════════
# STEP 2: BUILD
# ═══════════════════════════════════

step "ШАГ 2: Сборка Docker-образов"

log "Собираю образы (no-cache)..."
docker compose -f "${COMPOSE_FILE}" build --no-cache 2>&1 | tail -20
ok "Образы собраны"

# ═══════════════════════════════════
# STEP 3: DEPLOY
# ═══════════════════════════════════

step "ШАГ 3: Развёртывание"

log "Останавливаю текущие сервисы..."
docker compose -f "${COMPOSE_FILE}" down --timeout 30 2>/dev/null || true
ok "Сервисы остановлены"

log "Запускаю новые сервисы..."
docker compose -f "${COMPOSE_FILE}" up -d --remove-orphans 2>&1
ok "Сервисы запущены"

# ═══════════════════════════════════
# STEP 4: HEALTH CHECK
# ═══════════════════════════════════

if [[ "${SKIP_HEALTH_CHECK}" != "true" ]]; then
    step "ШАГ 4: Health check"

    HEALTHY=false
    for i in $(seq 1 "${HEALTH_RETRIES}"); do
        RESPONSE=$(curl -sf -o /dev/null -w '%{http_code}' "${HEALTH_URL}" 2>/dev/null || echo "000")
        if [[ "${RESPONSE}" == "200" ]]; then
            HEALTHY=true
            ok "Health check пройден (попытка ${i}/${HEALTH_RETRIES}) — HTTP ${RESPONSE}"
            break
        else
            log "Попытка ${i}/${HEALTH_RETRIES} — HTTP ${RESPONSE}"
            sleep "${HEALTH_INTERVAL}"
        fi
    done

    if [[ "${HEALTHY}" == "false" ]]; then
        err "Health check НЕ пройден после ${HEALTH_RETRIES} попыток!"
        log "Логи:"
        docker compose -f "${COMPOSE_FILE}" logs --tail=30 2>&1 || true
        rollback_and_exit
    fi
else
    warn "Health check пропущен (SKIP_HEALTH_CHECK=true)"
fi

# ═══════════════════════════════════
# STEP 5: SMOKE TESTS
# ═══════════════════════════════════

step "ШАГ 5: Smoke-тесты"

CONTAINERS=$(docker compose -f "${COMPOSE_FILE}" ps --format '{{.Name}} {{.Status}}' 2>/dev/null || true)
log "Контейнеры:"
echo "${CONTAINERS}" | while IFS= read -r line; do
    if [[ -n "${line}" ]]; then
        if echo "${line}" | grep -q "Up"; then
            ok "${line}"
        else
            warn "${line}"
        fi
    fi
done

ERR_COUNT=$(docker compose -f "${COMPOSE_FILE}" logs --tail=50 2>&1 | grep -ci 'error\|fatal\|panic' || echo "0")
if [[ "${ERR_COUNT}" -gt 0 ]]; then
    warn "Найдено ${ERR_COUNT} строк с error/fatal/panic в логах"
else
    ok "Критических ошибок в логах нет"
fi

# ═══════════════════════════════════
# STEP 6: CLEANUP
# ═══════════════════════════════════

step "ШАГ 6: Очистка"

docker image prune -f 2>/dev/null || true
ok "Неиспользуемые образы удалены"

docker builder prune -f --filter 'until=24h' 2>/dev/null || true
ok "Build cache очищен"

# ═══════════════════════════════════
# SAVE DEPLOY STATE
# ═══════════════════════════════════

echo "${TAG}|${COMMIT_SHA}|${TIMESTAMP}" >> "${DEPLOY_PATH}/.deploy-history"
echo "${TAG}" > "${DEPLOY_PATH}/.deploy-current-tag"

# ═══════════════════════════════════
# SUMMARY
# ═══════════════════════════════════

step "ДЕПЛОЙ УСПЕШНО ЗАВЕРШЁН"

echo -e "${GREEN}"
cat <<EOF
  ┌──────────────────────────────────────────────────┐
  │                                                  │
  │   SMARTTRAFFIC DEPLOYED SUCCESSFULLY            │
  │                                                  │
  │   Commit:  ${COMMIT_SHA} — ${COMMIT_MSG}
  │   Tag:     ${TAG}
  │   Path:    ${DEPLOY_PATH}
  │                                                  │
  └──────────────────────────────────────────────────┘
EOF
echo -e "${NC}"
