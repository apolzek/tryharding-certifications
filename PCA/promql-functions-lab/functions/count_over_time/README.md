# `count_over_time()`: quantas amostras cada série tem na janela

> **Em uma frase:** `count_over_time(v[janela])` conta, **para cada série**, **quantas amostras** existem na janela, ignorando completamente os valores. É o jeito de detectar scrapes faltando, séries intermitentes e de contar "quantas vezes aconteceu X" com subquery.

| | |
|---|---|
| **Assinatura** | `count_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer uma (gauge, counter, native histogram: todas as amostras contam igual) |
| **Unidade do resultado** | **número de amostras** (adimensional) |
| **Dashboard** | http://localhost:3300/d/fn-count_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a lista de chamada

O professor passa a lista **toda aula**. No fim do mês, ele não quer saber a nota de ninguém; quer saber **quantas vezes cada aluno respondeu "presente"**.

- Cada **scrape** é uma aula (a cada 5s).
- Cada **amostra** é um "presente".
- `count_over_time(x[1m])` = **quantas presenças** a série teve no último minuto. Com scrape de 5s, o máximo é **12**.

Se um aluno faltou, **não existe** uma presença "0" na lista: simplesmente não tem a marcação. É por isso que `count_over_time` enxerga falhas que [`min_over_time`](../min_over_time/) e [`avg_over_time`](../avg_over_time/) não enxergam: elas só olham as amostras que **existem**.

**Não confunda com o operador `count()`** (veja a planilha em [`avg_over_time`](../avg_over_time/)):

```
 count_over_time(x[1m])  →  quantas AMOSTRAS cada série tem no tempo  ("quantas vezes o João respondeu?")
 count(x)                ↓  quantas SÉRIES existem agora              ("quantos alunos estão na sala?")
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `count_over_time_node_hwmon_temp_celsius{exporter="stable"}` | gauge | aparece em **todo** scrape (~45°C) |
| `count_over_time_node_hwmon_temp_celsius{exporter="flaky"}` | gauge | **some da exposição por 30s a cada 2 min** (segundos 60..90 do ciclo), como um exporter cuja coleta falha |
| `count_over_time_request_latency_ms` | gauge | ~**80 ms**, mas ~**300 ms** nos **primeiros 20s de cada minuto** (4 scrapes lentos/min) |

```bash
curl -s localhost:8088/metrics | grep '^count_over_time_'
# count_over_time_node_hwmon_temp_celsius{exporter="flaky"} 61.2   <- some às vezes
# count_over_time_node_hwmon_temp_celsius{exporter="stable"} 44.1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-count_over_time
```

Espere **~2 min** para `[1m]`/`[2m]` e **~5 min** para a subquery `[5m:5s]`.

---

## 🔍 Queries passo a passo

### 1. O dado cru

```promql
count_over_time_node_hwmon_temp_celsius
```

**Resultado esperado:** `stable` contínuo; `flaky` com **buracos de 30s** a cada 2 min. No buraco não há "0": a série some (o Prometheus grava um *staleness marker* e o gráfico corta a linha).

---

### 2. Scrapes por minuto

```promql
count_over_time(count_over_time_node_hwmon_temp_celsius[1m])
```

**O que faz:** conta as amostras de cada série em `(agora − 1m, agora]`. O range é **aberto à esquerda** no Prometheus 3, então com scrape de 5s são **12** (não 13).
**Resultado esperado:**

| exporter | valor |
|---|---|
| `stable` | **12** (reto) |
| `flaky` | **12** fora do buraco; desce em rampa até **6** quando os 30s de buraco inteiros estão na janela, e sobe de volta |

> Se alguém rodar `tools/deploy.sh`, o gerador reinicia e **ambos** caem para ~11 por um minuto: é um scrape de verdade que faltou. `count_over_time` é honesto!

---

### 3. Janela igual ao período do problema

```promql
count_over_time(count_over_time_node_hwmon_temp_celsius[2m])
```

**Resultado esperado:** `stable` = **24**, `flaky` = **18** (24 − 6), **constantes**: toda janela de 2 min contém exatamente um buraco de 30s.

---

### 4. Porcentagem de scrapes recebidos

```promql
count_over_time(count_over_time_node_hwmon_temp_celsius[1m]) / 12
```

**Resultado esperado:** `stable` = **100%**, `flaky` oscilando entre **50%** e **100%**. Um alerta típico: `count_over_time(x[5m]) / (5*60/5) < 0.9`.

---

### 5. `count()` vs `count_over_time()`

