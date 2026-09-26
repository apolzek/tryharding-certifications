#!/usr/bin/env bash
# Teste do mapa mental: gera de novo e valida links.
#  - link local quebrado para conteúdo que JÁ deveria existir (promql-functions-lab, markmap.md...) = ERRO
#  - link para labs/desafios/simulado/gamedays/flashcards/STUDY-PLAN ainda não criados = AVISO
#  - se houver chromium + internet, confere que o markmap renderizou (senão só avisa)
set -euo pipefail
cd "$(dirname "$0")"
fail() { echo "FAIL mindmap: $*"; exit 1; }

python3 build.py --check || fail "links quebrados (veja ERRO acima)"
[ -s markmap-links.md ] && [ -s index.html ] || fail "saídas ausentes"
# nós preservados: toda linha de conteúdo do markmap.md aparece (prefixo) no enriquecido
python3 - <<'PY' || fail "markmap-links.md perdeu nós do markmap.md"
import re
src = open("../markmap.md", encoding="utf-8").read().split("---", 2)[2].splitlines()
dst = open("markmap-links.md", encoding="utf-8").read().split("---", 2)[2].splitlines()
src = [l for l in src if l.strip()]; dst = [l for l in dst if l.strip()]
assert len(src) == len(dst), (len(src), len(dst))
for a, b in zip(src, dst):
    assert b.startswith(a), (a, b)
n = sum(1 for l in dst if "](" in l)
print(f"  {len(dst)} nós preservados, {n} com links")
assert n >= 30, "poucos nós com link"
PY
grep -q 'markmap-autoloader@' index.html || fail "index.html sem markmap-autoloader"
grep -q '<section id="outline">' index.html && grep -q 'promql-functions-lab/functions/rate/' index.html || fail "outline offline ausente"

CHROME=$(command -v chromium || command -v chromium-browser || command -v google-chrome || true)
if [ -n "$CHROME" ]; then
  dom=$(timeout 60 "$CHROME" --headless=new --disable-gpu --no-sandbox --virtual-time-budget=10000 \
        --dump-dom "file://$PWD/index.html" 2>/dev/null || true)
  grep -q 'class="d0"' <<<"$dom" || fail "outline não aparece no DOM"
  if grep -q 'markmap-node' <<<"$dom"; then echo "  markmap renderizado (CDN ok)"
  else echo "  WARN markmap não renderizou (sem internet/CDN?); o outline offline está ok"; fi
fi
echo "PASS mindmap"
