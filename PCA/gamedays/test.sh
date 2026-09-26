#!/usr/bin/env bash
# Teste automático dos gamedays. Para cada cenário:
#   1. sobe QUEBRADO  -> check.sh tem que dizer "AINDA QUEBRADO"
#   2. aplica solution/ -> check.sh tem que dizer "RESOLVIDO"
#   3. derruba (down -v)
#   ./test.sh            # todos
#   ./test.sh 03 05      # só alguns
#   KEEP=1 ./test.sh 07  # não derruba no fim
set -euo pipefail
cd "$(dirname "$0")"
source _base/lib.sh

ids=("$@")
if [ ${#ids[@]} -eq 0 ]; then
  for d in [0-9][0-9]-*/; do ids+=("${d%%-*}"); done
fi

fail() { echo "FAIL gamedays: $*"; [ "${KEEP:-0}" = 1 ] || ./stop.sh >/dev/null 2>&1 || true; exit 1; }

# sanidade estática: todas as configs de solução passam no promtool/amtool
for d in [0-9][0-9]-*/; do
  for f in "$d"solution/prometheus/rules.yml "$d"scenario/prometheus/rules.yml; do
    [ -f "$f" ] || continue
    docker run --rm -v "$PWD/$(dirname "$f"):/r:ro" --entrypoint promtool "$PROM_IMAGE" \
      check rules /r/rules.yml >/dev/null || fail "promtool check rules $f"
  done
  for f in "$d"solution/alertmanager/alertmanager.yml "$d"scenario/alertmanager/alertmanager.yml; do
    [ -f "$f" ] || continue
    docker run --rm -v "$PWD/$(dirname "$f"):/a:ro" --entrypoint amtool prom/alertmanager:v0.34.1 \
      check-config /a/alertmanager.yml >/dev/null || fail "amtool check-config $f"
  done
done
echo "ok  configs passam em promtool/amtool"

for id in "${ids[@]}"; do
  S=$(scenario_dir "$id"); name=$(basename "$S"); t0=$SECONDS
  ./start.sh "$id" >/dev/null || fail "$name: start.sh falhou"
  if CHECK_TIMEOUT=10 ./check.sh "$id" >/tmp/gd-test.$$ 2>&1; then
    cat /tmp/gd-test.$$; fail "$name: check.sh disse RESOLVIDO com a config quebrada"
  fi
  grep -q 'AINDA QUEBRADO' /tmp/gd-test.$$ || { cat /tmp/gd-test.$$; fail "$name: check.sh não rodou direito"; }
  ./solve.sh "$id" >/dev/null || fail "$name: solve.sh (reload) falhou"
  if ! ./check.sh "$id" >/tmp/gd-test.$$ 2>&1; then
    cat /tmp/gd-test.$$; fail "$name: check.sh não aceitou a solução oficial"
  fi
  echo "ok  $name ($((SECONDS - t0))s)"
done
rm -f /tmp/gd-test.$$
[ "${KEEP:-0}" = 1 ] || ./stop.sh >/dev/null 2>&1
echo "PASS gamedays"
