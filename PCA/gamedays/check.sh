#!/usr/bin/env bash
# ./check.sh NN -> o incidente está resolvido? (exit 0 = sim)
#   CHECK_TIMEOUT=N  segundos esperando a condição virar verdade (padrão: do cenário)
set -uo pipefail
source "$(dirname "$0")/_base/lib.sh"
S=$(scenario_dir "${1:-}") || exit 2
# check.bash define: TIMEOUT (padrão) e a função check() que ecoa o motivo e retorna != 0 se ainda quebrado
TIMEOUT=60
source "$S/check.bash"
timeout_s=${CHECK_TIMEOUT:-$TIMEOUT}
end=$((SECONDS + timeout_s))
echo "🔎 conferindo $(basename "$S") (até ${timeout_s}s)..."
while :; do
  if reason=$(check 2>&1); then
    echo "✅ RESOLVIDO: ${reason:-incidente corrigido}"; exit 0
  fi
  [ $SECONDS -ge $end ] && break
  sleep 3
done
echo "❌ AINDA QUEBRADO: $reason"
exit 1
