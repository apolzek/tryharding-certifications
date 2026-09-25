# `histogram_sum()`: a soma de tudo que foi observado

> **Em uma frase:** `histogram_sum(v)` devolve a **soma dos valores observados** guardada em cada **native histogram** de `v`. Com `rate()`, vira "**quantidade por segundo**": bytes/s num histograma de tamanhos, ou "segundos de trabalho por segundo" (= **concorrência média**) num histograma de durações. Séries float são **ignoradas**.

| | |
|---|---|
| **Assinatura** | `histogram_sum(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ **Native** histogram · ❌ Classic (`x_sum` já é o número; `histogram_sum(x_sum)` → vazio) |
| **Unidade do resultado** | a unidade observada (bytes, segundos); com `rate`, unidade/s |
| **Equivalente classic** | `rate(x_sum[5m])` |
| **Dashboard** | http://localhost:3300/d/fn-histogram_sum |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a balança do caixa

No caixa do supermercado, cada produto passa pela **balança**. O sistema anota o peso de cada um em faixas ("até 100g", "até 1kg"...) **e** mantém um **peso total** acumulado.

- `histogram_count` = **quantos produtos** passaram.
- `histogram_sum` = **quantos quilos** passaram, somando tudo.
- `histogram_sum(rate(...))` = **quilos por segundo** que passam pelo caixa.

Troque "produto" por "resposta HTTP" e "peso" por "bytes": `histogram_sum(rate(...))` é o **throughput de rede**.

Troque "peso" por "duração do job": cada job de 8s que termina "adiciona 8 segundos de trabalho". Se chegam 0.5 job/s, o sistema acumula **4 segundos de trabalho a cada segundo**, ou seja, em média **4 workers ocupados ao mesmo tempo**. Isso é a **Lei de Little** (concorrência = taxa × duração média).

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_sum_http_response_size_bytes{route="/api/items"}` | histogram (native + classic) | **~50 resp/s** de **~2 KB** → ~100 KB/s |
| `histogram_sum_http_response_size_bytes{route="/download"}` | histogram (native + classic) | **~1 resp/s** de **~5 MB** → ~5 MB/s |
| `histogram_sum_job_duration_seconds{queue="emails"}` | histogram (native + classic) | **10 jobs/s** de **~0.2s** → ~2 workers ocupados |
| `histogram_sum_job_duration_seconds{queue="reports"}` | histogram (native + classic) | **0.5 job/s** de **~8s** → ~4 workers. **A cada 5 min, por 90s**, os relatórios levam **~16s** → ~8 workers |

```bash
curl -s localhost:8088/metrics | grep -E '^histogram_sum_job_duration_seconds_(sum|count)'
# histogram_sum_job_duration_seconds_sum{queue="reports"} 2410.7
# histogram_sum_job_duration_seconds_count{queue="reports"} 290
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_sum
```

Espere **~2 min** para `[1m]` e **~5 min** para `[5m]` e um período lento dos relatórios.

---

## 🔍 Queries passo a passo

### 1. Throughput em bytes/s

```promql
histogram_sum(rate(histogram_sum_http_response_size_bytes[1m]))
```

**O que faz:** `rate` = histograma "por segundo" do último minuto; `histogram_sum` = total de bytes desse histograma → **bytes por segundo**.
**Resultado esperado:**

| route | valor |
|---|---|
| `/api/items` | ≈ **100 kB/s** (50 × 2 KB) |
| `/download` | ≈ **5 MB/s** (1 × 5 MB), mais ruidoso (só ~60 downloads por minuto) |

**Moral:** 50× menos requisições, mas o `/download` manda 50× mais bytes. Olhar só req/s ([`histogram_count()`](../histogram_count/)) enganaria.

---

### 2. Native vs classic: mesma soma

```promql
histogram_sum(rate(histogram_sum_http_response_size_bytes{route="/download"}[1m]))   # native
rate(histogram_sum_http_response_size_bytes_sum{route="/download"}[1m])              # classic
```

**Resultado esperado:** **linhas sobrepostas**. O `_sum` classic e o sum do native carregam o mesmo total.

---

### 3. Quantos bytes em 5 minutos?

```promql
histogram_sum(increase(histogram_sum_http_response_size_bytes[5m]))
```

**Resultado esperado:** `/download` ≈ **1.5 GB** (5 MB × 300), `/api/items` ≈ **30 MB** (100 kB × 300). Útil para "quanto de banda esse endpoint consumiu na última hora" (com `[1h]`).

---

### 4. Durações: `histogram_sum(rate(...))` = workers ocupados

