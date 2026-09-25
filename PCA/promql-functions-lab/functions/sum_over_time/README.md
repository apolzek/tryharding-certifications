# `sum_over_time()`: somar as amostras de cada série na janela

> **Em uma frase:** `sum_over_time(v[janela])` soma, **para cada série**, os valores de todas as amostras da janela. Só faz sentido quando **cada amostra representa uma quantidade nova** (um "lote"), e não um nível.

| | |
|---|---|
| **Assinatura** | `sum_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge de "lote" (itens desde o último scrape, eventos por coleta) · ✅ 0/1 (via `> bool`) · ✅ native histograms (soma de histogramas) · ❌ Gauge de nível (memória, temperatura) · ❌ Counter |
| **Unidade do resultado** | a da entrada × **número de amostras** (itens por janela, "pontos", ...) |
| **Dashboard** | http://localhost:3300/d/fn-sum_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o cofrinho

Todo dia você coloca no cofrinho **as moedas do troco daquele dia**. No fim do mês, você abre e conta: esse é o `sum_over_time`.

- Cada **scrape** é um dia; o **valor** da amostra são as moedas daquele dia (itens processados **desde o último scrape**).
- `sum_over_time(x[1m])` = moedas que caíram no último minuto = **itens processados no último minuto**.

Agora imagine que, em vez de moedas, você anotasse todo dia **o saldo da conta** (um nível). Somar os saldos de 30 dias dá um número enorme que **não é dinheiro nenhum**. É o erro clássico de usar `sum_over_time` em um gauge como memória.

**Os dois eixos** (veja a planilha em [`avg_over_time`](../avg_over_time/)):

```
 sum_over_time(x[1m])   →  soma as amostras de UMA série no tempo   (1 resultado por worker)
 sum(x)                 ↓  soma as séries num instante             (1 resultado no total)
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `sum_over_time_items_processed_last_scrape{worker="batch-a"}` | gauge (lote) | ~**20** itens por scrape (±3) |
| `sum_over_time_items_processed_last_scrape{worker="batch-b"}` | gauge (lote) | **10** por scrape, **30** durante 60s a cada 4 min |
| `sum_over_time_container_memory_working_set_bytes` | gauge (nível) | ~**512 MiB**. Exemplo do que **não** somar |
| `sum_over_time_cpu_usage_percent` | gauge | senóide **30..90%**, período de **4 min** |

```bash
curl -s localhost:8088/metrics | grep '^sum_over_time_'
# sum_over_time_items_processed_last_scrape{worker="batch-a"} 21
# sum_over_time_items_processed_last_scrape{worker="batch-b"} 10
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sum_over_time
```

Espere **~2 min** para `[1m]` e **~8 min** para a subquery `[8m:5s]`.

---

## 🔍 Queries passo a passo

### 1. As moedas (dado cru)

```promql
sum_over_time_items_processed_last_scrape
```

**Resultado esperado:** pontos em ~20 (`batch-a`) e em 10 (`batch-b`), com `batch-b` pulando para 30 durante 1 min a cada 4 min.

---

### 2. Itens no último minuto

```promql
sum_over_time(sum_over_time_items_processed_last_scrape[1m])
```

**O que faz:** soma as **12 amostras** (60s / 5s) de cada worker na janela `(agora − 1m, agora]`.
**Resultado esperado:**

| worker | valor |
|---|---|
| `batch-a` | ≈ **240** itens/min (12 × 20) |
| `batch-b` | **120** fora do pico, rampa até **360** (12 × 30) durante o pico, e rampa de volta |

---

### 3. `sum = avg × count`

```promql
sum_over_time(x[1m])
avg_over_time(x[1m]) * count_over_time(x[1m])
```

**Resultado esperado:** as duas linhas **idênticas**. Isso deixa claro que o `sum_over_time` depende de **quantas amostras** existem na janela, ou seja, do **scrape interval**. Troque o scrape de 5s para 15s e o "240" vira "80" sem nada ter mudado na aplicação.

---

### 4. `sum()` vs `sum_over_time()`

```promql
sum(sum_over_time_items_processed_last_scrape)                         # ↓ entre workers, 1 scrape
sum(sum_over_time(sum_over_time_items_processed_last_scrape[1m]))      # → no tempo, depois ↓
```

