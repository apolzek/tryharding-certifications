# `acos()`: arco-cosseno (o ângulo da escada e a distância no GPS)

> **Em uma frase:** `acos(v)` é o inverso do cosseno: recebe uma **proporção em [-1, 1]** e devolve o **ângulo em radianos** entre **0 e π**. Fora do domínio → **NaN**.

| | |
|---|---|
| **Assinatura** | `acos(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com uma **razão** em [-1, 1] · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | radianos, em [0, π] (use `deg()` para 0°..180°) |
| **Dashboard** | http://localhost:3300/d/fn-acos |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a escada encostada na parede

Uma **escada de 5 m** encostada na parede. Meça a distância do pé da escada até a parede:

```
            |\
            | \   escada (5 m)
   parede   |  \
            |   \  θ = ângulo com o chão
            |____\
             distância

cos(θ) = distância / comprimento     ⇒     θ = acos(distância / comprimento)
```

- Pé a 2.5 m → `acos(0.5)` = **60°** (inclinada demais, perigosa).
- Pé a 1.25 m → `acos(0.25)` ≈ **75.5°** (o ângulo recomendado).
- Pé a 5.5 m → razão 1.1 → **NaN**: essa geometria **não existe**. A escada caiu.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `acos_ladder_base_distance_meters{ladder="segura"}` | gauge | oscila entre **1.0 e 2.5 m** (período 4 min) |
| `acos_ladder_base_distance_meters{ladder="caindo"}` | gauge | escorrega de **1 m até 5.5 m** a cada 3 min (passa de 5 m nos últimos ~20s) |
| `acos_ladder_length_meters{ladder}` | gauge | **5 m** |
| `acos_vehicle_latitude_degrees{vehicle}` / `acos_vehicle_longitude_degrees{vehicle}` | gauge | GPS: `caminhao-1` vai do CD em São Paulo (-23.55, -46.63) até Campinas (-22.91, -47.06) e volta a cada 4 min; `moto-2` parada no CD |

**Casos (do básico ao real):** 🟢 **básico:** escada (distância → ângulo) · 🟡 **nuance:** escada caída → NaN silencia o alerta · 🔴 **real:** distância do CD pelo GPS da frota

```bash
curl -s localhost:8088/metrics | grep '^acos_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-acos
```

---

## 🔍 Queries passo a passo

### 1 e 2. Da distância para o ângulo

```promql
acos_ladder_base_distance_meters
deg(acos(acos_ladder_base_distance_meters / acos_ladder_length_meters))
```

**O que faz:** divide (as séries casam pelo label `ladder`), aplica `acos` e converte para graus.
**Resultado esperado:**

| ladder | distância | ângulo |
|---|---|---|
| segura | 1.0 m | **78.5°** |
| segura | 2.5 m | **60°** |
| caindo | 1 → 5 m | 78.5° → **0°** |
| caindo | > 5 m | **NaN** (buraco no gráfico) |

---

### 3. O alerta que fica mudo

```promql
deg(acos(acos_ladder_base_distance_meters / acos_ladder_length_meters)) < 70
```

**Resultado esperado:** "segura" aparece quando o pé passa de ~1.71 m. "caindo" aparece e vai até 0°... e **some** exatamente quando a escada cai (NaN < 70 é **falso**). O pior momento é justamente quando o alerta desaparece.

---

### 4. Pegando a geometria impossível

```promql
acos_ladder_base_distance_meters / acos_ladder_length_meters > 1
```

**Resultado esperado:** só "caindo", valores entre **1.0 e 1.1**, por ~20s a cada 3 min. Esse é o alerta de "escada caiu".

---

### 5. Caso real: distância em linha reta pelo GPS (lei esférica dos cossenos)

```promql
6371 * acos(clamp(
    sin(rad(acos_vehicle_latitude_degrees)) * scalar(sin(rad(vector(-23.55))))
  + cos(rad(acos_vehicle_latitude_degrees)) * scalar(cos(rad(vector(-23.55))))
    * cos(rad(acos_vehicle_longitude_degrees + 46.63))
, -1, 1))
```

**O que faz:** o cosseno do ângulo central entre dois pontos da Terra é `sin φ1·sin φ2 + cos φ1·cos φ2·cos Δλ`. O `acos` devolve o ângulo (rad) e multiplicar pelo raio da Terra (**6371 km**) dá a distância. O CD (-23.55, -46.63) entra como constante via `scalar(vector(...))`.
**Resultado esperado:** `caminhao-1` sobe de **0 a ~84 km** (Campinas) e volta; `moto-2` fica em **0 km** (`acos(1) = 0`).

> 💡 O `clamp(..., -1, 1)` é uma proteção comum: por arredondamento, o argumento pode sair como `1.0000000000000002` para dois pontos quase iguais, e aí o `acos` daria **NaN** em vez de 0.

---

### 6. Casos especiais

| Query | Resultado |
|---|---|
| `acos(vector(1))` | `0` |
| `deg(acos(vector(0.5)))` | `59.99999999999999` |
| `acos(vector(0))` | `1.5707963267948966` (π/2) |
| `acos(vector(-1))` | `3.141592653589793` (π) |
| `acos(vector(1.01))` | `NaN` ([Go `math.Acos`](https://pkg.go.dev/math#Acos)) |

---

## 🏭 Casos reais

> Trigonometria é **rara** em alertas de infra; `acos` aparece em rastreamento (GPS) e em **similaridade de cosseno**.

**1. Frota/delivery: veículo fora do raio permitido.** Um exporter de telemetria de frota publica `vehicle_latitude_degrees` e `vehicle_longitude_degrees`:

```yaml
groups:
  - name: frota
    rules:
      - record: vehicle:distance_from_depot:km
        expr: |
          6371 * acos(clamp(
              sin(rad(vehicle_latitude_degrees)) * scalar(sin(rad(vector(-23.55))))
            + cos(rad(vehicle_latitude_degrees)) * scalar(cos(rad(vector(-23.55))))
              * cos(rad(vehicle_longitude_degrees + 46.63))
          , -1, 1))
      - alert: VehicleOutsideServiceArea
        expr: vehicle:distance_from_depot:km > 120
        for: 5m
