# `min_of()`: o menor de dois escalares (um "teto" dinâmico)

> **Em uma frase:** `min_of(a, b)` recebe **dois escalares** e devolve o **menor**. É a forma curta de dizer "use este valor calculado, **mas no máximo** aquele outro".

| | |
|---|---|
| **Assinatura** | `min_of(a scalar, b scalar) → scalar` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de entrada** | **escalares** (número literal, `scalar(...)`, `time()`, outra expressão escalar). ❌ Instant vector dá erro de parse |
| **Unidade do resultado** | a das entradas |
| **Dashboard** | http://localhost:3300/d/fn-min_of |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: "o que for menor"

Seu chefe diz: *"pode gastar até R$ 500 no almoço com o cliente, ou o que tiver no cartão corporativo, **o que for menor**"*. Você não decide sozinho: aplica a regra `min(500, saldo)`.

- Saldo R$ 2.000 → gasta até **500** (o teto manda).
- Saldo R$ 300 → gasta até **300** (o saldo manda).

`min_of(a, b)` é essa regra para **dois números soltos** (escalares). Ela aparece sempre que existe um **teto**: máximo de réplicas, limite de taxa, cota.

```
         desejado (0..18)
   18 ┤      ╭──╮
      │     ╱    ╲
   10 ┼────█████████────  ← teto (maxReplicas)   min_of(desejado, 10)
      │   ╱        ╲
    0 ┼──╯          ╰──
```

### E qual a diferença para `min()` e `clamp_max()`?

| | recebe | faz |
|---|---|---|
| `min_of(a, b)` | **2 escalares** | o menor dos dois |
| `min(v)` / `min by (x) (v)` | **1 vetor** (várias séries) | agregação: o menor valor **entre as séries** |
| `clamp_max(v, max)` | **vetor** + escalar | aplica o teto a **cada série**, mantendo labels |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica (imita...) | Labels | Valor |
|---|---|---|
| `min_of_rabbitmq_queue_messages_ready` (`rabbitmq_queue_messages_ready`) | *(nenhum)* | onda **0 → 1800** (5 min) |
| `min_of_kube_horizontalpodautoscaler_spec_max_replicas` (kube-state-metrics) | *(nenhum)* | **10** |
| `min_of_tenant_limit_rps` | *(nenhum)* | **500** (contrato do cliente) |
| `min_of_global_limit_rps` | *(nenhum)* | **800 / 300** alternando a cada 2 min (cluster sob pressão reduz o limite) |
| `min_of_backend_limit_rps` | `replica="a"` / `"b"` | **400** nas duas (armadilha: 2 séries) |

```bash
curl -s localhost:8088/metrics | grep '^min_of_'
# min_of_global_limit_rps 800
# min_of_kube_horizontalpodautoscaler_spec_max_replicas 10
# min_of_rabbitmq_queue_messages_ready 1312.4
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-min_of
```

Os dados aparecem em segundos; a onda da fila tem 5 min.

---

## 🔍 Queries passo a passo

### 🟢 1. A fila

```promql
min_of_rabbitmq_queue_messages_ready
```

**Resultado esperado:** uma onda de **0 a 1800** mensagens com período de 5 min.

---

### 🟢 2. Workers desejados, com e sem teto

```promql
scalar(ceil(min_of_rabbitmq_queue_messages_ready / 100))                        # sem teto
min_of(
  scalar(ceil(min_of_rabbitmq_queue_messages_ready / 100)),
  scalar(min_of_kube_horizontalpodautoscaler_spec_max_replicas)
)                                                                               # com teto
```

**O que faz:** "1 worker a cada 100 mensagens", arredondado para cima. O `scalar()` é obrigatório porque `min_of` só aceita escalares.
**Resultado esperado:**

| fila | desejado | `min_of(desejado, 10)` |
|---|---|---|
| 0 | 0 | 0 |
| 450 | 5 | 5 |
| 1000 | 10 | 10 |
| 1500 | 15 | **10** |
| 1800 | 18 | **10** |

