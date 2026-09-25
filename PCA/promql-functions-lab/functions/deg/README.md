# `deg()`: de radianos para graus

> **Em uma frase:** `deg(v)` multiplica cada amostra por **180/π** (≈ 57.2958): converte **radianos → graus**. Só isso: **não** normaliza para 0..360.

| | |
|---|---|
| **Assinatura** | `deg(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com um **ângulo em radianos** (ou o resultado de `asin`/`acos`/`atan`/`atan2`) · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | graus |
| **Dashboard** | http://localhost:3300/d/fn-deg |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o tradutor

Computadores, bibliotecas de controle (ROS, PLCs) e **todas** as funções trigonométricas do PromQL falam **radianos**: uma volta = **2π ≈ 6.283**. Pessoas, bússolas e dashboards falam **graus**: uma volta = **360**.

```
deg(x) = x · 180 / π
```

| radianos | graus |
|---|---|
| 1 | 57.2958 |
| π/2 ≈ 1.571 | 90 |
| π ≈ 3.142 | 180 |
| 2π ≈ 6.283 | 360 |
| 7 | **401.07** (não "41") |

O `deg()` é um **tradutor literal**: traduz o número, não interpreta. 7 radianos são 401°, mesmo que isso seja "uma volta e mais 41°".

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `deg_antenna_azimuth_radians{antenna="radar-1"}` | gauge | radar girando: **0 → 2π** a cada 2 min e recomeça |
| `deg_robot_arm_joint_radians{joint="ombro"}` | gauge | **±0.785 rad** (±45°), período 3 min |
| `deg_robot_arm_joint_radians{joint="cotovelo"}` | gauge | **0 → 1.571 rad** (0..90°), período 3 min |
| `deg_motor_shaft_position_radians{motor="esteira"}` | gauge | encoder **multi-volta**: acumula 2π a cada 60s, até ~31.4 rad (5 voltas) e zera a cada 5 min |

**Casos (do básico ao real):** 🟢 **básico:** radar (rad → graus) · 🟡 **nuance:** sem `mod 360`; `rad` no lugar de `deg` · 🔴 **real:** juntas de robô vs limite do fabricante

```bash
curl -s localhost:8088/metrics | grep '^deg_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-deg
```

---

## 🔍 Queries passo a passo

### 1 e 2. Azimute do radar

```promql
deg_antenna_azimuth_radians
deg(deg_antenna_azimuth_radians)
```

**Resultado esperado:** a mesma dente-de-serra, mas o eixo muda de **0..6.28** para **0..360°**. Agora dá para ler: 90° = leste, 180° = sul, 270° = oeste.

---

### 3. Juntas do braço robótico

```promql
deg(deg_robot_arm_joint_radians)
```

**Resultado esperado:** ombro entre **-45° e +45°**; cotovelo entre **0° e 90°**.

---

### 4. Encoder multi-volta: `deg()` não faz "mod 360"

```promql
deg(deg_motor_shaft_position_radians)          # acumulado
deg(deg_motor_shaft_position_radians) % 360    # posição dentro da volta
```

**Resultado esperado:** o acumulado sobe em rampa até **~1800°** (5 voltas) e zera a cada 5 min; com `% 360` vira uma dente-de-serra **0..360** que dá uma volta por minuto.

---

### 5. Casos especiais

| Query | Resultado |
|---|---|
| `deg(vector(pi()))` | `180` |
| `deg(vector(1))` | `57.29577951308232` |
| `deg(vector(2 * pi()))` | `360` |
| `deg(vector(7))` | `401.07045659157626` |
| `deg(atan(vector(1)))` | `45` |
| `rad(vector(pi()))` | `0.05483113556160755` ← ❌ **ERRADO**: usou `rad` (graus → rad) num valor que já estava em radianos |

`deg(±Inf)` = ±Inf, `deg(NaN)` = NaN. Não há domínio restrito.

---

## 🏭 Casos reais

> `deg()` é a função "de trigonometria" mais usada na prática, justamente porque é só uma **conversão de unidade** para humanos.

**1. Robótica / automação (exporter de ROS ou PLC).** Um exporter publica `robot_joint_position_radians{robot, joint}`. Para o painel e para o alerta de limite mecânico (o manual diz ±170°):

```yaml
groups:
  - name: robots
    rules:
      - record: robot:joint_position:degrees
        expr: deg(robot_joint_position_radians)
      - alert: RobotJointNearLimit
        expr: abs(robot:joint_position:degrees) > 160
        for: 10s
        annotations:
          summary: "Junta {{ $labels.joint }} do robô {{ $labels.robot }} a {{ $value | printf \"%.0f\" }}° (limite 170°)"
