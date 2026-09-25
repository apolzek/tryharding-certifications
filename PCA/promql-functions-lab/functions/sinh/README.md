# `sinh()`: seno hiperbólico (o amplificador simétrico)

> **Em uma frase:** `sinh(v)` calcula `(eˣ − e⁻ˣ) / 2` para cada amostra: fica **quase igual a x** perto de zero e **cresce exponencialmente** longe dele, **preservando o sinal**.

| | |
|---|---|
| **Assinatura** | `sinh(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (qualquer número real) · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | a mesma "escala" de x, mas deformada (não é ângulo!) |
| **Dashboard** | http://localhost:3300/d/fn-sinh |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o amplificador de pânico

Imagine um alarme que **quase não reage** a pequenos desvios, mas **grita** diante de desvios grandes, e que distingue "acima" de "abaixo":

| x | sinh(x) |
|---|---|
| 0 | 0 |
| 0.3 | 0.30 (igual) |
| 1 | 1.18 |
| 2 | 3.63 |
| 3 | **10.02** |
| -3 | **-10.02** |

Apesar do nome, `sinh` **não tem nada a ver com ângulos**. É uma função "hiperbólica", feita de exponenciais. Onde aparece na física: comprimento de um **cabo pendurado** (catenária): `L = 2a · sinh(vão / 2a)`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `sinh_latency_zscore{service="checkout"}` | gauge | z-score da latência: onda entre **-3σ e +3σ** (período 4 min) |
| `sinh_latency_zscore{service="search"}` | gauge | estável em **~0.3σ** |
| `sinh_powerline_catenary_param_meters{line="LT-138kV"}` | gauge | parâmetro `a` do cabo: **150 m** (quente, frouxo) ↔ **400 m** (frio, esticado), período 5 min |
| `sinh_powerline_span_meters{line="LT-138kV"}` | gauge | vão entre torres: **100 m** |

**Casos (do básico ao real):** 🟢 **básico:** amplificar z-score · 🟡 **nuance:** overflow e esquecer de normalizar · 🔴 **real:** comprimento do cabo de linha de transmissão

```bash
curl -s localhost:8088/metrics | grep '^sinh_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-sinh
```

---

## 🔍 Queries passo a passo

### 1 e 2. Amplificando o z-score

```promql
sinh_latency_zscore
sinh(sinh_latency_zscore)
```

**Resultado esperado:** "search" continua em **~0.30** (desvio pequeno passa quase igual). "checkout" vira uma onda **pontuda**: ±3 vira **±10.02**; a referência linear (±3) parece achatada ao lado. O "meio" da onda muda pouco, as pontas explodem.

---

### 3 e 4. Comprimento do cabo pendurado

```promql
2 * sinh_powerline_catenary_param_meters
  * sinh(sinh_powerline_span_meters / (2 * sinh_powerline_catenary_param_meters))
```

**Resultado esperado:** com vão de 100 m, o cabo tem **100.26 m** quando frio (a = 400) e **101.85 m** quando quente (a = 150). A linha do vão (100 m) fica logo abaixo. Uma dilatação de 1.6 m no cabo vira vários metros de flecha (ver [`cosh()`](../cosh/)).

---

### 5. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `sinh(vector(0))` | `0` | |
| `sinh(vector(1))` | `1.1752011936438014` | |
| `sinh(vector(-3))` | `-10.017874927409903` | **ímpar**: sinh(-x) = -sinh(x) |
| `sinh(vector(710))` | `+Inf` | e^710 passa do maior float64 (~1.8e308): **overflow** |
| `sinh(vector(-Inf))` | `-Inf` | ([Go `math.Sinh`](https://pkg.go.dev/math#Sinh)) |
| `sinh(sinh_powerline_span_meters)` | `1.3440585709080678e+43` | ❌ **ERRADO**: esqueceu o `/ (2a)`; sinh(100) é astronômico. Sempre normalize antes |

---

## 🏭 Casos reais

> Sinceridade: `sinh()` é **raríssima** em observabilidade. Os poucos usos são de "modelagem": deformar um score ou fórmulas físicas em exporters de IoT.

**1. Score de severidade que pune desvios grandes.** Com um z-score calculado em recording rule, `sinh` aumenta a distância entre "levemente anormal" e "muito anormal" sem perder o sinal:

```yaml
groups:
  - name: latency-anomaly
    rules:
      - record: job:latency_zscore:1h
        expr: |
          (job:http_request_duration_seconds:p99 - avg_over_time(job:http_request_duration_seconds:p99[1h]))
            / stddev_over_time(job:http_request_duration_seconds:p99[1h])
      - record: job:latency_severity:1h
        expr: sinh(job:latency_zscore:1h)     # 1σ → 1.2, 2σ → 3.6, 3σ → 10
