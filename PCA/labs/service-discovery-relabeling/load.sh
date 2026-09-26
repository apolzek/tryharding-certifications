#!/usr/bin/env bash
# Valida (promtool) e carrega uma config no Prometheus do lab, sem reiniciar nada.
#   ./load.sh exercises/01-keep-team-payments/prometheus.yml          # Prometheus principal (9120)
#   ./load.sh solutions/06-hashmod-sharding/shard1.yml shard1         # 2º Prometheus (9129)
#   ./load.sh configs/base.yml                                        # volta ao estado inicial
#   FORCE=1 ./load.sh arquivo.yml   # carrega mesmo se o promtool reclamar (para ver o reload falhar)
set -euo pipefail
cd "$(dirname "$0")"
src=${1:?uso: ./load.sh <arquivo.yml> [prometheus|shard1]}
which=${2:-prometheus}
case "$which" in
  prometheus) dst=prometheus/prometheus.yml; port=9120 ;;
  shard1)     dst=prometheus/shard1.yml;     port=9129 ;;
  *) echo "destino inválido: $which (use prometheus ou shard1)"; exit 2 ;;
esac
abs=$(realpath "$src")
echo "── promtool check config $src"
if ! docker run --rm -v "$abs:/tmp/candidate.yml:ro" -v "$PWD/prometheus:/etc/prometheus:ro" \
     --entrypoint promtool prom/prometheus:v3.15.0 check config /tmp/candidate.yml; then
  [ "${FORCE:-0}" = 1 ] || { echo "✘ config inválida: nada foi carregado (use FORCE=1 para tentar mesmo assim)"; exit 1; }
fi
[ "$abs" = "$(realpath "$dst")" ] || cp "$src" "$dst"
echo "── POST localhost:$port/-/reload"
resp=$(curl -sS -o /dev/stderr -w '%{http_code}' -X POST "localhost:$port/-/reload" 2>&1) || true
if [ "${resp: -3}" = 200 ]; then
  echo "✔ recarregado ($dst)"
else
  echo "$resp" | sed '$ s/[0-9]\{3\}$//'; echo "✘ reload falhou: o Prometheus continua com a config ANTERIOR"; exit 1
fi
