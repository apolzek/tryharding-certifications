# `day_of_year()`: dia do ano (1 a 365/366, em UTC)

> **Em uma frase:** `day_of_year(v)` interpreta cada valor de `v` como timestamp Unix e devolve o **dia do ano em UTC**: 1 em 1º de janeiro, **365** em 31 de dezembro (ou **366** em ano bissexto). Sem argumento, usa o instante avaliado ("hoje").

| | |
|---|---|
| **Assinatura** | `day_of_year(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (hoje) ou gauge cujo **valor é timestamp em segundos** |
| **Unidade do resultado** | inteiro 1-365 (1-366 em ano bissexto) |
| **Dashboard** | http://localhost:3300/d/fn-day_of_year |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a agenda anual com páginas numeradas

Sabe aquela **agenda de papel** com uma página por dia? `day_of_year()` responde **"em que página estamos?"**. 25 de setembro de 2026 é a página **268**.

A utilidade vem de comparar com o **total de páginas**: 268 / 365 = **73%** do ano já passou. Se você já gastou 88% do orçamento anual, está gastando rápido demais.

Anos bissextos têm **uma página a mais** (29 de fevereiro), e aí tudo depois de fevereiro "desliza" uma página: 25/set/2028 é a página **269**.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `day_of_year_simulated_clock_timestamp_seconds` | gauge (timestamp) | **relógio acelerado**: 2 dias simulados por segundo real, de 01/jan/2027 a 31/dez/2028 (ciclo de ~6 min) |
| `day_of_year_annual_budget_dollars{team}` | gauge | orçamento anual: platform **120 000**, data **60 000** USD |
| `day_of_year_budget_spent_year_to_date_dollars{team}` | gauge | gasto acumulado desde 1º/jan (UTC): platform gasta **20% acima** do ritmo, data **10% abaixo** |

```bash
curl -s localhost:8088/metrics | grep '^day_of_year_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-day_of_year
```

---

## 🔍 Queries passo a passo

### 1. Relógio acelerado: 365 e depois 366

```promql
day_of_year(day_of_year_simulated_clock_timestamp_seconds)
```

**Resultado esperado:** dente-de-serra: sobe de 1 até **365** (2027) em ~3 min, cai para 1 e sobe até **366** (2028, bissexto), e recomeça. Dá para ver no painel que o segundo pico é 1 unidade mais alto.

---

### 2. Hoje e % do ano

```promql
day_of_year()                 # 268 em 25/set/2026
day_of_year() / 365 * 100     # ≈ 73,4 %
```

---

### 3. Orçamento: % gasto vs % do ano

```promql
day_of_year_budget_spent_year_to_date_dollars / day_of_year_annual_budget_dollars
day_of_year() / 365
```

**O que faz:** a primeira divide gasto por orçamento **casando os labels** (`team`), porque as duas métricas têm os mesmos labels.
**Resultado esperado** (25/set, ~22h UTC, ~267,9 dias corridos):

| série | valor |
|---|---|
| ano decorrido | ≈ **73%** |
| platform gasto | ≈ **88%** (1,2 × 73%) → gastando rápido demais |
| data gasto | ≈ **66%** (0,9 × 73%) → folga |

---

### 4. Projeção de fim de ano

```promql
day_of_year_budget_spent_year_to_date_dollars / scalar(day_of_year()) * 365
```

**O que faz:** gasto médio por dia × 365.
**Resultado esperado:** platform ≈ **144 000** (orçamento 120 000 → estoura ~24 000), data ≈ **54 000** (cabe nos 60 000).

> ⚠️ Por que `scalar(day_of_year())`? Veja o próximo passo.

---

### 5. ❌ O que dá errado: sem `scalar()`

```promql
day_of_year_budget_spent_year_to_date_dollars / day_of_year() * 365
```

**Resultado esperado:** **vazio** ("No data"). `x{team="platform"} / day_of_year()` tenta casar labels: um lado tem `team`, o outro não tem nenhum → **resultado vazio**. `scalar()` transforma o vetor sem labels num número, e "vetor / escalar" sempre funciona.

---

## 🏭 Casos reais

### 1. FinOps: burn rate do orçamento anual de cloud

Com o custo exportado (OpenCost, Kubecost, ou um exporter de billing):

```yaml
- record: team:cloud_budget_burn_ratio
  expr: |
    (cloud_cost_year_to_date_dollars / cloud_annual_budget_dollars)
      / scalar(day_of_year() / 365)
