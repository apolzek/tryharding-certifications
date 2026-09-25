# `tanh()`: tangente hiperbólica (o "clamp" suave)

> **Em uma frase:** `tanh(v)` espreme **qualquer número real** para dentro de **(-1, 1)**: quase linear perto de zero, saturando **suavemente** em ±1 longe dele.

| | |
|---|---|
| **Assinatura** | `tanh(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (qualquer real; normalmente um sinal dividido por uma "referência") · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | adimensional, em (-1, 1) (±1 exatos por arredondamento a partir de ~|x| > 19) |
| **Dashboard** | http://localhost:3300/d/fn-tanh |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o amortecedor

Empurre um **amortecedor** com a força que quiser: ele comprime, comprime cada vez menos... e nunca passa do fim do curso.

| x | tanh(x) |
|---|---|
| 0 | 0 |
| 0.5 | 0.46 |
| 1 | 0.76 |
| 2 | 0.96 |
| 3 | 0.995 |
| 20 | 1 |

Comparado com [`clamp_max()`](../clamp_max/), que é um **batente seco** (sobe em linha reta e para de repente), o `tanh` é um **freio progressivo**. E comparado a dividir pelo máximo, ele não depende de saber qual é o máximo.

Truque: `tanh(sinal / referência)`. A "referência" é o valor que você considera "já é bastante" (vira 0.76).

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `tanh_queue_backlog_messages{queue="pedidos"}` | gauge | **~700 (±150)**, com pico em forma de sino até **~20 000** a cada 5 min (90s) |
| `tanh_error_ratio{service="api"}` | gauge | **0.002**, com degradação de 60s a cada 4 min para **0.08** |
| `tanh_latency_p99_seconds{service="api"}` | gauge | **0.12 s**, com a mesma degradação para **2.5 s** |

**Casos (do básico ao real):** 🟢 **básico:** espremer backlog em 0..1 · 🟡 **nuance:** `tanh` vs `clamp_max` vs sem normalizar · 🔴 **real:** health score de erro + latência

```bash
curl -s localhost:8088/metrics | grep '^tanh_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-tanh
```

---

## 🔍 Queries passo a passo

### 1 e 2. Índice de pressão da fila: `tanh` vs `clamp_max`

```promql
tanh(tanh_queue_backlog_messages / 1000)
clamp_max(tanh_queue_backlog_messages / 1000, 1)
```

**Resultado esperado:**

| backlog | tanh(x/1000) | clamp_max(x/1000, 1) |
|---|---|---|
| 550 | 0.50 | 0.55 |
| 700 | **0.60** | 0.70 |
| 850 | 0.69 | 0.85 |
| 1 000 | 0.76 | **1** (batente) |
| 3 000 | 0.995 | 1 |
| 20 000 | 1 | 1 |

No estado normal, a linha do `tanh` "respira" menos que a linear (o ruído é comprimido). No pico, o `clamp_max` bate no teto com uma **quina**, enquanto o `tanh` chega **curvando**.

❌ **O que dá errado** (terceira linha do painel 2): `tanh(tanh_queue_backlog_messages)` sem dividir pela referência é **sempre 1** (tanh(700) = 1): o índice não informa nada.

---

### 3 e 4. Health score com sinais de escalas diferentes

```promql
1 - (tanh(tanh_error_ratio / 0.01) + on(service) tanh(tanh_latency_p99_seconds / 0.5)) / 2
```

**O que faz:** cada sinal é dividido pela sua referência (1% de erro, 500 ms de p99) e espremido em 0..1. A média dos dois vira "quanto está ruim"; `1 − ...` vira "saúde".
**Resultado esperado:**
- normal: erro 0.002 → 0.20, p99 0.12 s → 0.24 → saúde ≈ **78%**;
- degradado: erro 0.08 → 1.00, p99 2.5 s → 1.00 → saúde ≈ **0%**.

Nenhum sinal sozinho consegue "estourar" o score: cada um contribui **no máximo 50%**.

---

### 5. Casos especiais

| Query | Resultado |
|---|---|
| `tanh(vector(0))` | `0` |
| `tanh(vector(1))` | `0.7615941559557649` |
| `tanh(vector(3))` | `0.9950547536867305` |
| `tanh(vector(-3))` | `-0.9950547536867305` (ímpar) |
| `tanh(vector(20))` | `1` (arredondado em float64) |
| `tanh(vector(+Inf))` | `1` |
| `tanh(vector(NaN))` | `NaN` ([Go `math.Tanh`](https://pkg.go.dev/math#Tanh)) |

---

## 🏭 Casos reais

> Das hiperbólicas, `tanh` é a mais útil em ops: normalizar sinais sem teto para compor **scores** e painéis de "saúde".

**1. Score de saúde do serviço (SRE do checkout).** Combina taxa de erro e latência, com referências definidas pelo SLO:

```yaml
groups:
  - name: checkout-health
    rules:
      - record: job:error_ratio:rate5m
        expr: |
          sum by (job) (rate(http_requests_total{job="checkout",code=~"5.."}[5m]))
            / sum by (job) (rate(http_requests_total{job="checkout"}[5m]))
      - record: job:latency_p99:5m
        expr: histogram_quantile(0.99, sum by (job, le) (rate(http_request_duration_seconds_bucket{job="checkout"}[5m])))
      - record: job:health_score:5m
        expr: 1 - (tanh(job:error_ratio:rate5m / 0.01) + tanh(job:latency_p99:5m / 0.5)) / 2