```promql
count(count_over_time_node_hwmon_temp_celsius)                # séries AGORA
count_over_time(count_over_time_node_hwmon_temp_celsius[1m])  # amostras POR série
```

**Resultado esperado:** `count()` é **uma** linha em **2**, caindo para **1** durante os 30s em que `flaky` sumiu. `count_over_time` são **duas** linhas (12 e 6..12), como nos painéis anteriores.

---

### 6 e 7. Subquery: "quantas vezes passou do limite?"

```promql
count_over_time((count_over_time_request_latency_ms > 200)[5m:5s])
count_over_time(count_over_time_request_latency_ms[5m])
```

**O que faz:**
1. `x > 200` (sem `bool`) é um **filtro**: os pontos rápidos **somem**, só ficam os lentos.
2. `[5m:5s]` avalia isso a cada 5s nos últimos 5 min (subquery).
3. `count_over_time` conta quantos pontos sobraram.

**Resultado esperado:** ≈ **20** pontos lentos (4 por minuto × 5) contra **60** amostras no total = **33%** das amostras acima de 200 ms.

> Repare no contraste com [`sum_over_time`](../sum_over_time/): lá usamos `> bool` (vira 0/1 e soma). Aqui usamos `>` sem `bool` (filtra e conta). Os dois dão o mesmo número.

### 9. ❌ O que dá errado: esperar um 0 quando a série some

```promql
count_over_time(count_over_time_node_hwmon_temp_celsius{exporter="flaky"}[20s])
```

**Resultado esperado:** **4** (20s / 5s) enquanto o flaky está no ar e **buracos** (não 0!) durante os 30s em que ele some. Um alerta `count_over_time(x[20s]) < 2` **nunca dispara** justamente quando a série sumiu de vez. Combine com `absent_over_time(x[20s])`.

---

## 🏭 Casos reais

### 1. Scrapes faltando (exporter lento ou timeout intermitente)

O node exporter de alguns hosts demora perto do `scrape_timeout` e às vezes falha. `up` registra 0, mas se o problema é **uma métrica** que o exporter não consegue coletar (ex.: coletor `hwmon` travando), só aquela métrica some. O cenário imita isso com `count_over_time_node_hwmon_temp_celsius`:

```yaml
- alert: MetricaComScrapesFaltando
  # scrape de 15s -> 20 amostras em 5 min
  expr: count_over_time(node_hwmon_temp_celsius[5m]) < 18
  for: 10m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.instance }}: só {{ $value }} de 20 amostras esperadas"
```

### 2. Descobrir o scrape interval real (e scrapes duplicados)

```promql
60 / count_over_time(up{job="kubernetes-pods"}[1m])
```

= segundos entre scrapes de cada target. Se um pod responde 7.5 em vez de 15, provavelmente há **anotações** duplicadas e o job raspa o mesmo endpoint com duas configs.

### 3. Quantas vezes a latência passou do SLO

```promql
count_over_time((probe_duration_seconds{job="blackbox"} > 0.5)[1h:])
```

Quantas sondas da última hora ficaram acima de 500 ms (com passo igual ao scrape, cada ponto ≈ uma sonda). Usado em relatórios de "número de violações".

### 4. Série intermitente (flapping) em Kubernetes

`kube_pod_container_status_ready` some e volta quando o kube-state-metrics reinicia. Para achar pods com dados incompletos:

```yaml
- alert: KubeStateMetricsComFalhas
  # scrape de 15s -> 40 amostras esperadas em 10 min
  expr: count_over_time(kube_pod_container_status_ready[10m]) < 35
  for: 10m
  labels: {severity: info}
- alert: KubeStateMetricsSumiu
  expr: absent_over_time(kube_pod_container_status_ready[10m])
  labels: {severity: critical}
```

O segundo alerta cobre o caso em que a série some **de vez** (onde o `count_over_time` não devolve nada).

---

## ✅ Quando usar

- **Detectar scrapes/amostras faltando:** `count_over_time(up[10m]) < 120` (scrape de 5s) ou, para métricas de aplicação que somem quando a coleta falha, `count_over_time(my_metric[5m]) < 50`.
- **Séries intermitentes (flapping):** quem aparece e some ganha contagem menor.
- **Contar eventos acima de um limite** com subquery: `count_over_time((latency > 0.5)[1h:])`.
- **Descobrir o scrape interval real** de um target: `60 / count_over_time(up{job="x"}[1m])` = segundos entre scrapes.
- **Denominador de médias manuais:** `sum_over_time(x[5m]) / count_over_time(x[5m])` = `avg_over_time(x[5m])`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Contar **eventos** de um counter ("quantas requisições?") | [`increase()`](../increase/): contar amostras de um counter conta scrapes, não requisições |
| Contar **séries** agora | operador `count()` |
| Saber só se a série **existiu** na janela | [`present_over_time()`](../present_over_time/) (sempre 1) |
| Alertar que a série **sumiu de vez** | [`absent_over_time()`](../absent_over_time/) (`count_over_time` de série sumida não devolve 0, devolve **nada**) |
| Quantas vezes o valor **mudou** | [`changes()`](../changes/) |

