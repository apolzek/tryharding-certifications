# `acosh()`: cosseno hiperbólico inverso (domínio x ≥ 1)

> **Em uma frase:** `acosh(v)` desfaz o `cosh()`: recebe um valor **≥ 1** e devolve o x **≥ 0** correspondente. Abaixo de 1 → **NaN**. Para x grande, `acosh(x) ≈ ln(2x)`.

| | |
|---|---|
| **Assinatura** | `acosh(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge do tipo **razão ≥ 1** ("quantas vezes o normal") · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | adimensional, ≥ 0 |
| **Dashboard** | http://localhost:3300/d/fn-acosh |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: ler o "U" de volta

O [`cosh()`](../cosh/) desenha um **U** que nunca desce abaixo de 1. O `acosh()` responde: "o U está na altura y; **a que distância do centro** estou?".

- `acosh(1)` = 0 (estou no fundo do U).
- `acosh(3.76)` = 2.
- `acosh(0.5)` = ??? Nenhum ponto do U está abaixo de 1 → **NaN**.

Na prática, `acosh` se comporta como um **log que começa em 1**: bom para razões do tipo "atual / normal", que valem 1 no dia a dia e 50 na Black Friday.

| razão | acosh | ln(2·razão) |
|---|---|---|
| 1 | 0 | 0.69 |
| 1.2 | 0.62 | 0.88 |
| 3 | 1.76 | 1.79 |
| 50 | 4.61 | 4.61 |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `acosh_traffic_peak_ratio{service="checkout"}` | gauge | **~1.2x**; a cada 4 min, rajada até **50x** (80s) |
| `acosh_traffic_peak_ratio{service="batch"}` | gauge | **0.5x ↔ 3x** (período 3 min): fica **abaixo de 1** parte do tempo |
| `acosh_powerline_sag_ratio{line="LT-138kV"}` | gauge | razão tração na torre / tração no meio de um cabo = `cosh(vão/2a)`: **1.008 ↔ 1.056** |
| `acosh_powerline_span_meters{line="LT-138kV"}` | gauge | vão de **100 m** |

**Casos (do básico ao real):** 🟢 **básico:** comprimir razão de pico ≥ 1 · 🟡 **nuance:** NaN abaixo de 1, `clamp_min` · 🔴 **real:** Black Friday; recuperar parâmetro físico do cabo

```bash
curl -s localhost:8088/metrics | grep '^acosh_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-acosh
```

---

## 🔍 Queries passo a passo

### 1 e 2. Comprimindo a razão de pico

```promql
acosh_traffic_peak_ratio
acosh(acosh_traffic_peak_ratio)
```

**Resultado esperado:**
- checkout: 1.2 → **0.62**; no pico de 50x → **4.61**. A rajada continua visível, mas não esmaga o resto.
- batch: 3x → **1.76**; quando cai **abaixo de 1** a linha **some** (NaN).

---

### 3. Corrigindo o domínio com `clamp_min`

```promql
acosh(clamp_min(acosh_traffic_peak_ratio, 1))
```

**Resultado esperado:** batch sem buracos: abaixo do baseline vira **0** ("normal ou menos").

---

### 4. Física: desfazendo um `cosh`

```promql
acosh_powerline_span_meters / (2 * acosh(acosh_powerline_sag_ratio))
```

**O que faz:** se `razão = cosh(vão / 2a)`, então `a = vão / (2 · acosh(razão))`.
**Resultado esperado:** `a` recuperado oscila entre **150 e 400 m**, exatamente o parâmetro usado pelo gerador (ver [`cosh()`](../cosh/)).

---

### 5. Casos especiais

| Query | Resultado |
|---|---|
| `acosh(vector(1))` | `0` |
| `acosh(vector(2))` | `1.3169578969248166` |
| `acosh(cosh(vector(2)))` | `2` |
| `acosh(vector(0.999))` | `NaN` |
| `acosh(vector(-5))` | `NaN` |
| `acosh(vector(+Inf))` | `+Inf` ([Go `math.Acosh`](https://pkg.go.dev/math#Acosh)) |

⚠️ `acosh(cosh(vector(-2)))` = **2**, não -2: o `cosh` perde o sinal, e o `acosh` sempre devolve o lado positivo.

---

## 🏭 Casos reais

> Sinceridade: `acosh` é **muito rara** em observabilidade. O uso mais defensável é comprimir razões ≥ 1 para visualização.

**1. Black Friday: quantas vezes o tráfego normal.** Com um baseline de 1 semana atrás:

```yaml
groups:
  - name: black-friday
    rules:
      - record: job:traffic_peak_ratio:5m
        expr: |
          sum by (job) (rate(http_requests_total[5m]))
            / sum by (job) (rate(http_requests_total[5m] offset 1w))
      - record: job:traffic_peak_ratio:acosh
        expr: acosh(clamp_min(job:traffic_peak_ratio:5m, 1))
