# Exercício 02: escolha os buckets para um SLO

**SLO:** *"95% das requisições de `/api/checkout` respondem em até **300 ms**"*.

O SLI (a fração de requisições "boas") sai direto de um bucket do histogram:

```promql
  sum(rate(http_request_duration_seconds_bucket{route="/api/checkout", le="0.3"}[5m]))
/
  sum(rate(http_request_duration_seconds_count{route="/api/checkout"}[5m]))
```

## O problema

Os apps usam os buckets padrão (`.005 .01 .025 .05 .1 .25 .5 1 2.5 5 10`). **Não existe `le="0.3"`**:

```bash
curl -s localhost:9131/metrics/good | grep 'route="/api/checkout"' | grep _bucket
curl -s localhost:9130/api/v1/query --data-urlencode 'query=http_request_duration_seconds_bucket{le="0.3"}'
# "result":[]
```

Você poderia usar `le="0.25"` (subestima) ou `le="0.5"` (superestima), ou `histogram_quantile` (que **interpola** dentro do bucket, com erro). Nenhuma é exata.

## O que fazer

Troque os buckets de `http_request_duration_seconds` nos dois apps para ter **um bucket exatamente no alvo** (0.3) e **mais resolução perto dele**. Algo como `0.025, 0.05, 0.1, 0.2, 0.3, 0.45, 0.6, 1, 2.5`.

## Como verificar

```bash
docker compose up -d --build --wait    # espere ~1 min de dados
curl -s localhost:9130/api/v1/query --data-urlencode 'query=
  sum by (job) (rate(http_request_duration_seconds_bucket{route="/api/checkout",le="0.3"}[1m]))
/ sum by (job) (rate(http_request_duration_seconds_count{route="/api/checkout"}[1m]))'
```

**Resultado esperado:** ≈ **0.90** para cada job (o app tem ~7% de cauda lenta entre 350 e 600 ms). Ou seja: **o SLO de 95% está sendo violado**, e agora você consegue provar isso com precisão.

> 💡 **Regras para escolher buckets:** (1) um bucket **exatamente** em cada limiar de SLO/alerta; (2) resolução onde está a massa da distribuição; (3) poucos buckets, pois **cada bucket é uma série** × cada combinação de labels; (4) mudar buckets depois quebra a continuidade do histórico. Native histograms resolvem boa parte disso (buckets automáticos exponenciais).

<details><summary>✅ Solução</summary>

```go
httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "http_request_duration_seconds",
    Help:    "Latência das requisições HTTP.",
    Buckets: []float64{0.025, 0.05, 0.1, 0.2, 0.3, 0.45, 0.6, 1, 2.5},
    NativeHistogramBucketFactor: 1.1,
}, []string{"method", "route"})
```

```python
HTTP_DURATION = Histogram("http_request_duration_seconds", "Latência das requisições HTTP.",
                          ["method", "route"], buckets=[0.025, 0.05, 0.1, 0.2, 0.3, 0.45, 0.6, 1, 2.5])
```

Alerta de SLO usando o bucket:

```yaml
- alert: CheckoutLatencySLOBreach
  expr: |
    sum(rate(http_request_duration_seconds_bucket{route="/api/checkout",le="0.3"}[5m]))
      / sum(rate(http_request_duration_seconds_count{route="/api/checkout"}[5m])) < 0.95
  for: 10m
  labels: { severity: page }
```
</details>
