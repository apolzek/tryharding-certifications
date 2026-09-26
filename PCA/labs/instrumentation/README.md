# Lab: Instrumentação (counter, gauge, histogram, summary e boas práticas)

> **Em uma frase:** instrumentar é colocar "sensores" no seu código; este lab mostra dois apps (Go e Python) com métricas **certas** e **erradas** lado a lado, e você conserta as erradas até o `promtool check metrics` ficar quieto.

| | |
|---|---|
| **Domínio da prova** | Instrumentation and Exporters (16%) + Observability Concepts (18%) |
| **Portas** | Prometheus http://localhost:9130 · app-go http://localhost:9131 · app-python http://localhost:9132 |
| **Versões** | `client_golang v1.24.1` (build `golang:1.27-alpine`) · `prometheus_client 0.26.0` (`python:3.14-alpine`) · `prom/prometheus:v3.15.0` |
| **Teste** | `./test.sh` (sobe o enunciado, verifica os problemas, sobe o gabarito, verifica as soluções) |

---

## 🧠 Analogia: o painel do carro

- **Counter** é o **hodômetro**: só sobe (ou zera quando você troca de carro = restart). Pergunta útil: "a que velocidade?" → `rate()`.
- **Gauge** é o **marcador de combustível**: sobe e desce. O valor atual é o que importa.
- **Histogram** é o **radar da estrada** que conta quantos carros passaram em cada faixa de velocidade (≤ 40, ≤ 60, ≤ 80...). Os radares de várias estradas podem ser **somados** para saber a distribuição da cidade inteira.
- **Summary** é cada motorista dizendo **"meu p99 foi 90 km/h"**. É preciso para aquele carro, mas **não dá para somar** motoristas: a média dos p99 não é o p99 da cidade.

E um carro com painel ruim: velocímetro em **milhas** num país que usa km (unidade errada), luz "motor" que acende **uma por parafuso** (cardinalidade), marcador de combustível que só sobe (gauge usado como counter).

---

## 🏗️ Arquitetura

```
                        ┌──────────────┐  X-User-Id aleatório (1..5000)
                        │   loadgen    │──────────────────────────────┐
                        └──────┬───────┘                              │
             ┌─────────────────┼────────────────────┐                 │
             ▼                 ▼                    ▼                 ▼
     ┌──────────────┐  ┌──────────────┐     ┌────────────────┐
     │ app-go (a)   │  │ app-go-b (b) │     │  app-python    │
     │ :9131        │  │ (sem porta)  │     │  :9132         │
     │ backend 1x   │  │ backend 5x   │     │                │
     └──────┬───────┘  └──────┬───────┘     └───────┬────────┘
            │ /metrics        │ /metrics            │ /metrics
            │ (protobuf:      │                     │ (OpenMetrics: sem protobuf
            │  native hist.)  │                     │  no client Python)
            └────────┬────────┴──────────┬──────────┘
                     ▼   scrape a cada 5s
              ┌───────────────────────────────┐
              │ Prometheus :9130              │
              │ --enable-feature=exemplar-storage
              │ scrape_native_histograms: true│
              └───────────────────────────────┘
```

Cada app expõe três endpoints de métricas:

| Endpoint | Conteúdo | `promtool check metrics` |
|---|---|---|
| `/metrics` | boas + ruins (é o que o Prometheus raspa) | ❌ reclama |
| `/metrics/good` | só as boas + coletores padrão (`go_*`, `process_*`, `python_*`) | ✅ passa |
| `/metrics/bad` | só as ruins | ❌ reclama |

## ▶️ Como rodar

```bash
cd labs/instrumentation
docker compose up -d --build --wait
# Prometheus: http://localhost:9130   app-go: :9131/metrics   app-python: :9132/metrics
# espere ~1 minuto para os rate()[1m] terem dados

# gabarito (todos os exercícios resolvidos):
docker compose -f docker-compose.yml -f solutions/docker-compose.solutions.yml up -d --build --wait

docker compose -f docker-compose.yml -f solutions/docker-compose.solutions.yml down -v   # derruba qualquer um dos dois
./test.sh                  # teste completo (~2 min)
KEEP=1 ./test.sh           # idem, mas deixa a stack do gabarito no ar
```

