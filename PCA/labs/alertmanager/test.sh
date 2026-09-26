#!/usr/bin/env bash
# Teste automático do lab de Alertmanager.
# Para cada exercício: sobe Prometheus + Alertmanager com a SOLUÇÃO (EX=solutions/NN-x),
# liga os gauges no app e verifica no receiver espião o que chegou (ou NÃO chegou).
#   ./test.sh          # testa tudo e derruba a stack
#   KEEP=1 ./test.sh   # deixa a stack no ar no fim
set -euo pipefail
cd "$(dirname "$0")"
LAB=labs/alertmanager
APP=localhost:9113 RCV=localhost:9112 PROM=localhost:9110 AM=localhost:9111
AMTOOL_IMG=prom/alertmanager:v0.34.1

cleanup() { [ "${KEEP:-0}" = 1 ] || docker compose down -v >/dev/null 2>&1 || true; }
trap cleanup EXIT
fail() {
  echo "FAIL $LAB: $*"
  echo "--- notificações recebidas:"; curl -s "$RCV/received" | jq -c '.[] | {hook, status, groupLabels, alerts: [.alerts[] | .labels.instance]}' 2>/dev/null || true
  exit 1
}
step() { echo "── $*"; }

set_g() { curl -sf "$APP/set?$1" >/dev/null || fail "app /set?$1"; }
received() { curl -sf "$RCV/received"; }

# wait_rcv <segundos> <descrição> <expressão jq sobre /received que deve dar true>
wait_rcv() {
  local end=$((SECONDS + $1))
  until received | jq -e "$3" >/dev/null 2>&1; do
    [ $SECONDS -lt $end ] || fail "timeout ($1s): $2"
    sleep 1
  done
}
# never_rcv <segundos> <descrição> <expressão jq que NÃO pode ficar true nesse período>
never_rcv() {
  local end=$((SECONDS + $1))
  while [ $SECONDS -lt $end ]; do
    received | jq -e "$3" >/dev/null 2>&1 && fail "chegou o que não devia: $2"
    sleep 1
  done
}
# wait_http <segundos> <descrição> <url> <expressão jq>
wait_http() {
  local end=$((SECONDS + $1))
  until curl -sf "$3" | jq -e "$4" >/dev/null 2>&1; do
    [ $SECONDS -lt $end ] || fail "timeout ($1s): $2"
    sleep 1
  done
}
# amtool offline (sem stack) para testar árvore de rotas de um arquivo
routes_test() { # <dir> <labels...>
  local d=$1; shift
  docker run --rm -v "$PWD/$d:/c:ro" --entrypoint amtool $AMTOOL_IMG config routes test --config.file=/c/alertmanager.yml "$@" 2>/dev/null | tail -1
}
use_ex() {
  step "EX=$1"
  curl -sf "$APP/reset" >/dev/null 2>&1 || true
  EX="./$1" docker compose up -d --force-recreate --wait prometheus alertmanager >/dev/null 2>&1 || fail "stack não subiu com EX=$1"
  curl -sf "$APP/reset" >/dev/null && curl -sf -X POST "$RCV/reset" >/dev/null || fail "reset app/receiver"
}
hook_has() { # <hook> <alertname> [instance]  -> expressão jq
  local inst=${3:-}
  echo "[.[] | select(.hook==\"$1\" and .status==\"firing\") | .alerts[] | select(.labels.alertname==\"$2\"${inst:+ and .labels.instance==\"$inst\"})] | length > 0"
}

# ─── validação estática de todos os arquivos ─────────────────────────────────
step "amtool check-config / promtool check rules"
docker run --rm -v "$PWD:/lab:ro" -w /lab --entrypoint amtool $AMTOOL_IMG \
  check-config config/alertmanager.yml exercises/*/alertmanager.yml solutions/*/alertmanager.yml >/dev/null 2>&1 \
  || fail "algum alertmanager.yml é inválido (rode amtool check-config)"
