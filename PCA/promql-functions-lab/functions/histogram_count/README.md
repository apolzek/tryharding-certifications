# `histogram_count()`: quantas observações tem o native histogram

> **Em uma frase:** `histogram_count(v)` devolve o **número de observações** guardado em cada **native histogram** de `v`. Com `rate()` em volta do histograma, vira **observações por segundo** (ex.: req/s). Séries float são **ignoradas**.

| | |
|---|---|
| **Assinatura** | `histogram_count(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ **Native** histogram · ❌ Classic (`x_count` já é o número; `histogram_count(x_count)` → vazio) |
| **Unidade do resultado** | "observações" (ou observações/s com `rate`) |
| **Equivalente classic** | `rate(x_count[5m])` |
| **Dashboard** | http://localhost:3300/d/fn-histogram_count |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a urna de votos

Numa eleição, cada eleitor coloca o voto na urna. A urna tem **divisórias** por candidato (os buckets). Na apuração, há duas perguntas diferentes:

- "**Quem** ganhou em cada faixa?" → isso é olhar os buckets (quantis, frações).
- "**Quantas pessoas votaram?**" → isso é o `histogram_count`. Ele ignora **em quem** votaram e só conta os votos.

Num histograma de latência, cada **requisição** é um voto. `histogram_count` = quantas requisições. `histogram_count(rate(...))` = quantas requisições **por segundo**. Ou seja: o histograma de latência **já é** o seu contador de tráfego, você não precisa de um `requests_total` separado.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_count_http_request_duration_seconds{route="/home"}` | histogram (native + classic) | tráfego em **onda**: **20 ± 10 req/s**, período de **5 min**. Latência ~60ms |
| `histogram_count_http_request_duration_seconds{route="/login"}` | histogram (native + classic) | **~5 req/s**, com **rajada de 60s a ~25 req/s a cada 4 min** (força bruta). Na rajada as requisições ficam lentas (~600ms) |

Buckets classic: `0.05, 0.1, 0.25, 0.5, 1, 2.5, +Inf`.

```bash
curl -s localhost:8088/metrics | grep '^histogram_count_http_request_duration_seconds_count'
# histogram_count_http_request_duration_seconds_count{route="/home"} 12040
# histogram_count_http_request_duration_seconds_count{route="/login"} 3010
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_count
```

Espere **~2 min** para `[1m]` e **~5 min** para o painel de `[5m]` e para ver a onda completa.

---

## 🔍 Queries passo a passo

### 1. `histogram_count(x)` cru: um counter disfarçado

```promql
histogram_count(histogram_count_http_request_duration_seconds)
```

**O que faz:** lê o número total de observações desde que o processo subiu.
**Resultado esperado:** duas linhas **sempre subindo** (e voltando a zero se o gerador reiniciar). Assim como um counter cru, não é muito útil sozinho.

---

### 2. `histogram_count(rate(x[1m]))`: requisições por segundo

```promql
histogram_count(rate(histogram_count_http_request_duration_seconds[1m]))
```

**O que faz:** o `rate()` age no histograma inteiro (cada bucket, o sum e o count), devolvendo um histograma "por segundo". O `histogram_count` extrai o count dele.
**Resultado esperado** (req/s):

| route | valor |
|---|---|
| `/home` | onda entre ≈ **10** e ≈ **30** (período 5 min) |
| `/login` | ≈ **5**, com platô em ≈ **25** por ~60s a cada 4 min |

> 💡 A ordem importa: **primeiro `rate`, depois `histogram_count`**. `rate(histogram_count(x)[1m])` nem é válido (seria preciso subquery) e perderia a compensação de resets.

---

### 3. Native vs classic: são o mesmo número

```promql
histogram_count(rate(histogram_count_http_request_duration_seconds{route="/login"}[1m]))   # native
rate(histogram_count_http_request_duration_seconds_count{route="/login"}[1m])              # classic
```

**Resultado esperado:** **duas linhas sobrepostas**. O `_count` do classic é o count do native exposto como série separada.

---

### 4. Quantas requisições em 5 minutos?

```promql
histogram_count(increase(histogram_count_http_request_duration_seconds[5m]))
```

**O que faz:** `increase` no histograma = "o histograma de tudo que aconteceu nos últimos 5 min". O count dele é o total de requisições.
**Resultado esperado:**
- `/home` ≈ **6000** (média de 20 req/s × 300s; a onda se cancela numa janela de um período inteiro).
- `/login` entre ≈ **2700** e ≈ **3900**: 5 req/s × 300s = 1500, mais 1200 por rajada. Como as rajadas vêm a cada 4 min, a janela de 5 min pega entre 1 e 2 rajadas.

