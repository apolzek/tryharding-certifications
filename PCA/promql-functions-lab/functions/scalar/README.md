# `scalar()`: transformar um vetor de 1 elemento em um número puro

> **Em uma frase:** `scalar(v)` pega um instant vector que tem **exatamente um** elemento e devolve **só o valor dele**, como escalar (sem labels). Se o vetor tiver **zero ou mais de um** elemento, devolve **`NaN`**, sem erro.

| | |
|---|---|
| **Assinatura** | `scalar(v instant-vector) → scalar` |
| **Tipo de métrica** | ✅ Qualquer float (normalmente o resultado de `sum`, `max`, `count`...) · ⚠️ amostras de **histogram** são ignoradas |
| **Unidade do resultado** | a mesma da entrada, mas **sem labels** |
| **Dashboard** | http://localhost:3300/d/fn-scalar |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: tirar o número da etiqueta

Um **instant vector** é uma prateleira de **caixas etiquetadas**: `{service="checkout"} 60`, `{service="cart"} 30`... Quando você faz uma conta entre dois vetores (`a / b`), o PromQL tenta **casar as etiquetas** caixa por caixa. Se as etiquetas não batem, a conta não acontece.

Um **escalar** é **um número solto, sem etiqueta**. Ele combina com **qualquer** caixa: `v / 100` divide todas as séries por 100.

O `scalar()` é o gesto de **abrir a caixa e tirar o número**. Mas ele só sabe fazer isso se houver **uma** caixa na prateleira:

```
prateleira com 1 caixa:   { } 100          → scalar() = 100
prateleira com 3 caixas:  {a} 60 {b} 30 …  → scalar() = NaN   ("qual das três?")
prateleira vazia:                          → scalar() = NaN   ("não tem nada")
```

E ele não reclama: devolve `NaN` em silêncio. Por isso é poderoso **e** perigoso.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Labels | Comportamento |
|---|---|---|
| `scalar_http_requests_total` (counter, imita `http_requests_total`) | `service="checkout"` / `cart` / `search` | cresce ≈ **60 / 30 / 10** por segundo (total ≈ **100**) |
| `scalar_node_cpu_usage_percent` | `node="node-1"` / `node-2` / `node-3` | **85 ± 10** / **65 ± 10** / **40 ± 5** % (ondas de 3 min) |
| `scalar_cpu_alert_threshold_percent` | *(nenhum)* | **80** e **70** alternando a cada 2 min (alguém ajusta o alerta em runtime) |
| `scalar_config_threshold_percent` | `replica="a"` / `"b"` | **80** nas duas (o mesmo limiar exportado por 2 réplicas) |

```bash
curl -s localhost:8088/metrics | grep '^scalar_'
# scalar_config_threshold_percent{replica="a"} 80
# scalar_config_threshold_percent{replica="b"} 80
# scalar_cpu_alert_threshold_percent 80
# scalar_node_cpu_usage_percent{node="node-1"} 91.2
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-scalar
```

Espere ~1 minuto (janela de `rate[1m]`); o limiar troca a cada 2 min.

---

## 🔍 Queries passo a passo

### 1. req/s por serviço

```promql
sum by (service) (rate(scalar_http_requests_total[1m]))
```

**Resultado esperado:** checkout ≈ **60**, cart ≈ **30**, search ≈ **10** req/s.

---

### 2. Porcentagem do total

```promql
sum by (service) (rate(scalar_http_requests_total[1m]))
  / scalar(sum(rate(scalar_http_requests_total[1m])))
```

**O que faz:** o denominador `sum(...)` é um vetor de **1** elemento **sem labels**; o `scalar()` o transforma em número. Aí cada série do numerador é dividida por esse número.
**Resultado esperado:** checkout ≈ **60%**, cart ≈ **30%**, search ≈ **10%**.

> Sem o `scalar()`, `vetor{service=...} / vetor{}` não casa (labels diferentes) e o resultado é **vazio**. Alternativas equivalentes: `/ ignoring(service) group_left sum(...)` ou `/ on() group_left sum(...)`. O `scalar()` é a forma mais curta.

---

### 3 e 4. Limiar vindo de uma métrica

```promql
scalar(scalar_cpu_alert_threshold_percent)                                  # linha do limiar
scalar_node_cpu_usage_percent > scalar(scalar_cpu_alert_threshold_percent)  # quem passou
```

