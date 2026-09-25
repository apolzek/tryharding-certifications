# `asin()`: arco-seno (do sensor de volta ao ângulo)

> **Em uma frase:** `asin(v)` é o inverso do seno: recebe uma **proporção entre -1 e 1** e devolve o **ângulo em radianos** (entre -π/2 e π/2). Fora de [-1, 1] o resultado é **NaN**.

| | |
|---|---|
| **Assinatura** | `asin(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com uma **razão** em [-1, 1] · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | radianos, em [-π/2, π/2] ≈ [-1.571, 1.571] (use `deg()` para graus) |
| **Dashboard** | http://localhost:3300/d/fn-asin |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o inclinômetro

O `sin()` responde "com esse ângulo, qual a altura?". O `asin()` faz a pergunta **ao contrário**: "com essa altura (proporção), qual o ângulo?".

Um **acelerômetro parado** só sente a gravidade. Se ele está inclinado de θ, o eixo X "pega" uma fatia da gravidade:

```
a_x = 9.81 · sin(θ)      ⇒      θ = asin(a_x / 9.81)
```

É assim que celulares, guindastes e betoneiras sabem sua inclinação.

E se a leitura passar de 9.81? Isso significaria sin(θ) > 1, que **não existe**. O `asin()` devolve **NaN**. Acontece de verdade: com **vibração**, o sensor mede gravidade + trepidação.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `asin_accelerometer_x_mps2{device="guindaste"}` | gauge | inclinação oscila **-60° ↔ +60°** (período 3 min) → a_x entre **-8.5 e +8.5** m/s² |
| `asin_accelerometer_x_mps2{device="betoneira"}` | gauge | inclinação **30°** (a_x ≈ 4.9); a cada 5 min passa **40s vibrando**: leituras de 2.5 a 14.5 m/s² (**acima de 9.81 = fora do domínio**) |
| `asin_gravity_mps2` | gauge | constante **9.81** |

**Casos (do básico ao real):** 🟢 **básico:** guindaste (inclinômetro → ângulo) · 🟡 **nuance:** vibração → fora de [-1, 1] → NaN; `clamp` · 🔴 **real:** alertas de inclinação + sensor inválido

```bash
curl -s localhost:8088/metrics | grep '^asin_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-asin
```

Espere **~5 min** para ver ao menos uma janela de vibração da betoneira.

---

## 🔍 Queries passo a passo

### 1. Leitura crua do sensor

```promql
asin_accelerometer_x_mps2
asin_gravity_mps2
```

**Resultado esperado:** o guindaste é uma onda entre ±8.5; a betoneira é uma linha em ~4.9 com **rajadas** que passam da linha azul de 9.81.

---

### 2. Inclinação em graus

```promql
deg(asin(asin_accelerometer_x_mps2 / scalar(asin_gravity_mps2)))
```

**O que faz:** divide pela gravidade (proporção -1..1), aplica `asin` (radianos) e converte com `deg()`.
`scalar()` é necessário porque `asin_gravity_mps2` não tem o label `device`: sem ele, a divisão vetor/vetor não encontraria pares.

**Resultado esperado:**
- guindaste: onda suave entre **-60° e +60°**;
- betoneira: **30°** estável; durante a vibração o gráfico tem **buracos** (NaN) e picos até ~90°.

---

### 3. Contando leituras fora do domínio

```promql
count_over_time((abs(asin_accelerometer_x_mps2 / scalar(asin_gravity_mps2)) > 1)[5m:5s])
```

**O que faz:** o filtro `> 1` só deixa passar amostras inválidas; `count_over_time` conta quantas houve nos últimos 5 min.
**Resultado esperado:** só aparece a betoneira, com **2 a 5 leituras** por janela de vibração. Sem vibração recente, a série some (resultado vazio).

---

### 4. "Consertando" com `clamp`

```promql
deg(asin(clamp(asin_accelerometer_x_mps2{device="betoneira"} / scalar(asin_gravity_mps2), -1, 1)))
```

**Resultado esperado:** sem buracos; as leituras inválidas viram **90°**. ⚠️ Isso **esconde** a vibração: é bom para um gráfico bonito, ruim se a vibração é o problema que você quer detectar.

---

### 5. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `deg(asin(vector(0.5)))` | `30.000000000000004` | sujeira de float |
| `asin(vector(1))` | `1.5707963267948966` (π/2) | |
| `asin(vector(-1))` | `-1.5707963267948966` | |
| `asin(vector(1.0001))` | `NaN` | nem 0.01% fora do domínio é tolerado ([Go `math.Asin`](https://pkg.go.dev/math#Asin)) |
| `asin(vector(-2))` | `NaN` | |

---

## 🏭 Casos reais

> Trigonometria é **rara** em alertas de infra; `asin` aparece em IoT industrial (inclinômetros, acelerômetros).

**1. Canteiro de obras: guindaste inclinado.** Um gateway IoT publica `crane_accel_x_mps2` por guindaste. Alerta se a inclinação passar de 5°, e alerta **separado** para leituras impossíveis (sensor com vibração ou defeito):

```yaml
groups:
  - name: guindastes
    rules:
      - record: crane:tilt:degrees
        expr: deg(asin(crane_accel_x_mps2 / 9.81))
      - alert: CraneTiltTooHigh
        expr: abs(crane:tilt:degrees) > 5
        for: 30s
      - alert: CraneSensorOutOfRange
        expr: abs(crane_accel_x_mps2 / 9.81) > 1
        for: 1m
        annotations:
          summary: "Leitura > 1 g no eixo X: vibração ou sensor com defeito (asin = NaN)"
