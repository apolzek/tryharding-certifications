# `cos()`: o cosseno (quanto do sol chega no painel)

> **Em uma frase:** `cos(v)` calcula o cosseno de cada amostra de `v`, **interpretando o valor como radianos**. O resultado fica entre **-1 e 1**.

| | |
|---|---|
| **Assinatura** | `cos(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com um **ângulo em radianos** · ⚠️ amostras de histograma são **ignoradas** |
| **Unidade do resultado** | adimensional, entre -1 e 1 |
| **Dashboard** | http://localhost:3300/d/fn-cos |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a lanterna na parede

Aponte uma **lanterna** direto para a parede (0°): o círculo de luz é pequeno e forte. Incline a lanterna: a mesma luz se espalha numa elipse maior e cada ponto recebe **menos**. A fração que chega é o **cosseno do ângulo**:

```
0°  → cos = 1.00  (100% da luz)
60° → cos = 0.50  (metade)
80° → cos = 0.17  (quase nada)
90° → cos = 0     (luz rasante)
```

Um **painel solar** funciona igual: `potência ≈ potência_nominal · cos(ângulo de incidência)`.

O cosseno também é a **sombra horizontal** de algo que gira: um braço de 2 m no ângulo θ está em `x = 2·cos(θ)`, `y = 2·sin(θ)`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `cos_solar_incidence_angle_degrees{panel="fixo"}` | gauge | **graus**: 80° → 0° → 80° a cada 4 min (o sol "passa") |
| `cos_solar_incidence_angle_degrees{panel="tracker"}` | gauge | painel com rastreador: sempre ~**5°** |
| `cos_solar_panel_rated_watts{panel}` | gauge | **400 W** (potência com o sol de frente) |
| `cos_robot_arm_angle_radians{arm="r1"}` | gauge | braço de 2 m: 0 → 2π a cada 3 min (**radianos**) |

**Casos (do básico ao real):** 🟢 **básico:** braço robótico (x = 2·cos θ) · 🟡 **nuance:** esquecer o `rad()`, `cos(π/2) ≠ 0` · 🔴 **real:** potência esperada de painel solar

```bash
curl -s localhost:8088/metrics | grep '^cos_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-cos
```

Espere **~4 min** para ver um ciclo completo do sol.

---

## 🔍 Queries passo a passo

### 1. Ângulo de incidência (graus)

```promql
cos_solar_incidence_angle_degrees
```

**Resultado esperado:** "fixo" é um **triângulo** 80 → 0 → 80; "tracker" é uma linha em ~5.

---

### 2. Potência esperada do painel

```promql
cos_solar_panel_rated_watts * cos(rad(cos_solar_incidence_angle_degrees))
```

**O que faz:** converte o ângulo para radianos com [`rad()`](../rad/), tira o cosseno e multiplica pela potência nominal (as duas séries casam pelo label `panel`).

**Resultado esperado:**

| panel | ângulo | potência |
|---|---|---|
| fixo | 0° | **400 W** |
| fixo | 60° | **200 W** |
| fixo | 80° | **≈ 69 W** |
| tracker | ~5° | **≈ 398 W** o tempo todo |

Moral: o rastreador ganha muito justamente quando o sol está baixo. Em produção: compare esse "esperado" com a potência medida para achar **painel sujo** ou inversor com defeito.

---

### 3. Pegadinha: esquecer o `rad()`

```promql
cos_solar_panel_rated_watts{panel="fixo"} * cos(cos_solar_incidence_angle_degrees{panel="fixo"})   # ERRADO
```

**Resultado esperado:** a linha "ERRADO" **chacoalha** entre -400 e +400 W (potência negativa!), porque cada grau vira "1 radiano ≈ 57°": o ângulo dá dezenas de voltas. A linha "certo" é a curva suave do painel 2.

---

### 4. Braço robótico: `cos` e `sin` juntos

```promql
2 * cos(cos_robot_arm_angle_radians)   # x
2 * sin(cos_robot_arm_angle_radians)   # y
```

**Resultado esperado:** duas ondas entre **-2 m e +2 m**, idênticas mas **defasadas de 1/4 de volta**: quando x está no máximo (θ = 0), y está em 0. É a projeção de um círculo.

---

### 5. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `cos(vector(0))` | `1` | |
| `cos(rad(vector(60)))` | `0.5000000000000001` | arredondamento de float |
| `cos(vector(pi() / 2))` | `6.123233995736757e-17` | **não** é 0 exato |
| `cos(vector(pi()))` | `-1` | |
| `cos(rad(vector(-60)))` | `0.5000000000000001` | cosseno é **par**: cos(-x) = cos(x) |
| `cos(vector(-Inf))` | `NaN` | ([Go `math.Cos`](https://pkg.go.dev/math#Cos)) |

---

## 🏭 Casos reais

> Trigonometria é **rara** em alertas de infra; aparece em exporters de IoT/energia e em estatística circular.

**1. Usina solar: detectar painel sujo ou inversor degradado.** O exporter do inversor (SunSpec/Modbus) publica `inverter_ac_power_watts` e uma estação meteorológica publica `sun_incidence_angle_degrees` e `irradiance_watts_per_m2`. A potência **esperada** usa `cos()`:

```yaml
groups:
  - name: solar
    rules:
      - record: panel:expected_power_watts
        expr: |
          panel_rated_watts
            * clamp_min(cos(rad(sun_incidence_angle_degrees)), 0)
            * on() group_left() (irradiance_watts_per_m2 / 1000)
      - alert: SolarPanelUnderperforming
        expr: inverter_ac_power_watts / panel:expected_power_watts < 0.7
        for: 30m
        annotations:
          summary: "Painel {{ $labels.panel }} gerando < 70% do esperado (sujeira? sombra? inversor?)"
