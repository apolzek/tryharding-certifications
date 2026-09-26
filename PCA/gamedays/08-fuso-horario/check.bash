TIMEOUT=20
check() {
  local out
  out=$(promtool_test "$S/check/tests.yml" 2>&1) || {
    echo "a janela de horário do alerta não bate com 09h-18h de Brasília:"
    grep -E 'time:|exp:|got:' <<<"$out" | head -9; return 1; }
  local qy; qy=$(rule_query BackofficeErrosHorarioComercial)
  [ -n "$qy" ] || { echo "BackofficeErrosHorarioComercial não está carregado"; return 1; }
  reloaded_after_edit || { echo "o arquivo está certo, mas o Prometheus ainda não carregou essa versão (rode ./reload.sh e veja se deu erro)"; return 1; }
  echo "alerta respeita 09h-18h de Brasília (testado com promtool) e está carregado"
}
