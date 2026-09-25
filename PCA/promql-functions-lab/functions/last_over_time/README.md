# `last_over_time()`: o último valor visto dentro da janela

> **Em uma frase:** `last_over_time(v[janela])` devolve, **para cada série**, a **amostra mais recente** dentro da janela. Serve para "segurar" o último valor de métricas **esparsas ou intermitentes** (batch jobs, séries que somem) por quanto tempo você quiser.

| | |
|---|---|
| **Assinatura** | `last_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer uma (gauge, counter, info, native histogram: funciona igual para float e histograma) |
| **Unidade do resultado** | a **mesma** da entrada (e o `__name__` é **mantido**) |
| **Dashboard** | http://localhost:3300/d/fn-last_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o placar do jogo que já acabou

O jogo dura 90 minutos, mas o **placar fica aceso no telão** até o próximo jogo. Quem chega no estádio no dia seguinte ainda vê "2 × 1".

- O batch job é o **jogo**: roda por poucos segundos e expõe `items_processed`. Quando termina, a série **some** (o Prometheus grava um *staleness marker*).
- A consulta crua `x` é a **TV ao vivo**: só mostra alguma coisa durante o jogo.
- `last_over_time(x[5m])` é o **placar pendurado**: "o último resultado que apareceu nos últimos 5 min".

A janela é **o tempo que o placar fica aceso**. Se o próximo jogo demora mais que a janela, o placar apaga (a série some do resultado).

Por série e **no tempo** (→). Não existe um operador `last()` entre séries; o mais próximo, "qual série tem o maior valor agora", seria `topk(1, x)`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `last_over_time_batch_items_processed{job_name="export"}` | gauge esparso | existe só **~20s a cada 2 min**; valor da execução: **1000 → 1250 → 1500 → 1750 → 2000 → 1000...** |
| `last_over_time_batch_items_processed{job_name="report"}` | gauge esparso | existe só **~20s a cada 3 min**; valor: **300 → 600 → 900 → 300...** |

O label se chama `job_name` (e não `job`) porque `job` é reservado para o target no Prometheus e viraria `exported_job`.

```bash
curl -s localhost:8088/metrics | grep '^last_over_time_'
# (vazio na maior parte do tempo!)
# last_over_time_batch_items_processed{job_name="export"} 1500   <- só nos ~20s de execução
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-last_over_time
```

Espere **~6 min** para ver as duas execuções várias vezes.

---

## 🔍 Queries passo a passo

### 1. Cru: quase nada

```promql
last_over_time_batch_items_processed
```

**Resultado esperado:** **pontinhos isolados** (4 scrapes por execução): `export` a cada 2 min, `report` a cada 3 min. Entre eles, **nada**. Repare: mesmo com `lookback-delta` de 5 min, o seletor instantâneo **não** preenche o buraco, porque o staleness marker diz "essa série acabou".

---

### 2. `last_over_time(x[5m])`: o placar pendurado

```promql
last_over_time(last_over_time_batch_items_processed[5m])
```

**O que faz:** para cada série, pega a amostra mais recente em `(agora − 5m, agora]` e devolve o valor dela (com o timestamp da avaliação).
**Resultado esperado:** **degraus contínuos**. `export`: 1000 → 1250 → 1500 → ... mudando a cada 2 min. `report`: 300 → 600 → 900 → ... a cada 3 min.

---

### 3 e 4. O tamanho da janela decide se há buraco

```promql
last_over_time(last_over_time_batch_items_processed[1m])   # curta demais para os dois
last_over_time(last_over_time_batch_items_processed[2m])   # ok para export, curta para report
```

**Resultado esperado:**
- `[1m]`: `export` com buracos de **~40 s** (a janela "esquece" o valor 1 min depois da execução); `report` com buracos de **~1m40s**.
- `[2m]`: `export` contínuo; `report` ainda com buracos de **~40 s**.

**Regra:** janela **maior** que o maior intervalo esperado entre amostras (com folga para atraso). Para um cron diário, `[1d]` ou `[26h]`.

---

### 5. Idade do dado

```promql
time() - ts_of_last_over_time(last_over_time_batch_items_processed[10m])
```

**O que faz:** `ts_of_last_over_time` (experimental, habilitada neste lab) devolve o **timestamp** da última amostra. `time() -` isso = **segundos desde a última execução**.
**Resultado esperado:** dente-de-serra: `export` de **0 a ~100 s**, `report` de **0 a ~160 s**. Um alerta `> 300` avisaria que um job atrasou.

> Sem a feature flag, a forma clássica é o job expor o próprio horário: `time() - job_last_success_timestamp_seconds`.

---

### 6. Usando o valor "preenchido" em contas

```promql
sum(last_over_time_batch_items_processed)                  # cru
sum(last_over_time(last_over_time_batch_items_processed[5m]))
```

**Resultado esperado:** o `sum` cru só existe nos segundos em que **algum** job roda, e soma só quem está rodando (ora ~1500, ora ~600, às vezes os dois). O `sum(last_over_time(...))` é contínuo: última execução do export + última do report, entre **1300** e **2900**.

---

### 7. Tabela

```promql
last_over_time(last_over_time_batch_items_processed[5m])
```

(instantânea) **Resultado esperado:** duas linhas, `export` e `report`, com o valor da última execução de cada um. Note que a coluna `__name__` **continua lá**: `last_over_time` (assim como `first_over_time`) **mantém o nome da métrica**, ao contrário de `avg/max/min/sum/count_over_time`.

---

## 🏭 Casos reais

### 1. Batch job de backup: "quando rodou e quanto fez?"

Um CronJob de backup roda 1x por dia às 02:00 e expõe métricas só enquanto roda (ou o scrape é feito via Kubernetes SD enquanto o pod existe). De manhã, o dashboard deve mostrar **o tamanho do último backup**:

```promql
last_over_time(backup_size_bytes{job="backup"}[26h])
```

26h = 1 dia + folga de 2h para atrasos. Com `[1d]`, se o backup de hoje atrasar 10 min, o painel fica vazio por 10 min.

### 2. Join com métricas que "piscam" (kube-state-metrics)

`kube_pod_info` e `kube_pod_labels` somem quando o kube-state-metrics reinicia ou tem um scrape lento. Um join direto quebra por alguns segundos e gera **falsos alertas**:

```promql
sum by (namespace, pod) (rate(container_cpu_usage_seconds_total[5m]))
  * on (namespace, pod) group_left (node)
    last_over_time(kube_pod_info[5m])
