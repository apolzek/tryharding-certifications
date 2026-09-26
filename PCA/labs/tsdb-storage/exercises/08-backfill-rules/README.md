# 08 · Backfill de recording rule (`create-blocks-from rules`)

**Objetivo:** você criou hoje a recording rule `store:tsdb_backfill_orders:rate5m`. Ela só tem dados **daqui para frente**. Gere o histórico dela para o dia importado no exercício 01.

## 🎯 Tarefa

1. Escreva a regra num arquivo (`sum by (store) (rate(tsdb_backfill_orders_total[5m]))`).
2. Rode `promtool tsdb create-blocks-from rules` apontando para o Prometheus (`--url`), com `--start`/`--end` cobrindo o dia importado e saída em `/prometheus`.
3. Consulte `store:tsdb_backfill_orders:rate5m` numa hora de ontem.

<details><summary>✅ Solução</summary>

```bash
END=$(( $(date +%s) / 3600 * 3600 - 2 * 3600 ))
docker compose exec -T prometheus sh -c "cat > /tmp/rules.yml && promtool tsdb create-blocks-from rules \
  --url=http://localhost:9090 --start=$((END - 86400)) --end=$END --output-dir=/prometheus /tmp/rules.yml" \
  < solutions/08-backfill-rules.yml
```

- O promtool **executa a query** da regra via API, passo a passo (`interval` do grupo, ou `--eval-interval`), e escreve os resultados em blocos novos.
- Resultado: `sp` ≈ **0.05/s** fora do horário comercial e ≈ **0.15/s** entre 9h e 18h UTC.
- Limitações (da doc): **regras de alerta são ignoradas**; regras que dependem de **outras** recording rules no mesmo arquivo não enxergam o backfill da primeira (rode em duas passadas); não use `--end` dentro da janela da head (padrão: 3h atrás).

Arquivos: [`solutions/08-backfill-rules.yml`](../../solutions/08-backfill-rules.yml) · [`solutions/08-backfill-rules.sh`](../../solutions/08-backfill-rules.sh).
</details>
