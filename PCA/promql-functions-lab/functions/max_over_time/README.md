# `max_over_time()`: o pico de cada série dentro da janela

> **Em uma frase:** `max_over_time(v[janela])` devolve, **para cada série**, o **maior valor** entre as amostras da janela. É o jeito de não perder picos curtos que um gráfico "de longe" esconde.

| | |
|---|---|
| **Assinatura** | `max_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (memória, latência, fila, temperatura) · ✅ resultado de `rate()` via **subquery** · ❌ Counter cru · ❌ histogram (amostras de histograma são ignoradas) |
| **Unidade do resultado** | a **mesma** da entrada |
| **Dashboard** | http://localhost:3300/d/fn-max_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o termômetro de máxima e mínima

Sabe aquele termômetro antigo com um **ponteirinho que fica parado no maior valor** até alguém resetar? Você olha para ele **uma vez por dia** e mesmo assim sabe que "às 14h fez 38°C", mesmo que agora esteja 22°C.

- O gauge `memory_usage_bytes` é o **termômetro normal**: mostra o valor *daquele instante*.
- `max_over_time(memory_usage_bytes[5m])` é o **ponteirinho de máxima**: "o valor mais alto dos últimos 5 min".

Por que isso importa? Porque **gráficos de janela longa não mostram cada scrape**. Um dashboard de 24h com 800 px de largura tem ~1 ponto a cada 2 min; o Grafana pergunta o valor **naquele instante** e pula tudo que houve entre um ponto e outro. Um pico de memória de 15s que causou um OOM kill simplesmente **não aparece**. O `max_over_time` faz cada ponto dizer "o maior valor desde o ponto anterior".

**Não confunda os eixos** (veja a planilha em [`avg_over_time`](../avg_over_time/)):

```
 max_over_time(x[5m])   →  ao longo do TEMPO, por série   (1 resultado por pod)
 max(x)                 ↓  ENTRE séries, num instante     (1 resultado no total)
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `max_over_time_container_memory_working_set_bytes{pod="api-1"}` | gauge | ~**400 MiB** (onda suave ±30), com **pico de ~950 MiB durante 15s** (3 scrapes) **a cada 3 min**, nos segundos 20..35 do ciclo |
| `max_over_time_container_memory_working_set_bytes{pod="api-2"}` | gauge | ~**550 MiB** estável |
| `max_over_time_http_requests_total` | counter | **5 req/s**, com **rajada de 50 req/s por 90s a cada 4 min** |

O pico de memória foi posicionado de propósito para **nunca coincidir com a virada de um minuto**: assim o painel que "olha 1x por minuto" nunca o vê.

```bash
curl -s localhost:8088/metrics | grep '^max_over_time_'
# max_over_time_container_memory_working_set_bytes{pod="api-1"} 4.1943e+08
# max_over_time_http_requests_total 1.2345e+05
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-max_over_time
```

Espere **~5 min** (janela `[5m]`) e **~10 min** para a subquery de `[10m:]` ficar estável.

---

## 🔍 Queries passo a passo

### 1. Memória crua em resolução de 5s

```promql
max_over_time_container_memory_working_set_bytes
```

**Resultado esperado:** `api-1` em ~400 MiB com **agulhas** até ~950 MiB a cada 3 min; `api-2` reta em ~550 MiB. Aqui o dashboard está em "últimos 15 min", então cada ponto do gráfico é um scrape e o pico aparece.

---

### 2. O mesmo dado "de longe": o pico some

```promql
last_over_time(max_over_time_container_memory_working_set_bytes[1m:1m])
```

**O que faz:** a subquery `[1m:1m]` avalia a métrica **só nas viradas de minuto** e o `last_over_time` repete esse valor até a próxima virada. É exatamente o que um gráfico com **resolução de 1 min** (ex.: dashboard de 24h) enxergaria.
**Resultado esperado:** degraus em ~400 MiB para `api-1`, **sem nenhum pico**. Olhando só este painel, você juraria que o pod nunca passou de 450 MiB.

