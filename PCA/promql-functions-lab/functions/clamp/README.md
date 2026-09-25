# `clamp()`: prender o valor numa faixa [min, max]

> **Em uma frase:** `clamp(v, min, max)` limita cada valor a ficar **entre `min` e `max`**: o que está abaixo vira `min`, o que está acima vira `max`, o resto passa intacto. É a combinação de [`clamp_min()`](../clamp_min/) e [`clamp_max()`](../clamp_max/) numa função só.

| | |
|---|---|
| **Assinatura** | `clamp(v instant-vector, min scalar, max scalar) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (porcentagens, razões, orçamentos) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-clamp |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o batente da porta

A porta abre e fecha livremente, mas **nunca passa do batente**, nem para dentro nem para fora.

```
   valor:   -8     0    30    55    99   100   108
             │     │                       │     │
  clamp:     0     0    30    55    99   100   100
             ↑ batente de baixo      batente de cima ↑
             (min = 0)                  (max = 100)
```

Formalmente: `clamp(v, min, max) = clamp_max(clamp_min(v, min), max)`, ou `min(max(v, min_), max_)`.

Três regras especiais **da documentação**:
- `min > max` → **vetor vazio** (nenhuma série volta);
- `min` ou `max` = `NaN` → resultado **NaN**;
- `min = -Inf` e `max = +Inf` → valores **inalterados**.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `clamp_sensor_relative_humidity_percent{sensor="sala-1"}` | sensor ambiental (SNMP/Modbus exporter) | gauge | **mal calibrado**: oscila entre **-8% e 108%** (3 min) |
| `clamp_sensor_relative_humidity_percent{sensor="sala-2"}` | idem | gauge | sensor bom: **50% a 70%** |
| `clamp_http_requests_total{code="200"\|"500"}` | `http_requests_total` | counter | **100 req/s** no total; a fração de `500` oscila entre **0% e 0.3%** (4 min) |

Premissa do caso real: o serviço tem **SLO de 99.9%** de sucesso, ou seja, um **orçamento de erro de 0.1%**.

```bash
curl -s localhost:8088/metrics | grep '^clamp_[hs]'
# clamp_http_requests_total{code="500"} 312.4
# clamp_sensor_relative_humidity_percent{sensor="sala-1"} 104.2
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-clamp
```

Espere **~3 minutos** para ver um ciclo do sensor e **~4 min** para um ciclo da taxa de erro.

---

## 🔍 Queries passo a passo

### 🟢 1 e 2. Sensor fora da faixa física

```promql
clamp_sensor_relative_humidity_percent
clamp(clamp_sensor_relative_humidity_percent, 0, 100)
```

**Resultado esperado:**

| sensor | cru | `clamp(x, 0, 100)` |
|---|---|---|
| sala-1 | onda de **-8 a 108** | **platôs** em **0** (fundo) e **100** (topo), onda normal no meio |
| sala-2 | 50 a 70 | **igual** ao cru (já está na faixa) |

Exemplos pontuais: `108 → 100`, `-8 → 0`, `55 → 55`.

---

### 🟡 3. O que o `clamp` esconde

```promql
clamp_sensor_relative_humidity_percent < 0
  or clamp_sensor_relative_humidity_percent > 100