No gráfico, a curva "sem teto" sobe até 18; a do `min_of` é **cortada em 10** (fica plana no topo).

---

### 🔴 3. Rate limit efetivo: o mais restritivo

```promql
min_of(scalar(min_of_tenant_limit_rps), scalar(min_of_global_limit_rps))
```

**Resultado esperado:** uma linha em degraus: **500** quando o global está em 800, e **300** quando o global cai para 300 (a cada 2 min). Exatamente o que um *API gateway* faria: "o cliente tem direito a 500, mas se o cluster só aguenta 300 agora, vale 300".

---

### 🟡 4. `min_of` × `clamp_max`

```promql
min_of(scalar(ceil(min_of_rabbitmq_queue_messages_ready / 100)), 10)    # escalar
clamp_max(ceil(min_of_rabbitmq_queue_messages_ready / 100), 10)         # vetor
```

**Resultado esperado:** as **duas curvas são idênticas** (0 → 10, plana no topo). A diferença é o **tipo**: `clamp_max` recebe um vetor, aplica o teto em **cada série** e mantém os labels; `min_of` só funciona com **dois números**. Se a fila tivesse `{queue="orders"}` e `{queue="emails"}`, só o `clamp_max` daria um valor por fila.

---

### 🟡 5a e 5b. O que dá errado: NaN contamina

```promql
min_of(scalar(min_of_backend_limit_rps), 500)          # 5a: NaN
min_of(scalar(max(min_of_backend_limit_rps)), 500)     # 5b: 400
```

**Resultado esperado:**

| painel | conta | resultado |
|---|---|---|
| 5a | `scalar(2 séries)` = NaN → `min_of(NaN, 500)` | **NaN** |
| 5b | `scalar(max(...))` = 400 → `min_of(400, 500)` | **400** |

Muita gente espera que `min_of(NaN, 500)` devolva 500 ("o NaN é ignorado"). **Não**: no Prometheus v3.15.0, se qualquer lado é NaN, o resultado é **NaN**. Garanta 1 elemento com uma agregação antes do `scalar()`.

---

## 🏭 Casos reais

### 1. Autoscaling orientado a fila respeitando o `maxReplicas` (KEDA-like)

Para exibir (ou alimentar um *external metrics adapter*) o número de workers "que o HPA vai de fato usar":

```yaml
groups:
  - name: autoscaling
    rules:
      - record: worker:desired_replicas:capped
        expr: |
          vector(
            min_of(
              scalar(ceil(sum(rabbitmq_queue_messages_ready{queue="orders"}) / 100)),
              scalar(max(kube_horizontalpodautoscaler_spec_max_replicas{horizontalpodautoscaler="worker"}))
            )
          )
```

**Decisões:** recording rules precisam gravar um **vetor**, por isso o `vector(...)` por fora; o `sum`/`max` dentro dos `scalar()` são o seguro anti-NaN (painel 5).

### 2. Alerta de "fila vai estourar mesmo no máximo de réplicas"

```yaml
- alert: WorkersAtMaxReplicas
  expr: |
    vector(
      scalar(ceil(sum(rabbitmq_queue_messages_ready{queue="orders"}) / 100))
      - min_of(
          scalar(ceil(sum(rabbitmq_queue_messages_ready{queue="orders"}) / 100)),
          scalar(max(kube_horizontalpodautoscaler_spec_max_replicas{horizontalpodautoscaler="worker"}))
        )
    ) > 0
  for: 15m
  annotations:
    summary: "Fila precisa de mais workers do que o maxReplicas permite"
```

"Desejado − com teto > 0" significa que o **teto está cortando**: é hora de subir o `maxReplicas` ou otimizar o consumidor.

### 3. Limite efetivo num painel de *API gateway*

```promql
min_of(scalar(max(tenant_rate_limit_rps{tenant="acme"})), scalar(max(global_rate_limit_rps)))
```

Painel *stat* "limite efetivo do cliente ACME agora", igual ao painel 3 do lab.