`promtool` roda via Docker (nada para instalar): `docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics`.

---

## 🔍 Passo a passo

### 1. Os 4 tipos de métrica, no fio

```bash
curl -s localhost:9131/metrics/good | grep -E '^# (HELP|TYPE) (http_|backend_)'
```

```
# HELP backend_call_duration_seconds Latência das chamadas ao backend de estoque.
# TYPE backend_call_duration_seconds summary
# HELP http_request_duration_seconds Latência das requisições HTTP.
# TYPE http_request_duration_seconds histogram
# HELP http_requests_in_flight Requisições sendo atendidas agora.
# TYPE http_requests_in_flight gauge
# HELP http_requests_total Total de requisições HTTP atendidas.
# TYPE http_requests_total counter
```

| Tipo | O que é | Operações no client | Como consultar | Exemplo real |
|---|---|---|---|---|
| **Counter** | valor que **só sobe** (zera no restart) | `Inc()`, `Add(n≥0)` | `rate()`, `increase()` | requisições, erros, bytes enviados |
| **Gauge** | valor que **sobe e desce** | `Set()`, `Inc()`, `Dec()`, `Add()`, `Sub()` | valor direto, `avg_over_time`, `deriv` | memória, fila, temperatura, in-flight |
| **Histogram** | contagem de observações **por faixa** (`_bucket{le}`) + `_sum` + `_count` | `Observe(v)` | `histogram_quantile(φ, sum by (le) (rate(x_bucket[5m])))` | latência, tamanho de resposta |
| **Summary** | quantis **calculados no cliente** (`{quantile}`) + `_sum` + `_count` | `Observe(v)` | valor direto de `{quantile="0.99"}` | latência de um único processo |

Um histogram vira **várias séries**:

```bash
curl -s localhost:9131/metrics/good | grep 'http_request_duration_seconds.*checkout' | head -14
# http_request_duration_seconds_bucket{method="GET",route="/api/checkout",le="0.005"} 0
# ...
# http_request_duration_seconds_bucket{method="GET",route="/api/checkout",le="0.25"} 842
# http_request_duration_seconds_bucket{method="GET",route="/api/checkout",le="0.5"} 905
# http_request_duration_seconds_bucket{method="GET",route="/api/checkout",le="+Inf"} 905
# http_request_duration_seconds_sum{method="GET",route="/api/checkout"} 118.2
# http_request_duration_seconds_count{method="GET",route="/api/checkout"} 905
```

Buckets são **cumulativos** (`le` = "less or equal"): `le="0.5"` inclui tudo que está em `le="0.25"`. `le="+Inf"` é sempre igual ao `_count`.

### 2. Histogram vs Summary (cai muito na prova)

| | Histogram | Summary |
|---|---|---|
| Quantil calculado | no **servidor** (`histogram_quantile`) | no **cliente** |
| **Agregável** entre instâncias | ✅ `sum by (le)` | ❌ média de quantis não é quantil |
| Escolher quantil depois | ✅ qualquer φ, a qualquer momento | ❌ fixos no código (`Objectives`) |
| Erro do quantil | na **dimensão do valor**: depende da largura do bucket (interpolação linear) | na **dimensão do φ**: configurável (`0.99: 0.001`) |
| Custo no cliente | barato (incrementa um contador) | caro (stream de quantis, lock) |
| Custo no servidor | 1 série por bucket × labels | poucas séries |
| Precisa definir de antemão | os **buckets** | os **quantis** e a janela (`MaxAge`) |
| Python `prometheus_client` | ✅ | ⚠️ só `_count` e `_sum`, **sem quantis** |

A doc do Prometheus resume: **"use histogram na dúvida"**, e summary só se você precisa de um quantil exato de **um único processo** e não vai agregar.

Veja o erro de agregar summaries (réplica b é 5× mais lenta):