```promql
histogram_sum(rate(histogram_sum_job_duration_seconds[1m]))
```

**O que faz:** soma dos segundos de trabalho concluídos, por segundo. A unidade "segundos por segundo" se cancela: sobra um **número de workers**.
**Resultado esperado:**

| queue | normal | relatórios lentos (90s a cada 5 min) |
|---|---|---|
| emails | ≈ **2** (10 × 0.2) | ≈ 2 |
| reports | ≈ **4** (0.5 × 8) | ≈ **8** (0.5 × 16) |

> 💡 A duração é registrada quando o job **termina**, por isso o degrau aparece "atrasado" e o sinal do `reports` é ruidoso (só ~30 jobs/min).

---

### 5. ...enquanto o **número** de jobs não muda

```promql
histogram_count(rate(histogram_sum_job_duration_seconds[1m]))
```

**Resultado esperado:** `emails` ≈ **10/s** e `reports` ≈ **0.5/s**, retos, o tempo todo. A fila não recebeu mais trabalho: cada trabalho ficou **mais pesado**. Só o `histogram_sum` mostra isso (e ele alimenta [`histogram_avg()`](../histogram_avg/)).

---

### 6. Total de workers ocupados, e o que acontece em classic

```promql
histogram_sum(sum(rate(histogram_sum_job_duration_seconds[1m])))   # ✅ ≈ 6 (≈ 10 no período lento)
histogram_sum(rate(histogram_sum_job_duration_seconds_sum[1m]))    # ❌ vazio (x_sum é float)
```

**Resultado esperado:** ≈ **6** workers (2 + 4), subindo para ≈ **10** quando os relatórios ficam lentos. Se o seu pool tem 8 workers, esse é o momento em que a fila começa a crescer. A segunda query dá **vazio**.

---

## 🏭 Casos reais

> O cenário fake imita os casos 1 e 2: `histogram_sum_http_response_size_bytes{route}` (tamanho de resposta) e `histogram_sum_job_duration_seconds{queue}` (duração de jobs).

### Caso 1: banda consumida por rota no ingress

O ingress-nginx expõe o histograma `nginx_ingress_controller_response_size` (bytes). O `_sum` é o total de bytes enviados:

```promql
# bytes/s por ingress
sum by (ingress) (rate(nginx_ingress_controller_response_size_sum[5m]))
# native: sum by (ingress) (histogram_sum(rate(nginx_ingress_controller_response_size[5m])))
```

Uso real: achar a rota que está estourando o custo de egress da nuvem (o `/download` do painel 1, com 1 req/s e 5 MB/s).

```yaml
- record: ingress:nginx_ingress_controller_response_size:bytes_rate5m
  expr: sum by (ingress) (rate(nginx_ingress_controller_response_size_sum[5m]))
- alert: IngressEgressHigh
  # > 50 MB/s sustentado em um único ingress
  expr: ingress:nginx_ingress_controller_response_size:bytes_rate5m > 50e6
  for: 30m
  labels: { severity: ticket }
```

### Caso 2: dimensionar o pool de workers (Lei de Little)

Um worker Celery/Sidekiq/Go expõe `job_duration_seconds{queue}`. `histogram_sum(rate(...))` = número médio de workers ocupados:

```yaml
groups:
  - name: workers
    rules:
      - record: queue:job_duration_seconds:busy_workers
        expr: histogram_sum(rate(job_duration_seconds[5m]))   # classic: rate(job_duration_seconds_sum[5m])
      - alert: WorkerPoolSaturating
        # pool tem 8 workers por fila
        expr: queue:job_duration_seconds:busy_workers / 8 > 0.8
        for: 10m
```

É o painel 4: os relatórios passam de 4 para 8 workers ocupados sem nenhum job a mais.

### Caso 3: tempo de CPU gasto em GC / em requisições

```promql
# segundos de GC por segundo (fração do tempo parado em GC) — summary do client_golang
rate(go_gc_duration_seconds_sum[5m])
```

Mesmo raciocínio: soma de durações por segundo = fração do tempo gasta naquilo. `0.02` = 2% do tempo em pausas de GC.

---

## ✅ Quando usar

