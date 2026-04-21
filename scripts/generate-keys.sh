#!/usr/bin/env bash
set -euo pipefail

usage() {
    echo "Использование:" >&2
    echo "  $0 server                    — сгенерировать пару ключей для сервера" >&2
    echo "  $0 client <name>             — сгенерировать пару ключей для клиента" >&2
    echo "  $0 tunnel                    — сгенерировать пары ключей для межсерверного тоннеля" >&2
    echo "  $0 peer-conf <name> <server_ip> <server_pubkey> <client_ip> [dns] — сгенерировать .conf для клиента" >&2
    echo "" >&2
    echo "Требования: wireguard-tools (wg)" >&2
    exit 1
}

check_wg() {
    if ! command -v wg &>/dev/null; then
        echo "Ошибка: wg не установлен. Установите wireguard-tools." >&2
        exit 1
    fi
}

gen_keypair() {
    local private_key public_key
    private_key=$(wg genkey)
    public_key=$(echo "$private_key" | wg pubkey)
    echo "Private Key: $private_key"
    echo "Public Key:  $public_key"
}

cmd_server() {
    echo "=== Ключи сервера WireGuard ==="
    gen_keypair
}

cmd_client() {
    local name="$1"
    echo "=== Ключи клиента '$name' ==="
    gen_keypair
}

cmd_tunnel() {
    echo "=== Ключи для межсерверного тоннеля (wg1 / wg0) ==="
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
    echo "--- sing-box WireGuard outbound ---"
    local sb_priv sb_pub
    sb_priv=$(wg genkey)
    sb_pub=$(echo "$sb_priv" | wg pubkey)
    echo "Private Key (в sing-box config): $sb_priv"
    echo "Public Key  (на зарубежном сервере): $sb_pub"
    echo ""
    echo "--- Настройка ---"
    echo "В wg1.conf (РФ): private_key = $ru_priv, peer_public_key = $foreign_pub"
    echo "В wg0.conf (зарубежный): private_key = $foreign_priv, peer_public_key = $ru_pub"
    echo "В sing-box config.json: private_key = $sb_priv, peer_public_key = $foreign_pub"
    echo "В wg0.conf (зарубежный, 2-й peer): PublicKey = $sb_pub, AllowedIPs = 10.30.0.2/32"
}

cmd_peer_conf() {
    local name="$1"
    local server_ip="$2"
    local server_pubkey="$3"
    local client_ip="$4"
    local dns="${5:-1.1.1.1,8.8.8.8}"

    local priv pub
    priv=$(wg genkey)
    pub=$(echo "$priv" | wg pubkey)

    echo "# Клиент: $name"
    echo "# Private Key: $priv"
    echo "# Public Key:  $pub"
    echo ""
    echo "[Interface]"
    echo "PrivateKey = $priv"
    echo "Address = $client_ip/24"
    echo "DNS = $dns"
    echo ""
    echo "[Peer]"
    echo "PublicKey = $server_pubkey"
    echo "Endpoint = $server_ip:51820"
    echo "AllowedIPs = 0.0.0.0/0, ::/0"
    echo "PersistentKeepalive = 25"
}

check_wg

case "${1:-}" in
    server)
        cmd_server
        ;;
    client)
        [[ -z "${2:-}" ]] && usage
        cmd_client "$2"
        ;;
    tunnel)
        cmd_tunnel
        ;;
    peer-conf)
        [[ -z "${2:-}" || -z "${3:-}" || -z "${4:-}" || -z "${5:-}" ]] && usage
        cmd_peer_conf "$2" "$3" "$4" "$5" "${6:-}"
        ;;
    *)
        usage
        ;;
esac
