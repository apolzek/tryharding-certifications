#!/usr/bin/env bash
# Solução do exercício 04: silencia api-2 por 10 minutos.
# Rode da pasta labs/alertmanager com a stack no ar. Imprime o ID do silence.
set -euo pipefail
docker compose exec -T alertmanager amtool --alertmanager.url=http://localhost:9093 \
  silence add alertname=HighLatency instance=api-2 \
  --duration=10m --author=aluno --comment="deploy do api-2"
