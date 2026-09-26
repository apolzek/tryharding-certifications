#!/usr/bin/env bash
# Exercício 02: explode a cardinalidade e descobre a métrica culpada com promtool tsdb analyze.
set -euo pipefail
cd "$(dirname "$0")/.."
curl -sf "localhost:9151/control?users=1500"; sleep 12   # 2+ scrapes
# A head (memória) não é um bloco: o analyze lê BLOCOS. O snapshot transforma a head em bloco.
name=$(curl -sf -XPOST localhost:9150/api/v1/admin/tsdb/snapshot | sed -E 's/.*"name":"([^"]+)".*/\1/')
docker compose exec -T prometheus promtool tsdb analyze --limit=5 "/prometheus/snapshots/$name" \
  | sed -n '/Highest cardinality metric names/,$p'
# Mesma resposta sem promtool, direto da head:
curl -sf localhost:9150/api/v1/status/tsdb | python3 -c \
  'import json,sys; d=json.load(sys.stdin)["data"]; print("API /status/tsdb top-1:", d["seriesCountByMetricName"][0])'
