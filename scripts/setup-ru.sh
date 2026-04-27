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

echo "=== SmartTraffic: Настройка РФ-сервера ==="

echo "[1/7] Обновление системы..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get upgrade -y

echo "[2/7] Установка базовых пакетов..."
apt-get install -y \
    curl \
    wget \
    git \
    ufw \
    net-tools \
    dnsutils \
    ca-certificates \
    gnupg \
    lsb-release \
    apparmor-utils \
    wireguard \
    wireguard-tools \
    qrencode \
    iptables-persistent

echo "[3/7] Установка Docker..."
if ! command -v docker &>/dev/null; then
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
    echo \
        "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
        $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
        | tee /etc/apt/sources.list.d/docker.list >/dev/null
    apt-get update -y
    apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
    systemctl enable docker
    systemctl start docker
    echo "Docker установлен: $(docker --version)"
else
    echo "Docker уже установлен: $(docker --version)"
fi

echo "[4/7] Настройка sysctl..."
cat > /etc/sysctl.d/99-smarttraffic.conf << 'SYSCTL'
net.ipv4.ip_forward = 1
net.ipv6.conf.all.forwarding = 1
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.conf.default.send_redirects = 0
SYSCTL
sysctl --system

echo "[5/7] Настройка UFW фаервола..."
ufw --force reset
ufw default deny incoming
ufw default allow outgoing
ufw default allow FORWARD
ufw allow 22/tcp comment 'SSH'
ufw allow 80/tcp comment 'HTTP'
ufw allow 443/tcp comment 'VLESS+Reality'
ufw allow 51821/udp comment 'WireGuard tunnel to foreign server'
sed -i 's/DEFAULT_FORWARD_POLICY="DROP"/DEFAULT_FORWARD_POLICY="ACCEPT"/' /etc/default/ufw
ufw --force enable

echo "[6/7] Запуск WireGuard (межсерверный тоннель)..."
systemctl enable wg-quick@wg1 || true

echo "[7/7] Создание рабочей директории..."
mkdir -p /opt/smarttraffic
mkdir -p /opt/smarttraffic/data
mkdir -p /opt/smarttraffic/singbox
mkdir -p /opt/smarttraffic/nginx
mkdir -p /opt/smarttraffic/certbot

echo ""
echo "=== Настройка РФ-сервера завершена ==="
echo ""
echo "Следующие шаги:"
echo "  1. Скопируйте конфиг WireGuard: deploy/server-ru/wireguard/wg1.conf -> /etc/wireguard/wg1.conf"
echo "  2. Заполните ключи в конфиге WireGuard (используйте scripts/generate-keys.sh)"
echo "  3. Примените iptables правила: bash deploy/server-ru/iptables/rules.sh"
echo "  4. Получите SSL сертификат: bash deploy/server-ru/scripts/certbot-setup.sh"
echo "  5. Запустите сервисы: docker compose -f deploy/server-ru/docker-compose.prod.yml up -d"