---

## ✅ Quando usar

- **Teto** para um número calculado: réplicas, limites, cotas, *budgets*.
- Escolher o **mais restritivo** entre duas configurações que vêm de métricas.
- Painéis *stat* e recording rules que operam em **um número** global.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Teto em **cada série** de um vetor | [`clamp_max()`](../clamp_max/) |
| Menor valor **entre várias séries** | agregador `min` / `min by (...)` |
| Menor valor **ao longo do tempo** | [`min_over_time()`](../min_over_time/) |
| Piso (o maior dos dois) | [`max_of()`](../max_of/) |
| Prometheus sem feature flag | `clamp_max(vector(a), b)` → vetor; `scalar(clamp_max(vector(a), b))` → escalar |

## ⚠️ Pegadinhas

1. **Só escalares:** `min_of(up, 1)` → `parse error: expected type scalar in call to function "min_of", got instant vector`. Use `scalar(...)` ou `clamp_max`.
2. **NaN contamina:** `min_of(NaN, 500) = NaN`. E `scalar()` de 0 ou 2+ séries é NaN (painel 5a).
3. **Resultado é escalar:** não tem labels; para gravar em recording rule, embrulhe com `vector(...)`.
4. **Experimental:** pode mudar/sumir; em Prometheus sem a flag, use `clamp_max(vector(a), b)`.
5. **Não confunda com `min()`:** `min(a, b)` **não** é válido (o agregador recebe **um** vetor).
6. **A ordem do `scalar()` importa:** funções como `ceil()`, `round()`, `abs()` só aceitam **vetor**. `ceil(scalar(fila) / 100)` é erro de parse (`expected type instant vector in call to function "ceil", got scalar`). Faça a conta no vetor e converta no fim: `scalar(ceil(fila / 100))`.

## 🎓 Na prova PCA

Funções experimentais aparecem pouco; o que cai é o conceito de **tipos** e as alternativas estáveis:

- `min_of`/`max_of` operam em **escalares**; `min`/`max` agregam **vetores**; `clamp_max`/`clamp_min` limitam **cada série**; `*_over_time` olham o **tempo**.
- Conversões `scalar()` ↔ `vector()`.

**1.** Qual expressão é **válida**?

- A) `min_of(node_load1, 4)`
- B) `min_of(scalar(max(node_load1)), 4)`
- C) `min(node_load1, 4)`
- D) `min_of(node_load1[5m], 4)`

<details><summary>Resposta</summary>

**B.** `min_of` exige dois escalares. A) passa um vetor; C) `min` é agregador de um vetor só; D) range vector.
</details>

**2.** Qual o resultado de `min_of(scalar(up), 1)` com 3 alvos?

- A) 1
- B) 0
- C) NaN
- D) 3

<details><summary>Resposta</summary>

**C.** `scalar(up)` com 3 séries é NaN, e NaN contamina o `min_of`.
</details>

**3.** Você quer limitar **cada** série de `desired_replicas{deployment=...}` a 10. Qual função?

- A) `min_of`
- B) `min`
- C) `clamp_max`
- D) `min_over_time`

<details><summary>Resposta</summary>

**C.** Aplica o teto por série e mantém os labels.
</details>

**4.** Qual o tipo de retorno de `min_of(1, 2)`?

- A) instant vector
- B) scalar
- C) range vector
- D) string

<details><summary>Resposta</summary>

**B.**
</details>

## 📝 Cola rápida

- `min_of(a, b)` = menor de **2 escalares** → **teto**.
- Vetor? `clamp_max(v, x)`. Várias séries? `min`. No tempo? `min_over_time`.
- `scalar(max(...))` antes, senão NaN; NaN **contamina**.
- Resultado escalar → `vector(...)` para recording rule.
- Experimental.

## 🔗 Relacionadas

[`max_of()`](../max_of/) · [`scalar()`](../scalar/) · [`vector()`](../vector/) · [`clamp_max()`](../clamp_max/) · [`min_over_time()`](../min_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#min_of
