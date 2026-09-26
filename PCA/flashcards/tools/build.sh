#!/usr/bin/env bash
# Gera todos os flashcards (inclusive o .apkg, que precisa do genanki).
#   tools/build.sh            -> usa .venv local (cria na 1ª vez) ou, se não der, docker python:3.14-alpine
#   USE_DOCKER=1 tools/build.sh
set -euo pipefail
cd "$(dirname "$0")/.."
GENANKI_VERSION=0.13.1          # última versão no PyPI (verificada em 2026-09)
PY_IMAGE=python:3.14-alpine

if [ "${USE_DOCKER:-0}" != 1 ]; then
  if [ ! -x .venv/bin/python ] || ! .venv/bin/python -c "import genanki, yaml" 2>/dev/null; then
    echo "» criando .venv com genanki==$GENANKI_VERSION"
    if python3 -m venv .venv >/dev/null 2>&1 && .venv/bin/pip install -q "genanki==$GENANKI_VERSION"; then :; else
      echo "» venv indisponível; usando docker"; rm -rf .venv; USE_DOCKER=1
    fi
  fi
fi

if [ "${USE_DOCKER:-0}" = 1 ]; then
  # monta PCA/ inteiro (read-only) para ler lições/labs e grava só em flashcards/
  docker run --rm -u "$(id -u):$(id -g)" -e HOME=/tmp -e PIP_DISABLE_PIP_VERSION_CHECK=1 \
    -v "$PWD/..:/pca:ro" -v "$PWD:/pca/flashcards" -w /pca/flashcards "$PY_IMAGE" \
    sh -c "pip install -q --user --no-warn-script-location genanki==$GENANKI_VERSION 2>&1 | grep -v 'WARNING: Running pip' || true; python tools/build.py $*"
else
  .venv/bin/python tools/build.py "$@"
fi
