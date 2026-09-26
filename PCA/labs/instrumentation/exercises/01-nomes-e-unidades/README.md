# Exercício 01: conserte os nomes (e faça o `promtool` passar)

**Objetivo:** `curl -s localhost:9131/metrics | promtool check metrics` (Go) e o mesmo para `:9132` (Python) devem terminar **sem nenhuma linha** e com exit code `0`.

## O problema

```bash
curl -s localhost:9131/metrics | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics
# cache_size_kb metric names should not contain abbreviated units
# cart_items_total no help text
# checkout_latency_ms metric names should not contain abbreviated units
# orders_processed_total non-counter metrics should not have "_total" suffix
# requestsCount counter metrics should have "_total" suffix
# requestsCount metric names should be written in 'snake_case' not 'camelCase'
```

No Python a lista é parecida, com dois detalhes: o client **força** o `_total` (então vira `requestsCount_total`, só o camelCase é apontado) e, no formato texto 0.0.4, ele também expõe `*_created`, que herdam o nome ruim.

## O que fazer (em `app-go/main.go` e `app-python/app.py`)

| Métrica ruim | Problema | Conserto esperado |
|---|---|---|
| `requestsCount` | camelCase, counter sem `_total` | `checkouts_total` |
| `checkout_latency_ms` | milissegundos, abreviado | `checkout_duration_seconds` **observando segundos** (e buckets em segundos!) |
| `orders_processed_total` (Gauge) | gauge usado como counter | vira **Counter** (o nome já estava certo) |
| `cache_size_kb` | unidade abreviada e não-base | `cache_size_bytes` (multiplique o valor por 1024) |
| `cart_items_total` | sem HELP | adicione um HELP |

Deixe a `http_requests_by_user_total` para o [exercício 04](../04-bomba-de-cardinalidade/) (o promtool nem vê o problema dela).

## Como verificar

```bash
docker compose up -d --build --wait
for p in 9131 9132; do
  curl -s localhost:$p/metrics | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics && echo "OK :$p"
done
```

> 💡 **Dica:** trocar o nome de uma métrica **quebra dashboards e alertas** que usam o nome antigo. Em produção, exponha as duas durante uma transição, ou use `metric_relabel_configs` no Prometheus para renomear sem mexer no código.

<details><summary>✅ Solução</summary>

Arquivos completos: [`solutions/app-go/main.go`](../../solutions/app-go/main.go) e [`solutions/app-python/app.py`](../../solutions/app-python/app.py) (procure `EX01`).

```go
checkouts = prometheus.NewCounter(prometheus.CounterOpts{Name: "checkouts_total", Help: "Checkouts realizados."})
checkoutDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
    Name: "checkout_duration_seconds", Help: "Latência do checkout.",
    Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1}, // em SEGUNDOS
})
ordersProcessed = prometheus.NewCounter(prometheus.CounterOpts{Name: "orders_processed_total", Help: "Pedidos processados."})
cacheSize = prometheus.NewGauge(prometheus.GaugeOpts{Name: "cache_size_bytes", Help: "Tamanho do cache."})
// ...
checkoutDuration.Observe(time.Since(start).Seconds())   // não Milliseconds()
cacheSize.Set(float64(kb * 1024))
```

```python
CHECKOUTS = Counter("checkouts_total", "Checkouts realizados.", registry=BAD)
CHECKOUT_DURATION = Histogram("checkout_duration_seconds", "Latência do checkout.",
                              buckets=[0.05, 0.1, 0.25, 0.5, 1], registry=BAD)
ORDERS_PROCESSED = Counter("orders_processed_total", "Pedidos processados.", registry=BAD)
CACHE_SIZE = Gauge("cache_size_bytes", "Tamanho do cache.", registry=BAD)
CART_ITEMS = Counter("cart_items_total", "Itens adicionados ao carrinho.", registry=BAD)
```

Para rodar o gabarito: `docker compose -f docker-compose.yml -f solutions/docker-compose.solutions.yml up -d --build --wait`.
</details>
