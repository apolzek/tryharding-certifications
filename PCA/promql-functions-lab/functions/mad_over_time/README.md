# `mad_over_time()`: dispersão robusta a outliers (experimental)

> **Em uma frase:** `mad_over_time(v[janela])` calcula o **MAD** (*Median Absolute Deviation*, desvio absoluto mediano) das amostras de cada série: a **mediana** das distâncias de cada amostra até a **mediana**. É o "desvio padrão que não se assusta com outliers".

| | |
|---|---|
| **Assinatura** | `mad_over_time(v range-vector) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Gauge · ❌ Counter cru · ❌ Histogram (amostras de histograma são ignoradas) |
| **Unidade do resultado** | a **mesma** da métrica (s → s) |
| **Dashboard** | http://localhost:3300/d/fn-mad_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o bilionário na sala

11 pessoas numa sala ganham entre R$ 4.500 e R$ 5.500. Entra um bilionário.

| | antes | depois |
|---|---|---|
| média | R$ 5.000 | **R$ 83 milhões** |
| desvio padrão | ~R$ 300 | **~R$ 276 milhões** |
| **mediana** | R$ 5.000 | R$ 5.050 |
| **MAD** | ~R$ 250 | ~R$ 260 |

Média e desvio padrão "viram" o bilionário. Mediana e MAD continuam descrevendo **a sala**.

Como calcular o MAD, na mão:

```
amostras:            92  95  98  100  101  104  1000
mediana:                         100
|xᵢ − mediana|:       8   5   2    0    1    4   900
ordenado:             0   1   2    4    5    8   900
MAD = mediana disso:            4               ← o 900 não mudou nada
```

Regra de bolso: para dados "em sino", `σ ≈ 1,4826 × MAD`. Por isso o **z-score robusto** é:

```
z_robusto = (x − mediana) / (1,4826 × MAD)
```

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita o `probe_duration_seconds` do **blackbox_exporter** em duas APIs idênticas, sendo que uma roda numa JVM que sofre **pausas de GC "stop-the-world"**:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `mad_over_time_probe_duration_seconds{target="api-go"}` | gauge | **0.100s ± 0.010** (ruído uniforme) |
| `mad_over_time_probe_duration_seconds{target="api-java"}` | gauge | igual ao api-go, mas **1 em cada 12 probes** (1 por minuto) pega uma pausa de GC e leva **1.0s** (~8% de outliers) |

```bash
curl -s localhost:8088/metrics | grep '^mad_over_time_'
# mad_over_time_probe_duration_seconds{target="api-go"} 0.0964
# mad_over_time_probe_duration_seconds{target="api-java"} 0.1079
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-mad_over_time
```

Espere **~5 min** para a janela `[5m]` encher.

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
mad_over_time_probe_duration_seconds
```

**Resultado esperado:** duas faixas em 90..110ms; `api-java` tem uma **agulha de 1s por minuto**.

---

### 2. MAD: quase igual nos dois targets

```promql
mad_over_time(mad_over_time_probe_duration_seconds[5m])
```

**O que faz:** mediana(|xᵢ − mediana(x)|) em ~60 amostras.
**Resultado esperado:**

| target | MAD |
|---|---|
| `api-go` | ≈ **5ms** (ruído uniforme ±10ms → as distâncias até a mediana vão de 0 a 10ms; a mediana delas é ~5ms) |
| `api-java` | ≈ **5 a 6,5ms**, praticamente o mesmo do `api-go`: os 5 outliers da janela mudam o MAD em no máximo **~1ms** |

---

### 3. Desvio padrão: explode

```promql
stddev_over_time(mad_over_time_probe_duration_seconds[5m])
```

**Resultado esperado:**

| target | stddev |
|---|---|
| `api-go` | ≈ **5,8ms** (10/√3) |
| `api-java` | ≈ **240 a 260ms**: ~8% de amostras a 900ms de distância, elevadas ao quadrado, dominam tudo |

Mesmo dado "normal", resposta **40× diferente**. O MAD vê o comportamento **típico**; o desvio padrão vê a **cauda**.

---

### 4. Z-score clássico vs robusto (`api-java`)

```promql
# clássico
(mad_over_time_probe_duration_seconds{target="api-java"} - avg_over_time(mad_over_time_probe_duration_seconds{target="api-java"}[5m]))
  / stddev_over_time(mad_over_time_probe_duration_seconds{target="api-java"}[5m])

# robusto
(mad_over_time_probe_duration_seconds{target="api-java"} - quantile_over_time(0.5, mad_over_time_probe_duration_seconds{target="api-java"}[5m]))
  / (1.4826 * mad_over_time(mad_over_time_probe_duration_seconds{target="api-java"}[5m]))
```