**Resultado esperado:**
- `sum()` sozinho: ≈ **30** (20 + 10), ≈ **50** no pico: itens de **um** scrape, somando os workers.
- `sum(sum_over_time(...[1m]))`: ≈ **360** itens/min, ≈ **600** no pico: total do serviço no último minuto.

---

### 5. ❌ Somar um gauge de nível

```promql
sum_over_time(sum_over_time_container_memory_working_set_bytes[1m])
```

**Resultado esperado:** ≈ **6 GiB** (512 MiB × 12). O pod **nunca** usou 6 GiB. O número só muda se o scrape interval mudar. Para nível, use [`avg_over_time`](../avg_over_time/) ou [`max_over_time`](../max_over_time/).

---

### 6 e 7. Subquery: quanto tempo acima do limite?

```promql
sum_over_time((sum_over_time_cpu_usage_percent > bool 80)[8m:5s]) * 5
```

**O que faz, de dentro para fora:**
1. `x > bool 80` vira **1** quando a CPU passa de 80% e **0** caso contrário (sem o `bool`, os pontos abaixo seriam **removidos**, não zerados).
2. `[8m:5s]` é uma **subquery**: avalia essa expressão a cada 5s nos últimos 8 min (96 pontos).
3. `sum_over_time` soma os 1s = **quantos pontos** passaram de 80%.
4. `* 5` converte "pontos de 5s" em **segundos**.

**Resultado esperado:** ≈ **128 s**. A onda fica acima de 80% em ~27% do período (~64s a cada 4 min); 8 min = 2 ondas. A linha fica quase reta porque a janela tem exatamente 2 períodos.

Equivalente: `avg_over_time((x > bool 80)[8m:5s])` ≈ **0.27** = fração do tempo acima do limite.

---

## 🏭 Casos reais

### 1. "Quantos minutos o node ficou acima de 90% de CPU hoje?"

O time de capacity quer um relatório diário de saturação:

```promql
sum_over_time(
  (
    1 - avg without (cpu, mode) (rate(node_cpu_seconds_total{mode="idle"}[1m]))
  > bool 0.9)[1d:1m]
)
```

Subquery com passo de 1 min: cada ponto vale 1 se o node estava acima de 90%, e a soma = **minutos saturados no dia**. O cenário imita isso com `sum_over_time_cpu_usage_percent` e passo de 5s.

### 2. Gauges de "lote" vindos de batch jobs / Pushgateway

Um job ETL roda a cada 5 min e empurra `etl_records_processed` = registros **daquela execução**. Total do dia:

```yaml
- record: etl:records_processed:sum1d
  expr: sum_over_time(etl_records_processed{job="etl"}[1d])
```

⚠️ Cuidado: o Pushgateway **mantém** o último valor e o Prometheus raspa esse mesmo valor várias vezes; somar daria N×. Só funciona se cada amostra é realmente um lote novo (ex.: exporter que zera após cada scrape). Na dúvida, peça ao time para expor um **counter** e use `increase()`.

### 3. Downtime em minutos (a partir de `up`)

```yaml
- record: job:downtime_minutes:1d
  expr: sum by (job, instance) (sum_over_time((up{job="api"} == bool 0)[1d:1m]))
- alert: DowntimeDiarioAlto
  expr: job:downtime_minutes:1d > 43   # ~ orçamento de erro de 99,9% num mês, gasto num dia
  labels: {severity: warning}
```

= minutos (aprox.) em que o target esteve fora no último dia. Mesma ideia do caso 1, com 0/1.

### 4. Soma de native histograms por janela

Para gauges de histograma (raros: ex. histograma de "tamanho dos arquivos processados neste scrape"), `sum_over_time` soma os histogramas da janela, e `histogram_quantile(0.9, sum_over_time(x[10m]))` dá o p90 dos arquivos dos últimos 10 min.

---

## ✅ Quando usar

- **Gauges de lote:** exporters que expõem "itens/eventos desde a última coleta" (alguns exporters de jobs, filas, logs), ou métricas empurradas por batch jobs.
- **Tempo acima de um limite** (com subquery + `bool`): "quantos minutos a CPU ficou acima de 90% hoje?".
- **Contar condições em 0/1:** `sum_over_time((up == bool 0)[1h:])` = quantos pontos o target esteve fora.
- **Native histograms "por scrape"** (raro): soma de histogramas na janela.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **counter** (`_total`), o caso normal de "quantos na janela" | [`increase()`](../increase/) |
| Gauge de **nível** (memória, fila, temperatura) | [`avg_over_time()`](../avg_over_time/) / [`max_over_time()`](../max_over_time/) |
| Soma **entre séries** num instante | operador `sum()` |
| Contar **amostras** (não valores) | [`count_over_time()`](../count_over_time/) |

