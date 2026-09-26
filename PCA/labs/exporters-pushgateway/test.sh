#!/usr/bin/env bash
# Teste automático do lab de exporters + Pushgateway.
#   Fase A: config do ENUNCIADO -> stack sobe, exporters respondem, bug do honor_labels aparece.
#   Fase B: config do GABARITO (solutions/prometheus + textfile gerado pelo script) -> exercícios 01..07.
# KEEP=1 ./test.sh  -> não derruba a stack no fim.
set -euo pipefail
cd "$(dirname "$0")"
LAB=exporters-pushgateway
PROM=http://localhost:9140
PGW=http://localhost:9143
BBX=http://localhost:9142
IMG=prom/prometheus:v3.15.0
TMPTXT=$(mktemp -d)

fail() { echo "FAIL $LAB: $*"; exit 1; }
cleanup() {
  [ "${KEEP:-0}" = 1 ] || docker compose down -v >/dev/null 2>&1 || true
  rm -rf "$TMPTXT"
}
trap cleanup EXIT

q() {
  curl -sf "$PROM/api/v1/query" --data-urlencode "query=$1" | python3 -c '
import json, sys
for r in json.load(sys.stdin)["data"]["result"]:
    print(r["value"][1])'
}
# expect DESC EXPR LO HI [TIMEOUT]: espera TODOS os resultados (>=1) ficarem em [LO,HI]
expect() {
  local desc=$1 expr=$2 lo=$3 hi=$4 deadline=$(( $(date +%s) + ${5:-90} )) out
  until out=$(q "$expr") && [ -n "$out" ] && awk -v lo="$lo" -v hi="$hi" '{ if ($1+0 < lo || $1+0 > hi) bad=1 } END { exit bad }' <<<"$out"; do
    [ "$(date +%s)" -gt "$deadline" ] && fail "$desc: esperado [$lo,$hi], veio: $(tr '\n' ' ' <<<"${out:-<vazio>}")"
    sleep 2
  done
  echo "  ok  $desc = $(tr '\n' ' ' <<<"$out")"
}
expect_empty() {
  local deadline=$(( $(date +%s) + ${3:-60} )) out
  until out=$(q "$2") && [ -z "$out" ]; do
    [ "$(date +%s)" -gt "$deadline" ] && fail "$1: esperava vazio, veio $(tr '\n' ' ' <<<"$out")"
    sleep 2
  done
  echo "  ok  $1 (vazio)"
}
push() { curl -sf --data-binary @- "$PGW/metrics/job/$1/instance/$2" || fail "push $1/$2"; }

echo "── validando configs"
docker run --rm -v "$PWD/prometheus:/etc/prometheus:ro" --entrypoint promtool $IMG check config /etc/prometheus/prometheus.yml >/dev/null || fail "prometheus.yml inválido"
docker run --rm -v "$PWD/solutions/prometheus:/etc/prometheus:ro" --entrypoint promtool $IMG check config /etc/prometheus/prometheus.yml >/dev/null || fail "solutions/prometheus.yml inválido"
docker run --rm -v "$PWD/solutions/prometheus:/p:ro" -w /p --entrypoint promtool $IMG test rules tests/batch.test.yml >/dev/null || fail "promtool test rules (PushgatewayJobStale)"
echo "  ok  promtool test rules: PushgatewayJobStale dispara com push velho"
docker run --rm -v "$PWD/blackbox:/config:ro" prom/blackbox-exporter:v0.28.0 --config.file=/config/blackbox.yml --config.check >/dev/null 2>&1 || fail "blackbox.yml inválido"
docker run --rm -i --entrypoint promtool $IMG check metrics < textfile/lab.prom || fail "textfile/lab.prom não passa no promtool"

# ═════════════════════════ FASE A: enunciado ═════════════════════════
echo "── fase A: stack do enunciado"
docker compose down -v >/dev/null 2>&1 || true
docker compose up -d --wait >/dev/null 2>&1 || fail "stack não subiu"
expect "4 targets up" 'count(up == 1)' 4 4
expect "textfile demo" 'lab_textfile_demo_info{owner="pca"}' 1 1
curl -sf "$BBX/probe?module=http_2xx&target=http://prometheus:9090/-/healthy" | grep -q '^probe_success 1' || fail "probe direta OK"
curl -sf "$BBX/probe?module=http_2xx&target=http://node-exporter:9999/" | grep -q '^probe_success 0' || fail "probe direta que deveria falhar"
echo "  ok  /probe direto no blackbox (1 e 0)"
push nightly-backup db01 <<'EOF'
# TYPE backup_size_bytes gauge
backup_size_bytes 52428800
EOF
expect "EX06 bug: sem honor_labels vira exported_job" 'backup_size_bytes{exported_job="nightly-backup",job="pushgateway"}' 52428800 52428800
expect_empty "EX06 bug: job=nightly-backup não existe" 'backup_size_bytes{job="nightly-backup"}' 1