**Resultado esperado:**
- **Clássico:** nas agulhas, z ≈ **+3,2** ((1.0 − 0.175) / 0.25). Pontos normais ficam em ≈ **−0,3**. Os outliers **se escondem** porque inflam a média e o σ usados para julgá-los (efeito *masking*).
- **Robusto:** pontos normais entre **−1,5 e +1,5**; as agulhas dão z ≈ **+85 a +165** ((1.0 − 0.100) / (1,4826 × 0.005) ≈ 121; varia conforme o MAD da janela). Impossível não ver.

---

### 5. Tabela: resumo por target

| target | mediana | MAD | avg | stddev |
|---|---|---|---|---|
| api-go | ~100ms | ~5ms | ~100ms | ~5,8ms |
| api-java | ~100ms | ~5–6ms | ~175 a 180ms | ~250ms |

---

### 6. 🔴 Caso real no dashboard: alerta de outlier robusto

```promql
abs(
  (mad_over_time_probe_duration_seconds - quantile_over_time(0.5, mad_over_time_probe_duration_seconds[5m]))
    / (1.4826 * mad_over_time(mad_over_time_probe_duration_seconds[5m]) > 0)
) > 5
```

**Resultado esperado:** pontos só em `api-java`, **1 por minuto** (cada pausa de GC), com valor ≈ **85 a 165**. `api-go` nunca aparece (seu |z| máximo é ~1,5).

---

### 7. 🟡 O que dá errado: alerta de jitter com `stddev_over_time`

```promql
stddev_over_time(mad_over_time_probe_duration_seconds[5m]) > 0.05   # "jitter > 50ms" com stddev
mad_over_time(mad_over_time_probe_duration_seconds[5m]) > 0.05      # "jitter > 50ms" com MAD
```

**Resultado esperado:**
- Com **stddev**: `api-java` fica **permanentemente** em alerta (≈ **0.25s**), porque sempre há ~5 pausas de GC na janela. O time silencia o alerta e perde o sinal de verdade.
- Com **MAD**: **vazio** (≈ 5–6ms < 50ms). O jitter típico está ótimo; as pausas de GC merecem um alerta **próprio**, não contaminar este.

---

## 🏭 Casos reais

### 1. Pausas de GC escondendo a latência "real" (JVM)

O time de uma API Java recebe alerta de "latência instável" toda hora, porque o `stddev_over_time` do probe explode a cada Full GC. Mas o que eles querem saber é: **o comportamento típico degradou?** Trocando por MAD:

```yaml
- alert: ProbeLatencyTypicalDegraded
  expr: |
    quantile_over_time(0.5, probe_duration_seconds{job="blackbox-http"}[30m])
      > 2 * quantile_over_time(0.5, probe_duration_seconds{job="blackbox-http"}[1d] offset 1d)
  for: 15m
- alert: ProbeLatencyJitterHigh
  expr: mad_over_time(probe_duration_seconds{job="blackbox-http"}[30m]) > 0.05
  for: 15m
```

As pausas de GC são tratadas **à parte** (ex.: `max_over_time(jvm_gc_pause_seconds_max[5m])`), sem poluir o sinal de jitter.

### 2. Detector de outliers robusto (z-score com MAD)

Para achar **pontos** anômalos numa métrica que já tem sujeira (`node_network_receive_errs_total` via rate, lag de réplica, tamanho de fila):

```yaml
- alert: QueueDepthOutlier
  expr: |
    (rabbitmq_queue_messages - quantile_over_time(0.5, rabbitmq_queue_messages[1h]))
      / (1.4826 * mad_over_time(rabbitmq_queue_messages[1h]) > 0)
    > 5
  for: 5m
```

O `> 0` no divisor evita `+Inf` quando a fila ficou parada (MAD = 0).

### 3. Comparar a "estabilidade típica" de réplicas

`mad_over_time(pg_replication_lag_seconds[1h])` por réplica: a réplica com MAD alto oscila **o tempo todo**; a que só tem stddev alto teve **um** evento isolado (backup, vacuum).

---

## ✅ Quando usar

