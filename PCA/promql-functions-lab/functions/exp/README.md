# `exp()`: e elevado a x (crescimento composto e o inverso do `ln`)

> **Em uma frase:** `exp(v)` calcula **eˣ** (com `e ≈ 2.71828`) para cada valor. Serve para duas coisas no dia a dia: **projetar crescimento composto** (`valor × exp(taxa × tempo)`) e **desfazer um logaritmo natural** (`exp(ln(x)) = x`).

| | |
|---|---|
| **Assinatura** | `exp(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (taxas de crescimento, valores em escala log) · histogramas são ignorados |
| **Unidade do resultado** | **adimensional** (um fator multiplicativo) ou a unidade "original" antes do `ln` |
| **Dashboard** | http://localhost:3300/d/fn-exp |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: juros compostos (e a bola de neve)

Uma aplicação rende **10% ao mês**. Depois de 12 meses você não tem `1 + 12 × 0.10 = 2.2×`, tem `1.10¹² ≈ 3.14×`, porque os juros rendem juros. Se o rendimento for **contínuo** (a cada instante), o fator vira:

```
 fator = exp(taxa × tempo)          exp(0.10 × 12) = exp(1.2) ≈ 3.32×

 x(t) = x(0) · exp(r · t)
         │         │
         │         └─ r = taxa contínua de crescimento (por segundo, por dia...)
         └─ valor de hoje
```

É o modelo de uma **bola de neve**: quanto maior, mais rápido cresce. Explosão de cardinalidade, crescimento de usuários, filas que se realimentam (retries) têm esse formato. Uma **reta** (`predict_linear`) sempre vai **subestimar** uma bola de neve.

`exp` é também o **"desfazer"** do `ln`: se alguém guardou um valor em escala logarítmica, `exp()` traz de volta.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `exp_prometheus_tsdb_head_series{pod="prometheus-0"}` | `prometheus_tsdb_head_series` | gauge | **explosão de cardinalidade**: de **10 000 a 160 000** séries em 10 min (dobra a cada 2.5 min), depois volta a 10 000 |
| `exp_fraud_model_score_logit{model="antifraude-v3"}` | saída de um modelo de regressão logística | gauge | **log-odds** oscilando entre **-5 e +5** (4 min) |

A taxa contínua da explosão é `r = ln(16) / 600 s ≈ 0.00462 /s`.

```bash
curl -s localhost:8088/metrics | grep '^exp_'
# exp_fraud_model_score_logit{model="antifraude-v3"} 2.31
# exp_prometheus_tsdb_head_series{pod="prometheus-0"} 18877
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-exp
```

Espere **~10 minutos** para ver uma explosão inteira e **~3 min** para o painel 3 (ele compara com uma previsão feita 2 min antes).

---

## 🔍 Queries passo a passo

### 1. A explosão de cardinalidade

```promql
exp_prometheus_tsdb_head_series
```

**Resultado esperado:** uma curva que começa quase plana em **10 000** e vai "empinando" até **160 000**, e então despenca de volta (o time removeu o label com o `user_id`). Não é uma reta: a cada 2.5 min o número **dobra** (10k → 20k → 40k → 80k → 160k).

---

### 2. Fator de crescimento em 2 minutos

```promql
exp(deriv(ln(exp_prometheus_tsdb_head_series)[1m:5s]) * 120)
```

**O que faz, de dentro pra fora:**
1. `ln(x)` transforma a curva exponencial numa **reta** (ver [`ln()`](../ln/)).
2. `deriv(...[1m:5s])` mede a inclinação dessa reta = a **taxa contínua** `r` ≈ **0.00462 /s** (subquery porque `ln(x)` é uma expressão).
3. `exp(r × 120)` converte a taxa em **fator multiplicativo** para 120 s.

**Resultado esperado:** uma linha em ≈ **1.74** (`exp(0.5545)`): "daqui a 2 minutos teremos **74% mais séries**". Logo depois da queda (reset do ciclo), o valor despenca por ~1 min, porque a janela pega a queda.

---

### 3. Previsão exponencial × linear

```promql
exp_prometheus_tsdb_head_series                                                                           # real
exp_prometheus_tsdb_head_series offset 2m * exp(deriv(ln(exp_prometheus_tsdb_head_series)[1m:5s] offset 2m) * 120)   # previsto com exp
predict_linear(exp_prometheus_tsdb_head_series[1m] offset 2m, 0)                                        # previsto com reta
```

No dashboard, as duas previsões ganham um `and (exp_prometheus_tsdb_head_series > exp_prometheus_tsdb_head_series offset 3m)`: elas só aparecem quando **não houve limpeza nos últimos 3 min** (senão a janela da previsão pega a queda e o número fica absurdo, até negativo no caso da reta).

**O que faz:** as duas previsões usam **dados de 2 minutos atrás** (`offset 2m`) e projetam **para agora**. Assim dá pra comparar a previsão com o que realmente aconteceu.

> ⚠️ **Pegadinha do `predict_linear` com `offset`:** o horizonte é contado a partir do **instante de avaliação** (agora), e não do fim da janela deslocada. Por isso é `predict_linear(x[1m] offset 2m, 0)` ("valor agora, usando a reta de 2 min atrás"). Com `, 120)` ele preveria **daqui a 2 min** (4 min à frente dos dados).
**Resultado esperado:**

| momento | real | previsto com `exp` | previsto com `predict_linear` |
|---|---|---|---|
| início | ~26 000 | **~26 000** ✅ | ~22 400 (-15%) |
| meio da explosão | ~40 000 | **~40 000** ✅ | ~34 000 (-15%) |
| perto do pico | ~159 000 | **~159 000** ✅ | ~136 000 (-15%) |

A linha exponencial **gruda** na real (erro < 0.01%); a linear fica sistematicamente **~15% abaixo**, e em valores absolutos o erro cresce junto com a explosão (23 000 séries a menos perto do pico). (Por isso o filtro: logo após a queda, as previsões feitas 2 min antes ainda projetam a explosão que já acabou, e o `predict_linear` com a queda dentro da janela chega a prever valores **negativos**.)

---

### 4 e 5. De log-odds para probabilidade (a sigmoide)

```promql
exp_fraud_model_score_logit                         # -5 .. +5
1 / (1 + exp(-exp_fraud_model_score_logit))         # 0 .. 1
```

**O que faz:** modelos de regressão logística trabalham em **log-odds** (`ln(p / (1-p))`). Para voltar para probabilidade, usa-se a função **sigmoide**, que tem um `exp` dentro.
**Resultado esperado:**

| logit | `exp(-logit)` | probabilidade |
|---|---|---|
| -5 | 148.4 | **0.7%** |
| 0 | 1 | **50%** |
| +2.3 | 0.10 | **~91%** |
| +5 | 0.0067 | **99.3%** |

A curva de probabilidade é uma onda **achatada** perto de 0% e 100% ("saturação" da sigmoide).

---

### 6. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `exp(vector(0))` | **1** | e⁰ = 1 (crescimento zero = fator 1) |
| `exp(vector(1))` | **2.718281828459045** | o próprio e |
| `exp(ln(vector(42)))` | **42.00000000000001** | inverso do `ln` (com erro de ponto flutuante) |
| `exp(vector(-Inf))` | **0** | |
| `exp(vector(+Inf))` | **+Inf** | doc: `Exp(+Inf) = +Inf` |
| `exp(vector(710))` | **+Inf** ⚠️ | **overflow**: e⁷¹⁰ passa do maior float64 (~1.8e308) |
| `exp(vector(NaN))` | **NaN** | doc: `Exp(NaN) = NaN` |

---

## 🏭 Casos reais

### 1. Explosão de cardinalidade: "em quanto tempo o Prometheus estoura a memória?"

Um deploy adicionou um label `user_id` e `prometheus_tsdb_head_series` começou a crescer como bola de neve. O time de observabilidade projeta **exponencialmente**:

```promql
# séries projetadas para daqui a 1 hora, usando a taxa contínua da última meia hora
prometheus_tsdb_head_series
  * exp(deriv(ln(prometheus_tsdb_head_series)[30m:1m]) * 3600)
