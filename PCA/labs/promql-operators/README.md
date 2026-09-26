# 🧮 PromQL sem funções: tipos, seletores, modificadores, operadores e vector matching

> **Em uma frase:** tudo o que o PromQL faz **além** das funções — escolher séries (seletores), viajar no tempo (`offset`, `@`, subquery), fazer conta e comparar (operadores binários), cruzar duas métricas (**vector matching**, o "PROCV" do Prometheus) e resumir muitas séries em poucas (agregações).

| | |
|---|---|
| **Domínio da PCA** | **PromQL — 28%** da prova (o maior domínio). Funções estão em [`promql-functions-lab`](../../promql-functions-lab/); aqui fica o resto. |
| **Stack** | usa o Prometheus do **promql-functions-lab** (`localhost:9095`, v3.15.0) — este lab **não** tem compose próprio |
| **Dashboard** | http://localhost:3300/d/fn-promql-operators |
| **Desafios** | 54 desafios auto-corrigidos, ids **200–253**: `cd ../../challenges && ./check.py list --topic operators` |
| **Exercícios** | [`exercises/`](exercises/) (6 "complete a query") |
| **Teste** | [`./test.sh`](test.sh) |

---

## 🧠 Analogia: a planilha gigante

Imagine o Prometheus como uma **planilha gigante** onde cada **linha** é uma série temporal
(`nome + labels`) e cada **coluna** é um instante de tempo.

| PromQL | Na planilha | Em SQL |
|---|---|---|
| **seletor** `metrica{label="x"}` | **Filtro** do Excel numa coluna | `WHERE label = 'x'` |
| `offset 1h` / `@ <ts>` | olhar **outra coluna** (outro horário) | `AS OF TIMESTAMP` |
| `[5m]` (range vector) | pegar **várias colunas** de uma vez (um pedaço da linha) | janela |
| subquery `expr[30m:1m]` | calcular uma **coluna auxiliar** várias vezes e guardar o histórico | CTE avaliada em loop |
| operador aritmético `a / b` | fórmula `=A2/B2` linha a linha | `SELECT a.v / b.v` |
| comparação `a > 10` | **filtro** "maior que" (esconde as linhas que não passam) | `WHERE v > 10` |
| `bool` | fórmula `=SE(A2>10;1;0)` (não esconde nada) | `CASE WHEN v > 10 THEN 1 ELSE 0 END` |
| **vector matching** `a / on(service) group_left b` | **PROCV / VLOOKUP**: para cada linha de A, procure a linha de B com a mesma chave | `JOIN ... ON a.service = b.service` |
| `and` / `unless` / `or` | interseção / diferença / união de **linhas** (pelas chaves, não pelos valores) | `INNER JOIN` (semi) / `NOT EXISTS` / `UNION` |
| agregação `sum by(ns)(...)` | **Tabela dinâmica** agrupando por `ns` | `GROUP BY ns` |

Guarde essa tabela: quase toda pegadinha da prova é esquecer que o **JOIN do PromQL é feito pelos
labels**, e que ele é **INNER** (quem não tem par, some).

---

## 🏗️ Arquitetura

```
┌──────────────────────────────┐   scrape 5s   ┌─────────────────────────────┐
│ gerador de métricas fake     │ ────────────▶ │ Prometheus v3.15.0          │◀── ./check.py (desafios 200–253)
│ localhost:8088/metrics       │               │ localhost:9095              │◀── ./test.sh (roda as queries deste README)
│ (~90 cenários do functions-  │               │ --enable-feature=promql-    │
│  lab: sort_*, label_replace_*│               │   experimental-functions    │
│  target_info, ...)           │               └─────────────┬───────────────┘
└──────────────────────────────┘                             │
                                                             ▼
                                               Grafana 13.2.2 localhost:3300
                                               dashboard fn-promql-operators
```

Todas as séries têm também os labels de scrape `job="lab"`, `instance="generator:8080"` e
`env="lab"` (nas tabelas abaixo eles foram omitidos para caber na tela). O Prometheus também
raspa a si mesmo (`job="prometheus"`), então `up` tem 2 séries.

### As métricas que usamos (todas já existem no gerador)

| Métrica | Labels | Valor | Usada para |
|---|---|---|---|
| `sort_desc_http_requests_total` | `service`, `code` | counter; `rate` **exato**: 200 → cart 99, checkout 95, payments 88, recommendations 0 · 500 → 1, 5, 12, 0 req/s | vector matching, taxa de erro, `and`/`unless` |
| `sort_cronjob_last_duration_seconds` | `cronjob` | **constante**: backup 120, cleanup 8, report 45, sync **NaN** | comparação, `bool`, NaN |
| `sort_node_filesystem_size_bytes` | `node`, `mountpoint` | **constante**: 100, 50, 200, 500, 1024 GiB | aritmética |
| `sort_node_filesystem_avail_bytes` | `node`, `mountpoint` | onda (varia) | one-to-one |
| `sort_by_label_desc_kafka_log_size_bytes` | `topic`, `partition` | **constante**: partição *n* = (*n*+1) GiB, n = 0..11 | agregações exatas |
| `sort_by_label_app_build_info` | `pod`, `version` | 1 (11 pods, 4 versões) | `count`, `group` |
| `days_in_month_node_total_hourly_cost` | `node`, `instance_type` | **constante**: 0.096, 0.096, 0.34 | `count_values` |
| `sort_by_label_node_load1` | `node`, `zone` | onda (12 nós, 2 zonas) | `topk by` |
| `sort_desc_container_cpu_usage_seconds_total` | `namespace`, `pod` | counter de CPU | seletores, `sum by` |
| `ceil_kube_deployment_spec_replicas` | `deployment` | checkout = **5** | one-to-one |
| `label_replace_kube_deployment_status_replicas_available` | `deployment` | cart 1, checkout **3**, payments 2 | one-to-one |
| `info_http_server_requests_total` + `target_info` | `route` / `version`, `k8s_*` | counter / 1 | `group_left(version)` |
| `label_replace_node_cpu_usage_percent` + `label_replace_kube_node_info` | `endpoint` / `internal_ip`, `node` | % / 1 | PROCV IP → nó |
| `increase_http_requests_total` | `code` | counter: 2/s (200) e ~0.1/s (500) | `offset`, `@` |
| `rate_http_requests_total` | `route` | counter com **pico de 40 req/s** em `/api/search` a cada 5 min | subquery |
| `scalar_node_cpu_usage_percent` + `scalar_cpu_alert_threshold_percent` | `node` / – | % / limite 70↔80 | vetor vs escalar |

