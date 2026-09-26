# Exercício 06: instrumente uma função com counter + histogram (Go e Python)

`processPayment()` (Go) / `process_payment()` (Python) é chamada em cada checkout, demora 5–40 ms e é **recusada em ~10%** das vezes. Hoje ela é invisível: não há métrica nenhuma.

## O que fazer

Nos dois apps, crie e alimente:

| Métrica | Tipo | Labels |
|---|---|---|
| `payments_total` | counter | `result` = `success` \| `declined` |
| `payment_duration_seconds` | histogram | nenhum (buckets entre 5 ms e 100 ms) |

> ⚠️ **Não** chame de `process_payment_...`: o prefixo `process_` é do **process collector** padrão (`process_cpu_seconds_total`, `process_resident_memory_bytes`...). Além de confundir quem lê, colisão exata de nome com uma métrica já registrada faz o `MustRegister` entrar em pânico.

## Como verificar

```bash
docker compose up -d --build --wait     # espere ~1 min
curl -s localhost:9130/api/v1/query --data-urlencode 'query=
  sum by (job) (rate(payments_total{result="declined"}[2m])) / sum by (job) (rate(payments_total[2m]))'
# ≈ 0.10 para cada job
curl -s localhost:9130/api/v1/query --data-urlencode 'query=
  histogram_quantile(0.9, sum by (job, le) (rate(payment_duration_seconds_bucket[1m])))'
# ≈ 0.035 (o p90 de uma uniforme 5–40ms)
```

<details><summary>✅ Solução</summary>

**Go:** instrumentação num `defer`, sem mexer na lógica (retorno nomeado para enxergar o erro):

```go
payments = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "payments_total", Help: "Pagamentos processados, por resultado.",
}, []string{"result"})
paymentDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "payment_duration_seconds", Help: "Duração do processamento de pagamento.",
    Buckets: []float64{0.005, 0.01, 0.02, 0.03, 0.04, 0.05, 0.1},
})

func processPayment() (err error) {
    start := time.Now()
    defer func() {
        paymentDuration.Observe(time.Since(start).Seconds())
        result := "success"
        if err != nil { result = "declined" }
        payments.WithLabelValues(result).Inc()
    }()
    // ... lógica original ...
}
```

Alternativa idiomática: `timer := prometheus.NewTimer(paymentDuration); defer timer.ObserveDuration()`.

**Python:** decorators/context managers do client:

```python
PAYMENTS = Counter("payments_total", "Pagamentos processados, por resultado.", ["result"])
PAYMENT_DURATION = Histogram("payment_duration_seconds", "Duração do processamento de pagamento.",
                             buckets=[0.005, 0.01, 0.02, 0.03, 0.04, 0.05, 0.1])

@PAYMENT_DURATION.time()
def process_payment():
    sleep_ms(5, 40)
    if random.random() < 0.10:
        PAYMENTS.labels("declined").inc()
        raise Declined("card declined")
    PAYMENTS.labels("success").inc()
```

Outros atalhos do Python: `Counter.count_exceptions()`, `Gauge.track_inprogress()`, `Summary.time()`.

**Dica de ouro:** inicialize as séries com labels conhecidos (`payments.WithLabelValues("declined")` sem `.Inc()`) para elas existirem com `0` desde o boot. Sem isso, `rate(payments_total{result="declined"}[5m])` fica **vazio** (não zero) até a primeira recusa, e alertas baseados nela não disparam.
</details>