- alert: OrcamentoAnualEmRisco
  expr: team:cloud_budget_burn_ratio > 1.1     # gastando 10% acima do ritmo
  for: 1d
  labels: {severity: warning, team: finops}
```

Um `burn_ratio` de 1,0 = no ritmo; 1,2 = vai gastar 20% a mais que o orçado.

### 2. Cota anual de uso (ex.: minutos de CI, e-mails enviados)

```yaml
- alert: CotaAnualDeCIVaiEstourar
  expr: |
    ci_minutes_used_year_to_date / scalar(day_of_year()) * 365
      > on(org) ci_minutes_annual_quota
  for: 1d
  labels: {severity: warning}
```

### 3. Sazonalidade: "estamos na semana da Black Friday?"

```promql
# Black Friday 2026 = 27/nov = dia 331
shop_orders_rate and on() (day_of_year() >= 328 <= 334)
```

(Para feriados móveis, prefira uma métrica "calendário" publicada por um exporter; o PromQL não sabe feriados.)

---

## ✅ Quando usar

- **% do ano decorrido** para metas, cotas e orçamentos anuais.
- Projeções anuais simples (`acumulado / dia_do_ano * 365`).
- Janelas sazonais fixas por data.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Duração em dias entre dois instantes | `(time() - x) / 86400` |
| Comparar com o mesmo período do ano passado | `x offset 52w` (ou `365d`) |
| Metas mensais | [`day_of_month()`](../day_of_month/) + [`days_in_month()`](../days_in_month/) |

## ⚠️ Pegadinhas

1. **Ano bissexto:** 366 dias. Um `/ 365` fixo erra ~0,3% em 2028. Se precisar exato: `day_of_year(vector(<timestamp de 31/dez>))` dá 365 ou 366.
2. **UTC:** a virada do ano (e de cada dia) acontece às 21h de Brasília.
3. **`scalar()` para combinar com métricas com labels**, ou `on() group_left`.
4. **É um inteiro**: o dia 268 às 00:01 e às 23:59 vale igual. Para precisão sub-diária some `hour()/24`.

## 🎓 Na prova PCA

- Faixa **1-365/366**, UTC, default `vector(time())`.
- Casamento de labels em operações binárias: vetor com labels ÷ vetor sem labels = vazio → `scalar()` ou `on()`.
- `scalar(v)`: vira escalar se `v` tem **exatamente 1** elemento; senão `NaN`.

**1.** Qual o valor de `day_of_year()` em 31 de dezembro de 2028?
- A) 364
- B) 365
- C) 366
- D) 0

<details><summary>Resposta</summary>

**C.** 2028 é bissexto: 366 dias.
</details>

**2.** `spent_dollars{team="a"} / day_of_year()` retorna vazio. Por quê?
- A) `day_of_year()` retorna escalar
- B) os labels dos dois lados não casam (um tem `team`, o outro não tem labels)
- C) divisão por inteiro não é permitida
- D) `day_of_year()` precisa de argumento

<details><summary>Resposta</summary>

**B.** É vetor ÷ vetor com matching por labels. Correções: `/ scalar(day_of_year())` ou `/ on() group_left day_of_year()`.
</details>

**3.** Qual é o valor de `scalar(v)` quando `v` tem 3 elementos?
- A) a soma deles
- B) o primeiro
- C) NaN
- D) erro

<details><summary>Resposta</summary>

**C.** `scalar()` só converte vetores de exatamente 1 elemento; caso contrário devolve `NaN`.
</details>

## 📝 Cola rápida

- `day_of_year()` → 1-365 (366 bissexto), UTC.
- % do ano: `day_of_year() / 365`.
- Projeção anual: `acumulado / scalar(day_of_year()) * 365`.
- Vetor com labels ÷ `day_of_year()` = vazio → use `scalar()`.

## 🔗 Relacionadas

[`day_of_month()`](../day_of_month/) · [`days_in_month()`](../days_in_month/) · [`month()`](../month/) · [`year()`](../year/) · [`scalar()`](../scalar/) · [`time()`](../time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#day_of_year
