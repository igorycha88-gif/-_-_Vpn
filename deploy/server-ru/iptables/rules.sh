#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Ошибка: запустите от root или через sudo" >&2
    exit 1
fi

echo "=== SmartTraffic: Применение iptables правил (межсерверный тоннель) ==="

IPT="iptables"
WG_TUNNEL="wg1"
PUBLIC_IFACE="${PUBLIC_IFACE:-eth0}"

echo "Интерфейсы: WG_TUNNEL=$WG_TUNNEL, PUBLIC=$PUBLIC_IFACE"

echo "[1/4] Очистка старых правил..."
$IPT -t nat -F PREROUTING 2>/dev/null || true

echo "[2/4] Настройка NAT для межсерверного тоннеля..."
$IPT -t nat -A POSTROUTING -o $WG_TUNNEL -j MASQUERADE

echo "[3/4] Настройка форвардинга..."
$IPT -A FORWARD -i $WG_TUNNEL -j ACCEPT
$IPT -A FORWARD -o $WG_TUNNEL -j ACCEPT
$IPT -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT
$IPT -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -o $PUBLIC_IFACE -j TCPMSS --clamp-mss-to-pmtu

echo "[4/4] Сохранение правил..."
if command -v netfilter-persistent &>/dev/null; then
    netfilter-persistent save
else
    $IPT-save > /etc/iptables/rules.v4 2>/dev/null || true
fi

echo ""
echo "=== iptables правила применены ==="
echo ""
echo "NAT POSTROUTING:"
$IPT -t nat -L POSTROUTING -n -v --line-numbers 2>/dev/null || true
echo ""
echo "FORWARD:"
$IPT -L FORWARD -n -v --line-numbers 2>/dev/null || true
