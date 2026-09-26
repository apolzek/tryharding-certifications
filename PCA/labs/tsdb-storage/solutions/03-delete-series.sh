#!/usr/bin/env bash
# Exercício 03: apaga uma série via Admin API e limpa os tombstones.
set -euo pipefail
# delete_series só MARCA (tombstone); a query já para de ver os dados na hora.
curl -sf -XPOST localhost:9150/api/v1/admin/tsdb/delete_series \
  --data-urlencode 'match[]=tsdb_backfill_temperature_celsius{city="curitiba"}'
# clean_tombstones reescreve os blocos afetados e libera o disco de verdade.
curl -sf -XPOST localhost:9150/api/v1/admin/tsdb/clean_tombstones
curl -sf localhost:9150/api/v1/series --data-urlencode 'match[]=tsdb_backfill_temperature_celsius' \
  --data-urlencode "start=$(( $(date +%s) - 3*86400 ))"; echo
