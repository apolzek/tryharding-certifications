TIMEOUT=45
check() {
  [ "$(qval 'prometheus_config_last_reload_successful{job="prometheus"}')" = "1" ] ||
    { echo "prometheus_config_last_reload_successful == 0: o Prometheus continua com a config antiga"; return 1; }
  [ "$(qval 'up{job="inventory"}')" = "1" ] || { echo 'up{job="inventory"} ainda não existe (ou != 1)'; return 1; }
  local qy; qy=$(rule_query PrometheusConfigReloadFailed)
  grep -q prometheus_config_last_reload_successful <<<"$qy" ||
    { echo "falta o alerta PrometheusConfigReloadFailed (baseado em prometheus_config_last_reload_successful) para isso nunca mais passar batido"; return 1; }
  reloaded_after_edit || { echo "work/prometheus foi editado depois do último reload bem-sucedido (rode ./reload.sh)"; return 1; }
  echo "config nova carregada, inventory sendo raspado e meta-alerta no lugar"
}
