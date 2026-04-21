#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Ошибка: запустите от root или через sudo" >&2
    exit 1
fi

echo "=== SmartTraffic: Применение iptables правил для transparent proxy ==="

IPT="iptables"
WG_CLIENT="wg0"
WG_TUNNEL="wg1"
SINGBOX_PORT=12345
SINGBOX_DNS_PORT=5353
CLIENT_SUBNET="10.10.0.0/24"
PUBLIC_IFACE="${PUBLIC_IFACE:-eth0}"

echo "Интерфейсы: WG_CLIENT=$WG_CLIENT, WG_TUNNEL=$WG_TUNNEL, PUBLIC=$PUBLIC_IFACE"

echo "[1/4] Очистка старых правил (только mangle и nat)..."
$IPT -t mangle -F PREROUTING 2>/dev/null || true
$IPT -t mangle -F OUTPUT 2>/dev/null || true
$IPT -t nat -F PREROUTING 2>/dev/null || true

echo "[2/4] Перенаправление TCP трафика клиентов в sing-box (transparent proxy)..."
$IPT -t nat -A PREROUTING -i $WG_CLIENT -p tcp -j REDIRECT --to-port $SINGBOX_PORT

echo "[3/4] Перенаправление DNS запросов клиентов в sing-box DNS..."
$IPT -t nat -A PREROUTING -i $WG_CLIENT -p udp --dport 53 -j REDIRECT --to-port $SINGBOX_DNS_PORT
$IPT -t nat -A PREROUTING -i $WG_CLIENT -p tcp --dport 53 -j REDIRECT --to-port $SINGBOX_DNS_PORT

echo "[4/4] Настройка NAT и форвардинга..."
$IPT -t nat -A POSTROUTING -s $CLIENT_SUBNET -o "$PUBLIC_IFACE" -j MASQUERADE
$IPT -t nat -A POSTROUTING -o $WG_TUNNEL -j MASQUERADE
$IPT -A FORWARD -i $WG_CLIENT -j ACCEPT
$IPT -A FORWARD -i $WG_TUNNEL -j ACCEPT
$IPT -A FORWARD -o $WG_TUNNEL -j ACCEPT
$IPT -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT

echo "Сохранение правил..."
if command -v netfilter-persistent &>/dev/null; then
    netfilter-persistent save
else
    $IPT-save > /etc/iptables/rules.v4 2>/dev/null || true
fi

echo ""
echo "=== iptables правила применены ==="
echo ""
echo "Текущие NAT правила:"
$IPT -t nat -L -n -v --line-numbers 2>/dev/null || true
echo ""
echo "ВНИМАНИЕ: правила применены только в памяти."
echo "Для сохранения после перезагрузки убедитесь что iptables-persistent установлен."
