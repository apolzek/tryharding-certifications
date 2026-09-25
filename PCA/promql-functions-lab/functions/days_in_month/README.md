# `days_in_month()`: quantos dias tem o mês (em UTC)

> **Em uma frase:** `days_in_month(v)` interpreta cada valor de `v` como timestamp Unix e devolve **quantos dias tem o mês** daquele instante (28, 29, 30 ou 31), em UTC. Sem argumento, usa o mês atual.

| | |
|---|---|
| **Assinatura** | `days_in_month(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (mês atual) ou gauge cujo **valor é timestamp em segundos** |
| **Unidade do resultado** | inteiro 28-31 |
| **Dashboard** | http://localhost:3300/d/fn-days_in_month |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: "30 dias tem novembro..."

Lembra da rima (ou de contar nos nós dos dedos) para saber quantos dias tem cada mês? `days_in_month()` faz isso por você, **inclusive o 29 de fevereiro** em ano bissexto.

Sozinha ela é quase uma curiosidade. O poder aparece na **conta de padaria** da projeção de custo:

```
                     gasto até agora
projeção do mês  =  ─────────────────  ×  dias no mês
                     dias já passados
```

Gastou 997 dólares em 25 dias → ~40/dia → num mês de **30** dias fecha em ~**1 200**. Se o mês tivesse 31 dias, ~1 240. Esse "30 ou 31" é o `days_in_month()`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `days_in_month_simulated_clock_timestamp_seconds` | gauge (timestamp) | **relógio acelerado**: 1 mês a cada **12 s**, jan/2027 → dez/2028 (ciclo de ~5 min) |
| `days_in_month_cloud_cost_month_to_date_dollars{team="platform"}` | gauge | custo acumulado no mês (UTC), ~**40 USD/dia** |
| `days_in_month_cloud_cost_month_to_date_dollars{team="data"}` | gauge | ~**25 USD/dia** |
| `days_in_month_cloud_budget_monthly_dollars{team}` | gauge | orçamento mensal: platform **1 000**, data **900** |
| `days_in_month_node_total_hourly_cost{node,instance_type}` | gauge | imita o **OpenCost**: 2 × m5.large (0,096 USD/h) + 1 × c5.2xlarge (0,34 USD/h) |

```bash
curl -s localhost:8088/metrics | grep '^days_in_month_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-days_in_month
```

---

## 🔍 Queries passo a passo

### 1. Relógio acelerado: a escada dos meses

```promql
days_in_month(days_in_month_simulated_clock_timestamp_seconds)
```

**Resultado esperado:** degraus de 12 s: **31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31** (2027) e depois o mesmo com **29** em fevereiro (2028 é bissexto). Os "vales" de fevereiro alternam entre 28 e 29.

---

### 2. Este mês

```promql
days_in_month()                    # 30 (setembro)
day_of_month()                     # 25
days_in_month() - day_of_month()   # 5 dias restantes
```

---

### 3. Custo acumulado (dado cru)

```promql
days_in_month_cloud_cost_month_to_date_dollars
```

**Resultado esperado:** duas rampas quase retas (15 min é pouco para ver a inclinação). Em 25/set ~22h UTC (~24,9 dias corridos): platform ≈ **997**, data ≈ **623**. No dia 1 do mês elas voltam a 0.

---

### 4. Projeção de fim de mês vs orçamento

```promql
# simples: usa o dia inteiro
days_in_month_cloud_cost_month_to_date_dollars / scalar(day_of_month()) * scalar(days_in_month())

# precisa: dias corridos com fração (dia-1 + hora/24 + minuto/1440)
days_in_month_cloud_cost_month_to_date_dollars
  / scalar(day_of_month() - 1 + hour() / 24 + minute() / 1440) * scalar(days_in_month())
```

**Resultado esperado** (25/set, 22h UTC):

| team | simples | precisa | orçamento |
|---|---|---|---|
| platform | ≈ **1 196** | ≈ **1 200** | 1 000 → **estoura** |
| data | ≈ **748** | ≈ **750** | 900 → ok |

A versão "simples" **subestima** porque conta o dia 25 como completo (25 dias) quando só se passaram 24,9. No começo do mês o erro é enorme: no dia 1 às 01h, a simples divide por 1 dia inteiro quando só passou 1 hora (projeção 24× menor!).

---

### 5. Pegadinha: sem `scalar()`, vazio

```promql
days_in_month_cloud_cost_month_to_date_dollars / day_of_month() * days_in_month()
```

**Resultado esperado:** **vazio**. `x{team="platform"} / day_of_month()` é vetor ÷ vetor: o Prometheus casa por labels, e `day_of_month()` não tem labels. Nada casa.

---

### 6. Estilo OpenCost: custo por hora → projeção mensal

```promql
sum(days_in_month_node_total_hourly_cost) * 24 * scalar(days_in_month())
```

**Resultado esperado:** (0,096 + 0,096 + 0,34) = 0,532 USD/h × 24 × **30** = **383,04 USD** em setembro. Em outubro (31 dias) daria 395,81. Note: `sum(...)` sem `by` também não tem labels, então aqui até funcionaria sem `scalar()`, mas com `sum by (team)` não.

---

## 🏭 Casos reais

### 1. FinOps: alerta de estouro do orçamento mensal (OpenCost / Kubecost)

O OpenCost expõe `node_total_hourly_cost`, `container_cpu_allocation` etc. Recording rule da projeção por namespace (a partir de uma métrica de custo acumulado):

```yaml
groups:
  - name: finops
    rules:
      - record: namespace:cost_month_projection_dollars
        expr: |
          namespace_cost_month_to_date_dollars
            / scalar(day_of_month() - 1 + hour() / 24)
            * scalar(days_in_month())
      - alert: OrcamentoMensalVaiEstourar
        expr: namespace:cost_month_projection_dollars > on(namespace) namespace_budget_monthly_dollars
        for: 6h
        labels: {severity: warning}
