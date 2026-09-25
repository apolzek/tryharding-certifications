# `rad()`: de graus para radianos

> **Em uma frase:** `rad(v)` multiplica cada amostra por **π/180** (≈ 0.017453): converte **graus → radianos**, a unidade que `sin()`, `cos()` e `tan()` esperam.

| | |
|---|---|
| **Assinatura** | `rad(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com um **ângulo em graus** (inclinômetros, bússolas, trackers, pitch) · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | radianos |
| **Dashboard** | http://localhost:3300/d/fn-rad |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o adaptador de tomada

Seus sensores entregam **graus**. As funções trigonométricas só "encaixam" **radianos**. O `rad()` é o **adaptador de tomada**:

```
rad(x) = x · π / 180
```

Esquecer o adaptador **não dá erro**: dá um resultado **errado e plausível**. `sin(30)` devolve -0.988 sem reclamar, quando você queria 0.5.

| graus | radianos |
|---|---|
| 1 | 0.017453 |
| 30 | 0.5236 |
| 90 | 1.5708 |
| 180 | 3.1416 (π) |
| 360 | 6.2832 (2π) |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `rad_solar_tracker_tilt_degrees{tracker="fileira-7"}` | gauge | rastreador solar: **-60° → +60°** em 4 min (segue o sol) e volta ao leste |
| `rad_solar_panel_width_meters{tracker="fileira-7"}` | gauge | largura do painel: **2 m** |
| `rad_turbine_blade_pitch_degrees{turbine="T1"}` | gauge | passo das pás: **~2°**; a cada 3 min, 30s **embandeirada a 90°** (proteção contra vento forte) |

**Casos (do básico ao real):** 🟢 **básico:** tracker (graus → rad) · 🟡 **nuance:** esquecer o `rad()` = lixo plausível · 🔴 **real:** sombra do tracker, pitch/yaw de turbina

```bash
curl -s localhost:8088/metrics | grep '^rad_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-rad
```

---

## 🔍 Queries passo a passo

### 1 e 2. Mesma curva, outra escala

```promql
rad_solar_tracker_tilt_degrees
rad(rad_solar_tracker_tilt_degrees)
```

**Resultado esperado:** a mesma dente-de-serra; o eixo muda de **-60..60** para **-1.047..1.047**.

---

### 3. Sombra no chão: com e sem `rad()`

```promql
rad_solar_panel_width_meters * cos(rad(rad_solar_tracker_tilt_degrees))   # certo
rad_solar_panel_width_meters * cos(rad_solar_tracker_tilt_degrees)        # ERRADO
```

**Resultado esperado:** a linha certa é um arco suave: **2 m** com o painel na horizontal (0°) e **1 m** a ±60°. A errada **chacoalha** entre -2 e +2 m: sombra **negativa**, fisicamente absurda.

Uso real: espaçamento entre fileiras de painéis para uma não sombrear a outra.

---

### 4. Pás embandeiradas

```promql
cos(rad(rad_turbine_blade_pitch_degrees))
```

**O que faz:** `cos(pitch)` ≈ fração da pá "de frente" para o vento.
**Resultado esperado:** **0.9994** com pitch de 2°, caindo para **~0** (6.1e-17) nos 30s de embandeiramento a 90°: a turbina "se protege" do vento.

---

### 5. Casos especiais

| Query | Resultado |
|---|---|
| `rad(vector(180))` | `3.141592653589793` |
| `rad(vector(1))` | `0.017453292519943295` |
| `rad(vector(360))` | `6.283185307179586` |
| `sin(rad(vector(30)))` | `0.49999999999999994` |
| `sin(vector(30))` | `-0.9880316240928618` ← sem `rad` |

---

## 🏭 Casos reais

> Como `deg()`, `rad()` é uma **conversão de unidade**: aparece sempre que um exporter de IoT publica ângulos em graus e você precisa de `sin`/`cos`/`tan`.

**1. Usina solar com rastreadores (exporter Modbus do tracker).** O controlador publica `tracker_tilt_degrees`. Para estimar a irradiância efetiva no plano do painel a partir da irradiância horizontal (`weather_ghi_watts_per_m2`) e da elevação do sol:

```yaml
groups:
  - name: solar-tracker
    rules:
      - record: tracker:incidence_cos:ratio
        expr: |
          clamp_min(cos(rad(tracker_tilt_degrees - on() group_left() sun_hour_angle_degrees)), 0)
      - alert: TrackerStuck
        expr: tracker:incidence_cos:ratio < 0.8 and on() (sun_elevation_degrees > 20)
        for: 15m
        annotations:
          summary: "Tracker {{ $labels.tracker }} desalinhado do sol (cos < 0.8)"