```

```yaml
- alert: CardinalidadeExplodindo
  expr: |
    prometheus_tsdb_head_series
      * exp(deriv(ln(prometheus_tsdb_head_series)[30m:1m]) * 3600) > 5e6
  for: 10m
  labels:
    severity: critical
  annotations:
    summary: "Na taxa atual, {{ $labels.instance }} terá > 5M séries em 1h"
```

**Decisão:** `predict_linear` seria otimista demais numa explosão (painel 3). Para crescimento **percentual constante**, projete no espaço log e volte com `exp`.

### 2. Crescimento de armazenamento "X% ao mês"

```promql
(node_filesystem_size_bytes{mountpoint="/data"} - node_filesystem_avail_bytes{mountpoint="/data"})
  * exp(0.15 * 3)       # taxa contínua de 15%/mês × 3 meses
```

O capacity planning do banco de dados assume crescimento contínuo de ~15% ao mês. Os parênteses são obrigatórios (`*` tem precedência sobre `-`). `exp(0.15 × 3) ≈ 1.57`: em 3 meses, 57% a mais. Linearmente seriam só 45%.

### 3. Desfazer uma recording rule em escala log (média geométrica)

```yaml
- record: job:request_latency_seconds:log_avg5m
  expr: avg by (job) (ln(request_latency_seconds))
