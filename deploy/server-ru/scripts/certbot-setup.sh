#!/usr/bin/env bash
set -euo pipefail

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Ошибка: запустите от root или через sudo" >&2
    exit 1
fi

DOMAIN="${1:-}"
EMAIL="${2:-}"

if [[ -z "$DOMAIN" || -z "$EMAIL" ]]; then
    echo "Использование: $0 <domain> <email>" >&2
    echo "Пример: $0 example.com admin@example.com" >&2
    exit 1
fi

echo "=== SmartTraffic: Настройка SSL для $DOMAIN ==="

echo "[1/4] Установка certbot..."
if ! command -v certbot &>/dev/null; then
    apt-get update -y
    apt-get install -y certbot
fi

echo "[2/4] Создание директории для webroot..."
mkdir -p /var/www/certbot

echo "[3/4] Получение SSL-сертификата..."
certbot certonly \
    --webroot \
    --webroot-path=/var/www/certbot \
    --email "$EMAIL" \
    --agree-tos \
    --no-eff-email \
    -d "$DOMAIN" \
    --non-interactive

echo "[4/4] Настройка автообновления сертификата..."
cat > /etc/cron.d/certbot-renew << CRON
0 */12 * * * root certbot renew --quiet --deploy-hook "docker kill -s HUP smarttraffic-nginx 2>/dev/null || true"
CRON
chmod 644 /etc/cron.d/certbot-renew

echo ""
echo "=== SSL настроен ==="
echo "Сертификат: /etc/letsencrypt/live/$DOMAIN/"
echo "Автообновление: каждые 12 часов (cron)"
echo ""
echo "ВНИМАНИЕ: обновите server_name и пути к SSL в Nginx конфиге:"
echo "  deploy/server-ru/nginx/conf.d/default.conf"
