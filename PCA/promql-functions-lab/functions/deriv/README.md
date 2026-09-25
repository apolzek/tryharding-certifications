# `deriv()`: a inclinação por segundo de um gauge

> **Em uma frase:** `deriv(v[janela])` passa uma **reta** (regressão linear simples) por **todas** as amostras da janela e devolve a **inclinação** dela, em unidades **por segundo**. É o "está subindo ou descendo, e a que velocidade?" de um **gauge**, resistente a ruído.

| | |
|---|---|
| **Assinatura** | `deriv(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (só floats) · ❌ Counter · ❌ native histograms (ignorados) |
| **Unidade do resultado** | "unidade do gauge **por segundo**" (bytes/s, °C/s...) |
| **Dashboard** | http://localhost:3300/d/fn-deriv |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a régua sobre os pontos

Imprima o gráfico da janela e jogue uma **régua** sobre os pontos, tentando ficar o mais perto possível de **todos** eles (mínimos quadrados). A **inclinação da régua** é o `deriv`.

```
 valor
   │            •    •
   │       •  •   ╱•
   │    •  ╱ •  ╱          ← a régua: inclinação = deriv
   │  • ╱ •
   │ ╱•
   └──────────────────── tempo
```

- Um ponto maluco (pico) mexe **pouco** na régua: os outros pontos "seguram".
- Compare com [`delta()`](../delta/), que liga **só o primeiro e o último ponto**: se um deles for o maluco, o resultado vai junto.
- É o "irmão para gauges" do [`rate()`](../rate/): ambos dão algo **por segundo**, mas o `rate` entende resets de counter e o `deriv` não.

A regressão precisa de **pelo menos 2 amostras** (float). Se aparecer `+Inf`/`-Inf` na janela, o resultado é `NaN`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `deriv_container_memory_working_set_bytes{pod="leaky"}` | gauge | imita o cAdvisor: **vazamento de 1 MiB/s**, de 200 a 800 MiB, **OOM kill a cada 10 min** |
| `deriv_container_memory_working_set_bytes{pod="healthy"}` | gauge | estável em ~**300 MiB** |
| (os dois pods) | | ruído: ±3 MiB sempre e **picos de +150 MiB** em ~10% dos scrapes (alocações temporárias) |
| `deriv_node_hwmon_temp_celsius{host="rack-01"}` | gauge | imita `node_hwmon_temp_celsius`: onda de **5 min** entre **18 e 28°C** |

```bash
curl -s localhost:8088/metrics | grep '^deriv_'
# deriv_container_memory_working_set_bytes{pod="healthy"} 3.02966983e+08
# deriv_container_memory_working_set_bytes{pod="leaky"} 3.43690438e+08
# deriv_node_hwmon_temp_celsius{host="rack-01"} 25.77
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-deriv
```

Espere **~10 minutos** para ver um ciclo completo (vazamento → OOM) e duas ondas de temperatura.

---

## 🔍 Queries passo a passo

### 1 e 2. Temperatura: derivada positiva, negativa e zero

```promql
deriv_node_hwmon_temp_celsius
deriv(deriv_node_hwmon_temp_celsius[1m]) * 60     # °C por minuto
```

**Resultado esperado:**
- **Cru:** onda suave entre 18 e 28°C a cada 5 min.
- **`deriv × 60`:** outra onda, "adiantada" um quarto de ciclo: **positiva** enquanto esquenta, **negativa** enquanto esfria e **zero** exatamente nos picos e vales. O máximo teórico é `2π × 5 / 300 × 60 ≈ 6.3 °C/min`; com a janela de 1m a régua suaviza um pouco e o pico fica em ≈ **±5.9 °C/min**.

---

### 3. Memória crua

```promql
deriv_container_memory_working_set_bytes
```

**Resultado esperado:** `leaky` é um dente-de-serra que sobe de ~200 para ~800 MiB em 10 min e despenca (OOM); `healthy` fica em ~300 MiB. Os dois têm "espinhos" de +150 MiB aqui e ali.

---

### 4. A velocidade do vazamento

```promql
deriv(deriv_container_memory_working_set_bytes[2m])
```

**O que faz:** regressão linear nas ~24 amostras dos últimos 2 min de cada pod.
**Resultado esperado** (unidade: bytes/s):

| pod | valor |
|---|---|
| `leaky` | ≈ **1.05 MB/s** (= 1 MiB/s = 1.048.576 B/s) enquanto vaza; logo após o OOM fica **negativo** por ~2 min (a régua pega a queda) |
| `healthy` | ≈ **0** (± algumas centenas de KB/s por causa dos picos) |

---

### 5. Ruído: `deriv` × `delta`/segundos

```promql
deriv(deriv_container_memory_working_set_bytes{pod="leaky"}[2m])
delta(deriv_container_memory_working_set_bytes{pod="leaky"}[2m]) / 120
```

**O que faz:** as duas tentam estimar a mesma coisa (bytes/s).
**Resultado esperado:** o `deriv` fica perto de **1 MiB/s**; o `delta/120` salta ±**1.25 MiB/s** (150 MiB / 120s) sempre que a primeira ou a última amostra da janela cai num pico. A régua é muito mais estável do que "ligar as pontas".

---

### 6. Vazamento em MiB/min (agora)

```promql
deriv(deriv_container_memory_working_set_bytes[2m]) * 60 / 1024 / 1024
```

**Resultado esperado:** `leaky` ≈ **60 MiB/min** (entre ~40 e ~80), `healthy` ≈ **0** (±20 MiB/min: um pico de 150 MiB perto da borda da janela ainda inclina um pouco a régua). (Se você abrir logo depois de um OOM, o `leaky` aparece negativo.)

---

### 7. O que dá errado: `rate()` num gauge

```promql
rate(deriv_container_memory_working_set_bytes[2m])    # errado
deriv(deriv_container_memory_working_set_bytes[2m])   # certo
```

**O que acontece:** o `rate` acha que a métrica é um counter. Toda **queda** (o fim de cada pico de +150 MiB, o OOM) vira um "reset", e ele soma o valor **inteiro** depois da queda (~300 MiB!) como se o counter tivesse recomeçado do zero.
**Resultado esperado:** `rate` dá **vários MB/s** para os dois pods, inclusive o `healthy`, que não cresce nada. O `deriv` dá ≈ **1 MiB/s** (`leaky`) e ≈ **0** (`healthy`). Moral: **gauge → `deriv`; counter → `rate`**.

---

## 🏭 Casos reais

### 1. Memory leak antes do OOMKill (imitado pelos painéis 3-6)

O pod do checkout é reiniciado por OOM a cada ~6 horas. Em vez de esperar o OOM, o time alerta quando a memória **cresce sem parar**:

```yaml
groups:
- name: memory
  rules:
  - alert: PossivelMemoryLeak
    expr: deriv(container_memory_working_set_bytes{container!="", container!="POD"}[1h]) > 100 * 1024   # > 100 KiB/s por 1h
    for: 30m
    labels: {severity: warning}
    annotations:
      summary: "{{ $labels.namespace }}/{{ $labels.pod }} cresce {{ $value | humanize1024 }}B/s"