```

```promql
exp(job:request_latency_seconds:log_avg5m)     # média geométrica, de volta em segundos
```

Latências variam em ordens de grandeza; a **média geométrica** (`exp(avg(ln(x)))`) não é dominada por um outlier de 30 s. Ver [`ln()`](../ln/).

### 4. Probabilidade a partir de modelos de ML

Serviços de scoring (antifraude, churn) às vezes exportam o **logit**. O dashboard converte com `1 / (1 + exp(-logit))` (painel 5), e o alerta pode ser escrito em probabilidade, que o time de negócio entende:

```promql
avg_over_time((1 / (1 + exp(-fraud_model_score_logit)))[15m:1m]) > 0.8
```

---

## ✅ Quando usar

- **Projetar crescimento composto** (cardinalidade, usuários, dados): `x * exp(r * t)`.
- **Voltar da escala log** (`exp(ln(x))`), inclusive **média geométrica**: `exp(avg(ln(x)))`.
- **Sigmoide / probabilidade** a partir de log-odds: `1 / (1 + exp(-x))`.
- **Decaimento exponencial** (meia-vida, "esquecimento"): `x * exp(-t / tau)`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| O crescimento é **constante em unidades** (+1 GB/dia) | [`predict_linear()`](../predict_linear/) |
| Quer "elevar a uma potência" qualquer (x², 2ˣ) | o operador `^` (`x ^ 2`, `2 ^ x`) |
| Quer só suavizar/prever série com tendência | [`double_exponential_smoothing()`](../double_exponential_smoothing/) |
| Quer a taxa de um counter | [`rate()`](../rate/) |

## ⚠️ Pegadinhas

1. **Overflow:** `exp(x)` com x > ~709.78 vira **+Inf**. Cuidado com unidades: `exp(r * t)` com `r` por segundo e `t` em segundos (não misture dias com segundos).
2. **`exp` amplifica erros:** um erro pequeno na taxa `r` vira um erro enorme na projeção de longo prazo. Projete horizontes curtos.
3. **Precisão:** `exp(ln(42)) = 42.00000000000001`. Não compare com `==`.
4. **`exp` não é potência genérica:** `2ˣ` é `2 ^ x`, não `exp(x)`. (`2 ^ x = exp(x * ln(2))`.)
5. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- `exp()` recebe **instant vector**; é o inverso de `ln()`. Casos especiais da doc: `exp(+Inf) = +Inf`, `exp(NaN) = NaN`.
- Diferença entre `exp(x)` (eˣ) e o operador `^` (`a ^ b`).
- Saber que `predict_linear` assume crescimento **linear**.
- Subquery para aplicar `deriv`/`_over_time` sobre uma expressão (`ln(x)`).

**1.** Qual expressão retorna o valor original `x` (a menos de erro de ponto flutuante)?

- A) `ln(exp(x) - 1)`
- B) `exp(ln(x))`
- C) `exp(x) / e`
- D) `log10(exp(x))`

<details><summary>Resposta</summary>

**B.** `exp` e `ln` são funções inversas (para x > 0). D) daria `x / ln(10) ≈ 0.434 x`.
</details>

**2.** Quanto vale `exp(vector(0))`?

- A) 0
- B) 1
- C) 2.718
- D) NaN

<details><summary>Resposta</summary>

**B.** Qualquer número elevado a zero é 1: e⁰ = 1.
</details>

**3.** Uma métrica cresce **10% a cada hora** (crescimento composto). Qual abordagem prevê melhor o valor daqui a 6 horas?

- A) `predict_linear(x[1h], 6 * 3600)`
- B) `x * exp(deriv(ln(x)[1h:1m]) * 6 * 3600)`
- C) `x + rate(x[1h]) * 6`
- D) `exp(x) * 6`

<details><summary>Resposta</summary>

**B.** No espaço log o crescimento composto vira uma reta; `deriv(ln(x))` estima a taxa contínua e `exp(r·t)` volta para fator multiplicativo. A) extrapola uma reta e subestima. C) usa `rate` num gauge. D) não faz sentido.
</details>

**4.** Qual é o resultado de `exp(vector(1000))`?

- A) Erro de overflow
- B) `+Inf`
- C) `NaN`
- D) `1.8e308`

<details><summary>Resposta</summary>

**B.** O float64 não representa e¹⁰⁰⁰, então o resultado é `+Inf`, sem erro.
</details>

## 📝 Cola rápida

- `exp(v)` = eˣ; inverso de `ln`: `exp(ln(x)) = x`.
- `exp(0) = 1`, `exp(-Inf) = 0`, `exp(+Inf) = +Inf`, `exp(NaN) = NaN`, `exp(>709) = +Inf`.
- Crescimento composto: `x * exp(r * t)` com `r = deriv(ln(x)[janela:passo])`.
- Média geométrica: `exp(avg(ln(x)))`. Sigmoide: `1 / (1 + exp(-x))`.
- Potência genérica é `^`, não `exp`.

## 🔗 Relacionadas

[`ln()`](../ln/) · [`log2()`](../log2/) · [`log10()`](../log10/) · [`predict_linear()`](../predict_linear/) · [`deriv()`](../deriv/) · [`double_exponential_smoothing()`](../double_exponential_smoothing/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#exp
