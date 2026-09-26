# 01 · Backfill: importar 1 dia de dados históricos (OpenMetrics)

**Objetivo:** você migrou de outro sistema e tem um **dia inteiro** de métricas antigas num arquivo. Coloque esses dados dentro do Prometheus e consulte-os.

O gerador serve o arquivo pronto: `curl -s localhost:9151/backfill.om | head` (24h, 1 amostra/min, timestamps em **segundos**, termina com `# EOF`, fim = hora cheia de 2h atrás).

## ▶️ Tarefa

1. Baixe o arquivo para o host.
2. Rode `promtool tsdb create-blocks-from openmetrics <arquivo> <dir-saida>` **de forma que os blocos acabem no diretório de dados do Prometheus** (`/prometheus` dentro do container).
3. Liste os blocos com `promtool tsdb list -r`. Quantos blocos? De quantas horas cada um?
4. Espere ~1 min e consulte: quantos pedidos a loja `sp` fez em **uma hora** do horário comercial (9h-18h UTC) do dia importado? E numa hora de madrugada?

## 🎯 Critério de pronto

```bash
# 1440 amostras (1/min × 24h) no dia importado
END=$(( $(date +%s) / 3600 * 3600 - 2 * 3600 ))
curl -s localhost:9150/api/v1/query --data-urlencode 'query=count_over_time(tsdb_backfill_temperature_celsius{city="sao_paulo"}[1d])' --data-urlencode "time=$((END-1))"
# ... "value":[...,"1440"]
```

## 💡 Dica

O `promtool` já existe **dentro** da imagem do Prometheus. `docker compose exec -T prometheus sh -c 'cat > /tmp/x.om && promtool ...' < arquivo.om` manda o arquivo do host pelo stdin.

<details><summary>✅ Solução</summary>

```bash
curl -s localhost:9151/backfill.om -o /tmp/day.om
docker compose exec -T prometheus sh -c \
  'cat > /tmp/day.om && promtool tsdb create-blocks-from openmetrics /tmp/day.om /prometheus' < /tmp/day.om
docker compose exec prometheus promtool tsdb list -r /prometheus
```

- Saem **~12 blocos de até 2h**, alinhados em múltiplos de 2h UTC (o padrão de `--max-block-duration` é 2h).
- Você **não** precisa reiniciar o Prometheus: ele relê o diretório a cada ciclo de compactação (~1 min). Logo depois, a própria compactação junta os blocos de 2h em blocos maiores (veja `promtool tsdb list` de novo em 1-2 min).
- Consulta: `increase(tsdb_backfill_orders_total{store="sp"}[1h])` com `time=` numa hora cheia entre 10h e 18h UTC → **≈ 540** (9 pedidos/min); de madrugada → **≈ 180** (3/min).

Script completo: [`solutions/01-backfill-openmetrics.sh`](../../solutions/01-backfill-openmetrics.sh).
</details>