```bash
curl -s localhost:8088/metrics | grep -E '^(sort_desc_http|sort_cronjob|sort_by_label_desc_kafka)'
```

## ▶️ Como rodar

```bash
# 1) o promql-functions-lab precisa estar no ar
curl -sf localhost:9095/-/ready || (cd ../../promql-functions-lab && tools/deploy.sh)

# 2) abra http://localhost:9095 (aba Table = instant query) e cole as queries abaixo
#    ou o Grafana: http://localhost:3300/d/fn-promql-operators

# 3) desafios auto-corrigidos
cd ../../challenges
./check.py list --topic operators
./check.py show 233
./check.py 233 'sua query aqui'

# 4) teste automático do lab (roda todas as queries deste README + selftest dos desafios)
./test.sh
```

> Convenção deste README: **cada linha** de um bloco ` ```promql ` é uma query executável (o `#` é
> comentário em PromQL). Linhas marcadas com `# ❌` **devem dar erro** — o `test.sh` confere as duas coisas.

---

## 🔍 Passo a passo

### 1. Tipos de dados

Toda expressão PromQL resulta em **um de 4 tipos**:

| Tipo | O que é | Exemplo | Pode ser resultado de gráfico (range query)? |
|---|---|---|---|
| **instant vector** | N séries, **1 amostra** cada, todas no mesmo instante | `sort_cronjob_last_duration_seconds` | ✅ |
| **range vector** | N séries, **várias amostras** cada (uma janela) | `sort_cronjob_last_duration_seconds[1m]` | ❌ (só instant query / aba Table) |
| **scalar** | um número solto, **sem labels** | `42`, `scalar(x)`, `time()` | ✅ |
| **string** | texto; quase não é usado | `"texto"` | ❌ |

```promql
sort_cronjob_last_duration_seconds                                  # instant vector: 4 séries
sort_cronjob_last_duration_seconds[1m]                              # range vector: 4 séries x ~12 amostras (scrape de 5s)
42                                                                  # scalar
"texto"                                                             # string
scalar(scalar_cpu_alert_threshold_percent)                          # vetor de 1 elemento -> scalar
rate(sort_cronjob_last_duration_seconds)                            # ❌ rate() quer range vector, recebeu instant
sum(sort_cronjob_last_duration_seconds[5m])                         # ❌ agregação quer instant vector, recebeu range
```

**Resultado esperado:**
- `sort_cronjob_last_duration_seconds` → `backup 120`, `cleanup 8`, `report 45`, `sync NaN`.
- `[1m]` → a aba **Table** mostra `120 @1790...` repetido ~12 vezes por série. A aba **Graph** dá erro
  `invalid expression type "range vector" for range query, must be Scalar or instant Vector`.
- As duas últimas dão **erro de parse** (`expected type range vector in call to function "rate", got instant vector`).

> **Instant query vs range query.** Não confunda *tipo de resultado* com *tipo de consulta*: uma
> **range query** (o gráfico) é só uma **instant query repetida a cada `step`**. Por isso ela só aceita
> expressões que resultam em instant vector ou scalar.

**Literais úteis:** durações `1h30m`, `90s`, `2d`; números `1e9`, `0x10`, `1_000_000`, `Inf`, `NaN`.
Durações podem até ter aritmética: `rate(x[5m * 2])`, `x offset (1h / 2)`.

---

### 2. Seletores e matchers

Um seletor escolhe **quais séries** entram. `metrica` é açúcar para `{__name__="metrica"}`.

| Matcher | Significa | Exemplo |
|---|---|---|
| `=` | igual | `{namespace="shop"}` |
| `!=` | diferente | `{namespace!="shop"}` |
| `=~` | casa com a regex (RE2, **ancorada**) | `{pod=~"api-.*"}` |
| `!~` | **não** casa com a regex | `{namespace!~"kube-system\|batch"}` |

```promql
sort_desc_container_cpu_usage_seconds_total{namespace="shop"}                     # 4 pods
sort_desc_container_cpu_usage_seconds_total{pod=~"api-.*"}                        # api-7f9c-a, api-7f9c-b
sort_desc_container_cpu_usage_seconds_total{pod=~"api"}                           # VAZIO: regex é ancorada (^api$)
sort_desc_container_cpu_usage_seconds_total{pod=~"(?i)API-.*"}                    # (?i) = case-insensitive
sort_desc_container_cpu_usage_seconds_total{namespace!~"kube-system|batch"}       # só shop
sort_desc_container_cpu_usage_seconds_total{namespace="shop", pod!~"api-.*"}      # redis-0, web-5d8e-x (matchers = AND)
label_replace_container_memory_working_set_bytes{pod=~"checkout-.*", pod!="checkout-7d9f8b6c4-m3n4b"}   # 2 matchers no MESMO label
label_join_http_requests_total{region=""}                                         # séries SEM o label region (staging)
label_join_http_requests_total{region!=""}                                        # séries COM region (prod)
{__name__=~"sort_node_filesystem_.+_bytes"}                                       # 10 séries, 2 métricas
{job=~".*"}                                                                       # ❌ precisa de 1 matcher que NÃO case com vazio
{job=~".+"} == 0                                                                  # ok: .+ não casa com vazio
```

**Resultado esperado / o que observar:**

- **Regex ancorada**: `=~"api"` é `^api$` → nada. Para "contém" use `=~".*api.*"`, para "começa com" `=~"api.*"`.
- **Label vazio = label ausente.** `{region=""}` pega as 2 séries de **staging**, que **não têm** `region`.
- Vários matchers são **AND**. Para OR entre valores do mesmo label, use regex `a|b`. Para OR entre labels diferentes, só com o operador `or`.
- `{__name__=~"..."}` seleciona por **nome** — útil para achar métricas (`count by(__name__)({__name__=~"node_.+"})`).

---

### 3. `offset`

`offset <duração>` desloca **aquele seletor** no tempo, para trás (ou para frente com duração negativa).

```promql
increase_http_requests_total offset 1h                                        # o valor do counter 1h atrás
increase_http_requests_total - increase_http_requests_total offset 1h         # ≈ 7200 (200) e ≈ 360 (500)
rate(increase_http_requests_total[5m] offset 1h)                              # a taxa como estava 1h atrás
sum(sort_desc_http_requests_total offset 10m)                                 # ✅ offset colado no seletor
sum(sort_desc_http_requests_total) offset 10m                                 # ❌ offset depois de uma agregação
increase_http_requests_total offset -1h                                       # olha para o FUTURO (aqui: vazio)
```

