TIMEOUT=60
check() {
  local v; v=$(qval 'job:http_request_duration_seconds:p99{job="checkout"}')
  [ -n "$v" ] || { echo 'job:http_request_duration_seconds:p99{job="checkout"} está vazio'; return 1; }
  awk -v a="$v" 'BEGIN{exit !(a>0.8 && a<1.0)}' ||
    { echo "o p99 registrado é $(printf '%.3f' "$v")s, mas 10% do tráfego leva ~0.75s (p99 real ≈ 0.95s)"; return 1; }
  alert_firing CheckoutLatenciaP99Alta || { echo "p99 ok, mas CheckoutLatenciaP99Alta ainda não está firing"; return 1; }
  echo "p99 real = $(printf '%.3f' "$v")s e CheckoutLatenciaP99Alta disparou"
}
