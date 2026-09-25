# `asinh()`: seno hiperbólico inverso (o "log simétrico")

> **Em uma frase:** `asinh(v)` comprime valores como um logaritmo, mas **aceita zero e negativos**: ≈ `x` perto de zero, ≈ `ln(2x)` para valores grandes, e `asinh(-x) = -asinh(x)`.

| | |
|---|---|
| **Assinatura** | `asinh(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge com **sinal** e **faixa enorme** (saldo, crescimento líquido, offset, deriv) · ⚠️ histogramas são **ignorados** |
| **Unidade do resultado** | escala "logarítmica" adimensional |
| **Dashboard** | http://localhost:3300/d/fn-asinh |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a lupa com zoom automático

Imagine uma lente que dá **zoom** nos valores pequenos e **afasta** nos gigantes, **dos dois lados do zero**:

| x | ln(x) | asinh(x) |
|---|---|---|
| -50 000 | **NaN** | -11.5 |
| -5 | **NaN** | -2.31 |
| 0 | **-Inf** | 0 |
| 0.3 | -1.20 | 0.30 |
| 5 | 1.61 | 2.31 |
| 2 000 | 7.60 | 8.29 |
| 50 000 | 10.8 | 11.5 |

O `ln()` (e o eixo log do Grafana) só funcionam para **x > 0**. Métricas que cruzam o zero (crescimento líquido de fila, offset de relógio, `deriv()`, saldo) precisam do `asinh`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `asinh_queue_net_growth_per_second{queue="emails"}` | gauge | entre **-5 e +5** (período 3 min) |
| `asinh_queue_net_growth_per_second{queue="eventos"}` | gauge | entre **-50 000 e +50 000** (período 4 min) |
| `asinh_queue_net_growth_per_second{queue="relatorios"}` | gauge | **0**, com burst de **+2 000** por 30s a cada 5 min |
| `asinh_node_timex_offset_seconds{host="db-1"}` | gauge | imita `node_timex_offset_seconds`: NTP saudável, **±0.0003 s** |
| `asinh_node_timex_offset_seconds{host="vm-7"}` | gauge | relógio derivando: **-0.2 s → +2 s** a cada 5 min |

> 💡 O cenário usa o label `host`, e não `instance`: um label `instance` exposto pelo alvo colide com o `instance` que o Prometheus adiciona no scrape e é renomeado para `exported_instance` (a menos que `honor_labels: true`). Tema clássico da PCA!

**Casos (do básico ao real):** 🟢 **básico:** filas de escalas diferentes · 🟡 **nuance:** `ln` quebra; `sinh` desfaz · 🔴 **real:** offset de relógio (`node_timex_offset_seconds`)

```bash
curl -s localhost:8088/metrics | grep '^asinh_'
```

## ▶️ Como rodar

```bash
docker compose up -d --build
# Grafana: http://localhost:3300/d/fn-asinh
```

---

## 🔍 Queries passo a passo

### 1. Escala linear: a fila pequena some

```promql
asinh_queue_net_growth_per_second
```

**Resultado esperado:** só "eventos" (±50 000) aparece; "emails" (±5) e "relatorios" ficam **grudados no zero**.

---

### 2. `asinh`: tudo visível

```promql
asinh(asinh_queue_net_growth_per_second)
```

**Resultado esperado:** eventos oscila entre **±11.5**, emails entre **±2.31**, relatorios salta de **0 para 8.29** no burst. As três no mesmo gráfico e todas cruzando o zero sem problema.

---

### 3. `ln` quebra

```promql
ln(asinh_queue_net_growth_per_second)
```

**Resultado esperado:** só aparecem as metades **positivas** das ondas; nos negativos é **NaN** (buraco) e "relatorios" em 0 é **-Inf** (fora do gráfico).

---

### 4. Voltando à unidade original

```promql
sinh(asinh(asinh_queue_net_growth_per_second{queue="emails"}))
```

**Resultado esperado:** a onda original, **-5 a +5**. [`sinh()`](../sinh/) é a inversa.

---

### 5 e 6. Caso real: offset do relógio (`node_timex_offset_seconds`)

```promql
asinh_node_timex_offset_seconds * 1000            # ms, cru
asinh(asinh_node_timex_offset_seconds * 1000)     # ms, comprimido
```

**Resultado esperado:** no cru, o db-1 (±0.3 ms) é uma linha reta em 0 ao lado do vm-7 (até 2000 ms). Com `asinh`: db-1 oscila em **±0.30**, vm-7 vai de **-6.0** (-200 ms, atrasado) a **+8.3** (+2000 ms, adiantado). O **sinal** (atrasado/adiantado) é preservado, o que um `abs()` + escala log perderia.

---

### 7. Casos especiais

| Query | Resultado |
|---|---|
| `asinh(vector(0))` | `0` |
| `asinh(vector(1))` | `0.881373587019543` |
| `asinh(vector(-1))` | `-0.881373587019543` |
| `asinh(vector(1e6))` | `14.50865773852447` |
| `ln(vector(2e6))` | `14.508657738524219` (≈ igual!) |
| `asinh(vector(-Inf))` | `-Inf` ([Go `math.Asinh`](https://pkg.go.dev/math#Asinh)) |

---

## 🏭 Casos reais

> Das inversas hiperbólicas, `asinh` é a mais prática: é a transformação padrão para **gráficos "log" de valores com sinal**.

**1. SRE de plataforma: drift de relógio na frota (node_exporter).** `node_timex_offset_seconds` vai de microssegundos (saudável) a segundos (VM com problema), positivo ou negativo. Um painel com todas as máquinas:

```promql
asinh(node_timex_offset_seconds * 1e6)    # em µs: 1 µs → 0.88, 1 ms → 7.6, 1 s → 14.5
```

O **alerta** continua usando o valor cru (limiar claro):

```yaml
- alert: ClockSkewDetected
  expr: abs(node_timex_offset_seconds) > 0.05
  for: 10m
