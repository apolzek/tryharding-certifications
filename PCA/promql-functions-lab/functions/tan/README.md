# `tan()`: a tangente (a placa de ladeira)

> **Em uma frase:** `tan(v)` calcula a tangente de cada amostra (em **radianos**): "quanto sobe para cada 1 que anda". Vai de -∞ a +∞ e **explode perto de 90°**.

| | |
|---|---|
| **Assinatura** | `tan(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com um **ângulo em radianos** · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | razão (subida / avanço), sem limite |
| **Dashboard** | http://localhost:3300/d/fn-tan |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a placa "subida 10%"

A placa na estrada não diz o **ângulo** da ladeira: diz quantos metros você **sobe** para cada 100 m que **anda**. Isso é a tangente:

```
             subida
tan(θ) = ───────────       grade % = 100 · tan(θ)
            avanço
```

| ângulo | tan | placa |
|---|---|---|
| 5.7° | 0.10 | 10% |
| 30° | 0.577 | 57.7% |
| 45° | **1** | **100%** (sobe 1 m para cada 1 m) |
| 80° | 5.67 | 567% |
| 90° | ∞ | parede! |

Perto de 90° a tangente **dispara**: é uma **assíntota**. Uma parede vertical não tem "avanço", então a razão não existe.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `tan_ramp_inclination_degrees{ramp="doca-1"}` | gauge | rampa de carga: **0° → 45° → 0°** a cada 4 min |
| `tan_antenna_elevation_degrees{antenna="sat-dish"}` | gauge | antena seguindo satélite: **30° → 89.9°** (fica ~25s travada em 89.9°) → 30°, a cada 3 min |

**Casos (do básico ao real):** 🟢 **básico:** rampa (ângulo → grade %) · 🟡 **nuance:** assíntota perto de 90°, `tan(45)` sem `rad` · 🔴 **real:** limite de grade da doca / alcance de antena

```bash
curl -s localhost:8088/metrics | grep '^tan_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-tan
```

---

## 🔍 Queries passo a passo

### 1 e 2. Da inclinação (graus) para a "grade %"

```promql
tan_ramp_inclination_degrees                   # triângulo 0..45°
100 * tan(rad(tan_ramp_inclination_degrees))   # grade %
```

**Resultado esperado:** o ângulo é um triângulo **reto**; a grade % é um triângulo **curvado para cima**: 10° → **17.6%**, 30° → **57.7%**, 45° → **100%**. Dobrar o ângulo (de 20° para 40°) **mais que dobra** a grade (36% → 84%).

Uso real: alertar quando uma empilhadeira elétrica vai subir uma rampa acima da grade máxima que ela aguenta (ex.: 15%).

---

### 3 e 4. A assíntota perto de 90°

```promql
tan_antenna_elevation_degrees
tan(rad(tan_antenna_elevation_degrees))
```

**Resultado esperado** (painel 4 em **escala log**):

| elevação | tan |
|---|---|
| 30° | 0.58 |
| 60° | 1.73 |
| 80° | 5.67 |
| 89° | 57.3 |
| 89.9° | **573** |

A elevação sobe como uma reta, mas a tangente vira um **pico agudo**. Em escala linear o pico de 573 esmagaria todo o resto contra o zero.

---

### 5. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `tan(rad(vector(45)))` | `1` | |
| `tan(rad(vector(89.9)))` | `572.9572133543033` | perto da assíntota |
| `tan(rad(vector(90)))` | `16331239353195392` (1.6e16) | **não** é +Inf: π/2 em float64 é um pouquinho menor que o π/2 real |
| `tan(rad(vector(135)))` | `-1` (≈) | depois de 90° o sinal inverte |
| `tan(vector(+Inf))` | `NaN` | ([Go `math.Tan`](https://pkg.go.dev/math#Tan)) |
| `tan(vector(45))` | `1.6197751905438615` | ❌ **ERRADO**: 45 **radianos**; o painel mostra lado a lado com o `tan(45°) = 1` |

---

## 🏭 Casos reais

> Trigonometria é **rara** em alertas de infra. `tan()` aparece em exporters industriais que medem inclinação.

**1. Logística: rampa de doca com limite de grade.** Um inclinômetro industrial (exporter Modbus) publica `dock_ramp_inclination_degrees`. As empilhadeiras elétricas do CD aguentam no máximo **15%** de grade:

```yaml
groups:
  - name: doca
    rules:
      - record: dock:ramp_grade:percent
        expr: 100 * tan(rad(dock_ramp_inclination_degrees))
      - alert: DockRampTooSteep
        expr: dock:ramp_grade:percent > 15
        for: 1m
        annotations:
          summary: "Rampa {{ $labels.ramp }} com {{ $value | printf \"%.1f\" }}% de inclinação (limite 15%)"
