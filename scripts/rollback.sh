#!/usr/bin/env bash
set -euo pipefail

REMOTE_HOST="${REMOTE_HOST:-}"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_PATH="${REMOTE_PATH:-/opt/smarttraffic}"
SSH_KEY="${SSH_KEY:-~/.ssh/id_rsa}"
SSH_OPTS="-o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 -i ${SSH_KEY}"
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/api/v1/health}"
HEALTH_RETRIES="${HEALTH_RETRIES:-12}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${BLUE}[ROLLBACK]${NC} $*"; }
ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; }

usage() {
    cat <<EOF
SmartTraffic — Rollback Script
================================

Usage: $0 [OPTIONS]

Options:
  --host HOST         Remote VPS hostname/IP (required or set REMOTE_HOST)
  --user USER         SSH user (default: root)
  --path PATH         Remote project path (default: /opt/smarttraffic)
  --key KEY           SSH private key path (default: ~/.ssh/id_rsa)
  --tag TAG           Specific deploy tag to rollback to (default: previous)
  --db-only           Rollback only database, not code
  --code-only         Rollback only code, not database
  --list              List available backups and deploy history
  --health-url URL    Health check URL (default: http://localhost:8080/api/v1/health)
  --yes               Skip confirmation prompt
  --dry-run           Show what would be done without executing
  -h, --help          Show this help

Examples:
  $0 --host 1.2.3.4
  $0 --host 1.2.3.4 --tag deploy_20260101_120000
  $0 --host 1.2.3.4 --list
  $0 --host 1.2.3.4 --db-only

EOF
    exit 0
}

TARGET_TAG=""
DB_ONLY=false
CODE_ONLY=false
LIST=false
SKIP_CONFIRM=false
DRY_RUN=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --host)        REMOTE_HOST="$2"; shift 2 ;;
        --user)        REMOTE_USER="$2"; shift 2 ;;
        --path)        REMOTE_PATH="$2"; shift 2 ;;
        --key)         SSH_KEY="$2"; shift 2 ;;
        --tag)         TARGET_TAG="$2"; shift 2 ;;
        --db-only)     DB_ONLY=true; shift ;;
        --code-only)   CODE_ONLY=true; shift ;;
        --list)        LIST=true; shift ;;
        --health-url)  HEALTH_URL="$2"; shift 2 ;;
        --yes)         SKIP_CONFIRM=true; shift ;;
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

if [[ -z "${REMOTE_HOST}" ]]; then
    err "REMOTE_HOST не указан. Используйте --host или установите REMOTE_HOST"
    exit 1
fi

if [[ ! -f "${SSH_KEY}" ]]; then
    err "SSH ключ не найден: ${SSH_KEY}"
    exit 1
fi

# ─────────────────────────────────────────────
# LIST MODE
# ─────────────────────────────────────────────

if [[ "${LIST}" == "true" ]]; then
    echo ""
    echo -e "${BLUE}История деплоев:${NC}"
    ssh_cmd "cat ${REMOTE_PATH}/.deploy-history 2>/dev/null" || warn "История деплоев не найдена"
    echo ""
    echo -e "${BLUE}Доступные бэкапы БД:${NC}"
    ssh_cmd "ls -lh ${REMOTE_PATH}/backups/db_*.sqlite.bak 2>/dev/null" || warn "Бэкапы БД не найдены"
    echo ""
    echo -e "${BLUE}Текущий деплой:${NC}"
    ssh_cmd "cat ${REMOTE_PATH}/.deploy-current-tag 2>/dev/null" || warn "Тег текущего деплоя не найден"
    echo ""
    echo -e "${BLUE}Предыдущий деплой:${NC}"
    ssh_cmd "cat ${REMOTE_PATH}/.deploy-previous-tag 2>/dev/null" || warn "Тег предыдущего деплоя не найден"
    exit 0
fi

# ─────────────────────────────────────────────
# CONFIRM
# ─────────────────────────────────────────────

CURRENT_TAG=$(ssh_cmd "cat ${REMOTE_PATH}/.deploy-current-tag 2>/dev/null" || echo "unknown")
PREV_TAG=$(ssh_cmd "cat ${REMOTE_PATH}/.deploy-previous-tag 2>/dev/null" || echo "unknown")

if [[ -z "${TARGET_TAG}" ]]; then
    TARGET_TAG="${PREV_TAG}"
fi

echo ""
echo -e "${YELLOW}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║         ВНИМАНИЕ: ОТКАТ НА ПРОДАКШЕНЕ              ║${NC}"
echo -e "${YELLOW}╠══════════════════════════════════════════════════════╣${NC}"
echo -e "${YELLOW}║                                                      ║${NC}"
echo -e "${YELLOW}║  Текущая версия:  ${CURRENT_TAG}"
echo -e "${YELLOW}║  Откат к:         ${TARGET_TAG}"
echo -e "${YELLOW}║  Сервер:          ${REMOTE_USER}@${REMOTE_HOST}"
echo -e "${YELLOW}║  Rollback DB:     $(${DB_ONLY} && echo 'YES' || ${CODE_ONLY} && echo 'NO' || echo 'YES')"
echo -e "${YELLOW}║  Rollback Code:   $(${CODE_ONLY} && echo 'YES' || ${DB_ONLY} && echo 'NO' || echo 'YES')"
echo -e "${YELLOW}║                                                      ║${NC}"
echo -e "${YELLOW}╚══════════════════════════════════════════════════════╝${NC}"
echo ""

