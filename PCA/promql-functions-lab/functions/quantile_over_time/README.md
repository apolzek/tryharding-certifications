# `quantile_over_time()`: o percentil de um gauge dentro da janela

> **Em uma frase:** `quantile_over_time(φ, v[janela])` pega **todas as amostras** de cada série dentro da janela, ordena e devolve o valor que fica na posição **φ** (0 = mínimo, 0.5 = mediana, 0.95 = p95, 1 = máximo).

| | |
|---|---|
| **Assinatura** | `quantile_over_time(φ scalar, v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (duração de probe, lag, tamanho de fila, temperatura, uso de CPU...) · ❌ Counter cru · ❌ Histogram (amostras de histograma são ignoradas) |
| **Unidade do resultado** | a **mesma** da métrica de entrada (segundos → segundos) |
| **Dashboard** | http://localhost:3300/d/fn-quantile_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a fila do banco

Imagine que, a cada 5 segundos, você anota **quanto tempo a última pessoa esperou** na fila do banco. Depois de 5 minutos você tem uma lista com 60 números.

- **Ordene** a lista do menor para o maior.
- O **p50 (mediana)** é o número do meio: "metade das vezes a espera foi menor que isso".
- O **p95** é o número na posição 95%: "em 95% das vezes a espera foi **menor** que isso; só 5% das vezes foi pior".
- O **máximo** é o pior caso absoluto, que pode ser uma única pessoa azarada.

```
amostras ordenadas:  0.15 0.16 0.17 ... 0.24 0.25 | 3.0 3.0
                     ^                   ^    ^      ^
                     p0 (min)          p50  p95     p99 / max
```

`quantile_over_time(0.95, x[5m])` faz exatamente isso: ordena as amostras da janela e devolve o p95.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) imita o `probe_duration_seconds` do **blackbox_exporter**: um gauge com a duração do **último** probe HTTP em cada endpoint.

| Métrica | Tipo | Comportamento |
|---|---|---|
| `quantile_over_time_probe_duration_seconds{target="checkout"}` | gauge | ≈ **0.20s** (±0.05) quase sempre; **1 em cada 30 probes** (um slot de 5s a cada 150s) leva **3.0s**. Ou seja, ~3% de probes lentos. |
| `quantile_over_time_probe_duration_seconds{target="catalog"}` | gauge | **bimodal (cache)**: 4 em cada 5 probes são cache **HIT** ≈ **0.10s**, 1 em cada 5 é **MISS** ≈ **1.0s** |

```bash
curl -s localhost:8088/metrics | grep '^quantile_over_time_'
# quantile_over_time_probe_duration_seconds{target="catalog"} 0.1043
# quantile_over_time_probe_duration_seconds{target="checkout"} 0.2217
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-quantile_over_time
```

Espere **~5 minutos** para a janela `[5m]` ficar cheia (60 amostras com scrape de 5s).

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
quantile_over_time_probe_duration_seconds
```

**O que faz:** mostra cada amostra.
**Resultado esperado:** `checkout` é uma linha baixa em ~0.2s com **agulhas finas de 3s** a cada 2,5 min. `catalog` pula o tempo todo entre ~0.1s e ~1.0s.

---

### 2. p50 / p95 / p99 vs max vs avg (target `checkout`)

```promql
quantile_over_time(0.5,  quantile_over_time_probe_duration_seconds{target="checkout"}[5m])
quantile_over_time(0.95, quantile_over_time_probe_duration_seconds{target="checkout"}[5m])
quantile_over_time(0.99, quantile_over_time_probe_duration_seconds{target="checkout"}[5m])
max_over_time(quantile_over_time_probe_duration_seconds{target="checkout"}[5m])
avg_over_time(quantile_over_time_probe_duration_seconds{target="checkout"}[5m])
```

**O que faz:** em `[5m]` há ~60 amostras, das quais ~2 são picos de 3s (5 min / 150s).
**Resultado esperado:**

| estatística | valor | por quê |
|---|---|---|
| `p50` | ≈ **0.20** | o meio da lista é uma amostra normal |
| `p95` | ≈ **0.245** | posição 0.95 × 59 = 56,05 → ainda é uma amostra normal (as 2 últimas posições são os picos) |
| `p99` | ≈ **2.3 a 3.0** | posição 0.99 × 59 = 58,41 → cai entre os picos (ou interpola entre a maior normal e um pico) |
| `max` | **3.0** | o pior caso |
| `avg` | ≈ **0.29** | média "puxada" pelos picos; não representa nem o normal nem o pico |

