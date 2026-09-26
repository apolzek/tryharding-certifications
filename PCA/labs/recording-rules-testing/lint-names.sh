#!/usr/bin/env bash
# Lint de convenção de nomes de recording rules: nível:métrica:operações
#   ./lint-names.sh arquivo.yml [...]     exit 0 = todos os nomes OK
# promtool check rules NÃO verifica isso (no Prometheus 3 até "job:http-requests:rate5m" é um nome UTF-8 válido).
set -uo pipefail
re='^[a-zA-Z_][a-zA-Z0-9_]*:[a-zA-Z_][a-zA-Z0-9_]*:[a-zA-Z_][a-zA-Z0-9_]*$'
rc=0
for f in "$@"; do
  while read -r name; do
    name=${name%\"}; name=${name#\"}; name=${name%\'}; name=${name#\'}
    if [[ ! $name =~ $re ]]; then echo "$f: '$name' não segue nível:métrica:operações ([a-zA-Z0-9_] em cada parte)"; rc=1
    elif [[ $name == *:*_total:* ]]; then echo "$f: '$name' mantém _total depois de rate/increase (tire o sufixo)"; rc=1
    fi
  done < <(sed -nE 's/^[[:space:]]*-?[[:space:]]*record:[[:space:]]*([^[:space:]#]+).*/\1/p' "$f")
done
exit $rc
