#!/usr/bin/env bash
#
# SmartTraffic — macOS launcher для proxy-режима (Cisco-safe)
#
# Запускает sing-box с mixed-inbound на 127.0.0.1:2080 и выставляет macOS
# system proxy (HTTP/HTTPS/SOCKS). Не создаёт TUN, не меняет таблицу
# маршрутов → мирно сосуществует с Cisco AnyConnect.
#
# Использование:
#   ./macos-proxy-launcher.sh start <config.json>   # запустить + вкл прокси
#   ./macos-proxy-launcher.sh stop                  # выкл прокси + останов
#   ./macos-proxy-launcher.sh status                # состояние
#   ./macos-proxy-launcher.sh install               # install launchd agent (auto-start)
#   ./macos-proxy-launcher.sh uninstall             # remove launchd agent
#
# Требования: sing-box в PATH (brew install sing-box), конфиг с mode=proxy.
#
set -euo pipefail

PROXY_HOST="127.0.0.1"
PROXY_PORT="2080"
SINGBOX_PIDFILE="${TMPDIR:-/tmp}/smarttraffic-singbox.pid"
LAUNCHD_LABEL="ai.smarttraffic.proxy"
LAUNCHD_PLIST="$HOME/Library/LaunchAgents/${LAUNCHD_LABEL}.plist"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[FAIL]${NC} $*" >&2; }

if ! command -v sing-box >/dev/null 2>&1; then
    err "sing-box не найден. Установите: brew install sing-box"
    exit 1
fi

# Список сетевых сервисов macOS (Wi-Fi, Ethernet, ...) — не хардкодим имя.
network_services() {
    networksetup -listallnetworkservices 2>/dev/null | tail -n +2 | grep -v -iE '^(VPN|Cisco|Anyc|Bluetooth PAN|Thunderbolt Bridge|6to4|IpSec)' || true
}

set_proxy_on() {
    while IFS= read -r svc; do
        [ -z "$svc" ] && continue
        networksetup -setwebproxy        "$svc" "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1 || true
        networksetup -setsecurewebproxy  "$svc" "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1 || true
        networksetup -setsocksfirewallproxy "$svc" "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1 || true
        ok "прокси выставлен для: $svc"
    done < <(network_services)
}

set_proxy_off() {
    while IFS= read -r svc; do
        [ -z "$svc" ] && continue
        networksetup -setwebproxystate        "$svc" off >/dev/null 2>&1 || true
        networksetup -setsecurewebproxystate  "$svc" off >/dev/null 2>&1 || true
        networksetup -setsocksfirewallproxestate "$svc" off >/dev/null 2>&1 || true
    done < <(network_services)
    ok "прокси снят"
}

start_singbox() {
    local cfg="$1"
    if [ ! -f "$cfg" ]; then
        err "конфиг не найден: $cfg"
        exit 1
    fi
    if ! sing-box check -c "$cfg" >/dev/null 2>&1; then
        err "конфиг невалиден: $cfg (sing-box check failed)"
        exit 1
    fi
    if [ -f "$SINGBOX_PIDFILE" ] && kill -0 "$(cat "$SINGBOX_PIDFILE")" 2>/dev/null; then
        warn "sing-box уже запущен (pid $(cat "$SINGBOX_PIDFILE"))"
        return
    fi
    sing-box run -c "$cfg" >/tmp/smarttraffic-singbox.log 2>&1 &
    echo $! > "$SINGBOX_PIDFILE"
    sleep 1
    if ! kill -0 "$(cat "$SINGBOX_PIDFILE")" 2>/dev/null; then
        err "sing-box не запустился, см. /tmp/smarttraffic-singbox.log"
        exit 1
    fi
    ok "sing-box запущен (pid $(cat "$SINGBOX_PIDFILE"))"
}

stop_singbox() {
    if [ -f "$SINGBOX_PIDFILE" ]; then
        local pid
        pid="$(cat "$SINGBOX_PIDFILE" 2>/dev/null || true)"
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            ok "sing-box остановлен (pid $pid)"
        fi
        rm -f "$SINGBOX_PIDFILE"
    fi
}

case "${1:-}" in
    start)
        cfg="${2:-$HOME/smarttraffic-proxy.json}"
        start_singbox "$cfg"
        set_proxy_on
        echo ""
        ok "Proxy-режим активен. Cisco AnyConnect не затронут."
        warn "Для полного покрытия отключите QUIC в браузере:"
        echo "    Chrome/Edge: chrome://flags/#enable-quic → Disabled"
        echo "    Firefox:     about:config → network.http.http3.enable = false"
        ;;
    stop)
        set_proxy_off
        stop_singbox
        ok "Proxy-режим выключен"
        ;;
    status)
        if [ -f "$SINGBOX_PIDFILE" ] && kill -0 "$(cat "$SINGBOX_PIDFILE")" 2>/dev/null; then
            ok "sing-box: запущен (pid $(cat "$SINGBOX_PIDFILE"))"
        else
            warn "sing-box: остановлен"
        fi
        networksetup -getwebproxy "Wi-Fi" 2>/dev/null | head -3 || true
        ;;
    install)
        cfg="${2:-$HOME/smarttraffic-proxy.json}"
        mkdir -p "$(dirname "$LAUNCHD_PLIST")"
        cat > "$LAUNCHD_PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>${LAUNCHD_LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BASH:-/bin/bash}</string>
    <string>$0</string>
    <string>start</string>
    <string>${cfg}</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>/tmp/smarttraffic-launchd.log</string>
  <key>StandardErrorPath</key><string>/tmp/smarttraffic-launchd.log</string>
</dict></plist>
EOF
        launchctl unload "$LAUNCHD_PLIST" >/dev/null 2>&1 || true
        launchctl load "$LAUNCHD_PLIST"
        ok "launchd-агент установлен: $LAUNCHD_PLIST"
        ;;
    uninstall)
        [ -f "$LAUNCHD_PLIST" ] && launchctl unload "$LAUNCHD_PLIST" >/dev/null 2>&1 || true
        rm -f "$LAUNCHD_PLIST"
        set_proxy_off
        stop_singbox
        ok "launchd-агент удалён"
        ;;
    *)
        echo "Использование: $0 {start <config.json>|stop|status|install <config.json>|uninstall}"
        exit 1
        ;;
esac