**O que faz:** o limiar é uma métrica (configurável em runtime, por exemplo por um *config exporter*), com labels diferentes das séries de CPU. Como escalar, ele pode ser comparado com **todas** as séries.
**Resultado esperado:**

- Painel 3: 3 linhas de CPU e uma linha "degrau" alternando **80 ↔ 70** a cada 2 minutos.
- Painel 4 (pontos): `node-1` (75..95) aparece **quase sempre**; `node-2` (55..75) só aparece quando o limiar está em **70** **e** ele está perto do pico; `node-3` (35..45) **nunca**.

---

### 5. Quando o `scalar()` devolve NaN

```promql
scalar(sum(rate(scalar_http_requests_total[1m])))                            # 1 elemento
scalar(rate(scalar_http_requests_total[1m]))                                 # 3 elementos
scalar(rate(scalar_http_requests_total{service="nao-existe"}[1m]))           # 0 elementos
```

**Resultado esperado (stat):**

| query | elementos | resultado |
|---|---|---|
| `scalar(sum(...))` | 1 | ≈ **100** |
| `scalar(rate(...))` | 3 | **NaN** |
| `scalar(... nao-existe ...)` | 0 | **NaN** |

Nenhuma delas dá **erro**. A query "funciona", só que o número é lixo.

---

### 6 e 7. A armadilha do alerta que nunca dispara

```promql
scalar_node_cpu_usage_percent > scalar(scalar_config_threshold_percent)       # painel 6: VAZIO
scalar_node_cpu_usage_percent > scalar(max(scalar_config_threshold_percent))  # painel 7: funciona
```

**O que acontece:** o limiar é exportado por **2 réplicas** (`replica="a"` e `"b"`), então `scalar()` recebe 2 elementos e devolve `NaN`. Qualquer comparação com `NaN` é **falsa**, logo o resultado é **vazio**, mesmo com o `node-1` em 90%. Um alerta escrito assim **nunca dispara**, e nada avisa você.
**Correção:** agregue antes (`max`, `min`, `avg`) para garantir **1 elemento**. No painel 7 o `node-1` volta a aparecer.

---

## 🏭 Casos reais

### 1. Percentual de uso do cluster (kube-state-metrics)

"Quanto da CPU alocável do cluster já foi pedida pelos pods, por namespace?"

```promql
sum by (namespace) (kube_pod_container_resource_requests{resource="cpu"})
  / scalar(sum(kube_node_status_allocatable{resource="cpu"}))
```

Cada namespace vira uma fração do total do cluster. **Decisão:** o denominador é **um** número para o cluster todo; `scalar()` evita escrever o `on() group_left`. Como recording rule:

```yaml
groups:
  - name: capacity
    rules:
      - record: namespace:cpu_requests:ratio_cluster
        expr: |
          sum by (namespace) (kube_pod_container_resource_requests{resource="cpu"})
            / scalar(sum(kube_node_status_allocatable{resource="cpu"}))
```

⚠️ Se você tiver **vários clusters** no mesmo Prometheus (ou Thanos), o `sum(...)` do denominador soma **todos** e o `scalar()` esconde isso. Aí o certo é `/ on (cluster) group_left sum by (cluster) (...)`.

### 2. Limiar dinâmico vindo de configuração

Um *feature flag service* exporta `alert_threshold{name="cpu_high"} 80`. O alerta:

```yaml
- alert: NodeCPUHigh
  expr: |
    100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))
      > scalar(max(alert_threshold{name="cpu_high"}))
  for: 10m
```

**Decisão:** o `max(...)` dentro do `scalar()` é o seguro contra o NaN silencioso se o exporter for escalado para 2 réplicas (painel 6 → 7).

### 3. Participação no tráfego (Black Friday)

```promql
sum by (route) (rate(http_requests_total[5m]))
  / scalar(sum(rate(http_requests_total[5m])))
```

Gráfico empilhado de 0 a 100%: mostra se o `/checkout` está ganhando participação (bom) ou se o `/search` está engolindo a capacidade (ruim).

### 4. Idade de algo sem labels atrapalhando

```promql
time() - scalar(max(backup_last_success_timestamp_seconds))
```

Resultado **escalar** (sem labels), bom para um painel *stat* "Último backup há X segundos".

