#!/usr/bin/env bash
# SmartTraffic — anti-DPI rate-limit на клиентский порт VLESS (RU:8443).
#
# Защищает от ТСПУ-зондирования (см. 195.133.246.34 / Mastertel AS29226):
# ограничивает кол-во НОВЫХ TCP на порт 8443 с одного IP.
#
# ВАЖНО: клиентский конфиг VLESS+Reality+xtls-rprx-vision НЕ поддерживает mux
# (vision несовместим с мультиплексированием), поэтому sing-box открывает много
# TCP-соединений. Лимит 15/мин убивал легитимных клиентов (i/o timeout, "то
# работает то нет"). Поднято до 180/мин burst 300 — держит всплески браузинга
# (тяжёлая страница ~100 соединений) и режет только сканеры-флуд (>>1000/мин).
#
# Идемпотентно: перед применением удаляет старые цепочки smarttraffic-*.

set -euo pipefail
[[ "$(id -u)" -eq 0 ]] || { echo "нужен root"; exit 1; }

IPT=iptables
VLESS_PORT="${VLESS_PORT:-8443}"
SSH_PORT="${SSH_PORT:-22}"

echo "=== SmartTraffic anti-DPI rate-limit (port $VLESS_PORT) ==="

# 1. Удалить старые правила (если есть) — идемпотентность
$IPT -D INPUT -p tcp --dport "$VLESS_PORT" -j smarttraffic-vless 2>/dev/null || true
$IPT -F smarttraffic-vless 2>/dev/null || true
$IPT -X smarttraffic-vless 2>/dev/null || true

# 2. Создать цепочку
$IPT -N smarttraffic-vless

# 3. Всегда пропускать уже установленные соединения (чтобы не порвать активных клиентов)
$IPT -A smarttraffic-vless -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT

# 4. Rate-limit НОВЫХ соединений с одного IP:
#    180 в минуту, burst 300, таблица на 65536 хостов
#    Сканеры, дающие >180 SYN/мин с IP → DROP; легитимные no-mux клиенты проходят
VLESS_PORT_RATE="${VLESS_PORT_RATE:-180/min}"
VLESS_PORT_BURST="${VLESS_PORT_BURST:-300}"
$IPT -A smarttraffic-vless -m conntrack --ctstate NEW \
    -m hashlimit --hashlimit-name vless_dpi \
        --hashlimit-above "$VLESS_PORT_RATE" --hashlimit-burst "$VLESS_PORT_BURST" \
        --hashlimit-mode srcip --hashlimit-srcmask 32 \
        --hashlimit-htable-size 65536 \
    -j DROP

# 5. Логировать заблокированных (не больше 3 вхождений/мин, чтобы не засорять dmesg)
$IPT -A smarttraffic-vless -m conntrack --ctstate NEW -m limit --limit 3/min -j LOG \
    --log-prefix "ST_DPI_BLOCK: " --log-level 4

# 6. Пропустить всё остальное
$IPT -A smarttraffic-vless -j ACCEPT

# 7. Применить цепочку к трафику на VLESS_PORT (вставить в начало INPUT)
$IPT -I INPUT 1 -p tcp --dport "$VLESS_PORT" -j smarttraffic-vless

# 8. Дополнительно — мягкая защита SSH (см. "Connection reset" от сканеров)
$IPT -D INPUT -p tcp --dport "$SSH_PORT" -j smarttraffic-ssh 2>/dev/null || true
$IPT -F smarttraffic-ssh 2>/dev/null || true
$IPT -X smarttraffic-ssh 2>/dev/null || true
$IPT -N smarttraffic-ssh
$IPT -A smarttraffic-ssh -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
$IPT -A smarttraffic-ssh -m conntrack --ctstate NEW \
    -m hashlimit --hashlimit-name ssh_dpi \
        --hashlimit-above 10/min --hashlimit-burst 20 \
        --hashlimit-mode srcip --hashlimit-srcmask 32 \
    -j DROP
$IPT -A smarttraffic-ssh -j ACCEPT
$IPT -I INPUT 2 -p tcp --dport "$SSH_PORT" -j smarttraffic-ssh

# 9. Сохранить
if command -v netfilter-persistent &>/dev/null; then
    netfilter-persistent save
elif command -v iptables-save &>/dev/null; then
    mkdir -p /etc/iptables
    iptables-save > /etc/iptables/rules.v4
fi

echo ""
echo "=== Применено. Текущие правила INPUT: ==="
$IPT -L INPUT -n -v --line-numbers
echo ""
echo "=== Цепочка smarttraffic-vless: ==="
$IPT -L smarttraffic-vless -n -v
echo ""
echo "Откат:"
echo "  iptables -D INPUT -p tcp --dport $VLESS_PORT -j smarttraffic-vless"
echo "  iptables -D INPUT -p tcp --dport $SSH_PORT -j smarttraffic-ssh"
echo "  iptables -F smarttraffic-vless && iptables -X smarttraffic-vless"
echo "  iptables -F smarttraffic-ssh && iptables -X smarttraffic-ssh"