**Resultado esperado:** `increase_http_requests_total` cresce **2/s** (200) → em 1 h, `7200`; o 500
cresce ~0.1/s → `≈ 360`. `sum(x) offset 10m` dá `parse error: offset modifier must be preceded by an
instant vector selector or range vector selector or a subquery`.

**Uso clássico — "comparado com a semana passada":**

```promql
sum(rate(sort_desc_http_requests_total[5m])) / sum(rate(sort_desc_http_requests_total[5m] offset 1h))   # 1 = igual a 1h atrás
```

---

### 4. Modificador `@`

`@ <timestamp unix>` fixa o **instante absoluto** de avaliação de um seletor.
`@ start()` e `@ end()` = início e fim da consulta (numa **instant query**, os dois = o instante avaliado).

```promql
sort_cronjob_last_duration_seconds @ 1790430000                                   # valor em 2026-09-26 13:40 UTC (vazio se fora da retenção)
increase_http_requests_total @ end() - increase_http_requests_total @ end() offset 30m   # offset é RELATIVO ao @ -> ≈ 3600 e ≈ 180
increase_http_requests_total offset 30m @ end()                                   # a ordem @/offset não importa
increase_http_requests_total @ end() - increase_http_requests_total @ start()     # instant query: start()=end() -> 0
```

**Onde o `@` brilha: gráficos.** Num painel (range query), `topk(5, rate(x[5m]))` é recalculado **a cada
ponto**, e a legenda acaba com 15 séries. Com `@ end()` você escolhe o top 5 **do fim do período** e
desenha a história **só delas**:

```promql
rate(sort_desc_container_cpu_usage_seconds_total[5m]) and topk(3, rate(sort_desc_container_cpu_usage_seconds_total[5m] @ end()))
```

(O dashboard mostra lado a lado o `topk` "normal" e esse com `@ end()`.)

> Duração com aritmética **não** vale no `@` (`@ (time() - 60)` é inválido): ele só aceita número literal, `start()` ou `end()`.

---

### 5. Subqueries

`<expressão instant>[<range>:<resolução>]` roda a expressão **várias vezes** (uma a cada `resolução`,
dentro de `range`) e devolve um **range vector**. Serve para aplicar `*_over_time` em algo que **não é
um seletor** (ex.: o resultado de `rate` ou de `sum`).

```promql
max_over_time(rate(rate_http_requests_total{route="/api/search"}[1m])[10m:30s])     # ≈ 40: o pico
max_over_time(rate(rate_http_requests_total{route="/api/search"}[1m])[10m:])        # resolução omitida = evaluation_interval (5s aqui)
rate(rate_http_requests_total{route="/api/search"}[10m])                            # ≈ 12: a média DILUI o pico
max_over_time(sum(rate(sort_desc_http_requests_total[1m]))[15m:1m])                 # 300 (99+95+88+1+5+12)
count_over_time(sum(rate(sort_desc_http_requests_total[1m]))[10m:1m])               # 10 pontos
count_over_time((rate(rate_http_requests_total{route="/api/search"}[1m]) > 30)[30m:1m])   # nº de minutos acima de 30 req/s
max_over_time(sum(rate(sort_desc_http_requests_total[1m]))[15m])                    # ❌ range [15m] só em seletor: falta o ":"
```

**Resultado esperado:** o `max_over_time(...[10m:30s])` fica em **≈ 40** o tempo todo (sempre há um pico
nos últimos 10 min), enquanto `rate(...[10m])` fica em **≈ 12**.

> ⚠️ Subquery é cara: `[30d:1m]` = 43 200 avaliações por série. Em produção prefira uma **recording rule**
> e aplique `max_over_time` sobre a série gravada.

---

### 6. Operadores aritméticos

`+  -  *  /  %  ^` (e `atan2`). Funcionam entre **scalar/scalar**, **vetor/scalar** (aplica em cada
série) e **vetor/vetor** (casamento de labels — seção 8).

```promql
sort_node_filesystem_size_bytes / 1024^3                     # 100, 50, 200, 500, 1024 GiB
sort_node_filesystem_size_bytes / 1024*1024*1024             # ERRADO: ((x/1024)*1024)*1024 = x*1024
100 * (1 - sort_node_filesystem_avail_bytes / sort_node_filesystem_size_bytes)   # % usado
rate(sort_desc_http_requests_total{service="checkout"}[5m]) * 60                 # req/min: 5700 (200) e 300 (500)
-sort_cronjob_last_duration_seconds{cronjob="backup"}        # menos unário: -120
2 ^ 3 ^ 2                                                    # 512  (^ é associativo à DIREITA: 2^(3^2))
-2 ^ 2                                                       # -4   (^ antes do menos unário)
2 * 3 % 4                                                    # 2    (esquerda p/ direita: (2*3)%4)
5 % 3                                                        # 2
1 atan2 1                                                    # 0.785 (π/4) — atan2 é operador binário
```

**O que observar:** o resultado de uma operação aritmética **perde o nome da métrica** (`__name__`)
— faz sentido: `bytes / 1024^3` não é mais "bytes". Divisão por zero dá `+Inf` (ou `NaN` para 0/0), não erro.

---

### 7. Operadores de comparação

`==  !=  >  <  >=  <=`. Por padrão **filtram**: a série fica (com o valor **original**) se a condição
for verdadeira. Com **`bool`**, **não filtram**: devolvem `1`/`0` e descartam o nome.

```promql
sort_cronjob_last_duration_seconds > 60                  # backup 120 (mantém o nome e o valor)
sort_cronjob_last_duration_seconds > bool 30             # backup 1, cleanup 0, report 1, sync 0 (NaN -> 0)
sort_cronjob_last_duration_seconds != 120                # cleanup 8, report 45, sync NaN  (NaN != x é VERDADEIRO)
sort_cronjob_last_duration_seconds == sort_cronjob_last_duration_seconds   # tira o NaN (NaN == NaN é falso)
count(sort_cronjob_last_duration_seconds > 30)           # 2
sum(sort_cronjob_last_duration_seconds > bool 30)        # 2 (e 0, não vazio, se ninguém passar)
count(sort_cronjob_last_duration_seconds > bool 30)      # 4 — pegadinha: conta os zeros também
1 > bool 2                                               # scalar 0
1 > 2                                                    # ❌ escalar vs escalar EXIGE bool
scalar_node_cpu_usage_percent > scalar(scalar_cpu_alert_threshold_percent)   # vetor vs limite vindo de métrica
scalar_node_cpu_usage_percent > scalar_cpu_alert_threshold_percent           # VAZIO: labels não casam
```

