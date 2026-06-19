#!/usr/bin/env bash
# SmartTraffic Tier 2 (safe part) — RU server
# Добавляет запасной транспорт Hysteria2 на :8444/UDP + расширяет short_ids.
# НЕ ломает текущих клиентов: существующие UUID/short_id/server_name остаются.
#
# Что делает:
#   1. Бэкап конфига sing-box
#   2. Генерирует self-signed TLS cert + Hysteria2 пароль
#   3. В JSON конфиге: добавляет hysteria2 inbound на 8444/UDP,
#      добавляет ещё 4 short_id к существующему Reality
#   4. Открывает UDP 8444 в iptables (+ rate-limit от сканеров)
#   5. Проверяет конфиг, рестарт sing-box
#   6. Печатает клиентский конфиг Hysteria2 для раздачи
#
# Запуск: bash apply-tier2-ru.sh

set -euo pipefail
[[ "$(id -u)" -eq 0 ]] || { echo "нужен root"; exit 1; }

STAMP=$(date -u +"%Y%m%dT%H%M%SZ")
BK="/root/stm-backup-tier2-$STAMP"
SINGBOX_DIR="${SINGBOX_DIR:-/opt/smarttraffic/deploy/server-ru/singbox}"
CONFIG="$SINGBOX_DIR/config.json"
HY2_PORT="${HY2_PORT:-8444}"

mkdir -p "$BK"
cp -a "$CONFIG" "$BK/config.json"
iptables-save > "$BK/iptables.v4" 2>/dev/null || true
echo "=== Бэкап: $BK ==="

# 1. Самоподписанный сертификат для Hysteria2 (внутри $SINGBOX_DIR, т.к. он примонтирован в контейнер)
echo ""
echo "=== [1/5] Генерация self-signed cert + пароля Hysteria2 ==="
mkdir -p "$SINGBOX_DIR/hy2"
if [ ! -f "$SINGBOX_DIR/hy2/cert.pem" ] || [ ! -f "$SINGBOX_DIR/hy2/key.pem" ]; then
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout "$SINGBOX_DIR/hy2/key.pem" \
        -out "$SINGBOX_DIR/hy2/cert.pem" \
        -days 3650 -subj "/CN=www.bing.com" 2>/dev/null
fi
chmod 600 "$SINGBOX_DIR/hy2/key.pem"
chmod 644 "$SINGBOX_DIR/hy2/cert.pem"

HY2_PASS_FILE="$SINGBOX_DIR/hy2/password"
if [ ! -f "$HY2_PASS_FILE" ]; then
    openssl rand -base64 24 | tr -d '/+=' | head -c 22 > "$HY2_PASS_FILE"
    chmod 600 "$HY2_PASS_FILE"
fi
HY2_PASS=$(cat "$HY2_PASS_FILE")
echo "Hysteria2 password: $HY2_PASS"
echo "(сохранён в $HY2_PASS_FILE; внутри контейнера: /etc/singbox/hy2/password)"

# 2. Патч JSON конфига: добавить hysteria2 inbound + расширить short_ids
echo ""
echo "=== [2/5] Патч JSON: +hysteria2 inbound, +short_ids ==="
python3 <<PYEOF
import json, sys

CONFIG = "$CONFIG"
HY2_PORT = $HY2_PORT
HY2_PASS = "$HY2_PASS"

with open(CONFIG) as f:
    c = json.load(f)

inbounds = c.setdefault("inbounds", [])

# (а) расширить short_id у Reality inbound (если ещё не расширен)
EXTRA_SHORT_IDS = ["7c2d3e4f5a6b7c8d", "1a2b3c4d5e6f7a8b", "9f8e7d6c5b4a3928", "0f1e2d3c4b5a6978"]
for i in inbounds:
    tls = i.get("tls", {})
    reality = tls.get("reality")
    if not reality:
        continue
    sids = reality.setdefault("short_id", [])
    if not isinstance(sids, list):
        sids = [sids]
    for s in EXTRA_SHORT_IDS:
        if s not in sids:
            sids.append(s)
    reality["short_id"] = sids
    print(f"Reality inbound '{i.get('tag')}' short_ids now: {sids}")

# (б) добавить hysteria2 inbound (если ещё нет)
if not any(i.get("tag") == "hy2-in" for i in inbounds):
    inbounds.append({
        "type": "hysteria2",
        "tag": "hy2-in",
        "listen": "::",
        "listen_port": HY2_PORT,
        "up_mbps": 100,
        "down_mbps": 100,
        "ignore_client_bandwidth": False,
        "users": [{"password": HY2_PASS}],
        "tls": {
            "enabled": True,
            "alpn": ["h3"],
            "certificate_path": "/etc/singbox/hy2/cert.pem",
            "key_path": "/etc/singbox/hy2/key.pem"
        }
    })
    print(f"Добавлен hysteria2 inbound на UDP :{HY2_PORT}")