```promql
backend_call_duration_seconds{quantile="0.99"}        # a ≈ 0.050   b ≈ 0.249
avg(backend_call_duration_seconds{quantile="0.99"})   # ≈ 0.149  ← ERRADO
```

O p99 verdadeiro das duas juntas é ≈ **0.246** (resolva no [exercício 03](exercises/03-summary-para-histogram/)).

### 3. Native histograms

O histogram do Go foi criado com `NativeHistogramBucketFactor: 1.1`. Em vez de buckets fixos, o client usa **buckets exponenciais automáticos** (cada um no máximo 10% maior que o anterior) e manda tudo numa **única série** do tipo histogram. Isso exige **protobuf** no scrape e `scrape_native_histograms: true` no Prometheus:

```yaml
global:
  scrape_native_histograms: true          # negocia protobuf com quem suporta
scrape_configs:
  - job_name: app-go
    always_scrape_classic_histograms: true  # guarda TAMBÉM os _bucket clássicos
```

```promql
# native: sem _bucket e sem "by (le)"
histogram_quantile(0.99, sum by (job) (rate(http_request_duration_seconds{job="app-go"}[1m])))
histogram_count(rate(http_request_duration_seconds{job="app-go"}[1m]))   # req/s
histogram_fraction(0, 0.3, sum(rate(http_request_duration_seconds{job="app-go",route="/api/checkout"}[1m])))  # SLI ≤ 300ms sem precisar de bucket em 0.3!

# clássico (continua existindo graças ao always_scrape_classic_histograms)
histogram_quantile(0.99, sum by (job, le) (rate(http_request_duration_seconds_bucket[1m])))
```

Vantagens: resolução alta com custo baixo, não precisa escolher buckets, agregável. O client Python 0.26 **não** expõe native histograms.

### 4. Nomes e unidades

Formato: `<namespace>_<o_que_mede>_<unidade>_<sufixo>`, tudo em **snake_case**.

| Regra | ✅ Bom | ❌ Ruim (no lab) |
|---|---|---|
| Counter termina em `_total` | `http_requests_total` | `requestsCount` |
| Só counter termina em `_total` | `orders_processed_total` (counter) | `orders_processed_total` como **gauge** |
| **Unidade base**: segundos, bytes, metros, volts, ratio 0–1 | `checkout_duration_seconds` | `checkout_latency_ms` |
| Unidade por extenso, no plural | `cache_size_bytes` | `cache_size_kb` |
| snake_case | `http_requests_in_flight` | `requestsCount` |
| Sempre com HELP | `# HELP cart_items_total Itens...` | `cart_items_total` sem HELP |
| **Nada de label no nome** | `http_requests_total{code="500"}` | `http_requests_500_total` |
| Não repita o tipo no nome | `app_requests_total` | `app_requests_counter_total` |
| `_info` para metadados (valor 1) | `app_info{version="1.4.2"} 1` | `app_version{} 1.42` |
| Percentual é **ratio** (0–1) | `disk_used_ratio` | `disk_used_percent` |

O **linter** do Prometheus aponta boa parte disso:

```bash
curl -s localhost:9131/metrics | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics
# cache_size_kb metric names should not contain abbreviated units
# cart_items_total no help text
# checkout_latency_ms metric names should not contain abbreviated units
# orders_processed_total non-counter metrics should not have "_total" suffix
# requestsCount counter metrics should have "_total" suffix
# requestsCount metric names should be written in 'snake_case' not 'camelCase'
echo $?   # 3 → dá para usar em CI

curl -s localhost:9131/metrics/good | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics; echo $?
# (nada) 0
```

Se o nome fosse `checkout_latency_milliseconds`, a mensagem seria `use base unit "seconds" instead of "milliseconds"`.

O que o linter **não** pega: cardinalidade, label no nome (`_500_total`), gauge que só sobe com nome sem `_total`, unidade errada no **valor** (observar ms num histogram `_seconds`).

### 5. Labels e cardinalidade