**Moral:** o p95 **ignora** eventos que acontecem em menos de 5% das amostras. Se você quer ver os picos raros, use p99 ou `max_over_time`.

> 💡 Como o Prometheus calcula: ordena as N amostras, calcula a posição `rank = φ × (N − 1)` e faz **interpolação linear** entre os dois vizinhos. É o mesmo método do operador de agregação `quantile()`.

---

### 3. Distribuição bimodal (cache HIT/MISS): a média mente, a mediana não

```promql
avg_over_time(quantile_over_time_probe_duration_seconds{target="catalog"}[5m])
quantile_over_time(0.5,  quantile_over_time_probe_duration_seconds{target="catalog"}[5m])
quantile_over_time(0.95, quantile_over_time_probe_duration_seconds{target="catalog"}[5m])
```

**Resultado esperado:**
- `avg` ≈ **0.28s** (0.8 × 0.1 + 0.2 × 1.0). **Nenhum** probe real levou 0.28s!
- `p50` ≈ **0.10s**: a experiência **típica** (cache HIT).
- `p95` ≈ **1.0s**: a **cauda** (os 20% de cache MISS).

**Analogia:** se metade da turma tirou 0 e metade tirou 10, a média 5 não descreve **ninguém**. Percentis descrevem pessoas de verdade.

---

### 4. Poucas amostras = interpolação estranha

```promql
quantile_over_time(0.95, quantile_over_time_probe_duration_seconds{target="checkout"}[1m])   # ~12 amostras
quantile_over_time(0.95, quantile_over_time_probe_duration_seconds{target="checkout"}[5m])   # ~60 amostras
```

**Resultado esperado:**
- `[5m]`: linha estável em ≈ **0.245s**.
- `[1m]`: fica em ~0.24s, mas quando um pico de 3s entra na janela, sobe para ≈ **1.5s** por 1 minuto. Por quê? Com 12 amostras, `rank = 0.95 × 11 = 10,45`: interpola 45% do caminho entre a maior amostra normal (~0.25) e o pico (3.0) → `0.25 + 0.45 × 2.75 ≈ 1.49`. Um valor que **nunca** aconteceu.

**Regra prática:** para p95 tenha pelo menos ~**20 amostras** na janela; para p99, ~**100**. Com scrape de 5s: p95 → `[2m]`+; p99 → `[10m]`+. Com o scrape padrão de 15s, multiplique por 3.

---

### 5. Tabela: resumo por target

```promql
quantile_over_time(0.5,  quantile_over_time_probe_duration_seconds[5m])
quantile_over_time(0.95, quantile_over_time_probe_duration_seconds[5m])
quantile_over_time(0.99, quantile_over_time_probe_duration_seconds[5m])
max_over_time(quantile_over_time_probe_duration_seconds[5m])
```

**Resultado esperado:**

| target | p50 | p95 | p99 | max |
|---|---|---|---|---|
| catalog | ~0.10 | ~1.02 | ~1.04 | ~1.05 |
| checkout | ~0.20 | ~0.245 | ~2.3–3.0 | 3.00 |

---

### 6. φ fora de `[0, 1]`

```promql
quantile_over_time(1.5, quantile_over_time_probe_duration_seconds{target="checkout"}[5m])   # +Inf
quantile_over_time(-1,  quantile_over_time_probe_duration_seconds{target="checkout"}[5m])   # -Inf
```

**Resultado esperado:** `+Inf` e `-Inf`. **Não dá erro**: o Prometheus só devolve um *warning* na resposta da API (`PromQL warning: quantile value should be between 0 and 1, got -1`), que quase ninguém lê. Na tabela do Grafana os dois podem aparecer como `∞`. Cuidado com variáveis de dashboard que passam `95` em vez de `0.95`.

---

### 7. 🔴 Caso real no dashboard: a regra `EndpointSlowP95`

```promql
quantile_over_time(0.95, quantile_over_time_probe_duration_seconds[5m]) > 0.5
```

**Resultado esperado:** só `catalog` aparece, com valor ≈ **1.0s** (20% dos probes são cache MISS, mais que os 5% que o p95 tolera). `checkout` **não** aparece: seus probes de 3s são só ~3% das amostras.

---

### 8. 🟡 O que dá errado: alertar com `max_over_time`

```promql
max_over_time(quantile_over_time_probe_duration_seconds[5m]) > 0.5
```

**Resultado esperado:** **os dois** targets aparecem: `catalog` ≈ 1.05 e `checkout` = **3.0**. O `checkout` vira falso positivo: um único probe lento a cada 150s bastaria para acordar o plantonista a noite toda. Compare com o painel 7: p95 separa "problema sustentado" de "evento raro".

