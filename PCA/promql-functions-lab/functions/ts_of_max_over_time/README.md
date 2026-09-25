# `ts_of_max_over_time()`: QUANDO aconteceu o pico (experimental)

> **Em uma frase:** `ts_of_max_over_time(v[janela])` devolve o **timestamp Unix (em segundos)** da **última** amostra que tem o **valor máximo** da janela, para cada série. É o "quando" que falta no `max_over_time()`.

| | |
|---|---|
| **Assinatura** | `ts_of_max_over_time(v range-vector) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Gauge (ou recording rule de `rate`) · ⚠️ Counter cru (o máximo é quase sempre a última amostra) · ❌ Histogram (ignorado) |
| **Unidade do resultado** | **segundos desde 1970** (timestamp Unix, com fração) |
| **Dashboard** | http://localhost:3300/d/fn-ts_of_max_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o placar e a data do recorde

`max_over_time` é o **placar**: "o recorde de velocidade da pista é 312 km/h".
`ts_of_max_over_time` é a **data do recorde**: "foi batido em 14/03 às 15:42".

Se dois pilotos cravaram exatamente 312 km/h, a função fica com o **mais recente** (a documentação: *"the timestamp of the last float sample that has the maximum value"*).

```
CPU %
 95 ┤    ●                     ← agulha: 1 amostra → ts = o instante dela
 80 ┤              ●●●●●●●●●●  ← platô: empate → ts = o ÚLTIMO ● do platô
 30 ┤~~~~ ~~~~~~~~~          ~~~~
```

Truque: **`time() - ts_of_max_over_time(x[5m])`** = "o pico foi **há N segundos**".

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita o uso de CPU em % por host (como a recording rule `instance:node_cpu_utilisation:rate5m` do node-mixin, × 100):

| Métrica | Tipo | Comportamento |
|---|---|---|
| `ts_of_max_over_time_node_cpu_utilisation_percent{host="web-1"}` | gauge | ~**30%** (±5) e uma **agulha de exatamente 95%** por 5s (1 amostra) a cada **4 min** |
| `ts_of_max_over_time_node_cpu_utilisation_percent{host="web-2"}` | gauge | ~**40%** (±5) e um **platô de exatamente 80%** por **60s** a cada **4 min** (empate no máximo) |
| `ts_of_max_over_time_http_requests_total{host="web-1"}` | **counter** | **10 req/s**, com **rajada de 50 req/s** por 20s a cada 4 min (junto com a agulha de CPU). Para mostrar o erro de usar a função direto num counter |

```bash
curl -s localhost:8088/metrics | grep '^ts_of_max_over_time_'
# ts_of_max_over_time_node_cpu_utilisation_percent{host="web-1"} 31.7
# ts_of_max_over_time_node_cpu_utilisation_percent{host="web-2"} 80
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-ts_of_max_over_time
```

Espere **~5 min** para a janela `[5m]` conter um ciclo completo.

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
ts_of_max_over_time_node_cpu_utilisation_percent
```

**Resultado esperado:** `web-1` com uma **agulha fina** de 95% a cada 4 min; `web-2` com um **platô largo** de 80% por 1 min a cada 4 min.

---

### 2. QUAL foi o pico

```promql
max_over_time(ts_of_max_over_time_node_cpu_utilisation_percent[5m])
```

**Resultado esperado:** `web-1` = **95**, `web-2` = **80**, constantes (a janela de 5 min sempre contém um pico, porque o ciclo é de 4 min).

---

### 3. O pico foi há quantos segundos?

```promql
time() - ts_of_max_over_time(ts_of_max_over_time_node_cpu_utilisation_percent[5m])
```

**Resultado esperado** (unidade: s):
- `web-1`: **dente-de-serra** de 0 a ~**240s**: zera a cada agulha. (Quando a janela contém **duas** agulhas de 95, vale a mais recente.)
- `web-2`: fica em ≈ **0 a 5s** durante **todo** o platô (cada nova amostra de 80 passa a ser "o último máximo") e só depois começa a subir, de 0 a ~**180s**, até o próximo platô.

**Moral:** no platô, a função diz "o pico é **agora**" enquanto ele durar. Se você queria "quando o pico **começou**", essa função não responde isso.

