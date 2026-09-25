# `range()`: a duração da janela do gráfico (experimental)

> **Em uma frase:** `range()` devolve a **duração em segundos** da consulta *range* em andamento (`end() - start()`), o equivalente ao `$__range` do Grafana dentro do PromQL. Numa consulta **instantânea**, `range()` = **0**.

| | |
|---|---|
| **Assinatura** | `range() → scalar` |
| **Status** | 🧪 **experimental**: exige `--enable-feature=promql-experimental-functions` |
| **Tipo de métrica** | nenhuma. Também pode ser usado como **duração**: `x[range()]` |
| **Unidade do resultado** | segundos |
| **Dashboard** | http://localhost:3300/d/fn-range |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a duração do filme

O gráfico é um **filme** (veja [`start()`](../start/)). `range()` é a **duração** dele: um gráfico de "Last 15 minutes" é um filme de **900 s**; "Last 24 hours", de **86 400 s**.

Uma **foto** (stat instantâneo, regra de alerta) tem duração **zero**.

O truque mais útil: usar essa duração **como janela**. `increase(x[range()] @ end())` = "quanto x cresceu **no período que estou vendo**", e o número muda sozinho quando você muda o zoom.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `range_http_requests_total` | counter | ~**10 req/s**, com pico de ~**30 req/s** durante **60 s a cada 7 min** |
| `range_cpu_usage_ratio` | gauge | uso de CPU, onda **0,5 ± 0,3** (período 5 min) |

```bash
curl -s localhost:8088/metrics | grep '^range_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-range   (troque entre 5m, 15m e 1h para ver os números mudarem)
```

---

## 🔍 Queries passo a passo

### 1. `range()` num gráfico

```promql
range()
```

**Resultado esperado:** linha **reta em 900** (15 min). Troque para "Last 1 hour" → **3600**; "Last 5 minutes" → **300**.

---

### 2. % da janela já percorrida

```promql
(time() - start()) / range()
```

**Resultado esperado:** rampa de **0%** (borda esquerda) a **100%** (borda direita). Útil como "barra de progresso" ou para ponderar pontos.

---

### 3. Média da janela visível

```promql
range_cpu_usage_ratio                                    # a onda
avg_over_time(range_cpu_usage_ratio[range()] @ end())    # média de tudo que está na tela
```

**Resultado esperado:** onda entre ~20% e ~80% e uma **linha reta em ≈ 50%**. Com zoom de 2 min (menos de meia onda), a média fica bem diferente de 50%, porque ela é **da janela visível**.

---

### 4. Total de requisições no período (stat com consulta range)

```promql
increase(range_http_requests_total[range()] @ end())
```

**O que faz:** `[range()]` = janela do tamanho do gráfico; `@ end()` ancora no fim. Resultado = total do período visível (com o tratamento de resets e extrapolação do `increase`).
**Resultado esperado** (15 min): 10/s × 900 s = **9 000** + cada pico traz 60 s × 20/s extra = **1 200**. Em 15 min cabem **2 ou 3** picos (um a cada 7 min) → entre ≈ **11 400** e ≈ **12 600**, dependendo de onde a janela caiu.

> 💡 O stat deste painel está configurado com **`instant: false`**: o Grafana faz uma consulta **range** e mostra o último ponto. Se fosse instantânea, `range()` seria 0 → erro (próximo passo).

---

### 5. ❌ O que dá errado: consulta instantânea

```promql
range()                                           # stat instantâneo → 0
increase(range_http_requests_total[range()])      # instantâneo → ERRO: duration must be greater than 0
```

**Resultado esperado:** o stat mostra **0**. A segunda nem roda (não está no dashboard justamente por isso). Regra de alerta com `[range()]` → a regra falha em toda avaliação.

---

### 6. (Comparação) com e sem `@ end()`

```promql
increase(range_http_requests_total[range()] @ end())   # linha reta
increase(range_http_requests_total[range()])           # curva
```

**Resultado esperado:** a primeira é uma **linha reta** (o total da tela, entre ≈ 11 400 e ≈ 12 600). A segunda fica quase sempre em ≈ **11 400** (2 picos na janela) com **lombadas triangulares** até ≈ **12 600** a cada 7 min: em cada ponto ela calcula os 15 min que **terminam naquele ponto**, então sobe enquanto um 3º pico **entra** na janela e desce enquanto o pico mais antigo **sai** por trás. A última posição da curva coincide com a linha reta (na borda direita, as duas janelas são a mesma).

---

### 7. (Comparação) `range()` vs `$__range` do Grafana

```promql
increase(range_http_requests_total[range()] @ end())     # PromQL puro (experimental)
increase(range_http_requests_total[$__range])            # Grafana substitui por "900s" antes de enviar
```

Os dois dão ≈ o mesmo número. `$__range` só existe **no Grafana** (não funciona na UI do Prometheus nem em regras); `range()` funciona em qualquer cliente que faça range query, mas é experimental.