```

**2. Parques eólicos:** `rad_turbine_blade_pitch_degrees` e a direção do vento (graus) entram em `cos()`/`sin()` para calcular componentes de força e erro de alinhamento (yaw).

```yaml
groups:
  - name: turbinas
    rules:
      # erro de alinhamento (yaw) entre nacele e vento; modelo simples de perda: cos²
      - record: turbine:yaw_efficiency:ratio
        expr: |
          cos(rad(turbine_nacelle_direction_degrees
                  - on(site) group_left() weather_wind_direction_degrees)) ^ 2
      - alert: TurbineYawMisaligned
        expr: turbine:yaw_efficiency:ratio < 0.9     # ≈ 18° de desalinhamento
        for: 10m
```

## ✅ Quando usar

- **Sempre** que um ângulo em graus for entrar em `sin()`, `cos()` ou `tan()`.
- Converter limites/constantes: `rad(vector(45))`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| O dado já está em radianos | nada (aplicar `rad()` duas vezes encolhe tudo 57x) |
| Quer exibir para pessoas | [`deg()`](../deg/) |
| Resultado de `asin`/`acos`/`atan` (já é radiano) | [`deg()`](../deg/) para ler em graus |

## ⚠️ Pegadinhas

1. **Sem erro, só resultado errado:** `sin(30)` é válido e dá -0.988.
2. **Verifique a unidade do exporter** (nome `_degrees` vs `_radians`, e o HELP da métrica).
3. **Arredondamento:** `sin(rad(30))` = 0.49999999999999994; não compare com `== 0.5`.
4. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- `rad()`: instant vector → instant vector, `x · π/180`; remove o nome da métrica.
- Pegadinha clássica: funções trigonométricas **não** aceitam graus.

**1.** Qual query retorna `1` para um ângulo de 90 graus?
- A) `sin(vector(90))`
- B) `sin(deg(vector(90)))`
- C) `sin(rad(vector(90)))`
- D) `rad(sin(vector(90)))`

<details><summary>Resposta</summary>

**C.** Converter para radianos **antes** do seno.
</details>

**2.** `rad(vector(180))` retorna:
- A) `1`  B) `3.14159...`  C) `10313.2`  D) `0.5`

<details><summary>Resposta</summary>

**B.** 180° = π rad. (C seria `deg(vector(180))`.)
</details>

**3.** Qual é o tipo de retorno de `rad(pi_metric)`?
- A) escalar  B) instant vector  C) range vector  D) string

<details><summary>Resposta</summary>

**B.** Recebe e devolve instant vector.
</details>

## 📝 Cola rápida

- `rad(x) = x · π/180`: **graus → rad**. Inverso: `deg()`.
- Padrão: `sin(rad(x_degrees))`, `cos(rad(...))`, `tan(rad(...))`.
- Esquecer não dá erro, dá **lixo plausível**.
- Remove `__name__`; histogramas ignorados.

## 🔗 Relacionadas

[`deg()`](../deg/) · [`pi()`](../pi/) · [`sin()`](../sin/) · [`cos()`](../cos/) · [`tan()`](../tan/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