---

### 4. "O pico foi há menos de 30s?"

```promql
(time() - ts_of_max_over_time(ts_of_max_over_time_node_cpu_utilisation_percent[5m])) < 30
```

**Resultado esperado:** pontos que aparecem logo depois de cada agulha do `web-1` (por ~30s) e durante o platô do `web-2` (+30s). Na maior parte do tempo, **vazio**.

---

### 5. Tabela: pico, horário e idade

```promql
max_over_time(ts_of_max_over_time_node_cpu_utilisation_percent[5m])
ts_of_max_over_time(ts_of_max_over_time_node_cpu_utilisation_percent[5m]) * 1000
time() - ts_of_max_over_time(ts_of_max_over_time_node_cpu_utilisation_percent[5m])
```

**Resultado esperado:**

| host | pico em 5m | quando foi o pico | há quanto tempo |
|---|---|---|---|
| web-1 | 95% | `2026-09-25 19:24:00` | ex.: 1.7 min |
| web-2 | 80% | `2026-09-25 19:22:55` | ex.: 2.8 min |

---

### 6. 🟡 O que dá errado: direto num counter

```promql
time() - ts_of_max_over_time(ts_of_max_over_time_http_requests_total[5m])
```

**Resultado esperado:** linha **grudada em 0 a 5s**, sempre. Um counter só cresce, então a amostra máxima é **sempre a última**. A função "funciona", mas a resposta é inútil.

---

### 7. ✅ O jeito certo: `rate` + subquery

```promql
time() - ts_of_max_over_time(rate(ts_of_max_over_time_http_requests_total[1m])[5m:])
```

**O que faz:** `rate(...[1m])` vira a velocidade (req/s); a subquery `[5m:]` avalia essa velocidade a cada 5s (intervalo global de avaliação) nos últimos 5 min, gerando um range vector; aí sim `ts_of_max_over_time` acha **quando a velocidade foi máxima**.
**Resultado esperado:** dente-de-serra de 0 a ~**240s**. Ele zera ≈ **20 a 60s depois** do início de cada rajada: com `rate[1m]`, a velocidade máxima (≈ 10 + 40×20/60 ≈ 23 req/s) é atingida quando a janela de 1 min contém a rajada inteira, e fica nesse patamar por ~40s. Como o patamar é um empate, vale o **último** ponto dele.

---

## 🏭 Casos reais

### 1. Postmortem: "a que horas o pico de memória aconteceu?"

O pod foi `OOMKilled` durante a noite. Antes de abrir logs, descubra o minuto exato do pico:

```promql
max_over_time(container_memory_working_set_bytes{pod=~"checkout-.*", container="app"}[12h])
ts_of_max_over_time(container_memory_working_set_bytes{pod=~"checkout-.*", container="app"}[12h]) * 1000
```

Com o horário, você correlaciona com o cron job, o deploy ou o pico de tráfego daquele minuto.

### 2. Anotação no alerta: "o pico foi há X minutos"

Um alerta de CPU que diz **quando** foi o pior momento economiza tempo do plantonista:

```yaml
- alert: HostCPUSpike
  expr: max_over_time(instance:node_cpu_utilisation:rate5m[15m]) > 0.95
  annotations:
    summary: "CPU de {{ $labels.instance }} bateu {{ $value | humanizePercentage }}"
    description: >-
      Pico há {{ with printf "time() - ts_of_max_over_time(instance:node_cpu_utilisation:rate5m{instance='%s'}[15m])" $labels.instance | query }}{{ . | first | value | humanizeDuration }}{{ end }}.
```

### 3. Horário de pico de tráfego do dia (capacity planning)

```yaml
- record: job:http_requests:rate5m:ts_of_max_1d
  expr: ts_of_max_over_time(job:http_requests:rate5m{job="api"}[1d])
```

Registrado diariamente, mostra se o pico está "andando" (ex.: campanhas de marketing mudando o horário de maior carga) e ajuda a agendar o autoscaling preditivo.

---

## ✅ Quando usar

