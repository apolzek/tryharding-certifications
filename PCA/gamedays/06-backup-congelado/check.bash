TIMEOUT=60
check() {
  [ "$(qcount 'push_time_seconds{job="backup"}')" -ge 1 ] || {
    echo 'push_time_seconds{job="backup"} não existe: o label job veio do Prometheus (job="pushgateway") e não do backup'; return 1; }
  local qy; qy=$(rule_query BackupNaoRodou)
  [ -n "$qy" ] || { echo "BackupNaoRodou não está carregado"; return 1; }
  curl -fsS "$PROM/api/v1/alerts" | jq -e '[.data.alerts[] | select(.labels.alertname=="BackupNaoRodou"
      and .state=="firing" and .labels.job=="backup" and .labels.instance=="db01")] | length > 0' >/dev/null ||
    { echo "o backup não roda desde $(date -d @"$(qval 'max(push_time_seconds{job="backup"})' | cut -d. -f1)" +%T 2>/dev/null) e BackupNaoRodou{job=\"backup\",instance=\"db01\"} NÃO está firing"; return 1; }
  echo "BackupNaoRodou firing para job=backup instance=db01"
}