- **Throughput de bytes** a partir de um histograma de tamanhos: `histogram_sum(rate(http_response_size_bytes[5m]))`.
- **Concorrência / ocupação** a partir de um histograma de durações (Lei de Little): quantos workers, conexões ou threads estão ocupados em média.
- **Custo total**: soma de tempo de CPU, de tempo de fila, de valor de pedidos (`order_value_reais`).
- **Numerador de médias**: `histogram_sum / histogram_count` = [`histogram_avg()`](../histogram_avg/).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| A métrica é **classic** | `rate(x_sum[5m])` |
| Quer o **número** de eventos | [`histogram_count()`](../histogram_count/) |
| Quer o **valor típico** de um evento | [`histogram_avg()`](../histogram_avg/) · [`histogram_quantile()`](../histogram_quantile/) |
| Somar **séries** diferentes | [`sum()`](../sum/) (agregador) — e dá para combinar: `histogram_sum(sum(rate(x[5m])))` |

## ⚠️ Pegadinhas

1. **Só native**: `histogram_sum(x_sum)` → vazio, sem erro.
2. **Sem `rate`/`increase`**: é o total desde o start do processo.
3. **Observações negativas** reduzem a soma (histogramas podem ter valores negativos, ex.: diferença de temperatura). Aí a soma não é mais "total de trabalho".
4. **Lei de Little conta jobs que terminaram**: jobs longos em andamento só entram quando acabam, então o número atrasa e fica ruidoso com pouco tráfego.
5. **Não é estimativa**: o `sum` é exato (não depende dos buckets), diferente de quantis e desvio padrão.
6. **Não confunda** `histogram_sum` (soma **dentro** do histograma) com `sum` (soma **entre séries**).

## 🎓 Na prova PCA

O que costuma cair:
- `_sum` de um histograma/summary classic é um counter com a **soma dos valores observados** → `rate(x_sum[5m])` = unidade por segundo.
- **Média** = `rate(x_sum) / rate(x_count)`. Cai muito.
- `histogram_sum` é o equivalente **native**; em floats → vazio.
- Não confundir `sum()` (agregador entre séries) com `histogram_sum()` (valor dentro do histograma) nem com `sum_over_time()` (soma de amostras no tempo).

**1.** `http_response_size_bytes` é um histograma classic. Qual query dá bytes/s?
- A) `sum(http_response_size_bytes_bucket)`
- B) `rate(http_response_size_bytes_sum[5m])`
- C) `sum_over_time(http_response_size_bytes_sum[5m])`
- D) `rate(http_response_size_bytes_count[5m])`

<details><summary>Resposta</summary>

**B.** `_sum` acumula os bytes; o `rate` dá bytes por segundo. D é respostas/s. C soma valores **acumulados** de cada scrape, que não tem significado útil.
</details>

**2.** `rate(job_duration_seconds_sum[5m])` = 4. Qual interpretação?
- A) Cada job leva 4s
- B) 4 jobs por segundo
- C) Em média, 4 jobs estão em execução ao mesmo tempo
- D) A fila tem 4 jobs esperando

<details><summary>Resposta</summary>

**C.** Segundos de trabalho por segundo = concorrência média (Lei de Little). A duração média precisaria dividir pelo `rate` do `_count`.
</details>

**3.** Qual expressão retorna vazio?
- A) `histogram_sum(rate(x[5m]))` com `x` native
- B) `rate(x_sum[5m])`
- C) `histogram_sum(rate(x_sum[5m]))`
- D) `sum(rate(x_sum[5m]))`

<details><summary>Resposta</summary>

**C.** `x_sum` é float e `histogram_sum` ignora floats.
</details>

**4.** Qual é a diferença entre `sum(x)` e `histogram_sum(x)` para um native histogram `x`?
- A) Nenhuma
- B) `sum` soma histogramas de séries diferentes (resultado ainda é histograma); `histogram_sum` extrai o float "soma das observações" de cada histograma
- C) `sum` não funciona com native histograms
- D) `histogram_sum` agrega entre séries

<details><summary>Resposta</summary>

**B.** Por isso é comum combinar: `histogram_sum(sum(rate(x[5m])))`.
</details>

## 📝 Cola rápida

- `histogram_sum(rate(x[5m]))` (native) = `rate(x_sum[5m])` (classic) = unidade/s.
- Bytes → throughput. Durações → concorrência média (Lei de Little).
- Média = sum ÷ count (= `histogram_avg`).
- `sum()` ≠ `histogram_sum()` ≠ `sum_over_time()`.
- Soma é exata (não depende de buckets). Só native; em floats → vazio.

## 🔗 Relacionadas

[`histogram_count()`](../histogram_count/) · [`histogram_avg()`](../histogram_avg/) · [`histogram_quantile()`](../histogram_quantile/) · [`rate()`](../rate/) · [`increase()`](../increase/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_count-and-histogram_sum
