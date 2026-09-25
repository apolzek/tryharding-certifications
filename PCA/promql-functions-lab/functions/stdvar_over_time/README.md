# `stdvar_over_time()`: a variância de um gauge dentro da janela

> **Em uma frase:** `stdvar_over_time(v[janela])` calcula a **variância populacional** das amostras de cada série na janela, ou seja, a **média dos quadrados das distâncias até a média**. É exatamente o `stddev_over_time()` **ao quadrado**.

| | |
|---|---|
| **Assinatura** | `stdvar_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge · ❌ Counter cru · ❌ Histogram (amostras de histograma são ignoradas) |
| **Unidade do resultado** | a unidade da métrica **ao quadrado** (V → V², s → s²) |
| **Dashboard** | http://localhost:3300/d/fn-stdvar_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a área dos quadradinhos

Desenhe cada amostra como um ponto e a média como uma linha horizontal. Para cada ponto, desenhe um **quadrado** cujo lado é a distância até a média.

```
 30 ●───┐            ●            lado = 10  →  área = 100
        │ 10
 20 ────┴──── média ──────────
        │ 10
 10     └───●            ●        lado = 10  →  área = 100
```

- **Variância** = a **área média** desses quadrados (V²).
- **Desvio padrão** = o **lado** de um quadrado com essa área média (√variância, em V).

Por que existir uma função que devolve "área" em vez de "lado"? Porque **áreas se somam**: a variância da soma de duas fontes **independentes** é a soma das variâncias (`σ²ₜₒₜₐₗ = σ²ₐ + σ²ᵦ`). Desvios padrão **não** se somam.

```
                 Σ (xᵢ − média)²
  σ²  =   ────────────────────          (populacional: divide por N)
                      N
```

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita o `node_hwmon_in_volts` do **node_exporter** (tensões lidas pelos sensores da placa-mãe). Tudo **determinístico** (sem ruído), para os números saírem redondos. Todos têm **média 20V**:

| Métrica | Tipo | Comportamento | Variância esperada | Desvio |
|---|---|---|---|---|
| `stdvar_over_time_node_hwmon_in_volts{sensor="square_big"}` | gauge | onda quadrada **10V ↔ 30V** (30s em cada) | **100 V²** | 10 V |
| `stdvar_over_time_node_hwmon_in_volts{sensor="square_small"}` | gauge | onda quadrada **18V ↔ 22V** (30s em cada) | **4 V²** | 2 V |
| `stdvar_over_time_node_hwmon_in_volts{sensor="flat"}` | gauge | constante **20V** | **0** | 0 |

```bash
curl -s localhost:8088/metrics | grep '^stdvar_over_time_'
# stdvar_over_time_node_hwmon_in_volts{sensor="flat"} 20
# stdvar_over_time_node_hwmon_in_volts{sensor="square_big"} 30
# stdvar_over_time_node_hwmon_in_volts{sensor="square_small"} 22
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-stdvar_over_time
```

Espere **~5 min** para a janela `[5m]` encher.

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
stdvar_over_time_node_hwmon_in_volts
```

**Resultado esperado:** duas ondas quadradas (uma alta 10↔30, uma baixinha 18↔22) trocando de nível a cada 30s, e uma linha reta em 20.

---

### 2. A média não diferencia nada

```promql
avg_over_time(stdvar_over_time_node_hwmon_in_volts[5m])
```

**Resultado esperado:** as três linhas em ≈ **20V**.

---

### 3. A variância

```promql
stdvar_over_time(stdvar_over_time_node_hwmon_in_volts[5m])
```

**O que faz:** para `square_big`, metade das 60 amostras vale 10 e metade vale 30. Cada uma está a **10** da média → cada quadrado tem área **100** → a média das áreas é **100**.
**Resultado esperado:**

| sensor | stdvar |
|---|---|
| `square_big` | **100** (V²) |
| `square_small` | **4** (V²) |
| `flat` | **0** |

> 💡 Repare: a amplitude do `square_big` é **5× maior** que a do `square_small` (10 vs 2), mas a variância é **25× maior** (100 vs 4). Variância cresce com o **quadrado** da dispersão.

---

### 4. `sqrt(stdvar)` = `stddev`

