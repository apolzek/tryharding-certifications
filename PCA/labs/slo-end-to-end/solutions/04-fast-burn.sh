#!/usr/bin/env bash
# Exercício 04: injeta 5% de erro e acompanha o SLOErrorBudgetBurnFast: inactive -> pending -> firing.
#   ERROR_RATE=0.05 LATENCY_MS=0 ./solutions/04-fast-burn.sh
#   KEEP_CHAOS=1 mantém a falha no fim (padrão: volta ao normal)
set -euo pipefail
ALERT=${ALERT:-SLOErrorBudgetBurnFast}
curl -sf "localhost:9171/chaos?error_rate=${ERROR_RATE:-0.05}&latency_ms=${LATENCY_MS:-0}&latency_ratio=${LATENCY_RATIO:-1}"
start=$(date +%s); last=""
while :; do
  st=$(curl -sf localhost:9170/api/v1/alerts | python3 -c "
import json, sys
a = [x['state'] for x in json.load(sys.stdin)['data']['alerts'] if x['labels']['alertname'] == '$ALERT']
print(a[0] if a else 'inactive')")
  br=$(curl -sf localhost:9170/api/v1/query --data-urlencode 'query=job:slo_errors_per_request:ratio_rate1m / 0.001' \
       | python3 -c 'import json,sys; r=json.load(sys.stdin)["data"]["result"]; print(round(float(r[0]["value"][1]),1) if r else "-")')
  [ "$st" != "$last" ] && echo "t+$(( $(date +%s) - start ))s  $ALERT=$st  (burn rate 1m ≈ ${br}x)" && last=$st
  [ "$st" = firing ] && break
  [ $(( $(date +%s) - start )) -gt 180 ] && { echo "timeout: $ALERT não disparou"; exit 1; }
  sleep 1
done
[ "${KEEP_CHAOS:-0}" = 1 ] || curl -sf "localhost:9171/chaos?reset=1" >/dev/null