```

**O que faz:** mostra só as leituras **impossíveis** (filtro de comparação + `or`).
**Resultado esperado:** pontos apenas da **sala-1**, nos trechos em que ela passa de 100 (até 108) ou fica abaixo de 0 (até -8): cerca de **17% do tempo em cada lado**, um terço do tempo no total. No painel 2 esses mesmos momentos apareciam como "0%" e "100%" perfeitamente plausíveis.

> ⚠️ **Moral:** use `clamp` para **exibir**, mas **alerte no valor cru**. Um sensor que reporta 108% está quebrado, e o `clamp` apagaria essa evidência.

---

### 🔴 4. Taxa de erro × orçamento

```promql
sum(rate(clamp_http_requests_total{code="500"}[1m])) / sum(rate(clamp_http_requests_total[1m]))
vector(0.001)
```

**Resultado esperado:** uma onda entre **0% e 0.3%** e uma linha reta em **0.1%**. Metade do tempo a taxa de erro está acima do orçamento.

---

### 🔴 5 e 6. Orçamento restante, cru × `clamp(…, 0, 1)`

```promql
1 - (taxa_de_erro) / 0.001                 # cru
clamp(1 - (taxa_de_erro) / 0.001, 0, 1)    # 0% a 100%
```

(onde `taxa_de_erro` é a expressão do painel 4)

**O que faz:** "quanto do orçamento **sobrou**" = `1 - consumido / orçamento`. Se a taxa de erro é 3× o orçamento, o cru dá `1 - 3 = -2` (**-200%**), um número que ninguém sabe ler num gauge.
**Resultado esperado:**

| taxa de erro | cru | `clamp(…, 0, 1)` |
|---|---|---|
| 0% | **100%** | **100%** |
| 0.05% | **50%** | **50%** |
| 0.1% | **0%** | **0%** |
| 0.3% | **-200%** | **0%** |

O gauge do painel 6 fica sempre entre 0% e 100% ("tanque de combustível").

---

### ⚠️ 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `clamp(vector(108), 0, 100)` | **100** | acima do máximo |
| `clamp(vector(-8), 0, 100)` | **0** | abaixo do mínimo |
| `clamp(vector(55), 0, 100)` | **55** | dentro da faixa |
| `clamp(vector(5), -Inf, +Inf)` | **5** | doc: inalterado com -Inf/+Inf |
| `clamp(vector(5), NaN, 10)` | **NaN** | doc: `min` ou `max` NaN → NaN |
| `clamp(vector(NaN), 0, 100)` | **NaN** | NaN na entrada continua NaN |
| `clamp(vector(5), 100, 0)` | **vazio** ⚠️ | doc: `min > max` → vetor vazio (o painel não mostra nada) |

---

## 🏭 Casos reais

### 1. Gauge de "orçamento de erro restante" (SLO)

O time de SRE mostra no painel do serviço quanto do orçamento de erro do mês ainda resta. Com uma janela de 30 dias:

```yaml
groups:
  - name: slo-checkout
    rules:
      - record: slo:error_ratio:rate30d
        expr: |
          sum(rate(http_requests_total{service="checkout", code=~"5.."}[30d]))
          / sum(rate(http_requests_total{service="checkout"}[30d]))

      - record: slo:error_budget_remaining:ratio
        expr: clamp(1 - slo:error_ratio:rate30d / (1 - 0.999), 0, 1)
```

**Decisão:** o `clamp` é para o **gauge** (0..100%). O **alerta** de burn rate usa a taxa de erro crua (`slo:error_ratio:rate1h > 14.4 * 0.001`), que não perde informação quando o orçamento já acabou.

### 2. Sensores físicos com faixa conhecida

```yaml
- record: room:humidity_percent:clamped
  expr: clamp(sensor_relative_humidity_percent, 0, 100)

- alert: SensorUmidadeDescalibrado
  expr: sensor_relative_humidity_percent < -2 or sensor_relative_humidity_percent > 102
  for: 10m
  annotations:
    summary: "Sensor {{ $labels.sensor }} lendo {{ $value }}% (fisicamente impossível)"
