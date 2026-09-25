# `delta()`: quanto um gauge mudou na janela

> **Em uma frase:** `delta(v[janela])` é a diferença entre o **último** e o **primeiro** valor da janela (extrapolada para a janela inteira). É o "quanto subiu ou desceu" de um **gauge**, e pode ser negativo.

| | |
|---|---|
| **Assinatura** | `delta(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (floats e native histograms gauge) · ❌ Counter |
| **Unidade do resultado** | a **mesma do gauge** (°C, bytes, mensagens...), variação total na janela |
| **Dashboard** | http://localhost:3300/d/fn-delta |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o termômetro olhado duas vezes

Às 14:00 você olha o termômetro: **50°C**. Às 14:01 olha de novo: **56°C**. A pergunta "quanto mudou no último minuto?" tem resposta **+6°C**. Isso é o `delta`:

```
delta(temp[1m])  ≈  (último valor da janela) − (primeiro valor da janela)
                    ... extrapolado para cobrir exatamente 1m
```

- Se esfriou, o resultado é **negativo** (−6°C). Gauges sobem e descem, e o `delta` respeita isso.
- O `delta` **só olha as pontas**. O que aconteceu no meio (subiu até 80 e voltou) **não aparece**.
- É o "irmão para gauges" do [`increase()`](../increase/). A diferença: o `increase` acha que toda queda é um reset de counter; o `delta` aceita quedas como quedas.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `delta_node_hwmon_temp_celsius{host="zeus"}` | gauge | imita `node_hwmon_temp_celsius`: onda de **5 min** entre **40°C e 70°C** |
| `delta_node_hwmon_temp_celsius{host="hera"}` | gauge | estável em **~45°C** (±0.3) |
| `delta_container_memory_working_set_bytes{pod="api"}` | gauge | imita o cAdvisor: vaza **1 MiB/s**, com **picos de +150 MiB** em ~10% dos scrapes; volta a 200 MiB a cada 10 min (OOM) |
| `delta_http_requests_total` | **counter** | +4/s, reseta a cada 3 min (para a pegadinha) |

```bash
curl -s localhost:8088/metrics | grep '^delta_'
# delta_container_memory_working_set_bytes{pod="api"} 3.36217977e+08
# delta_http_requests_total 249
# delta_node_hwmon_temp_celsius{host="hera"} 45.3
# delta_node_hwmon_temp_celsius{host="zeus"} 63
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-delta
```

Espere **~5 minutos** para ver um ciclo completo da temperatura e a janela `[5m]` cheia.

---

## 🔍 Queries passo a passo

### 1. A temperatura crua

```promql
delta_node_hwmon_temp_celsius
```

**Resultado esperado:** `zeus` é uma onda suave de 40 a 70°C (período de 5 min); `hera` é uma linha reta em ~45°C.

---

### 2. Quanto a temperatura mudou no último minuto

```promql
delta(delta_node_hwmon_temp_celsius[1m])
```

**O que faz:** para cada host, pega o primeiro e o último ponto do último minuto, subtrai e extrapola para 60s.
**Resultado esperado** (unidade: °C por minuto de janela):

| host | valor |
|---|---|
| `zeus` | oscila entre ≈ **+18** (esquentando rápido) e ≈ **−18** (esfriando), passando por **0** nos picos e vales |
| `hera` | ≈ **0** (±0.6 de ruído) |

De onde vem o 18? Numa onda de amplitude 15 e período 300s, a maior variação em 60s é `2 × 15 × sen(π·60/300) ≈ 17.6`, mais um pouco de extrapolação.

---

### 3. A janela muda a história: `[30s]` × `[5m]`

```promql
delta(delta_node_hwmon_temp_celsius{host="zeus"}[30s])
delta(delta_node_hwmon_temp_celsius{host="zeus"}[5m])
```

**Resultado esperado:**
- `[30s]`: uma onda de ≈ **±9°C** (metade da janela, metade da variação).
- `[5m]`: praticamente **0** (±2°C)! A janela tem exatamente **um ciclo** da onda: começo e fim estão no mesmo ponto, então "não mudou nada", apesar da temperatura ter variado **30°C** no meio.

**Moral:** `delta` responde "onde eu estava vs onde estou", não "quanto variou no caminho". Para o caminho, use [`max_over_time()`](../max_over_time/) − [`min_over_time()`](../min_over_time/).

---

### 4. Gauge com picos: `delta` (2 pontos) × `deriv` (todos os pontos)

```promql
delta(delta_container_memory_working_set_bytes[2m])
deriv(delta_container_memory_working_set_bytes[2m]) * 120
```

**O que faz:** as duas estimam "quanto a memória cresceu em 2 min". O vazamento real é 1 MiB/s = **120 MiB em 2 min**.
**Resultado esperado:**
- `delta[2m]`: pula entre ≈ **−30 MiB**, **120 MiB** e **270 MiB**. Sempre que a primeira ou a última amostra cai num **pico de +150 MiB** (alocação temporária), o resultado erra 150 MiB para um lado ou outro.
- `deriv[2m] × 120`: fica perto de **120 MiB** (≈ ±40).
- (Eixo do painel cortado em −64 a 320 MiB: no OOM, a cada 10 min, as duas despencam por ~2 min, porque a janela pega a queda de 700 → 200 MiB.) A regressão linear usa as 24 amostras e um pico isolado quase não muda a inclinação.

Com gauges ruidosos, prefira [`deriv()`](../deriv/).

---

### 5. Pegadinha: `delta` num counter

```promql
delta(delta_http_requests_total[1m])      # errado
increase(delta_http_requests_total[1m])   # certo
```

**Resultado esperado:**
- `increase[1m]`: ≈ **240** o tempo todo (4/s × 60s).
- `delta[1m]`: ≈ **240** na maior parte do tempo, mas despenca para ≈ **−520 a −620** (−480 "puro", mais a extrapolação) durante o minuto seguinte a cada reset do counter, porque ele faz "último − primeiro" sem saber que o counter zerou.

---

### 6. Variação agora (bargauge)

```promql
delta(delta_node_hwmon_temp_celsius[1m])
```

**Resultado esperado:** `hera` ≈ 0; `zeus` entre −18 e +18, dependendo da fase da onda. Recarregue algumas vezes.

---

## 🏭 Casos reais

### 1. Superaquecimento: "a CPU esquentou mais de 10°C em 5 minutos" (imitado pelos painéis 1-2)

Uma falha de ventilador não deixa a temperatura alta de cara; ela **sobe rápido**. Um alerta de variação pega o problema antes do limite absoluto:

```yaml
groups:
- name: hardware
  rules:
  - alert: TemperaturaSubindoRapido
    expr: delta(node_hwmon_temp_celsius{sensor="temp1"}[5m]) > 10
    for: 2m
    labels: {severity: warning}
    annotations:
      summary: "{{ $labels.instance }} esquentou {{ $value | humanize }}°C em 5 min"
