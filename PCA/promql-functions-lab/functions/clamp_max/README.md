# `clamp_max()`: um teto (nada fica acima do máximo)

> **Em uma frase:** `clamp_max(v, max)` troca todo valor **maior que `max`** por `max` e deixa o resto intacto. Serve para **cortar outliers** que achatam o gráfico e para **limitar porcentagens a 100%** em gauges. Mas atenção: ele corta **informação** também.

| | |
|---|---|
| **Assinatura** | `clamp_max(v instant-vector, max scalar) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (latências, razões de uso) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-clamp_max |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o teto baixo do porão

No porão, você pode andar e pular à vontade, mas **nada passa da altura do teto**. Um balão de gás sobe até encostar no teto e para ali.

```
  valor:    0.25   0.3    1.9    2     30     30
                                 │─────┴──────┴── teto em 2
  clamp_max(v, 2):
            0.25   0.3    1.9    2      2      2
```

O efeito colateral: quem olha o porão **não sabe** se o balão queria subir 3 metros ou 300. `30 s` e `2.1 s` viram a mesma coisa.

Regras especiais **da documentação**:
- `max = NaN` → resultado **NaN**;
- `max = +Inf` → valores **inalterados**;
- `max = -Inf` → **todos** os valores viram `-Inf`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `clamp_max_http_request_duration_seconds_p99{service="checkout"}` | recording rule de p99 | gauge | **0.2 a 0.3 s**, com **picos de 30 s por 20 s a cada 2 min** (timeout do gateway de pagamento) |
| `clamp_max_container_cpu_usage_seconds_total{pod="checkout-6f7c"}` | `container_cpu_usage_seconds_total` (cAdvisor) | counter | taxa oscila entre **0.2 e 1.2 cores** (4 min) |
| `clamp_max_kube_pod_container_resource_requests{pod="checkout-6f7c",resource="cpu"}` | `kube_pod_container_resource_requests` | gauge | **0.5** core |

```bash
curl -s localhost:8088/metrics | grep '^clamp_max_'
# clamp_max_http_request_duration_seconds_p99{service="checkout"} 30
# clamp_max_kube_pod_container_resource_requests{pod="checkout-6f7c",resource="cpu"} 0.5
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-clamp_max
```

Espere **~4 minutos** (dois picos de latência e uma volta completa da CPU).

---

## 🔍 Queries passo a passo

### 🟢 1 e 2. Outlier que achata o gráfico

```promql
clamp_max_http_request_duration_seconds_p99
clamp_max(clamp_max_http_request_duration_seconds_p99, 2)
```

**Resultado esperado:**
- **Cru:** o eixo vai até **30 s**. Os picos são "torres" de 20 s de largura a cada 2 min; o resto (0.2 a 0.3 s) é uma **linha colada no zero**, onde uma piora de 0.25 → 0.5 s seria invisível.
- **`clamp_max(…, 2)`:** o eixo vai até **2 s**. Os picos viram **platôs em 2** e a ondinha de **0.2 a 0.3 s** fica visível.

| cru | `clamp_max(x, 2)` |
|---|---|
| 0.25 | **0.25** |
| 1.9 | **1.9** |
| 30 | **2** |

---

### 🟡 3. O que dá errado: médias com o valor cortado

```promql
avg_over_time(clamp_max_http_request_duration_seconds_p99[4m])                     # cru
avg_over_time(clamp_max(clamp_max_http_request_duration_seconds_p99, 2)[4m:5s])    # cortado
```

**O que faz:** média de 4 min (dois ciclos de 2 min). A segunda precisa de **subquery** porque `clamp_max(...)` é uma expressão.
**Resultado esperado:**
- **Cru:** ≈ **5.2 s** = `(20 s × 30 + 100 s × 0.25) / 120 s`.
- **Cortado:** ≈ **0.54 s** = `(20 s × 2 + 100 s × 0.25) / 120 s`.

O `clamp_max` fez os usuários "esperarem" **10× menos** do que realmente esperaram. **Regra:** `clamp_max` é para **visualização**; SLOs, médias, alertas e recording rules usam o **cru**.

---

### 🔴 4, 5 e 6. Uso de CPU em relação ao request

```promql
sum by (pod) (rate(clamp_max_container_cpu_usage_seconds_total[1m]))                   # uso (cores)
sum by (pod) (clamp_max_kube_pod_container_resource_requests{resource="cpu"})            # request (0.5)

