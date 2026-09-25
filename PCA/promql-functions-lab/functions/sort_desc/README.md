# `sort_desc()`: ordenar pelo valor, do maior para o menor (o "Top N")

> **Em uma frase:** `sort_desc(v)` devolve as mesmas séries **do maior para o menor valor**. É o parceiro natural do `topk` para montar tabelas "Top N". Assim como o [`sort()`](../sort/), **só faz efeito em consultas instantâneas**.

| | |
|---|---|
| **Assinatura** | `sort_desc(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer float (normalmente o resultado de `rate`, `sum`, razões) · ⚠️ amostras de **histogram** são ignoradas |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-sort_desc |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a tabela do campeonato

A tabela do Brasileirão mostra o líder em cima e o lanterna embaixo. Ninguém mexe nos pontos dos times: só a **ordem das linhas** muda a cada rodada. `sort_desc()` é exatamente isso: **ranking do maior para o menor**.

E, como no futebol, a tabela é uma **foto da rodada** (consulta instantânea). O gráfico da temporada inteira (range query) tem uma linha por time, e ali "ordem" não significa nada: o Prometheus ignora o `sort_desc`.

```
sort_desc( CPU por pod )   ← consulta instantânea
┌─────────────────┬───────┐
│ api-7f9c-b      │ 0.89  │ 🥇 (líder agora)
│ api-7f9c-a      │ 0.61  │
│ web-5d8e-x      │ 0.52  │
│ ...             │ ...   │
│ coredns-66bff   │ 0.01  │ (lanterna)
└─────────────────┴───────┘
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica (imita...) | Labels | Velocidade (`rate`) |
|---|---|---|
| `sort_desc_container_cpu_usage_seconds_total` (cAdvisor, counter) | `namespace="shop"`, `pod="api-7f9c-a"` | **0.6 ± 0.4** cores (onda de 4 min) |
| idem | `shop`/`api-7f9c-b` | **0.7 ± 0.2** (4 min, fase **oposta** ao api-a) |
| idem | `shop`/`web-5d8e-x` | **0.5 ± 0.1** |
| idem | `batch`/`worker-0` | **0.25 ± 0.15** |
| idem | `batch`/`cron-2911` | **0.15 ± 0.05** |
| idem | `shop`/`redis-0` · `kube-system`/`coredns-66bff` | **0.08** · **0.01** |
| `sort_desc_http_requests_total` (counter) | `service`, `code="200"`/`"500"` | payments 88+12/s · checkout 95+5/s · cart 99+1/s · recommendations **0** |

O `api-a` e o `api-b` se revezam no 1º lugar (quando um está no pico, o outro está no vale). O `recommendations` tem as séries criadas, mas **nunca recebe tráfego**, então a taxa de erro dele é `0/0 = NaN`.

```bash
curl -s localhost:8088/metrics | grep '^sort_desc_'
# sort_desc_container_cpu_usage_seconds_total{namespace="shop",pod="api-7f9c-a"} 812.4
# sort_desc_http_requests_total{code="500",service="payments"} 43200
# sort_desc_http_requests_total{code="500",service="recommendations"} 0
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sort_desc
```

Espere ~1 minuto (janela de `rate[1m]`). O líder da tabela troca a cada ~2 minutos.

---

## 🔍 Queries passo a passo

### 1. CPU por pod no tempo

```promql
sum by (pod) (rate(sort_desc_container_cpu_usage_seconds_total[1m]))
```

**Resultado esperado:** 7 linhas. `api-7f9c-a` oscila entre 0.2 e 1.0; `api-7f9c-b` entre 0.5 e 0.9, **em fase oposta**. As duas se cruzam, então o "líder" muda.

---

### 2. `sort_desc()`: ranking completo

```promql
sort_desc(sum by (namespace, pod) (rate(sort_desc_container_cpu_usage_seconds_total[1m])))
```

**Resultado esperado** (exemplo de um instante com api-b no pico):

| namespace | pod | cores |
|---|---|---|
| shop | api-7f9c-b | 0.89 |
| shop | web-5d8e-x | 0.55 |
| batch | worker-0 | 0.31 |
| shop | api-7f9c-a | 0.22 |
| batch | cron-2911 | 0.14 |
| shop | redis-0 | 0.08 |
| kube-system | coredns-66bff | 0.01 |

