#!/usr/bin/env bash
# Roda TODOS os testes do material PCA. exit 0 = tudo OK.
#   ./test-all.sh               # tudo
#   ./test-all.sh labs/alertmanager challenges   # só alguns
set -uo pipefail
cd "$(dirname "$0")"
targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(promql-functions-lab challenges exam flashcards mindmap gamedays)
  for d in labs/*/; do targets+=("${d%/}"); done
fi
pass=(); fail=()
for t in "${targets[@]}"; do
  if [ ! -x "$t/test.sh" ]; then echo "SKIP $t (sem test.sh)"; continue; fi
  echo "════════ $t"
  start=$(date +%s)
  if (cd "$t" && ./test.sh); then pass+=("$t"); else fail+=("$t"); fi
  echo "──────── $t: $(( $(date +%s) - start ))s"
done
echo
echo "PASS (${#pass[@]}): ${pass[*]:-}"
echo "FAIL (${#fail[@]}): ${fail[*]:-}"
[ ${#fail[@]} -eq 0 ]
