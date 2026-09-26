#!/usr/bin/env bash
# Teste automático do lab slo-end-to-end.
#   ./test.sh          # valida regras, sobe a stack, injeta chaos, confere alertas e Grafana, derruba
#   KEEP=1 ./test.sh   # não derruba no fim
set -euo pipefail
cd "$(dirname "$0")"
LAB=slo-end-to-end
PROM=localhost:9170
APP=localhost:9171
GRAFANA=localhost:3170
IMG=prom/prometheus:v3.15.0

fail() { echo "FAIL $LAB: $*"; exit 1; }
TMP=$(mktemp -d); chmod 755 "$TMP"
cleanup() {
  rm -rf "$TMP"
  [ "${KEEP:-0}" = 1 ] || docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

promtool() { docker run --rm -v "$1:/w" -w /w --entrypoint promtool "$IMG" "${@:2}"; }
# unit_test <arquivo de teste> <origem>=<nome no diretório do teste> ...
unit_test() {
  local d; d=$(mktemp -d -p "$TMP"); chmod 755 "$d"
  cp "$1" "$d/"; shift
  for pair in "$@"; do cp "${pair%%=*}" "$d/${pair##*=}"; done
  chmod 644 "$d"/*
  promtool "$d" test rules "$(ls "$d" | grep -m1 '\.test\.yml$')" >/dev/null 2>&1
}
# alert_state <alertname> -> inactive|pending|firing
alert_state() {
  curl -sf "$PROM/api/v1/alerts" | python3 -c "
import json, sys
a = [x['state'] for x in json.load(sys.stdin)['data']['alerts'] if x['labels']['alertname'] == '$1']
print(a[0] if a else 'inactive')"
}
q1() {  # primeiro valor de uma query instantânea (ou vazio)
  curl -sf "$PROM/api/v1/query" --data-urlencode "query=$1" \
    | python3 -c 'import json,sys; r=json.load(sys.stdin)["data"]["result"]; print(r[0]["value"][1] if r else "")'
}

echo "── promtool: check + unit tests"
docker run --rm -v "$PWD/prometheus:/etc/prometheus:ro" --entrypoint promtool "$IMG" \
  check config /etc/prometheus/prometheus.yml >/dev/null || fail "prometheus.yml inválido"
promtool "$PWD" check rules prometheus/rules/slo-lab.yml rules-production/slo-rules.yml >/dev/null || fail "regras inválidas"
promtool "$PWD/rules-production" test rules slo-rules.test.yml >/dev/null || fail "unit tests de produção (burn rate) falharam"
promtool "$PWD/prometheus/rules" test rules slo-lab.test.yml >/dev/null || fail "unit tests das regras escaladas falharam"
for f in solutions/*.yml; do promtool "$PWD" check rules "$f" >/dev/null || fail "$f inválido"; done
unit_test exercises/01-sli-recording-rules/rules.test.yml solutions/01-sli-recording-rules.yml=rules.yml || fail "ex01: solução não passa"
unit_test exercises/02-burn-rate-alert/alerts.test.yml exercises/02-burn-rate-alert/sli.yml=sli.yml solutions/02-burn-rate-alerts.yml=alerts.yml || fail "ex02: solução não passa"
unit_test exercises/03-error-budget/budget.test.yml solutions/03-error-budget.yml=budget.yml || fail "ex03: solução não passa"
unit_test exercises/05-latency-slo/latency.test.yml solutions/05-latency-slo.yml=latency.yml || fail "ex05: solução não passa"
# os enunciados (com TODO) NÃO podem passar
unit_test exercises/01-sli-recording-rules/rules.test.yml exercises/01-sli-recording-rules/rules.yml=rules.yml && fail "ex01: enunciado já passa?"
unit_test exercises/02-burn-rate-alert/alerts.test.yml exercises/02-burn-rate-alert/sli.yml=sli.yml exercises/02-burn-rate-alert/alerts.yml=alerts.yml && fail "ex02: enunciado já passa?"

echo "── subindo a stack"
docker compose down -v >/dev/null 2>&1 || true
docker compose up -d --build --wait >/dev/null 2>&1 || fail "docker compose up"
curl -sf "$PROM/api/v1/rules" | grep -q SLOErrorBudgetBurnFast || fail "regras do lab não carregaram"

deadline=$(( $(date +%s) + 60 ))
until [ -n "$(q1 job:slo_errors_per_request:ratio_rate1m)" ] && [ -n "$(q1 job:slo_error_budget_remaining:ratio)" ]; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "recording rules sem dados"
  sleep 2
done
[ "$(alert_state SLOErrorBudgetBurnFast)" = inactive ] || fail "fast burn ativo sem chaos"

echo "── Grafana: datasource e dashboard provisionados"
# o Grafana pode levar alguns segundos a mais que o Prometheus para ficar pronto
deadline=$(( $(date +%s) + 90 ))
until curl -sf "$GRAFANA/api/datasources/uid/prometheus/health" | grep -q '"status":"OK"'; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "datasource Prometheus não está OK no Grafana"
  sleep 3
done
deadline=$(( $(date +%s) + 60 ))
until curl -sf "$GRAFANA/api/dashboards/uid/slo-checkout" | grep -q '"title":"SLO · checkout (lab 1:60)"'; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "dashboard slo-checkout não provisionado"
  sleep 3
done
# a query de um painel, passando pelo Grafana, precisa devolver dados
curl -sf -XPOST "$GRAFANA/api/ds/query" -H 'Content-Type: application/json' -d "{
  \"from\": \"now-5m\", \"to\": \"now\",
  \"queries\": [{\"refId\": \"A\", \"datasource\": {\"uid\": \"prometheus\"},
                 \"expr\": \"job:slo_error_budget_remaining:ratio\", \"instant\": true}]}" \
  | grep -q '"values"' || fail "Grafana não conseguiu consultar job:slo_error_budget_remaining:ratio"

echo "── 04 chaos: 5% de erro -> SLOErrorBudgetBurnFast pending -> firing"
out=$(KEEP_CHAOS=1 solutions/04-fast-burn.sh) || { echo "$out"; fail "fast burn não disparou"; }
grep -q '=pending' <<<"$out" || fail "não vi o estado pending"
[ "$(alert_state SLOErrorBudgetBurnFast)" = firing ] || fail "/api/v1/alerts não mostra firing"
curl -sf "$PROM/api/v1/alerts" | grep -q '"severity":"page"' || fail "alerta sem severity=page"
br=$(q1 'job:slo_errors_per_request:ratio_rate1m / 0.001')
python3 -c "import sys; sys.exit(0 if 30 < float('$br') < 70 else 1)" || fail "burn rate 1m = $br, esperava ~50x"
budget=$(q1 job:slo_error_budget_remaining:ratio)
python3 -c "import sys; sys.exit(0 if float('$budget') < 0.5 else 1)" || fail "budget restante deveria ter caído (=$budget)"

echo "── janela curta: reset -> fast burn resolve em segundos"
curl -sf "$APP/chaos?reset=1" >/dev/null
deadline=$(( $(date +%s) + 30 ))
until [ "$(alert_state SLOErrorBudgetBurnFast)" = inactive ]; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "fast burn não resolveu após o reset (janela de 5s deveria derrubar)"
  sleep 1
done

echo "── 05 latência: 30% das requisições +400ms -> SLOLatencyBudgetBurnFast"
ALERT=SLOLatencyBudgetBurnFast LATENCY_MS=400 LATENCY_RATIO=0.3 ERROR_RATE=0.0005 solutions/04-fast-burn.sh >/dev/null \
  || fail "alerta de latência não disparou"
lat=$(q1 'histogram_fraction(0, 0.3, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[5s])))')
python3 -c "import sys; sys.exit(0 if 0.99 > float('$lat') else 1)" || fail "histogram_fraction deveria mostrar < 99% rápidas (=$lat)"

echo "PASS $LAB"
