#!/usr/bin/env bash
# Teste automático do simulado: extrai + gera + valida o banco, confere o sorteio (100x),
# roda o exam.py sem interação e abre o index.html no chromium headless.
# exit 0 = OK. SHOT=1 salva um screenshot em /tmp/pca-exam.png.
set -euo pipefail
export PYTHONDONTWRITEBYTECODE=1
cd "$(dirname "$0")"
fail() { echo "FAIL exam: $*"; exit 1; }
TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT

echo "── 1. extract / build / validate"
python3 tools/extract.py || fail "extract.py"
python3 tools/build.py   || fail "build.py"
python3 tools/validate.py || fail "validate.py"

echo "── 2. proporções do sorteio (sampler.js, 100 seeds × 2 idiomas)"
command -v node >/dev/null || fail "node não encontrado"
node - <<'EOF' || fail "sorteio fora das cotas"
const S = require("./sampler.js"); global.window = {}; require("./questions.js");
const Q = window.PCA_QUESTIONS, want = {"PromQL":17,"Prometheus Fundamentals":12,"Observability Concepts":11,"Alerting and Dashboarding":11,"Instrumentation and Exporters":9};
const q60 = S.quotas(60);
for (const d in want) if (q60[d] !== want[d]) { console.error("quota errada", d, q60[d]); process.exit(1); }
for (let n = 1; n <= 120; n++) { const s = Object.values(S.quotas(n)).reduce((a, b) => a + b, 0); if (s !== n) { console.error("quotas(" + n + ") soma " + s); process.exit(1); } }
const seen = new Set();
for (const lang of ["en", "all"]) for (let seed = 1; seed <= 100; seed++) {
  const d = S.draw(Q, { n: 60, lang, seed });
  if (d.questions.length !== 60 || d.shortfall) { console.error("draw", lang, seed, d.questions.length, d.shortfall); process.exit(1); }
  const ids = new Set(d.questions.map(q => q.id)); if (ids.size !== 60) { console.error("ids repetidos", seed); process.exit(1); }
  const c = {}; d.questions.forEach(q => { c[q.domain] = (c[q.domain] || 0) + 1; seen.add(q.id);
    if (lang === "en" && q.lang !== "en") { console.error("questão PT no modo EN", q.id); process.exit(1); } });
  for (const k in want) if (c[k] !== want[k]) { console.error("proporção", lang, seed, k, c[k]); process.exit(1); }
}
const o = S.optionOrder(S.rng(9)); if (o.slice().sort().join() !== "0,1,2,3") process.exit(1);
console.log("  200 sorteios OK (17/12/11/11/9), " + seen.size + " questões distintas apareceram");
EOF

echo "── 3. exam.py: réplica Python do sorteio == sampler.js"
for seed in 1 42 2026; do
  for lang in en all; do
    py=$(./exam.py sample --seed $seed --lang $lang | python3 -c 'import json,sys; print(",".join(json.load(sys.stdin)["ids"]))')
    js=$(node -e 'const S=require("./sampler.js");global.window={};require("./questions.js");console.log(S.draw(window.PCA_QUESTIONS,{n:60,lang:process.argv[2],seed:+process.argv[1]}).questions.map(q=>q.id).join(","))' $seed $lang)
    [ "$py" = "$js" ] || fail "sorteio Python != JS (seed=$seed lang=$lang)"
  done
done
js=$(node -e 'const S=require("./sampler.js");global.window={};require("./questions.js");console.log(window.PCA_QUESTIONS.filter(S.citesLetters).map(q=>q.id).join(","))')
py=$(python3 -c 'import json,exam; print(",".join(q["id"] for q in json.load(open("questions.json")) if exam.cites_letters(q)))')
[ "$js" = "$py" ] || fail "citesLetters Python != JS"
echo "  mesmos ids em Python e JS (sorteio e regra de embaralhar alternativas)"

