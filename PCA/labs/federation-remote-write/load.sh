#!/usr/bin/env bash
# Valida (promtool) e carrega uma config em um dos Prometheus do lab via /-/reload.
#   ./load.sh exercises/01-federate-job-rules/global.yml global
#   ./load.sh configs/prom-b.yml prom-b         # volta ao estado inicial
#   ./load.sh reset                             # volta TODOS ao estado inicial
#   FORCE=1 ./load.sh arquivo.yml agent         # carrega mesmo se o promtool reclamar
set -euo pipefail
cd "$(dirname "$0")"
declare -A PORT=([prom-a]=9160 [prom-b]=9161 [global]=9162 [receiver]=9163 [agent]=9164)
if [ "${1:-}" = reset ]; then
  for s in prom-a prom-b global receiver agent; do "$0" "configs/$s.yml" "$s"; done
  exit 0
fi
src=${1:?uso: ./load.sh <arquivo.yml> <prom-a|prom-b|global|receiver|agent>}
svc=${2:?informe o destino: prom-a|prom-b|global|receiver|agent}
[ -n "${PORT[$svc]:-}" ] || { echo "destino inválido: $svc"; exit 2; }
agent_flag=(); [ "$svc" = agent ] && agent_flag=(--agent)
abs=$(realpath "$src"); dst="prometheus/$svc/prometheus.yml"
echo "── promtool check config ${agent_flag[*]} $src"
if ! out=$(docker run --rm -v "$abs:/tmp/candidate.yml:ro" -v "$PWD/prometheus/$svc:/etc/prometheus:ro" \
     --entrypoint promtool prom/prometheus:v3.15.0 check config "${agent_flag[@]}" /tmp/candidate.yml 2>&1); then
  echo "$out" | grep -v '^$'
  [ "${FORCE:-0}" = 1 ] || { echo "✘ config inválida: nada foi carregado (use FORCE=1 para tentar mesmo assim)"; exit 1; }
else
  echo "$out" | grep -v '^$'
fi
[ "$abs" = "$(realpath "$dst")" ] || cp "$src" "$dst"
echo "── POST localhost:${PORT[$svc]}/-/reload"
resp=$(curl -sS -o /dev/stderr -w '%{http_code}' -X POST "localhost:${PORT[$svc]}/-/reload" 2>&1) || true
if [ "${resp: -3}" = 200 ]; then
  echo "✔ recarregado $svc"
else
  echo "$resp" | sed '$ s/[0-9]\{3\}$//'; echo "✘ reload falhou: o Prometheus continua com a config ANTERIOR"; exit 1
fi
