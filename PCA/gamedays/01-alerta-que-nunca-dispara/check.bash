TIMEOUT=120
check() {
  local qy; qy=$(rule_query CheckoutErrosAltos)
  [ -n "$qy" ] || { echo "a regra CheckoutErrosAltos não está carregada"; return 1; }
  grep -q 'http_requests_total' <<<"$qy" || { echo "a regra não olha mais para http_requests_total"; return 1; }
  alert_firing CheckoutErrosAltos || { echo "o checkout está com ~20% de erro e CheckoutErrosAltos NÃO está firing"; return 1; }
  echo "CheckoutErrosAltos está firing"
}
