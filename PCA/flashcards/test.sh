#!/usr/bin/env bash
# Teste automático dos flashcards: gera tudo e valida .apkg, CSV e HTML.
# exit 0 = OK. Imprime "PASS flashcards" ou "FAIL flashcards: motivo".
set -euo pipefail
cd "$(dirname "$0")"
MIN_NOTES=${MIN_NOTES:-700}     # curadas (≥120) + top15 + lições (~900) → folga
MIN_CURATED=${MIN_CURATED:-120}
fail() { echo "FAIL flashcards: $*"; exit 1; }

echo "» build"
tools/build.sh --stats || fail "build falhou"
for f in dist/pca-flashcards.apkg dist/flashcards.csv dist/quizlet.csv flashcards.html dist/stats.json; do
  [ -s "$f" ] || fail "saída ausente: $f"
done

echo "» valida .apkg, CSV e stats"
python3 - "$MIN_NOTES" "$MIN_CURATED" <<'PY' || fail "validação do apkg/csv"
import csv, io, json, sqlite3, sys, tempfile, zipfile, os
min_notes, min_cur = int(sys.argv[1]), int(sys.argv[2])
stats = json.load(open("dist/stats.json"))
z = zipfile.ZipFile("dist/pca-flashcards.apkg")
assert z.testzip() is None, "zip corrompido"
names = z.namelist()
col = next((n for n in ("collection.anki21", "collection.anki2") if n in names), None)
assert col, f"sem collection db no apkg: {names}"
with tempfile.TemporaryDirectory() as td:
    p = os.path.join(td, "c.db"); open(p, "wb").write(z.read(col))
    db = sqlite3.connect(p)
    notes = db.execute("select count(*) from notes").fetchone()[0]
    cards = db.execute("select count(*) from cards").fetchone()[0]
    guids = db.execute("select count(distinct guid) from notes").fetchone()[0]
    decks = json.loads(db.execute("select decks from col").fetchone()[0])
    tagged = db.execute("select count(*) from notes where tags like '%domain::%'").fetchone()[0]
dnames = sorted(d["name"] for d in decks.values())
print(f"  apkg: {notes} notas, {cards} cards, {len(dnames)} decks")
assert notes == stats["total"], f"notas {notes} != stats {stats['total']}"
assert notes >= min_notes, f"poucas notas: {notes} < {min_notes}"
assert guids == notes, "GUIDs duplicados"
assert cards >= notes, "cards < notas"
assert tagged == notes, "nota sem tag domain::"
for dom in ("Observability Concepts", "Prometheus Fundamentals", "PromQL", "Instrumentation & Exporters", "Alerting & Dashboarding"):
    assert any(dom in n for n in dnames), f"deck do domínio ausente: {dom}"
cur = stats["by_origin"]["curated"]
assert cur >= min_cur, f"curadas {cur} < {min_cur}"
for k, v in stats["by_domain"].items():
    assert v >= 20, f"domínio {k} com só {v} cards"
# CSV (Anki): linhas de cabeçalho '#...' + header + 1 por card
lines = open("dist/flashcards.csv", encoding="utf-8").read()
body = "\n".join(l for l in lines.split("\n") if not l.startswith("#"))
rows = list(csv.reader(io.StringIO(body)))
assert rows[0] == ["guid", "deck", "front", "back", "source", "tags"], rows[0]
assert len(rows) - 1 == notes, f"CSV {len(rows)-1} linhas != {notes}"
assert all(len(r) == 6 and r[2] and r[3] for r in rows[1:]), "linha CSV incompleta"
q = list(csv.reader(open("dist/quizlet.csv", encoding="utf-8")))
assert len(q) == notes and all(len(r) == 2 and r[0] and r[1] for r in q), "quizlet.csv inválido"
print(f"  csv: {len(rows)-1} linhas · quizlet: {len(q)} linhas · curadas: {cur}")
PY

echo "» valida HTML no chromium headless"
CHROME=$(command -v chromium || command -v chromium-browser || command -v google-chrome || true)
if [ -z "$CHROME" ]; then
  echo "  (chromium não encontrado: pulando teste do HTML)"
else
  dom=$(timeout 60 "$CHROME" --headless=new --disable-gpu --no-sandbox --virtual-time-budget=3000 \
        --dump-dom "file://$PWD/flashcards.html#q=group_wait&list" 2>/dev/null) || fail "chromium falhou"
  n=$(grep -o 'class="item"' <<<"$dom" | wc -l)
  [ "$n" -ge 3 ] || fail "HTML: busca 'group_wait' mostrou só $n cards na lista"
  grep -q 'card 1 de' <<<"$dom" || fail "HTML: contador não renderizou"
  dom2=$("$CHROME" --headless=new --disable-gpu --no-sandbox --virtual-time-budget=3000 \
        --dump-dom "file://$PWD/flashcards.html" 2>/dev/null)
  front=$(sed -n 's/.*<div class="content" id="cf">\(.\{1,80\}\).*/\1/p' <<<"$dom2" | head -1)
  [ -n "$front" ] || fail "HTML: card inicial vazio"
  echo "  html: ok (lista com $n cards p/ 'group_wait'; 1º card: ${front:0:60}…)"
fi
echo "PASS flashcards"