```

Por que converter na recording rule? Porque o limite do fabricante e o operador estão em **graus**: o alerta fica legível.

**2. Depois de `asin`/`acos`/`atan`/`atan2`:** todas devolvem radianos. `deg()` quase sempre embrulha essas funções (ex.: inclinômetro em [`asin()`](../asin/), direção do vento em [`atan()`](../atan/)).

**3. Rumo de veículo a partir das componentes de velocidade (GPS).** Rumo é medido a partir do norte, no sentido horário, então `atan2(leste, norte)`:

```yaml
groups:
  - name: frota
    rules:
      - record: vehicle:heading:degrees
        expr: (deg(vehicle_velocity_east_mps atan2 vehicle_velocity_north_mps) + 360) % 360
```

`deg()` converte, `+ 360` e `% 360` levam o resultado de (-180, 180] para [0, 360).

## ✅ Quando usar

- Mostrar ângulos para pessoas (dashboards, anotações de alerta).
- Converter o resultado de funções trigonométricas inversas.
- Comparar com limites de fabricantes expressos em graus.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Dado **já está em graus** e vai para sin/cos/tan | [`rad()`](../rad/) (a direção oposta) |
| Quer normalizar para 0..360 | `deg(x) % 360` (e `+ 360` antes se x pode ser negativo) |
| Grafana já tem unidade "degree" | ainda precisa do `deg()`: a unidade do Grafana só coloca o símbolo "°", não converte |

## ⚠️ Pegadinhas

1. **Direção:** `deg` = rad → graus. Usar `deg` antes de `sin()` é o erro clássico (o certo é `rad`).
2. **Sem wrap:** `deg(7)` = 401°. Use `% 360`.
3. **Negativos:** `-90 % 360` = -90 em PromQL (o sinal segue o dividendo); para 0..360 use `(deg(x) % 360 + 360) % 360`.
4. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- `deg()`: instant vector → instant vector, `x · 180/π`; remove o nome da métrica.
- As funções trigonométricas do PromQL trabalham em **radianos**: `deg`/`rad` convertem.

**1.** Qual o resultado de `deg(vector(pi()))`?
- A) `3.14159`  B) `57.2958`  C) `180`  D) `360`

<details><summary>Resposta</summary>

**C.** π rad = 180°.
</details>

**2.** Uma métrica `joint_rad` vale `7`. O que `deg(joint_rad)` retorna?
- A) `41.07` (normalizado)  B) `401.07`  C) `NaN` (fora de 0..2π)  D) `0.122`

<details><summary>Resposta</summary>

**B.** `deg` apenas multiplica por 180/π; não há normalização nem domínio restrito.
</details>

**3.** Você quer a inclinação em graus a partir de `ratio` (entre -1 e 1). Qual query?
- A) `asin(deg(ratio))`  B) `deg(asin(ratio))`  C) `rad(asin(ratio))`  D) `asin(ratio) * 180`

<details><summary>Resposta</summary>

**B.** `asin` devolve radianos; `deg` converte. A aplica a conversão antes (e sai do domínio); D esquece o π.
</details>

## 📝 Cola rápida

- `deg(x) = x · 180/π`: **rad → graus**. Inverso: `rad()`.
- Não normaliza: use `% 360`.
- Embrulha `asin`/`acos`/`atan`/`atan2` para leitura humana.
- Remove `__name__`; histogramas ignorados.

## 🔗 Relacionadas

[`rad()`](../rad/) · [`pi()`](../pi/) · [`atan()`](../atan/) · [`asin()`](../asin/) · [`acos()`](../acos/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