Cada **combinação única** de valores de labels é uma **série** nova no TSDB (memória, disco, tempo de query). `http_requests_by_user_total{user_id}` é a bomba do lab:

```promql
count by (job) (http_requests_by_user_total)          # centenas e subindo
topk(5, count by (__name__) ({__name__=~".+"}))       # as métricas com mais séries
```

```bash
curl -s localhost:9131/metrics | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics --extended 2>/dev/null | head -4
# Metric                           Cardinality    Percentage
# http_requests_by_user_total      58             38.16%
# http_request_duration_seconds    28             18.42%
```

E na UI: **Status → TSDB Status** (http://localhost:9130/tsdb-status).

Regras:
- Labels devem ter **conjunto de valores limitado e pequeno** (método, rota **normalizada**, código de status, região).
- Nunca: user_id, e-mail, IP de cliente, request id, URL crua (`/users/123`), timestamp, mensagem de erro.
- A cardinalidade **multiplica**: 5 métodos × 50 rotas × 10 códigos × 11 buckets = 27.500 séries **por instância**.
- Dado por usuário/requisição vai para **logs** e **traces**. As métricas apontam para eles via **exemplars**.

### 6. Formatos de exposição

| Formato | Content-Type | Particularidades |
|---|---|---|
| Prometheus text **0.0.4** | `text/plain; version=0.0.4` | o clássico; sem exemplars, sem `_created` nativo, sem native histograms |
| **OpenMetrics** 1.0 | `application/openmetrics-text; version=1.0.0` | termina com `# EOF`; exemplars; `_created`; `# UNIT`; tipos `info`, `stateset`, `gaugehistogram`; família de counter sem `_total` no `# TYPE` |
| **Protobuf** | `application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited` | binário; é o que transporta **native histograms** |

```bash
curl -s -H 'Accept: application/openmetrics-text; version=1.0.0' localhost:9132/metrics | grep -A4 '^# HELP http_requests '
# # HELP http_requests Total de requisições HTTP atendidas.
# # TYPE http_requests counter
# http_requests_total{code="200",method="GET",route="/api/items"} 16.0
# http_requests_created{code="200",method="GET",route="/api/items"} 1.7904331339165254e+09
```

- `# HELP` = descrição; `# TYPE` = tipo (`counter|gauge|histogram|summary|untyped`, e no OpenMetrics também `info|stateset|gaugehistogram|unknown`).
- `_created` = timestamp (Unix) em que a série foi criada; ajuda a detectar resets. O client Python também o expõe no formato 0.0.4, mas como uma **gauge separada** (`http_requests_created`). Neste lab o Prometheus ingere essas linhas como séries comuns (`count({__name__=~".*_created"})` dá centenas). Para desligar no Python: `PROMETHEUS_DISABLE_CREATED_SERIES=True`.
- O formato é negociado pelo header `Accept`; no Prometheus a ordem de preferência vem de `scrape_protocols`.

Veja o [exercício 07](exercises/07-formatos-de-exposicao/).

### 7. Exemplars: da métrica ao trace

Um exemplar é um "post-it" pendurado num bucket: *"uma das requisições que caiu aqui foi o trace `5fb5...`, que levou 0.0616 s"*.

```bash
curl -s -H 'Accept: application/openmetrics-text' localhost:9131/metrics | grep -m1 trace_id
# http_request_duration_seconds_bucket{method="GET",route="/api/checkout",le="0.1"} 21 # {trace_id="5fb5b178..."} 0.0616 1.79e+09

curl -s -G localhost:9130/api/v1/query_exemplars \
  --data-urlencode 'query=http_request_duration_seconds_bucket{job="app-go"}' \
  --data-urlencode "start=$(( $(date +%s) - 300 ))" --data-urlencode "end=$(date +%s)" | head -c 300
```

Requisitos: o Prometheus com `--enable-feature=exemplar-storage` e o scrape em **OpenMetrics ou protobuf**. No Grafana, os exemplars aparecem como pontinhos no gráfico que abrem o trace no Tempo/Jaeger.

```go
httpDuration.WithLabelValues(r.Method, route).(prometheus.ExemplarObserver).
    ObserveWithExemplar(time.Since(start).Seconds(), prometheus.Labels{"trace_id": traceID()})
```

```python
HTTP_DURATION.labels("GET", path).observe(elapsed, exemplar={"trace_id": secrets.token_hex(16)})
```

### 8. Coletores padrão: `process_*`, `go_*`, `python_*`

Você ganha de graça, só por registrar os coletores (no Go, `prometheus.DefaultRegisterer` já traz; aqui registramos à mão com `collectors.NewGoCollector()` e `collectors.NewProcessCollector(...)`; no Python o `REGISTRY` padrão já traz):

```promql
rate(process_cpu_seconds_total{job=~"app.*"}[1m])     # CPU (segundos de CPU por segundo)
process_resident_memory_bytes{job=~"app.*"}           # RSS
process_open_fds / process_max_fds                    # uso de file descriptors
go_goroutines                                         # vazamento de goroutines?
go_memstats_heap_alloc_bytes
python_info                                           # {implementation="CPython", version="3.14.7"} 1
python_gc_objects_collected_total
```

`process_*` só funciona em Linux (lê `/proc`). Por isso **não** use o prefixo `process_` nas suas métricas.

### 9. O que instrumentar: Golden Signals, RED e USE

| Método | Para quê | Sinais | No lab |
|---|---|---|---|
| **4 Golden Signals** (Google SRE book) | qualquer serviço | **Latency**, **Traffic**, **Errors**, **Saturation** | histogram, `rate(http_requests_total)`, `code=~"5.."`, `http_requests_in_flight` |
| **RED** (Tom Wilkie) | serviços request/response | **Rate**, **Errors**, **Duration** | idem, sem saturação |
| **USE** (Brendan Gregg) | **recursos** (CPU, disco, fila, pool) | **Utilization**, **Saturation**, **Errors** | `process_cpu_seconds_total`, fila, `process_open_fds` |

```promql
# RED do checkout
sum by (job) (rate(http_requests_total{route="/api/checkout"}[1m]))                        # Rate
sum by (job) (rate(http_requests_total{route="/api/checkout",code=~"5.."}[1m]))
  / sum by (job) (rate(http_requests_total{route="/api/checkout"}[1m]))                    # Errors (ratio)
histogram_quantile(0.99, sum by (job, le) (rate(http_request_duration_seconds_bucket{route="/api/checkout"}[1m])))  # Duration
```

**Onde** instrumentar (guia oficial "Instrumentation"):

- **Serviços online** (HTTP/gRPC): RED na entrada **e** em cada chamada de saída (DB, cache, APIs). Middleware resolve 90%.
- **Serviços offline / filas / workers** (o `syncInventory` do lab): itens na fila, em processamento, processados, erros, duração, e o **timestamp do último processamento**.
- **Batch jobs**: duração, último sucesso (`_last_success_timestamp_seconds`), itens processados. Batch de vida curta → **Pushgateway** (veja [labs/exporters-pushgateway](../exporters-pushgateway/)).
- **Bibliotecas**: instrumente dentro da lib (pool de conexões, cache: hits/misses/size), aceitando um `Registerer` como parâmetro, para quem usa a lib ganhar métricas sem esforço.
- **Coisas que falham**: todo `catch`/`if err != nil` importante merece um counter.
- **Threads, pools, caches**: tamanho, em uso, hits/misses.

E **inicialize** as séries com labels conhecidos em zero (`payments.WithLabelValues("declined")`): série ausente ≠ série com 0 (`rate()` de algo que não existe é **vazio**).

---

## 🏭 Casos reais

### Caso 1: middleware HTTP padrão no Go (promhttp)

Em produção ninguém escreve o middleware à mão como o lab faz; o `promhttp` já traz:

```go
reqs := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "..."}, []string{"code", "method"})
dur  := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "...",
        Buckets: prometheus.DefBuckets, NativeHistogramBucketFactor: 1.1}, []string{"handler", "code", "method"})
inflight := prometheus.NewGauge(prometheus.GaugeOpts{Name: "http_requests_in_flight", Help: "..."})

h := promhttp.InstrumentHandlerInFlight(inflight,
       promhttp.InstrumentHandlerDuration(dur.MustCurryWith(prometheus.Labels{"handler": "checkout"}),
         promhttp.InstrumentHandlerCounter(reqs, checkoutHandler)))
```

Equivalentes: Spring Boot Actuator + Micrometer (`http_server_requests_seconds`), `prometheus-fastapi-instrumentator`, `express-prom-bundle`, o `grpc-ecosystem/go-grpc-middleware/providers/prometheus`.

### Caso 2: recording rules + alerta de SLO sobre o histogram

```yaml
groups:
  - name: checkout-slo
    rules:
      - record: job:http_request_duration_seconds:ratio_le_0_3_rate5m
        expr: |
          sum by (job) (rate(http_request_duration_seconds_bucket{route="/api/checkout",le="0.3"}[5m]))
            / sum by (job) (rate(http_request_duration_seconds_count{route="/api/checkout"}[5m]))
      - alert: CheckoutLatencySLOBreach
        expr: job:http_request_duration_seconds:ratio_le_0_3_rate5m < 0.95
        for: 10m
        labels: { severity: page }
        annotations:
          summary: "{{ $labels.job }}: só {{ $value | humanizePercentage }} dos checkouts em ≤300ms"
```

### Caso 3: contendo cardinalidade no Prometheus (quando o time do app não conserta a tempo)

```yaml
scrape_configs:
  - job_name: app-go
    sample_limit: 10000           # scrape com mais amostras que isso FALHA (up=0), protegendo o TSDB
    label_limit: 30
    label_value_length_limit: 200
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: http_requests_by_user_total
        action: drop
      - regex: user_id            # ou só arranca o label de todas as métricas
        action: labeldrop
```

### Caso 4: `_info` + `group_left` para enriquecer

É o mesmo padrão de `kube_pod_info`, `node_uname_info` e `go_build_info`:

```promql
sum by (job, instance) (rate(http_requests_total[5m]))
  * on (job, instance) group_left (version) app_info
```

Útil para comparar a taxa de erro entre versões durante um canary.

### Caso 5: linter no CI

```yaml
# .github/workflows/metrics-lint.yml
- name: promtool check metrics
  run: |
    ./app & sleep 2
    curl -s localhost:8080/metrics | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics
```

---

## 🧪 Exercícios

| # | Exercício | Linguagem |
|---|---|---|
| 01 | [Conserte os nomes e as unidades](exercises/01-nomes-e-unidades/) | Go + Python |
| 02 | [Escolha buckets para um SLO de 300 ms](exercises/02-buckets-para-slo/) | Go + Python |
| 03 | [Troque o summary por histogram e agregue o p99 de 2 réplicas](exercises/03-summary-para-histogram/) | Go |
| 04 | [Desarme a bomba de cardinalidade](exercises/04-bomba-de-cardinalidade/) | Go + Python |
| 05 | [Adicione uma métrica `_info`](exercises/05-metrica-info/) | Go + Python |
| 06 | [Instrumente uma função com counter + histogram](exercises/06-instrumentar-funcao/) | Go + Python |
| 07 | [Formatos de exposição, `_created` e exemplars](exercises/07-formatos-de-exposicao/) | curl |

Gabarito: [`solutions/app-go/main.go`](solutions/app-go/main.go), [`solutions/app-python/app.py`](solutions/app-python/app.py) e [`solutions/docker-compose.solutions.yml`](solutions/docker-compose.solutions.yml).

---

## ⚠️ Pegadinhas

1. **Gauge para contar coisas**: `orders_processed` como gauge com `Inc()` funciona até o restart, e aí `rate()` não compensa o reset (e `delta` numa gauge que só sobe vira lixo). Se só sobe, é **counter**.
2. **Milissegundos**: o Prometheus usa **segundos** em tudo (`rate` é por segundo, `histogram_quantile` devolve na unidade dos buckets). Misturar ms e s obriga o dashboard a adivinhar.
3. **Média de percentis**: `avg(x{quantile="0.99"})` é estatisticamente sem sentido. Some **buckets**, não quantis.
4. **`sum by (job)` sem `le`** antes do `histogram_quantile` clássico: você perde o `le` e o resultado é vazio/NaN.
5. **Série ausente ≠ zero**: counters com label só aparecem no primeiro `Inc()`. Inicialize com `WithLabelValues(...)`.
6. **`promtool check metrics` não vê cardinalidade**: use `--extended`, o TSDB Status ou `count by (__name__)`.
7. **Python força `_total`** em counters (`Counter("requests")` → `requests_total`) e expõe `_created`. No Go, o nome é exatamente o que você escreveu.
8. **Summary do Python não tem quantis.**
9. **Metadata no OpenMetrics**: a família do counter não tem `_total` (`# TYPE http_requests counter`), então `/api/v1/metadata?metric=http_requests_total` pode vir vazio para alvos raspados via OpenMetrics.
10. **Mudar buckets** muda as séries `_bucket`: comparar antes/depois no mesmo gráfico dá degraus estranhos.
11. **Timestamps nas métricas**: não exponha timestamp nas amostras; o Prometheus põe o do scrape. Timestamp como **valor** (`*_timestamp_seconds`) é ok.

## 🎓 Na prova PCA

O que costuma cair:
- Qual tipo usar para X (requisições → counter; memória → gauge; latência agregável → histogram).
- Histogram × summary: agregação, onde o quantil é calculado, custo.
- Convenções: `_total`, `_seconds`, `_bytes`, `_info`, base units, snake_case, sem label no nome.
- Cardinalidade: o que **não** pode virar label.
- Formatos: `# HELP`, `# TYPE`, `# EOF` do OpenMetrics, exemplars.
- Golden signals / RED / USE.

**1.** You need the 99th percentile latency across **20 replicas** of a service. Which metric type should the service expose?
- A) Gauge
- B) Summary with a 0.99 objective
- C) Histogram
- D) Counter

