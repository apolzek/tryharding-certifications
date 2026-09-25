# `log10()`: logaritmo base 10 (ordem de grandeza e decibéis)

> **Em uma frase:** `log10(v)` responde "**10 elevado a quanto dá esse valor?**", ou seja, **conta os zeros**: `log10(1000) = 3`, `log10(0.001) = -3`. Serve para colocar valores que vão de **microssegundos a segundos** na mesma régua, classificar por **ordem de grandeza** e converter potência para **decibéis** (`10 * log10(mW) = dBm`).

| | |
|---|---|
| **Assinatura** | `log10(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões **positivos** (latências, tamanhos, potência) · ⚠️ `0 → -Inf`, negativo → `NaN` · histogramas são ignorados |
| **Unidade do resultado** | "ordens de grandeza" (décadas); `10 × log10(...)` = decibéis |
| **Dashboard** | http://localhost:3300/d/fn-log10 |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: contar os zeros (e a escala Richter)

A **escala Richter** de terremotos é logarítmica: um terremoto 6 é **10×** mais forte que um 5, e 100× mais forte que um 4. Ninguém diria "o terremoto teve 1 000 000 de unidades de amplitude"; diz-se "magnitude 6".

```
 valor (s):   0.0001   0.001   0.01    0.1     1      10
 log10:         -4       -3      -2     -1      0       1
              100 µs    1 ms   10 ms  100 ms   1 s    10 s
              └──────── cada passo = ×10 ─────────────┘
```

Com latências acontece o mesmo: um cache de **200 µs** e um relatório de **8 s** diferem por um fator de 40 000. Num gráfico linear, o cache é uma linha colada no zero. Depois do `log10`, cada um fica na sua "casa": **-3.7** e **0.9**.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `log10_http_request_duration_seconds_p99{endpoint="/cache"}` | recording rule de p99 | gauge | **~0.0002 s** (200 µs, ±8%) |
| `...{endpoint="/health"}` | idem | gauge | **~0.0015 s** (1.5 ms) |
| `...{endpoint="/api/users"}` | idem | gauge | **~0.05 s** (50 ms) |
| `...{endpoint="/search"}` | idem | gauge | **~0.4 s** |
| `...{endpoint="/report"}` | idem | gauge | oscila entre **~1.1 s e ~8.9 s** (3 min) |
| `log10_optical_rx_power_milliwatts{interface="xe-0/0/1"}` | potência óptica RX (SNMP DOM) | gauge | **~0.5 mW** (-3 dBm, saudável) |
| `...{interface="xe-0/0/2"}` | idem | gauge | **~0.05 mW** (-13 dBm) |
| `...{interface="xe-0/0/3"}` | idem | gauge | **degradando** de 0.05 mW a 0.002 mW em 5 min (**-13 → -27 dBm**, fibra suja) |

```bash
curl -s localhost:8088/metrics | grep '^log10_'
# log10_http_request_duration_seconds_p99{endpoint="/cache"} 0.000207
# log10_optical_rx_power_milliwatts{interface="xe-0/0/3"} 0.0113
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-log10
```

Espere **~1 minuto** para os painéis de latência e **~5 min** para ver a fibra degradar inteira.

---

## 🔍 Queries passo a passo

### 🟢 1. p99 cru num eixo linear (o problema)

```promql
log10_http_request_duration_seconds_p99
```

**Resultado esperado:** o `/report` (1.1 a 8.9 s) ocupa o gráfico inteiro; `/search` (0.4 s) aparece baixinho; `/cache`, `/health` e `/api/users` são **uma linha só colada no zero**. Se o `/api/users` dobrar de 50 para 100 ms, **ninguém vai ver**.

---

### 🟢 2. `log10`: todos na mesma régua

```promql
log10(log10_http_request_duration_seconds_p99)
```

**Resultado esperado:**

| endpoint | p99 | `log10` |
|---|---|---|
| /cache | 0.0002 s | **≈ -3.7** |
| /health | 0.0015 s | **≈ -2.8** |
| /api/users | 0.05 s | **≈ -1.3** |
| /search | 0.4 s | **≈ -0.4** |
| /report | 1.1 .. 8.9 s | **0.05 .. 0.95** (onda) |

Agora um `/api/users` que dobrasse apareceria como um degrau de **+0.3** (`log10(2) = 0.301`), bem visível.

---

### 🟡 3. A alternativa só visual: eixo log do Grafana

Mesma query do painel 1, mas com **Axis → Scale → Logarithmic (base 10)** no painel.
**Resultado esperado:** o mesmo desenho do painel 2, mas o eixo continua em **segundos** (100 µs, 1 ms, 10 ms...), o que é mais legível. **Regra prática:** se o objetivo é só **enxergar**, use o eixo log; use `log10()` na **query** quando precisar do número (ordem de grandeza, dB, alertas, `count_values`).

---

### 🟡 4. Ordem de grandeza e bucketing

```promql
count_values("ordem", floor(log10(log10_http_request_duration_seconds_p99)))