```

**2. Fila de mensagens (Kafka lag).** `tanh(sum by (consumergroup) (kafka_consumergroup_lag) / 10000)` dá um "índice de pressão" 0..1 para um **mapa de calor** de muitos consumer groups: um grupo com lag de 5 milhões não apaga todos os outros da escala de cores.

> Para **alertas**, prefira o sinal cru com limiar claro (`lag > 10000`). O `tanh` é para **visualização e composição**.

```yaml
groups:
  - name: kafka
    rules:
      - record: consumergroup:lag_pressure:tanh
        expr: tanh(sum by (consumergroup) (kafka_consumergroup_lag) / 10000)   # só para o mapa de calor
      - alert: KafkaConsumerLagHigh
        expr: sum by (consumergroup) (kafka_consumergroup_lag) > 10000         # alerta no valor cru
        for: 10m
```

## ✅ Quando usar

- Normalizar sinais **sem teto** para 0..1 (ou -1..1) sem saber o máximo.
- Compor **scores** de sinais com escalas diferentes.
- Visualização: evitar que um outlier achate o resto do gráfico.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer um limite rígido | [`clamp()`](../clamp/) / [`clamp_max()`](../clamp_max/) |
| Quer comprimir mas **sem** saturar (log simétrico) | [`asinh()`](../asinh/) |
| Quer desfazer um tanh | [`atanh()`](../atanh/) |
| Alertas com limiar | o sinal cru, ex.: `lag > 10000` |

## ⚠️ Pegadinhas

1. **Escolha da referência** manda em tudo: `tanh(x)` com x em milhares satura sempre em 1. Divida antes.
2. **Satura cedo:** acima de x ≈ 3 tudo vira ~1: diferenças entre "ruim" e "catastrófico" somem.
3. **Não é `tan()`:** tangente trigonométrica explode; tanh é limitada.
4. Em float64, `tanh(20)` = 1 exato: não use `== 1` como "infinito".
5. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- Instant vector → instant vector; domínio: todos os reais; imagem: (-1, 1).
- `tanh(±Inf)` = **±1**; `tanh(NaN)` = NaN.

**1.** Qual o intervalo de valores possíveis de `tanh(x)` para x real finito?
- A) [0, 1]  B) (-1, 1)  C) (-π/2, π/2)  D) (-∞, +∞)

<details><summary>Resposta</summary>

**B.** Ímpar e limitada a ±1 (C é o intervalo do `atan`).
</details>

**2.** `tanh(vector(+Inf))` retorna:
- A) `+Inf`  B) `NaN`  C) `1`  D) erro

<details><summary>Resposta</summary>

**C.** O limite da tangente hiperbólica no infinito é 1.
</details>

**3.** Você aplica `tanh(queue_backlog)` com backlog típico de 5 000. O resultado é sempre 1. Por quê?
- A) Bug do Prometheus
- B) `tanh` exige valores entre -1 e 1
- C) Faltou normalizar: tanh(5000) satura em 1; use `tanh(queue_backlog / 5000)`
- D) `tanh` só funciona com counters

<details><summary>Resposta</summary>

**C.** `tanh` aceita qualquer real, mas satura muito antes de 5 000. Divida por uma referência.
</details>

## 📝 Cola rápida

- `tanh(x)`: ℝ → (-1, 1); `tanh(1) ≈ 0.76`, `tanh(3) ≈ 0.995`; ±Inf → ±1.
- Receita: `tanh(sinal / referência)` = "clamp suave".
- Score: `1 - avg(tanh(sinal_i / ref_i))`.
- Inversa: `atanh`. Remove `__name__`.

## 🔗 Relacionadas

[`atanh()`](../atanh/) · [`sinh()`](../sinh/) · [`cosh()`](../cosh/) · [`clamp()`](../clamp/) · [`clamp_max()`](../clamp_max/) · [`asinh()`](../asinh/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