**Comparação vetor/vetor devolve o valor da ESQUERDA:**

```promql
ceil_kube_deployment_spec_replicas != label_replace_kube_deployment_status_replicas_available   # checkout 5 (desejado)
ceil_kube_deployment_spec_replicas - label_replace_kube_deployment_status_replicas_available    # checkout 2 (faltando)
```

---

### 8. Vector matching

Quando os **dois lados** de um operador binário são vetores, o PromQL precisa decidir **qual série da
esquerda combina com qual da direita**. É um **JOIN** — ou um **PROCV**: "para cada linha da esquerda,
procure na direita a linha com a mesma chave".

- A **chave** padrão é **o conjunto inteiro de labels** (menos `__name__`).
- `on(a, b)` → a chave é **só** `a, b`. `ignoring(c)` → a chave é **tudo menos** `c`.
- É um **INNER JOIN**: quem não tem par **some** do resultado (sem erro, sem aviso).

#### 8.1 One-to-one (1 série de cada lado por chave)

```promql
sort_node_filesystem_avail_bytes / sort_node_filesystem_size_bytes      # 5 séries: fração livre
ceil_kube_deployment_spec_replicas - label_replace_kube_deployment_status_replicas_available   # só checkout: 5 - 3 = 2
```

`cart` e `payments` existem só à direita → somem. É o que acontece quando você escreve uma razão e
"metade dos serviços desaparece do gráfico".

#### 8.2 Labels diferentes: `ignoring` / `on`

```promql
rate(sort_desc_http_requests_total{code="500"}[5m]) / rate(sort_desc_http_requests_total{code="200"}[5m])                   # VAZIO: code difere
rate(sort_desc_http_requests_total{code="500"}[5m]) / ignoring(code) rate(sort_desc_http_requests_total{code="200"}[5m])   # cart 0.0101, checkout 0.0526, payments 0.136, recommendations NaN
rate(sort_desc_http_requests_total{code="500"}[5m]) / on(service) rate(sort_desc_http_requests_total{code="200"}[5m])      # mesmos valores, mas SÓ com o label service
```

> Com `on(...)` num casamento one-to-one, o resultado fica **só com os labels do `on`**. Com
> `ignoring(...)`, fica com todos **menos** os ignorados.

#### 8.3 Many-to-one: `group_left` / `group_right`

Na esquerda há **várias** séries por chave (uma por `code`), na direita **uma** (o total do serviço):

```promql
rate(sort_desc_http_requests_total[5m]) / on(service) sum by(service)(rate(sort_desc_http_requests_total[5m]))              # ❌ many-to-one must be explicit
rate(sort_desc_http_requests_total[5m]) / on(service) group_left sum by(service)(rate(sort_desc_http_requests_total[5m]))   # ✅ share de cada code no serviço
```

**Resultado esperado (o PROCV em ação):**

| service | code | esquerda (rate) | direita (total do service) | resultado |
|---|---|---|---|---|
| cart | 200 | 99 | 100 | **0.99** |
| cart | 500 | 1 | 100 | **0.01** |
| checkout | 200 | 95 | 100 | **0.95** |
| checkout | 500 | 5 | 100 | **0.05** |
| payments | 200 | 88 | 100 | **0.88** |
| payments | 500 | 12 | 100 | **0.12** |
| recommendations | 200/500 | 0 | 0 | **NaN** |

`group_left` = "o lado **esquerdo** é o que tem **mais** séries (many)". `group_right` é o espelho.

#### 8.4 `group_left(<labels>)`: copiar labels do lado "one" (enriquecer com metadados)

O uso nº 1 em produção: juntar uma métrica com uma **info metric** (valor sempre 1) para ganhar labels.

```promql
info_http_server_requests_total * on(job, instance) group_left(version) target_info                        # ganha version (1.4.0 ou 1.5.0)
info_http_server_requests_total * on(job, instance) group_left(version, k8s_cluster_name) target_info      # ganha 2 labels
target_info * on(job, instance) group_right(version) info_http_server_requests_total                       # mesmo resultado, lados trocados
```

**Resultado esperado:** `{route="/api/orders", version="1.4.0"}` e `{route="/api/users", version="1.4.0"}`,
com o **valor do counter** (× 1). O cenário alterna `version` entre `1.4.0` e `1.5.0` a cada 3 min
(simulando deploy/rollback): num gráfico, a série "troca de label" junto com o `target_info`.

**PROCV com chave que precisa ser construída.** A CPU vem com `endpoint="10.0.0.5:9100"`, o inventário com
`internal_ip="10.0.0.5", node="worker-1"`. Crie a chave com `label_replace` e faça o join:

```promql
label_replace(label_replace_node_cpu_usage_percent, "internal_ip", "$1", "endpoint", "(.*):.*") * on(internal_ip) group_left(node) label_replace_kube_node_info
```

**Resultado esperado:** 3 séries com `node="worker-1"`, `"worker-2"`, `"worker-3"` e o % de CPU.

#### 8.5 Many-to-many: sempre erro

```promql
sort_desc_http_requests_total / on(service) sort_desc_http_requests_total                                   # ❌ many-to-many matching not allowed
rate(sort_desc_http_requests_total[5m]) / on(service) group_left rate(sort_desc_http_requests_total{code="200"}[5m])   # ✅ restrinja um lado a 1 série por chave
```

O erro diz exatamente qual grupo duplicou: `found duplicate series for the match group {service="cart"} on the
right hand-side of the operation ... many-to-many matching not allowed: matching labels must be unique on one side`.
Conserto: filtrar/agregar um dos lados até ele ter **1 série por chave**.

#### 8.6 Resumo visual

```
            chave = labels (menos __name__)      modificadores
one-to-one   A{svc=a} ── B{svc=a}               on(...) / ignoring(...)
many-to-one  A{svc=a,code=200} ─┐
             A{svc=a,code=500} ─┴─ B{svc=a}     + group_left(<labels a copiar de B>)
one-to-many  A{svc=a} ─┬─ B{svc=a,code=200}
                       └─ B{svc=a,code=500}     + group_right(<labels a copiar de A>)
many-to-many                                    ❌ erro — agregue/filtre um lado
```

---

### 9. Operadores lógicos (`and`, `or`, `unless`)

Operam **só sobre os labels** (existência), **nunca** sobre valores. O valor sempre vem de quem sobrevive.