# versão do dashboard: "0 - floor(...)" só para os labels ficarem 0, 1, 2, 3, 4 e ordenarem bem
sort_by_label(count_values("ordem", 0 - floor(log10(log10_http_request_duration_seconds_p99))), "ordem")
```

**O que faz:** `floor(log10(x))` dá a **década** do valor (-4 = "centenas de µs", -3 = "ms", -2 = "dezenas de ms"...). `count_values` conta quantos endpoints há em cada década.
**Resultado esperado:** 5 barras com **1** cada: `10^-0 s` (/report, 1..10 s), `10^-1 s` (/search), `10^-2 s` (/api/users), `10^-3 s` (/health), `10^-4 s` (/cache).

---

### 🔴 5 e 6. Decibéis: potência óptica de uma fibra

```promql
log10_optical_rx_power_milliwatts                 # mW, cru
10 * log10(log10_optical_rx_power_milliwatts)     # dBm
```

**O que faz:** **dBm** é "decibéis em relação a 1 mW": `dBm = 10 × log10(P / 1 mW)`. Equipamentos de rede alarmam potência óptica em dBm (tipicamente abaixo de -20 a -25 dBm o link começa a dar erros).
**Resultado esperado:**

| interface | mW | dBm |
|---|---|---|
| xe-0/0/1 | 0.5 | **≈ -3** |
| xe-0/0/2 | 0.05 | **≈ -13** |
| xe-0/0/3 | 0.05 → 0.002 | **-13 → -27** (reta descendo, cruza **-20** na metade do ciclo) |

No painel 5 (mW), a xe-0/0/3 caindo de 0.05 para 0.002 é **invisível** perto dos 0.5 mW da xe-0/0/1. No painel 6 (dBm), é uma **reta descendo** que cruza a linha de alarme: a degradação é "constante em %", e o log a transforma em reta.

---

### ⚠️ 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `log10(vector(1000))` | **3** | 10³ |
| `log10(vector(0.001))` | **-3** | 10⁻³ |
| `log10(vector(1))` | **0** | 10⁰ |
| `log10(vector(0.5))` | **-0.301** | entre 0 e 1 → negativo |
| `log10(vector(0))` | **-Inf** ⚠️ | um endpoint sem tráfego com p99 = 0 vira -Inf |
| `log10(vector(-1))` | **NaN** | negativo |
| `log10(vector(+Inf))` | **+Inf** | |

(A doc diz que os casos especiais de `log10` são **os mesmos do `ln`**.)

---

## 🏭 Casos reais

### 1. Alarme de potência óptica (SNMP exporter)

O time de redes coleta a potência RX dos transceivers via `snmp_exporter` (em mW, ou em milésimos de dBm dependendo do fabricante). Para alarmar com o mesmo limiar do datasheet:

```yaml
groups:
  - name: optics
    rules:
      - record: interface:optical_rx_power:dbm
        expr: 10 * log10(optical_rx_power_milliwatts > 0)

      - alert: FibraSinalFraco
        expr: interface:optical_rx_power:dbm < -20
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.interface }} recebendo {{ $value | printf \"%.1f\" }} dBm (limite -20)"
```

**Decisão:** `> 0` antes do `log10` evita `-Inf` quando o link cai (potência 0). O alerta de link **down** fica em outra regra (`ifOperStatus`).

### 2. "Ordem de grandeza" de latência por endpoint (recording rule)

```yaml
groups:
  - name: latency-magnitude
    rules:
      - record: endpoint:http_request_duration_seconds:p99
        expr: |
          histogram_quantile(0.99,
            sum by (endpoint, le) (rate(http_request_duration_seconds_bucket[5m])))
      - record: endpoint:http_request_duration_seconds:p99_log10
        expr: log10(endpoint:http_request_duration_seconds:p99 > 0)
```

```promql
# endpoints que pioraram UMA ORDEM DE GRANDEZA em relação a ontem
endpoint:http_request_duration_seconds:p99_log10
  - endpoint:http_request_duration_seconds:p99_log10 offset 1d > 1
```

**Decisão:** um limite absoluto ("p99 > 1 s") não serve para endpoints de 200 µs e de 5 s ao mesmo tempo. "**Ficou 10× mais lento**" (`Δlog10 > 1`) serve para todos.

### 3. Tamanho de objetos/respostas em "dígitos"

```promql
count_values("ordem_bytes", floor(log10(max by (bucket) (s3_object_size_bytes) > 0)))
```

Quantos buckets têm objetos na casa dos KB (3), MB (6), GB (9). Bom para descobrir quem está guardando arquivos gigantes.

### 4. SNR / relação sinal-ruído e decibéis em geral

```promql
10 * log10(signal_power_watts / noise_power_watts)     # SNR em dB
20 * log10(voltage_rms_volts / 1)                      # dBV (amplitude usa 20×)
```

Potência usa **10 ×**; amplitude (tensão, pressão sonora) usa **20 ×**, porque potência ∝ amplitude².

---

## ✅ Quando usar

- **Valores que cobrem várias ordens de grandeza** e precisam ser comparados numericamente (alertas de "10× pior", `count_values` por década).
- **Decibéis** (dBm, dB de SNR): `10 * log10(P / P_ref)`.
- **Classificação por magnitude**: `floor(log10(x))` = número de dígitos − 1 (para x ≥ 1).
- **Recording rules** que alimentam comparações relativas (`Δlog10`).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só quer **visualizar** melhor | eixo **Logarithmic** do Grafana (painel 3) |
| Quer "quantas dobras" / potências de 2 | [`log2()`](../log2/) |
| Quer taxa de crescimento contínua / tempo de dobra | [`ln()`](../ln/) |
| Os dados têm zeros e negativos | filtre (`x > 0`) ou use `sgn(x) * log10(abs(x) + 1)` |
| Quer a distribuição completa de latências | histogramas + [`histogram_quantile()`](../histogram_quantile/) |

## ⚠️ Pegadinhas

1. **Zero vira `-Inf`** (endpoint sem tráfego, link caído) e **negativo vira `NaN`**. Filtre com `x > 0` **dentro** do `log10`.
2. **Unidade importa:** `log10` de segundos ≠ `log10` de milissegundos (diferença de 3). Documente a unidade.
3. **Médias no espaço log** são médias **geométricas** (`10 ^ avg(log10(x))`), não aritméticas.
4. **10× vs 20×** em decibéis: potência → 10, amplitude → 20.
5. **`floor(log10(x))` perto de potências de 10** oscila: um valor em torno de 1 ms (0.00098 ↔ 0.00102) fica pulando entre -4 e -3. Por isso o `/health` do lab está em 1.5 ms.
6. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- Casos especiais iguais aos do `ln`: `log10(0) = -Inf`, `log10(<0) = NaN`, `log10(+Inf) = +Inf`, `log10(NaN) = NaN`.
- `log10` recebe **instant vector**; `log10(100)` com escalar literal é **erro de parse**.
- Diferença entre transformar o dado (`log10()` na query) e só mudar a visualização (eixo log no Grafana).
- Inverso: `10 ^ log10(x) = x` (operador `^`).

**1.** Quanto vale `log10(vector(0.01))`?

- A) 2
- B) -2
- C) 0.01
- D) NaN

<details><summary>Resposta</summary>

**B.** 10⁻² = 0.01.
</details>

**2.** Um p99 foi de 20 ms para 200 ms. Quanto variou `log10(p99)`?

- A) +180
- B) +10
- C) +1
- D) +0.18

<details><summary>Resposta</summary>

**C.** O valor ficou 10× maior; em log10, multiplicar por 10 soma 1 (`log10(0.2) - log10(0.02) = -0.7 - (-1.7) = 1`).
</details>

**3.** Qual expressão converte uma potência em **mW** para **dBm**?

- A) `log10(x) * 20`
- B) `10 * log10(x)`
- C) `10 ^ x`
- D) `ln(x) / 10`

<details><summary>Resposta</summary>

**B.** dBm = 10 × log10(P / 1 mW). O fator 20 é para grandezas de **amplitude** (tensão).
</details>

**4.** A métrica `link_rx_power_mw` vale `0` quando o link cai. O que acontece com `10 * log10(link_rx_power_mw)` nesse momento, e como evitar?

- A) Erro de query; use `try()`
- B) Resultado `-Inf`; filtre com `log10(link_rx_power_mw > 0)`
- C) Resultado `0`; nada a fazer
- D) Resultado `NaN`; use `abs()`

<details><summary>Resposta</summary>

**B.** `log10(0) = -Inf` (sem erro). O filtro `> 0` remove a série nesse instante; o link caído deve ser tratado por outro alerta. `NaN` seria para valores **negativos**.
</details>

## 📝 Cola rápida

- `log10(v)` "conta os zeros": `log10(1000) = 3`, `log10(0.001) = -3`, `log10(1) = 0`.
- `log10(0) = -Inf`, `log10(<0) = NaN` (iguais ao `ln`); filtre `x > 0` dentro.
- Ordem de grandeza: `floor(log10(x))`; "10× pior": `Δlog10 > 1`.
- Decibéis: potência `10 * log10(P/Pref)`, amplitude `20 * log10(A/Aref)`.
- Só pra enxergar? Eixo log do Grafana.

## 🔗 Relacionadas

[`ln()`](../ln/) · [`log2()`](../log2/) · [`exp()`](../exp/) · [`floor()`](../floor/) · [`histogram_quantile()`](../histogram_quantile/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#log10
