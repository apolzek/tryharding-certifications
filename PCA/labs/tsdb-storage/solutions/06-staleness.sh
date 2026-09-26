#!/usr/bin/env bash
# Exercício 06: stale markers (alvo vivo, série some) vs série com timestamp explícito vs alvo fora.
set -euo pipefail
cd "$(dirname "$0")/.."
q() { curl -sf localhost:9150/api/v1/query --data-urlencode "query=$1" | grep -o '"result":\[.*\]' ; }
curl -sf "localhost:9151/control?ephemeral=0&timestamped=0"; sleep 11
echo "sem timestamp (stale marker):   $(q tsdb_demo_ephemeral)"
echo "com timestamp (só lookback 5m): $(q tsdb_demo_timestamped)"
docker compose exec -T prometheus promtool tsdb dump --match=tsdb_demo_ephemeral \
  --min-time=$(( ($(date +%s) - 30) * 1000 )) /prometheus | grep NaN || true
curl -sf "localhost:9151/control?down=1"; sleep 11
echo "alvo fora: up=$(q 'up{job="generator"}')  séries=$(q 'count(tsdb_demo_requests_total) or vector(0)')"
curl -sf "localhost:9151/control?down=0&ephemeral=1&timestamped=1"