- **Investigação/postmortem**: "quando foi o pico?" (memória, CPU, latência, fila, erros).
- **Alertas e anotações** com contexto temporal ("pico há 3 min").
- **Detectar picos recentes**: `time() - ts_of_max_over_time(...) < N`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só precisa do valor do pico | [`max_over_time()`](../max_over_time/) |
| Quer quando foi o **vale** | [`ts_of_min_over_time()`](../ts_of_min_over_time/) |
| Quer quando a série foi vista por último | [`ts_of_last_over_time()`](../ts_of_last_over_time/) |
| Quer quando o platô **começou** | não há função direta; olhe o gráfico de `max_over_time` ou use `changes()`/`resets()` conforme o caso |
| Ambiente sem a flag experimental | não há equivalente direto |

## ⚠️ Pegadinhas

1. **Experimental**: sem a feature flag, erro de função desconhecida.
2. **Empate = ÚLTIMA amostra**: num platô, a resposta acompanha o fim do platô (query 3, `web-2`).
3. **Segundos Unix**: `* 1000` para o Grafana formatar como data.
4. **Timestamp do scrape**, não do evento real: com scrape de 30s, o pico "real" pode ter sido até 30s antes.
5. **Só dentro da janela**: um pico maior 1 segundo antes do início da janela é ignorado.
6. **Counter cru**: o máximo de um counter é a última amostra (salvo reset). Aplique `rate` antes.

---

## 🎓 Na prova PCA

Funções `ts_of_*` são **experimentais**; o que a prova cobra de verdade são os conceitos em volta: `max_over_time`, `time()`, `timestamp()`, range vs instant vector.

**1.** Você quer saber **quando**, na última hora, o uso de memória de cada pod foi máximo. Qual query?

- A) `timestamp(max_over_time(container_memory_working_set_bytes[1h]))`
- B) `ts_of_max_over_time(container_memory_working_set_bytes[1h])`
- C) `max_over_time(timestamp(container_memory_working_set_bytes)[1h])`
- D) `time() - max_over_time(container_memory_working_set_bytes[1h])`

<details><summary>Resposta</summary>

**B.** A) retorna o instante de avaliação. C) é inválido sem subquery e, com subquery, daria o timestamp mais recente, não o do pico. D) mistura bytes com segundos.
</details>

**2.** Na janela, a CPU bateu 100% em t=50 e novamente 100% em t=90. `ts_of_max_over_time` retorna:

- A) 50
- B) 90
- C) 70
- D) as duas amostras

<details><summary>Resposta</summary>

**B.** Empate → última amostra com o valor máximo. E a função retorna **uma** amostra por série.
</details>

**3.** Por que `ts_of_max_over_time(http_requests_total[1h])` quase sempre retorna um valor próximo de "agora"?

- A) Porque a função ignora counters.
- B) Porque um counter só cresce, então o máximo é a amostra mais recente (salvo resets).
- C) Porque o resultado é sempre `time()`.
- D) Porque counters não têm timestamp.

<details><summary>Resposta</summary>

**B.** Counters são monotônicos: o maior valor é o último. Para "quando foi o pico de **taxa**", aplique `rate` antes (subquery ou recording rule).
</details>

**4.** O que é necessário para usar `ts_of_max_over_time`?

- A) Prometheus ≥ 2.0 sem flags.
- B) `--enable-feature=promql-experimental-functions`.
- C) Um histogram nativo.
- D) Um recording rule.

<details><summary>Resposta</summary>

**B.** É uma função experimental.
</details>

---

## 📝 Cola rápida

- `ts_of_max_over_time(x[w])` = **quando** (timestamp Unix em s) foi o máximo da janela; **experimental**.
- Empate/platô → **última** amostra com o valor máximo.
- `time() - ts_of_max_over_time(x[w])` = "o pico foi há N segundos".
- Grafana: `* 1000` + `dateTimeAsIso`.
- Counter? `rate` primeiro. Par natural: `max_over_time` (o quê) + `ts_of_max_over_time` (quando).

## 🔗 Relacionadas

[`max_over_time()`](../max_over_time/) · [`ts_of_min_over_time()`](../ts_of_min_over_time/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/) · [`ts_of_first_over_time()`](../ts_of_first_over_time/) · [`timestamp()`](../timestamp/) · [`time()`](../time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
