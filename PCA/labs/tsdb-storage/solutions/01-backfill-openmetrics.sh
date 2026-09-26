#!/usr/bin/env bash
# Exercício 01: importa 1 dia de dados históricos (OpenMetrics) como blocos TSDB.
set -euo pipefail
cd "$(dirname "$0")/.."
# 1) baixa o arquivo OpenMetrics (24h, 1 amostra/min, termina na hora cheia de 2h atrás)
END=${END:-$(( $(date +%s) / 3600 * 3600 - 2 * 3600 ))}
curl -sf "localhost:9151/backfill.om?end=$END" -o /tmp/pca-day.om
head -4 /tmp/pca-day.om; tail -1 /tmp/pca-day.om
# 2) gera os blocos DIRETO no diretório de dados do Prometheus (promtool roda dentro do container)
docker compose exec -T prometheus sh -c \
  'cat > /tmp/day.om && promtool tsdb create-blocks-from openmetrics /tmp/day.om /prometheus' < /tmp/pca-day.om
# 3) confere os blocos (o Prometheus os carrega sozinho em até ~1 min)
docker compose exec -T prometheus promtool tsdb list -r /prometheus
