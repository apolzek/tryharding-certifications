TIMEOUT=90
check() {
  local qy; qy=$(rule_query 'job:http_requests:rate1m')
  [ -n "$qy" ] || { echo "a recording rule job:http_requests:rate1m não está carregada"; return 1; }
  local mn mx
  mn=$(qval 'min_over_time(job:http_requests:rate1m{job="api"}[50s])')
  mx=$(qval 'max_over_time(job:http_requests:rate1m{job="api"}[50s])')
  [ -n "$mn" ] || { echo "job:http_requests:rate1m{job=\"api\"} sem dados"; return 1; }
  # a API atende ~10 req/s o tempo todo; qualquer coisa fora de 5..15 é mentira do painel
  awk -v a="$mn" 'BEGIN{exit !(a>5)}' || { echo "nos últimos 50s o painel mostrou $(printf '%.2f' "$mn") req/s (a API atende ~10 req/s sem parar)"; return 1; }
  awk -v a="$mx" 'BEGIN{exit !(a<15)}' || { echo "nos últimos 50s o painel mostrou $(printf '%.2f' "$mx") req/s (a API atende ~10 req/s)"; return 1; }
  alert_firing ApiSemTrafego && { echo "ApiSemTrafego ainda está firing"; return 1; }
  echo "throughput estável entre $(printf '%.1f' "$mn") e $(printf '%.1f' "$mx") req/s, mesmo com o pod reiniciando"
}