sum by (pod) (rate(...[1m])) / sum by (pod) (...requests{resource="cpu"})                # cru
clamp_max(sum by (pod) (rate(...[1m])) / sum by (pod) (...requests{resource="cpu"}), 1)  # 0..100%
```

**O que faz:** divide o uso pelo request do mesmo pod (os dois lados agregados `by (pod)` para os labels casarem). No Kubernetes, o request **não é um limite**: o pod pode usar mais do que pediu se o nó tiver folga.
**Resultado esperado:**

| uso (cores) | uso / request (cru) | `clamp_max(…, 1)` |
|---|---|---|
| 0.2 | **40%** | **40%** |
| 0.5 | **100%** | **100%** |
| 0.9 | **180%** | **100%** |
| 1.2 | **240%** | **100%** |

O gauge do painel 6 fica sempre entre 0% e 100% ("quanto do request está em uso"). Mas o painel 5 mostra a informação que o gauge esconde: o pod passa boa parte do tempo usando **mais que o dobro** do que pediu, o que é um problema de **dimensionamento** (o scheduler acha que o pod precisa de 0.5 core).

---

### ⚠️ 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `clamp_max(vector(30), 2)` | **2** | acima do teto |
| `clamp_max(vector(0.25), 2)` | **0.25** | abaixo: intacto |
| `clamp_max(vector(-5), 0)` | **-5** | negativos continuam (é só teto) |
| `clamp_max(vector(5), +Inf)` | **5** | doc: `+Inf` → inalterado |
| `clamp_max(vector(5), -Inf)` | **-Inf** | doc: `-Inf` → tudo vira -Inf |
| `clamp_max(vector(5), NaN)` | **NaN** | doc: `max` NaN → NaN |
| `clamp_max(vector(NaN), 2)` | **NaN** | NaN na entrada continua NaN |

---

## 🏭 Casos reais

### 1. Painel de latência legível durante incidentes de timeout

O time do checkout tem um painel de p99 que, quando o gateway de pagamento dá timeout (30 s), fica ilegível. Solução: **dois painéis** (ou duas queries), um cortado para o dia a dia e um cru para o incidente:

```promql
clamp_max(histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{service="checkout"}[5m]))), 2)
```

E o alerta **sempre** no cru:

```yaml
- alert: CheckoutLatenciaAlta
  expr: |
    histogram_quantile(0.99,
      sum by (le) (rate(http_request_duration_seconds_bucket{service="checkout"}[5m]))) > 1
  for: 10m
  labels:
    severity: page
```

**Decisão:** alternativa sem mudar a query: em Grafana, *Axis → Soft max* ou *Max* = 2. O `clamp_max` na query é útil quando o valor vai para **outra conta** (heatmap, `topk`, etc.).

### 2. Utilização de CPU em relação ao request (gauge 0..100%)

```yaml
groups:
  - name: rightsizing
    rules:
      - record: pod:cpu_request_utilisation:ratio
        expr: |
          sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!=""}[5m]))
          / sum by (namespace, pod) (kube_pod_container_resource_requests{resource="cpu"})

      - record: pod:cpu_request_utilisation:ratio_capped
        expr: clamp_max(pod:cpu_request_utilisation:ratio, 1)

      - alert: PodUsandoMuitoAcimaDoRequest
        expr: pod:cpu_request_utilisation:ratio > 2
        for: 1h
        labels:
          severity: info
        annotations:
          summary: "{{ $labels.pod }} usa {{ $value | humanizePercentage }} do request de CPU"