```promql
sqrt(stdvar_over_time(stdvar_over_time_node_hwmon_in_volts[5m]))
stddev_over_time(stdvar_over_time_node_hwmon_in_volts[5m])
```

**Resultado esperado:** as duas queries dão **exatamente** o mesmo valor por sensor: **10 / 2 / 0 V**. As linhas se sobrepõem.

---

### 5. Janela que não cobre um ciclo inteiro

```promql
stdvar_over_time(stdvar_over_time_node_hwmon_in_volts{sensor="square_big"}[20s])   # ~4 amostras
stdvar_over_time(stdvar_over_time_node_hwmon_in_volts{sensor="square_big"}[5m])    # ~60 amostras
```

**Resultado esperado:**
- `[5m]`: linha reta em **100**.
- `[20s]`: pula entre **0** (as 4 amostras no mesmo nível), **75** (3 num nível e 1 no outro: média 15 ou 25, desvios 5,5,5,15 → (25+25+25+225)/4 = 75) e **100** (2 e 2). Com passo de 15s no gráfico, você vê quase só 0 e 100; o 75 aparece de vez em quando, num único ponto.

**Moral:** a janela precisa ser **bem maior** que o "ritmo" da variação que você quer medir.

---

### 6. Tabela: variância e desvio por sensor

```promql
stdvar_over_time(stdvar_over_time_node_hwmon_in_volts[5m])
stddev_over_time(stdvar_over_time_node_hwmon_in_volts[5m])
```

**Resultado esperado:** `square_big` 100 / 10 · `square_small` 4 / 2 · `flat` 0 / 0.

---

### 7. 🔴/🟡 Alerta "σ > 3V": limiar certo vs errado

```promql
stdvar_over_time(stdvar_over_time_node_hwmon_in_volts[5m]) > 9    # ✅ 9 = 3²
stdvar_over_time(stdvar_over_time_node_hwmon_in_volts[5m]) > 3    # ❌ limiar pensado em V
```

**Resultado esperado** (tabela):

| sensor | ✅ `> 9` | ❌ `> 3` |
|---|---|---|
| square_big (σ = 10V) | **100** (dispara, correto) | **100** |
| square_small (σ = 2V) | — | **4** (dispara **indevidamente**: σ = 2V < 3V) |
| flat | — | — |

**Moral:** limiar de variância é o **quadrado** do limiar de desvio. Na dúvida, alerte em `stddev_over_time` ou em `sqrt(stdvar_over_time(...))`.

---

## 🏭 Casos reais

### 1. Ripple da fonte de alimentação (node_exporter)

O time de datacenter quer saber se a linha de 12V de um servidor está **instável** (fonte pifando). `node_hwmon_in_volts` é gauge:

```yaml
- alert: PSUVoltageUnstable
  expr: stdvar_over_time(node_hwmon_in_volts{sensor="in1"}[15m]) > 0.04   # σ > 0.2V
  for: 15m
  annotations:
    summary: "Tensão de {{ $labels.instance }}/{{ $labels.sensor }} oscilando (σ² > 0.04 V²)"
```

**Decisão:** o limiar em V² é o quadrado do limiar em V (0,2² = 0,04). Muita gente erra isso e põe `> 0.2`, que equivale a σ > 0,45V.

### 2. Variância de latência de ponta a ponta (somando componentes)

Um request passa pelo **gateway** e pelo **backend**. Se as latências são independentes, a variância total é a soma:

```yaml
- record: service:e2e_latency_seconds:stdvar_10m
  expr: |
    stdvar_over_time(gateway_latency_seconds[10m])
    + on(service) stdvar_over_time(backend_latency_seconds[10m])
- record: service:e2e_latency_seconds:stddev_10m
  expr: sqrt(service:e2e_latency_seconds:stdvar_10m)
```

Somar os `stddev` direto daria um valor **maior** que o real (σ não é aditivo).

### 3. "Variância média" de uma frota

Para um relatório "quão instável é o CPU dos nodes, em média", a média das variâncias é estatisticamente correta; a média dos desvios padrão não:

```promql
sqrt(avg(stdvar_over_time(instance:node_cpu_utilisation:rate5m[1h])))
```

---

## ✅ Quando usar

