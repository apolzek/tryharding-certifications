#!/usr/bin/env bash
# Teste automático do lab tsdb-storage: sobe a stack, aplica TODAS as soluções e confere o resultado.
#   ./test.sh          # sobe do zero, testa, derruba
#   KEEP=1 ./test.sh   # não derruba no fim
set -euo pipefail
cd "$(dirname "$0")"
LAB=tsdb-storage
PROM=localhost:9150
GEN=localhost:9151
CFG=prometheus/prometheus.yml
cp "$CFG" /tmp/pca-tsdb-prometheus.yml.bak

fail() { echo "FAIL $LAB: $*"; exit 1; }
cleanup() {
  cp /tmp/pca-tsdb-prometheus.yml.bak "$CFG"
  curl -s "$GEN/control?users=10&ephemeral=1&timestamped=1&down=0&ts_offset=0" >/dev/null 2>&1 || true
  [ "${KEEP:-0}" = 1 ] || docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

# q "<promql>" [time] [extra curl args...] -> imprime os valores (um por linha) ou nada
q() {
  local expr=$1; shift; local t=${1:-}; [ $# -gt 0 ] && shift
  curl -sf "$PROM/api/v1/query" --data-urlencode "query=$expr" ${t:+--data-urlencode "time=$t"} "$@" \
    | python3 -c 'import json,sys; [print(r["value"][1]) for r in json.load(sys.stdin)["data"]["result"]]'
}
# wait_for <segundos> <descrição> <comando...>
wait_for() {
  local timeout=$1 desc=$2; shift 2; local deadline=$(( $(date +%s) + timeout ))
  until "$@" >/dev/null 2>&1; do
    [ "$(date +%s)" -gt "$deadline" ] && fail "timeout esperando: $desc"
    sleep 3
  done
}

echo "── promtool check config"
docker run --rm -v "$PWD/prometheus:/p:ro" -v "$PWD/solutions:/s:ro" --entrypoint promtool prom/prometheus:v3.15.0 \
  check config /p/prometheus.yml >/dev/null || fail "prometheus.yml inválido"
docker run --rm -v "$PWD/solutions:/s:ro" --entrypoint promtool prom/prometheus:v3.15.0 \
  check config /s/07-retention.yml >/dev/null || fail "solutions/07-retention.yml inválido"
docker run --rm -v "$PWD/solutions:/s:ro" --entrypoint promtool prom/prometheus:v3.15.0 \
  check rules /s/08-backfill-rules.yml >/dev/null || fail "solutions/08-backfill-rules.yml inválido"

echo "── subindo a stack (do zero)"
docker compose down -v >/dev/null 2>&1 || true
docker compose up -d --build --wait >/dev/null 2>&1 || fail "docker compose up"
wait_for 60 "alvos UP" bash -c "[ \"\$(curl -sf '$PROM/api/v1/query?query=sum(up)' | grep -o '\"2\"')\" ]"

export END=$(( $(date +%s) / 3600 * 3600 - 2 * 3600 ))
T=$(( END - 3600 ))

echo "── 01 backfill OpenMetrics"
solutions/01-backfill-openmetrics.sh >/dev/null || fail "01: create-blocks-from openmetrics falhou"
nblocks=$(docker compose exec -T prometheus promtool tsdb list /prometheus | tail -n +2 | wc -l)
[ "$nblocks" -ge 12 ] || fail "01: esperava >= 12 blocos de 2h, achei $nblocks"

echo "── 02 top cardinality (analyze)"   # roda enquanto o Prometheus ainda não carregou os blocos
out=$(solutions/02-top-cardinality.sh) || fail "02: script falhou"
grep -A1 'Highest cardinality metric names' <<<"$out" | grep -qE '^1500 tsdb_demo_requests_total$' \
  || fail "02: analyze não apontou tsdb_demo_requests_total com 1500 séries"
grep -q "'name': 'tsdb_demo_requests_total', 'value': 1500" <<<"$out" || fail "02: /api/v1/status/tsdb top-1 errado"
curl -sf "$GEN/control?users=10" >/dev/null

echo "── 06 staleness"
out=$(solutions/06-staleness.sh) || fail "06: script falhou"
grep -q 'sem timestamp (stale marker):   "result":\[\]' <<<"$out" || fail "06: série sem timestamp deveria sumir na hora"
grep -q 'com timestamp (só lookback 5m): "result":\[{' <<<"$out" || fail "06: série com timestamp deveria continuar visível"
grep -q 'NaN' <<<"$out" || fail "06: tsdb dump deveria mostrar o stale marker (NaN)"
grep -qE 'alvo fora: up=.*"0"\]\}\].*séries=.*"0"\]' <<<"$out" || fail "06: alvo fora deveria ter up=0 e 0 séries"

echo "── 09 out-of-order"
out=$(solutions/09-out-of-order.sh) || fail "09: script falhou"
grep -qE 'head_out_of_order_samples_appended_total\{type="float"\} [1-9]' <<<"$out" || fail "09: nenhuma amostra OOO aceita"
grep -qE 'too_old_samples_total\{type="float"\} [1-9]' <<<"$out" || fail "09: nenhuma amostra too_old rejeitada"

echo "── 01 (cont.) esperando o Prometheus carregar os blocos importados"
wait_for 150 "blocos importados visíveis" bash -c "[ \"\$(curl -sf '$PROM/api/v1/query?query=count(tsdb_backfill_orders_total)&time=$T' | grep -o '\"2\"')\" ]"
n=$(q "count_over_time(tsdb_backfill_temperature_celsius{city=\"sao_paulo\"}[1d])" $((END - 1)))
[ "$n" = 1440 ] || fail "01: esperava 1440 amostras em 1 dia, achei '$n'"

echo "── 08 backfill de recording rule"
solutions/08-backfill-rules.sh >/dev/null 2>&1 || fail "08: create-blocks-from rules falhou"

echo "── 05 lookback delta"
hour=$(( (T / 3600) % 24 ))
[ "$(q tsdb_backfill_sparse_gauge $((T + 240)))" = "$hour" ] || fail "05: em T+4m deveria ver a amostra de T ($hour)"
[ -z "$(q tsdb_backfill_sparse_gauge $((T + 360)))" ] || fail "05: em T+6m (> 5m) não deveria ter resultado"
[ "$(q tsdb_backfill_sparse_gauge $((T + 360)) --data-urlencode lookback_delta=10m)" = "$hour" ] || fail "05: lookback_delta=10m deveria achar a amostra"
out=$(solutions/05-lookback-delta.sh) || fail "05: script falhou"
grep -q 'T+6m (fora dos 5m):    \[\]' <<<"$out" || fail "05: saída do script inesperada"

echo "── 04 snapshot"
solutions/04-snapshot.sh >/dev/null 2>&1 || fail "04: snapshot falhou"
name=$(cat /tmp/pca-snapshot-name)
docker compose exec -T prometheus test -d "/prometheus/snapshots/$name" || fail "04: diretório do snapshot não existe"
ls /tmp/pca-snapshot/*/meta.json >/dev/null 2>&1 || fail "04: cópia do snapshot sem blocos"

echo "── 03 delete_series + clean_tombstones"
out=$(solutions/03-delete-series.sh) || fail "03: admin API falhou"
grep -q curitiba <<<"$out" && fail "03: a série curitiba ainda existe"
grep -q sao_paulo <<<"$out" || fail "03: apagou demais (sao_paulo sumiu)"

echo "── 08 (cont.) esperando a recording rule retroativa"
# fora do horário comercial sp = 3 pedidos/min = 0.05/s; dentro (9h-18h UTC) = 9/min = 0.15/s
exp=0.05; h=$(( ((T - 1800) / 3600) % 24 )); { [ $h -ge 9 ] && [ $h -lt 18 ]; } && exp=0.15
wait_for 150 "store:tsdb_backfill_orders:rate5m" bash -c "[ -n \"\$(curl -sf '$PROM/api/v1/query?query=store:tsdb_backfill_orders:rate5m&time=$((T - 1800))' | grep -o sp)\" ]"
v=$(q 'store:tsdb_backfill_orders:rate5m{store="sp"}' $((T - 1800)))
python3 -c "import sys; sys.exit(0 if abs(float('$v') - $exp) < 0.01 else 1)" || fail "08: rate5m retroativo = $v, esperava ~$exp"

echo "── 07 retenção (15d -> 6h via config + reload)"
cp solutions/07-retention.yml "$CFG"
curl -sf -XPOST "$PROM/-/reload" || fail "07: reload falhou"
curl -sf "$PROM/api/v1/status/runtimeinfo" | grep -q '"storageRetention":"6h"' || fail "07: runtimeinfo não mostra 6h"
# retenção é medida a partir do bloco MAIS NOVO (fim do backfill = END), não do relógio
wait_for 150 "blocos antigos apagados" bash -c "[ -z \"\$(curl -sf '$PROM/api/v1/query?query=tsdb_backfill_orders_total&time=$((END - 20 * 3600))' | grep -o store)\" ]"
[ -n "$(q tsdb_backfill_orders_total $((END - 3600)))" ] || fail "07: apagou blocos recentes demais"
wait_for 30 "prometheus_tsdb_time_retentions_total > 0" bash -c "curl -sf '$PROM/api/v1/query?query=prometheus_tsdb_time_retentions_total>0' | grep -q value"

echo "── self-metrics"
for m in prometheus_tsdb_head_series prometheus_tsdb_wal_storage_size_bytes prometheus_tsdb_blocks_loaded; do
  [ -n "$(q "$m")" ] || fail "métrica $m ausente"
done

echo "PASS $LAB"