| Operador | Resultado | SQL |
|---|---|---|
| `a and b` | séries de **a** que têm par em **b** | semi-join (`WHERE EXISTS`) |
| `a or b` | tudo de **a** + séries de **b** sem par em **a** | `UNION` |
| `a unless b` | séries de **a** **sem** par em **b** | anti-join (`WHERE NOT EXISTS`) |

```promql
sort_cronjob_last_duration_seconds > 100 or sort_cronjob_last_duration_seconds < 10          # backup 120, cleanup 8
sort_cronjob_last_duration_seconds > 100 and sort_cronjob_last_duration_seconds < 10         # vazio
sort_by_label_app_build_info unless sort_by_label_app_build_info{version="1.10.0"}           # 6 pods
sort_desc_http_requests_total and on(service) rate(sort_desc_http_requests_total{code="500"}[5m]) > 3        # checkout e payments (200 E 500)
sort_desc_http_requests_total and rate(sort_desc_http_requests_total{code="500"}[5m]) > 3                    # só as séries 500 (sem on, code precisa bater)
sort_desc_http_requests_total unless on(service) rate(sort_desc_http_requests_total{code="500"}[5m]) > 0     # recommendations (serviço sem erro)
sum(rate(sort_desc_http_requests_total{service="search"}[5m])) or vector(0)                                 # 0 em vez de "No data"
sort_by_label_app_build_info{version="1.9.2"} or sort_cronjob_last_duration_seconds{cronjob="backup"}       # or mistura métricas diferentes
sort_cronjob_last_duration_seconds and on() (scalar_cpu_alert_threshold_percent > 0)                         # "interruptor": and on() com 1 série
```

- `and`/`unless` aceitam `on`/`ignoring` (e o **padrão** é casar com **todas** as combinações — não existe many-to-many error aqui).
- `group_left`/`group_right` **não** valem para operadores lógicos.
- `and on()` com uma série sem labels é um "**interruptor**" global: tudo passa se ela existir, nada passa se não.

---

### 10. Operadores de agregação

Resumem **muitas séries** em **menos séries**, **no mesmo instante** (não no tempo — isso é `*_over_time`).

```
<agregação> [by|without (<labels>)] ([parâmetro,] <instant vector>)
<agregação>([parâmetro,] <instant vector>) [by|without (<labels>)]     # também vale
```

| Agregador | Faz | Exemplo neste lab | Resultado |
|---|---|---|---|
| `sum` | soma | `sum(sort_by_label_desc_kafka_log_size_bytes) / 1024^3` | **78** |
| `avg` | média | `avg(...) / 1024^3` | **6.5** |
| `min` / `max` | menor / maior | `max(sort_cronjob_last_duration_seconds)` | **120** (ignora NaN) |
| `count` | nº de séries | `count(sort_by_label_desc_kafka_log_size_bytes)` | **12** |
| `group` | 1 por grupo | `group by(version)(sort_by_label_app_build_info)` | 4 séries = 1 |
| `stddev` / `stdvar` | desvio padrão / variância (populacional) | `stddev(...) / 1024^3` | **3.452** |
| `quantile(φ, v)` | φ-quantil **entre séries** | `quantile(0.9, ...) / 1024^3` | **10.9** |
| `topk(k, v)` / `bottomk(k, v)` | as k maiores/menores **séries originais** | `topk(3, ...)` | partições 11, 10, 9 |
| `count_values("l", v)` | conta por **valor** (vira label) | `count_values("cost", days_in_month_node_total_hourly_cost)` | `{cost="0.096"} 2`, `{cost="0.34"} 1` |
| `limitk(k, v)` 🧪 | k séries pseudo-aleatórias (determinístico) | `limitk(3, sort_by_label_node_load1)` | 3 séries |
| `limit_ratio(r, v)` 🧪 | fração r das séries (r negativo = complemento) | `limit_ratio(0.5, ...)` | ~6 séries |

🧪 = experimental, exige `--enable-feature=promql-experimental-functions` (ligado neste lab).

```promql
sum(sort_by_label_desc_kafka_log_size_bytes) / 1024^3                      # 78
avg(sort_by_label_desc_kafka_log_size_bytes) / 1024^3                      # 6.5
stddev(sort_by_label_desc_kafka_log_size_bytes) / 1024^3                   # 3.452 (= sqrt(143/12))
stdvar(sort_by_label_desc_kafka_log_size_bytes) / 1024^6                   # 11.9167
quantile(0.5, sort_by_label_desc_kafka_log_size_bytes) / 1024^3            # 6.5
quantile(0.9, sort_by_label_desc_kafka_log_size_bytes) / 1024^3            # 10.9 (interpolação: posição 0.9*11 = 9.9)
topk(3, sort_by_label_desc_kafka_log_size_bytes)                           # partições 11, 10, 9 (ordenado)
bottomk(2, sort_by_label_desc_kafka_log_size_bytes)                        # partições 0, 1
count by(version)(sort_by_label_app_build_info)                            # 1.10.0: 5, 1.10.12: 3, 1.9.2: 2, 1.2.0: 1
count(group by(version)(sort_by_label_app_build_info))                     # 4 versões distintas
count_values("cost", days_in_month_node_total_hourly_cost)                 # 0.096 -> 2, 0.34 -> 1
count_values("replicas", label_replace_kube_deployment_status_replicas_available)   # 1 -> 1, 2 -> 1, 3 -> 1
topk by(zone)(1, sort_by_label_node_load1)                                 # o nó mais carregado DE CADA zona (mantém node)
max by(zone)(sort_by_label_node_load1)                                     # o mesmo valor, mas PERDE o label node
limitk(3, sort_by_label_node_load1)                                        # 3 séries quaisquer
limit_ratio(0.5, sort_by_label_node_load1)                                 # ~metade
```

#### `by` vs `without`

```promql
sum by(zone)(sort_by_label_node_load1)                   # {zone} — só o que você listou
sum without(node)(sort_by_label_node_load1)              # {zone, job, instance, env} — tudo menos node
sum(sort_by_label_node_load1) by (zone)                  # cláusula depois: igual
sum by(namespace)(rate(sort_desc_container_cpu_usage_seconds_total[5m]))    # primeiro rate, DEPOIS sum
```

`without` é o preferido em recording rules: preserva `job`/`instance` e qualquer label que você não conhecia.

#### NaN nas agregações (cai na prova e acontece em produção)