- **Detecção de anomalia em dados sujos**: latências com picos de GC, filas com rajadas, sensores com leituras espúrias.
- **Linha de base robusta** para alertas (mediana + k × MAD).
- **Medir a dispersão "típica"**, ignorando eventos raros.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Você **quer** que os eventos raros pesem (ex.: pior caso) | [`stddev_over_time()`](../stddev_over_time/), [`max_over_time()`](../max_over_time/) |
| Precisa somar dispersões de fontes independentes | [`stdvar_over_time()`](../stdvar_over_time/) |
| Quer "95% do tempo fica abaixo de X" | [`quantile_over_time()`](../quantile_over_time/) |
| Ambiente sem a feature flag experimental | aproximação: `(quantile_over_time(0.75, x[5m]) - quantile_over_time(0.25, x[5m])) / 2` (meio IQR ≈ MAD para distribuições simétricas) |

## ⚠️ Pegadinhas

1. **Experimental**: sem `--enable-feature=promql-experimental-functions` a query dá erro de função desconhecida. O nome/comportamento pode mudar entre versões.
2. **MAD ≠ σ**: para comparar com o desvio padrão, multiplique por **1,4826** (vale para distribuição normal).
3. **MAD = 0** quando mais da metade das amostras é **igual** (ex.: gauge que fica parado): o z-score robusto vira `±Inf`/`NaN`. Proteja com `(... > 0)`.
4. **Robusto até ~50% de outliers**: se mais da metade das amostras forem "estranhas", elas passam a ser o "normal".
5. **Histogramas nativos são ignorados** (só amostras float).

---

## 🎓 Na prova PCA

A PCA foca em funções estáveis; `mad_over_time` é **experimental** e pode aparecer como "qual funciona sem feature flag?". O que vale saber:
- Funções experimentais exigem `--enable-feature=promql-experimental-functions`.
- Range vector → instant vector, por série, igual às outras `_over_time`.
- Conceito: estatística **robusta** (mediana, MAD) vs sensível a outliers (média, desvio padrão).

**1.** Qual destas funções **exige** `--enable-feature=promql-experimental-functions` no Prometheus 3.x?

- A) `stddev_over_time`
- B) `quantile_over_time`
- C) `mad_over_time`
- D) `present_over_time`

<details><summary>Resposta</summary>

**C.** `mad_over_time` (e as `ts_of_*_over_time`) são experimentais. As demais são estáveis.
</details>

**2.** Numa janela, 90% das amostras estão em 100±5 e 10% valem 10000. Qual medida de dispersão fica **mais próxima** de 5?

- A) `stddev_over_time`
- B) `stdvar_over_time`
- C) `mad_over_time`
- D) `max_over_time - min_over_time`

<details><summary>Resposta</summary>

**C.** MAD usa medianas e ignora até ~50% de outliers. Desvio padrão e variância são dominados pelos 10000; a amplitude (D) dá ~9900.
</details>

**3.** Para transformar o MAD numa estimativa do desvio padrão (dados normais), multiplica-se por:

- A) 0,6745
- B) 1
- C) 1,4826
- D) 2

<details><summary>Resposta</summary>

**C.** σ ≈ 1,4826 × MAD (1,4826 = 1/0,6745, onde 0,6745 é o quantil 75% da normal padrão).
</details>

**4.** O que acontece com `mad_over_time(x[5m])` se `x` for um **counter** crescente?

- A) Dá erro de tipo.
- B) Calcula a dispersão dos valores acumulados, um número sem significado operacional.
- C) Aplica `rate` automaticamente.
- D) Retorna vazio.

<details><summary>Resposta</summary>

**B.** PromQL não verifica tipo de métrica. Para counters, use `rate` antes (subquery ou recording rule).
</details>

---

## 📝 Cola rápida

- `mad_over_time(x[w])` = mediana de |xᵢ − mediana(x)|; **experimental** (flag `promql-experimental-functions`).
- Robusto a outliers (até ~50%); `σ ≈ 1,4826 × MAD`.
- z-score robusto: `(x − quantile_over_time(0.5, x[w])) / (1.4826 * mad_over_time(x[w]))`.
- MAD = 0 quando a maioria das amostras é igual → proteja a divisão com `> 0`.
- Quer que picos pesem? Então **não** use MAD: `stddev_over_time`/`max_over_time`.

## 🔗 Relacionadas

[`stddev_over_time()`](../stddev_over_time/) · [`stdvar_over_time()`](../stdvar_over_time/) · [`quantile_over_time()`](../quantile_over_time/) · [`avg_over_time()`](../avg_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
