# `max_of()`: o maior de dois escalares (um "piso" dinâmico)

> **Em uma frase:** `max_of(a, b)` recebe **dois escalares** e devolve o **maior**. É a forma curta de dizer "use este valor calculado, **mas nunca menos que** aquele outro".

| | |
|---|---|
| **Assinatura** | `max_of(a scalar, b scalar) → scalar` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de entrada** | **escalares** (literal, `scalar(...)`, `time()`...). ❌ Instant vector dá erro de parse |
| **Unidade do resultado** | a das entradas |
| **Dashboard** | http://localhost:3300/d/fn-max_of |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o salário mínimo

A comissão de um vendedor é calculada todo mês, mas a lei diz que ele **nunca** recebe menos que o salário mínimo. O RH paga `max(comissão, mínimo)`:

- Mês bom, comissão R$ 5.000 → recebe **5.000**.
- Mês ruim, comissão R$ 800 → recebe **o mínimo**.

`max_of(a, b)` é essa regra para **dois números soltos**. Aparece sempre que existe um **piso**: limiar de alerta que não pode ficar sensível demais, mínimo de réplicas, SLO mínimo.

```
      baseline aprendido (60..95)
  95 ┤  ╭─╮           ╭─╮
     │ ╱   ╲         ╱   ╲
  80 ┼█     █████████     █  ← piso: max_of(baseline, 80)
     │       ╲     ╱
  60 ┤        ╰───╯           (sem o piso, o alerta ficaria em 60!)
```

| | recebe | faz |
|---|---|---|
| `max_of(a, b)` | **2 escalares** | o maior dos dois |
| `max(v)` / `max by (x) (v)` | **1 vetor** | agregação: maior valor **entre as séries** |
| `clamp_min(v, min)` | **vetor** + escalar | aplica o piso em **cada série**, mantém labels |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica (imita...) | Labels | Valor |
|---|---|---|
| `max_of_node_cpu_usage_percent` | `node="node-1"` / `"node-2"` | **82 ± 13** (69..95) / **60 ± 10** (50..70), ondas de 3 min |
| `max_of_cpu_baseline_percent` (uma recording rule de baseline) | *(nenhum)* | onda **60 → 95** (5 min) |
| `max_of_rabbitmq_queue_messages_ready` (`rabbitmq_queue_messages_ready`) | *(nenhum)* | onda **0 → 1000** (4 min) |
| `max_of_kube_horizontalpodautoscaler_spec_min_replicas` (kube-state-metrics) | *(nenhum)* | **2** |
| `max_of_backup_threshold_percent` | `replica="a"` / `"b"` | **85** nas duas (armadilha: 2 séries) |

```bash
curl -s localhost:8088/metrics | grep '^max_of_'
# max_of_cpu_baseline_percent 63.1
# max_of_kube_horizontalpodautoscaler_spec_min_replicas 2
# max_of_node_cpu_usage_percent{node="node-1"} 90.4
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-max_of
```

Dados em segundos; ondas de 3 a 5 min.

---

## 🔍 Queries passo a passo

### 🟢 1. Baseline × baseline com piso

```promql
scalar(max_of_cpu_baseline_percent)                    # 60..95
max_of(scalar(max_of_cpu_baseline_percent), 80)        # 80..95
```

**Resultado esperado:**

| baseline | `max_of(baseline, 80)` |
|---|---|
| 60 | **80** |
| 72 | **80** |
| 80 | 80 |
| 91 | 91 |

No gráfico, a curva com piso é a mesma onda, mas **plana em 80** na metade de baixo.

---

### 🔴 2. Alerta de CPU com limiar dinâmico

```promql
max_of_node_cpu_usage_percent                          # CPU por nó
max_of(scalar(max_of_cpu_baseline_percent), 80)        # limiar efetivo
```

**Resultado esperado:** `node-1` (69..95) cruza o limiar **em parte do tempo** (quando está alto e o limiar está no piso de 80); `node-2` (50..70) fica **sempre abaixo** do limiar efetivo.

---

### 🟡 3. Lado a lado: sem piso × com piso

```promql
max_of_node_cpu_usage_percent > scalar(max_of_cpu_baseline_percent)              # SEM piso
max_of_node_cpu_usage_percent > max_of(scalar(max_of_cpu_baseline_percent), 80)  # COM piso
```

**Resultado esperado:**
- **SEM piso:** quando o baseline desce para ~60, **até o `node-2`** (≈65%) passa a "alertar". Um nó a 65% de CPU não é incidente: é **falso positivo** que acorda o plantão.
- **COM piso:** só o `node-1` aparece, e só quando passa de 80% (ou do baseline, se estiver acima de 80).

---

### 🔴 4. Mínimo de réplicas

```promql
scalar(ceil(max_of_rabbitmq_queue_messages_ready / 100))                          # sem piso
max_of(
  scalar(ceil(max_of_rabbitmq_queue_messages_ready / 100)),
  scalar(max_of_kube_horizontalpodautoscaler_spec_min_replicas)
)                                                                                 # com piso
```

**Resultado esperado:**

| fila | desejado | `max_of(desejado, 2)` |
|---|---|---|
| 0 | 0 | **2** |
| 120 | 2 | 2 |
| 650 | 7 | 7 |
| 1000 | 10 | 10 |

Sem o piso, as réplicas iriam a **0** quando a fila esvazia, e a próxima mensagem sofreria *cold start*.

---

### 🟡 5a e 5b. O que dá errado: o piso não salva do NaN

```promql
max_of(scalar(max_of_backup_threshold_percent), 80)          # 5a: NaN
max_of(scalar(max(max_of_backup_threshold_percent)), 80)     # 5b: 85
```