```promql
sum(sort_cronjob_last_duration_seconds)                  # NaN  (120 + 8 + 45 + NaN)
avg(sort_cronjob_last_duration_seconds)                  # NaN
max(sort_cronjob_last_duration_seconds)                  # 120  (max/min ignoram NaN)
min(sort_cronjob_last_duration_seconds)                  # 8
bottomk(2, sort_cronjob_last_duration_seconds)           # cleanup 8, report 45 (NaN fica "mais longe")
quantile(0.5, sort_cronjob_last_duration_seconds)        # 26.5 — NaN conta como o MENOR valor
count(sort_cronjob_last_duration_seconds)                # 4
sum(sort_cronjob_last_duration_seconds == sort_cronjob_last_duration_seconds)   # 173 — filtre o NaN antes
```

#### A razão certa: somar primeiro, dividir depois

```promql
sum by(service)(rate(sort_desc_http_requests_total{code="500"}[5m])) / sum by(service)(rate(sort_desc_http_requests_total[5m]))   # 0.01, 0.05, 0.12, NaN
sum(rate(sort_desc_http_requests_total{code="500"}[5m])) / sum(rate(sort_desc_http_requests_total[5m]))                           # 0.06 global (18/300)
avg(sum by(service)(rate(sort_desc_http_requests_total{code="500"}[5m])) / sum by(service)(rate(sort_desc_http_requests_total[5m])))   # NaN: "média de médias"
sum by(code)(rate(sort_desc_http_requests_total[5m])) / ignoring(code) group_left sum(rate(sort_desc_http_requests_total[5m]))    # 200: 0.94, 500: 0.06
```

---

### 11. Precedência de operadores

Da **maior** para a **menor** (mesmo nível = esquerda para a direita, exceto `^`):

| Nível | Operadores | Associatividade |
|---|---|---|
| 1 | `^` | **direita** (`2^3^2 = 2^9 = 512`) |
| 2 | `*` `/` `%` `atan2` | esquerda |
| 3 | `+` `-` | esquerda |
| 4 | `==` `!=` `<=` `<` `>=` `>` | esquerda |
| 5 | `and` `unless` | esquerda |
| 6 | `or` | esquerda |

(O menos unário fica abaixo de `^`: `-2 ^ 2 = -4`.)

```promql
sort_cronjob_last_duration_seconds > 100 or sort_cronjob_last_duration_seconds < 50 and sort_cronjob_last_duration_seconds < 100     # backup, cleanup, report  (and primeiro!)
(sort_cronjob_last_duration_seconds > 100 or sort_cronjob_last_duration_seconds < 50) and sort_cronjob_last_duration_seconds < 100   # cleanup, report
sum by(service)(rate(sort_desc_http_requests_total{code="500"}[5m])) / sum by(service)(rate(sort_desc_http_requests_total[5m])) > 0.03   # / antes de >: checkout, payments
```

---

### 12. O que acontece com `__name__`

| Operação | Nome da métrica no resultado |
|---|---|
| seletor, `offset`, `@` | mantém |
| aritmética (`+ - * / % ^`), menos unário | **remove** |
| comparação **sem** `bool` | mantém o da **esquerda** (remove se usar `on(...)`; com `group_right`, mantém o da direita) |
| comparação **com** `bool` | **remove** |
| `and` / `unless` | mantém o da esquerda · `or`: cada série mantém o seu |
| `sum`, `avg`, `count`, ... | **remove** (só sobram os labels do `by`) |
| `topk`, `bottomk`, `limitk`, `limit_ratio` | mantém (devolvem as séries originais) |
| maioria das funções (`rate`, `abs`, `max_over_time`...) | **remove** (exceções: `last_over_time`, `first_over_time`, `sort*`, `label_*`...) |

```promql
sort_cronjob_last_duration_seconds{cronjob="backup"} + 0                   # sem nome
up{job="lab"} == 1                                                         # com nome "up"
up{job="lab"} == on(job) up                                                # sem nome (usou on)
last_over_time(sort_cronjob_last_duration_seconds{cronjob="backup"}[1m])   # com nome
{__name__=~"sort_node_filesystem_.+_bytes"} * 1                            # ❌ vector cannot contain metrics with the same labelset
max_over_time({__name__=~"sort_node_filesystem_.+_bytes"}[1m])             # ❌ idem: avail e size ficam iguais sem o nome
label_replace({__name__=~"sort_node_filesystem_.+_bytes"}, "metric", "$1", "__name__", "(.+)") * 1   # ✅ copie o nome para outro label antes
sum by(__name__)({__name__=~"sort_node_filesystem_.+_bytes"})              # ✅ agrupar por __name__ também preserva
```

O erro `vector cannot contain metrics with the same labelset` aparece quando **tirar o nome** deixa duas
séries com labels idênticos (ex.: `avail` e `size` do mesmo `node`/`mountpoint`).

---

## 🏭 Casos reais

### 1. SLO de erro por serviço (recording rule + alerta) — many-to-one e agregação

Time de plataforma do e-commerce: taxa de 5xx por serviço, gravada a cada 30 s, alerta se passar de 5%.

```yaml
groups:
  - name: http-slo
    rules:
      - record: service:http_requests:rate5m
        expr: sum without(instance, pod) (rate(http_requests_total[5m]))
      - record: service_code:http_requests:rate5m
        expr: sum by(service, code) (rate(http_requests_total[5m]))
      - record: service:http_errors:ratio_rate5m
        # soma primeiro, divide depois; os dois lados agregados pelo MESMO by -> one-to-one
        expr: |
          sum by(service) (rate(http_requests_total{code=~"5.."}[5m]))
            /
          sum by(service) (rate(http_requests_total[5m]))
      - alert: HighErrorRate
        expr: service:http_errors:ratio_rate5m > 0.05
        for: 5m
        labels:
          severity: page
        annotations:
          summary: "{{ $labels.service }} com {{ $value | humanizePercentage }} de erros"
```

No lab: `sum by(service)(rate(sort_desc_http_requests_total{code="500"}[5m])) / sum by(service)(rate(sort_desc_http_requests_total[5m])) > 0.05` → só `payments` (0.12).
(checkout está em **exatamente** 0.05 e `> 0.05` é estrito.)

### 2. `KubeDeploymentReplicasMismatch` (kube-prometheus) — comparação one-to-one + `and`

```yaml
groups:
  - name: kubernetes-apps
    rules:
      - alert: KubeDeploymentReplicasMismatch
        expr: |
          (
            kube_deployment_spec_replicas{job="kube-state-metrics"}
              >
            kube_deployment_status_replicas_available{job="kube-state-metrics"}
          ) and (
            changes(kube_deployment_status_replicas_updated{job="kube-state-metrics"}[10m]) == 0
          )
        for: 15m
        labels:
          severity: warning
```

