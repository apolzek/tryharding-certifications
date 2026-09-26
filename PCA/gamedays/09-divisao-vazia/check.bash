TIMEOUT=60
check() {
  local qy; qy=$(rule_query SearchTaxaDeErroAlta)
  [ -n "$qy" ] || { echo "SearchTaxaDeErroAlta não está carregado"; return 1; }
  grep -q http_errors_total <<<"$qy" && grep -q http_requests_total <<<"$qy" ||
    { echo "a regra precisa continuar sendo erros / requisições"; return 1; }
  alert_firing SearchTaxaDeErroAlta ||
    { echo "o search está com ~20% de erro (2 de 10 req/s) e SearchTaxaDeErroAlta NÃO está firing"; return 1; }
  echo "SearchTaxaDeErroAlta firing"
}