```

**Decisão:** `deriv` (e não `delta`) porque o working set tem picos de GC; a regressão ignora espinhos isolados. `rate` estaria errado: memória é gauge.

### 2. Temperatura subindo (imitado pelos painéis 1-2)

```yaml
- alert: TemperaturaSubindo
  expr: deriv(node_hwmon_temp_celsius[10m]) * 60 > 1    # esquentando mais de 1°C por minuto
  for: 10m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.instance }}/{{ $labels.chip }}: +{{ $value | humanize }}°C/min"
```

**Decisão:** `deriv` com janela de 10m em vez de `delta`: sensores de temperatura oscilam ±1°C a cada leitura, e a regressão ignora isso.

### 3. "Disco caindo X GB por hora" para capacity planning

```promql
-deriv(node_filesystem_avail_bytes{mountpoint="/"}[6h]) * 3600   # bytes consumidos por hora (tendência de 6h)
```

É a mesma regressão que o [`predict_linear()`](../predict_linear/) usa por baixo; lá ela é projetada para o futuro.

### 4. Taxa de variação de uma recording rule gauge

Quando você só tem um gauge agregado (ex.: `job:queue_depth:sum`), `deriv(job:queue_depth:sum[15m])` diz se a fila está acumulando (>0) ou drenando (<0).

---

## ✅ Quando usar

- **Tendência de gauges ruidosos**: memória, fila, temperatura, espaço em disco.
- **Alertas de "está crescendo"** que precisam ser estáveis (a regressão não pisca).
- **Capacity planning:** consumo médio por hora/dia.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **counter** | [`rate()`](../rate/) (compensa resets) |
| Quer a **variação total** (não por segundo) de um gauge limpo | [`delta()`](../delta/) |
| Quer **prever** um valor futuro | [`predict_linear()`](../predict_linear/) |
| Quer a mudança entre os **2 últimos scrapes** | [`idelta()`](../idelta/) |
| Série é **native histogram** | não suportado (ignorado) |

## ⚠️ Pegadinhas

1. **Mudanças bruscas** (OOM, reinício, limpeza de disco) dentro da janela entortam a régua por um tempo igual à janela (painel 2 logo após o OOM).
2. **Janela curta** com ruído = inclinação instável. Janela longa = estável, mas reage devagar.
3. **Em counter**, cada reset vira uma inclinação muito negativa. Use `rate`.
4. **`+Inf`/`-Inf`** na janela → resultado `NaN`.
5. **Unidade por segundo:** multiplique por 60/3600/86400 para ler em min/h/dia.

## 🎓 Na prova PCA

O que costuma cair:
- `deriv` usa **regressão linear simples** sobre **todas** as amostras da janela (≠ `delta`, que usa as pontas).
- Resultado **por segundo**; só **gauges**; precisa de **≥ 2 amostras float**.
- Par clássico: `deriv` (gauge) ↔ `rate` (counter). E `predict_linear` usa a mesma regressão para projetar o futuro.

**1.** Qual função calcula a derivada por segundo de um gauge usando regressão linear?
- A) `rate()`
- B) `delta()`
- C) `deriv()`
- D) `idelta()`

<details><summary>Resposta</summary>

**C.** `rate` é para counters; `delta`/`idelta` não usam regressão e não são por segundo.
</details>

**2.** Para detectar um vazamento de memória em `process_resident_memory_bytes`, qual expressão é a mais adequada?
- A) `rate(process_resident_memory_bytes[1h]) > 0`
- B) `deriv(process_resident_memory_bytes[1h]) > 0`
- C) `increase(process_resident_memory_bytes[1h]) > 0`
- D) `irate(process_resident_memory_bytes[1h]) > 0`

<details><summary>Resposta</summary>

**B.** Memória residente é gauge. A, C e D são funções de counter e tratariam cada queda (GC) como reset.
</details>

**3.** Qual o requisito mínimo para `deriv()` produzir resultado para uma série?
- A) Pelo menos 1 amostra na janela
- B) Pelo menos 2 amostras float na janela
- C) Pelo menos 4 amostras na janela
- D) A série precisa ser um counter

<details><summary>Resposta</summary>

**B.** Com uma amostra só não há reta. A documentação diz "at least two float samples".
</details>

**4.** Por que `deriv(x[10m])` costuma ser mais estável que `delta(x[10m]) / 600` em um gauge ruidoso?
- A) Porque `deriv` descarta os valores extremos
- B) Porque `deriv` usa todas as amostras (mínimos quadrados), enquanto `delta` depende só do primeiro e do último ponto
- C) Porque `deriv` usa uma janela maior internamente
- D) Porque `delta` não extrapola

<details><summary>Resposta</summary>

**B.** A regressão dilui o efeito de um ponto isolado; o `delta` é sensível a ruído nas pontas.
</details>

## 📝 Cola rápida

- `deriv(gauge[janela])` = inclinação da **reta de regressão** (por **segundo**).
- Usa **todas** as amostras → aguenta ruído melhor que `delta`.
- Só **gauges** (counter → `rate`), só floats, ≥ 2 amostras.
- Memory leak: `deriv(container_memory_working_set_bytes[1h]) > limite`.
- `predict_linear` = mesma reta, projetada `t` segundos à frente.

## 🔗 Relacionadas

[`predict_linear()`](../predict_linear/) · [`delta()`](../delta/) · [`idelta()`](../idelta/) · [`rate()`](../rate/) · [`double_exponential_smoothing()`](../double_exponential_smoothing/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#deriv
