#!/usr/bin/env bash
# Teste automático do lab service-discovery-relabeling.
# Sobe a stack, valida a config base (static/file_sd/http_sd, hot reload do file_sd),
# carrega CADA solução via /-/reload e confere o resultado pela API. KEEP=1 mantém a stack.
set -euo pipefail
cd "$(dirname "$0")"
LAB=service-discovery-relabeling
PROM=localhost:9120
SHARD1=localhost:9129
IMG=prom/prometheus:v3.15.0

fail() { echo "FAIL $LAB: $*"; exit 1; }

cleanup() {
  rm -f prometheus/sd/apps-inventory.json
  cp configs/base.yml prometheus/prometheus.yml
  cp configs/shard1-base.yml prometheus/shard1.yml
  if [ "${KEEP:-0}" = 1 ]; then
    curl -fsS -X POST "$PROM/-/reload" >/dev/null 2>&1 || true
    curl -fsS -X POST "$SHARD1/-/reload" >/dev/null 2>&1 || true
  else
    docker compose down -v >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# ---------- helpers ----------
# q <host> <promql>  -> imprime o resultado (JSON do vetor)
q() { curl -fsS "http://$1/api/v1/query" --data-urlencode "query=$2" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin)["data"]["result"]))'; }
# n <promql> -> número de séries no resultado
n() { q "$PROM" "$1" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))'; }
# v <promql> -> valor da primeira série (ou "none")
v() { q "$PROM" "$1" | python3 -c 'import json,sys; r=json.load(sys.stdin); print(r[0]["value"][1] if r else "none")'; }
# instances <host> <scrapePool> -> instances ativas (ordenadas, separadas por vírgula)
instances() {
  curl -fsS "http://$1/api/v1/targets?state=active&scrapePool=$2" | python3 -c '
import json,sys
print(",".join(sorted(t["labels"]["instance"] for t in json.load(sys.stdin)["data"]["activeTargets"])))'
}
# wait_for <descrição> <comando...>: repete até o comando dar exit 0 (timeout 60s)
wait_for() {
  local desc=$1; shift
  local deadline=$(( $(date +%s) + 60 ))
  until "$@" >/dev/null 2>&1; do
    [ "$(date +%s)" -gt "$deadline" ] && fail "$desc"
    sleep 2
  done
  echo "  ok  $desc"
}
eq() { [ "$($1 "${@:2:$#-2}")" = "${!#}" ]; }   # eq <func> <args...> <esperado>
check() { promtool_check "$1" || fail "promtool rejeitou $1"; }
promtool_check() {
  docker run --rm -v "$PWD/$1:/tmp/c.yml:ro" -v "$PWD/prometheus:/etc/prometheus:ro" \
    --entrypoint promtool "$IMG" check config /tmp/c.yml >/dev/null 2>&1
}
load() {  # load <arquivo> [host] [destino]
  local host=${2:-$PROM} dst=${3:-prometheus/prometheus.yml}
  check "$1"
  cp "$1" "$dst"
  curl -fsS -X POST "http://$host/-/reload" >/dev/null || fail "reload de $1 falhou"
}

# ---------- sobe ----------
cp configs/base.yml prometheus/prometheus.yml
cp configs/shard1-base.yml prometheus/shard1.yml
rm -f prometheus/sd/apps-inventory.json
docker compose up -d --wait >/dev/null 2>&1 || fail "docker compose up"
# se a stack já estava no ar (KEEP=1), garante que as configs base estão carregadas
curl -fsS -X POST "$PROM/-/reload" >/dev/null && curl -fsS -X POST "$SHARD1/-/reload" >/dev/null || fail "reload inicial"
echo "stack no ar"

# ---------- promtool: todas as configs de solução/base válidas; o 05 quebrado é inválido ----------
for f in configs/*.yml solutions/*/*.yml; do check "$f"; done
if promtool_check exercises/05-labeldrop/prometheus.yml; then fail "exercício 05 deveria ser rejeitado pelo promtool"; fi
echo "  ok  promtool check config (configs + solutions; exercício 05 rejeitado como esperado)"

# ---------- base ----------
wait_for "base: 10 alvos up" eq v 'count(up == 1)' 10
wait_for "base: file_sd com 4 alvos" eq instances "$PROM" file-sd-apps "checkout-api:8000,payments-api:8000,payments-worker:8000,search-api:8000"
wait_for "base: http_sd com 2 alvos" eq instances "$PROM" http-sd-apps "checkout-api:8000,search-api:8000"
grep -q '"__meta_url":"http://sd-server:8000/targets.json"' <<<"$(curl -fsS "$PROM/api/v1/targets?scrapePool=http-sd-apps")" \
  || fail "discoveredLabels do http_sd sem __meta_url"
echo "  ok  discoveredLabels do http_sd tem __meta_url"
wait_for "base: params -> ?format=prometheus chegou no alvo" eq n 'app_scrape_param{job="legacy-batch",name="format",value="prometheus"}' 1
wait_for "base: scrape_samples_scraped existe por alvo" eq v 'count(scrape_samples_scraped)' 10
wait_for "base: honor_labels=false gera exported_job" eq n 'batch_records_processed_total{job="legacy-batch",exported_job="nightly-batch"}' 1
wait_for "base: honor_timestamps (timestamp explícito ~30s atrás)" eq v '(time() - timestamp(batch_heartbeat{job="legacy-batch"})) > 20 < 60 > bool 0' 1

# hot reload do file_sd: cria um arquivo novo que casa com o glob apps*.json
cat > prometheus/sd/apps-inventory.json <<'EOF'
[{"targets": ["inventory-api:8000"], "labels": {"team": "inventory"}}]
EOF
wait_for "file_sd hot reload: inventory-api apareceu sem reload" eq n 'up{job="file-sd-apps",instance="inventory-api:8000",team="inventory"} == 1' 1
rm -f prometheus/sd/apps-inventory.json
wait_for "file_sd hot reload: inventory-api sumiu ao apagar o arquivo" eq instances "$PROM" file-sd-apps "checkout-api:8000,payments-api:8000,payments-worker:8000,search-api:8000"

# ---------- vitrine de actions ----------
load configs/demo-actions.yml
wait_for "demo: keepequal deixou 3 alvos (checkout fora)" eq instances "$PROM" demo-actions "payments-api:8000,payments-worker:8000,search-api:8000"
wait_for "demo: lowercase/uppercase/__tmp_" eq n 'up{job="demo-actions",env="staging",region="us-east-1",team_code="SEARCH",host_env="search-api@staging"}' 1
wait_for "demo: labelkeep removeu pod_uid" eq n 'app_cache_entries{job="demo-actions",pod_uid!=""}' 0
wait_for "demo: labelkeep manteve as séries" eq n 'app_cache_entries{job="demo-actions"}' 3

# ---------- soluções ----------
echo "exercícios:"
load solutions/01-keep-team-payments/prometheus.yml
wait_for "01 keep team=payments" eq instances "$PROM" payments-only "payments-api:8000,payments-worker:8000"
dropped=$(curl -fsS "$PROM/api/v1/targets?state=dropped" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["droppedTargetCounts"].get("payments-only",0))')
[ "$dropped" = 2 ] || fail "01: esperado 2 alvos descartados, veio $dropped"
echo "  ok  01 droppedTargetCounts[payments-only]=2"

load solutions/02-instance-sem-porta/prometheus.yml
wait_for "02 instance sem porta" eq instances "$PROM" apps-hostname "checkout-api,payments-api,payments-worker,search-api"

load solutions/03-address-from-meta/prometheus.yml
wait_for "03 __address__ montado a partir de __meta_*" eq v 'count(up{job="kubernetes-pods"} == 1)' 2
wait_for "03 labels namespace/pod/app + __metrics_path__ da annotation" \
  eq n 'up{job="kubernetes-pods",instance="legacy-batch:8000",namespace="data",pod="legacy-batch-5c7d9-qq8wz",app="legacy-batch",tier="batch"} == 1' 1

load solutions/04-drop-high-cardinality/prometheus.yml
wait_for "04 app_requests_total presente" eq n 'app_requests_total{job="payments-api"}' 3
wait_for "04 app_requests_by_user_total ausente" eq n 'app_requests_by_user_total{job="payments-api"}' 0
wait_for "04 post_metric_relabeling < scraped" eq n 'scrape_samples_post_metric_relabeling{job="payments-api"} < scrape_samples_scraped{job="payments-api"}' 1

load solutions/05-labeldrop/prometheus.yml
wait_for "05 labeldrop manteve 4 séries" eq n 'app_cache_entries{job="apps-labeldrop"}' 4
wait_for "05 sem pod_uid" eq n 'app_cache_entries{job="apps-labeldrop",pod_uid!=""}' 0

load solutions/06-hashmod-sharding/prometheus.yml
load solutions/06-hashmod-sharding/shard1.yml "$SHARD1" prometheus/shard1.yml
shard_ok() {
  local a b; a=$(instances "$PROM" sharded-apps); b=$(instances "$SHARD1" sharded-apps)
  python3 - "$a" "$b" <<'EOF'
import sys
a = set(filter(None, sys.argv[1].split(","))); b = set(filter(None, sys.argv[2].split(",")))
assert not (a & b), f"sobreposição {a & b}"
assert len(a | b) == 6, f"união {a | b}"
EOF
}
wait_for "06 hashmod: 6 alvos divididos sem sobreposição entre 9120 e 9129" shard_ok

load solutions/07-param-target/prometheus.yml
wait_for "07 3 probes" eq n 'probe_success{job="probe-apps"}' 3
wait_for "07 payments-api sondado com sucesso" eq v 'probe_success{job="probe-apps",instance="payments-api:8000"}' 1
wait_for "07 nao-existe falhou" eq v 'probe_success{job="probe-apps",instance="nao-existe:8000"}' 0

load solutions/08-honor-labels/prometheus.yml
wait_for "08 honor_labels: job=nightly-batch" eq n 'batch_records_processed_total{job="nightly-batch"}' 1
wait_for "08 sem exported_job no job batch" eq n '{job="nightly-batch",exported_job!=""}' 0
wait_for "08 up continua com job do scrape" eq v 'up{job="batch"}' 1

load solutions/09-sample-limit/prometheus.yml
wait_for "09 sample_limit respeitado e up=1" eq v 'up{job="payments-limited"}' 1
wait_for "09 amostras <= 100" eq n 'scrape_samples_post_metric_relabeling{job="payments-limited"} <= 100' 1

load solutions/10-http-sd-labelmap/prometheus.yml
wait_for "10 http_sd + labelmap" eq v 'count(up{job="http-sd",datacenter="sa-east-1a",owner_team!=""} == 1)' 2
wait_for "10 sem label url" eq n 'up{job="http-sd",url!=""}' 0

echo "PASS $LAB"
