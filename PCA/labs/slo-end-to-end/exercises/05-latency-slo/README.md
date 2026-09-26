# 05 · SLO de latência com histograma

**Objetivo:** "**99%** das requisições em até **300ms**". Transformar isso num SLI a partir do histograma `http_request_duration_seconds` (classic: `_bucket{le}`, `_count`, `_sum`) e num alerta de burn rate.

Complete o [`latency.yml`](latency.yml):

1. `job:slo_latency_slow_per_request:ratio_rate5m` e `..._rate1h`: fração de requisições **lentas**, pela **razão de buckets**.
2. `job:slo_latency_good_per_request:fraction_rate5m`: fração **rápida**, com `histogram_fraction()`.
3. `SLOLatencyBudgetBurnFast`: 14.4x em 1h e 5m (budget = 1%), `for: 2m`, `severity: page`.

## 🎯 Critério de pronto

```bash
cd exercises/05-latency-slo
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules latency.test.yml
```

No lab rodando: `curl -s 'localhost:9171/chaos?latency_ms=400&latency_ratio=0.3'` (30% das requisições ganham +400ms) e veja o `SLOLatencyBudgetBurnFast` disparar em ~1 min.

## 💡 Dica

Buckets classic são **cumulativos**: `le="0.3"` já é "tudo até 300ms". Por isso o SLO só é exato se **existir** um bucket exatamente no limiar. `histogram_fraction(0, 0.3, <histograma>)` devolve a fração entre 0 e 0.3.

<details><summary>✅ Solução</summary>

```yaml
- record: job:slo_latency_slow_per_request:ratio_rate5m
  expr: |
    1 - (
      sum by (job) (rate(http_request_duration_seconds_bucket{job="checkout",le="0.3"}[5m]))
      /
      sum by (job) (rate(http_request_duration_seconds_count{job="checkout"}[5m]))
    )
- record: job:slo_latency_good_per_request:fraction_rate5m
  expr: |
    histogram_fraction(0, 0.3,
      sum by (job, le) (rate(http_request_duration_seconds_bucket{job="checkout"}[5m])))
```

- **Razão de buckets**: exata, funciona em qualquer versão, mas só no limiar de um bucket existente.
- **`histogram_fraction`**: nasceu para native histograms (limiar qualquer, buckets finos). No Prometheus 3.x também aceita classic (com `le` no `by`); se o limiar cair **no meio** de um bucket classic, ele interpola (estimativa).
- **Não** use `histogram_quantile(0.99, ...) < 0.3` como SLI: dá um sim/não por janela, não uma **contagem** de eventos bons/ruins, e não permite calcular budget.

Arquivo: [`solutions/05-latency-slo.yml`](../../solutions/05-latency-slo.yml).
</details>