```

**2. Monitoramento de linhas de transmissão:** sensores de temperatura do condutor + fórmula da catenária (`2a·sinh(vão/2a)`) para estimar o comprimento e, daí, a flecha.

```yaml
groups:
  - name: linhas-transmissao
    rules:
      - record: line:conductor_length:meters
        expr: |
          2 * powerline_catenary_param_meters
            * sinh(powerline_span_meters / (2 * powerline_catenary_param_meters))
      - alert: ConductorOverstretched
        expr: line:conductor_length:meters / powerline_span_meters > 1.02
        for: 30m
        annotations:
          summary: "Condutor da {{ $labels.line }} 2% mais longo que o vão (dilatação térmica alta)"
```

## ✅ Quando usar

- Amplificar desvios grandes **mantendo o sinal**.
- Fórmulas físicas (catenária, relatividade, etc.).
- Desfazer um [`asinh()`](../asinh/): `sinh(asinh(x)) = x`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer comprimir (o oposto) | [`asinh()`](../asinh/) ou [`tanh()`](../tanh/) |
| Quer penalidade simétrica (sinal não importa) | [`cosh()`](../cosh/) ou [`abs()`](../abs/) |
| Quer só exponencial | [`exp()`](../exp/) |
| Quer trigonometria de ângulos | [`sin()`](../sin/) (sem o "h") |

## ⚠️ Pegadinhas

1. **Não confunda com `sin()`:** o "h" muda tudo; `sinh` não é periódica nem limitada.
2. **Overflow:** a partir de |x| ≈ 710 o resultado é ±Inf.
3. **Escala:** multiplicar x por 10 multiplica o resultado por muito mais que 10. Normalize x antes (z-score, x/referência).
4. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- Instant vector entra/sai; **domínio: todos os reais**; nunca NaN para números finitos.
- Função **ímpar** e ilimitada; overflow → ±Inf.
- Faz parte das "trigonometric functions" da documentação, junto com `cosh`, `tanh` e as inversas.

**1.** Qual o resultado de `sinh(vector(0))`?
- A) `1`  B) `0`  C) `NaN`  D) `-Inf`

<details><summary>Resposta</summary>

**B.** (e⁰ − e⁰) / 2 = 0. Quem vale 1 em zero é o `cosh`.
</details>

**2.** `sinh(vector(1000))` retorna:
- A) `NaN`  B) erro de overflow  C) `+Inf`  D) `1000`

<details><summary>Resposta</summary>

**C.** O valor não cabe em float64 e vira +Inf; PromQL não aborta a query.
</details>

**3.** Qual par de funções é inverso uma da outra?
- A) `sinh` e `asin`
- B) `sinh` e `asinh`
- C) `sinh` e `cosh`
- D) `sinh` e `atanh`

<details><summary>Resposta</summary>

**B.** `asinh` é a inversa hiperbólica do `sinh`.
</details>

## 📝 Cola rápida

- `sinh(x) = (eˣ − e⁻ˣ)/2`; `sinh(0) = 0`; ímpar; ilimitada.
- Perto de 0 ≈ x; longe explode; |x| ≳ 710 → ±Inf.
- Inversa: `asinh`. Não é ângulo!
- Remove `__name__`; histogramas ignorados.

## 🔗 Relacionadas

[`cosh()`](../cosh/) · [`tanh()`](../tanh/) · [`asinh()`](../asinh/) · [`exp()`](../exp/) · [`sin()`](../sin/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
