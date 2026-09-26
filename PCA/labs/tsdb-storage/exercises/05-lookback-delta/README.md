# 05 · Lookback delta: consulta num instante sem amostra

**Objetivo:** provar, com uma query num timestamp fixo, que um **instant vector** olha até **5 min para trás** atrás da última amostra.

Pré-requisito: exercício 01. A série `tsdb_backfill_sparse_gauge` tem **1 amostra a cada 30 min** (valor = hora UTC da amostra).

## 🎯 Tarefa

Seja `T` uma hora cheia dentro do dia importado (ex.: `END - 3600`, onde `END=$(( $(date +%s)/3600*3600 - 2*3600 ))`). Consulte `tsdb_backfill_sparse_gauge` em:

| `time=` | resultado esperado |
|---|---|
| `T` | a amostra de T |
| `T + 4m` | ? |
| `T + 6m` | ? |
| `T + 6m` com `lookback_delta=10m` | ? |

## 💡 Dica

A API `/api/v1/query` aceita `time=` e também o parâmetro `lookback_delta=` (sobrescreve o `--query.lookback-delta` só nessa query).

<details><summary>✅ Solução</summary>

```bash
END=$(( $(date +%s) / 3600 * 3600 - 2 * 3600 )); T=$((END - 3600))
curl -s localhost:9150/api/v1/query --data-urlencode query=tsdb_backfill_sparse_gauge --data-urlencode time=$((T+240))  # acha (valor = hora de T)
curl -s localhost:9150/api/v1/query --data-urlencode query=tsdb_backfill_sparse_gauge --data-urlencode time=$((T+360))  # result: []
curl -s localhost:9150/api/v1/query --data-urlencode query=tsdb_backfill_sparse_gauge --data-urlencode time=$((T+360)) \
  --data-urlencode lookback_delta=10m                                                                                   # acha de novo
```

- Um seletor instantâneo pega, para cada série, a **amostra mais recente dentro de `[t - 5m, t]`**. Em T+4m a amostra de T está dentro; em T+6m, não.
- É por isso que um gráfico dessa série fica "picotado": 5 min de linha, 25 min de buraco.
- Scrape/regra com intervalo **> 5m** gera esse buraco em qualquer query. Por isso ninguém usa `scrape_interval` maior que ~2m.

Script: [`solutions/05-lookback-delta.sh`](../../solutions/05-lookback-delta.sh).
</details>