---

## 🏭 Casos reais

### 1. SLO de latência de um endpoint externo (blackbox_exporter)

O time de SRE do e-commerce monitora a home e o checkout "de fora", com o **blackbox_exporter**. Ele expõe `probe_duration_seconds` como **gauge** (duração do último probe), e **não** um histogram. Para ter um "p95 da latência medida pelo probe", o único caminho é `quantile_over_time`:

```yaml
groups:
  - name: blackbox-latency
    rules:
      - alert: EndpointSlowP95
        expr: quantile_over_time(0.95, probe_duration_seconds{job="blackbox-http"}[10m]) > 1
        for: 10m
        labels: {severity: warning}
        annotations:
          summary: "p95 do probe em {{ $labels.instance }} acima de 1s há 10 min"
```

**Por que p95 e não `max_over_time`?** Um único probe lento (DNS demorou, retransmissão TCP) não deve acordar ninguém de madrugada. O p95 só sobe se **mais de 5%** dos probes forem lentos, ou seja, se o problema for **sustentado**.

### 2. Lag de consumidor Kafka "típico" vs pior caso

`kafka_consumergroup_lag` (kafka_exporter) é gauge. O lag oscila muito em rajadas. Dashboard com `quantile_over_time(0.5, kafka_consumergroup_lag[15m])` (lag típico) ao lado de `max_over_time(...)` (pior rajada) mostra se o consumidor está **cronicamente** atrasado (mediana alta) ou só sofre **picos** (mediana baixa, máximo alto).

### 3. Capacity planning de memória

Para dimensionar o `requests.memory` de um pod, use o p99 do uso real ao longo de dias (numa recording rule, para não ter que ler milhões de amostras toda vez):

```yaml
- record: pod:container_memory_working_set_bytes:p99_1d
  expr: quantile_over_time(0.99, container_memory_working_set_bytes{container!=""}[1d])
```

"99% do tempo o pod usa menos que X" é um alvo muito melhor que a média (que subdimensiona) e que o máximo (que superdimensiona por causa de um pico isolado).

### 4. CPU "sustentadamente" alta (sem falso positivo por agulha)

```promql
quantile_over_time(0.9, instance:node_cpu_utilisation:rate5m[30m]) > 0.8
```

"Em 90% dos últimos 30 min, a CPU esteve acima de 80%?" é mais robusto que `avg_over_time` (uma madrugada ociosa derruba a média) e que `max_over_time` (uma agulha dispara).

---

## ✅ Quando usar

- **Percentil de um gauge "amostrado"**: duração de probe, tamanho da fila, lag de replicação, temperatura.
- **Alertas robustos a picos isolados**: "o p90 da CPU nos últimos 30 min passou de 80%".
- **Capacity planning**: `quantile_over_time(0.99, ...[1d])`.
- **Mediana (`φ=0.5`)** como alternativa ao `avg_over_time` quando a distribuição é torta ou bimodal.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Percentil de **latência de requisições** (você tem histogram) | [`histogram_quantile()`](../histogram_quantile/): considera **todas** as requisições, não só 1 amostra a cada scrape |
| Métrica é **counter** | primeiro [`rate()`](../rate/) numa subquery: `quantile_over_time(0.95, rate(x[1m])[1h:])` |
| Percentil **entre séries** num instante (p95 dos pods agora) | operador de agregação `quantile(0.95, x)` |
| Quer o pior caso absoluto | [`max_over_time()`](../max_over_time/) |
| Quer só "o quão espalhado" está | [`stddev_over_time()`](../stddev_over_time/) ou [`mad_over_time()`](../mad_over_time/) |

## ⚠️ Pegadinhas

1. **É o percentil das AMOSTRAS, não dos eventos.** Um gauge de "latência da última requisição" raspado a cada 5s ignora todas as requisições entre um scrape e outro. Um p99 de verdade de requisições só vem de histogram (ou summary).
2. **Poucas amostras** → interpolação gera valores que nunca existiram (query 4).
3. **φ fora de [0,1]** → `±Inf`, sem erro, só um *warning* (query 6).
4. **O argumento φ vem PRIMEIRO**: `quantile_over_time(0.95, x[5m])`, não `quantile_over_time(x[5m], 0.95)`.
5. **Todas as amostras têm o mesmo peso**, mesmo que não estejam igualmente espaçadas (ex.: buraco de scrape).
6. **Histogramas nativos são ignorados**: só amostras float entram no cálculo.
7. **Percentis não se somam nem se tiram média**: `avg(quantile_over_time(0.95, ...))` entre pods **não** é o p95 do serviço.

