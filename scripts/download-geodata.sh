#!/usr/bin/env bash
set -euo pipefail

SINGBOX_DIR="${1:-/opt/smarttraffic/singbox}"

GEOIP_URL="https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
GEOSITE_URL="https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"

MIRROR_GEOIP_URLS=(
  "https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
  "https://ghgo.xyz/https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
  "https://ghproxy.net/https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
  "https://mirror.ghproxy.com/https://github.com/SagerNet/sing-geoip/releases/latest/download/geoip.db"
)

MIRROR_GEOSITE_URLS=(
  "https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
  "https://ghgo.xyz/https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
  "https://ghproxy.net/https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
  "https://mirror.ghproxy.com/https://github.com/SagerNet/sing-geosite/releases/latest/download/geosite.db"
)

download_with_fallback() {
  local output_path="$1"
  shift
  local urls=("$@")

  for url in "${urls[@]}"; do
    echo "  Попытка: $url"
    if curl -fsSL --connect-timeout 10 --max-time 120 -o "$output_path" "$url"; then
      if [ -s "$output_path" ]; then
        echo "  OK: скачан $(stat -f%z "$output_path" 2>/dev/null || stat -c%s "$output_path" 2>/dev/null) байт"
        return 0
      fi
    fi
    rm -f "$output_path"
  done

  return 1
}

echo "=== SmartTraffic: Скачивание гео-баз данных ==="
echo "Целевая директория: $SINGBOX_DIR"
mkdir -p "$SINGBOX_DIR"

echo ""
echo "[1/2] Скачивание geoip.db..."
if ! download_with_fallback "$SINGBOX_DIR/geoip.db" "${MIRROR_GEOIP_URLS[@]}"; then
  echo "  ОШИБКА: не удалось скачать geoip.db ни с одного зеркала"
  echo "  Скачайте вручную: curl -L -o $SINGBOX_DIR/geoip.db $GEOIP_URL"
  exit 1
fi

echo ""
echo "[2/2] Скачивание geosite.db..."
if ! download_with_fallback "$SINGBOX_DIR/geosite.db" "${MIRROR_GEOSITE_URLS[@]}"; then
  echo "  ОШИБКА: не удалось скачать geosite.db ни с одного зеркала"
  echo "  Скачайте вручную: curl -L -o $SINGBOX_DIR/geosite.db $GEOSITE_URL"
  exit 1
fi

echo ""
echo "=== Готово ==="
ls -la "$SINGBOX_DIR/geoip.db" "$SINGBOX_DIR/geosite.db"
echo ""
echo "Перезапустите sing-box: docker restart smarttraffic-singbox"