- **Somar dispersões** de componentes independentes e só no fim tirar `sqrt()`.
- **Juntar variâncias de várias séries** (média das variâncias é válida; média de desvios padrão, não).
- **Entrada para cálculos estatísticos** em recording rules.
- Quando você quer **amplificar** a diferença entre séries pouco e muito instáveis (o quadrado exagera).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Mostrar para humanos "o quanto oscila" (unidade legível) | [`stddev_over_time()`](../stddev_over_time/) |
| Dados com outliers | [`mad_over_time()`](../mad_over_time/) |
| "95% do tempo fica abaixo de..." | [`quantile_over_time()`](../quantile_over_time/) |
| Variância entre séries num instante | operador de agregação `stdvar(x)` |
| Variância de um histogram (distribuição de requisições) | [`histogram_stdvar()`](../histogram_stdvar/) |

## ⚠️ Pegadinhas

1. **Unidade ao quadrado**: 100 V² **não** é "100 volts". No Grafana, não use unidade `volt` no painel de variância.
2. **Populacional (÷N)**, não amostral (÷N−1).
3. **Outliers dominam**: um único ponto a 100 da média contribui 10.000 para a soma.
4. **Janela curta** → resultado dependente de fase (query 5).
5. **Variâncias só se somam se as fontes forem independentes.**
6. **Histogramas nativos são ignorados** (só amostras float).

---

## 🎓 Na prova PCA

O que costuma cair:
- `stdvar_over_time` = `stddev_over_time`² (e `stdvar` operador = `stddev` operador²).
- Range vector de entrada, instant vector de saída, **por série**.
- Diferença `_over_time` (tempo) vs operador de agregação (entre séries).
- A unidade do resultado é ao quadrado.

**1.** `stdvar_over_time(x[10m])` retornou `25`. Qual o desvio padrão?

- A) 625
- B) 12.5
- C) 5
- D) 25

<details><summary>Resposta</summary>

**C.** σ = √25 = 5. Em PromQL: `sqrt(stdvar_over_time(x[10m]))`, que é igual a `stddev_over_time(x[10m])`.
</details>

**2.** Qual expressão é **equivalente** a `stddev_over_time(x[5m])`?

- A) `stdvar_over_time(x[5m]) ^ 2`
- B) `sqrt(stdvar_over_time(x[5m]))`
- C) `stdvar(x[5m])`
- D) `sqrt(stdvar(x))`

<details><summary>Resposta</summary>

**B.** O desvio padrão é a raiz da variância. C) o operador `stdvar` não aceita range vector. D) calcula entre séries num instante.
</details>

**3.** Qual a diferença entre `stdvar(node_load1)` e `stdvar_over_time(node_load1[1h])`?

- A) Nenhuma.
- B) O primeiro devolve uma série com a variância **entre os nodes** agora; o segundo devolve **uma série por node** com a variância ao longo de 1h.
- C) O primeiro é populacional e o segundo é amostral.
- D) O segundo junta todos os nodes numa série.

<details><summary>Resposta</summary>

**B.** Operadores de agregação trabalham entre séries num instante; `_over_time` trabalha no tempo, série a série. Ambos são populacionais.
</details>

**4.** Um sensor ficou **constante** a janela inteira. `stdvar_over_time` retorna:

- A) `NaN`
- B) `0`
- C) vetor vazio
- D) o próprio valor

<details><summary>Resposta</summary>

**B.** Sem variação, todas as distâncias até a média são 0 → variância 0. (Só vira vazio se não houver nenhuma amostra na janela.)
</details>

---

## 📝 Cola rápida

- `stdvar_over_time(gauge[janela])` = variância **populacional** por série = `stddev_over_time²`.
- Unidade **ao quadrado** (s², V²): limiares de alerta também ao quadrado.
- Variâncias de fontes independentes **somam**; desvios padrão não. Some variâncias, depois `sqrt()`.
- `stdvar(x)` operador = entre séries; `stdvar_over_time(x[w])` = no tempo.
- Série constante → 0; janela sem amostras → sem resultado.

## 🔗 Relacionadas

[`stddev_over_time()`](../stddev_over_time/) · [`mad_over_time()`](../mad_over_time/) · [`avg_over_time()`](../avg_over_time/) · [`histogram_stdvar()`](../histogram_stdvar/) · [`sqrt()`](../sqrt/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
