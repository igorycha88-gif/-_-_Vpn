#!/usr/bin/env bash
#
# SmartTraffic — macOS launcher для proxy-режима (Cisco-safe) с watchdog
#
# Запускает sing-box с mixed-inbound на 127.0.0.1:2080 и выставляет macOS
# system proxy (HTTP/HTTPS/SOCKS). Не создаёт TUN, не меняет таблицу
# маршрутов → мирно сосуществует с Cisco AnyConnect.
#
# Watchdog: следит за живостью sing-box на 127.0.0.1:2080.
#   - N неудач подряд → proxy OFF (рунет работает напрямую, не ломается).
#   - M удач подряд   → proxy ON  (самовосстановление после подъёма sing-box).
#
# Использование:
#   ./macos-proxy-launcher.sh start <config.json>   # sing-box + proxy + watchdog
#   ./macos-proxy-launcher.sh stop                  # стоп всего + proxy OFF
#   ./macos-proxy-launcher.sh status                # состояние
#   ./macos-proxy-launcher.sh install <config.json> # launchd-автозапуск (2 агента)
#   ./macos-proxy-launcher.sh uninstall             # удалить launchd
#
# Тюнинг watchdog через env:
#   SMARTTRAFFIC_CHECK_INTERVAL=5   # интервал проверки, сек
#   SMARTTRAFFIC_FAILS=3            # неудач подряд -> proxy OFF
#   SMARTTRAFFIC_SUCCESSES=3        # удач подряд -> proxy ON
#
set -euo pipefail

PROXY_HOST="127.0.0.1"
PROXY_PORT="2080"
STATEDIR="/tmp"
SINGBOX_PIDFILE="${STATEDIR}/smarttraffic-singbox.pid"
WATCHDOG_PIDFILE="${STATEDIR}/smarttraffic-watchdog.pid"
SINGBOX_LOG="/tmp/smarttraffic-singbox.log"
WATCHDOG_LOG="/tmp/smarttraffic-watchdog.log"

LAUNCHD_LABEL_SINGBOX="ai.smarttraffic.singbox"
LAUNCHD_LABEL_WATCHDOG="ai.smarttraffic.watchdog"
LAUNCHD_PLIST_SINGBOX="$HOME/Library/LaunchAgents/${LAUNCHD_LABEL_SINGBOX}.plist"
LAUNCHD_PLIST_WATCHDOG="$HOME/Library/LaunchAgents/${LAUNCHD_LABEL_WATCHDOG}.plist"