```

Por que converter para %? Porque o limite do fabricante vem em **grade %**, não em graus (15% ≈ 8.5°).

**2. Antenas/telecom:** a altura do ponto de reflexão ou o alcance no solo de uma antena inclinada usam `altura / tan(rad(tilt))`. Cuidado com tilt perto de 0° (divisão por ~0) e perto de 90° (assíntota).

```yaml
groups:
  - name: antenas
    rules:
      # alcance no solo do feixe principal = altura / tan(downtilt)
      - record: antenna:ground_range:meters
        expr: antenna_height_meters / tan(rad(clamp_min(antenna_downtilt_degrees, 0.5)))
      - alert: AntennaCoverageTooShort
        expr: antenna:ground_range:meters < 500
        for: 15m
```

O `clamp_min(..., 0.5)` evita dividir por `tan(0) = 0` (alcance infinito) quando o tilt é zerado.

## ✅ Quando usar

- **Inclinação → grade %** (rampas, estradas, telhados, esteiras).
- **Altura por triangulação:** altura de um prédio = distância · tan(ângulo de visada).
- **Comprimento de sombra:** sombra = altura / tan(elevação do sol).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer o **ângulo** a partir de uma inclinação (subida/avanço) | [`atan()`](../atan/) |
| Ângulo pode chegar a 90° e você quer um valor limitado | [`sin()`](../sin/) ou [`cos()`](../cos/) (limitados a ±1) |
| Quer "espremer" valores num intervalo | [`tanh()`](../tanh/) (é outra função!) |

## ⚠️ Pegadinhas

1. **Radianos:** `tan(45)` = 1.62, não 1. Use `tan(rad(45))`.
2. **Assíntota:** perto de ±90° o valor explode; `tan(90°)` dá **1.6e16**, não +Inf. Um alerta `> 1e10` pega isso.
3. **Escala do gráfico:** use escala **log** ou limite o eixo, senão o pico esconde tudo.
4. **Sinal troca** ao passar de 90°: `tan(91°)` = -57.3.
5. **±Inf → NaN** e histogramas são ignorados.

## 🎓 Na prova PCA

- Instant vector entra/sai; **radianos**; nome da métrica removido.
- Assíntota em ±π/2: valor enorme (1.6e16), **não** +Inf.
- ±Inf → **NaN**.

**1.** Qual o resultado de `tan(rad(vector(90)))`?
- A) `+Inf`
- B) `NaN`
- C) um número finito muito grande (~1.6e16)
- D) `0`

<details><summary>Resposta</summary>

**C.** π/2 não é representável exatamente em float64; o valor fica um pouco abaixo do π/2 real e a tangente dá ~1.633e16.
</details>

**2.** Você quer o **ângulo** (em graus) de uma rampa que sobe 1 m a cada 10 m. Qual query?
- A) `deg(tan(vector(0.1)))`
- B) `deg(atan(vector(0.1)))`
- C) `tan(rad(vector(0.1)))`
- D) `rad(atan(vector(0.1)))`

<details><summary>Resposta</summary>

**B.** A razão subida/avanço é a tangente; o ângulo vem da **inversa**, `atan`, e `deg()` converte o resultado para graus (≈ 5.71°).
</details>

**3.** `tan(node_load1)`: o que acontece com o label `__name__`?
- A) Mantido  B) Removido  C) Renomeado para `tan`  D) Erro

<details><summary>Resposta</summary>

**B.** Funções matemáticas removem o nome da métrica.
</details>

## 📝 Cola rápida

- `tan(v)`: radianos → (-∞, +∞); `tan(45°) = 1`.
- Grade %: `100 * tan(rad(graus))`.
- Perto de 90° explode (1.6e16 em 90°); gráfico em escala log.
- Inversa: `atan()`. ±Inf → NaN. Remove `__name__`.

## 🔗 Relacionadas

[`atan()`](../atan/) · [`sin()`](../sin/) · [`cos()`](../cos/) · [`tanh()`](../tanh/) · [`rad()`](../rad/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