<details><summary>Resposta</summary>

**C.** Histogram: `histogram_quantile(0.99, sum by (le) (rate(x_bucket[5m])))` agrega os buckets das 20 réplicas. Quantis de summary **não podem ser agregados** (média de p99 ≠ p99).
</details>

**2.** Which metric name follows Prometheus naming best practices?
- A) `http_request_duration_ms`
- B) `httpRequestDurationSeconds`
- C) `http_request_duration_seconds`
- D) `http_request_duration_seconds_total` (histogram)

<details><summary>Resposta</summary>

**C.** snake_case, unidade base (segundos) e por extenso. A usa ms, B é camelCase, D usa `_total` num histogram (`_total` é só para counters).
</details>

**3.** A developer adds a label `request_id` to `http_requests_total`. What is the main problem?
- A) Label names can't contain underscores
- B) Unbounded cardinality: each request creates a new time series
- C) Counters cannot have labels
- D) `promtool check metrics` will reject it

<details><summary>Resposta</summary>

**B.** Cada valor único vira uma série: memória e disco explodem. O `promtool check metrics` **não** detecta isso (D é falso). IDs únicos vão para logs/traces.
</details>

**4.** In the OpenMetrics exposition format, how must the payload end?
- A) With an empty line
- B) With `# EOF`
- C) With `# END`
- D) No special ending is required

