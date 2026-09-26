#!/usr/bin/env bash
# Exercício 05: lookback delta. tsdb_backfill_sparse_gauge tem 1 amostra a cada 30 min.
set -euo pipefail
end=${END:-$(( $(date +%s) / 3600 * 3600 - 2 * 3600 ))}   # mesmo "fim" do backfill do exercício 01
T=$(( end - 3600 ))                                # existe uma amostra exatamente em T
q() { curl -sf localhost:9150/api/v1/query --data-urlencode 'query=tsdb_backfill_sparse_gauge' "$@" \
        | sed -E 's/.*"result":(.*)\}\}$/\1/'; echo; }
echo "T+4m (dentro dos 5m):  $(q --data-urlencode "time=$((T + 240))")"
echo "T+6m (fora dos 5m):    $(q --data-urlencode "time=$((T + 360))")"
echo "T+6m, lookback 10m:    $(q --data-urlencode "time=$((T + 360))" --data-urlencode lookback_delta=10m)"
echo "T+4m, lookback 1m:     $(q --data-urlencode "time=$((T + 240))" --data-urlencode lookback_delta=1m)"
