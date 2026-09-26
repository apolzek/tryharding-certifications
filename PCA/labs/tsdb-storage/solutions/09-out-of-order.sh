#!/usr/bin/env bash
# Exercício 09: out-of-order. A série com timestamp explícito "volta no tempo".
set -euo pipefail
m() { curl -sf localhost:9150/metrics | grep -E "^prometheus_tsdb_($1)\{type=\"float\"\}"; }
curl -sf "localhost:9151/control?ts_offset=600"; sleep 11     # 10 min no passado: dentro da janela de 30m
m head_out_of_order_samples_appended_total
curl -sf "localhost:9151/control?ts_offset=3600"; sleep 11    # 1h no passado: fora da janela
m too_old_samples_total
curl -sf "localhost:9151/control?ts_offset=0"