<details><summary>Resposta</summary>

**B.** `# EOF` é obrigatório no OpenMetrics; permite detectar resposta truncada. O formato text 0.0.4 não tem isso.
</details>

**5.** Which statement about summaries is TRUE?
- A) Quantiles are calculated by the Prometheus server at query time
- B) You can choose a different quantile at query time
- C) Quantiles are calculated on the client and cannot be meaningfully aggregated across instances
- D) Summaries require the `le` label

<details><summary>Resposta</summary>

**C.** A e B descrevem o histogram; D também (`le` é label do histogram; o summary usa `quantile`).
</details>

**6.** What does the RED method stand for?
- A) Requests, Errors, Downtime
- B) Rate, Errors, Duration
- C) Rate, Exceptions, Delay
- D) Resources, Events, Duration

<details><summary>Resposta</summary>

**B.** Rate, Errors, Duration, para serviços. O **USE** (Utilization, Saturation, Errors) é para recursos; os **4 golden signals** são Latency, Traffic, Errors, Saturation.
</details>

**7.** A counter `jobs_processed_total` is exposed. What is the correct way to express "jobs per second"?
- A) `jobs_processed_total / 60`
- B) `rate(jobs_processed_total[5m])`
- C) `deriv(jobs_processed_total[5m])`
- D) `delta(jobs_processed_total[5m])`