# ═════════════════════════ FASE B: gabarito ═════════════════════════
echo "── fase B: stack do gabarito"
docker compose down -v >/dev/null 2>&1
cp textfile/lab.prom "$TMPTXT/"
solutions/textfile/backup-metrics.sh "$TMPTXT" >/dev/null
chmod 755 "$TMPTXT"
PROM_DIR=./solutions/prometheus TEXTFILE_DIR="$TMPTXT" docker compose up -d --wait >/dev/null 2>&1 || fail "stack do gabarito não subiu"
expect "todos os targets com scrape OK" 'count(up == 1)' 12 12

# EX01
expect "EX01 CPU %" '100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m])))' 0 100
expect "EX01 memória %" '100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)' 0.1 100
expect "EX01 disco / %" '100 * (1 - node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes{mountpoint="/",fstype!~"tmpfs|overlay"})' 0.01 100
expect "EX01 recording rule de CPU" 'instance:node_cpu_utilisation:ratio_rate1m' 0 1

# EX02
expect "EX02 textfile: idade do último backup (s)" 'time() - backup_last_success_timestamp_seconds{job="node",target="postgres"}' 0 600
expect "EX02 textfile sem erro" 'node_textfile_scrape_error' 0 0

# EX03
expect "EX03 prometheus OK" 'probe_success{job="blackbox-http",instance="http://prometheus:9090/-/healthy"}' 1 1
expect "EX03 pushgateway OK" 'probe_success{job="blackbox-http",instance="http://pushgateway:9091/-/healthy"}' 1 1
expect "EX03 alvo quebrado falha" 'probe_success{job="blackbox-http",instance="http://node-exporter:9999/"}' 0 0
expect "EX03 tcp/icmp/dns/https OK" 'probe_success{job=~"blackbox-(tcp|icmp|dns|https)"}' 1 1
expect "EX03 dias até o cert expirar" '(probe_ssl_earliest_cert_expiry{job="blackbox-https"} - time()) / 86400' 30 4000
expect "EX03 alerta BlackboxProbeFailed só no alvo quebrado" 'count(ALERTS{alertname="BlackboxProbeFailed",alertstate="firing",instance="http://node-exporter:9999/"})' 1 1 60
expect_empty "EX03 nenhum alerta de sonda nos alvos bons" 'ALERTS{alertname="BlackboxProbeFailed",instance!="http://node-exporter:9999/"}' 1

# EX04 + EX06
push nightly-backup db01 <<EOF
# TYPE backup_duration_seconds gauge
backup_duration_seconds 42.5
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds $(date +%s)
# TYPE backup_size_bytes gauge
backup_size_bytes 52428800
EOF
expect "EX06 honor_labels: job/instance do push preservados" 'backup_size_bytes{job="nightly-backup",instance="db01"}' 52428800 52428800
expect_empty "EX06 sem exported_job" '{exported_job!=""}' 1
expect "EX04 idade do push (push_time_seconds)" 'time() - push_time_seconds{job="nightly-backup",instance="db01"}' 0 60
expect "EX04 regra PushgatewayJobStale carregada e inativa" 'count(ALERTS{alertname="PushgatewayJobStale"}) or vector(0)' 0 0
curl -sf "$PROM/api/v1/rules" | grep -q '"name":"PushgatewayJobStale"' || fail "EX04 regra PushgatewayJobStale não carregada"
push nightly-backup db-old <<'EOF'
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds 1000000000
EOF
expect "EX04 BackupNotSucceededRecently dispara p/ db-old" 'count(ALERTS{alertname="BackupNotSucceededRecently",alertstate="firing",instance="db-old"})' 1 1 60

# EX05: PUT substitui o grupo inteiro; DELETE apaga o grupo
curl -sf -X PUT --data-binary @- "$PGW/metrics/job/nightly-backup/instance/db01" <<'EOF' || fail "PUT"
# TYPE backup_size_bytes gauge
backup_size_bytes 1024
EOF
expect_empty "EX05 PUT apagou as outras métricas do grupo" 'backup_duration_seconds{instance="db01"}' 30
expect "EX05 PUT gravou o novo valor" 'backup_size_bytes{instance="db01"}' 1024 1024
curl -sf -X DELETE "$PGW/metrics/job/nightly-backup/instance/db-old" || fail "DELETE"
curl -sf "$PGW/api/v1/metrics" | grep -q '"db-old"' && fail "EX05 grupo db-old ainda no Pushgateway"
expect_empty "EX05 db-old sumiu do Prometheus" '{instance="db-old"}' 30
expect_empty "EX05 alerta do db-old resolvido" 'ALERTS{alertname="BackupNotSucceededRecently",alertstate="firing"}' 30

# EX07: portas padrão do ecossistema
for hp in node-exporter:9100 blackbox:9115 pushgateway:9091 prometheus:9090; do
  curl -sf "$BBX/probe?module=tcp_connect&target=$hp" | grep -q '^probe_success 1' || fail "EX07 porta padrão $hp"
done
echo "  ok  EX07 portas padrão 9090/9100/9115/9091 (via sonda tcp)"

echo "PASS $LAB"
