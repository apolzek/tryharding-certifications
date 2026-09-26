# 09 · Janela out-of-order

**Objetivo:** entender o que o TSDB faz com amostras que chegam **atrasadas** (timestamp menor do que a última já gravada).

O lab tem `storage.tsdb.out_of_order_time_window: 30m`. O gerador pode expor `tsdb_demo_timestamped` com o timestamp deslocado para o passado: `/control?ts_offset=<segundos>`.

## 🎯 Tarefa

1. `curl -s 'localhost:9151/control?ts_offset=600'` (10 min no passado). Que contador sobe?
2. `curl -s 'localhost:9151/control?ts_offset=3600'` (1h no passado). E agora?
3. O que aconteceria com `out_of_order_time_window: 0` (o padrão)?
4. Volte: `curl -s 'localhost:9151/control?ts_offset=0'`.

<details><summary>✅ Solução</summary>

```bash
curl -s localhost:9150/metrics | grep -E '^prometheus_tsdb_(head_out_of_order_samples_appended|too_old_samples|out_of_order_samples)_total'
```

- 10 min atrás (dentro da janela de 30m): **aceita**, soma em `prometheus_tsdb_head_out_of_order_samples_appended_total`. Amostras OOO vão para uma head separada e um log próprio, o **WBL** (`/prometheus/wbl`).
- 1h atrás (fora da janela): **rejeitada**, soma em `prometheus_tsdb_too_old_samples_total`.
- Janela 0 (padrão): qualquer amostra fora de ordem é rejeitada (`prometheus_tsdb_out_of_order_samples_total`; no log do scrape: `out of order sample`).
- Uso real: **remote write** de agentes que ficaram offline, OTLP, backfill "quente". Com scrape normal a ordem é garantida, então raramente precisa.

Script: [`solutions/09-out-of-order.sh`](../../solutions/09-out-of-order.sh).
</details>
