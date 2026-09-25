#!/usr/bin/env bash
# (Re)gera dashboards e (re)builda o gerador só com cenários que compilam.
# Usa flock para que várias pessoas/agentes possam chamar ao mesmo tempo.
set -uo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
exec 9>"$ROOT/.deploy.lock"
flock 9
cd "$ROOT"
python3 tools/gen_dashboards.py || exit 1
rm -rf .build/scenarios && mkdir -p .build/scenarios
ok=(); skip=()
for d in functions/*/; do
  fn=$(basename "$d")
  [ -f "$d/setup/scenario.go" ] || continue
  if tools/check-go.sh "$fn" >/dev/null 2>&1; then
    cp "$d/setup/scenario.go" ".build/scenarios/zz_${fn}_scenario.go"; ok+=("$fn")
  else
    skip+=("$fn")
  fi
done
# checagem final do conjunto (conflitos entre cenários)
if ! (T=$(mktemp -d); cp generator/{go.mod,go.sum,*.go} .build/scenarios/*.go "$T"/ && cd "$T" && go build -o /dev/null . ); then
  echo "ERRO: conjunto de cenários não compila junto" >&2; exit 1
fi
echo "cenários OK (${#ok[@]}): ${ok[*]}"
[ ${#skip[@]} -gt 0 ] && echo "IGNORADOS (não compilam): ${skip[*]}"
docker compose up -d --build generator >/dev/null 2>&1 || { echo "docker build falhou" >&2; exit 1; }
docker compose up -d >/dev/null 2>&1
echo "deploy ok"
