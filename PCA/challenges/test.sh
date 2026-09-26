#!/usr/bin/env bash
# Teste dos desafios PromQL (challenges/). Usa o Prometheus do promql-functions-lab (:9095),
# que NÃO é derrubado aqui (ele é compartilhado com outros labs).
#   1. ./check.py selftest: todo gabarito roda, accepted_answers passam e wrong_answers são rejeitadas
#   2. smoke test da CLI (list/show/hint/resposta certa e errada/progress) com um progresso temporário
#   3. exporter de progresso numa porta temporária servindo pca_challenges_total
set -euo pipefail
cd "$(dirname "$0")"
fail() { echo "FAIL challenges: $*"; exit 1; }

PROM_URL=${PROM_URL:-http://localhost:9095}
python3 -c 'import requests, yaml' 2>/dev/null || fail "faltam dependências python: pip install pyyaml requests"

# --- 0. o lab precisa estar no ar (espera um pouco caso esteja subindo)
deadline=$(( $(date +%s) + 60 ))
until curl -sf -m 3 "$PROM_URL/-/ready" >/dev/null; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "Prometheus do promql-functions-lab não responde em $PROM_URL. Suba-o com: (cd ../promql-functions-lab && tools/deploy.sh)  — ou docker compose up -d"
  sleep 3
done
curl -sf -m 5 "$PROM_URL/api/v1/query?query=up%7Bjob%3D%22lab%22%7D" | grep -q '"value"' \
  || fail "o job 'lab' (gerador de métricas) não está sendo raspado em $PROM_URL"

# --- 1. selftest (uma nova tentativa para absorver um scrape/restart no meio do caminho)
if ! out=$(./check.py selftest 2>&1); then
  echo "$out" | grep -v '^WARN' | tail -20
  echo "selftest falhou; nova tentativa em 20s..."
  sleep 20
  out=$(./check.py selftest 2>&1) || { echo "$out" | grep -v '^WARN' | tail -30; fail "selftest"; }
fi
summary=$(echo "$out" | tail -1)
echo "$summary"
n=$(echo "$summary" | sed -n 's/selftest: \([0-9]*\) desafios.*/\1/p')
[ "${n:-0}" -ge 100 ] || fail "esperava ≥ 100 desafios, achei ${n:-0}"

# --- 2. smoke test da CLI com progresso temporário
tmp=$(mktemp -d)
exp_pid=""
cleanup() { [ -n "$exp_pid" ] && kill "$exp_pid" 2>/dev/null; rm -rf "$tmp"; }
trap cleanup EXIT
export PCA_PROGRESS="$tmp/progress.json" PCA_EXAM_RESULTS="$tmp/exam.json"

./check.py list | grep -q '001' || fail "list não mostra o desafio 001"
./check.py list --topic counters | grep -q '020' || fail "list --topic counters"
./check.py show 001 | grep -q 'increase_http_requests_total' || fail "show 001"
./check.py hint 001 | grep -q 'dica 1/' || fail "hint 001"
if ./check.py 001 'increase_http_requests_total' >/dev/null; then fail "resposta ERRADA do 001 foi aceita"; fi
./check.py 001 'increase_http_requests_total{code="500"}' | grep -q 'CORRETO' || fail "resposta certa do 001 foi rejeitada"
./check.py 020 -f <(echo 'rate(rate_http_requests_total[5m])') | grep -q 'CORRETO' || fail "resposta via -f"
if ./check.py 001 'sum(' >/dev/null; then fail "query com erro de sintaxe foi aceita"; fi
./check.py progress | grep -q "TOTAL 2/$n" || fail "progress não mostra 2/$n: $(./check.py progress | tail -1)"
./check.py solution 001 | grep -q 'code="500"' || fail "solution 001"
python3 - "$PCA_PROGRESS" <<'EOF' || fail "arquivo de progresso inconsistente"
import json, sys
p = json.load(open(sys.argv[1]))
assert set(p["solved"]) == {"001", "020"}, p["solved"]
assert p["attempts"]["001"] == 3 and p["hints"]["001"] == 1, p
EOF

# --- 3. exporter numa porta temporária
echo '{"by_domain": {"PromQL": 0.8}, "score": 0.8, "ts": 1790000000}' > "$PCA_EXAM_RESULTS"
port=$(( 19200 + RANDOM % 500 ))
PORT=$port ./check.py exporter >"$tmp/exporter.log" 2>&1 &
exp_pid=$!
deadline=$(( $(date +%s) + 20 ))
until curl -sf -m 2 "localhost:$port/metrics" >"$tmp/metrics" 2>/dev/null; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "exporter não respondeu em :$port ($(cat "$tmp/exporter.log"))"
  sleep 0.5
done
grep -q '^pca_challenges_total{topic="counters",level="basic"}' "$tmp/metrics" || fail "exporter sem pca_challenges_total"
grep -q '^pca_exam_score_ratio{domain="PromQL"} 0.8' "$tmp/metrics" || fail "exporter sem pca_exam_score_ratio"
python3 - "$tmp/metrics" "$n" <<'EOF' || fail "valores do exporter inconsistentes"
import re, sys
txt = open(sys.argv[1]).read()
tot = lambda m: sum(float(v) for v in re.findall(rf"^{m}{{[^}}]*}} (\S+)$", txt, re.M))
assert tot("pca_challenges_total") == int(sys.argv[2]), tot("pca_challenges_total")
assert tot("pca_challenges_solved") == 2, tot("pca_challenges_solved")
assert tot("pca_challenge_attempts_total") == 4 and tot("pca_challenge_hints_used") == 1
# cada família agrupada (formato de exposição)
names = [l.split("{")[0].split(" ")[0] for l in txt.splitlines() if l and not l.startswith("#")]
blocks = [k for i, k in enumerate(names) if i == 0 or names[i - 1] != k]
assert len(blocks) == len(set(blocks)), "famílias de métricas intercaladas"
EOF

# --- 4. o Prometheus do lab tem o job pca-progress e o dashboard existe
curl -sf -m 5 "$PROM_URL/api/v1/status/config" | grep -q 'pca-progress' \
  || fail "job pca-progress ausente no Prometheus do lab (recarregue: curl -X POST $PROM_URL/-/reload)"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["uid"]=="pca-progress"' \
  "../promql-functions-lab/static-dashboards/00 - Meu progresso PCA/pca-progress.json" || fail "dashboard de progresso inválido"

echo "PASS challenges ($n desafios)"