---

### 5. Requisições **lentas** por segundo

```promql
# native: total × fração acima de 500ms
histogram_count(rate(histogram_count_http_request_duration_seconds{route="/login"}[1m]))
  * histogram_fraction(0.5, +Inf, rate(histogram_count_http_request_duration_seconds{route="/login"}[1m]))

# classic: total − (quem ficou até 0.5s)
rate(histogram_count_http_request_duration_seconds_count{route="/login"}[1m])
  - ignoring(le) rate(histogram_count_http_request_duration_seconds_bucket{route="/login", le="0.5"}[1m])
```

**Resultado esperado:** ≈ **0 req/s** fora da rajada (login normal leva ~80ms). Durante a rajada, ≈ **18 req/s** lentas (≈ 73% de 25 req/s passam de 500ms).
**Moral:** `histogram_count` combina muito bem com [`histogram_fraction()`](../histogram_fraction/) para transformar "%" em "quantidade".

---

### 6. Total do serviço, e o que acontece em classic

```promql
histogram_count(sum(rate(histogram_count_http_request_duration_seconds[1m])))   # ✅ total
histogram_count(rate(histogram_count_http_request_duration_seconds_count[1m]))  # ❌ vazio
```

**Resultado esperado:** o total fica entre ≈ **25** e ≈ **55 req/s** (onda do /home + 5 ou 25 do /login). A segunda query dá **vazio**: `x_count` é float, e `histogram_count` ignora floats.
Tanto faz `histogram_count(sum(...))` ou `sum(histogram_count(...))` aqui: contar e somar comutam.

---

## 🏭 Casos reais

> O cenário fake imita os casos 1 e 2: `histogram_count_http_request_duration_seconds{route}` com tráfego em onda no `/home` e uma rajada de força bruta no `/login`.

### Caso 1: o histograma de latência É o seu contador de tráfego

Muitos serviços (OpenTelemetry, Micrometer) **não** expõem `http_requests_total`: só o histograma de duração. O "R" e o "E" do método RED saem dele:

```promql
# Rate (req/s) por rota
sum by (http_route) (histogram_count(rate(http_server_request_duration_seconds[5m])))

# Errors (fração de 5xx)
  sum(histogram_count(rate(http_server_request_duration_seconds{http_response_status_code=~"5.."}[5m])))
/ sum(histogram_count(rate(http_server_request_duration_seconds[5m])))
```

Em classic, o mesmo com `rate(http_server_request_duration_seconds_count[5m])`.

Recording rules RED usando só o histograma:

```yaml
- record: job_route:http_server_requests:rate5m
  expr: sum by (job, http_route) (histogram_count(rate(http_server_request_duration_seconds[5m])))
- record: job:http_server_requests_errors:ratio5m
  expr: |2
      sum by (job) (histogram_count(rate(http_server_request_duration_seconds{http_response_status_code=~"5.."}[5m])))
    / sum by (job) (histogram_count(rate(http_server_request_duration_seconds[5m])))
- alert: HighErrorRatio
  expr: job:http_server_requests_errors:ratio5m > 0.05
  for: 10m
```

### Caso 2: força bruta no login

```yaml
- alert: LoginBruteForce
  expr: histogram_count(rate(http_server_request_duration_seconds{http_route="/login"}[2m])) > 20
  for: 1m
  labels: { severity: warning }
  annotations:
    summary: "{{ $value | humanize }} logins/s (normal: ~5/s)"
```

É o painel 2: platô de ~25 req/s a cada 4 min.

### Caso 3: throughput do API server

```promql
sum by (verb) (rate(apiserver_request_duration_seconds_count{job="apiserver"}[5m]))
```

O `_count` do histograma de latência é usado como taxa de requisições por verbo (GET, LIST, WATCH...) em vários dashboards do kube-prometheus. Com native seria `histogram_count(rate(apiserver_request_duration_seconds[5m]))`.

### Caso 4: "quantas requisições violaram o SLO hoje?"

```promql
histogram_count(increase(http_server_request_duration_seconds{job="checkout"}[1d]))
  * histogram_fraction(0.3, +Inf, increase(http_server_request_duration_seconds{job="checkout"}[1d]))
```

Transforma "0.4% lentas" em "1.730 requisições lentas hoje", número que o time de negócio entende.