CHECK_INTERVAL="${SMARTTRAFFIC_CHECK_INTERVAL:-5}"
FAILS_TO_DISABLE="${SMARTTRAFFIC_FAILS:-3}"
SUCCESSES_TO_ENABLE="${SMARTTRAFFIC_SUCCESSES:-3}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
ok()   { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()  { echo -e "${RED}[FAIL]${NC} $*" >&2; }
step() { echo -e "${BLUE}=== $1 ===${NC}"; }

logw() { echo "$(date '+%Y-%m-%d %H:%M:%S') $*" >> "$WATCHDOG_LOG" 2>/dev/null || true; }

SELF="$0"
case "$SELF" in
    /*) ;;
    *)  SELF="$(cd "$(dirname "$SELF")" && pwd)/$(basename "$SELF")" ;;
esac

require_singbox() {
    if ! command -v sing-box >/dev/null 2>&1; then
        err "sing-box не найден. Установите: brew install sing-box"
        exit 1
    fi
}

network_services() {
    networksetup -listallnetworkservices 2>/dev/null | tail -n +2 \
        | grep -v -iE '^(VPN|Cisco|Anyc|Bluetooth PAN|Thunderbolt Bridge|6to4|IpSec|VanyaVPN|SFM)' || true
}

set_proxy_on() {
    local svc
    while IFS= read -r svc; do
        [ -z "$svc" ] && continue
        networksetup -setwebproxy           "$svc" "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1 || true
        networksetup -setsecurewebproxy     "$svc" "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1 || true
        networksetup -setsocksfirewallproxy "$svc" "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1 || true
    done < <(network_services)
}

set_proxy_off() {
    local svc
    while IFS= read -r svc; do
        [ -z "$svc" ] && continue
        networksetup -setwebproxystate           "$svc" off >/dev/null 2>&1 || true
        networksetup -setsecurewebproxystate     "$svc" off >/dev/null 2>&1 || true
        networksetup -setsocksfirewallproxystate "$svc" off >/dev/null 2>&1 || true
    done < <(network_services)
}

proxy_is_on() {
    networksetup -getwebproxy "Wi-Fi" 2>/dev/null | grep -q "Enabled: Yes"
}

tcp_open() {
    nc -z -G 1 -w 1 "$PROXY_HOST" "$PROXY_PORT" >/dev/null 2>&1
}

start_singbox() {
    local cfg="$1"
    [ -f "$cfg" ] || { err "конфиг не найден: $cfg"; exit 1; }
    if ! sing-box check -c "$cfg" >/dev/null 2>&1; then
        err "конфиг невалиден: $cfg (sing-box check failed)"
        exit 1
    fi
    if [ -f "$SINGBOX_PIDFILE" ] && kill -0 "$(cat "$SINGBOX_PIDFILE")" 2>/dev/null; then
        warn "sing-box уже запущен (pid $(cat "$SINGBOX_PIDFILE"))"
        return 0
    fi
    sing-box run -c "$cfg" >"$SINGBOX_LOG" 2>&1 &
    echo "$!" > "$SINGBOX_PIDFILE"
}

wait_for_port() {
    local tries=20
    while (( tries-- > 0 )); do
        tcp_open && return 0
        sleep 0.5
    done
    return 1
}

start_watchdog() {
    if [ -f "$WATCHDOG_PIDFILE" ] && kill -0 "$(cat "$WATCHDOG_PIDFILE")" 2>/dev/null; then
        warn "watchdog уже запущен (pid $(cat "$WATCHDOG_PIDFILE"))"
        return 0
    fi
    nohup "$SELF" watchdog-run >/dev/null 2>&1 &
    local wpid=$!
    echo "$wpid" > "$WATCHDOG_PIDFILE"
    disown "$wpid" 2>/dev/null || true
}

watchdog_run() {
    trap 'logw "watchdog stopped"; exit 0' INT TERM
    local fails=0 succs=0 last_state
    if proxy_is_on; then last_state="on"; else last_state="off"; fi
    logw "watchdog started (interval=${CHECK_INTERVAL}s fails=${FAILS_TO_DISABLE} succs=${SUCCESSES_TO_ENABLE}) proxy=${last_state}"
    while true; do
        if tcp_open; then
            fails=0
            succs=$((succs+1))
            if [ "$succs" -ge "$SUCCESSES_TO_ENABLE" ] && [ "$last_state" != "on" ]; then
                set_proxy_on
                last_state="on"
                logw "sing-box healthy -> proxy ON"
            fi
        else
            succs=0
            fails=$((fails+1))
            if [ "$fails" -ge "$FAILS_TO_DISABLE" ] && [ "$last_state" != "off" ]; then
                set_proxy_off
                last_state="off"
                logw "sing-box unreachable (${fails} fails) -> proxy OFF (runet direct)"
            fi
        fi
        sleep "$CHECK_INTERVAL"
    done
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

stop_watchdog() {
    if [ -f "$WATCHDOG_PIDFILE" ]; then
        local pid
        pid="$(cat "$WATCHDOG_PIDFILE" 2>/dev/null || true)"
        if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
            ok "watchdog остановлен (pid $pid)"
        fi
        rm -f "$WATCHDOG_PIDFILE"
    fi
}

cmd_start() {
    local cfg="${1:-$HOME/smarttraffic-proxy.json}"
    require_singbox
    step "Запуск sing-box"
    start_singbox "$cfg"
    if ! wait_for_port; then
        err "sing-box не открыл порт ${PROXY_HOST}:${PROXY_PORT} за ~10с — см. ${SINGBOX_LOG}"
        err "Прокси НЕ включён. Проверьте конфиг."
        stop_singbox
        exit 1
    fi
    ok "sing-box слушает ${PROXY_HOST}:${PROXY_PORT}"
    step "Включение system proxy"
    set_proxy_on
    ok "прокси выставлен для сетевых сервисов (Cisco/VPN исключены)"
    step "Запуск watchdog"
    start_watchdog
    ok "watchdog запущен (pid $(cat "$WATCHDOG_PIDFILE"))"
    echo ""
    ok "Proxy-режим активен. Cisco AnyConnect не затронут."
    warn "Для полного покрытия отключите QUIC в браузере:"
    echo "    Chrome/Edge: chrome://flags/#enable-quic → Disabled"
    echo "    Firefox:     about:config → network.http.http3.enable = false"
}

cmd_stop() {
    step "Остановка"
    stop_watchdog
    stop_singbox
    set_proxy_off
    ok "прокси снят"
    ok "Proxy-режим выключен. Рунет работает напрямую."
}

cmd_status() {
    step "sing-box"
    if [ -f "$SINGBOX_PIDFILE" ] && kill -0 "$(cat "$SINGBOX_PIDFILE")" 2>/dev/null; then
        ok "запущен (pid $(cat "$SINGBOX_PIDFILE"))"
    else
        warn "остановлен"
    fi
    if tcp_open; then ok "порт ${PROXY_HOST}:${PROXY_PORT} — открыт"; else warn "порт ${PROXY_HOST}:${PROXY_PORT} — закрыт"; fi
    step "watchdog"
    if [ -f "$WATCHDOG_PIDFILE" ] && kill -0 "$(cat "$WATCHDOG_PIDFILE")" 2>/dev/null; then
        ok "запущен (pid $(cat "$WATCHDOG_PIDFILE"))"
    else
        warn "остановлен"
    fi
    step "system proxy (Wi-Fi)"
    networksetup -getwebproxy "Wi-Fi" 2>/dev/null | head -3 || true
    step "watchdog лог (последние 5)"
    tail -n 5 "$WATCHDOG_LOG" 2>/dev/null || echo "(лог пуст)"
}

cmd_install() {
    local cfg="${1:-$HOME/smarttraffic-proxy.json}"
    require_singbox
    [ -f "$cfg" ] || { err "конфиг не найден: $cfg"; exit 1; }
    if ! sing-box check -c "$cfg" >/dev/null 2>&1; then
        err "конфиг невалиден: $cfg"
        exit 1
    fi
    cmd_stop >/dev/null 2>&1 || true
    launchctl unload "$LAUNCHD_PLIST_SINGBOX" >/dev/null 2>&1 || true
    launchctl unload "$LAUNCHD_PLIST_WATCHDOG" >/dev/null 2>&1 || true
    launchctl unload "$HOME/Library/LaunchAgents/ai.smarttraffic.proxy.plist" >/dev/null 2>&1 || true

    mkdir -p "$(dirname "$LAUNCHD_PLIST_SINGBOX")"

    cat > "$LAUNCHD_PLIST_SINGBOX" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>${LAUNCHD_LABEL_SINGBOX}</string>
  <key>ProgramArguments</key>
  <array>
    <string>sing-box</string>
    <string>run</string>
    <string>-c</string>
    <string>${cfg}</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${SINGBOX_LOG}</string>
  <key>StandardErrorPath</key><string>${SINGBOX_LOG}</string>
</dict></plist>
EOF

    cat > "$LAUNCHD_PLIST_WATCHDOG" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>${LAUNCHD_LABEL_WATCHDOG}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BASH:-/bin/bash}</string>
    <string>${SELF}</string>
    <string>watchdog-run</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>SMARTTRAFFIC_CHECK_INTERVAL</key><string>${CHECK_INTERVAL}</string>
    <key>SMARTTRAFFIC_FAILS</key><string>${FAILS_TO_DISABLE}</string>
    <key>SMARTTRAFFIC_SUCCESSES</key><string>${SUCCESSES_TO_ENABLE}</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>${WATCHDOG_LOG}</string>
  <key>StandardErrorPath</key><string>${WATCHDOG_LOG}</string>
</dict></plist>
EOF

    launchctl load "$LAUNCHD_PLIST_SINGBOX"
    launchctl load "$LAUNCHD_PLIST_WATCHDOG"
    sleep 2

    local spid wpid
    spid="$(pgrep -f "sing-box run -c ${cfg}" 2>/dev/null | head -1 || true)"
    [ -n "$spid" ] && echo "$spid" > "$SINGBOX_PIDFILE"
    wpid="$(pgrep -f "macos-proxy-launcher.sh watchdog-run" 2>/dev/null | head -1 || true)"
    [ -n "$wpid" ] && echo "$wpid" > "$WATCHDOG_PIDFILE"

    ok "launchd-агенты установлены и запущены:"
    echo "   sing-box: ${LAUNCHD_PLIST_SINGBOX}"
    echo "   watchdog: ${LAUNCHD_PLIST_WATCHDOG}"
    ok "Прокси включается watchdog'ом, когда sing-box здоров; выключается, когда падает."
}

cmd_uninstall() {
    launchctl unload "$LAUNCHD_PLIST_SINGBOX" >/dev/null 2>&1 || true
    launchctl unload "$LAUNCHD_PLIST_WATCHDOG" >/dev/null 2>&1 || true
    launchctl unload "$HOME/Library/LaunchAgents/ai.smarttraffic.proxy.plist" >/dev/null 2>&1 || true
    rm -f "$LAUNCHD_PLIST_SINGBOX" "$LAUNCHD_PLIST_WATCHDOG"
    stop_watchdog
    stop_singbox
    set_proxy_off
    ok "launchd-агенты удалены, прокси снят"
}

usage() {
    cat <<EOF
Использование: $0 {start <config.json>|stop|status|install <config.json>|uninstall|watchdog-run}

  start <cfg>     запустить sing-box + system proxy + watchdog
  stop            остановить всё, снять прокси
  status          состояние (sing-box / watchdog / proxy)
  install <cfg>   автозапуск через launchd (2 агента: sing-box + watchdog)
  uninstall       удалить launchd
  watchdog-run    (внутренняя) цикл watchdog
EOF
}

case "${1:-}" in
    start)        shift; cmd_start "${1:-}" ;;
    stop)         cmd_stop ;;
    status)       cmd_status ;;
    install)      shift; cmd_install "${1:-}" ;;
    uninstall)    cmd_uninstall ;;
    watchdog-run) watchdog_run ;;
    -h|--help)    usage ;;
    *)            usage; exit 1 ;;
esac