```

**Decisão:** o gauge usa a versão `capped`; o alerta de right-sizing usa a **crua**, porque "240% do request" é exatamente a informação que o `clamp_max` jogaria fora.

### 3. Limitar a contribuição de um único alvo num agregado

```promql
sum(clamp_max(rate(http_requests_total{job="crawler"}[5m]), 100))
```

Um alvo com bug (contador "disparando" após um reset mal detectado) pode dominar uma soma. Limitar cada série a um máximo plausível antes do `sum` torna o agregado robusto. Documente o limite!

### 4. Escalas fixas em heatmaps e bargauges

```promql
clamp_max(node_load1 / count without (cpu, mode) (node_cpu_seconds_total{mode="idle"}), 2)
```

Load por CPU acima de 2 é "vermelho" de qualquer forma; cortar em 2 mantém a escala de cores útil para o resto da frota.

---

## ✅ Quando usar

- **Visualização** com outliers extremos (timeouts, contadores com bug).
- **Gauges/barras 0..100%** quando a razão pode passar de 1 (uso/request, progresso estimado).
- **Robustez em agregados**: limitar a contribuição de cada série antes de `sum`/`avg`.
- **Antes de funções** que explodem com valores grandes (ex.: `exp`, que dá `+Inf` acima de ~709).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| **SLOs, médias, alertas** de latência | o valor **cru** (painel 3 mostra o erro) |
| Quer **descartar** outliers (não achatar) | filtro `x < 2`, ou [`quantile_over_time()`](../quantile_over_time/) |
| Precisa de teto **e** piso | [`clamp()`](../clamp/) |
| Só quer limitar o eixo do gráfico | *Axis → Max / Soft max* no Grafana |
| Quer zerar negativos | [`clamp_min(x, 0)`](../clamp_min/) |

## ⚠️ Pegadinhas

1. **Médias subestimadas:** `avg_over_time(clamp_max(x, 2)[…])` ignora quanto os picos passaram do teto (≈0.54 s vs ≈5.2 s no lab).
2. **"100%" esconde "240%"**: num gauge de utilização, o `clamp_max(…, 1)` apaga o sinal de subdimensionamento.
3. **`max` é escalar:** `clamp_max(x, kube_pod_container_resource_limits)` é erro de tipo. Para limitar por série, use `x < y or y` (para cada série, o menor).
4. **`clamp_max(x, -Inf)` = tudo `-Inf`** (caso da doc, geralmente um typo).
5. **Não confunda com o agregador `max`:** `max(x)` pega o maior valor **entre séries**; `clamp_max(x, 2)` limita **cada** série.
6. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- Assinatura `clamp_max(v instant-vector, max scalar)`.
- Casos da doc: `max` NaN → NaN; `+Inf` → inalterado; `-Inf` → tudo `-Inf`.
- Diferença entre `clamp_max` (função, por série) e `max` (agregador entre séries) e `max_over_time` (no tempo).
- Subquery para aplicar `_over_time` numa expressão.

**1.** Qual a diferença entre `clamp_max(x, 100)` e `max(x)`?

- A) Nenhuma
- B) `clamp_max` limita **cada série** a 100; `max` retorna o **maior valor entre as séries**
- C) `max` limita cada série a 100; `clamp_max` retorna o maior valor
- D) `clamp_max` só funciona com counters

<details><summary>Resposta</summary>

**B.** `clamp_max` é função elemento a elemento (mantém todas as séries e labels, exceto o nome); `max` é agregador (reduz a uma série por grupo).
</details>

**2.** Quanto vale `clamp_max(vector(7), -Inf)`?

- A) 7
- B) -Inf
- C) NaN
- D) vazio

<details><summary>Resposta</summary>

**B.** Pela documentação, se `max` é `-Inf`, todos os valores viram `-Inf`.
</details>

**3.** Um p99 tem picos de 30 s e você quer um SLO de "p99 médio < 1 s na última hora". Qual expressão é adequada?

- A) `avg_over_time(clamp_max(p99, 1)[1h:1m]) < 1`
- B) `avg_over_time(p99[1h]) < 1`
- C) `avg_over_time(clamp_max(p99, 5)[1h:1m]) < 1`
- D) `max(clamp_max(p99, 1)) < 1`

<details><summary>Resposta</summary>

**B.** A) e D) cortam tudo em 1, então a média nunca passa de 1: o SLO só falharia se **todas** as amostras estivessem no teto. C) subestima a média (um pico de 30 s conta como 5 s) e pode aprovar uma hora que violou o SLO. SLOs usam o valor cru.
</details>

**4.** Qual expressão é **inválida**?

- A) `clamp_max(node_load1, 4)`
- B) `clamp_max(rate(http_requests_total[5m]), 1000)`
- C) `clamp_max(node_load1, node_cpu_count)`
- D) `clamp_max(node_load1, scalar(count(node_cpu_seconds_total{mode="idle"})))`

<details><summary>Resposta</summary>

**C.** O segundo argumento precisa ser **escalar**; `node_cpu_count` é um instant vector. D) converte com `scalar()` e é válida (se houver exatamente uma série).
</details>

## 📝 Cola rápida

- `clamp_max(v, max)`: valores `> max` viram `max`; `max` é **escalar**.
- Casos: `max` NaN → NaN; `+Inf` → inalterado; `-Inf` → tudo `-Inf`.
- Para **ver**: cortar outliers (`clamp_max(p99, 2)`), gauge 0..100% (`clamp_max(ratio, 1)`).
- Para **medir** (SLO, média, alerta): valor **cru**.
- `clamp_max` (por série) ≠ `max` (entre séries) ≠ `max_over_time` (no tempo).

## 🔗 Relacionadas

[`clamp()`](../clamp/) · [`clamp_min()`](../clamp_min/) · [`max_over_time()`](../max_over_time/) · [`histogram_quantile()`](../histogram_quantile/) · [`quantile_over_time()`](../quantile_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#clamp_max
