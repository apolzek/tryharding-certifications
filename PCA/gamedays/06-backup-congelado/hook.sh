#!/usr/bin/env bash
# Simula a ÚLTIMA execução bem-sucedida do job de backup (depois disso o cron morreu).
set -euo pipefail
source "$(dirname "$0")/../_base/lib.sh"
cat <<M | curl -fsS --data-binary @- "$PUSHGW/metrics/job/backup/instance/db01"
# TYPE backup_last_status gauge
backup_last_status 1
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds $(date +%s)
# TYPE backup_duration_seconds gauge
backup_duration_seconds 42
M
echo "▶ job de backup rodou uma última vez às $(date +%T) e... nunca mais."
