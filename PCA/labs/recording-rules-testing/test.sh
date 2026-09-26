#!/usr/bin/env bash
# Teste automático do lab de recording rules + promtool (sem stack, só promtool via docker).
#   - examples/ e solutions/: check rules --lint-fatal + lint de nomes + test rules  -> TÊM que passar
#   - exercises/ (arquivos iniciais quebrados)                                     -> TÊM que falhar
# KEEP=1 é aceito por convenção, mas não há stack para manter.
set -euo pipefail
cd "$(dirname "$0")"
LAB=labs/recording-rules-testing
IMG=prom/prometheus:v3.15.0
promtool() { docker run --rm -v "$PWD:/w:ro" -w /w --entrypoint promtool "$IMG" "$@"; }
fail() { echo "FAIL $LAB: $*"; exit 1; }

# roda check + lint de nomes + testes de uma pasta; exit 0 só se TUDO passar
validate_dir() {
  local d=$1 rules tests
  rules=$(ls "$d"/*.yml 2>/dev/null | grep -v '\.test\.yml$' || true)
  tests=$(ls "$d"/*.test.yml 2>/dev/null || true)
  [ -n "$rules" ] || return 1
  # shellcheck disable=SC2086
  promtool check rules --lint-fatal $rules >/dev/null 2>&1 || { echo "   check rules falhou"; return 1; }
  # shellcheck disable=SC2086
  ./lint-names.sh $rules >/dev/null || { echo "   lint de nomes falhou"; return 1; }
  if [ -n "$tests" ]; then
    # shellcheck disable=SC2086
    promtool test rules $tests >/dev/null 2>&1 || { echo "   test rules falhou"; return 1; }
  fi
}

echo "── examples/ (devem passar)"
validate_dir examples || fail "examples/ não passa"

n=0
for d in solutions/*/; do
  d=${d%/}; ex=exercises/$(basename "$d")
  [ -d "$ex" ] || fail "$d sem exercício correspondente"
  [ -f "$ex/README.md" ] || fail "$ex sem README.md"
  echo "── $(basename "$d")"
  validate_dir "$d" || fail "solução $d não passa"
  echo "   solução: OK"
  if out=$(validate_dir "$ex"); then
    fail "exercício $ex (arquivos iniciais) deveria FALHAR e passou"
  fi
  echo "   exercício quebrado falha como esperado:${out#   }"
  n=$((n + 1))
done
[ "$n" -ge 6 ] || fail "esperava ≥ 6 exercícios, achei $n"

# O teste da solução tem que ser o MESMO do exercício quando o exercício é "conserte a regra"
# (garante que o aluno não passa "mexendo no teste").
for ex in exercises/*/; do
  ex=${ex%/}; sol=solutions/$(basename "$ex")
  grep -q 'conserte a REGRA\|Conserte a REGRA\|o teste está certo\|já está pronto e correto' "$ex/rules.yml" 2>/dev/null || continue
  cmp -s "$ex/rules.test.yml" "$sol/rules.test.yml" || fail "$ex: o teste da solução difere do teste do exercício"
done

echo "PASS $LAB (examples + $n exercícios: soluções passam, iniciais falham)"
