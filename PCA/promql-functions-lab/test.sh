#!/usr/bin/env bash
# Teste do lab de funções: garante stack no ar e roda todas as queries das 90 lições.
# A stack NÃO é derrubada (ela é usada pelos desafios e por labs/promql-operators).
set -uo pipefail
cd "$(dirname "$0")"
tools/check-go.sh >/dev/null || { echo "FAIL promql-functions-lab: cenários Go não compilam"; exit 1; }
if ! curl -sf localhost:9095/-/ready >/dev/null || ! curl -sf localhost:8088/metrics >/dev/null; then
  tools/deploy.sh >/dev/null || { echo "FAIL promql-functions-lab: deploy"; exit 1; }
fi
docker run --rm -v "$PWD/prometheus:/p:ro" -v "$PWD/prometheus/rules:/etc/prometheus/rules:ro" --entrypoint promtool prom/prometheus:v3.15.0 \
  --enable-feature=promql-experimental-functions check config /p/prometheus.yml >/dev/null 2>&1 \
  || { echo "FAIL promql-functions-lab: prometheus.yml/rules inválidos"; exit 1; }
# janelas de até 10m: re-tenta por até 12 min enquanto os dados acumulam
deadline=$(( $(date +%s) + 720 ))
while :; do
  out=$(python3 tools/validate.py $(ls functions) 2>&1)
  failed=$(grep -c ': FALHOU' <<<"$out")
  [ "$failed" -eq 0 ] && break
  if [ "$(date +%s)" -gt "$deadline" ]; then
    grep -A3 ': FALHOU' <<<"$out"; echo "FAIL promql-functions-lab: $failed lições falharam"; exit 1
  fi
  sleep 30
done
echo "PASS promql-functions-lab (90 lições)"
