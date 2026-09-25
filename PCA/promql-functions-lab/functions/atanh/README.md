# `atanh()`: tangente hiperbólica inversa (domínio -1 < x < 1)

> **Em uma frase:** `atanh(v)` desfaz o `tanh()`: **estica** valores de (-1, 1) de volta para toda a reta real. Em **±1** dá **±Inf**; fora de [-1, 1] dá **NaN**.

| | |
|---|---|
| **Assinatura** | `atanh(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge em (-1, 1): **utilização 0..1**, **coeficiente de correlação**, score normalizado · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | adimensional, (-∞, +∞) |
| **Dashboard** | http://localhost:3300/d/fn-atanh |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: esticar a mola de volta

O [`tanh()`](../tanh/) é um **amortecedor** que espreme tudo em (-1, 1). O `atanh()` **estica a mola de volta**: no meio quase não muda nada, mas perto das bordas cada milímetro vira quilômetros.

| x | atanh(x) |
|---|---|
| 0.3 | 0.31 |
| 0.5 | 0.55 |
| 0.9 | 1.47 |
| 0.99 | 2.65 |
| 0.999 | 3.80 |
| 1 | **+Inf** |
| 1.02 | **NaN** |

Isso é ótimo para um **"índice de estresse"**: a diferença entre 90% e 99% de uso é muito mais grave que entre 30% e 39%, e o `atanh` mostra isso.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `atanh_disk_utilization_ratio{disk="/data"}` | gauge | enche de **30% → 99.9%** em 3 min, fica **30s em 100%** (1.0 exato), é limpo (volta a 30%) |
| `atanh_correlation_coefficient{pair}` | gauge | correlação de Pearson por par: `cpu~latencia` ~0.8, `gc~latencia` ~0.6, `fila~latencia` ~0.97, `cache~latencia` ~-0.5 (todos oscilando) |
| `atanh_buggy_correlation_coefficient{pair="disco~latencia"}` | gauge | job com bug: 0.88 ↔ **1.02** (passa de 1 em ~1/4 do tempo) |

**Casos (do básico ao real):** 🟢 **básico:** estresse de utilização do disco · 🟡 **nuance:** ±1 = ±Inf, |x| > 1 = NaN · 🔴 **real:** média de correlações por Fisher

```bash
curl -s localhost:8088/metrics | grep '^atanh_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-atanh
```

---

## 🔍 Queries passo a passo

### 1 e 2. Estresse do disco

```promql
atanh_disk_utilization_ratio
atanh(atanh_disk_utilization_ratio)
```

**Resultado esperado:** o uso é uma rampa **reta**; o estresse é uma curva que **dispara** no fim: 50% → **0.55**, 90% → **1.47**, 99.9% → **3.8**; em 100% vira **+Inf** e sai do gráfico. Depois da limpeza (30%) volta para **0.31**.

---

### 3. Média de correlações: a transformação de Fisher

```promql
avg(atanh_correlation_coefficient)                       # ingênua
tanh(avg(atanh(atanh_correlation_coefficient)))          # Fisher
```

**O que faz:** coeficientes de correlação **não** devem ser somados/médios diretamente (a escala é distorcida perto de ±1). A estatística clássica: leve para a escala de Fisher com `atanh`, faça a média, volte com `tanh`.
**Resultado esperado:** a média ingênua oscila entre **0.35 e 0.59**; a de Fisher, entre **0.63 e 0.76**, sempre acima: correlações fortes (fila~latencia ≈ 0.97) pesam mais, como deveriam.

---

### 4. Correlação com bug

```promql
atanh_buggy_correlation_coefficient
atanh(atanh_buggy_correlation_coefficient)
```

**Resultado esperado:** o `r` cru passa de 1.0 às vezes (impossível para Pearson). O `atanh(r)` sobe até ~3.5 e **some** (NaN) sempre que `r > 1`: você vê exatamente os trechos inválidos.

---

### 5. Casos especiais

| Query | Resultado |
|---|---|
| `atanh(vector(0.5))` | `0.5493061443340548` |
| `atanh(vector(0.99))` | `2.6466524123622457` |
| `atanh(vector(1))` | `+Inf` |
| `atanh(vector(-1))` | `-Inf` |
| `atanh(vector(1.02))` | `NaN` ([Go `math.Atanh`](https://pkg.go.dev/math#Atanh)) |
| `tanh(atanh(vector(0.5)))` | `0.49999999999999994` (ida e volta com arredondamento) |

---

## 🏭 Casos reais

> Sinceridade: `atanh` é **rara** em observabilidade. Os usos são estatísticos (Fisher) e de "índice de estresse".

**1. Índice de estresse de disco (node_exporter).**

```yaml
groups:
  - name: disk-stress
    rules:
      - record: instance:disk_used:ratio
        expr: 1 - node_filesystem_avail_bytes{fstype!="tmpfs"} / node_filesystem_size_bytes{fstype!="tmpfs"}
      - record: instance:disk_stress:atanh
        expr: atanh(clamp_max(instance:disk_used:ratio, 0.9999))
