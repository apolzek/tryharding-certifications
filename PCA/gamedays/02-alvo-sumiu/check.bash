TIMEOUT=30
check() {
  [ "$(qval 'up{job="payments"}')" = "1" ] || { echo 'up{job="payments"} não existe (ou != 1): o target continua fora do Prometheus'; return 1; }
  promtool_test "$S/check/tests.yml" >/tmp/gd-promtool.$$ 2>&1 || {
    echo "PaymentsDown não dispara quando o target SOME (teste promtool falhou):"; tail -5 /tmp/gd-promtool.$$; rm -f /tmp/gd-promtool.$$; return 1; }
  rm -f /tmp/gd-promtool.$$
  [ -n "$(rule_query PaymentsDown)" ] || { echo "PaymentsDown não está carregado no Prometheus (./reload.sh?)"; return 1; }
  reloaded_after_edit || { echo "o Prometheus não carregou a versão atual de work/prometheus (rode ./reload.sh e veja se deu erro)"; return 1; }
  echo "target payments de volta e PaymentsDown cobre o caso 'target sumiu'"
}
