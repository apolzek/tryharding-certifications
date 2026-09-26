TIMEOUT=45
check() {
  local n; n=$(qval 'scrape_samples_post_metric_relabeling{job="checkout"}')
  [ -n "$n" ] || { echo 'sem scrape_samples_post_metric_relabeling{job="checkout"}: o job checkout está sendo raspado?'; return 1; }
  [ "$(qval 'up{job="checkout"}')" = "1" ] || { echo 'up{job="checkout"} != 1: o "conserto" derrubou o scrape (sample_limit baixo demais?)'; return 1; }
  [ "${n%.*}" -lt 200 ] || { echo "o checkout ainda ingere $n amostras por scrape (limite do gameday: < 200)"; return 1; }
  [ "$(qcount 'http_requests_total{job="checkout"}')" -ge 1 ] || { echo "http_requests_total{job=\"checkout\"} sumiu: jogou fora métrica boa junto!"; return 1; }
  local live; live=$(qval 'count({job="checkout"})')
  [ "${live%.*}" -lt 200 ] || { echo "ainda há $live séries vivas do job checkout"; return 1; }
  echo "checkout ingere $n amostras/scrape e as métricas boas continuam lá"
}
