#!/usr/bin/env bash
# SmartTraffic Tier 1 — RU server (130.49.129.241)
# Безопасное применение системного усиления с бэкапами.
#
# Что делает:
#   1. Бэкап .env, iptables, sing-box конфига, docker-compose
#   2. Устанавливает sysctl (BBR, TCP-буферы, TFO)
#   3. Применяет anti-DPI rate-limit на порт 8443 (защита от ТСПУ-сканеров)
#   4. Останавливает certbot (DOMAIN=IP → letsencrypt бесполезен)
#   5. Smoke-проверка + откат
#
# Запуск: bash apply-tier1-ru.sh

set -euo pipefail
[[ "$(id -u)" -eq 0 ]] || { echo "нужен root"; exit 1; }

STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BK="/root/stm-backup-$STAMP"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_PATH="${DEPLOY_PATH:-/opt/smarttraffic}"
COMPOSE_FILE="$DEPLOY_PATH/deploy/server-ru/docker-compose.prod.yml"

mkdir -p "$BK"
echo "=== Бэкапы в $BK ==="
cp -a "$DEPLOY_PATH/.env" "$BK/env" 2>/dev/null || true
cp -a "$DEPLOY_PATH/deploy/server-ru/singbox/config.json" "$BK/singbox-config.json" 2>/dev/null || true
cp -a "$COMPOSE_FILE" "$BK/docker-compose.prod.yml" 2>/dev/null || true
cp -a /etc/sysctl.d/99-smarttraffic.conf "$BK/sysctl.conf" 2>/dev/null || true
iptables-save > "$BK/iptables.v4" 2>/dev/null || true
sysctl net.ipv4.tcp_congestion_control net.core.rmem_max > "$BK/sysctl-before.txt" 2>&1

echo ""
echo "=== [1/4] Установка sysctl (BBR + буферы + TFO) ==="
install -m 0644 "$SCRIPT_DIR/../common/sysctl/99-smarttraffic.conf" /etc/sysctl.d/99-smarttraffic.conf
sysctl --system >/dev/null 2>&1 || sysctl -p /etc/sysctl.d/99-smarttraffic.conf
echo "tcp_congestion_control = $(sysctl -n net.ipv4.tcp_congestion_control)"
echo "rmem_max               = $(sysctl -n net.core.rmem_max)"

echo ""
echo "=== [2/4] Anti-DPI rate-limit на порт 8443 (и защита SSH) ==="
# Сначала проверим, что модули загружены
modprobe ipt_hashlimit 2>/dev/null || true
modprobe xt_hashlimit 2>/dev/null || true
bash "$SCRIPT_DIR/iptables/anti-dpi.sh"

echo ""
echo "=== [3/4] Остановка certbot (DOMAIN=IP → letsencrypt невозможен) ==="
cd "$DEPLOY_PATH"
if docker compose -f "$COMPOSE_FILE" ps certbot 2>/dev/null | grep -q smarttraffic-certbot; then
    docker compose -f "$COMPOSE_FILE" stop certbot
    echo "certbot остановлен (но не удалён из compose — это Tier 3)"
else
    echo "certbot не запущен — пропускаем"
fi

echo ""
echo "=== [4/4] Smoke-проверка ==="
echo "Контейнеры:"
docker compose -f "$COMPOSE_FILE" ps --format 'table {{.Name}}\t{{.Status}}' | head -10
echo ""
echo "Anti-DPI счётчики (подождите минуту и проверьте снова):"
iptables -L smarttraffic-vless -n -v --line-numbers | head -10

echo ""
echo "============================================================"
echo " Tier 1 (RU) применён. Бэкапы: $BK"
echo "============================================================"
echo " ВАЖНО: понаблюдайте за счётчиками DPI в течение часа:"
echo "   watch -n 30 'iptables -L smarttraffic-vless -n -v'"
echo " Если легитимных клиентов стало резать — увеличьте лимит или откатите."
echo ""
echo " Откат:"
echo "   iptables -D INPUT -p tcp --dport 8443 -j smarttraffic-vless"
echo "   iptables -D INPUT -p tcp --dport 22 -j smarttraffic-ssh"
echo "   iptables -F smarttraffic-vless && iptables -X smarttraffic-vless"
echo "   iptables -F smarttraffic-ssh && iptables -X smarttraffic-ssh"
echo "   cp $BK/sysctl.conf /etc/sysctl.d/99-smarttraffic.conf && sysctl --system"
echo "   docker compose -f $COMPOSE_FILE start certbot"
echo "============================================================"
