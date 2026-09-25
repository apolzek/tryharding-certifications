# `sin()`: o seno (a altura na roda-gigante)

> **Em uma frase:** `sin(v)` calcula o seno de cada amostra de `v`, **interpretando o valor como radianos**. O resultado fica sempre entre **-1 e 1**.

| | |
|---|---|
| **Assinatura** | `sin(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com um **ângulo em radianos** (ou algo que você converteu para radianos) · ⚠️ amostras de histograma são **ignoradas** |
| **Unidade do resultado** | adimensional, entre -1 e 1 |
| **Dashboard** | http://localhost:3300/d/fn-sin |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a roda-gigante

Você está numa **roda-gigante**. O **ângulo** da sua cabine só cresce: 0, 90°, 180°, 270°, 360°... e recomeça. Mas a sua **altura** em relação ao eixo sobe e desce **suavemente**:

```
ângulo:  0      π/2     π      3π/2    2π
altura:  0  →   +1  →   0  →   -1  →   0      (altura = sin(ângulo))
```

- O `sin()` transforma um **ângulo** (que dá voltas) em uma **onda suave** entre -1 e 1.
- Multiplicando pelo raio você tem a altura de verdade: `raio * sin(ângulo)`.
- Com o tempo no lugar do ângulo, `sin(2π · t / período)` vira um **gerador de ondas periódicas**, perfeito para modelar sazonalidade (tráfego que sobe de dia e cai de madrugada).

⚠️ O PromQL trabalha em **radianos**. Uma volta = **2π ≈ 6.283**, não 360. Dado em graus? Use `sin(rad(x))`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `sin_turbine_rotor_angle_radians{turbine="T1"}` | gauge | ângulo da pá A: **0 → 2π em 2 min** e recomeça (dente-de-serra) |
| `sin_turbine_rotor_angle_radians{turbine="T2"}` | gauge | igual, mas **90° (π/2) adiantada** |
| `sin_http_requests_per_second{service="shop"}` | gauge | `100 + 50·sin(2π·t/300)` + ruído: ciclo "diário" comprimido em **5 min**. A cada **10 min**, 60s de **incidente** (tráfego cai para 20%) |

**Casos (do básico ao real):** 🟢 **básico:** turbina (ângulo → onda) · 🟡 **nuance:** graus vs radianos, `sin(π) ≠ 0` · 🔴 **real:** baseline sazonal e detecção de incidente

```bash
curl -s localhost:8088/metrics | grep '^sin_'
# sin_http_requests_per_second{service="shop"} 131.2
# sin_turbine_rotor_angle_radians{turbine="T1"} 4.31
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-sin
```

Espere **~2 min** para ver uma volta completa da turbina e **~10 min** para ver um incidente.

---

## 🔍 Queries passo a passo

### 1. O ângulo cru (radianos)

```promql
sin_turbine_rotor_angle_radians
```

**Resultado esperado:** dois **dentes-de-serra** subindo de 0 a **6.28** e despencando para 0 a cada 2 min. T2 está 30s (1/4 de volta) à frente de T1.

---

### 2. `sin(ângulo)`: a altura da ponta da pá

```promql
sin(sin_turbine_rotor_angle_radians)
```

**O que faz:** aplica o seno em cada amostra.
**Resultado esperado:** duas **ondas suaves** entre **-1 e +1**, sem o "salto" do dente-de-serra (sin(2π) = sin(0) = 0, então a volta fecha certinho).

| ângulo | sin |
|---|---|
| 0 | 0 |
| π/2 ≈ 1.571 | **1** (pá no topo) |
| π ≈ 3.142 | ≈ 0 |
| 3π/2 ≈ 4.712 | **-1** (pá embaixo) |

T2 está defasada de 90°: quando T1 passa por 0 subindo, T2 está no topo.

---

### 3. Modelo sazonal sintético: `sin(2π · time() / período)`

```promql
sin_http_requests_per_second                          # real
100 + 50 * sin(vector(2 * pi() * time() / 300))       # modelo esperado
```

**O que faz:** `time()` é o relógio (segundos Unix). Dividindo pelo período (300s) e multiplicando por 2π, temos um ângulo que dá **uma volta a cada 5 min**. O `sin()` transforma isso numa onda entre -1 e 1, e `100 + 50 · (...)` estica para **50..150 req/s**.

**Resultado esperado:** a linha "modelo esperado" é uma senoide perfeita entre **50 e 150**; a linha "real" segue por cima dela com ruído de ±4, **exceto** durante o incidente, quando despenca.

> 💡 Em produção troque `300` por `86400` (ciclo diário) ou `604800` (semanal). Ajuste a **fase** somando um deslocamento: `sin(vector(2*pi()*(time() - 6*3600)/86400))` coloca o pico 6h mais tarde.

⚠️ `sin()` só aceita **instant vector**. `time()` e `pi()` são **escalares**, então é preciso embrulhar: `sin(vector(...))`. Sem isso: `expected type instant vector in call to function "sin", got scalar`.

---

### 4. Real − esperado: detecção de anomalia

```promql
sin_http_requests_per_second - scalar(100 + 50 * sin(vector(2 * pi() * time() / 300)))
```

**O que faz:** `scalar()` transforma o modelo (vetor sem labels) num escalar, que pode ser subtraído de qualquer série.
**Resultado esperado:** uma linha perto de **0 (±5)**. Durante o incidente cai para algo entre **-70 e -115 req/s** (o valor exato depende de em que ponto da onda o incidente cai). Um alerta como `... < -40` pegaria o incidente sem disparar no "vale" natural da madrugada.

---

### 5. Pegadinha: graus vs radianos

```promql
sin(vector(90))          # 0.8940  ← 90 RADIANOS (≈ 14 voltas + 57°)
sin(rad(vector(90)))     # 1       ← 90 graus
```

Nenhum erro é emitido: o resultado só fica **errado**. Veja [`rad()`](../rad/).

---

### 6. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `sin(vector(0))` | `0` | |
| `sin(vector(pi() / 2))` | `1` | |
| `sin(vector(pi()))` | `1.2246467991473515e-16` | π não é representável exatamente em float64: **não** dá 0 exato |
| `sin(vector(+Inf))` | `NaN` | seno de infinito não existe ([Go `math.Sin`](https://pkg.go.dev/math#Sin)) |
| `sin(vector(NaN))` | `NaN` | NaN entra, NaN sai |

> Nunca compare `sin(...) == 0`. Use uma tolerância: `abs(sin(x)) < 1e-9`.

---

## 🏭 Casos reais

> Sinceridade: funções trigonométricas são **raras** em alertas de infraestrutura. Onde aparecem, é quase sempre em (a) **baselines sazonais sintéticos** e (b) **exporters de IoT/indústria** que publicam ângulos.

**1. SRE do e-commerce: baseline "diário" para detecção de anomalia.**
O tráfego do checkout tem um ciclo diário previsível (pico às 20h, vale às 5h). Em vez de um limiar fixo (que dispara toda madrugada), compare o real com um modelo senoidal:

```yaml
groups:
  - name: checkout-baseline
    rules:
      # modelo: média 800 req/s, amplitude 600, pico às 20h UTC (fase de 14h)
      - record: checkout:http_requests:expected_rate
        expr: 800 + 600 * sin(vector(2 * pi() * (time() - 14 * 3600) / 86400))
      - alert: CheckoutTrafficBelowExpected
        expr: |
          sum(rate(http_requests_total{job="checkout"}[5m]))
            < 0.5 * scalar(checkout:http_requests:expected_rate)
        for: 10m
        labels: {severity: page}
        annotations:
          summary: "Checkout com menos da metade do tráfego esperado para este horário"