---

### 3. `max_over_time`: o ponteirinho de máxima

```promql
max_over_time(max_over_time_container_memory_working_set_bytes[5m])
```

**O que faz:** para cada pod, o maior valor entre as ~60 amostras dos últimos 5 min.
**Resultado esperado:**

| pod | valor |
|---|---|
| `api-1` | ≈ **950 MiB** (sempre, já que há um pico a cada 3 min e a janela é de 5 min) |
| `api-2` | ≈ **555 MiB** (o maior ruído dos 5 min) |

Mesmo que você só olhasse este painel uma vez por minuto, o pico estaria lá.

---

### 4. `max` vs `avg` na mesma janela

```promql
max_over_time(max_over_time_container_memory_working_set_bytes{pod="api-1"}[5m])   # ≈ 950 MiB
avg_over_time(max_over_time_container_memory_working_set_bytes{pod="api-1"}[5m])   # ≈ 430–455 MiB
```

**Resultado esperado:** 15s de pico em 300s = 5% das amostras. A média sobe só uns **30–50 MiB**, o máximo sobe **550 MiB**. Se o limite do container é 1 GiB, só o `max` mostra que você está a 70 MiB do OOM kill.

---

### 5. `max(x)` vs `max_over_time(x[5m])`

```promql
max(max_over_time_container_memory_working_set_bytes)                 # entre pods, agora
max_over_time(max_over_time_container_memory_working_set_bytes[5m])   # cada pod, no tempo
```

**Resultado esperado:** `max()` é **uma** linha: ~550 MiB (api-2 é o maior "agora") e só pula para ~950 durante os 15s de pico. `max_over_time` são **duas** linhas, cada uma o topo do seu pod nos últimos 5 min.

Combinando: `max(max_over_time(x[5m]))` = "o maior valor de qualquer pod nos últimos 5 min".

---

### 6. Quando foi o pico?

```promql
time() - ts_of_max_over_time(max_over_time_container_memory_working_set_bytes{pod="api-1"}[10m])
```

**O que faz:** `ts_of_max_over_time` (experimental, habilitada neste lab com `--enable-feature=promql-experimental-functions`) devolve o **timestamp** da amostra máxima. Subtraindo de `time()`, temos "há quantos segundos foi o pico".
**Resultado esperado:** dente-de-serra de **~0 até ~180 s**, zerando a cada novo pico.

> 💡 Detalhe: todos os picos do cenário valem **exatamente** 950 MiB, e `ts_of_max_over_time` devolve a **última** amostra com o valor máximo, por isso aponta para o pico **mais recente**. Se os picos tivessem valores diferentes (ex.: 947, 959, 951), ele apontaria para o **maior** deles, que pode ter acontecido 9 min atrás.

---

### 7 e 8. Subquery: o maior req/s dos últimos 10 min

```promql
rate(max_over_time_http_requests_total[1m])                          # 5 → 50 → 5
max_over_time(rate(max_over_time_http_requests_total[1m])[10m:])     # ≈ 50 constante
```

**O que faz:** `max_over_time` precisa de um **range vector**, mas `rate(...)` devolve um **instant vector**. A **subquery** `[10m:]` resolve: avalia o `rate` em vários instantes dos últimos 10 min (no passo padrão = `evaluation_interval`, 5s aqui) e entrega essa série como range vector.
**Resultado esperado:** `rate[1m]` em **5 req/s**, subindo para um platô de **≈ 50 req/s** a cada 4 min. O `max_over_time(...[10m:])` fica reto em **≈ 50**, porque sempre há uma rajada nos últimos 10 min.

É assim que se responde "qual foi o **pico de tráfego** do dia?": `max_over_time(sum(rate(http_requests_total[5m]))[1d:5m])`.

---

### 9. ❌ Errado: `max_over_time` no counter cru