```

**2. Ângulo entre dois vetores (similaridade de cosseno):** se um job exporta a similaridade entre o perfil de tráfego de hoje e o da semana passada (`traffic_profile_cosine_similarity`, entre -1 e 1), `deg(acos(x))` vira um "ângulo de desvio" mais intuitivo: 0° = idêntico, 90° = sem relação.

```yaml
groups:
  - name: perfil-trafego
    rules:
      - record: job:traffic_profile_deviation:degrees
        expr: deg(acos(clamp(traffic_profile_cosine_similarity, -1, 1)))
      - alert: TrafficProfileChanged
        expr: job:traffic_profile_deviation:degrees > 30
        for: 30m
        annotations:
          summary: "Perfil de tráfego do {{ $labels.job }} mudou {{ $value | printf \"%.0f\" }}° em relação à semana passada"
```

## ✅ Quando usar

- Ângulo a partir de "cateto adjacente / hipotenusa" (escada, cabo, braço).
- Distância entre coordenadas geográficas (lei esférica dos cossenos).
- Converter similaridade de cosseno em ângulo.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Tem a razão "cateto oposto / hipotenusa" | [`asin()`](../asin/) |
| Tem a razão "subida / avanço" | [`atan()`](../atan/) |
| Valor ≥ 1 sem limite superior (razão de pico) | [`acosh()`](../acosh/) |

## ⚠️ Pegadinhas

1. **Domínio [-1, 1]:** fora → NaN, sem erro.
2. **NaN silencia alertas:** crie um alerta separado para a razão fora do domínio.
3. **Arredondamento:** `acos(1.0000000000000002)` = NaN. Use `clamp(x, -1, 1)` em fórmulas geométricas.
4. **Radianos:** `deg()` para ler em graus.
5. **Constantes sem labels** não casam com vetores rotulados: use `scalar()`.

## 🎓 Na prova PCA

- Instant vector entra/sai; **radianos**; faixa **[0, π]**; remove `__name__`.
- Fora de [-1, 1] → **NaN**.

**1.** Qual o resultado de `acos(vector(-1))`?
- A) `0`  B) `-π`  C) `π` (3.14159...)  D) `NaN`

<details><summary>Resposta</summary>

**C.** A faixa do `acos` é [0, π]; `cos(π) = -1`.
</details>

**2.** `acos(x)` com `x = 1.5` resulta em:
- A) Erro de avaliação da query
- B) `NaN` para aquela série
- C) `0`
- D) A série é descartada do resultado

<details><summary>Resposta</summary>

**B.** Fora do domínio a amostra vira NaN; a série continua no resultado (só "some" do gráfico e de comparações).
</details>

**3.** `a_ratio` tem label `{ladder="x"}` e `a_len` não tem label nenhuma. Qual expressão funciona?
- A) `acos(a_ratio / a_len)`
- B) `acos(a_ratio / scalar(a_len))`
- C) `acos(a_ratio) / a_len`
- D) `acos(a_ratio[5m] / a_len)`

<details><summary>Resposta</summary>

**B.** Sem labels em comum, a divisão vetor/vetor não encontra pares (A e C retornam vazio). `scalar()` resolve. D passa range vector: inválido.
</details>

## 📝 Cola rápida

- `acos(v)`: domínio **[-1, 1]**, saída **[0, π]** rad.
- `acos(1) = 0`, `acos(0) = π/2`, `acos(-1) = π`; fora → **NaN**.
- Geometria: `deg(acos(adjacente / hipotenusa))`; GPS: `6371 * acos(clamp(..., -1, 1))`.
- Remove `__name__`; histogramas ignorados.

## 🔗 Relacionadas

[`cos()`](../cos/) · [`asin()`](../asin/) · [`atan()`](../atan/) · [`acosh()`](../acosh/) · [`deg()`](../deg/) · [`rad()`](../rad/) · [`clamp()`](../clamp/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