Comparação vetor/vetor com os **mesmos labels** (one-to-one) filtra e devolve o valor da esquerda; o
`and` só deixa passar se, **para os mesmos labels**, o rollout está parado. No lab:
`ceil_kube_deployment_spec_replicas > label_replace_kube_deployment_status_replicas_available` → checkout 5.

### 3. Enriquecer métricas com info metrics — `group_left(label)`

node_exporter expõe `node_uname_info{nodename="ip-10-0-0-5", instance="10.0.0.5:9100"} 1`. Para o alerta
de CPU mostrar o **nome** do host (e o kube-state-metrics dar o `node` de cada pod):

```yaml
groups:
  - name: node
    rules:
      - record: instance:node_cpu_utilisation:rate5m
        expr: |
          1 - avg without(cpu, mode) (rate(node_cpu_seconds_total{mode="idle"}[5m]))
      - alert: NodeHighCPU
        expr: |
          instance:node_cpu_utilisation:rate5m
            * on(instance) group_left(nodename) node_uname_info
          > 0.9
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "CPU alta em {{ $labels.nodename }}"
      - record: namespace_pod:container_cpu_usage:rate5m_with_node
        expr: |
          sum by(namespace, pod) (rate(container_cpu_usage_seconds_total{container!=""}[5m]))
            * on(namespace, pod) group_left(node) kube_pod_info
```

`* on(...) group_left(label) info_metric` é o padrão "**PROCV**": como a info vale 1, o valor não
muda — só **ganha labels**. No lab: `info_http_server_requests_total * on(job, instance) group_left(version) target_info`.

### 4. Silenciar alerta fora do horário / em manutenção — `and on()` e `unless`

```yaml
groups:
  - name: batch
    rules:
      - alert: BatchQueueBacklog
        # só em horário comercial (UTC 12h-21h): "interruptor" and on() com um vetor sem labels
        expr: |
          sum by(queue) (rabbitmq_queue_messages_ready) > 1000
            and on() (hour() >= 12 and hour() < 21)
        for: 10m
      - alert: TargetDown
        # derruba o alerta dos jobs marcados como "em manutenção" por uma métrica de controle
        expr: |
          up == 0
            unless on(job) maintenance_mode == 1
        for: 5m
```

`hour()` retorna um vetor **sem labels** — por isso o `on()` (chave vazia) casa com todas as séries da esquerda.

---

## 🧪 Exercícios

| # | Exercício | Desafios |
|---|---|---|
| 01 | [Seletores e regex ancorada](exercises/01-seletores-regex/) | 201, 202, 203 |
| 02 | [offset e subquery](exercises/02-offset-subquery/) | 208, 211, 212 |
| 03 | [Filtro vs `bool` (e o NaN)](exercises/03-bool-e-nan/) | 219, 220, 221 |
| 04 | [PROCV: `group_left(label)`](exercises/04-procv-group-left/) | 234, 236 |
| 05 | [many-to-one e many-to-many](exercises/05-many-to-many/) | 233, 237 |
| 06 | [Agregações sem armadilhas](exercises/06-agregacoes/) | 245, 248, 249 |

E mais **54 desafios** auto-corrigidos: `cd ../../challenges && ./check.py list --topic operators`.

---

## ⚠️ Pegadinhas

1. **Regex é ancorada.** `=~"api"` ≠ "contém api". Use `.*api.*`.
2. **Label vazio = label ausente.** `{region=""}` seleciona quem **não tem** `region`.
3. **`offset`/`@` colados no seletor**, nunca depois de função/agregação (`sum(x offset 5m)`).
4. **Escalar vs escalar precisa de `bool`** (`1 > bool 2`); sem `bool`, é erro de parse.
5. **Comparação sem `bool` filtra**, com `bool` devolve 0/1 e **perde o nome**.
6. **NaN:** `NaN == NaN` é falso, `NaN != x` é verdadeiro, `sum`/`avg` viram NaN, `max`/`min`/`topk` ignoram, `quantile` trata como o menor valor.
7. **Vector matching é INNER JOIN:** séries sem par somem **sem erro**. Resultado vazio? Compare os labels dos dois lados.
8. **`on()` em one-to-one descarta os outros labels**; `ignoring()` mantém.
9. **many-to-one sem `group_left` = erro**; **many-to-many = sempre erro**.
10. `group_left`/`group_right` **não** valem para `and`/`or`/`unless`.
11. **`and`/`or`/`unless` olham só labels**, nunca valores. `a and b` devolve o valor de **a**.
12. **`^` é associativo à direita** e `and` tem precedência **maior** que `or`.
13. **Média de médias:** some numerador e denominador separadamente e só então divida.
14. `topk` numa **range query** é recalculado a cada ponto (a legenda tem mais que k séries) — use `@ end()`.
15. `sum(x) or vector(0)` só funciona sem `by`; com `by(service)` o `vector(0)` (sem labels) vira uma série extra.

---

## 🎓 Na prova PCA

O domínio PromQL (28%) cobra muito mais **operadores e matching** do que funções exóticas. Espere:
tipos de dados, seletores/regex, `offset`, `bool`, `on`/`ignoring`/`group_left`, `by`/`without`,
`topk`, `count_values`, precedência e "qual o resultado desta query".

**1.** Which selector returns series whose `pod` label **starts with** `api-`?

A) `x{pod=~"api-"}`  B) `x{pod="api-.*"}`  C) `x{pod=~"api-.*"}`  D) `x{pod=~"^api-"}`

<details><summary>Resposta</summary>

**C.** Regex do PromQL é **totalmente ancorada**: A e D viram `^api-$` (só casa com o texto exato
`api-`). B usa `=`, que compara a string literal `api-.*`.
</details>

**2.** `errors_total{code="500", method="get"}` and `requests_total{method="get"}` exist. What does
`errors_total / requests_total` return?

A) The error ratio per method  B) An empty result  C) An error: many-to-one matching must be explicit  D) `NaN`

<details><summary>Resposta</summary>

**B.** Sem modificador, a chave de casamento é o conjunto **inteiro** de labels; `{code, method}` ≠
`{method}`, então nada casa e o resultado é **vazio** (sem erro). O certo é `/ ignoring(code)` (ou
`on(method)`). Se houvesse **vários** `code` por método, aí seria preciso `ignoring(code) group_left`.
</details>

**3.** What does `node_memory_bytes > bool 1e9` return for a series whose value is `5e8`?

A) The series is dropped  B) `0`, keeping the metric name  C) `0`, without the metric name  D) `5e8`