```promql
max_over_time(max_over_time_http_requests_total[10m])
```

**Resultado esperado:** uma linha **idêntica** ao counter cru. Um counter só sobe, então o máximo da janela é sempre a última amostra. Não diz nada sobre picos de tráfego. Use a subquery do item 8.

---

## 🏭 Casos reais

### 1. OOMKilled "do nada": dimensionando `limits` de memória

O pod do `checkout` é morto por OOM (limite 1 GiB), mas o dashboard de 24h mostra memória "estável em 400 MiB". O motivo é o painel 2 deste lab: o gráfico não enxerga picos de segundos. A query certa para dimensionar o `limit`:

```promql
max_over_time(container_memory_working_set_bytes{namespace="shop", container="checkout"}[1d])
```

E o alerta preventivo (do kubernetes-mixin, adaptado):

```yaml
- alert: ContainerPertoDoLimiteDeMemoria
  expr: |
    max_over_time(container_memory_working_set_bytes{container!=""}[5m])
      / on (namespace, pod, container)
    kube_pod_container_resource_limits{resource="memory"} > 0.9
  for: 5m
  labels: {severity: warning}
```

O cenário imita esse caso com `max_over_time_container_memory_working_set_bytes`.

### 2. Pico de tráfego do dia (capacity planning)

Na reunião pós-Black Friday: "qual foi o pico de req/s?":

```promql
max_over_time(sum(rate(http_requests_total{job="api"}[5m]))[1d:1m])
```

Subquery `[1d:1m]`: avalia o `sum(rate(...))` a cada minuto do dia e pega o maior. Combine com `ts_of_max_over_time` (experimental) para saber **quando**.

### 3. Latência máxima de um gauge de "última requisição"

Alguns exporters expõem só a duração da última execução (ex.: `pg_stat_activity_max_tx_duration`, `probe_duration_seconds`). Para não perder a pior:

```yaml
- record: probe:duration_seconds:max5m
  expr: max_over_time(probe_duration_seconds{job="blackbox"}[5m])
```

### 4. Painel de longo prazo "honesto" no Grafana

```promql
max_over_time(node_load1[$__interval])
```

Cada ponto do gráfico vira "o maior load do intervalo que ele representa". Em 30 dias, os picos continuam aparecendo.

---

## ✅ Quando usar

- **Picos de memória** para dimensionar `limits`: `max_over_time(container_memory_working_set_bytes[1h])`.
- **Painéis de longo prazo sem esconder picos:** `max_over_time(x[$__interval])` no Grafana faz cada ponto ser o máximo do intervalo que ele representa.
- **Pior latência/fila** num período: `max_over_time(queue_depth[15m]) > 1000`.
- **Pico de tráfego** via subquery: `max_over_time(sum(rate(x[5m]))[1d:5m])`.
- **Alertas "já chegou perto"**: `max_over_time(disk_used_ratio[1h]) > 0.95`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Maior valor **entre séries** num instante | operador `max()` / `topk()` |
| Counter (`_total`) | `max_over_time(rate(x[1m])[1h:])` (subquery) |
| Valor "típico" do período | [`avg_over_time()`](../avg_over_time/) ou [`quantile_over_time()`](../quantile_over_time/) |
| Um outlier isolado não deveria contar | [`quantile_over_time(0.99, ...)`](../quantile_over_time/) |
| Quer o **vale** (menor valor) | [`min_over_time()`](../min_over_time/) |
| Latência de histograma | `histogram_quantile()` (histogramas são ignorados pelo `max_over_time`) |

## ⚠️ Pegadinhas