```

### 2. Crescimento de memória por deploy (imitado pelo painel 4)

"Quanto a memória do pod cresceu na última hora?":

```promql
delta(container_memory_working_set_bytes{namespace="shop", container="api"}[1h])
```

Com picos de GC/alocação, o `delta` erra conforme o scrape cai num pico. Para alertas, o time troca por `deriv(...[1h]) * 3600` ou compara médias: `avg_over_time(x[10m]) - avg_over_time(x[10m] offset 1h)`.

### 3. Fila acumulando

```yaml
- alert: FilaAcumulando
  expr: delta(rabbitmq_queue_messages{queue="emails"}[10m]) > 1000   # a fila cresceu mais de 1000 em 10 min
  for: 10m
  labels: {severity: warning}
  annotations:
    summary: "Fila {{ $labels.queue }} cresceu {{ $value }} mensagens em 10 min: consumidores parados?"
```

**Decisão:** `delta` (e não `increase`) porque a fila é um gauge: ela **desce** quando os consumidores trabalham, e isso não é reset.

### 4. Native histograms do tipo gauge

`delta` também funciona com **native histograms gauge** (ex.: distribuição de algo "no momento", não acumulada): calcula a diferença bucket a bucket, count e sum entre o primeiro e o último histograma da janela. Se a janela misturar floats e histogramas, a série é omitida com um aviso.

---

## ✅ Quando usar

- **Variação de gauges** numa janela: temperatura, memória, tamanho de fila, espaço em disco, número de conexões.
- **Alertas de "subiu X em Y minutos"** quando o gauge é bem-comportado (pouco ruído).
- **Comparar começo × fim** de um período em relatórios ("o disco perdeu 20 GiB hoje": `delta(node_filesystem_avail_bytes[1d])`).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **counter** | [`increase()`](../increase/) |
| Gauge **ruidoso** e você quer a tendência | [`deriv()`](../deriv/) |
| Quer a **previsão** ("vai encher em 4h?") | [`predict_linear()`](../predict_linear/) |
| Quer só a mudança entre os **2 últimos scrapes** | [`idelta()`](../idelta/) |
| Quer a **amplitude** (quanto variou no caminho) | `max_over_time() - min_over_time()` |

## ⚠️ Pegadinhas

1. **Só olha as pontas:** um gauge que sobe e volta dentro da janela dá `delta ≈ 0` (query 3).
2. **Ruído nas pontas** vira ruído no resultado (query 4).
3. **Extrapolação:** como `increase`, o resultado é estendido até as bordas da janela, então valores inteiros viram fracionados.
4. **Em counter**, resets viram quedas enormes (query 5).
5. **Mistura float + histograma** na janela: a série some do resultado (warning).

## 🎓 Na prova PCA

O que costuma cair:
- `delta` recebe **range vector**, devolve **instant vector**, e é **só para gauges**.
- Pode ser **negativo**; **não** compensa resets (quem compensa são `rate`/`increase`/`irate`).
- **Extrapola** para a janela inteira (resultado fracionado com valores inteiros é normal).
- Pares clássicos: `delta` (gauge) ↔ `increase` (counter); `idelta` (gauge) ↔ `irate` (counter); `deriv` (gauge) ↔ `rate` (counter).

**1.** Qual função é adequada para "quanto a temperatura mudou nas últimas 2 horas"?
- A) `increase(cpu_temp_celsius[2h])`
- B) `delta(cpu_temp_celsius[2h])`
- C) `rate(cpu_temp_celsius[2h])`
- D) `resets(cpu_temp_celsius[2h])`

<details><summary>Resposta</summary>

**B.** Temperatura é gauge. É inclusive o exemplo da documentação: `delta(cpu_temp_celsius{host="zeus"}[2h])`. A e C tratariam as quedas como resets.
</details>

**2.** Um gauge vale `10` no início da janela, sobe até `90` e volta para `10` no fim. Qual o `delta` aproximado?
- A) 80
- B) 160
- C) 0
- D) −80

<details><summary>Resposta</summary>

**C.** `delta` compara só o primeiro e o último valor (com extrapolação). O caminho no meio não conta.
</details>

**3.** Por que `delta(http_requests_total[5m])` é uma má ideia?
- A) Porque `delta` só aceita instant vectors
- B) Porque `delta` não compensa resets de counter e pode dar valores negativos sem sentido
- C) Porque `delta` sempre retorna por segundo
- D) Porque `delta` não extrapola

<details><summary>Resposta</summary>

**B.** Para counters use `increase`, que detecta e compensa resets.
</details>

**4.** `delta(x[10m])` pode retornar `3.27` mesmo quando `x` só assume valores inteiros. Por quê?
- A) Arredondamento do TSDB
- B) Extrapolação para cobrir a janela inteira
- C) `delta` calcula a média da janela
- D) É um bug conhecido

<details><summary>Resposta</summary>

**B.** A documentação diz explicitamente que o delta é extrapolado e pode ser não inteiro.
</details>

## 📝 Cola rápida

- `delta(gauge[janela])` ≈ último − primeiro, **extrapolado**; pode ser negativo.
- Só **gauges**. Counter → `increase`.
- Só olha as **pontas**: ruído nas pontas e "ida e volta" enganam.
- Gauge ruidoso → `deriv`; previsão → `predict_linear`; últimos 2 scrapes → `idelta`.

## 🔗 Relacionadas

[`idelta()`](../idelta/) · [`deriv()`](../deriv/) · [`increase()`](../increase/) · [`predict_linear()`](../predict_linear/) · [`max_over_time()`](../max_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#delta