```

A recording rule "limpa" alimenta os dashboards e outras regras; o alerta no **cru** pega o sensor quebrado (painel 3).

### 3. Normalizar um "score" para 0..1

```promql
clamp(
  (sum by (instance) (rate(node_cpu_seconds_total{mode!="idle"}[5m]))
    / count by (instance) (node_cpu_seconds_total{mode="idle"})),
  0, 1
)
```

Utilização de CPU pode sair levemente fora de 0..1 por causa da extrapolação do `rate()` e de contadores com jitter (`1.003`). Para um heatmap/gauge com escala fixa, o `clamp(..., 0, 1)` remove esses artefatos.

### 4. Progresso estimado de um job

```promql
clamp(batch_rows_processed / batch_rows_total, 0, 1)
```

Se `batch_rows_total` é uma **estimativa** (e o job processa um pouco mais que o estimado), o progresso passaria de 100%. O `clamp` mantém a barra de progresso honesta.

---

## ✅ Quando usar

- **Gauges e barras** com escala fixa (0..100%, 0..1): orçamento de erro, progresso, utilização.
- **Faixas físicas** conhecidas (umidade 0..100, bateria 0..100) em recording rules "limpas".
- **Remover artefatos** de extrapolação (`rate` dando 1.003 de utilização).
- **Proteger contas seguintes** (ex.: antes de `ln`, `sqrt`) contra valores fora do domínio.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só precisa de **um** limite | [`clamp_min()`](../clamp_min/) ou [`clamp_max()`](../clamp_max/) |
| Quer **descartar** (e não achatar) valores fora da faixa | filtro: `x >= 0 and x <= 100` |
| Quer **alertar** quando sai da faixa | comparação no valor **cru** |
| Só quer limitar o eixo do gráfico | *Min/Max* do painel no Grafana |

## ⚠️ Pegadinhas

1. **`min > max` → vazio**, sem erro. Um typo (`clamp(x, 100, 0)`) faz o painel ficar "No data".
2. **`clamp` esconde anomalias:** 108% e 100% viram a mesma coisa. Alerte no cru (painel 3).
3. **`min`/`max` são escalares:** `clamp(x, 0, other_metric)` é erro de tipo. Use `scalar(...)` ou operadores com matching (`x < y` e `min by`) para limites por série.
4. **NaN não é "consertado":** `clamp(NaN, 0, 1) = NaN`.
5. **Agregar depois do clamp muda o resultado:** `avg(clamp(x, 0, 100))` ≠ `clamp(avg(x), 0, 100)`.
6. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- Assinatura: `clamp(v instant-vector, min scalar, max scalar)` → 3 argumentos obrigatórios.
- **Casos especiais da doc**: `min > max` → vazio; `min`/`max` NaN → NaN; `-Inf`/`+Inf` → inalterado.
- Equivalência: `clamp(v, a, b) = clamp_max(clamp_min(v, a), b)`.

**1.** O que retorna `clamp(node_load1, 10, 1)`?

- A) Todos os valores iguais a 1
- B) Todos os valores iguais a 10
- C) Um vetor vazio
- D) Erro de parse

<details><summary>Resposta</summary>

**C.** Pela documentação, se `min > max` o resultado é um vetor vazio (sem erro).
</details>

**2.** Qual expressão é **equivalente** a `clamp(x, 0, 1)`?

- A) `clamp_min(clamp_max(x, 0), 1)`
- B) `clamp_max(clamp_min(x, 0), 1)`
- C) `min(max(x, 0), 1)` (agregadores)
- D) `x > 0 and x < 1`

<details><summary>Resposta</summary>

**B.** Primeiro garante o piso 0, depois o teto 1. A) inverte os limites (daria 1 sempre). C) `min`/`max` são **agregadores** de séries, não funções de dois argumentos. D) **filtra** em vez de limitar.
</details>

**3.** Quanto vale `clamp(vector(42), -Inf, +Inf)`?

- A) -Inf
- B) +Inf
- C) 42
- D) NaN

<details><summary>Resposta</summary>

**C.** Com limites infinitos, os valores ficam inalterados.
</details>

**4.** Um sensor de umidade às vezes reporta 110%. Qual é a melhor prática?

- A) Usar `clamp(x, 0, 100)` no alerta de "umidade alta"
- B) Usar `clamp` nos dashboards e alertar separadamente no valor cru quando `x > 100`
- C) Apagar a série
- D) Usar `abs(x)`

<details><summary>Resposta</summary>

**B.** `clamp` é bom para exibição, mas esconde que o sensor está reportando valores impossíveis. O alerta deve olhar o dado cru.
</details>

## 📝 Cola rápida

- `clamp(v, min, max)`: 3 argumentos; `min`/`max` são **escalares**.
- `min > max` → **vazio**; `min`/`max` NaN → NaN; `-Inf, +Inf` → inalterado.
- `clamp(v, a, b) = clamp_max(clamp_min(v, a), b)`.
- Para **exibir** (gauge 0..100%); para **alertar**, use o cru.
- Achatar ≠ filtrar: `clamp` mantém a série; `x > a and x < b` remove.

## 🔗 Relacionadas

[`clamp_min()`](../clamp_min/) · [`clamp_max()`](../clamp_max/) · [`abs()`](../abs/) · [`round()`](../round/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#clamp