```

### 2. Custo mensal do cluster "se continuar assim"

```promql
sum(node_total_hourly_cost) * 24 * days_in_month()
```

Painel clássico "Monthly cost (projected)". Em fevereiro ele mostra naturalmente um valor ~10% menor que em janeiro, com a mesma infra.

### 3. Cota mensal de API externa (ex.: 1 M chamadas/mês)

```yaml
- alert: CotaMensalDaApiVaiEstourar
  expr: |
    external_api_calls_month_to_date
      / scalar(day_of_month() - 1 + hour() / 24)
      * scalar(days_in_month()) > 1e6
  for: 2h
  labels: {severity: warning}
```

(`increase(external_api_calls_total[30d])` é a aproximação "últimos 30 dias corridos", que não bate com o ciclo de cobrança do mês.)

---

## ✅ Quando usar

- **Projeções mensais** (custo, uso, cota) com o tamanho real do mês.
- "Quantos dias faltam no mês?": `days_in_month() - day_of_month()`.
- Converter taxa por hora/dia em total mensal.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Projeção por **tendência** (não linear, com sazonalidade) | [`predict_linear()`](../predict_linear/) |
| Total dos últimos 30 dias corridos | `increase(x[30d])` / `sum_over_time(x[30d])` |
| Dia atual | [`day_of_month()`](../day_of_month/) |

## ⚠️ Pegadinhas

1. **Vetor sem labels + métrica com labels = vazio.** Use `scalar()` (ou `on() group_left`).
2. **Divisão por `day_of_month()` subestima** no início do mês. Use dias corridos com fração.
3. **UTC:** o mês vira às 21h de Brasília no último dia. O "custo do mês" do seu provedor de cloud pode usar outro fuso (a AWS usa UTC; outros podem usar o fuso da conta).
4. **Dia 1 às 00h:** "dias corridos" = 0 → divisão por zero → `+Inf`.

## 🎓 Na prova PCA

- Faixa **28-31**, UTC, default `vector(time())`.
- Regras de **vector matching** (one-to-one por labels idênticos; `on()`, `ignoring()`, `group_left`).
- `scalar()` como "ponte" entre vetor sem labels e aritmética.

**1.** Quanto vale `days_in_month(vector(1709164800))` (29/fev/2024 00:00 UTC)?
- A) 28
- B) 29
- C) 30
- D) 31

<details><summary>Resposta</summary>

**B.** 2024 é bissexto; fevereiro tem 29 dias.
</details>

**2.** Qual expressão projeta corretamente o custo mensal por time a partir de `cost_mtd{team}`?
- A) `cost_mtd / day_of_month() * days_in_month()`
- B) `cost_mtd / scalar(day_of_month()) * scalar(days_in_month())`
- C) `rate(cost_mtd[30d]) * days_in_month()`
- D) `sum(cost_mtd) by (team) * days_in_month`

<details><summary>Resposta</summary>

**B.** A retorna vazio (labels não casam). C aplica `rate()` (feito para counters) num gauge que zera todo mês; D é uma armadilha: `days_in_month` **sem parênteses** é lido como um **seletor de métrica** chamada `days_in_month` (que não existe) → vazio, sem erro.
</details>

**3.** O que retorna `days_in_month() - day_of_month()` no dia 31 de janeiro?
- A) 0
- B) 1
- C) 30
- D) -1

<details><summary>Resposta</summary>

**A.** 31 − 31 = 0: é o último dia (em UTC).
</details>

## 📝 Cola rápida

- `days_in_month()` → 28/29/30/31, UTC.
- Projeção: `acumulado / scalar(dias_corridos) * scalar(days_in_month())`.
- Dias restantes: `days_in_month() - day_of_month()`.
- Custo/h → mês: `sum(custo_hora) * 24 * days_in_month()`.

## 🔗 Relacionadas

[`day_of_month()`](../day_of_month/) · [`day_of_year()`](../day_of_year/) · [`month()`](../month/) · [`hour()`](../hour/) · [`scalar()`](../scalar/) · [`predict_linear()`](../predict_linear/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#days_in_month