<details><summary>Resposta</summary>

**C.** Com `bool`, a comparação não filtra: devolve `0`/`1` e **descarta o `__name__`**. Sem `bool`, a série seria descartada (A).
</details>

**4.** Which expression is **invalid**?

A) `sum(rate(x[5m] offset 1h))`  B) `sum(rate(x[5m])) offset 1h`  C) `rate(x[5m] @ end())`  D) `x offset 5m @ 1609746000`

<details><summary>Resposta</summary>

**B.** `offset` (e `@`) precisam vir **logo depois de um seletor** (ou subquery), não depois de uma
agregação. D é válida: a ordem entre `@` e `offset` é livre e o offset é relativo ao `@`.
</details>

**5.** `method_code:http_errors:rate5m` has labels `{method, code}` and `method:http_requests:rate5m`
has `{method}`. Which query returns, **for each method and code**, the fraction of requests?

A) `method_code:http_errors:rate5m / on(method) method:http_requests:rate5m`
B) `method_code:http_errors:rate5m / ignoring(code) group_left method:http_requests:rate5m`
C) `method_code:http_errors:rate5m / ignoring(code) group_right method:http_requests:rate5m`
D) `method_code:http_errors:rate5m and ignoring(code) method:http_requests:rate5m`

<details><summary>Resposta</summary>

**B.** Várias séries por `method` à **esquerda** (uma por `code`) e uma à direita → many-to-one com
`group_left`. A dá erro (many-to-one precisa ser explícito), C inverte o lado "many", D é operador
lógico (não divide nada; devolve os valores da esquerda). É o exemplo da documentação oficial.
</details>

**6.** What is the result of `2 ^ 3 ^ 2` in PromQL?

A) 64  B) 512  C) 36  D) Parse error

<details><summary>Resposta</summary>

**B.** `^` é **associativo à direita**: `2 ^ (3 ^ 2)` = 2⁹ = 512. Todos os outros operadores binários são associativos à esquerda.
</details>

**7.** You need the total memory per `application` **keeping every other label** except `instance`. Which is correct?

A) `sum by(application)(mem)`  B) `sum without(instance)(mem)`  C) `sum(mem) by (instance)`  D) `max without(application)(mem)`

<details><summary>Resposta</summary>

**B.** `without` remove só os labels listados e preserva todos os outros; `by` mantém **só** os listados.
</details>

**8.** `build_version` is a gauge whose **value** is the build number. Which counts how many instances run each build?

A) `count by(build_version)(build_version)`  B) `count_values("version", build_version)`  C) `sum(build_version)`  D) `group(build_version)`

<details><summary>Resposta</summary>

**B.** `count_values` cria uma série por **valor distinto**, com esse valor no label indicado, contando as ocorrências.
</details>

**9.** Which statement about `topk(3, x)` is **true**?

A) It returns a single series with the 3rd largest value
B) It returns the 3 largest series with all their original labels
C) With `by (job)` it returns only the `job` label
D) In a range query it always returns exactly 3 series in the legend

<details><summary>Resposta</summary>

**B.** `topk`/`bottomk` devolvem as **séries originais**. O `by` só separa os grupos (C é falsa). Numa
range query o top 3 é recalculado em **cada** ponto, então a legenda pode ter mais de 3 séries (D é falsa).
</details>

**10.** Which is the lowest-precedence binary operator?

A) `and`  B) `unless`  C) `or`  D) `==`

<details><summary>Resposta</summary>

**C.** A ordem é `^` > `* / % atan2` > `+ -` > comparações > `and unless` > `or`.
</details>

---

## 📝 Cola rápida

- **Tipos:** instant vector · range vector (`[5m]`, não vai para gráfico) · scalar · string. Range query = instant query repetida por `step`.
- **Matchers:** `=` `!=` `=~` `!~`; regex RE2 **ancorada**; `{l=""}` = label ausente; precisa de 1 matcher que não case com vazio.
- **Tempo:** `x offset 1h` · `x @ 1609746000` · `@ start()`/`@ end()` · subquery `expr[30m:1m]` (sem resolução = `evaluation_interval`).
- **Comparação:** filtra (valor da esquerda) · `bool` → 0/1 sem nome · escalar/escalar exige `bool`.
- **Matching:** chave = todos os labels − `__name__` · `on(l)` / `ignoring(l)` · many-to-one → `group_left(labels a copiar)` · many-to-many → erro · INNER JOIN.
- **Lógicos:** `and` (semi-join) · `or` (união) · `unless` (anti-join) — só labels, nunca valores.
- **Agregações:** `sum avg min max count group stddev stdvar quantile(φ) topk(k) bottomk(k) count_values("l") limitk🧪 limit_ratio🧪` + `by`/`without`, antes ou depois.
- **NaN:** `sum`/`avg` → NaN · `max`/`min`/`topk` ignoram · `quantile` = menor · `x == x` remove NaN.

| Precedência (maior → menor) | |
|---|---|
| 1 | `^` (direita) |
| 2 | `*` `/` `%` `atan2` |
| 3 | `+` `-` |
| 4 | `==` `!=` `<=` `<` `>=` `>` |
| 5 | `and` `unless` |
| 6 | `or` |

---

## 🔗 Relacionadas

- Funções usadas junto: [`rate()`](../../promql-functions-lab/functions/rate/), [`label_replace()`](../../promql-functions-lab/functions/label_replace/), [`scalar()`](../../promql-functions-lab/functions/scalar/), [`vector()`](../../promql-functions-lab/functions/vector/), [`info()`](../../promql-functions-lab/functions/info/), [`*_over_time`](../../promql-functions-lab/functions/max_over_time/)
- Desafios: `../../challenges/operators/` (ids 200–253)

## 📚 Referências

- Operators (v3.15.0): https://prometheus.io/docs/prometheus/latest/querying/operators/ — fonte: https://github.com/prometheus/prometheus/blob/v3.15.0/docs/querying/operators.md
- Querying basics (tipos, seletores, offset, @, subquery): https://prometheus.io/docs/prometheus/latest/querying/basics/
- Novidades da v3.x fora do escopo da PCA, citadas só para você reconhecer: operadores de *trim* de native histograms `</` e `>/`, e os modificadores experimentais `fill()`/`fill_left()`/`fill_right()` (flag `promql-binop-fill-modifiers`, **desligada** neste lab).
- kube-prometheus / kubernetes-mixin (alertas reais): https://github.com/prometheus-operator/kube-prometheus
- "Left joins in PromQL" (Robust Perception): https://www.robustperception.io/left-joins-in-promql
