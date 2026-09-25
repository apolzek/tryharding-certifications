# `atan()`: arco-tangente (ângulo de subida, sem NaN)

> **Em uma frase:** `atan(v)` é o inverso da tangente: recebe **qualquer número** (uma inclinação "subida / avanço") e devolve o **ângulo em radianos** entre **-π/2 e π/2**. Não tem domínio restrito.

| | |
|---|---|
| **Assinatura** | `atan(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge ou resultado de `deriv()`/divisão (uma inclinação) · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | radianos, em (-π/2, π/2) ≈ (-90°, +90°) |
| **Dashboard** | http://localhost:3300/d/fn-atan |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o ângulo de subida do avião

Um avião **sobe 3 m** a cada **5 m** que avança. A inclinação é `3 / 5 = 0.6`. Qual o **ângulo** da subida?

```
atan(0.6) = 0.54 rad  →  deg(...) ≈ 31°
```

- `atan()` aceita **qualquer** número: 0 → 0°, 1 → 45°, 1000 → 89.94°, +Inf → 90°.
- Por isso ele também serve como um **"compressor"** que leva (-∞, +∞) para (-90°, +90°).

⚠️ Com `atan(y / x)` você **perde o quadrante**: `(-3)/(-5)` e `3/5` dão o mesmo 0.6. Para ângulos de 0° a 360° (direção do vento, azimute) use o **operador binário `atan2`**: `y atan2 x`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `atan_drone_altitude_meters{drone="inspetor-1"}` | gauge | ciclo de 4 min: **sobe 60s a 3 m/s** (até 180 m), cruza 60s, **desce 60s a 3 m/s**, fica pousado 60s |
| `atan_drone_ground_speed_mps{drone="inspetor-1"}` | gauge | velocidade horizontal **5 m/s** |
| `atan_wind_direction_degrees{station="aeroporto"}` | gauge | vento vindo do **norte**, oscilando entre **345° e 15°** (cruza 0/360 a todo momento) |

**Casos (do básico ao real):** 🟢 **básico:** ângulo de subida do drone (`atan(deriv/vel)`) · 🟡 **nuance:** média de ângulos (`avg` errado vs `atan2`) · 🔴 **real:** direção média do vento

```bash
curl -s localhost:8088/metrics | grep '^atan_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-atan
```

---

## 🔍 Queries passo a passo

### 1 e 2. Ângulo de subida do drone: `atan(deriv(...))`

```promql
atan_drone_altitude_meters
deg(atan(deriv(atan_drone_altitude_meters[30s]) / atan_drone_ground_speed_mps))
```

**O que faz:** [`deriv()`](../deriv/) dá a velocidade vertical (m/s); dividir pela horizontal dá a inclinação da trajetória; `atan` + `deg` transformam em graus.
**Resultado esperado:**

| fase | vertical | inclinação | ângulo |
|---|---|---|---|
| subindo | +3 m/s | 0.6 | **≈ +31°** |
| cruzeiro / pousado | 0 | 0 | **0°** |
| descendo | -3 m/s | -0.6 | **≈ -31°** |

As transições são rampas (a janela `[30s]` mistura as fases).

---

### 3. Direção do vento: o problema do 359 → 0

```promql
atan_wind_direction_degrees
```

**Resultado esperado:** o vento vem **sempre do norte**, mas o número salta entre ~345 e ~15. O gráfico parece um pente.

---

### 4. Média de ângulos: `avg_over_time` (errado) vs `atan2` (certo)

```promql
avg_over_time(atan_wind_direction_degrees[2m])                     # ERRADO

deg(
  avg_over_time(sin(rad(atan_wind_direction_degrees))[2m:5s])
    atan2
  avg_over_time(cos(rad(atan_wind_direction_degrees))[2m:5s])
)                                                                   # certo
```

**O que faz (certo):** transforma cada direção num vetor unitário (`cos`, `sin`), faz a média dos vetores e usa `atan2(média_sin, média_cos)` para recuperar o ângulo no quadrante certo.
**Resultado esperado:** a média ingênua passeia entre **~150° e ~195°** (o **SUL**!), pois a média de 350 e 10 é 180. A média vetorial fica colada em **0°** (dentro de ±1°). Some 360 e aplique `% 360` se quiser só positivos.

---

### 5. Casos especiais

| Query | Resultado |
|---|---|
| `deg(atan(vector(1)))` | `45` |
| `deg(atan(vector(0.6)))` | `30.96...` |
| `atan(vector(+Inf))` | `1.5707963267948966` (π/2): **sem NaN!** |
| `atan(vector(-Inf))` | `-1.5707963267948966` |
| `deg(atan(vector(-1)))` | `-45` |

`atan(NaN)` continua NaN ([Go `math.Atan`](https://pkg.go.dev/math#Atan)).

---

## 🏭 Casos reais

> Trigonometria é **rara** em alertas de infra. `atan` aparece em IoT/meteorologia e, às vezes, como "compressor" de tendência.

**1. Estação meteorológica (aeroporto, usina eólica): direção média do vento.** Exporters de estação (ex.: `weather_wind_direction_degrees`) precisam de **média circular** para painéis e para decidir a orientação da turbina:

```yaml
groups:
  - name: vento
    rules:
      - record: station:wind_direction:avg10m_degrees
        expr: |
          (deg(
            avg_over_time(sin(rad(weather_wind_direction_degrees))[10m:30s])
              atan2
            avg_over_time(cos(rad(weather_wind_direction_degrees))[10m:30s])
          ) + 360) % 360
