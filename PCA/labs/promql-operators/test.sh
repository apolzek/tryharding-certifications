#!/usr/bin/env bash
# Teste do lab promql-operators. Usa o Prometheus do promql-functions-lab (:9095) — NÃO sobe nem derruba nada.
#   1) toda linha dos blocos ```promql do README roda (e as marcadas com "# ❌" DEVEM dar erro)
#   2) valores exatos prometidos no README batem (métricas constantes do gerador)
#   3) regras YAML dos "Casos reais" passam no promtool
#   4) selftest dos desafios 200-299 (tópico operators) + soluções dos exercícios passam no check.py
#   5) dashboard do lab (lab.yaml) gera JSON válido e todas as queries dele retornam dados
# Imprime PASS/FAIL. KEEP=1 é aceito por convenção (não há nada para derrubar).
set -euo pipefail
cd "$(dirname "$0")"
LAB=promql-operators
PROM=${PROM_URL:-http://localhost:9095}
fail() { echo "FAIL $LAB: $*"; exit 1; }

# 0) Prometheus do functions-lab pronto (espera até 60s; não sobe nada por conta própria)
deadline=$(( $(date +%s) + 60 ))
until curl -sf "$PROM/-/ready" >/dev/null; do
  [ "$(date +%s)" -gt "$deadline" ] && fail "Prometheus em $PROM não está pronto (suba o promql-functions-lab: tools/deploy.sh)"
  sleep 2
done
curl -sf "$PROM/api/v1/query?query=count(sort_desc_http_requests_total)" | grep -q '"8"' \
  || fail "métricas do gerador ausentes (sort_desc_http_requests_total)"

# 1 + 2 + 5) queries do README, valores exatos e dashboard
PROM="$PROM" python3 - <<'PY' || fail "queries do README/dashboard (veja acima)"
import json, math, os, pathlib, re, sys
import requests, yaml

PROM = os.environ["PROM"]
bad = 0

def q(expr):
    r = requests.post(f"{PROM}/api/v1/query", data={"query": expr}, timeout=30).json()
    return r

# 1) cada linha de ```promql
text = pathlib.Path("README.md").read_text()
blocks = re.findall(r"```promql\n(.*?)```", text, flags=re.S)
n = 0
for b in blocks:
    for line in b.splitlines():
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        n += 1
        must_fail = "# ❌" in s
        r = q(s)
        ok = (r["status"] != "success") if must_fail else (r["status"] == "success")
        if not ok:
            bad += 1
            print(f"  README: {'deveria FALHAR' if must_fail else 'falhou'}: {s}\n     -> {r.get('error', 'rodou sem erro')}")
print(f"  README: {n} queries executadas")

# 2) valores exatos (métricas constantes / taxas fixas do gerador)
def vals(expr, key=None):
    global last_n
    r = q(expr)
    assert r["status"] == "success", (expr, r.get("error"))
    d = r["data"]
    if d["resultType"] == "scalar":
        last_n = 1
        return {"": float(d["result"][1])}
    last_n = len(d["result"])
    return {(s["metric"].get(key, "") if key else ""): float(s["value"][1]) for s in d["result"]}

def near(a, b, tol=1e-3):
    if isinstance(b, float) and math.isnan(b):
        return math.isnan(a)
    return abs(a - b) <= tol * max(1, abs(b))

NaN = float("nan")
K = "sort_by_label_desc_kafka_log_size_bytes"
R = "sort_desc_http_requests_total"
EXP = [
    ("sort_node_filesystem_size_bytes / 1024^3", "mountpoint", None, {"count": 5}),
    ('sort_node_filesystem_size_bytes{node="node-a"} / 1024^3', "mountpoint", {"/": 100, "/data": 500}, None),
    ("2 ^ 3 ^ 2", None, {"": 512}, None),
    ("-2 ^ 2", None, {"": -4}, None),
    ("2 * 3 % 4", None, {"": 2}, None),
    ("sort_cronjob_last_duration_seconds > 60", "cronjob", {"backup": 120}, None),
    ("sort_cronjob_last_duration_seconds > bool 30", "cronjob", {"backup": 1, "cleanup": 0, "report": 1, "sync": 0}, None),
    ("sort_cronjob_last_duration_seconds != 120", "cronjob", {"cleanup": 8, "report": 45, "sync": NaN}, None),
    ("count(sort_cronjob_last_duration_seconds > bool 30)", None, {"": 4}, None),
    ("sum(sort_cronjob_last_duration_seconds > bool 30)", None, {"": 2}, None),
    ("ceil_kube_deployment_spec_replicas - label_replace_kube_deployment_status_replicas_available", "deployment", {"checkout": 2}, None),
    ("ceil_kube_deployment_spec_replicas != label_replace_kube_deployment_status_replicas_available", "deployment", {"checkout": 5}, None),
    (f'rate({R}[5m]) / on(service) group_left sum by(service)(rate({R}[5m]))', None, None, {"count": 8}),
    (f'sum by(service)(rate({R}{{code="500"}}[5m])) / sum by(service)(rate({R}[5m]))', "service",
     {"cart": 0.01, "checkout": 0.05, "payments": 0.12, "recommendations": NaN}, None),
    (f'sum(rate({R}{{code="500"}}[5m])) / sum(rate({R}[5m]))', None, {"": 0.06}, None),
    (f'sum by(code)(rate({R}[5m])) / ignoring(code) group_left sum(rate({R}[5m]))', "code", {"200": 0.94, "500": 0.06}, None),
    (f'max_over_time(sum(rate({R}[1m]))[15m:1m])', None, {"": 300}, None),
    (f'count_over_time(sum(rate({R}[1m]))[10m:1m])', None, {"": 10}, None),
    (f'rate({R}{{service="checkout"}}[5m]) * 60', "code", {"200": 5700, "500": 300}, None),
    (f'{R} unless on(service) rate({R}{{code="500"}}[5m]) > 0', "code", None, {"count": 2}),
    (f'{R} and on(service) rate({R}{{code="500"}}[5m]) > 3', "code", None, {"count": 4}),
    ('sort_by_label_app_build_info unless sort_by_label_app_build_info{version="1.10.0"}', "pod", None, {"count": 6}),
    ("sort_cronjob_last_duration_seconds > 100 or sort_cronjob_last_duration_seconds < 10", "cronjob", {"backup": 120, "cleanup": 8}, None),
    ("sort_cronjob_last_duration_seconds > 100 or sort_cronjob_last_duration_seconds < 50 and sort_cronjob_last_duration_seconds < 100",
     "cronjob", {"backup": 120, "cleanup": 8, "report": 45}, None),
    ("(sort_cronjob_last_duration_seconds > 100 or sort_cronjob_last_duration_seconds < 50) and sort_cronjob_last_duration_seconds < 100",
     "cronjob", {"cleanup": 8, "report": 45}, None),
    (f"sum({K}) / 1024^3", None, {"": 78}, None),
    (f"avg({K}) / 1024^3", None, {"": 6.5}, None),
    (f"stddev({K}) / 1024^3", None, {"": 3.452}, None),
    (f"stdvar({K}) / 1024^6", None, {"": 11.9167}, None),
    (f"quantile(0.9, {K}) / 1024^3", None, {"": 10.9}, None),
    (f"topk(3, {K})", "partition", {"11": 12 * 2**30, "10": 11 * 2**30, "9": 10 * 2**30}, None),
    (f"bottomk(2, {K})", "partition", {"0": 2**30, "1": 2 * 2**30}, None),
    ("count by(version)(sort_by_label_app_build_info)", "version", {"1.10.0": 5, "1.10.12": 3, "1.9.2": 2, "1.2.0": 1}, None),
    ("count(group by(version)(sort_by_label_app_build_info))", None, {"": 4}, None),
    ('count_values("cost", days_in_month_node_total_hourly_cost)', "cost", {"0.096": 2, "0.34": 1}, None),
    ("sum(sort_cronjob_last_duration_seconds)", None, {"": NaN}, None),
    ("max(sort_cronjob_last_duration_seconds)", None, {"": 120}, None),
    ("quantile(0.5, sort_cronjob_last_duration_seconds)", None, {"": 26.5}, None),
    ("sum(sort_cronjob_last_duration_seconds == sort_cronjob_last_duration_seconds)", None, {"": 173}, None),
    ("info_http_server_requests_total * on(job, instance) group_left(version) target_info", "version", None, {"count": 2}),
    ('label_replace(label_replace_node_cpu_usage_percent, "internal_ip", "$1", "endpoint", "(.*):.*") * on(internal_ip) group_left(node) label_replace_kube_node_info',
     "node", None, {"keys": {"worker-1", "worker-2", "worker-3"}}),
    ('max_over_time(rate(rate_http_requests_total{route="/api/search"}[1m])[10m:30s])', None, None, {"between": (30, 50)}),
    ("increase_http_requests_total - increase_http_requests_total offset 1h", "code", None, {"approx": {"200": 7200}}),
]
for expr, key, exact, other in EXP:
    try:
        got = vals(expr, key)
    except AssertionError as e:
        bad += 1; print(f"  valor: query falhou: {e}"); continue
    ok = True
    if exact is not None:
        ok = set(got) == set(exact) and all(near(got[k], float(v)) for k, v in exact.items())
    if other:
        if "count" in other: ok = ok and last_n == other["count"]
        if "keys" in other: ok = ok and set(got) == other["keys"]
        if "between" in other: ok = ok and all(other["between"][0] <= v <= other["between"][1] for v in got.values())
        if "approx" in other: ok = ok and all(near(got.get(k, -1), v, 0.02) for k, v in other["approx"].items())
    if not ok:
        bad += 1; print(f"  valor diferente do README: {expr}\n     esperado {exact or other}, veio {got}")
print(f"  valores exatos: {len(EXP)} conferidos")

# 5) dashboard: lab.yaml -> todas as queries retornam dados
lab = yaml.safe_load(pathlib.Path("lab.yaml").read_text())
nd = 0
for p in lab["panels"]:
    for qq in p.get("queries", []):
        nd += 1
        r = q(qq["expr"])
        v = qq.get("validate", "nonempty")
        empty = r["status"] != "success" or (isinstance(r["data"]["result"], list) and not r["data"]["result"])
        if r["status"] != "success" or (v == "nonempty" and empty):
            bad += 1; print(f"  dashboard: '{p['title']}': {qq['expr']} -> {r.get('error', 'vazio')}")
print(f"  dashboard: {nd} queries")
sys.exit(1 if bad else 0)
PY

# 3) regras YAML do README -> promtool
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
python3 - "$tmp" <<'PY'
import pathlib, re, sys
out = pathlib.Path(sys.argv[1])
for i, b in enumerate(re.findall(r"```yaml\n(.*?)```", pathlib.Path("README.md").read_text(), flags=re.S)):
    if b.lstrip().startswith("groups:"):
        (out / f"rules-{i}.yml").write_text(b)
PY
ls "$tmp"/rules-*.yml >/dev/null 2>&1 || fail "nenhuma regra YAML encontrada no README"
chmod -R a+rX "$tmp"
files=$(cd "$tmp" && ls rules-*.yml | sed 's|^|/r/|')
# shellcheck disable=SC2086
docker run --rm -v "$tmp:/r:ro" --entrypoint promtool prom/prometheus:v3.15.0 check rules $files >"$tmp/err" 2>&1 \
  || { cat "$tmp/err"; fail "regras YAML do README inválidas"; }
echo "  promtool: $(ls "$tmp"/rules-*.yml | wc -l) arquivos de regras OK"

# 4) desafios 200-299 + soluções dos exercícios
out=$(cd ../../challenges && ./check.py selftest operators 2>&1) || { echo "$out" | grep -v '^WARN'; fail "selftest dos desafios operators"; }
echo "  $(tail -1 <<<"$out")"
prog=$(mktemp)
for f in solutions/*.promql; do
  id=$(basename "$f" | cut -d- -f1)
  PCA_PROGRESS=$prog ../../challenges/check.py "$id" -f "$f" >/dev/null \
    || { rm -f "$prog"; fail "solução $f não passa no desafio $id"; }
done
rm -f "$prog"
echo "  exercícios: $(ls solutions/*.promql | wc -l) soluções aceitas pelo check.py"

echo "PASS $LAB"
