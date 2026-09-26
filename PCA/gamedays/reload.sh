#!/usr/bin/env bash
# ./reload.sh -> recarrega Prometheus e Alertmanager com o que está em work/
set -euo pipefail
source "$(dirname "$0")/_base/lib.sh"
chmod -R a+rX "$WORK"
if reload_all; then echo "✅ Prometheus e Alertmanager recarregados"; else exit 1; fi