```

O `clamp_min` é obrigatório: à noite o tráfego pode ficar **abaixo** da semana passada e o `acosh` daria NaN.

**2. IoT de estruturas:** exporters que publicam razões de tração de cabos (pontes estaiadas, linhas de transmissão) usam `acosh` para recuperar o parâmetro físico da catenária, como na query 4.

```yaml
groups:
  - name: estruturas
    rules:
      # a = vão / (2·acosh(razão)); clamp_min acima de 1 evita acosh(1) = 0 → divisão por zero
      - record: line:catenary_param:meters
        expr: powerline_span_meters / (2 * acosh(clamp_min(powerline_tension_ratio, 1.000001)))
      - alert: CatenaryParamLow
        expr: line:catenary_param:meters < 180
        for: 30m
        annotations:
          summary: "Cabo da {{ $labels.line }} frouxo demais (a < 180 m)"
```

## ✅ Quando usar

- Desfazer um [`cosh()`](../cosh/).
- Comprimir razões ≥ 1 ("x vezes o normal") com zero no valor 1.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Valor pode ser < 1 ou negativo | [`asinh()`](../asinh/) ou [`ln()`](../ln/) com cuidado |
| Quer um log simples de algo > 0 | [`ln()`](../ln/) / [`log2()`](../log2/) |
| Ângulo a partir de razão em [-1, 1] | [`acos()`](../acos/) (sem "h") |

## ⚠️ Pegadinhas

1. **Domínio x ≥ 1:** 0.999 já é NaN. Use `clamp_min(x, 1)`.
2. **NaN silencia alertas** e some dos gráficos.
3. **Sempre ≥ 0:** perde o sinal de quem gerou o cosh.
4. Não confundir com `acos()` (domínio [-1, 1]).
5. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- Instant vector → instant vector; domínio **[1, +∞)**; imagem **[0, +∞)**.
- Abaixo de 1 → **NaN** (sem erro de query).

**1.** Qual o resultado de `acosh(vector(0.5))`?
- A) `0`  B) `-0.69`  C) `NaN`  D) erro de domínio

<details><summary>Resposta</summary>

**C.** Fora do domínio [1, ∞) o resultado é NaN; PromQL não lança erro por valor.
</details>

**2.** Qual valor faz `acosh(vector(x))` retornar exatamente `0`?
- A) `0`  B) `1`  C) `-1`  D) `π`

<details><summary>Resposta</summary>

**B.** `cosh(0) = 1`, então `acosh(1) = 0`.
</details>

**3.** `acosh(cosh(vector(-3)))` retorna:
- A) `-3`  B) `3`  C) `NaN`  D) `+Inf`

<details><summary>Resposta</summary>

**B.** `cosh` é par (perde o sinal) e `acosh` devolve sempre o valor ≥ 0.
</details>

## 📝 Cola rápida

- `acosh(x)`: domínio **x ≥ 1** → [0, +∞); `acosh(1) = 0`.
- x < 1 → **NaN**: proteja com `clamp_min(x, 1)`.
- Para x grande, ≈ `ln(2x)`.
- Inversa de `cosh` (lado positivo). Remove `__name__`.

## 🔗 Relacionadas

[`cosh()`](../cosh/) · [`asinh()`](../asinh/) · [`atanh()`](../atanh/) · [`acos()`](../acos/) · [`ln()`](../ln/) · [`clamp_min()`](../clamp_min/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
