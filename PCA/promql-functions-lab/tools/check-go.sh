#!/usr/bin/env bash
# Compila o núcleo do gerador + os cenários indicados (ou todos).
#   tools/check-go.sh rate increase     -> checa só essas funções
#   tools/check-go.sh                   -> checa todas
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
cp "$ROOT"/generator/{go.mod,go.sum,*.go} "$TMP"/
if [ $# -eq 0 ]; then set -- $(ls "$ROOT/functions"); fi
for fn in "$@"; do
  f="$ROOT/functions/$fn/setup/scenario.go"
  [ -f "$f" ] || { echo "sem $f" >&2; exit 1; }
  cp "$f" "$TMP/zz_${fn}_scenario.go"
done
cd "$TMP" && go vet . && go build -o /dev/null . && echo "OK: $*"
