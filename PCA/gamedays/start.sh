#!/usr/bin/env bash
# ./start.sh NN   -> derruba o gameday anterior e sobe o cenário NN já "quebrado"
set -euo pipefail
source "$(dirname "$0")/_base/lib.sh"
S=$(scenario_dir "${1:-}")
echo "▶ gameday: $(basename "$S")"

"${COMPOSE[@]}" --profile pushgateway down -v --remove-orphans >/dev/null 2>&1 || true
rm -rf "$WORK"; mkdir -p "$WORK"
cp -r "$GD_DIR/_base/defaults/." "$WORK/"
# pre/ = estado "antes do deploy" (quando existe, o scenario/ é aplicado depois, via reload)
if [ -d "$S/pre" ]; then cp -r "$S/pre/." "$WORK/"; else cp -r "$S/scenario/." "$WORK/"; fi
chmod -R a+rX "$WORK"

export COMPOSE_PROFILES=""
[ -f "$S/profiles" ] && COMPOSE_PROFILES=$(cat "$S/profiles")
if ! out=$("${COMPOSE[@]}" up -d --wait 2>&1); then echo "$out"; exit 1; fi

if [ -d "$S/pre" ]; then
  echo "▶ aplicando o 'deploy' que causou o incidente..."
  cp -r "$S/scenario/." "$WORK/"; chmod -R a+rX "$WORK"
  reload_all >/dev/null 2>&1 || true
fi
[ -x "$S/hook.sh" ] && "$S/hook.sh"

cat <<MSG
✅ cenário no ar.
   Prometheus   http://localhost:9180
   Alertmanager http://localhost:9181
   App/pager    http://localhost:9182   (notificações recebidas: /received)
   Configs ao vivo (EDITE AQUI): $WORK/
   Depois de editar:  ./reload.sh      Conferir: ./check.sh ${1}
   O chamado: $S/README.md
MSG