A coluna de valores está **sempre decrescente**; os nomes mudam de posição com o tempo.

---

### 3. Top 3: `sort_desc(topk(3, ...))`

```promql
sort_desc(topk(3, sum by (pod) (rate(sort_desc_container_cpu_usage_seconds_total[1m]))))
```

**O que faz:** `topk(3, ...)` **escolhe** os 3 maiores, mas a documentação **não garante a ordem** da saída do `topk`. O `sort_desc` por fora resolve isso.
**Resultado esperado:** 3 barras, a maior em cima. Quase sempre `api-7f9c-b`, `web-5d8e-x` e `api-7f9c-a`; quando o `api-7f9c-a` está no vale (≈0.2) e o `worker-0` no pico (≈0.4), o `worker-0` rouba o 3º lugar.

---

### 4. Ranking de taxa de erro e o NaN

```promql
sort_desc(
  sum by (service) (rate(sort_desc_http_requests_total{code=~"5.."}[1m]))
    / sum by (service) (rate(sort_desc_http_requests_total[1m]))
)
```

**Resultado esperado:**

| service | erro |
|---|---|
| payments | **12 %** |
| checkout | **5 %** |
| cart | **1 %** |
| recommendations | **NaN** ← no fim, **não** no topo |

Por que NaN? `recommendations` teve 0 erros **e** 0 requisições: `0 / 0 = NaN`. O Prometheus coloca NaN **no fim** tanto no `sort` quanto no `sort_desc`.

---

### 5. Tirando o NaN do ranking

```promql
sort_desc(razao == razao)   # onde "razao" é a expressão do painel 4
```

**O que faz:** `NaN == NaN` é **falso** (é a única coisa diferente de si mesma), então o filtro remove só as séries NaN.
**Resultado esperado:** só payments, checkout, cart.

---

### 6. `sort_desc()` num gráfico

**Resultado esperado:** **idêntico** ao painel 1, e com um ⚠️ no título: o Prometheus responde `PromQL warning: sort is ineffective for range queries since results are always ordered by labels`. Para ordenar a legenda use *Legend → Sort by → Last / Max* no Grafana; para o tooltip, *Tooltip → Sort order: Descending*.

---

## 🏭 Casos reais

### 1. "Quem está comendo a CPU do cluster?" (Top 10 pods)

```promql
sort_desc(
  topk(10,
    sum by (namespace, pod) (
      rate(container_cpu_usage_seconds_total{container!="", container!="POD"}[5m])
    )
  )
)
```

Com recording rule (o `sort_desc` fica só no painel):

```yaml
groups:
  - name: pod-cpu
    rules:
      - record: namespace_pod:container_cpu_usage_seconds:rate5m
        expr: sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{container!="", container!="POD"}[5m]))
```

Painel: `sort_desc(topk(10, namespace_pod:container_cpu_usage_seconds:rate5m))`.

Tabela no dashboard do cluster. **Decisões:** `container!=""` remove a série agregada do cgroup do pod (senão conta em dobro); `sum by (namespace, pod)` junta os containers do mesmo pod; `topk` limita; `sort_desc` ordena. Este é o painel 2/3 do lab.

### 2. Ranking de endpoints mais lentos (histograma)

```promql
sort_desc(
  topk(5,
    histogram_quantile(0.99, sum by (le, handler) (rate(http_request_duration_seconds_bucket[5m])))
  )
)
```

Útil numa *war room*: "quais 5 rotas estão com o p99 pior agora?".

### 3. Serviços com mais erro, sem os ociosos

```promql
sort_desc(
  (sum by (service) (rate(http_requests_total{code=~"5.."}[5m]))
     / sum by (service) (rate(http_requests_total[5m])))
  > 0
)
```

O `> 0` resolve dois problemas de uma vez: tira os serviços sem erro **e** os NaN (qualquer comparação com NaN é falsa). Mesma lógica do painel 5. O alerta correspondente usa a mesma razão, sem ordenar:

```yaml
- alert: ServiceHighErrorRate
  expr: |
    (sum by (service) (rate(http_requests_total{code=~"5.."}[5m]))
       / sum by (service) (rate(http_requests_total[5m]))) > 0.05
  for: 10m
  annotations:
    summary: "{{ $labels.service }} com {{ $value | humanizePercentage }} de erro"
```

---

## ✅ Quando usar

- **Tabelas "Top N"**: pods por CPU/memória, rotas por req/s, clientes por consumo, filas por tamanho.
- **Com `topk`**, sempre que a ordem de exibição importa (`sort_desc(topk(N, ...))`).
- **Relatórios via API** (`/api/v1/query`): ChatOps, "top 5 no Slack", scripts.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| O menor é o problema (disco livre, réplicas prontas) | [`sort()`](../sort/) |
| Ordenar por **nome**/versão (label) | [`sort_by_label_desc()`](../sort_by_label_desc/) |
| Só quer **limitar** a N séries | `topk(N, v)` (o `sort_desc` não corta nada) |
| Painel **timeseries** | *Legend → Sort by* no Grafana |
| Alertas e recording rules | não use: ordem não tem significado |

## ⚠️ Pegadinhas

1. **Range query ignora** o `sort_desc`.
2. **NaN vai para o fim**, não para o topo. Um serviço com 0 tráfego não aparece como "o pior".
3. **`topk` não garante ordem**: use `sort_desc(topk(...))`.
4. **`topk` em gráfico** é traiçoeiro (outro assunto, mas cai junto): em range query, `topk` é calculado **em cada passo**, então a legenda pode ter mais de N séries.
5. **Histogramas são descartados** do resultado.

## 🎓 Na prova PCA

O que costuma cair:

- `sort_desc` × `topk`: um **ordena**, o outro **seleciona** (e não garante ordem).
- Só funciona em **instant queries**.
- NaN no fim.

**1.** Qual query mostra os 5 pods com maior uso de CPU, do maior para o menor?

- A) `topk(5, sort(rate(container_cpu_usage_seconds_total[5m])))`
- B) `sort_desc(topk(5, sum by (pod) (rate(container_cpu_usage_seconds_total[5m]))))`
- C) `sort_desc(rate(container_cpu_usage_seconds_total[5m]))[5]`
- D) `limit(5, sort_desc(container_cpu_usage_seconds_total))`

<details><summary>Resposta</summary>

**B.** `topk` seleciona, `sort_desc` ordena. C e D não são sintaxe válida para isso; A ordena **antes** de selecionar (e crescente), e a saída do `topk` não tem ordem garantida.
</details>

**2.** Qual a diferença entre `sort_desc(x)` e `topk(3, x)`?

- A) Nenhuma
- B) `sort_desc` devolve todas as séries ordenadas; `topk` devolve só 3, sem garantia de ordem
- C) `topk` só funciona em range queries
- D) `sort_desc` remove séries NaN

<details><summary>Resposta</summary>

**B.**
</details>

**3.** A taxa de erro de um serviço sem tráfego é `NaN`. Em `sort_desc(taxa_erro)`, onde ele aparece?

- A) Primeiro, porque NaN é tratado como +Inf
- B) Último
- C) É removido
- D) Causa erro na query

<details><summary>Resposta</summary>

**B.** Tanto `sort` quanto `sort_desc` colocam NaN no fim.
</details>

**4.** Você coloca `sort_desc(...)` num painel timeseries e a legenda não muda. Por quê?

- A) Bug do Grafana
- B) Precisa de `by (le)`
- C) `sort_desc` só afeta instant queries
- D) Precisa usar `sort_by_label_desc`

<details><summary>Resposta</summary>

**C.**
</details>

## 📝 Cola rápida

- `sort_desc(v)` = maior primeiro; **só instant query**.
- Top N correto: `sort_desc(topk(N, sum by (...) (rate(...))))`.
- NaN **no fim**; para removê-lo: `v == v` ou `v > 0`.
- Não corta séries e não altera valores.

## 🔗 Relacionadas

[`sort()`](../sort/) · [`sort_by_label_desc()`](../sort_by_label_desc/) · [`sort_by_label()`](../sort_by_label/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#sort_desc