```

**2. Capacidade: tendência de disco com sinal.** `deriv(node_filesystem_avail_bytes[1h])` é negativo quando o disco enche e positivo quando é limpo, com magnitudes de bytes a gigabytes por segundo. `asinh(deriv(...))` coloca todos os discos num **mapa de calor** legível.

```yaml
groups:
  - name: disk-trend
    rules:
      # deriv em KiB/s: negativo = enchendo, positivo = liberando; asinh deixa todos os discos legíveis
      - record: instance:filesystem_avail_deriv:asinh
        expr: asinh(deriv(node_filesystem_avail_bytes{fstype!="tmpfs"}[1h]) / 1024)
```

## ✅ Quando usar

- Gráficos "log" de métricas que **cruzam o zero** ou chegam a zero.
- Comparar séries de ordens de grandeza muito diferentes, **mantendo o sinal**.
- Pré-processar antes de agregar valores com cauda longa.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Valores sempre > 0 | [`ln()`](../ln/) / [`log10()`](../log10/) (mais fáceis de ler) |
| Quer saturar em ±1 | [`tanh()`](../tanh/) |
| Alertas com limiar em unidade real | o valor cru |
| Valor ≥ 1 do tipo "razão" | [`acosh()`](../acosh/) |

## ⚠️ Pegadinhas

1. **A unidade vira "escala asinh":** não some/compare os valores transformados como se fossem msgs/s; volte com `sinh()`.
2. **Escolha a unidade antes:** `asinh(x_segundos)` e `asinh(x_ms)` têm formas diferentes (a "zona linear" é |x| < 1). Multiplique para colocar o "ruído normal" perto de 1.
3. **Não é `asin()`:** `asin` tem domínio [-1, 1]; `asinh` aceita todos os reais.
4. Remove `__name__`; histogramas ignorados.

## 🎓 Na prova PCA

- Instant vector → instant vector; **domínio: todos os reais** (nunca NaN para número finito).
- `asinh(0) = 0`, ímpar, `asinh(±Inf) = ±Inf`.
- Diferente de `ln`, que dá NaN para negativos e -Inf para 0.

**1.** Qual função retorna um valor finito para **todas** as entradas -5, 0 e 5?
- A) `ln()`  B) `log2()`  C) `asinh()`  D) `acosh()`

<details><summary>Resposta</summary>

**C.** `ln`/`log2` dão NaN para -5 e -Inf para 0; `acosh` dá NaN para x < 1.
</details>

**2.** `asinh(vector(-1))` retorna aproximadamente:
- A) `NaN`  B) `-0.8814`  C) `0.8814`  D) `-1.5708`

<details><summary>Resposta</summary>

**B.** `asinh` é ímpar: asinh(-1) = -asinh(1).
</details>

**3.** Qual a inversa de `asinh`?
- A) `asin`  B) `sinh`  C) `acosh`  D) `exp`

<details><summary>Resposta</summary>

**B.** `sinh(asinh(x)) = x`.
</details>

## 📝 Cola rápida

- `asinh(x) = ln(x + √(x²+1))`: ≈ x perto de 0; ≈ ln(2x) para x grande.
- Aceita **todos os reais** (0 → 0; negativos → negativos).
- "Log simétrico" para gráficos de métricas com sinal.
- Inversa: `sinh`. Remove `__name__`.

## 🔗 Relacionadas

[`sinh()`](../sinh/) · [`ln()`](../ln/) · [`log10()`](../log10/) · [`acosh()`](../acosh/) · [`tanh()`](../tanh/) · [`asin()`](../asin/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#trigonometric-functions
