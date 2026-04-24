#!/usr/bin/env bash
set -euo pipefail

RU_SERVER_IP="${1:-}"
WG_CONFIG_FILE="${WG_CONFIG_FILE:-/etc/wireguard/wg0.conf}"
BACKUP_DIR="/etc/wireguard/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log()  { echo -e "${BLUE}[FOREIGN-DEPLOY]${NC} $*"; }
ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[ERROR]${NC} $*" >&2; }

if [[ "$(id -u)" -ne 0 ]]; then
    err "Запустите от root или через sudo"
    exit 1
fi

step() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

rollback_wg() {
    err "Откат конфигурации WireGuard..."
    local LATEST_BACKUP
    LATEST_BACKUP=$(ls -1t "${BACKUP_DIR}/wg0_"*.conf.bak 2>/dev/null | head -1 || true)
    if [[ -n "${LATEST_BACKUP}" ]]; then
        log "Восстанавливаю: ${LATEST_BACKUP}"
        cp "${LATEST_BACKUP}" "${WG_CONFIG_FILE}"
        chmod 600 "${WG_CONFIG_FILE}"
        wg-quick down wg0 2>/dev/null || true
        if wg-quick up wg0; then
            warn "Откат выполнен. WireGuard запущен с предыдущим конфигом."
        else
            err "ОТКАТ ПРОВАЛЕН! Требуется ручное вмешательство!"
        fi
    else
        err "Нет бэкапов для отката!"
    fi
    exit 1
}

# ═══════════════════════════════════
# STEP 0: PRE-FLIGHT
# ═══════════════════════════════════

step "ШАГ 0: Pre-flight проверки"

if ! command -v wg-quick &>/dev/null; then
    err "WireGuard не установлен"
    exit 1
fi
ok "WireGuard доступен"

FREE_GB=$(df -BG /etc/wireguard | tail -1 | awk '{print $4}' | tr -d 'G')
log "Свободное место: ${FREE_GB} GB"
if [[ "${FREE_GB}" -lt 1 ]]; then
    err "Мало свободного места на диске (< 1 GB)"
    exit 1
fi
ok "Свободное место: ${FREE_GB} GB"

if [[ "$(sysctl -n net.ipv4.ip_forward 2>/dev/null || echo 0)" != "1" ]]; then
    warn "net.ipv4.ip_forward не включён — NAT может не работать"
fi

# ═══════════════════════════════════
# STEP 1: BACKUP
# ═══════════════════════════════════

step "ШАГ 1: Бэкап текущего конфига"

mkdir -p "${BACKUP_DIR}"

if [[ -f "${WG_CONFIG_FILE}" ]]; then
    cp "${WG_CONFIG_FILE}" "${BACKUP_DIR}/wg0_${TIMESTAMP}.conf.bak"
    ok "Конфиг забэкаплен: ${BACKUP_DIR}/wg0_${TIMESTAMP}.conf.bak"

    ls -1t "${BACKUP_DIR}/wg0_"*.conf.bak 2>/dev/null | tail -n +6 | xargs -r rm -f
    ok "Старые бэкапы очищены (оставляю 5)"
else
    warn "Текущий конфиг не найден — первый запуск"
fi

# ═══════════════════════════════════
# STEP 2: VALIDATE CONFIG
# ═══════════════════════════════════

step "ШАГ 2: Валидация конфига"

if [[ ! -f "${WG_CONFIG_FILE}" ]]; then
    err "Конфиг ${WG_CONFIG_FILE} не найден"
    exit 1
fi

if ! grep -q '^\[Interface\]' "${WG_CONFIG_FILE}"; then
    err "Некорректный формат WireGuard конфига — нет секции [Interface]"
    rollback_wg
fi

if ! grep -q '^\[Peer\]' "${WG_CONFIG_FILE}"; then
    err "Некорректный формат WireGuard конфига — нет секции [Peer]"
    rollback_wg
fi

if grep -q '<.*>' "${WG_CONFIG_FILE}"; then
    err "Конфиг содержит незаполненные плейсхолдеры (<...>)"
    rollback_wg
fi

if ! grep -q '^PrivateKey' "${WG_CONFIG_FILE}"; then
    err "Конфиг не содержит PrivateKey"
    rollback_wg
fi

ok "Конфиг валиден"

# ═══════════════════════════════════
# STEP 3: RESTART WireGuard
# ═══════════════════════════════════

step "ШАГ 3: Перезапуск WireGuard"

wg-quick down wg0 2>/dev/null || true

if ! wg-quick up wg0; then
    err "Не удалось запустить WireGuard!"
    rollback_wg
fi
ok "WireGuard запущен"

# ═══════════════════════════════════
# STEP 4: VERIFY TUNNEL
# ═══════════════════════════════════

step "ШАГ 4: Проверка тоннеля"

echo "WireGuard статус:"
wg show wg0

PEER_COUNT=$(wg show wg0 peers 2>/dev/null | wc -l || echo "0")
ok "Активных пиров: ${PEER_COUNT}"

LATEST_HANDSHAKE=$(wg show wg0 latest-handshakes 2>/dev/null | head -1 | awk '{print $2}' || echo "0")
if [[ "${LATEST_HANDSHAKE}" != "0" && "${LATEST_HANDSHAKE}" -gt 0 ]]; then
    ok "Последний handshake: $(date -d @${LATEST_HANDSHAKE} '+%Y-%m-%d %H:%M:%S' 2>/dev/null || echo ${LATEST_HANDSHAKE})"
else
    warn "Нет активных handshake — тоннель может быть не установлен"
fi

TRANSFER=$(wg show wg0 transfer 2>/dev/null || true)
if [[ -n "${TRANSFER}" ]]; then
    log "Трафик: ${TRANSFER}"
fi

# ═══════════════════════════════════
# STEP 5: CONNECTIVITY CHECK
# ═══════════════════════════════════

step "ШАГ 5: Проверка связности"

RU_IP="${RU_SERVER_IP:-10.20.0.1}"
PING_OK=false
for i in 1 2 3; do
    if ping -c 1 -W 3 "${RU_IP}" 2>/dev/null; then
        PING_OK=true
        ok "Пинг до ${RU_IP} успешен (попытка ${i})"
        break
    else
        log "Попытка ${i}/3 — нет ответа от ${RU_IP}"
        sleep 2
    fi
done

if [[ "${PING_OK}" != "true" ]]; then
    warn "Пинг до РФ-сервера (${RU_IP}) не прошёл"
    warn "Тоннель может требовать времени для установления"
    warn "Проверьте: wg show wg0 latest-handshakes"
fi

# ═══════════════════════════════════
# SAVE DEPLOY STATE
# ═══════════════════════════════════

echo "SUCCESS|${TIMESTAMP}|$(date '+%Y-%m-%d_%H:%M:%S')" >> /etc/wireguard/.deploy-history

step "ДЕПЛОЙ ЗАРУБЕЖНОГО СЕРВЕРА ЗАВЕРШЁН"

echo -e "${GREEN}"
cat <<EOF
  ┌──────────────────────────────────────────────────┐
  │                                                  │
  │   FOREIGN SERVER DEPLOYED SUCCESSFULLY          │
  │                                                  │
  │   WG Config: ${WG_CONFIG_FILE}
  │   Peers:     ${PEER_COUNT}
  │   Tunnel:    $([[ "${PING_OK}" == "true" ]] && echo "OK" || echo "PENDING")
  │                                                  │
  └──────────────────────────────────────────────────┘
EOF
echo -e "${NC}"