---

## 🎓 Na prova PCA

O que costuma cair:
- **Ordem dos argumentos**: `φ` (scalar) primeiro, **range vector** depois. Retorna **instant vector**.
- **`quantile_over_time` vs `quantile` vs `histogram_quantile`**: tempo (uma série, várias amostras) vs espaço (várias séries, um instante) vs buckets de histogram.
- **Tipo de métrica**: é para **gauge**. Em counter, o resultado é só "algum valor do total acumulado", sem sentido.
- **φ fora de [0,1]** → `+Inf`/`-Inf`.

**1.** Você tem o gauge `probe_duration_seconds` e quer o p95 dos últimos 10 minutos. Qual query está correta?

- A) `histogram_quantile(0.95, probe_duration_seconds[10m])`
- B) `quantile_over_time(0.95, probe_duration_seconds[10m])`
- C) `quantile(0.95, probe_duration_seconds[10m])`
- D) `quantile_over_time(probe_duration_seconds[10m], 0.95)`

<details><summary>Resposta</summary>

**B.** `quantile_over_time(φ, range-vector)`. A) `histogram_quantile` exige buckets (`le`) e instant vector. C) `quantile` é operador de agregação e recebe **instant** vector. D) ordem dos argumentos invertida (erro de parse/tipo).
</details>

**2.** Qual a diferença entre `quantile(0.9, x)` e `quantile_over_time(0.9, x[5m])`?

- A) Nenhuma, são sinônimos.
- B) `quantile` calcula entre **séries** num instante; `quantile_over_time` calcula entre **amostras no tempo**, para **cada** série.
- C) `quantile` só funciona com histogram.
- D) `quantile_over_time` agrega todas as séries em uma.

<details><summary>Resposta</summary>

**B.** O operador `quantile` "achata" várias séries num instante (ex.: p90 dos pods agora). `quantile_over_time` mantém uma saída por série e olha as amostras ao longo da janela. D) está errado: `_over_time` **não** junta séries.
</details>

**3.** O que retorna `quantile_over_time(1.2, up[5m])`?

- A) Erro de sintaxe.
- B) O máximo de `up` na janela.
- C) `+Inf` para cada série.
- D) Vetor vazio.

<details><summary>Resposta</summary>

**C.** φ > 1 → `+Inf` (φ < 0 → `-Inf`). Não é erro; a API só anexa um warning.
</details>

**4.** Você quer o p99 da latência de **todas** as requisições HTTP de um serviço instrumentado com `http_request_duration_seconds` (histogram). Qual abordagem é a certa?

- A) `quantile_over_time(0.99, http_request_duration_seconds_sum[5m])`
- B) `histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))`
- C) `quantile_over_time(0.99, http_request_duration_seconds_bucket[5m])`
- D) `max_over_time(http_request_duration_seconds_count[5m])`

<details><summary>Resposta</summary>

**B.** Com histogram, use `histogram_quantile` sobre o `rate` dos buckets. `quantile_over_time` em `_sum`/`_bucket` (counters) calcula o percentil de valores acumulados, sem sentido.
</details>

**5.** Com scrape de 15s, qual janela é mais adequada para um p99 confiável via `quantile_over_time`?

- A) `[30s]`
- B) `[1m]`
- C) `[5m]`
- D) `[30m]`

<details><summary>Resposta</summary>

**D.** p99 precisa de muitas amostras (~100+). `[30m]` com scrape de 15s = 120 amostras. Com `[1m]` (4 amostras) o "p99" é praticamente uma interpolação entre as duas maiores amostras.
</details>

---

## 📝 Cola rápida

- `quantile_over_time(φ, gauge[janela])`: **φ primeiro**, range vector depois, retorna instant vector (uma saída **por série**).
- Percentil das **amostras** raspadas, não dos eventos. Latência de requisições → histogram + `histogram_quantile`.
- `quantile` (operador) = entre séries; `quantile_over_time` = ao longo do tempo; `histogram_quantile` = buckets.
- φ fora de [0,1] → `±Inf`. Poucas amostras → interpolação estranha.
- Use para alertas robustos a agulhas (p90/p95) e capacity planning (p99 em janelas longas, via recording rule).

## 🔗 Relacionadas

[`max_over_time()`](../max_over_time/) · [`min_over_time()`](../min_over_time/) · [`avg_over_time()`](../avg_over_time/) · [`stddev_over_time()`](../stddev_over_time/) · [`mad_over_time()`](../mad_over_time/) · [`histogram_quantile()`](../histogram_quantile/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
