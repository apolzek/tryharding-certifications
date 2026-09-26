#!/usr/bin/env bash
# ./stop.sh -> derruba o gameday e apaga os dados
set -euo pipefail
source "$(dirname "$0")/_base/lib.sh"
"${COMPOSE[@]}" --profile pushgateway down -v --remove-orphans