<details><summary>Resposta</summary>

**B.** `rate` é para counters e compensa resets. `deriv`/`delta` são para gauges.
</details>

**8.** You want to expose the running application version so it can be joined with other metrics. What is the conventional approach?
- A) A gauge whose value is the version number, e.g. `app_version 1.42`
- B) A label `version` on every metric
- C) An `_info` metric with value 1 and the version as a label, e.g. `app_info{version="1.4.2"} 1`
- D) A counter incremented at each deploy

<details><summary>Resposta</summary>

**C.** O padrão `_info` (como `go_build_info`, `node_uname_info`) guarda os metadados em labels com valor constante 1; junta-se com `* on (...) group_left (version) app_info`. B multiplicaria a cardinalidade de tudo e A não representa `1.4.2`.
</details>

**9.** Which is a valid reason to prefer a summary over a histogram?
- A) You need to aggregate latency across many instances
- B) You need an accurate quantile from a single instance and cannot predict the value range to choose buckets
- C) You want to compute an Apdex score
- D) You want to change the quantile later

<details><summary>Resposta</summary>

**B.** É o caso de uso de summary descrito na doc. Apdex usa buckets de histogram (A, C, D são pró-histogram).
</details>

## 📝 Cola rápida

- **Counter** só sobe → `rate`/`increase`. **Gauge** sobe e desce → valor direto. **Histogram** → `histogram_quantile(φ, sum by (le) (rate(x_bucket[5m])))`. **Summary** → `{quantile}` pronto, **não agregável**.
- Nomes: `snake_case`, `<o_que>_<unidade>_total`, unidades **base** (`_seconds`, `_bytes`, `_ratio`), `_info` com valor 1, nada de label no nome, nada de tipo no nome.
- Buckets: um **exatamente** em cada limiar de SLO; poucos; native histograms (`NativeHistogramBucketFactor`) dispensam a escolha.
- Cardinalidade = produto dos valores dos labels. Nunca IDs, e-mails, URLs cruas.
- `promtool check metrics` = linter de nomes/HELP/unidades (exit ≠ 0). `--extended` = cardinalidade.
- Formatos: text 0.0.4 · OpenMetrics (`# EOF`, exemplars, `_created`) · protobuf (native histograms).
- Exemplars: `--enable-feature=exemplar-storage` + OpenMetrics/protobuf + `/api/v1/query_exemplars`.
- Golden signals (Latency, Traffic, Errors, Saturation) · RED (serviços) · USE (recursos).
- Coletores padrão: `process_*` (CPU, RSS, FDs), `go_*` (goroutines, GC, heap), `python_*` (gc, info).

## 📚 Referências

- https://prometheus.io/docs/concepts/metric_types/
- https://prometheus.io/docs/practices/naming/
- https://prometheus.io/docs/practices/instrumentation/
- https://prometheus.io/docs/practices/histograms/
- https://prometheus.io/docs/instrumenting/exposition_formats/
- https://prometheus.io/docs/specs/native_histograms/
- https://github.com/prometheus/OpenMetrics/blob/main/specification/OpenMetrics.md
- https://pkg.go.dev/github.com/prometheus/client_golang/prometheus
- https://prometheus.github.io/client_python/