**Resultado esperado:** 5a = **NaN** (2 séries → `scalar()` = NaN → `max_of(NaN, 80)` = NaN); 5b = **85**.
A intuição "pelo menos 80, aconteça o que acontecer" está **errada**: NaN contamina o `max_of`. Um alerta `cpu > max_of(scalar(x), 80)` com `x` duplicado **nunca dispara**.

---

### 🟡 6. `clamp_min`: o piso para vetores

```promql
clamp_min(max_of_node_cpu_usage_percent, 80)
```

**Resultado esperado:** `node-2` vira **80 o tempo todo** (nunca passa de 70); `node-1` fica em 80 quando está abaixo disso e segue a onda acima de 80. Os **labels são mantidos**: é o equivalente "por série" do `max_of`.

---

## 🏭 Casos reais

### 1. Alerta de latência com limiar adaptativo, mas com piso

O time grava um baseline semanal de p99 e quer alertar em "2× o normal", mas **nunca** abaixo de 300 ms (senão, numa semana calma, 150 ms já alertaria):

```yaml
groups:
  - name: latency
    rules:
      - record: checkout:http_request_duration_seconds:p99_1w
        expr: |
          quantile_over_time(0.99,
            (histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[5m]))))[1w:5m]
          )
      - alert: CheckoutLatencyHigh
        expr: |
          histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[5m])))
            > max_of(2 * scalar(max(checkout:http_request_duration_seconds:p99_1w)), 0.3)
        for: 10m
```

**Decisões:** `max(...)` antes do `scalar()` (anti-NaN); `max_of(..., 0.3)` é o piso de 300 ms. É o painel 2/3 do lab com outra métrica.

### 2. Réplicas mínimas em recording rule de capacidade

```yaml
- record: worker:desired_replicas:floored
  expr: |
    vector(
      max_of(
        scalar(ceil(sum(rabbitmq_queue_messages_ready{queue="orders"}) / 100)),
        scalar(max(kube_horizontalpodautoscaler_spec_min_replicas{horizontalpodautoscaler="worker"}))
      )
    )
```

`vector(...)` porque recording rules gravam vetores.

### 3. "Últimos N minutos, mas pelo menos X"

```promql
max_of(scalar(sum(rate(http_requests_total[5m]))), 1)
```

Útil como **denominador** para não dividir por zero (ou por um número minúsculo) em horários de madrugada: `erros / max_of(total, 1)`.

---

## ✅ Quando usar

- **Piso** para limiares dinâmicos (evitar alertas hipersensíveis).
- **Mínimos operacionais**: réplicas, conexões, capacidade reservada.
- Denominador "pelo menos 1" para evitar divisões explosivas.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Piso em **cada série** de um vetor | [`clamp_min()`](../clamp_min/) |
| Maior valor **entre várias séries** | agregador `max` / `max by (...)` |
| Maior valor **ao longo do tempo** | [`max_over_time()`](../max_over_time/) |
| Teto (o menor dos dois) | [`min_of()`](../min_of/) |
| Prometheus sem feature flag | `scalar(clamp_min(vector(a), b))` |

## ⚠️ Pegadinhas

1. **Só escalares:** `max_of(node_load1, 1)` é erro de parse (`expected type scalar ... got instant vector`).
2. **NaN contamina:** `max_of(NaN, 80) = NaN`, então o piso **não** protege contra `scalar()` de 0 ou 2+ séries.
3. **Resultado sem labels:** não dá para usar em `by (...)`; para gravar, use `vector(...)`.
4. **Experimental:** pode mudar; a alternativa estável é `clamp_min(vector(a), b)`.
5. **`max(a, b)` não existe:** o agregador `max` recebe **um** vetor.
6. **A ordem do `scalar()` importa:** funções como `ceil()`, `round()`, `abs()` só aceitam **vetor**. `ceil(scalar(fila) / 100)` é erro de parse (`expected type instant vector in call to function "ceil", got scalar`). Faça a conta no vetor e converta no fim: `scalar(ceil(fila / 100))`.

## 🎓 Na prova PCA

O que cai de verdade é a diferença entre as "famílias" de máximo e os tipos:

- `max_of` (2 escalares) × `max` (agrega vetor) × `clamp_min` (piso por série) × `max_over_time` (no tempo).
- `scalar()` retorna NaN se não houver exatamente 1 elemento.

**1.** Qual expressão aplica um piso de 80 ao limiar vindo de `threshold` (1 série)?

- A) `max(threshold, 80)`
- B) `max_of(scalar(threshold), 80)`
- C) `max_over_time(threshold[80s])`
- D) `clamp_max(threshold, 80)`

<details><summary>Resposta</summary>

**B.** (Também valeria `clamp_min(threshold, 80)`, que devolve vetor.) A) é inválida; C) é outra coisa; D) é um **teto**.
</details>

**2.** `max_of(scalar(x), 80)` com `x` tendo 2 séries retorna:

- A) 80
- B) o maior valor de `x`
- C) NaN
- D) erro

<details><summary>Resposta</summary>

**C.**
</details>

**3.** Qual função mantém os labels e aplica o piso por série?

- A) `max_of`
- B) `clamp_min`
- C) `max`
- D) `scalar`

<details><summary>Resposta</summary>

**B.**
</details>

## 📝 Cola rápida

- `max_of(a, b)` = maior de **2 escalares** → **piso**.
- Vetor? `clamp_min`. Várias séries? `max`. No tempo? `max_over_time`.
- `scalar(max(...))` antes: NaN contamina e o piso **não** salva.
- Escalar → `vector(...)` para gravar em regra.
- Experimental.

## 🔗 Relacionadas

[`min_of()`](../min_of/) · [`scalar()`](../scalar/) · [`vector()`](../vector/) · [`clamp_min()`](../clamp_min/) · [`max_over_time()`](../max_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#max_of