```

**2. Tendência limitada para score:** `deriv(node_filesystem_avail_bytes[1h])` pode ir de -1e9 a +1e9 bytes/s. `atan(deriv(...) / 1e6) / (pi()/2)` vira um **score entre -1 e 1** ("enchendo rápido" ↔ "esvaziando rápido") sem um pico dominar o gráfico. Alternativa com mesmo espírito: [`tanh()`](../tanh/).

```yaml
groups:
  - name: disk-trend
    rules:
      # -1 = esvaziando muito rápido ... 0 = estável ... +1 = liberando espaço muito rápido
      - record: instance:disk_trend:score
        expr: atan(deriv(node_filesystem_avail_bytes{fstype!="tmpfs"}[1h]) / 1e6) / (pi() / 2)
```

Em 1 MB/s o score vale 0.5; em 10 MB/s, 0.94: dá para colorir um mapa de calor de centenas de discos sem um outlier dominar. Para **alertar**, continue usando [`predict_linear()`](../predict_linear/).

## ✅ Quando usar

- **Ângulo de uma inclinação** (subida/avanço, deriv/velocidade).
- **Comprimir** um valor sem limite para (-π/2, π/2) sem gerar NaN.
- Junto com o **operador `atan2`**, para ângulos em 360° (vento, azimute, média circular).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Precisa do quadrante (0..360°) | operador `y atan2 x` |
| A razão é "oposto / hipotenusa" (limitada a ±1) | [`asin()`](../asin/) |
| Quer só a taxa de variação | [`deriv()`](../deriv/) |
| Quer compressão com saturação em ±1 | [`tanh()`](../tanh/) |

## ⚠️ Pegadinhas

1. **Resultado em radianos:** use `deg()`.
2. **`atan(y/x)` perde o quadrante** e quebra com `x = 0` (divisão por zero → ±Inf → ±90°, ou NaN se `0/0`). Prefira `y atan2 x`.
3. **Média de ângulos:** nunca faça `avg_over_time` de graus que cruzam 0/360.
4. **`atan` é função; `atan2` é operador binário** (como `+`): `a atan2 b`, não `atan2(a, b)`.
5. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- `atan()`: instant vector → instant vector, em **radianos**, faixa (-π/2, π/2). **Não** gera NaN para números reais nem para ±Inf.
- `atan2` é um **operador binário** (precedência igual a `*` e `/`), não uma função.

**1.** Qual expressão é **válida** em PromQL?
- A) `atan2(a, b)`
- B) `a atan2 b`
- C) `atan(a, b)`
- D) `atan(a[5m])`

<details><summary>Resposta</summary>

**B.** `atan2` é operador binário. `atan()` recebe um único instant vector; D passa range vector.
</details>

**2.** `atan(vector(+Inf))` retorna:
- A) `+Inf`  B) `NaN`  C) `1.5708` (π/2)  D) erro

<details><summary>Resposta</summary>

**C.** O limite do arco-tangente no infinito é π/2.
</details>

**3.** Direções de vento 350° e 10° são registradas. `avg_over_time()` delas dá:
- A) 0°  B) 180°  C) 360°  D) NaN

<details><summary>Resposta</summary>

**B.** Média aritmética: (350 + 10) / 2 = 180, o oposto da realidade. Use média de `sin`/`cos` + `atan2`.
</details>

## 📝 Cola rápida

- `atan(v)`: qualquer real → (-π/2, π/2) rad; ±Inf → ±π/2; sem NaN de domínio.
- Ângulo de inclinação: `deg(atan(subida / avanço))`.
- 360°/quadrante: operador `y atan2 x` (não é função!).
- Média circular: `avg(sin)` `atan2` `avg(cos)`.

## 🔗 Relacionadas

[`tan()`](../tan/) · [`asin()`](../asin/) · [`acos()`](../acos/) · [`atanh()`](../atanh/) · [`deriv()`](../deriv/) · [`deg()`](../deg/) · [`tanh()`](../tanh/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
