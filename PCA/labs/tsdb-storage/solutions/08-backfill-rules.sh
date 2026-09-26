#!/usr/bin/env bash
# Exercício 08: backfill de recording rule com promtool tsdb create-blocks-from rules.
set -euo pipefail
cd "$(dirname "$0")/.."
end=${END:-$(( $(date +%s) / 3600 * 3600 - 2 * 3600 ))}; start=$(( end - 24 * 3600 ))
# promtool roda DENTRO do container: --url aponta para o próprio Prometheus (localhost:9090 lá dentro)
docker compose exec -T prometheus sh -c "cat > /tmp/rules.yml && promtool tsdb create-blocks-from rules \
  --url=http://localhost:9090 --start=$start --end=$end --output-dir=/prometheus /tmp/rules.yml" \
  < solutions/08-backfill-rules.yml
