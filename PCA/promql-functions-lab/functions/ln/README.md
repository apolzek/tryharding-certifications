# `ln()`: logaritmo natural (crescimento percentual, tempo de dobra, média geométrica)

> **Em uma frase:** `ln(v)` calcula o **logaritmo natural** (base e ≈ 2.718) de cada valor. Ele transforma **multiplicação em soma**: uma série que cresce **X% por segundo** vira uma **reta**, e daí dá pra medir a taxa de crescimento, o tempo de dobra e fazer médias geométricas.

| | |
|---|---|
| **Assinatura** | `ln(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões **positivos** · ⚠️ `0 → -Inf`, negativo → `NaN` · histogramas são ignorados |
| **Unidade do resultado** | adimensional ("nepers"); diferenças de `ln` são **variações relativas** |
| **Dashboard** | http://localhost:3300/d/fn-ln |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a régua que conta "quantas multiplicações"

Numa régua normal, os passos são de **somar**: 0, 1, 2, 3. Numa régua logarítmica, cada passo igual é um **multiplicar**:

```
 valor:     100      200      400      800     1600
 ln:        4.61     5.30     5.99     6.68    7.38
              └─+0.69─┘└─+0.69─┘└─+0.69─┘└─+0.69─┘
                    cada DOBRA soma ln(2) = 0.6931
```

Por isso uma **bola de neve** (algo que dobra sempre no mesmo intervalo) vira uma **reta** depois do `ln`. E a inclinação dessa reta é a **taxa de crescimento percentual**, a mesma do início ao fim, mesmo que no gráfico cru pareça que "de repente explodiu".

`ln` é o inverso do [`exp()`](../exp/): `ln(exp(x)) = x`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `ln_rabbitmq_queue_messages{queue="payments.retry"}` | `rabbitmq_queue_messages` | gauge | **tempestade de retries**: de **100 a 12 800** mensagens em 5 min (dobra a cada **~43 s**), depois é purgada |
| `ln_probe_duration_seconds{target=...}` | `probe_duration_seconds` (blackbox_exporter) | gauge | 4 sites em **20, 25, 30, 40 ms** (±5%) e o `legado` em **~8 s** |

A taxa contínua da tempestade é `r = ln(128) / 300 s ≈ 0.0162 /s`.

```bash
curl -s localhost:8088/metrics | grep '^ln_'
# ln_probe_duration_seconds{target="https://legado.exemplo.com"} 7.93
# ln_rabbitmq_queue_messages{queue="payments.retry"} 1617
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-ln
```

Espere **~5 minutos** para ver uma tempestade inteira.

---

## 🔍 Queries passo a passo

### 1 e 2. A exponencial crua e a reta do `ln`

```promql
ln_rabbitmq_queue_messages
ln(ln_rabbitmq_queue_messages)
```

**Resultado esperado:**
- **Cru:** por ~3 minutos a curva parece **plana perto do zero** (100, 200, 400...) e aí "explode" até **12 800**. Um olho humano diria "estava tudo bem e de repente quebrou".
- **`ln`:** uma **rampa reta** de `ln(100) = 4.61` até `ln(12800) = 9.46` (um "dente de serra" de retas, por causa das purgas). A fila estava crescendo **no mesmo ritmo percentual** desde o primeiro segundo.

| mensagens | `ln` |
|---|---|
| 100 | 4.61 |
| 200 | 5.30 |
| 1 600 | 7.38 |
| 12 800 | 9.46 |

---

### 3. Taxa de crescimento contínua

```promql
deriv(ln(ln_rabbitmq_queue_messages)[1m:5s])
```

**O que faz:** `deriv()` mede a inclinação da reta `ln(x)`. Como `ln(x)` é uma expressão, é preciso uma **subquery** `[1m:5s]` para gerar o range vector.
**Resultado esperado:** ≈ **0.0162 /s** (ou ~1.6% por segundo), **constante** durante toda a tempestade. Logo após cada purga, a janela pega a queda e o valor fica negativo por ~1 min.

---

### 4. Tempo de dobra

```promql
0.6931 / deriv(ln(ln_rabbitmq_queue_messages)[1m:5s]) > 0
```

**O que faz:** se a taxa contínua é `r`, o tempo para dobrar é `ln(2) / r`. O `> 0` filtra os momentos pós-purga (taxa negativa).
**Resultado esperado:** ≈ **43 s**. "A fila de retry dobra a cada 43 segundos" é uma frase que faz qualquer on-call agir.

> ⚠️ Por que `0.6931` e não `ln(2)`? Porque `ln(2)` com **escalar** é **erro de parse** no PromQL (as funções matemáticas só aceitam instant vector). Alternativas: o literal `0.6931` ou `scalar(ln(vector(2)))`.

---

### 5 e 6. Média geométrica: a média que não é sequestrada por outliers

```promql
avg(ln_probe_duration_seconds)                  # aritmética
exp(avg(ln(ln_probe_duration_seconds)))         # geométrica
quantile(0.5, ln_probe_duration_seconds)        # mediana, para comparação
```

**O que faz:** a média geométrica é a média **no espaço log**, convertida de volta com `exp`. Ela responde "qual é o fator típico?" em vez de "qual é a soma dividida por N?".
**Resultado esperado:**

| estatística | valor | leitura |
|---|---|---|
| média aritmética | ≈ **1.62 s** | "os sites estão lentos" (falso para 4 de 5) |
| média geométrica | ≈ **0.086 s** | "tipicamente dezenas de ms, com algo bem fora" |
| mediana | ≈ **0.030 s** | o site "do meio" |

---

### 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `ln(vector(1))` | **0** | e⁰ = 1 |
| `ln(exp(vector(1)))` | **1** | ln(e) = 1 |
| `ln(vector(2))` | **0.6931...** | "uma dobra" |
| `ln(vector(0))` | **-Inf** | doc: `ln(0) = -Inf` |
| `ln(vector(-1))` | **NaN** | doc: `ln(x < 0) = NaN` |
| `ln(vector(+Inf))` | **+Inf** | doc |
| `ln(vector(NaN))` | **NaN** | doc |

---

## 🏭 Casos reais

### 1. Tempestade de retries / crescimento percentual de uma fila

```promql
# taxa contínua de crescimento da fila (por segundo)
deriv(ln(rabbitmq_queue_messages{queue=~".*retry.*"})[5m:30s])
```

```yaml
- alert: FilaCrescendoExponencialmente
  expr: |
    deriv(ln(rabbitmq_queue_messages{queue=~".*retry.*"} > 0)[10m:30s]) > 0.001
      and rabbitmq_queue_messages{queue=~".*retry.*"} > 1000
  for: 5m
  annotations:
    summary: "Fila {{ $labels.queue }} crescendo {{ $value | humanizePercentage }} por segundo"