---

## 🏭 Casos reais

### 1. "Disponibilidade no período selecionado" (SLO dashboard)

```promql
1 - (
  sum(increase(http_requests_total{job="api",code=~"5.."}[range()] @ end()))
  /
  sum(increase(http_requests_total{job="api"}[range()] @ end()))
)
```

O gerente seleciona "última semana" ou "último mês" e vê a disponibilidade **daquele período**. Para alertar, porém, SLOs usam janelas fixas e recording rules:

```yaml
groups:
  - name: slo-api
    rules:
      - record: job:http_errors:ratio_rate30d
        expr: |
          sum by (job) (increase(http_requests_total{code=~"5.."}[30d]))
            / sum by (job) (increase(http_requests_total[30d]))
      - alert: ErrorBudgetEsgotado
        expr: job:http_errors:ratio_rate30d{job="api"} > 0.001   # SLO 99,9%
        for: 1h
```

### 2. "Top 5 endpoints por volume no período"

```promql
topk(5, sum by (handler) (increase(http_requests_total[range()] @ end())))
```

Recording rule equivalente para relatórios diários:

```yaml
- record: handler:http_requests:increase1d
  expr: sum by (handler) (increase(http_requests_total[1d]))
```

---

## ✅ Quando usar

- **Totais/médias/máximos do período visível** em dashboards (`[range()] @ end()`).
- "% da janela" e normalizações pela duração.
- Clientes que não são Grafana (sem `$__range`), ex.: scripts chamando `/api/v1/query_range`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Alertas / recording rules | janela fixa (`[30d]`, `[1h]`) |
| Painel instantâneo (stat/tabela) | `$__range` do Grafana, ou `instant: false` |
| Resolução entre pontos | [`step()`](../step/) |
| Produção estável | `$__range` (não é experimental) |

## ⚠️ Pegadinhas

1. **Instant query → `range()` = 0** → `x[range()]` dá erro *"duration must be greater than 0"*.
2. **Sem `@ end()`**, `increase(x[range()])` num gráfico calcula, **em cada ponto**, a janela de 15 min **terminando naquele ponto** (uma curva, não uma constante; e a borda esquerda olha 15 min para trás, antes do gráfico).
3. **Experimental:** precisa da feature flag.
4. **Janelas grandes são caras:** `[range()]` num gráfico de 30 dias lê 30 dias de amostras em cada ponto (com `@ end()`, uma vez só; o Prometheus otimiza `@` para avaliar uma vez).

## 🎓 Na prova PCA

- Consulta **range** = várias avaliações instantâneas de `start` a `end` a cada `step`.
- **Range vector selector** (`x[5m]`) ≠ **range query** (API `query_range`): nomes parecidos, conceitos diferentes!
- `increase()` sobre janela → total no período; `$__range` é do Grafana.

**1.** Qual a diferença entre um *range vector* e uma *range query*?
- A) nenhuma
- B) range vector é um tipo de dado (`x[5m]`, várias amostras por série); range query é uma chamada de API que avalia uma expressão em vários instantes
- C) range query só aceita range vectors
- D) range vector só existe em range queries

<details><summary>Resposta</summary>

**B.** Você pode usar `x[5m]` numa instant query (ex.: `rate(x[5m])`) e uma range query pode avaliar expressões sem range vector (ex.: `up`).
</details>

**2.** O que o Grafana faz com `$__range` numa consulta?
- A) envia literalmente `$__range` para o Prometheus
- B) substitui pela duração do intervalo selecionado (ex.: `900s`) antes de enviar
- C) converte para `range()`
- D) usa como `step`

<details><summary>Resposta</summary>

**B.** É uma variável de template do Grafana, resolvida no cliente.
</details>

**3.** Qual expressão dá o total de requisições nas últimas 24 h, numa regra de alerta?
- A) `increase(http_requests_total[24h])`
- B) `increase(http_requests_total[range()])`
- C) `sum_over_time(http_requests_total[24h])`
- D) `http_requests_total - http_requests_total offset 24h`

<details><summary>Resposta</summary>

**A.** B falha (range() = 0 em avaliação instantânea); C soma os **valores** acumulados do counter (número sem sentido); D quebra com resets.
</details>

## 📝 Cola rápida

- `range()` = `end() - start()` = duração do gráfico; **0** em instant query.
- Total do período visível: `increase(x[range()] @ end())`.
- Média/máx. da tela: `avg_over_time(x[range()] @ end())`.
- Em stat: `instant: false` (ou `$__range`). Em alerta: janela fixa.

## 🔗 Relacionadas

[`start()`](../start/) · [`end()`](../end/) · [`step()`](../step/) · [`increase()`](../increase/) · [`avg_over_time()`](../avg_over_time/) · [`time()`](../time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#range