```

`clamp_min(..., 0)` evita potência **negativa** quando o sol está "atrás" do painel (ângulo > 90°).

**2. Codificar hora do dia como ciclo.** Para comparar "a mesma hora" sem descontinuidade entre 23h e 0h: `cos(2 * pi() * hour() / 24)` e `sin(...)` (`hour()` sem argumento já devolve um instant vector). 23h e 0h ficam vizinhas no círculo.

```yaml
groups:
  - name: features-ciclicas
    rules:
      # hora do dia como ponto num círculo: 23h e 0h ficam vizinhas
      - record: hour_of_day:cos
        expr: cos(2 * pi() * hour() / 24)
      - record: hour_of_day:sin
        expr: sin(2 * pi() * hour() / 24)
```

Essas séries alimentam modelos de previsão/anomalia externos (que leem do Prometheus) sem a "quebra" artificial entre 23h e 0h.

## ✅ Quando usar

- **Energia solar:** potência teórica em função do ângulo de incidência.
- **Projeções / componentes:** componente horizontal de vento, força, braço robótico.
- **Codificar ciclos:** `cos(2π·hora/24)` junto com `sin(...)` deixa 23h e 0h vizinhas.
- **Média de ângulos:** média de `sin` e `cos` + `atan2` (ver [`atan()`](../atan/)).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Dado em graus | `cos(rad(x))` (ver [`rad()`](../rad/)) |
| Quer o ângulo a partir de uma razão | [`acos()`](../acos/) |
| Quer a componente vertical | [`sin()`](../sin/) |
| Quer uma "penalidade em U" que cresce sem limite | [`cosh()`](../cosh/) |

## ⚠️ Pegadinhas

1. **Radianos:** `cos(60)` = -0.95, não 0.5.
2. **`cos(π/2)` não é 0:** use tolerância nas comparações.
3. **Resultado negativo** quando o ângulo passa de 90°: num painel solar isso significa "sol atrás do painel"; use `clamp_min(..., 0)`.
4. **Escalar não entra:** `cos(pi())` dá erro; use `cos(vector(pi()))`.
5. **±Inf → NaN** e histogramas são ignorados.

## 🎓 Na prova PCA

- Entrada e saída: **instant vector**; **radianos**; nome da métrica é **removido**.
- `cos` é **par** (`cos(-x) = cos(x)`) e fica em **[-1, 1]**.
- ±Inf → **NaN**. Histogramas são ignorados.

**1.** Uma métrica `panel_angle_degrees` vale `60`. Qual query retorna `0.5` (aprox.)?
- A) `cos(panel_angle_degrees)`
- B) `cos(deg(panel_angle_degrees))`
- C) `cos(rad(panel_angle_degrees))`
- D) `acos(panel_angle_degrees)`

<details><summary>Resposta</summary>

**C.** `rad()` converte graus para radianos antes do cosseno. A daria cos(60 rad) ≈ -0.95; B converte na direção errada; D daria NaN (60 está fora de [-1, 1]).
</details>

**2.** Qual destas chamadas é **inválida**?
- A) `cos(vector(0))`
- B) `cos(up)`
- C) `cos(rate(http_requests_total[5m]))`
- D) `cos(http_requests_total[5m])`

<details><summary>Resposta</summary>

**D.** `cos()` exige **instant vector**; `x[5m]` é range vector. As outras três passam um instant vector (válido, ainda que sem sentido físico em B e C).
</details>

**3.** `cos(vector(-Inf))` retorna:
- A) `-1`  B) `0`  C) `NaN`  D) erro

<details><summary>Resposta</summary>

**C.** Seguindo o `math.Cos` do Go, cosseno de ±Inf é NaN. Funções PromQL não "dão erro" por valor: devolvem NaN.
</details>

## 📝 Cola rápida

- `cos(v)`: radianos → [-1, 1]; `cos(0) = 1`, `cos(π/2) ≈ 6e-17`, `cos(π) = -1`.
- Potência solar/projeção: `valor * cos(rad(ângulo_graus))`.
- Par: `cos(-x) = cos(x)`. ±Inf → NaN.
- Remove `__name__`; histogramas ignorados.

## 🔗 Relacionadas

[`sin()`](../sin/) · [`tan()`](../tan/) · [`acos()`](../acos/) · [`cosh()`](../cosh/) · [`rad()`](../rad/) · [`deg()`](../deg/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
