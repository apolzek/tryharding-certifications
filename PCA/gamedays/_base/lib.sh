# Funções compartilhadas pelos scripts de gameday (source, não executa).
GD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$GD_DIR/work"
COMPOSE=(docker compose -f "$GD_DIR/_base/docker-compose.yml")
PROM="${PROM:-http://localhost:9180}"
AM="${AM:-http://localhost:9181}"
APP="${APP:-http://localhost:9182}"
PUSHGW="${PUSHGW:-http://localhost:9183}"
PROM_IMAGE="prom/prometheus:v3.15.0"

command -v jq >/dev/null || { echo "precisa de jq instalado"; exit 2; }

# scenario_dir 01  -> /.../gamedays/01-alerta-que-nunca-dispara
scenario_dir() {
  local id; id=$(printf '%02d' "$((10#${1:?informe o número do cenário, ex: 01}))")
  local d; d=$(ls -d "$GD_DIR/$id"-*/ 2>/dev/null | head -1)
  [ -n "$d" ] || { echo "cenário $1 não existe" >&2; return 1; }
  echo "${d%/}"
}

# q 'expr'  -> JSON .data.result da instant query
q() { curl -fsS --get "$PROM/api/v1/query" --data-urlencode "query=$1" | jq -c '.data.result'; }
# qval 'expr' -> primeiro valor (ou vazio)
qval() { q "$1" | jq -r '.[0].value[1] // empty'; }
# qcount 'expr' -> número de séries no resultado
qcount() { q "$1" | jq 'length'; }

# alert_firing NOME -> 0 se o alerta está firing no Prometheus
alert_firing() {
  curl -fsS "$PROM/api/v1/alerts" |
    jq -e --arg n "$1" '[.data.alerts[] | select(.labels.alertname==$n and .state=="firing")] | length > 0' >/dev/null
}

# rule_query NOME -> a expressão da regra carregada (alerta ou recording)
rule_query() {
  curl -fsS "$PROM/api/v1/rules" |
    jq -r --arg n "$1" '[.data.groups[].rules[] | select(.name==$n) | .query][0] // empty'
}

# reload_all -> recarrega Prometheus e Alertmanager; mostra erro se falhar
reload_all() {
  local rc=0 out
  out=$(curl -sS -X POST "$PROM/-/reload" -w '\n%{http_code}') || rc=1
  [ "$(tail -1 <<<"$out")" = "200" ] || { echo "⚠️  Prometheus reload falhou: $(head -n -1 <<<"$out")"; rc=1; }
  out=$(curl -sS -X POST "$AM/-/reload" -w '\n%{http_code}') || rc=1
  [ "$(tail -1 <<<"$out")" = "200" ] || { echo "⚠️  Alertmanager reload falhou: $(head -n -1 <<<"$out")"; rc=1; }
  return $rc
}

# promtool_test ARQ_DE_TESTE -> roda promtool test rules com work/prometheus montado em /work
promtool_test() {
  docker run --rm -v "$WORK/prometheus:/work:ro" -v "$(dirname "$1"):/check:ro" \
    --entrypoint promtool "$PROM_IMAGE" test rules "/check/$(basename "$1")"
}

# wait_until TIMEOUT cmd... -> repete cmd até dar certo
wait_until() {
  local t=$1; shift; local end=$((SECONDS + t))
  until "$@"; do [ $SECONDS -ge $end ] && return 1; sleep 2; done
}

# reloaded_after_edit -> 0 se o Prometheus recarregou (com sucesso) DEPOIS da última edição em work/prometheus
reloaded_after_edit() {
  local ok ts mt
  ok=$(qval 'prometheus_config_last_reload_successful{job="prometheus"}')
  ts=$(qval 'prometheus_config_last_reload_success_timestamp_seconds{job="prometheus"}')
  mt=$(stat -c %Y "$WORK"/prometheus/* | sort -n | tail -1)
  [ "$ok" = "1" ] && [ -n "$ts" ] && [ "${ts%.*}" -ge "$mt" ]
}
