#!/usr/bin/env bash
# ./solve.sh NN -> aplica a solução oficial (solution/) em work/ e recarrega. Spoiler!
set -euo pipefail
source "$(dirname "$0")/_base/lib.sh"
S=$(scenario_dir "${1:-}")
cp -r "$S/solution/." "$WORK/"
"$GD_DIR/reload.sh"