---

## ✅ Quando usar

- **Throughput (req/s)** a partir de um histograma de latência: `histogram_count(rate(http_request_duration_seconds[5m]))`.
- **Taxa de erro**: `sum(histogram_count(rate(x{code=~"5.."}[5m]))) / sum(histogram_count(rate(x[5m])))`.
- **Quantidade de eventos numa faixa**: count × [`histogram_fraction()`](../histogram_fraction/) (painel 5).
- **Denominador de médias**: `histogram_sum / histogram_count` (é o que [`histogram_avg()`](../histogram_avg/) faz).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| A métrica é **classic** | `rate(x_count[5m])` |
| Quer a **soma** dos valores (bytes, segundos) | [`histogram_sum()`](../histogram_sum/) |
| Quer o número de **séries** (não de observações) | [`count()`](../count/) (agregador) |
| Quer contar amostras de uma série ao longo do tempo | [`count_over_time()`](../count_over_time/) |

## ⚠️ Pegadinhas

1. **Só native**: `histogram_count(x_count)` ou `histogram_count(x_bucket)` dão vazio, sem erro.
2. **Sem `rate`/`increase`** é um total acumulado desde o start (painel 1).
3. **Confundir com `count()`**: `count(x)` conta **séries**; `histogram_count(x)` conta **observações dentro** de cada histograma.
4. **Resultado fracionado** (ex.: 4.27 req/s): `rate` extrapola e faz média na janela. Normal.
5. **`histogram_count(increase(...))` não é inteiro**: mesma extrapolação do [`increase()`](../increase/).

## 🎓 Na prova PCA

O que costuma cair:
- O `_count` de um histograma/summary classic é um **counter** com o número de observações → `rate(x_count[5m])` = eventos/s.
- `x_bucket{le="+Inf"}` tem o **mesmo valor** que `x_count`.
- `histogram_count` é o equivalente para **native**; em floats → vazio.
- Diferença entre **`count()`** (conta séries), **`count_over_time()`** (conta amostras no tempo) e **`histogram_count()`** (conta observações dentro do histograma).

**1.** Qual query dá requisições por segundo a partir de um histograma classic `http_request_duration_seconds`?
- A) `count(http_request_duration_seconds_bucket)`
- B) `rate(http_request_duration_seconds_count[5m])`
- C) `rate(http_request_duration_seconds_sum[5m])`
- D) `count_over_time(http_request_duration_seconds_count[5m])`

<details><summary>Resposta</summary>

**B.** `_count` é o counter de observações. A conta séries (buckets × labels). C é a soma das durações por segundo. D conta **amostras raspadas** (≈ janela ÷ scrape interval), não requisições.
</details>

**2.** Num histograma classic, qual série é sempre igual a `x_count`?
- A) `x_sum`
- B) `x_bucket{le="1"}`
- C) `x_bucket{le="+Inf"}`
- D) Nenhuma

<details><summary>Resposta</summary>

**C.** Os buckets são cumulativos; o último (`+Inf`) contém todas as observações.
</details>

**3.** `histogram_count(rate(x_count[5m]))` retorna...
- A) req/s
- B) vazio
- C) o número de buckets
- D) erro de parse

<details><summary>Resposta</summary>

**B.** `x_count` é float; `histogram_count` ignora floats. Use `rate(x_count[5m])` direto, ou `histogram_count(rate(x[5m]))` no native.
</details>

**4.** Com scrape a cada 15s, `count_over_time(x_count[5m])` retorna aproximadamente...
- A) O número de requisições em 5 min
- B) 20
- C) 300
- D) req/s

<details><summary>Resposta</summary>

**B.** 300s ÷ 15s = 20 amostras. `count_over_time` conta amostras, não observações. Pegadinha clássica.
</details>

## 📝 Cola rápida

- `histogram_count(rate(x[5m]))` (native) = `rate(x_count[5m])` (classic) = eventos por segundo.
- `histogram_count(increase(x[1h]))` = eventos na última hora.
- `_bucket{le="+Inf"}` == `_count`.
- `count()` = séries · `count_over_time()` = amostras · `histogram_count()` = observações.
- count × `histogram_fraction` = quantos eventos numa faixa.

## 🔗 Relacionadas

[`histogram_sum()`](../histogram_sum/) · [`histogram_avg()`](../histogram_avg/) · [`histogram_fraction()`](../histogram_fraction/) · [`rate()`](../rate/) · [`increase()`](../increase/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_count-and-histogram_sum