```

Por que o segundo alerta? Porque **NaN falha em toda comparação**: `NaN > 5` é falso. Sem ele, um sensor quebrado deixaria o primeiro alerta **mudo**.

**2. Painéis solares com seguidor (tracker):** o controlador publica a componente vertical do vetor solar; `asin()` dá a **elevação do sol**.

```yaml
groups:
  - name: solar-tracker
    rules:
      - record: sun:elevation:degrees
        expr: deg(asin(clamp(sun_vector_z, -1, 1)))
      - alert: TrackerMovingAtNight
        expr: abs(deriv(tracker_tilt_degrees[5m])) > 0.01 and on() (sun:elevation:degrees < -5)
        for: 10m
        annotations:
          summary: "Tracker {{ $labels.tracker }} se movendo com o sol abaixo do horizonte"
```

## ✅ Quando usar

- Recuperar um **ângulo** a partir de uma **proporção** (acelerômetro, componente vertical, razão cateto/hipotenusa).
- Validar sensores: `abs(x) > 1` antes do `asin` indica leitura impossível.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Tem a razão "cateto adjacente / hipotenusa" | [`acos()`](../acos/) |
| Tem a razão "subida / avanço" (sem limite) | [`atan()`](../atan/) |
| Valor pode ser qualquer número real | [`asinh()`](../asinh/) (sem domínio restrito) |
| Quer só limitar valores | [`clamp()`](../clamp/) |

## ⚠️ Pegadinhas

1. **Domínio [-1, 1]:** fora disso, **NaN** (não erro). Normalize antes: `x / máximo`.
2. **NaN some do gráfico** e **nunca** dispara alerta (`NaN > x` é sempre falso).
3. **Resultado em radianos:** embrulhe em `deg()` para ler em graus.
4. **Só dá -90°..+90°:** `asin` não distingue 30° de 150° (ambos têm seno 0.5).
5. **Divisão entre vetores precisa casar labels:** use `scalar()` ou `on()`/`ignoring()` para a constante.

## 🎓 Na prova PCA

- Entrada e saída: **instant vector**; resultado em **radianos**; remove `__name__`.
- Fora de [-1, 1] → **NaN** (não é erro de query).
- Faixa de saída: **[-π/2, π/2]**.

**1.** Qual o resultado de `asin(vector(2))`?
- A) Erro: domain error
- B) `NaN`
- C) `1.5708`
- D) Resultado vazio

<details><summary>Resposta</summary>

**B.** Funções PromQL não abortam a query por valor inválido; o resultado da amostra vira **NaN**.
</details>

**2.** Um alerta `deg(asin(x / 9.81)) > 45` está configurado. O sensor começa a enviar `x = 12`. O que acontece?
- A) O alerta dispara, pois 12/9.81 > 1
- B) O alerta **não** dispara: o valor é NaN e a comparação é falsa
- C) A regra falha com erro
- D) O Prometheus descarta a amostra na ingestão

<details><summary>Resposta</summary>

**B.** `asin(1.22)` = NaN, e comparações com NaN são sempre falsas: a série é filtrada. Por isso crie um alerta separado para `abs(x/9.81) > 1`.
</details>

**3.** Qual query retorna `30` (aprox.)?
- A) `asin(vector(0.5))`
- B) `rad(asin(vector(0.5)))`
- C) `deg(asin(vector(0.5)))`
- D) `asin(deg(vector(0.5)))`

<details><summary>Resposta</summary>

**C.** `asin(0.5)` = π/6 rad ≈ 0.5236; `deg()` converte para 30°.
</details>

## 📝 Cola rápida

- `asin(v)`: domínio **[-1, 1]**, saída **[-π/2, π/2]** rad.
- Fora do domínio → **NaN** → buraco no gráfico e alerta mudo.
- Graus: `deg(asin(x))`. Inclinômetro: `deg(asin(a / 9.81))`.
- Remove `__name__`; histogramas ignorados.

## 🔗 Relacionadas

[`sin()`](../sin/) · [`acos()`](../acos/) · [`atan()`](../atan/) · [`asinh()`](../asinh/) · [`deg()`](../deg/) · [`clamp()`](../clamp/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