echo "── 4. exam.py não interativo (--answers)"
python3 - "$TMP" <<'EOF' || fail "respostas de teste"
import json, sys
qs = json.load(open("questions.json")); t = sys.argv[1]
json.dump({q["id"]: q["answer"] for q in qs}, open(f"{t}/all-right.json", "w"))
wrong = {q["id"]: ("A" if q["answer"] != "A" else "B") if q["domain"] == "Observability Concepts" else q["answer"] for q in qs}
open(f"{t}/obs-wrong.txt", "w").write("\n".join(f"{k} {v}" for k, v in wrong.items()))
EOF
./exam.py simulado --seed 7 --answers "$TMP/all-right.json" --results "$TMP/r1.json" >/dev/null || fail "exam.py simulado (todas certas)"
./exam.py simulado --seed 8 --lang all --answers "$TMP/obs-wrong.txt" --results "$TMP/r2.json" >/dev/null || fail "exam.py simulado (obs erradas)"
./exam.py treino --domain promql --n 5 --seed 1 --answers "$TMP/all-right.json" --results "$TMP/r3.json" --save >/dev/null || fail "exam.py treino"
python3 - "$TMP" <<'EOF' || fail "resultado do exam.py"
import json, sys
t = sys.argv[1]
r1, r2, r3 = (json.load(open(f"{t}/r{i}.json")) for i in (1, 2, 3))
for r in (r1, r2, r3):
    assert {"by_domain", "score", "ts"} <= set(r), r
assert r1["score"] == 1.0 and r1["total"] == 60 and len(r1["by_domain"]) == 5, r1
assert r2["by_domain"]["Observability Concepts"] == 0.0 and r2["by_domain"]["PromQL"] == 1.0, r2
assert abs(r2["score"] - 49 / 60) < 1e-9, r2["score"]
assert r3["total"] == 5 and r3["by_domain"] == {"PromQL": 1.0}, r3
print("  simulado 100%, obs-errado 81.7% (obs=0), treino OK; formato {by_domain, score, ts} OK")
EOF

echo "── 5. index.html no chromium headless"
CHROME=$(command -v chromium || command -v chromium-browser || command -v google-chrome || true)
if [ -z "$CHROME" ]; then
  echo "  SKIP: chromium não encontrado (o resto passou)"
else
  out=$(timeout 90 "$CHROME" --headless=new --disable-gpu --no-sandbox --virtual-time-budget=8000 \
        --dump-dom "file://$PWD/index.html#selftest" 2>/dev/null | grep -o 'SELFTEST {.*}' | head -1 | sed 's/^SELFTEST //; s/<\/pre>.*//')
  [ -n "$out" ] || fail "index.html não produziu o SELFTEST (erro de JS?)"
  echo "$out" | python3 -c '
import json, sys
r = json.load(sys.stdin)
want = {"PromQL":17,"Prometheus Fundamentals":12,"Observability Concepts":11,"Alerting and Dashboarding":11,"Instrumentation and Exporters":9}
assert not r["errors"], r["errors"]
for k in ("sim_en", "sim_all"):
    s = r[k]
    assert s["n"] == 60 and s["counts"] == want, s
    assert s["rendered_options"] == 4 and s["question_text"], s
    assert s["review_items"] > 0, s
assert r["treino"]["explained"] and r["treino"]["n"] > 0, r["treino"]
print("  renderiza %d questões do banco; simulado EN/ALL com 60 questões e cotas corretas; revisão e treino OK; 0 erros JS" % r["total_bank"])
' || fail "selftest do index.html: $out"
  if [ "${SHOT:-}" = 1 ]; then
    timeout 60 "$CHROME" --headless=new --disable-gpu --no-sandbox --hide-scrollbars --window-size=1000,1200 \
      --virtual-time-budget=3000 --screenshot=/tmp/pca-exam.png "file://$PWD/index.html#autostart=simulado&seed=3" >/dev/null 2>&1 \
      && echo "  screenshot: /tmp/pca-exam.png"
  fi
fi

echo "PASS exam"