```

O `last_over_time(...[5m])` "segura" o lado info por até 5 min e o join continua estável.

### 3. Métricas de baixa frequência (scrape de 5 min, SNMP, cloud)

Exporters de nuvem (CloudWatch, Stackdriver) frequentemente têm scrape de 5–10 min, maior que o `lookback-delta` padrão de 5 min. Sem nada, o gráfico fica pontilhado:

```yaml
groups:
  - name: cloud
    rules:
      - record: aws_rds:free_storage_space_bytes:last
        expr: last_over_time(aws_rds_free_storage_space_average[15m])
      - alert: RDSDiscoQuaseCheio
        expr: aws_rds:free_storage_space_bytes:last < 10e9
        for: 30m
```

### 4. Alerta de "job atrasado" (sem a métrica de timestamp)

```yaml
- alert: ExportJobAtrasado
  expr: time() - ts_of_last_over_time(batch_items_processed{job_name="export"}[1h]) > 600
  labels: {severity: warning}
  annotations:
    summary: "Job export não roda há mais de 10 min"
```

(Requer `--enable-feature=promql-experimental-functions`. Em produção, prefira o job expor `*_last_success_timestamp_seconds` via Pushgateway.)

---

## ✅ Quando usar

- **Métricas esparsas/intermitentes** (batch jobs, cron, séries que somem) que devem aparecer contínuas em painéis.
- **Joins** com séries info (`kube_pod_info`, `kube_node_labels`, `target_info`) que podem piscar.
- **Scrape interval maior que o lookback** (5 min): `last_over_time(x[15m])`.
- **Recording rules** para "congelar" o último valor conhecido.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica contínua (scrape normal) | o seletor puro `x` já pega a última amostra (dentro do lookback de 5 min) |
| Quer o valor **mais antigo** da janela | [`first_over_time()`](../first_over_time/) |
| Quer saber **se** a série existiu | [`present_over_time()`](../present_over_time/) |
| Alertar que a métrica **parou de chegar** | [`absent_over_time()`](../absent_over_time/) (o `last_over_time` **esconde** o problema pela janela inteira) |
| Pushgateway | ele já mantém o último valor; `last_over_time` é desnecessário |

## ⚠️ Pegadinhas

1. **Esconde dados velhos:** com `[1d]`, um exporter que morreu há 23h ainda aparece "vivo" com o último valor. Combine com a idade (`ts_of_last_over_time`) ou com `absent_over_time`.
2. **Janela menor que o intervalo entre amostras = buracos.** Veja os painéis 3 e 4.
3. **O timestamp do resultado é o da avaliação**, não o da amostra. `timestamp(last_over_time(x[5m]))` devolve o horário da consulta; para o horário da amostra use `ts_of_last_over_time` (experimental) ou `timestamp(x)` no seletor puro.
4. **Mantém `__name__`** (como `first_over_time`), então `last_over_time(a[5m]) or last_over_time(b[5m])` preserva os nomes. As outras `*_over_time` removem.
5. **Staleness vs lookback:** o seletor puro preenche até 5 min **somente** se não houve staleness marker. Séries que somem do target (como aqui) cortam na hora; por isso o `last_over_time` é necessário.

## 🎓 Na prova PCA

O que costuma cair:
- `last_over_time` recebe **range vector** e devolve **instant vector**, um valor por série.
- A diferença entre **lookback delta** (5 min, seletor instantâneo) e uma **janela explícita** (`[x]`), e o papel dos **staleness markers**.
- Uso com séries **esparsas** e em **joins** com métricas info.
- Quais `*_over_time` preservam o nome da métrica.

**1.** Um batch job expõe `items_processed` por 20s a cada 30 min. Qual consulta mostra o último valor continuamente num painel?

- A) `items_processed`
- B) `last_over_time(items_processed[5m])`
- C) `last_over_time(items_processed[35m])`
- D) `max(items_processed)`

<details><summary>Resposta</summary>

**C.** A janela precisa ser **maior** que o intervalo entre execuções (30 min) com folga. A) só mostra algo durante os 20s (staleness). B) tem buracos de 25 min. D) é agregação entre séries, também só existe durante a execução.
</details>

**2.** Sem `last_over_time`, por quanto tempo uma consulta instantânea `x` continua devolvendo a última amostra de uma série **que ainda existe** no target, mas não teve scrape recente?

- A) 1 minuto
- B) Até o próximo scrape
- C) Até 5 minutos (lookback delta padrão)
- D) Para sempre

<details><summary>Resposta</summary>

**C.** O `--query.lookback-delta` padrão é 5m. Se a série **sumiu** do target (staleness marker), porém, o resultado para imediatamente.
</details>

**3.** Qual destas funções **mantém** o label `__name__` no resultado?

- A) `avg_over_time`
- B) `last_over_time`
- C) `sum_over_time`
- D) `rate`

<details><summary>Resposta</summary>

**B.** No Prometheus 3, `last_over_time` e `first_over_time` mantêm o nome (só selecionam uma amostra, não transformam o valor). As demais removem.
</details>

**4.** Qual é o risco de `last_over_time(temperature[1d])` num alerta de temperatura?

- A) Nenhum
- B) Um sensor morto há horas continua aparecendo com o último valor e o alerta nunca percebe
- C) Soma as temperaturas do dia
- D) Erro de sintaxe: janela máxima é 1h

<details><summary>Resposta</summary>

**B.** O `last_over_time` "segura" o valor pela janela inteira, escondendo a falta de dados. Use janelas curtas e/ou um alerta de `absent_over_time`.
</details>

## 📝 Cola rápida

- `last_over_time(x[j])` = **última amostra** de cada série dentro de `(agora − j, agora]`.
- Serve para séries **esparsas** e **joins** com info metrics; janela **>** maior intervalo entre amostras.
- Seletor puro já tem lookback de **5 min**, mas **staleness markers** cortam na hora.
- **Mantém `__name__`** (junto com `first_over_time`).
- Esconde dados velhos: combine com `absent_over_time` ou com a idade (`time() - ts_of_last_over_time(...)`).

## 🔗 Relacionadas

[`first_over_time()`](../first_over_time/) · [`present_over_time()`](../present_over_time/) · [`absent_over_time()`](../absent_over_time/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/) · [`timestamp()`](../timestamp/) · [`count_over_time()`](../count_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