docker run --rm -v "$PWD:/lab:ro" -w /lab --entrypoint promtool prom/prometheus:v3.15.0 \
  check rules prometheus/rules/*.yml exercises/*/*.rules.yml solutions/*/*.rules.yml >/dev/null 2>&1 \
  || fail "alguma regra é inválida (rode promtool check rules)"

# ─── os exercícios quebrados precisam estar de fato quebrados (routes test) ─
step "exercícios quebrados roteiam errado"
[ "$(routes_test exercises/01-roteamento-severidade severity=critical)" = slack ] || fail "ex01 quebrado deveria mandar critical p/ slack"
[ "$(routes_test exercises/05-arvore-de-rotas team=db severity=critical)" = pager ] || fail "ex05 quebrado deveria dar só pager"
[ "$(routes_test exercises/05-arvore-de-rotas team=web severity=warning)" = default ] || fail "ex05 quebrado: regex 'warn' não deveria casar"

# ─── routes test das soluções ───────────────────────────────────────────────
step "amtool config routes test nas soluções"
[ "$(routes_test solutions/01-roteamento-severidade severity=critical)" = pager ] || fail "sol01 critical"
[ "$(routes_test solutions/01-roteamento-severidade severity=warning)" = slack ] || fail "sol01 warning"
for row in "team=db severity=critical:dba,pager" "team=db severity=warning:dba,slack" \
           "team=web severity=critical:pager" "team=web severity=warning:slack" "severity=info:default"; do
  labels=${row%%:*}; want=${row#*:}
  # shellcheck disable=SC2086
  docker run --rm -v "$PWD/solutions/05-arvore-de-rotas:/c:ro" --entrypoint amtool $AMTOOL_IMG \
    config routes test --config.file=/c/alertmanager.yml --verify.receivers="$want" $labels >/dev/null 2>&1 \
    || fail "sol05: '$labels' deveria ir para $want (got: $(routes_test solutions/05-arvore-de-rotas $labels))"
done

step "subindo a stack"
docker compose up -d --wait >/dev/null 2>&1 || fail "docker compose up"

# ─── config padrão: roteamento + inibição ───────────────────────────────────
use_ex config
set_g "name=lab_up&value=0&instance=api-1&cluster=eu"
wait_rcv 45 "config: ServiceDown no pager" "$(hook_has pager ServiceDown api-1)"
set_g "name=lab_latency_seconds&value=2&instance=api-1&cluster=eu"
set_g "name=lab_latency_seconds&value=2&instance=api-2&cluster=us"
wait_rcv 45 "config: HighLatency us no slack" "$(hook_has slack HighLatency api-2)"
never_rcv 8 "config: HighLatency eu deveria estar inibido" "$(hook_has slack HighLatency api-1)"
curl -sf "$PROM/api/v1/query?query=ALERTS_FOR_STATE" | jq -e '.data.result | length >= 3' >/dev/null || fail "ALERTS_FOR_STATE vazio"

# ─── 01 roteamento por severidade ───────────────────────────────────────────
use_ex solutions/01-roteamento-severidade
set_g "name=lab_up&value=0&instance=api-1&cluster=eu"
set_g "name=lab_latency_seconds&value=2&instance=api-2&cluster=us"
wait_rcv 45 "01: ServiceDown no pager" "$(hook_has pager ServiceDown)"
wait_rcv 45 "01: HighLatency no slack" "$(hook_has slack HighLatency)"
received | jq -e "$(hook_has slack ServiceDown) | not" >/dev/null || fail "01: ServiceDown também foi p/ slack"
received | jq -e "$(hook_has pager HighLatency) | not" >/dev/null || fail "01: HighLatency foi p/ pager"

# ─── 02 agrupamento ────────────────────────────────────────────────────────
use_ex solutions/02-agrupamento
set_g "name=lab_latency_seconds&value=2&instance=api-1&cluster=eu"
set_g "name=lab_latency_seconds&value=2&instance=api-2&cluster=eu"
set_g "name=lab_latency_seconds&value=2&instance=api-3&cluster=us"
wait_rcv 45 "02: grupo eu com 2 alertas e grupo us com 1" \
  '([.[] | select(.groupLabels=={"alertname":"HighLatency","cluster":"eu"})] | last | .alerts | length) == 2
   and ([.[] | select(.groupLabels=={"alertname":"HighLatency","cluster":"us"})] | last | .alerts | length) == 1'
received | jq -e '[.[].groupKey] | unique | length == 2' >/dev/null || fail "02: esperava exatamente 2 grupos"

# ─── 03 inibição ────────────────────────────────────────────────────────────
use_ex solutions/03-inibicao
set_g "name=lab_up&value=0&instance=api-1&cluster=eu"
wait_rcv 45 "03: ServiceDown (source) chegou" "$(hook_has pager ServiceDown)"
set_g "name=lab_latency_seconds&value=2&instance=api-2&cluster=eu"
set_g "name=lab_latency_seconds&value=2&instance=api-3&cluster=us"
wait_rcv 45 "03: warning do cluster us chegou" "$(hook_has slack HighLatency api-3)"
never_rcv 8 "03: warning do cluster eu deveria estar inibido" "$(hook_has slack HighLatency api-2)"
curl -sf "$AM/api/v2/alerts" | jq -e '[.[] | select(.labels.instance=="api-2") | .status.inhibitedBy | length] == [1]' >/dev/null \
  || fail "03: api-2 deveria ter status.inhibitedBy preenchido"

# ─── 04 silence com amtool ──────────────────────────────────────────────────
use_ex solutions/04-silenciar
sid=$(./solutions/04-silenciar/silence.sh | tr -d '\r')
[[ $sid =~ ^[0-9a-f-]{36}$ ]] || fail "04: silence add não devolveu ID ($sid)"
A="docker compose exec -T alertmanager amtool --alertmanager.url=http://localhost:9093"
$A silence query -q instance=api-2 | grep -q "$sid" || fail "04: silence query não achou o silence"
set_g "name=lab_latency_seconds&value=2&instance=api-1&cluster=eu"
set_g "name=lab_latency_seconds&value=2&instance=api-2&cluster=eu"
wait_rcv 45 "04: api-1 chegou" "$(hook_has slack HighLatency api-1)"
never_rcv 8 "04: api-2 está silenciado" "$(hook_has slack HighLatency api-2)"
curl -sf "$AM/api/v2/alerts" | jq -e --arg s "$sid" '[.[] | select(.labels.instance=="api-2") | .status.silencedBy[]] == [$s]' >/dev/null \
  || fail "04: api-2 deveria ter status.silencedBy=$sid"
$A silence expire "$sid" >/dev/null || fail "04: silence expire"
wait_rcv 40 "04: após expirar, api-2 chega" "$(hook_has slack HighLatency api-2)"

# ─── 05 árvore de rotas (ao vivo: dba E pager) ─────────────────────────────
use_ex solutions/05-arvore-de-rotas
set_g "name=lab_up&value=0&instance=pg-1&cluster=eu&team=db"
wait_rcv 45 "05: ServiceDown team=db no dba" "$(hook_has dba ServiceDown pg-1)"
wait_rcv 20 "05: ServiceDown team=db também no pager (continue)" "$(hook_has pager ServiceDown pg-1)"

# ─── 06 pending -> firing com for: 1m ───────────────────────────────────────
use_ex solutions/06-pending-for
set_g "name=lab_queue_depth&value=500&queue=orders&cluster=eu"
wait_http 30 "06: QueueBacklog pending" "$PROM/api/v1/alerts" '.data.alerts[] | select(.labels.alertname=="QueueBacklog" and .state=="pending")'
t0=$SECONDS
wait_http 15 "06: ALERTS{alertstate=\"pending\"} deveria existir" \
  "$PROM/api/v1/query?query=ALERTS%7Balertname%3D%22QueueBacklog%22%2Calertstate%3D%22pending%22%7D" '.data.result | length == 1'
never_rcv 40 "06: nada pode ser enviado enquanto pending" "$(hook_has slack QueueBacklog)"
wait_http 40 "06: QueueBacklog firing" "$PROM/api/v1/alerts" '.data.alerts[] | select(.labels.alertname=="QueueBacklog" and .state=="firing")'
wait_rcv 30 "06: notificação após o for" "$(hook_has slack QueueBacklog)"
[ $((SECONDS - t0)) -ge 50 ] || fail "06: disparou rápido demais ($((SECONDS - t0))s), o for: 1m não está valendo"

# ─── 07 send_resolved + templates ───────────────────────────────────────────
use_ex solutions/07-resolved-e-templates
set_g "name=lab_checkout_error_ratio&value=0.25&service=checkout&cluster=eu"
wait_rcv 45 "07: summary 'checkout com 25% de erros'" \
  '.[] | select(.status=="firing") | .alerts[] | select(.annotations.summary=="checkout com 25% de erros")'
set_g "name=lab_checkout_error_ratio&value=0&service=checkout&cluster=eu"
wait_rcv 45 "07: notificação resolved" '.[] | select(.status=="resolved" and .alerts[0].labels.alertname=="CheckoutErrors")'

# ─── 08 mute_time_intervals ─────────────────────────────────────────────────
use_ex solutions/08-janela-manutencao
set_g "name=lab_latency_seconds&value=2&instance=b-1&cluster=eu&team=batch"
set_g "name=lab_latency_seconds&value=2&instance=w-1&cluster=us&team=web"
wait_rcv 45 "08: warning do team=web chegou" "$(hook_has slack HighLatency w-1)"
never_rcv 8 "08: team=batch está na janela de manutenção" "$(hook_has slack HighLatency b-1)"

echo "PASS $LAB (config padrão + 8 exercícios)"