```

Por que `sin()` e não `offset 1d`? O `offset` copia também as **anomalias de ontem** (se ontem teve incidente, o baseline de hoje está errado). O modelo sintético é estável. Na prática muitos times combinam os dois.

**2. Exporter de turbina eólica (Modbus/SCADA → Prometheus).** O exporter publica `turbine_rotor_position_radians`. A altura da ponta da pá é `blade_length * sin(...)`, e a projeção horizontal alimenta um painel de "sombra estroboscópica" (shadow flicker) exigido por regulação ambiental.

**Regra para o caso 2 (turbina):**

```yaml
groups:
  - name: turbinas
    rules:
      - record: turbine:blade_tip_height:meters
        expr: |
          turbine_hub_height_meters
            + on(turbine) turbine_blade_length_meters * sin(turbine_rotor_position_radians)
      - alert: TurbineBladeLowTipClearance
        expr: min_over_time(turbine:blade_tip_height:meters[5m]) < 30
        annotations:
          summary: "Ponta da pá da {{ $labels.turbine }} passando a menos de 30 m do solo"
```

## ✅ Quando usar

- **Modelo sazonal / baseline sintético:** tráfego esperado por hora do dia, para comparar com o real ou gerar dados de teste.
- **Coordenadas de algo que gira:** braço robótico, pá de turbina, antena: `raio * sin(ângulo)` dá a componente vertical.
- **Decompor ângulos** em componentes (vento, direção) para depois fazer média corretamente (ver [`atan()`](../atan/)).
- **Codificar ciclos** (hora do dia, dia da semana) de forma contínua: 23h e 0h ficam "perto" em `sin`/`cos`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Dado está em **graus** | `sin(rad(x))`: ver [`rad()`](../rad/) |
| Quer o **ângulo** a partir de uma razão (-1..1) | [`asin()`](../asin/) |
| Quer a componente **horizontal** | [`cos()`](../cos/) |
| Quer só "suavizar" uma série | [`avg_over_time()`](../avg_over_time/) |
| Quer prever tendência | [`predict_linear()`](../predict_linear/) |

## ⚠️ Pegadinhas

1. **Radianos, sempre.** `sin(90)` ≠ 1.
2. **Escalar não entra:** `sin(pi())` dá erro; use `sin(vector(pi()))`.
3. **Ponto flutuante:** `sin(π)` = 1.22e-16, não 0.
4. **±Inf vira NaN**, e NaN "some" do gráfico (vira buraco).
5. **Histogramas são ignorados:** amostras de native histogram não geram saída.
6. **O nome da métrica é removido** do resultado (como em toda função matemática).

## 🎓 Na prova PCA

O que costuma cair sobre as funções trigonométricas:

- Recebem e devolvem **instant vector** (não aceitam range vector nem escalar).
- Trabalham em **radianos**; `deg()`/`rad()` convertem.
- Fora do domínio (ou com ±Inf) o resultado é **NaN**, não erro.
- Como toda função matemática, **removem o nome da métrica** (`__name__`) do resultado.
- `pi()` é um **escalar**.

**1.** Qual o resultado de `sin(vector(90))`?
- A) `1`
- B) `0.894` (aproximadamente)
- C) Erro: argumento fora do domínio
- D) `NaN`

<details><summary>Resposta</summary>

**B.** O argumento é interpretado como **90 radianos**, não 90 graus. Para 90° use `sin(rad(vector(90)))` = 1.
</details>

**2.** A query `sin(2 * pi() * time() / 86400)` é executada. O que acontece?
- A) Retorna uma onda diária
- B) Retorna um escalar entre -1 e 1
- C) Erro de parse: `sin` espera instant vector e recebeu escalar
- D) Retorna um vetor vazio

<details><summary>Resposta</summary>

**C.** `pi()` e `time()` são escalares, e `sin()` só aceita instant vector. Correto: `sin(vector(2 * pi() * time() / 86400))`.
</details>

**3.** `sin(my_angle_radians{motor="m1"})` retorna uma série com quais labels?
- A) `{__name__="my_angle_radians", motor="m1"}`
- B) `{motor="m1"}`
- C) `{__name__="sin", motor="m1"}`
- D) Nenhuma label

<details><summary>Resposta</summary>

**B.** Funções que transformam o valor **removem o nome da métrica** e mantêm as demais labels.
</details>

## 📝 Cola rápida

- `sin(v instant-vector)`: radianos entram, valor em **[-1, 1]** sai; nome da métrica é removido.
- Graus? `sin(rad(x))`. Escalar? `sin(vector(x))`.
- `sin(±Inf)` = NaN; `sin(π)` = 1.22e-16 (não 0).
- Onda periódica sem métrica: `sin(vector(2*pi()*time()/período))`.
- Histogramas são ignorados.

## 🔗 Relacionadas

[`cos()`](../cos/) · [`tan()`](../tan/) · [`asin()`](../asin/) · [`sinh()`](../sinh/) · [`rad()`](../rad/) · [`deg()`](../deg/) · [`pi()`](../pi/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
