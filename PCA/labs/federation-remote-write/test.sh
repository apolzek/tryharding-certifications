#!/usr/bin/env bash
# Teste automático do lab federation-remote-write.
# Sobe a stack, confere que os dados FLUEM (federation, remote write, agent, remote read)
# e, para cada exercício, carrega a versão quebrada (confere o sintoma) e a solução
# (confere o conserto com dados frescos). KEEP=1 mantém a stack no fim.
set -euo pipefail
cd "$(dirname "$0")"
LAB=federation-remote-write
IMG=prom/prometheus:v3.15.0
declare -A PORT=([prom-a]=9160 [prom-b]=9161 [global]=9162 [receiver]=9163 [agent]=9164)
SVCS=(prom-a prom-b global receiver agent)

fail() { echo "FAIL $LAB: $*"; exit 1; }

cleanup() {
  for s in "${SVCS[@]}"; do cp "configs/$s.yml" "prometheus/$s/prometheus.yml"; done
  if [ "${KEEP:-0}" = 1 ]; then
    for s in "${SVCS[@]}"; do curl -fsS -X POST "localhost:${PORT[$s]}/-/reload" >/dev/null 2>&1 || true; done
  else
    docker compose down -v >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# ---------- helpers ----------
# n <svc> <promql> -> número de séries do resultado
n() { curl -fsS "localhost:${PORT[$1]}/api/v1/query" --data-urlencode "query=$2" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["data"]["result"]))'; }
# v <svc> <promql> -> valor da 1ª série (ou none)
v() { curl -fsS "localhost:${PORT[$1]}/api/v1/query" --data-urlencode "query=$2" | python3 -c 'import json,sys; r=json.load(sys.stdin)["data"]["result"]; print(r[0]["value"][1] if r else "none")'; }
# fresh <svc> <seletor> -> nº de séries com amostra nos últimos 20s (ignora séries "velhas" do lookback)
fresh() { n "$1" "last_over_time($2[20s])"; }
eq() { [ "$($1 "${@:2:$#-2}")" = "${!#}" ]; }
gt0() { [ "$($1 "${@:2}")" -gt 0 ]; }
wait_for() {  # wait_for <descrição> <comando...>  (timeout 75s)
  local desc=$1; shift
  local deadline=$(( $(date +%s) + 75 ))
  until "$@" >/dev/null 2>&1; do
    [ "$(date +%s)" -gt "$deadline" ] && fail "$desc"
    sleep 2
  done
  echo "  ok  $desc"
}
promtool_ok() {  # promtool_ok <arquivo> <svc>
  local fl=(); [ "$2" = agent ] && fl=(--agent)
  docker run --rm -v "$PWD/$1:/tmp/c.yml:ro" -v "$PWD/prometheus/$2:/etc/prometheus:ro" \
    --entrypoint promtool "$IMG" check config "${fl[@]}" /tmp/c.yml >/dev/null 2>&1
}
load() {  # load <arquivo> <svc>
  promtool_ok "$1" "$2" || fail "promtool rejeitou $1"
  cp "$1" "prometheus/$2/prometheus.yml"
  curl -fsS -X POST "localhost:${PORT[$2]}/-/reload" >/dev/null || fail "reload de $1 em $2 falhou"
}

# ---------- sobe ----------
for s in "${SVCS[@]}"; do cp "configs/$s.yml" "prometheus/$s/prometheus.yml"; done
docker compose up -d --wait >/dev/null 2>&1 || fail "docker compose up"
for s in "${SVCS[@]}"; do curl -fsS -X POST "localhost:${PORT[$s]}/-/reload" >/dev/null || fail "reload inicial de $s"; done
echo "stack no ar"

for f in configs/*.yml solutions/*/*.yml; do
  svc=$(basename "$f" .yml); promtool_ok "$f" "$svc" || fail "promtool rejeitou $f"
done
echo "  ok  promtool check config (configs + solutions)"

# ---------- base: os dados fluem ----------
wait_for "prom-a gera job:http_requests:rate1m" eq n prom-a 'job:http_requests:rate1m{job="app"}' 1
wait_for "global federou job:* do cluster a" eq fresh global 'job:http_requests:rate1m{cluster="a",job="app"}' 1
wait_for "global federou job:* do cluster b" eq fresh global 'job:http_requests:rate1m{cluster="b",job="app"}' 1
wait_for "global tem só os 3 agregados x 2 clusters" eq fresh global '{job="app"}' 6
wait_for "global NÃO tem séries cruas (http_requests_total)" eq n global 'http_requests_total' 0
wait_for "receiver recebeu job:* de a e b" eq fresh receiver '{__name__=~"job:.*"}' 6
wait_for "receiver recebeu up de a (write_relabel keep)" eq fresh receiver 'up{cluster="a"}' 2
wait_for "receiver NÃO recebeu prometheus_* do cluster a" eq fresh receiver '{cluster="a",__name__=~"prometheus_.*"}' 0
wait_for "prom-a ainda tem prometheus_* localmente" gt0 n prom-a 'prometheus_http_requests_total'
wait_for "prom-a contou amostras descartadas pelo write_relabel" gt0 n prom-a 'prometheus_remote_storage_samples_dropped_total{reason="dropped_series"} > 0'
wait_for "agent -> receiver: http_requests_total{cluster=edge}" eq fresh receiver 'http_requests_total{cluster="edge",job="app"}' 6
wait_for "agent -> receiver: up{cluster=edge,job=app}=1" eq v receiver 'up{cluster="edge",job="app"}' 1
grep -q 'unavailable with Prometheus Agent' <<<"$(curl -s "localhost:9164/api/v1/query?query=up")" || fail "agent deveria recusar queries"
echo "  ok  agent recusa /api/v1/query (unavailable with Prometheus Agent)"

# ---------- exercícios: quebrado -> sintoma, solução -> conserto ----------
echo "exercícios:"
# 01
load exercises/01-federate-job-rules/global.yml global
wait_for "01 quebrado: federate up=1 mas 0 amostras" eq n global 'scrape_samples_scraped{job="federate"} == 0' 2
load solutions/01-federate-job-rules/global.yml global
wait_for "01 solução: federate traz amostras" eq n global 'scrape_samples_scraped{job="federate"} > 0' 2
wait_for "01 solução: job:* frescos no global" eq fresh global '{__name__=~"job:.*",job="app"}' 6

# 02
load exercises/02-external-labels/prom-b.yml prom-b
wait_for "02 quebrado: nenhum job:* fresco com cluster=b no receiver" eq fresh receiver 'job:http_requests:rate1m{cluster="b"}' 0
load solutions/02-external-labels/prom-b.yml prom-b
wait_for "02 solução: cluster=b de volta no receiver" eq fresh receiver 'job:http_requests:rate1m{cluster="b"}' 1
wait_for "02 solução: cluster=b de volta no global" eq fresh global 'job:http_requests:rate1m{cluster="b"}' 1

# 03
load exercises/03-write-relabel/prom-a.yml prom-a
wait_for "03 quebrado: prometheus_* do cluster a vazando para o receiver" gt0 fresh receiver '{cluster="a",__name__=~"prometheus_.*"}'
load solutions/03-write-relabel/prom-a.yml prom-a
wait_for "03 solução: nada de prometheus_* do cluster a" eq fresh receiver '{cluster="a",__name__=~"prometheus_.*"}' 0
wait_for "03 solução: só job:* (3) + up (2) do cluster a" eq fresh receiver '{cluster="a"}' 5

# 04
if promtool_ok exercises/04-agent-mode/agent.yml agent; then fail "04: promtool --agent deveria rejeitar a config quebrada"; fi
cp exercises/04-agent-mode/agent.yml prometheus/agent/prometheus.yml
if curl -fsS -X POST localhost:9164/-/reload >/dev/null 2>&1; then fail "04: reload da config quebrada deveria falhar"; fi
grep -q '^prometheus_config_last_reload_successful 0' <<<"$(curl -s localhost:9164/metrics)" || fail "04: agent deveria reportar reload falho"
echo "  ok  04 quebrado: promtool --agent e /-/reload rejeitam (alerting/rule_files)"
load solutions/04-agent-mode/agent.yml agent
grep -q '^prometheus_config_last_reload_successful 1' <<<"$(curl -s localhost:9164/metrics)" || fail "04: reload da solução não marcou sucesso"
wait_for "04 solução: dados do agent frescos no receiver" eq fresh receiver 'up{cluster="edge",job="app"}' 1

# 05 (comparação de contagens)
wait_for "05 prom-a tem muito mais séries que o que federa/envia" \
  eq n prom-a 'prometheus_tsdb_head_series > 10 * 6' 1
a_head=$(v prom-a 'prometheus_tsdb_head_series'); g_a=$(v global 'count(last_over_time({cluster="a"}[20s]))'); r_a=$(v receiver 'count(last_over_time({cluster="a"}[20s]))')
[ "$g_a" = 3 ] && [ "$r_a" = 5 ] || fail "05: esperado global{cluster=a}=3 e receiver{cluster=a}=5, veio $g_a e $r_a"
echo "  ok  05 séries: prom-a head=$a_head · global{cluster=a}=$g_a · receiver{cluster=a}=$r_a"

# 06
load exercises/06-honor-labels-federation/global.yml global
wait_for "06 quebrado: job=federate + exported_job=app" eq fresh global 'job:http_requests:rate1m{job="federate",exported_job="app"}' 2
load solutions/06-honor-labels-federation/global.yml global
wait_for "06 solução: job=app de novo, sem exported_job fresco" eq fresh global '{exported_job!=""}' 0
wait_for "06 solução: job:* com job=app" eq fresh global 'job:http_requests:rate1m{job="app"}' 2

# 07
load exercises/07-remote-read/global.yml global
wait_for "07 quebrado: global não enxerga up{cluster=edge}" eq n global 'up{cluster="edge"}' 0
load solutions/07-remote-read/global.yml global
wait_for "07 solução: global lê up{cluster=edge} do receiver via remote_read" eq n global 'up{cluster="edge"}' 2
wait_for "07 solução: sem cluster=edge na query, não vai ao remoto (required_matchers)" eq n global 'app_build_info' 0

echo "PASS $LAB"
