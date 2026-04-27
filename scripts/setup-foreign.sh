#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Ошибка: запустите от root или через sudo" >&2
    exit 1
fi

if [[ ! -f /etc/lsb-release ]] || ! grep -q "Ubuntu" /etc/lsb-release 2>/dev/null; then
    echo "Ошибка: скрипт предназначен для Ubuntu 22.04/24.04 LTS" >&2
    exit 1
fi

RU_SERVER_IP="${1:-}"
if [[ -z "$RU_SERVER_IP" ]]; then
    echo "Использование: $0 <IP_РФ_сервера>" >&2
    echo "Пример: $0 203.0.113.10" >&2
    exit 1
fi

echo "=== SmartTraffic: Настройка зарубежного сервера ==="
echo "РФ-сервер IP: $RU_SERVER_IP"

echo "[1/5] Обновление системы..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get upgrade -y

echo "[2/5] Установка пакетов..."
apt-get install -y \
    curl \
    wget \
    ufw \
    net-tools \
    dnsutils \
    ca-certificates \
    wireguard \
    wireguard-tools \
    iptables-persistent

echo "[3/5] Настройка sysctl..."
cat > /etc/sysctl.d/99-smarttraffic.conf << 'SYSCTL'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
SYSCTL
sysctl --system

echo "[4/5] Настройка UFW фаервола..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp comment 'SSH'
ufw allow 443/tcp comment 'VLESS+Reality sing-box'
ufw allow from "$RU_SERVER_IP" to any port 51821 proto udp comment 'WireGuard tunnel from RU server'
ufw --force enable

echo "[5/5] Настройка NAT для WG тоннеля..."
cat > /etc/iptables/rules.v4 << IPTABLES
*filter
:INPUT ACCEPT [0:0]
:FORWARD ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
-A FORWARD -i wg0 -j ACCEPT
-A FORWARD -o wg0 -j ACCEPT
-A FORWARD -m state --state ESTABLISHED,RELATED -j ACCEPT
COMMIT
*nat
:PREROUTING ACCEPT [0:0]
:INPUT ACCEPT [0:0]
:OUTPUT ACCEPT [0:0]
:POSTROUTING ACCEPT [0:0]
-A POSTROUTING -s 10.20.0.0/30 -o eth0 -j MASQUERADE
COMMIT
IPTABLES
iptables-restore < /etc/iptables/rules.v4

echo ""
echo "=== Настройка зарубежного сервера завершена ==="
echo ""
echo "Следующие шаги:"
echo "  1. Скопируйте конфиг WireGuard: deploy/server-foreign/wireguard/wg0.conf -> /etc/wireguard/wg0.conf"
echo "  2. Заполните ключи в конфиге (используйте scripts/generate-keys.sh)"
echo "  3. Установите и настройте sing-box (VLESS+Reality на порту 443)"
echo "  4. Запустите WireGuard: systemctl enable --now wg-quick@wg0"
echo ""
echo "ВНИМАНИЕ: убедитесь что публичный IP интерфейс eth0. Если нет — отредактируйте /etc/iptables/rules.v4"