---

## ✅ Quando usar

- **Dividir/comparar todas as séries por UM número** (total, capacidade, limiar).
- **Limiar vindo de métrica** em alertas (com `max`/`min` dentro!).
- Converter um resultado agregado para usar em parâmetros que exigem **escalar**: `clamp_max(v, scalar(...))`, `max_of(scalar(...), 80)`, `histogram_quantile(scalar(...), ...)`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| O "denominador" tem **várias** séries (por cluster, por região) | `/ on (cluster) group_left ...` (join vetor-vetor) |
| Converter escalar → vetor | [`vector()`](../vector/) |
| Escolher o maior/menor de dois escalares | [`max_of()`](../max_of/) / [`min_of()`](../min_of/) |
| Dados que **podem** ter 0 séries e você precisa de fallback | `(... or vector(0))` antes, ou trate o NaN |

## ⚠️ Pegadinhas

1. **NaN silencioso:** 0 ou 2+ elementos → `NaN`, sem erro nem warning. Em alertas, isso vira "nunca dispara" (painel 6).
2. **Agregue antes:** `scalar(sum(...))`, `scalar(max(...))`. Nunca `scalar(metrica_crua)` se ela pode ganhar uma segunda série (novo pod, nova réplica, label novo).
3. **Perde os labels:** o resultado não tem labels, então não dá para usá-lo em `by (...)`, `on (...)` ou legenda.
4. **Histograms ignorados:** um vetor com 1 histograma → `NaN`.
5. **Escalar em range query:** no gráfico, o `scalar()` é recalculado a cada passo; se em algum passo houver 2 séries (ex.: durante um rollout), aquele ponto é `NaN` (um "buraco" na linha).
6. **Nem toda função aceita escalar calculado:** parâmetros como o `t` de `predict_linear` e o `φ` de `histogram_quantile` aceitam expressões escalares, mas o **tamanho da janela** `[5m]` não (tem de ser literal).

## 🎓 Na prova PCA

O que costuma cair:

- **Tipos de dados do PromQL:** instant vector, range vector, **scalar**, string. `scalar()` converte vetor → escalar; `vector()` faz o contrário.
- **O que retorna com 0 ou >1 elementos** (NaN).
- Operações **vetor op escalar** aplicam a todas as séries; **vetor op vetor** exige casamento de labels.

**1.** Qual o resultado de `scalar(up)` se existem 3 alvos?

- A) 3
- B) 1
- C) NaN
- D) Erro de execução

<details><summary>Resposta</summary>

**C.** Mais de um elemento → NaN (sem erro).
</details>

**2.** Por que `rate(x[5m]) / sum(rate(x[5m]))` normalmente retorna vazio, e `rate(x[5m]) / scalar(sum(rate(x[5m])))` funciona?

- A) `sum` não funciona com `rate`
- B) Vetor/vetor exige labels iguais; o `sum` não tem labels. Com `scalar`, vira vetor/escalar e aplica a todas as séries
- C) `scalar` arredonda o resultado
- D) Não há diferença

<details><summary>Resposta</summary>

**B.**
</details>

**3.** Uma regra de alerta `cpu > scalar(threshold)` nunca dispara, e `threshold` existe. Qual a causa mais provável?

- A) `threshold` tem mais de uma série, e `scalar()` retorna NaN
- B) `scalar()` só aceita counters
- C) `>` não funciona com escalares
- D) Falta o `for:`

<details><summary>Resposta</summary>

**A.** Comparações com NaN são sempre falsas.
</details>

**4.** Qual o tipo de retorno de `scalar(sum(up))`?

- A) instant vector
- B) range vector
- C) scalar
- D) string

<details><summary>Resposta</summary>

**C.**
</details>

## 📝 Cola rápida

- `scalar(v)`: **1 elemento** → valor; **0 ou 2+** → `NaN` (sem erro).
- Sempre **agregue antes**: `scalar(sum(...))`, `scalar(max(...))`.
- Uso nº 1: "% do total" → `x / scalar(sum(x))`.
- NaN em comparação = falso → alerta que nunca dispara.
- Inverso: [`vector(s)`](../vector/).

## 🔗 Relacionadas

[`vector()`](../vector/) · [`max_of()`](../max_of/) · [`min_of()`](../min_of/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#scalar
