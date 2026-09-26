# Exercício 03: troque o summary por histogram e agregue o p99 das 2 réplicas

O app Go roda em **2 réplicas** (`replica="a"` rápida e `replica="b"` 5× mais lenta no backend). A métrica `backend_call_duration_seconds` é um **summary**: cada réplica calcula seus próprios quantis.

## O problema

```promql
backend_call_duration_seconds{quantile="0.99"}
# replica="a" ≈ 0.050   replica="b" ≈ 0.249

avg(backend_call_duration_seconds{quantile="0.99"})
# ≈ 0.149   ← ERRADO! "média de percentis" não é percentil de nada
```

Metade das chamadas vem da réplica b (50–250 ms). O p99 **real** do conjunto é o p98 da réplica b: ≈ **0.246 s**. A média dos p99 (0.149) mente por quase 100 ms.

## O que fazer (só no `app-go/main.go`)

1. Troque `prometheus.NewSummary` por `prometheus.NewHistogram` com buckets que cubram 10 ms a 1 s, com resolução perto de 0.2–0.3 s.
2. Escreva a query do p99 **agregado** das duas réplicas.

## Como verificar

```bash
docker compose up -d --build --wait     # espere ~1 min
curl -s localhost:9130/api/v1/query --data-urlencode \
  'query=histogram_quantile(0.99, sum by (le) (rate(backend_call_duration_seconds_bucket{job="app-go"}[1m])))'
```

**Esperado:** entre **0.20 e 0.26**.

> ⚠️ Se você rodar o gabarito junto com o app-python original, a metadata de `backend_call_duration_seconds` terá dois tipos (histogram no Go, summary no Python). Mesmo nome com tipos diferentes é outra má prática: evite.

<details><summary>✅ Solução</summary>

```go
backendDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name:    "backend_call_duration_seconds",
    Help:    "Latência das chamadas ao backend de estoque.",
    Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.5, 1},
})
```

O resto do código (`backendDuration.Observe(...)`) não muda: Summary e Histogram implementam a mesma interface `Observer`.

```promql
histogram_quantile(0.99, sum by (le) (rate(backend_call_duration_seconds_bucket[5m])))
# por réplica:
histogram_quantile(0.99, sum by (replica, le) (rate(backend_call_duration_seconds_bucket[5m])))
```

**Regra:** para agregar, some os **buckets** (`sum by (le)`) e só depois calcule o quantil. O `le` **tem** que sobreviver à agregação.
</details>
