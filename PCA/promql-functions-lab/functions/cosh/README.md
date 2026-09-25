# `cosh()`: cosseno hiperbólico (a curva do cabo pendurado)

> **Em uma frase:** `cosh(v)` calcula `(eˣ + e⁻ˣ) / 2`: uma curva em **U**, simétrica (`cosh(-x) = cosh(x)`), com **mínimo 1** em x = 0 e crescimento exponencial dos dois lados.

| | |
|---|---|
| **Assinatura** | `cosh(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (qualquer real) · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | adimensional, **sempre ≥ 1** |
| **Dashboard** | http://localhost:3300/d/fn-cosh |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a corrente pendurada

Segure uma **corrente** (ou um fio de alta tensão) pelas duas pontas. A curva que se forma **não** é uma parábola, é uma **catenária**:

```
y(x) = a · cosh(x / a)
```

`a` = tração horizontal / peso por metro. Cabo **esticado** (frio) → `a` grande → curva rasa. Cabo **dilatado** (quente) → `a` pequeno → cabo **baixa** no meio.

A **flecha** (quanto o meio desce em relação às torres) é:

```
flecha = a · (cosh(vão / 2a) − 1)
```

Outra leitura: `cosh` é uma **penalidade em U**: vale 1 no ponto ideal e cresce igualmente para os dois lados.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `cosh_powerline_catenary_param_meters{line="LT-138kV"}` | gauge | `a` entre **150 m** (quente) e **400 m** (frio), período 5 min |
| `cosh_powerline_span_meters{line="LT-138kV"}` | gauge | vão de **100 m** |
| `cosh_temperature_deviation_celsius{rack="r42"}` | gauge | desvio da temperatura do rack em relação a 22 °C: **-4 ↔ +4** (período 3 min) |

**Casos (do básico ao real):** 🟢 **básico:** penalidade de temperatura em U · 🟡 **nuance:** `cos` no lugar de `cosh` (flecha negativa) · 🔴 **real:** flecha do cabo de alta tensão

```bash
curl -s localhost:8088/metrics | grep '^cosh_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-cosh
```

---

## 🔍 Queries passo a passo

### 1 e 2. Flecha do cabo de energia

```promql
cosh_powerline_catenary_param_meters
cosh_powerline_catenary_param_meters
  * (cosh(cosh_powerline_span_meters / (2 * cosh_powerline_catenary_param_meters)) - 1)
```

**Resultado esperado:**

| a | vão/2a | cosh | flecha |
|---|---|---|---|
| 400 m (frio) | 0.125 | 1.0078 | **3.13 m** |
| 150 m (quente) | 0.333 | 1.0561 | **8.41 m** |

A flecha é o **espelho** de `a`: quando `a` cai, a flecha sobe.

❌ **O que dá errado** (também no painel 2): trocar `cosh` por `cos`:

```promql
cosh_powerline_catenary_param_meters * (cos(cosh_powerline_span_meters / (2 * cosh_powerline_catenary_param_meters)) - 1)
```

Como `cos(x) ≤ 1`, a "flecha" fica **negativa**: de **-3.1 m a -8.3 m**, como se o cabo subisse acima das torres. Um "h" a menos, um resultado fisicamente absurdo e sem nenhum erro de query.

---

### 3 e 4. Penalidade em U

```promql
cosh_temperature_deviation_celsius
cosh(cosh_temperature_deviation_celsius / 2)
```

**Resultado esperado:** o desvio é uma onda de -4 a +4; a penalidade vale **1** quando o desvio é 0 e sobe para **1.54** em ±2 °C e **3.76** em ±4 °C. Como é função **par**, o gráfico tem **dois picos por período** (um no quente, um no frio), iguais.

---

### 5. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `cosh(vector(0))` | `1` | mínimo |
| `cosh(vector(2))` | `3.7621956910836314` | |
| `cosh(vector(-2))` | `3.7621956910836314` | **par** |
| `cosh(vector(710))` | `+Inf` | overflow |
| `cosh(vector(-Inf))` | `+Inf` | sempre positivo ([Go `math.Cosh`](https://pkg.go.dev/math#Cosh)) |

---

## 🏭 Casos reais

> Sinceridade: `cosh()` é **raríssima** em alertas de TI. Aparece em IoT de energia/estruturas e em scores com penalidade simétrica.

**1. Concessionária de energia: distância mínima do cabo ao solo.** Sensores na linha publicam a temperatura do condutor, e um exporter calcula `a`. O alerta é sobre a **flecha**, que é o que causa curto com árvores:

```yaml
groups:
  - name: linhas-transmissao
    rules:
      - record: line:sag:meters
        expr: powerline_catenary_param_meters * (cosh(powerline_span_meters / (2 * powerline_catenary_param_meters)) - 1)
      - alert: PowerLineSagTooLow
        expr: powerline_tower_height_meters - on(line) line:sag:meters < 8
        for: 10m
        annotations:
          summary: "Cabo da {{ $labels.line }} a menos de 8 m do solo"
