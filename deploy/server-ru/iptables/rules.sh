#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Ошибка: запустите от root или через sudo" >&2
    exit 1
fi

echo "=== SmartTraffic: Применение iptables правил (TProxy + TCPMSS) ==="

IPT="iptables"
WG_CLIENT="wg0"
WG_TUNNEL="wg1"
TPROXY_PORT=12345
TPROXY_MARK=0x1
TPROXY_TABLE=100
CLIENT_SUBNET="10.10.0.0/24"
PUBLIC_IFACE="${PUBLIC_IFACE:-eth0}"

echo "Интерфейсы: WG_CLIENT=$WG_CLIENT, WG_TUNNEL=$WG_TUNNEL, PUBLIC=$PUBLIC_IFACE"
echo "TProxy port=$TPROXY_PORT, mark=$TPROXY_MARK, table=$TPROXY_TABLE"

echo "[1/6] Очистка старых правил..."
$IPT -t mangle -D PREROUTING -i $WG_CLIENT -p tcp -j SINGBOX 2>/dev/null || true
$IPT -t mangle -D PREROUTING -i $WG_CLIENT -p udp -j SINGBOX 2>/dev/null || true
$IPT -t mangle -F SINGBOX 2>/dev/null || true
$IPT -t mangle -X SINGBOX 2>/dev/null || true
$IPT -t nat -F PREROUTING 2>/dev/null || true
ip rule del fwmark $TPROXY_MARK table $TPROXY_TABLE 2>/dev/null || true
ip route del local default dev lo table $TPROXY_TABLE 2>/dev/null || true

echo "[2/6] Настройка policy routing для TProxy..."
ip rule add fwmark $TPROXY_MARK table $TPROXY_TABLE
ip route add local default dev lo table $TPROXY_TABLE

echo "[3/6] Создание цепочки SINGBOX для TProxy..."
$IPT -t mangle -N SINGBOX 2>/dev/null || true
$IPT -t mangle -F SINGBOX
$IPT -t mangle -A SINGBOX -p tcp -j TPROXY --tproxy-mark $TPROXY_MARK/$TPROXY_MARK --on-port $TPROXY_PORT --on-ip 127.0.0.1
$IPT -t mangle -A SINGBOX -p udp -j TPROXY --tproxy-mark $TPROXY_MARK/$TPROXY_MARK --on-port $TPROXY_PORT --on-ip 127.0.0.1

echo "[4/6] Перенаправление всего трафика клиентов в sing-box (TProxy)..."
$IPT -t mangle -A PREROUTING -i $WG_CLIENT -p tcp -j SINGBOX
$IPT -t mangle -A PREROUTING -i $WG_CLIENT -p udp -j SINGBOX

echo "[5/6] Настройка NAT, форвардинга и TCPMSS..."
$IPT -t nat -A POSTROUTING -s $CLIENT_SUBNET -o "$PUBLIC_IFACE" -j MASQUERADE
$IPT -t nat -A POSTROUTING -o $WG_TUNNEL -j MASQUERADE
$IPT -A FORWARD -i $WG_CLIENT -j ACCEPT
$IPT -A FORWARD -i $WG_TUNNEL -j ACCEPT
$IPT -A FORWARD -o $WG_TUNNEL -j ACCEPT
$IPT -A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT
$IPT -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -o $WG_CLIENT -j TCPMSS --clamp-mss-to-pmtu
$IPT -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -o $PUBLIC_IFACE -j TCPMSS --clamp-mss-to-pmtu

echo "[6/6] Сохранение правил..."
if command -v netfilter-persistent &>/dev/null; then
    netfilter-persistent save
else
    $IPT-save > /etc/iptables/rules.v4 2>/dev/null || true
fi

echo ""
echo "=== iptables правила применены (TProxy режим) ==="
echo ""
echo "Policy routing:"
ip rule show | grep $TPROXY_MARK || true
ip route show table $TPROXY_TABLE || true
echo ""
echo "Mangle PREROUTING:"
$IPT -t mangle -L PREROUTING -n -v --line-numbers 2>/dev/null || true
echo ""
echo "Mangle SINGBOX:"
$IPT -t mangle -L SINGBOX -n -v --line-numbers 2>/dev/null || true
echo ""
echo "NAT POSTROUTING:"
$IPT -t nat -L POSTROUTING -n -v --line-numbers 2>/dev/null || true