```

O `clamp_max(..., 0.9999)` evita +Inf em disco 100% cheio (limita o estresse a ~4.95). **Alertas** continuam no ratio cru (`> 0.9`); o índice é para ranking/mapa de calor ("quais discos estão *realmente* no limite").

**2. Análise de correlação entre métricas.** Um job de análise (ex.: exporter de um notebook de capacidade) publica `metric_pair_correlation{a, b}`. Para a correlação **média por serviço**, use Fisher: `tanh(avg by (service) (atanh(metric_pair_correlation)))`.

```yaml
groups:
  - name: correlacoes
    rules:
      # clamp evita ±Inf (r = ±1) e NaN (bug com |r| > 1) antes da média de Fisher
      - record: service:latency_correlation:fisher_avg
        expr: tanh(avg by (service) (atanh(clamp(metric_pair_correlation, -0.9999, 0.9999))))
```

## ✅ Quando usar

- Desfazer um [`tanh()`](../tanh/).
- Média/soma de **coeficientes de correlação** (Fisher).
- "Índice de estresse" que explode perto de 100% de utilização.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Valor pode passar de 1 (ex.: utilização > 100% em CPU com vários cores) | normalize antes, ou [`asinh()`](../asinh/) |
| Alertas com limiar | o valor cru |
| Ângulo | [`atan()`](../atan/) (sem "h") |

## ⚠️ Pegadinhas

1. **Domínio (-1, 1):** ±1 → ±Inf; fora → NaN.
2. **Utilização de 100% é comum** (disco cheio, CPU saturada): proteja com `clamp_max(x, 0.9999)`.
3. **Unidade errada:** se o dado está em **percentual** (0..100), divida por 100 antes, senão tudo vira NaN.
4. Não confundir com `atan()` (que aceita qualquer real).
5. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- Instant vector → instant vector; domínio **[-1, 1]** (±1 → ±Inf); fora → **NaN**.
- Inversa de `tanh`.

**1.** `atanh(vector(1))` retorna:
- A) `NaN`  B) `+Inf`  C) `1`  D) `π/4`

<details><summary>Resposta</summary>

**B.** Na borda do domínio o limite é +Inf.
</details>

**2.** Uma métrica de uso de CPU em **percentual** (0..100) vale 45. `atanh(cpu_percent)` retorna:
- A) `0.48`  B) `NaN`  C) `+Inf`  D) erro

<details><summary>Resposta</summary>

**B.** 45 está fora do domínio; divida por 100 antes (`atanh(cpu_percent / 100)` ≈ 0.48).
</details>

**3.** Qual expressão calcula a média de correlações pela transformação de Fisher?
- A) `avg(atanh(r))`
- B) `atanh(avg(r))`
- C) `tanh(avg(atanh(r)))`
- D) `atanh(avg(tanh(r)))`

<details><summary>Resposta</summary>

**C.** Vai para a escala de Fisher (`atanh`), faz a média e volta (`tanh`).
</details>

## 📝 Cola rápida

- `atanh(x)`: domínio **(-1, 1)**; ±1 → ±Inf; |x| > 1 → NaN.
- Estresse de utilização: `atanh(clamp_max(ratio, 0.9999))`.
- Fisher: `tanh(avg(atanh(r)))`.
- Inversa de `tanh`. Remove `__name__`.

## 🔗 Relacionadas

[`tanh()`](../tanh/) · [`atan()`](../atan/) · [`asinh()`](../asinh/) · [`acosh()`](../acosh/) · [`clamp_max()`](../clamp_max/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
