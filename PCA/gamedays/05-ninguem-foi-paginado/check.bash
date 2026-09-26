TIMEOUT=45
check() {
  alert_firing CheckoutDown || { echo "CheckoutDown nem está firing no Prometheus (não mexa nas regras, o problema está depois)"; return 1; }
  curl -fsS "$APP/received" | jq -e '[.[] | select(.receiver=="pager-payments") | .alerts[]
      | select(.status=="firing" and .labels.alertname=="CheckoutDown")] | length > 0' >/dev/null ||
    { echo "o pager do time payments (receiver pager-payments) NUNCA recebeu CheckoutDown"; return 1; }
  echo "pager-payments recebeu CheckoutDown"
}