else:
    print("hysteria2 inbound уже есть — пропускаем")

# (в) убедиться, что route.final не режет hy2 трафик — он идёт в существующий route
with open(CONFIG, "w") as f:
    json.dump(c, f, indent=2, ensure_ascii=False)
    f.write("\n")
print("Конфиг записан")
PYEOF

# 3. Валидация
echo ""
echo "=== [3/5] Валидация конфига ==="
if ! docker exec smarttraffic-singbox sing-box check -c /etc/singbox/config.json 2>&1; then
    echo "ОШИБКА: конфиг невалиден. Откат."
    cp "$BK/config.json" "$CONFIG"
    exit 1
fi
echo "OK"

# 4. iptables: открыть UDP 8444 + rate-limit
echo ""
echo "=== [4/5] iptables: открыть UDP $HY2_PORT + rate-limit ==="
IPT=iptables
$IPT -D INPUT -p udp --dport "$HY2_PORT" -j smarttraffic-hy2 2>/dev/null || true
$IPT -F smarttraffic-hy2 2>/dev/null || true
$IPT -X smarttraffic-hy2 2>/dev/null || true
$IPT -N smarttraffic-hy2
# limit: 20 NEW UDP "connections"/min per source IP
$IPT -A smarttraffic-hy2 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
$IPT -A smarttraffic-hy2 -m conntrack --ctstate NEW -m hashlimit \
    --hashlimit-name hy2_dpi --hashlimit-above 20/min --hashlimit-burst 40 \
    --hashlimit-mode srcip --hashlimit-srcmask 32 --hashlimit-htable-size 65536 \
    -j DROP
$IPT -A smarttraffic-hy2 -j ACCEPT
$IPT -I INPUT 3 -p udp --dport "$HY2_PORT" -j smarttraffic-hy2
# persist
netfilter-persistent save 2>/dev/null || iptables-save > /etc/iptables/rules.v4 2>/dev/null || true

# 5. Рестарт sing-box + smoke
echo ""
echo "=== [5/5] Рестарт sing-box + проверка ==="
cd /opt/smarttraffic
docker compose -f deploy/server-ru/docker-compose.prod.yml restart singbox
sleep 5
docker compose -f deploy/server-ru/docker-compose.prod.yml ps singbox --format '{{.Status}}'
echo "Слушает ли UDP $HY2_PORT:"
ss -ulnp | grep ":$HY2_PORT " || echo "(порт не слушается — проверить логи)"
echo "Reality short_id в работе:"
docker exec smarttraffic-singbox cat /etc/singbox/config.json 2>/dev/null | python3 -c "
import json,sys
c=json.load(sys.stdin)
for i in c.get('inbounds',[]):
    r=i.get('tls',{}).get('reality',{})
    if r: print(f\"  {i.get('tag')}: short_ids={r.get('short_id')}\")
    if i.get('type')=='hysteria2': print(f\"  {i.get('tag')}: port={i.get('listen_port')}\")
"

# 6. Печать клиентского конфига Hysteria2 (для раздачи)
echo ""
echo "============================================================"
echo " Tier 2 (RU) применён. Бэкап: $BK"
echo "============================================================"
echo ""
echo "=== Клиентский outbound Hysteria2 (вставить в клиентский sing-box конфиг): ==="
cat <<HYEOF
{
  "type": "hysteria2",
  "tag": "proxy-hy2",
  "server": "130.49.129.241",
  "server_port": 8444,
  "password": "$HY2_PASS",
  "tls": {
    "enabled": true,
    "server_name": "www.bing.com",
    "insecure": true,
    "alpn": ["h3"]
  }
}
HYEOF
echo ""
echo " ВНИМАНИЕ: insecure=true (самоподписанный cert). Для продакшна рекомендуется"
echo " купить домен и выпустить Let's Encrypt сертификат — тогда insecure можно убрать."
echo ""
echo " Откат:"
echo "   cp $BK/config.json $CONFIG"
echo "   cd /opt/smarttraffic && docker compose -f deploy/server-ru/docker-compose.prod.yml restart singbox"
echo "   iptables -D INPUT -p udp --dport $HY2_PORT -j smarttraffic-hy2"
echo "   iptables -F smarttraffic-hy2 && iptables -X smarttraffic-hy2"
echo "============================================================"