## ⚠️ Pegadinhas

1. **Depende do scrape interval.** O resultado é "soma de N amostras"; mudou o scrape, mudou o número. Counters + `increase()` não têm esse problema.
2. **Scrape perdido = moedas perdidas.** Se um scrape falhar, aquele lote **não é contado**. Um counter guardaria o total e o próximo scrape recuperaria a diferença. (Neste lab, quando alguém roda `tools/deploy.sh` o gerador reinicia e você vê pequenos "buracos" na soma.)
3. **Dois Prometheus raspando o mesmo "desde o último scrape"** dividem os lotes entre si: cada um vê só metade. Mais um motivo para preferir counters.
4. **`x > 80` sem `bool`** filtra em vez de zerar: `sum_over_time((x > 80)[8m:5s])` soma os **valores** de CPU acima de 80 (ex.: 85 + 88 + ...), não conta pontos.
5. **Histogramas:** se a janela mistura amostras float e histogram, a série é **removida** do resultado (com aviso).
6. **Resultado sem `__name__`.**

## 🎓 Na prova PCA

O que costuma cair:
- `sum_over_time` (soma das amostras de uma série no tempo) vs `sum` (soma entre séries) vs `increase` (quanto um **counter** cresceu).
- Por que **não** usar `sum_over_time` em counter nem em gauge de nível.
- Uso de `> bool` para transformar condição em 0/1 e somar/contar.
- `sum_over_time = avg_over_time × count_over_time`.

**1.** Para saber quantas requisições `http_requests_total` (counter) recebeu na última hora, a melhor opção é:

- A) `sum_over_time(http_requests_total[1h])`
- B) `increase(http_requests_total[1h])`
- C) `sum(http_requests_total)`
- D) `count_over_time(http_requests_total[1h])`

<details><summary>Resposta</summary>

**B.** `increase` calcula quanto o counter cresceu (tratando resets). A soma os **valores acumulados** (número gigante sem sentido); D conta amostras.
</details>

**2.** Um gauge `memory_bytes` vale ~1 GiB constante, com scrape de 15s. Quanto vale `sum_over_time(memory_bytes[1m])`?

- A) 1 GiB
- B) ~4 GiB, número sem significado físico
- C) 60 GiB
- D) 0

<details><summary>Resposta</summary>

**B.** 4 amostras × 1 GiB. Somar gauge de nível não faz sentido; use `avg_over_time`/`max_over_time`.
</details>

**3.** O que retorna `sum_over_time((cpu_percent > bool 80)[10m:30s])`?

- A) A soma da CPU acima de 80
- B) Quantos pontos (de 30s) nos últimos 10 min tiveram CPU acima de 80
- C) Erro, `bool` não pode ser usado em subquery
- D) 1 ou 0

<details><summary>Resposta</summary>

**B.** `> bool` vira 0/1; a subquery gera 20 pontos; a soma conta os 1s. Multiplicando por 30 = segundos acima de 80%. Sem `bool`, seria A.
</details>

**4.** Qual relação é sempre verdadeira para uma série de floats na mesma janela?

- A) `sum_over_time = avg_over_time / count_over_time`
- B) `sum_over_time = avg_over_time × count_over_time`
- C) `sum_over_time = max_over_time × count_over_time`
- D) `sum_over_time = increase`

<details><summary>Resposta</summary>

**B.** Definição de média: soma / quantidade.
</details>

## 📝 Cola rápida

- `sum_over_time(x[j])` = soma das amostras **de cada série** na janela; depende do **número de scrapes**.
- Só para gauges de **lote** ("desde o último scrape") ou condições 0/1 (`> bool`).
- Counter → `increase()`. Gauge de nível → `avg/max_over_time`.
- `sum_over_time = avg_over_time × count_over_time`.
- `sum_over_time((cond > bool X)[j:passo]) * passo` = tempo em que a condição foi verdadeira.

## 🔗 Relacionadas

[`increase()`](../increase/) · [`count_over_time()`](../count_over_time/) · [`avg_over_time()`](../avg_over_time/) · [`max_over_time()`](../max_over_time/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