```

**2. Datacenter: penalidade de temperatura.** `cosh(desvio / 2)` penaliza igualmente rack frio demais (condensação) e quente demais, crescendo rápido nos extremos. Útil para um "custo de conforto térmico" num painel de eficiência.

```yaml
groups:
  - name: datacenter
    rules:
      - record: rack:thermal_penalty:cosh
        expr: cosh((rack_inlet_temperature_celsius - 22) / 2)
      - alert: RackThermalPenaltyHigh
        expr: rack:thermal_penalty:cosh > 3     # ≈ ±3.5 °C do ideal
        for: 15m
```

## ✅ Quando usar

- Fórmulas de catenária (cabos, correntes, arcos).
- Penalidade simétrica, suave e crescente.
- Desfazer um [`acosh()`](../acosh/) (para x ≥ 0).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer saber o sinal do desvio | [`sinh()`](../sinh/) |
| Penalidade linear basta | [`abs()`](../abs/) |
| Quer valores limitados | [`tanh()`](../tanh/) ou [`cos()`](../cos/) |
| Quer ângulo/trigonometria | [`cos()`](../cos/) (sem "h") |

## ⚠️ Pegadinhas

1. **Nunca menor que 1:** um `cosh(x) < 1` é impossível; `cosh(x) - 1` é que começa em 0.
2. **Par:** perde o sinal de x.
3. **Overflow:** |x| ≳ 710 → +Inf (mesmo com x negativo).
4. Não confundir com `cos()` (periódica, limitada a ±1).
5. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- Instant vector → instant vector; domínio: todos os reais; resultado ≥ 1.
- `cosh(-Inf)` = **+Inf**; `cosh(0)` = **1**.

**1.** Qual o resultado de `cosh(vector(0))`?
- A) `0`  B) `1`  C) `NaN`  D) `π/2`

<details><summary>Resposta</summary>

**B.** (e⁰ + e⁰)/2 = 1: o ponto mínimo da curva.
</details>

**2.** `cosh(vector(-Inf))` retorna:
- A) `-Inf`  B) `NaN`  C) `+Inf`  D) `1`

<details><summary>Resposta</summary>

**C.** `cosh` é par e sempre positiva: os dois lados vão a +Inf.
</details>

**3.** `cosh(my_metric)` e `cosh(-my_metric)` retornam:
- A) Valores opostos  B) O mesmo valor  C) O segundo é NaN  D) Erro, pois `-` não pode ser aplicado a vetor

<details><summary>Resposta</summary>

**B.** Função par. (E o menos unário em vetor é válido em PromQL.)
</details>

## 📝 Cola rápida

- `cosh(x) = (eˣ + e⁻ˣ)/2`; **≥ 1**; `cosh(0) = 1`; par.
- Catenária: flecha = `a * (cosh(vão/(2a)) - 1)`.
- ±Inf → +Inf; overflow em |x| ≈ 710.
- Inversa (x ≥ 1): `acosh`. Remove `__name__`.

## 🔗 Relacionadas

[`sinh()`](../sinh/) · [`tanh()`](../tanh/) · [`acosh()`](../acosh/) · [`cos()`](../cos/) · [`exp()`](../exp/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