1. **Um único scrape estranho domina.** O `max` é sensível a outliers; às vezes o `quantile_over_time(0.99, ...)` é mais honesto.
2. **Picos menores que o scrape interval não existem para o Prometheus.** Se a memória subiu e desceu entre dois scrapes de 15s, nenhuma função recupera isso.
3. **Subquery sem passo** (`[10m:]`) usa o `evaluation_interval` global. Em janelas longas (`[1d:]`), defina o passo (`[1d:1m]`) para não custar caro.
4. **Resultado não tem `__name__`.** Em joins/regras, lembre que o nome foi removido.
5. **Histograms:** `max_over_time` ignora amostras de histograma (só floats).
6. **O resultado "lembra" do pico pela janela inteira.** Com `[5m]`, um alerta `max_over_time(x[5m]) > limite` continua disparado por até 5 min depois que o valor normalizou.

## 🎓 Na prova PCA

O que costuma cair:
- `max_over_time(x[j])` (por série, no tempo) **vs** `max(x)` / `max by (...)` (entre séries) vs `topk()`.
- Não usar `max_over_time` em counters; usar subquery sobre `rate()`.
- Sintaxe de **subquery**: `<instant query>[<range>:<step>]`, com passo opcional.
- Resolução de gráfico e picos perdidos (por que dashboards longos escondem picos).

**1.** Qual consulta retorna o maior uso de memória de **cada** container nos últimos 30 min?

- A) `max(container_memory_working_set_bytes)`
- B) `max_over_time(container_memory_working_set_bytes[30m])`
- C) `topk(1, container_memory_working_set_bytes)`
- D) `max by (container) (container_memory_working_set_bytes offset 30m)`

<details><summary>Resposta</summary>

**B.** Máximo por série ao longo do tempo. A e C olham só o instante atual entre séries; D olha um instante 30 min atrás.
</details>

**2.** Como obter o maior valor de `rate(http_requests_total[5m])` na última hora?

- A) `max_over_time(http_requests_total[1h])`
- B) `max(rate(http_requests_total[1h]))`
- C) `max_over_time(rate(http_requests_total[5m])[1h:])`
- D) `rate(max_over_time(http_requests_total[1h])[5m])`

<details><summary>Resposta</summary>

**C.** Subquery: o `rate` é avaliado em vários instantes da última hora e o `max_over_time` pega o maior. A dá o valor atual do counter; B é a taxa média da hora (não o pico); D é inválida.
</details>

**3.** Um dashboard de 7 dias mostra memória estável em 400 MiB, mas o pod sofre OOM. Qual a causa mais provável?

- A) O Prometheus descarta picos automaticamente
- B) Cada ponto do gráfico mostra o valor em um instante; picos curtos entre pontos não aparecem
- C) `container_memory_working_set_bytes` é um counter
- D) O lookback delta é de 7 dias

<details><summary>Resposta</summary>

**B.** Com passo grande, o gráfico "amostra" instantes. `max_over_time(x[$__interval])` resolve.
</details>

**4.** O que `max_over_time(node_network_receive_bytes_total[1h])` retorna?

- A) O pico de bytes/s da última hora
- B) Praticamente o valor atual do counter (ou o valor antes de um reset)
- C) Erro de tipo
- D) A soma de bytes da hora

<details><summary>Resposta</summary>

**B.** Counters só crescem: o máximo é a última amostra (a não ser que tenha havido reset). Para pico de taxa, use subquery sobre `rate`.
</details>

## 📝 Cola rápida

- `max_over_time(x[j])` = maior amostra **de cada série** na janela; `max(x)` = maior **entre séries** agora.
- Pega **picos** que gráficos de baixa resolução escondem; use para memória/latência/fila e `limits`.
- Counter? `max_over_time(rate(x[5m])[1h:])` (subquery).
- Ignora amostras de histograma; resultado sem `__name__`.
- Quando foi o pico: `ts_of_max_over_time` (experimental).

## 🔗 Relacionadas

[`min_over_time()`](../min_over_time/) · [`avg_over_time()`](../avg_over_time/) · [`quantile_over_time()`](../quantile_over_time/) · [`ts_of_max_over_time()`](../ts_of_max_over_time/) · [`rate()`](../rate/) · [`last_over_time()`](../last_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