## ⚠️ Pegadinhas

1. **Série sem amostra na janela não vira 0: some do resultado.** `count_over_time(x[1m]) < 6` **não** alerta quando a série sumiu totalmente. Combine com `absent_over_time()` ou use `or on() vector(0)`.
2. **Depende do scrape interval.** "12 por minuto" só vale para scrape de 5s. Com 15s são 4.
3. **O valor é ignorado:** uma amostra `0`, `NaN` ou `-1` conta igual a qualquer outra. Staleness markers **não** contam.
4. **Range aberto à esquerda (Prometheus 3):** `[1m]` contém 12 amostras de 5s, não 13 como em versões antigas.
5. **Subquery conta pontos da subquery, não amostras cruas.** `count_over_time((x > 200)[5m:5s])` conta avaliações a cada 5s; se o passo for diferente do scrape (ex.: `[5m:1s]`), a mesma amostra é contada várias vezes (o lookback "repete" o último valor).
6. **Histogramas:** amostras de native histogram contam normalmente (como float).

## 🎓 Na prova PCA

O que costuma cair:
- `count_over_time` (amostras de uma série no tempo) vs `count` (séries num instante) vs `increase` (eventos de um counter).
- Série sem amostra na janela **não** retorna 0.
- Relação com o scrape interval (quantas amostras cabem numa janela).
- Filtro vs `bool` em subqueries.

**1.** Scrape interval de 15s. Quanto vale, em condições normais, `count_over_time(up[1m])` no Prometheus 3?

- A) 1
- B) 4
- C) 5
- D) 60

<details><summary>Resposta</summary>

**B.** 60s / 15s = 4. No Prometheus 3 o range é aberto à esquerda `(t−1m, t]`, então a amostra exatamente em `t−1m` não entra (nas versões 2.x podia dar 5 dependendo do alinhamento).
</details>

**2.** Qual a diferença entre `count(x)` e `count_over_time(x[5m])`?

- A) Nenhuma
- B) `count` conta séries num instante; `count_over_time` conta amostras de cada série na janela
- C) `count` conta amostras; `count_over_time` conta séries
- D) `count_over_time` soma os valores

<details><summary>Resposta</summary>

**B.** Eixos diferentes: ↓ entre séries vs → no tempo.
</details>

**3.** Para saber quantos **erros** (`http_errors_total`, counter) ocorreram em 1 hora, use:

- A) `count_over_time(http_errors_total[1h])`
- B) `increase(http_errors_total[1h])`
- C) `count(http_errors_total)`
- D) `sum_over_time(http_errors_total[1h])`

<details><summary>Resposta</summary>

**B.** `count_over_time` contaria **scrapes** (ex.: 240), não erros.
</details>

**4.** Uma série sumiu há 20 min. O que `count_over_time(x[10m]) < 5` retorna para ela?

- A) 0, e o alerta dispara
- B) Nada: a série não está no resultado e o alerta **não** dispara
- C) NaN
- D) 1

<details><summary>Resposta</summary>

**B.** Sem amostras na janela, a série não existe no resultado. Combine com `absent_over_time(x[10m])`.
</details>

## 📝 Cola rápida

- `count_over_time(x[j])` = **quantas amostras** cada série tem na janela; valor ignorado.
- Esperado = janela / scrape interval (range **aberto à esquerda** no Prom 3: `[1m]` / 5s = 12).
- Detecta **scrapes faltando** e **flapping**; série totalmente ausente → **nada** (use `absent_over_time`).
- Counter? Para eventos use `increase`, não `count_over_time`.
- `count_over_time((x > L)[j:])` = quantos pontos acima de L (filtro, sem `bool`).

## 🔗 Relacionadas

[`present_over_time()`](../present_over_time/) · [`absent_over_time()`](../absent_over_time/) · [`sum_over_time()`](../sum_over_time/) · [`changes()`](../changes/) · [`increase()`](../increase/) · [`avg_over_time()`](../avg_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