```

**Decisão:** um limite fixo (`> 10000`) dispararia tarde; a **taxa relativa** (`deriv(ln(x))`) pega a tempestade no início, quando ainda são 200 mensagens. O `> 0` dentro do `ln` evita `-Inf` quando a fila zera.

### 2. Média geométrica de latência entre endpoints (blackbox)

```promql
exp(avg by (job) (ln(probe_duration_seconds{job="blackbox-http"})))
```

```yaml
- record: job:probe_duration_seconds:geomean
  expr: exp(avg by (job) (ln(probe_duration_seconds{job="blackbox-http"} > 0)))
```

Um painel "latência típica dos sites" com média aritmética é dominado por um único site em timeout. A geométrica mostra a tendência geral; o outlier aparece num painel separado (`topk(3, probe_duration_seconds)`).

### 3. Variação relativa simétrica (log-ratio)

```promql
ln(
  sum(rate(http_requests_total[1h]))
  / sum(rate(http_requests_total[1h] offset 1w))
)
```

`+0.69` = dobrou em relação à semana passada; `-0.69` = caiu pela metade. Diferente da variação percentual (`+100%` vs `-50%`), o log-ratio é **simétrico**, o que facilita alertas de "mudou demais para qualquer lado": `abs(...) > 0.69`.

### 4. Escala logarítmica para comparar coisas muito diferentes

```promql
ln(sum by (job) (rate(http_requests_total[5m])))
```

Serviços com 0.1 req/s e 10 000 req/s cabem no mesmo gráfico. (Para leitura humana, prefira [`log10()`](../log10/) ou o eixo log do Grafana.)

---

## ✅ Quando usar

- **Taxa de crescimento percentual** e **tempo de dobra** de algo que cresce "como bola de neve".
- **Média geométrica** (`exp(avg(ln(x)))`) de latências, fatores, speedups.
- **Razões simétricas** (`ln(a / b)`) para comparar períodos ou ambientes.
- **Linearizar** uma exponencial antes de `deriv()` / `predict_linear()` e voltar com [`exp()`](../exp/).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer "ordem de grandeza" legível (1 ms, 10 ms, 100 ms) | [`log10()`](../log10/) |
| Quer "quantas dobras" / potências de 2 | [`log2()`](../log2/) |
| Só quer **visualizar** em escala log | eixo **Logarithmic** do Grafana (Axis → Scale) |
| Crescimento é linear (+N por hora) | [`deriv()`](../deriv/) / [`predict_linear()`](../predict_linear/) direto |
| O valor pode ser 0 ou negativo | filtre antes (`x > 0`) ou [`clamp_min(x, 1e-9)`](../clamp_min/) |

## ⚠️ Pegadinhas

1. **Zero vira `-Inf`** e quebra médias (`avg` com um `-Inf` = `-Inf`) e gráficos. Filtre: `ln(x > 0)`.
2. **Negativo vira `NaN`**, que "contamina" qualquer conta em que entrar.
3. **`ln(2)` com escalar é erro de parse.** Use `ln(vector(2))` ou o literal `0.6931`.
4. **Base errada:** `ln` é base **e**. Para base 10 use `log10`, para base 2 `log2`. Conversão: `log10(x) = ln(x) / 2.302585`.
5. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- Casos especiais **da doc**: `ln(+Inf) = +Inf`, `ln(0) = -Inf`, `ln(x < 0) = NaN`, `ln(NaN) = NaN` (e `log2`/`log10` seguem os mesmos).
- `ln` recebe **instant vector**; `ln(2)` sozinho é erro.
- Relação com `exp` (inversas) e com `log2`/`log10` (só muda a base).
- Subquery para aplicar `deriv` sobre `ln(x)`.

**1.** Quanto vale `ln(vector(0))`?

- A) 0
- B) NaN
- C) -Inf
- D) Erro de divisão por zero

<details><summary>Resposta</summary>

**C.** Pela documentação, `ln(0) = -Inf`. Para negativos seria NaN.
</details>

**2.** Qual expressão calcula a **média geométrica** de `probe_duration_seconds` entre todos os alvos?

- A) `avg(probe_duration_seconds)`
- B) `exp(avg(ln(probe_duration_seconds)))`
- C) `ln(avg(exp(probe_duration_seconds)))`
- D) `sqrt(avg(probe_duration_seconds ^ 2))`

<details><summary>Resposta</summary>

**B.** Média no espaço log, de volta com `exp`. D) é o RMS (média quadrática), que é ainda **mais** sensível a outliers.
</details>

**3.** Qual das expressões é **inválida**?

- A) `ln(node_load1)`
- B) `ln(vector(2))`
- C) `ln(2)`
- D) `ln(rate(http_requests_total[5m]))`

<details><summary>Resposta</summary>

**C.** `2` é escalar; `ln` exige instant vector.
</details>

**4.** Uma métrica cresce 1% por segundo, de forma composta. O que mostra `deriv(ln(x)[5m:10s])`?

- A) ≈ 0.01, constante
- B) Um valor que aumenta com o tempo
- C) ≈ 1
- D) ≈ ln(0.01)

<details><summary>Resposta</summary>

**A.** A inclinação de `ln(x)` é a taxa contínua de crescimento (`ln(1.01) ≈ 0.00995 /s`), constante para crescimento exponencial. A inclinação do valor **cru** é que aumentaria com o tempo (B).
</details>

## 📝 Cola rápida

- `ln(v)` = log base e; inverso do `exp`. `ln(1) = 0`, `ln(0) = -Inf`, `ln(<0) = NaN`, `ln(+Inf) = +Inf`.
- Exponencial vira reta: `deriv(ln(x)[janela:passo])` = taxa contínua `r`; tempo de dobra = `0.6931 / r`.
- Média geométrica: `exp(avg(ln(x)))`.
- `ln(2)` literal = erro de parse; funções matemáticas só aceitam instant vector.
- Filtre zeros antes: `ln(x > 0)`.

## 🔗 Relacionadas

[`exp()`](../exp/) · [`log2()`](../log2/) · [`log10()`](../log10/) · [`deriv()`](../deriv/) · [`predict_linear()`](../predict_linear/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#ln
