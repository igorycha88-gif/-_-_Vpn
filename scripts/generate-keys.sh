#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Использование:" >&2
    echo "  $0 tunnel  — сгенерировать пары ключей для межсерверного WG тоннеля" >&2
    echo "  $0 vless   — сгенерировать ключи для VLESS+Reality" >&2
    echo "" >&2
    echo "Требования: wireguard-tools (wg), openssl" >&2
    exit 1
}

check_wg() {
    if ! command -v wg &>/dev/null; then
        echo "Ошибка: wg не установлен. Установите wireguard-tools." >&2
        exit 1
    fi
}

cmd_tunnel() {
    echo "=== Ключи для межсерверного тоннеля (wg1 ↔ wg0) ==="
    echo ""
    echo "--- РФ-сервер (wg1) ---"
    local ru_priv ru_pub
    ru_priv=$(wg genkey)
    ru_pub=$(echo "$ru_priv" | wg pubkey)
    echo "Private Key: $ru_priv"
    echo "Public Key:  $ru_pub"
    echo ""
    echo "--- Зарубежный сервер (wg0) ---"
    local foreign_priv foreign_pub
    foreign_priv=$(wg genkey)
    foreign_pub=$(echo "$foreign_priv" | wg pubkey)
    echo "Private Key: $foreign_priv"
    echo "Public Key:  $foreign_pub"
    echo ""
    echo "--- Настройка ---"
    echo "В wg1.conf (РФ): PrivateKey = $ru_priv, PublicKey (peer) = $foreign_pub"
    echo "В wg0.conf (зарубежный): PrivateKey = $foreign_priv, PublicKey (peer) = $ru_pub"
}

cmd_vless() {
    echo "=== Ключи VLESS+Reality ==="
    echo ""
    echo "--- Reality ключи ---"
    echo "Для генерации ключей Reality выполните на сервере с sing-box:"
    echo "  sing-box generate reality-keypair"
    echo ""
    echo "--- Short ID ---"
    local short_id
    short_id=$(openssl rand -hex 8)
    echo "Short ID: $short_id"
    echo ""
    echo "--- UUID клиента ---"
    local client_uuid
    client_uuid=$(uuidgen 2>/dev/null || python3 -c "import uuid; print(uuid.uuid4())")
    echo "Client UUID: $client_uuid"
    echo ""
    echo "--- Настройка ---"
    echo "В .env: VLESS_PRIVATE_KEY=<reality-private-key>"
    echo "        VLESS_PUBLIC_KEY=<reality-public-key>"
    echo "        VLESS_SHORT_ID=$short_id"
    echo "        FOREIGN_VLESS_UUID=$client_uuid"
}

case "${1:-}" in
    tunnel)
        check_wg
        cmd_tunnel
        ;;
    vless)
        cmd_vless
        ;;
    *)
        usage
        ;;
esac