if [[ "${SKIP_CONFIRM}" == "false" ]]; then
    read -rp "Подтвердите откат [y/N]: " CONFIRM
    if [[ "${CONFIRM}" != "y" && "${CONFIRM}" != "Y" ]]; then
        log "Откат отменён"
        exit 0
    fi
fi

# ─────────────────────────────────────────────
# ROLLBACK DATABASE
# ─────────────────────────────────────────────

if [[ "${CODE_ONLY}" == "false" ]]; then
    log "Откат базы данных..."

    LATEST_BACKUP=$(ssh_cmd "ls -1t ${REMOTE_PATH}/backups/db_*.sqlite.bak 2>/dev/null | head -1" || echo "")
    if [[ -z "${LATEST_BACKUP}" ]]; then
        warn "Бэкап БД не найден. Пропускаю откат БД."
    else
        log "Восстанавливаю из: ${LATEST_BACKUP}"

        if ! $DRY_RUN; then
            ssh_cmd "cp ${REMOTE_PATH}/data/smarttraffic.db ${REMOTE_PATH}/data/smarttraffic.db.pre-rollback-\$(date +%Y%m%d_%H%M%S) 2>/dev/null || true"
            ssh_cmd "cp ${LATEST_BACKUP} ${REMOTE_PATH}/data/smarttraffic.db"
            ok "БД восстановлена"
        else
            echo -e "${YELLOW}[DRY-RUN]${NC} cp ${LATEST_BACKUP} → ${REMOTE_PATH}/data/smarttraffic.db"
        fi
    fi
fi

# ─────────────────────────────────────────────
# ROLLBACK CODE
# ─────────────────────────────────────────────

if [[ "${DB_ONLY}" == "false" ]]; then
    log "Откат кода к: ${TARGET_TAG}..."

    if ! $DRY_RUN; then
        ssh_cmd "cd ${REMOTE_PATH} && git fetch --all && git checkout ${TARGET_TAG}"
        ok "Код откаччен к ${TARGET_TAG}"
    else
        echo -e "${YELLOW}[DRY-RUN]${NC} git checkout ${TARGET_TAG}"
    fi
fi

# ─────────────────────────────────────────────
# RESTART SERVICES
# ─────────────────────────────────────────────

log "Пересобираю и перезапускаю сервисы..."

if ! $DRY_RUN; then
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml down --timeout 30" || true
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml build --no-cache" 2>&1 | tail -10
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml up -d --remove-orphans" 2>&1
else
    echo -e "${YELLOW}[DRY-RUN]${NC} docker compose down + build + up"
fi
ok "Сервисы перезапущены"

# ─────────────────────────────────────────────
# HEALTH CHECK
# ─────────────────────────────────────────────

log "Проверяю здоровье после отката..."

HEALTHY=false
for i in $(seq 1 "${HEALTH_RETRIES}"); do
    if ! $DRY_RUN; then
        RESPONSE=$(ssh_cmd "curl -sf -o /dev/null -w '%{http_code}' ${HEALTH_URL} 2>/dev/null" || echo "000")
        if [[ "${RESPONSE}" == "200" ]]; then
            HEALTHY=true
            ok "Health check пройден (попытка ${i}/${HEALTH_RETRIES})"
            break
        fi
        log "Попытка ${i}/${HEALTH_RETRIES} — HTTP ${RESPONSE}"
        sleep "${HEALTH_INTERVAL}"
    else
        HEALTHY=true
        ok "[DRY-RUN] Health check был бы пройден"
        break
    fi
done

if [[ "${HEALTHY}" == "false" ]]; then
    err "Health check НЕ пройден после отката!"
    log "Логи:"
    ssh_cmd "cd ${REMOTE_PATH}/deploy/server-ru && docker compose -f docker-compose.prod.yml logs --tail=50" 2>&1 || true
    err "ТРЕБУЕТСЯ РУЧНОЕ ВМЕШАТЕЛЬСТВО!"
    exit 1
fi

# ─────────────────────────────────────────────
# UPDATE TAGS
# ─────────────────────────────────────────────

if ! $DRY_RUN; then
    ssh_cmd "echo '${CURRENT_TAG}' > ${REMOTE_PATH}/.deploy-previous-tag"
    ssh_cmd "echo '${TARGET_TAG}' > ${REMOTE_PATH}/.deploy-current-tag"
fi

echo ""
echo -e "${GREEN}┌──────────────────────────────────────────────────┐${NC}"
echo -e "${GREEN}│  ROLLBACK УСПЕШНО ВЫПОЛНЕН                       │${NC}"
echo -e "${GREEN}│                                                  │${NC}"
echo -e "${GREEN}│  Откат к:  ${TARGET_TAG}${NC}"
echo -e "${GREEN}│  Сервер:   ${REMOTE_USER}@${REMOTE_HOST}${NC}"
echo -e "${GREEN}└──────────────────────────────────────────────────┘${NC}"
echo ""
