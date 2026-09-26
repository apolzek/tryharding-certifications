#!/usr/bin/env bash
# Teste automático do lab de instrumentação.
#   Fase A: stack do ENUNCIADO (métricas boas + ruins) -> verifica que os problemas existem.
#   Fase B: stack do GABARITO (solutions/) -> verifica que cada exercício foi resolvido.
# KEEP=1 ./test.sh  -> não derruba a stack no fim (fica a do gabarito).
set -euo pipefail
cd "$(dirname "$0")"
LAB=instrumentation
PROM=http://localhost:9130
PROMTOOL=(docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0)
SOL=(-f docker-compose.yml -f solutions/docker-compose.solutions.yml)

fail() { echo "FAIL $LAB: $*"; exit 1; }
cleanup() { [ "${KEEP:-0}" = 1 ] || docker compose "${SOL[@]}" down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT

# q EXPR -> "valor por linha" (label job/replica opcional na frente)
q() {
  curl -sf "$PROM/api/v1/query" --data-urlencode "query=$1" | python3 -c '
import json, sys
for r in json.load(sys.stdin)["data"]["result"]:
    print(r["value"][1])'
}
# expect DESC EXPR LO HI [TIMEOUT]: espera até TODOS os resultados (>=1) ficarem em [LO,HI]
expect() {
  local desc=$1 expr=$2 lo=$3 hi=$4 deadline=$(( $(date +%s) + ${5:-120} )) out
  until out=$(q "$expr") && [ -n "$out" ] && awk -v lo="$lo" -v hi="$hi" '{ if ($1+0 < lo || $1+0 > hi) bad=1 } END { exit bad }' <<<"$out"; do
    [ "$(date +%s)" -gt "$deadline" ] && fail "$desc: esperado [$lo,$hi], veio: $(tr '\n' ' ' <<<"${out:-<vazio>}")"
    sleep 3
  done
  echo "  ok  $desc = $(tr '\n' ' ' <<<"$out")"
}
expect_empty() {
  local out; out=$(q "$2")
  [ -z "$out" ] || fail "$1: esperava vazio, veio $out"
  echo "  ok  $1 (vazio)"
}
# mtype JOB METRIC TYPES_REGEX: tipo da métrica via /api/v1/targets/metadata
mtype() {
  # no OpenMetrics o nome da FAMÍLIA não tem _total/_info (ex.: "# TYPE http_requests counter")
  local t
  t=$(curl -sf -G "$PROM/api/v1/targets/metadata" --data-urlencode "match_target={job=\"$1\"}" |
      python3 -c 'import json,sys,re
m=sys.argv[1]; names={m, re.sub("_(total|info)$","",m)}
print(" ".join(sorted({x["type"] for x in json.load(sys.stdin)["data"] if x["metric"] in names})))' "$2")
  [[ "$t" =~ ^($3)$ ]] || fail "metadata $1/$2: esperado $3, veio '${t:-<nada>}'"
  echo "  ok  metadata $1 $2 = $t"
}
lint() { curl -sf "$1" | "${PROMTOOL[@]}" check metrics 2>&1; }

echo "── validando config"
docker run --rm -v "$PWD/prometheus:/p:ro" --entrypoint promtool prom/prometheus:v3.15.0 check config /p/prometheus.yml >/dev/null \
  || fail "prometheus.yml inválido"

# ═════════════════════════ FASE A: enunciado ═════════════════════════
echo "── fase A: subindo stack do enunciado"
docker compose "${SOL[@]}" down -v >/dev/null 2>&1 || true
docker compose up -d --build --wait >/dev/null 2>&1 || fail "stack do enunciado não subiu"

for app in 9131 9132; do
  lint "localhost:$app/metrics/good" >/dev/null || fail "promtool reclamou de /metrics/good em :$app"
  echo "  ok  :$app/metrics/good passa no promtool"
  out=$(lint "localhost:$app/metrics") && fail ":$app/metrics deveria falhar no promtool"
  for pat in "camelCase" "abbreviated units" "no help text" 'non-counter metrics should not have "_total"'; do
    grep -q "$pat" <<<"$out" || fail ":$app/metrics: promtool não apontou '$pat'"
  done
  echo "  ok  :$app/metrics reprovado pelo promtool (camelCase, unidade, help, _total)"
done
grep -q 'counter metrics should have "_total" suffix' <<<"$(lint localhost:9131/metrics/bad)" || fail "Go: requestsCount sem _total não foi apontado"

# formatos de exposição (exercício 07)
curl -sf -H 'Accept: application/openmetrics-text; version=1.0.0' localhost:9131/metrics | grep -q '^# EOF' || fail "OpenMetrics sem # EOF"
curl -sf -H 'Accept: application/openmetrics-text; version=1.0.0' localhost:9132/metrics | grep -q '^http_requests_created' || fail "Python OM sem _created"
curl -sf localhost:9132/metrics | grep -q '^# TYPE http_requests_created gauge' || fail "Python text 0.0.4 sem _created como gauge"
ct=$(curl -sfo /dev/null -w '%{content_type}' -H 'Accept: application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited' localhost:9131/metrics)
[[ "$ct" == application/vnd.google.protobuf* ]] || fail "Go não negociou protobuf (veio $ct)"
echo "  ok  negociação text/OpenMetrics/protobuf"

expect "4 targets up" 'count(up == 1)' 4 4 90
expect "jobs com tráfego" 'count(sum by (job) (rate(http_requests_total[1m])) > 0)' 2 2
for job in app-go app-python; do
  mtype $job http_requests_total counter
  mtype $job http_request_duration_seconds histogram
  mtype $job http_requests_in_flight gauge
  mtype $job backend_call_duration_seconds summary
done
expect "bomba de cardinalidade (séries por job)" 'count by (job) (http_requests_by_user_total)' 100 100000
expect "taxa de erro 5xx" 'sum by (job) (rate(http_requests_total{code=~"5.."}[1m])) / sum by (job) (rate(http_requests_total[1m]))' 0.001 0.08
expect "native histogram do Go" 'sum(histogram_count(rate(http_request_duration_seconds{job="app-go"}[1m])))' 1 200
expect "p99 do summary na réplica b (lenta)" 'backend_call_duration_seconds{quantile="0.99",replica="b"}' 0.2 0.26
expect "média dos p99 (resposta ERRADA)" 'avg(backend_call_duration_seconds{quantile="0.99"})' 0.12 0.18
expect_empty "sem bucket le=0.3 no enunciado" 'http_request_duration_seconds_bucket{le="0.3"}'
for job in app-go app-python; do
  n=$(curl -sf -G "$PROM/api/v1/query_exemplars" --data-urlencode "query=http_request_duration_seconds_bucket{job=\"$job\"}" \
        --data-urlencode "start=$(( $(date +%s) - 300 ))" --data-urlencode "end=$(date +%s)" | grep -o '"trace_id"' | wc -l)
  [ "$n" -gt 0 ] || fail "sem exemplars para $job"
  echo "  ok  exemplars com trace_id em $job ($n)"
done

# ═════════════════════════ FASE B: gabarito ═════════════════════════
echo "── fase B: subindo stack do gabarito (solutions/)"
docker compose down -v >/dev/null 2>&1
docker compose "${SOL[@]}" up -d --build --wait >/dev/null 2>&1 || fail "stack do gabarito não subiu"

for app in 9131 9132; do
  out=$(lint "localhost:$app/metrics") || fail "EX01: promtool ainda reclama em :$app/metrics: $out"
  echo "  ok  EX01 :$app/metrics passa no promtool"
done
expect "4 targets up" 'count(up == 1)' 4 4 90
expect "jobs com tráfego" 'count(sum by (job) (rate(payments_total[1m])) > 0)' 2 2
for job in app-go app-python; do
  mtype $job checkouts_total counter
  mtype $job orders_processed_total counter
  mtype $job checkout_duration_seconds histogram
  mtype $job cache_size_bytes gauge
  mtype $job cart_items_total counter
  mtype $job checkouts_by_plan_total counter
  mtype $job app_info 'gauge|info'
  mtype $job payments_total counter
  mtype $job payment_duration_seconds histogram
done
mtype app-go backend_call_duration_seconds histogram
expect "EX02 fração do checkout <= 300ms (SLI)" \
  'sum by (job) (rate(http_request_duration_seconds_bucket{route="/api/checkout",le="0.3"}[1m])) / sum by (job) (rate(http_request_duration_seconds_count{route="/api/checkout"}[1m]))' 0.75 0.98
expect "EX03 p99 agregado das 2 réplicas (histogram)" \
  'histogram_quantile(0.99, sum by (le) (rate(backend_call_duration_seconds_bucket{job="app-go"}[1m])))' 0.2 0.26
expect_empty "EX04 bomba de cardinalidade removida" 'http_requests_by_user_total'
expect "EX04 cardinalidade limitada (séries por instância)" 'count by (instance) (checkouts_by_plan_total)' 1 3
expect "EX05 app_info por instância" 'count(app_info{version="1.4.2"})' 3 3
expect "EX06 taxa de pagamentos recusados" 'sum by (job) (rate(payments_total{result="declined"}[2m])) / sum by (job) (rate(payments_total[2m]))' 0.02 0.25
expect "EX06 p90 do pagamento" 'histogram_quantile(0.9, sum by (job, le) (rate(payment_duration_seconds_bucket[1m])))' 0.02 0.06

echo "PASS $LAB"
