# Recording rules + testes unitários com `promtool`

> **Em uma frase:** recording rules **pré-calculam** queries caras e gravam o resultado como uma série nova; `promtool test rules` prova, **sem subir nada**, que suas recording rules e alertas fazem o que você acha que fazem.

| | |
|---|---|
| **Stack** | nenhuma: só `promtool` via Docker (`prom/prometheus:v3.15.0`) |
| **Portas** | nenhuma |
| **Exemplo completo** | [`examples/http.rules.yml`](examples/http.rules.yml) + [`examples/http.test.yml`](examples/http.test.yml) |
| **CI** | [`ci/prometheus-rules.yml`](ci/prometheus-rules.yml) (GitHub Actions) |
| **Teste automático** | [`./test.sh`](test.sh) (~15 s) |

---

## 🧠 Analogia: o fechamento do caixa e o simulador de voo

**Recording rule = fechamento do caixa.** O dono da padaria não soma todos os cupons fiscais do mês toda vez que alguém pergunta "quanto vendemos hoje?". No fim de cada dia ele fecha o caixa e anota **um número** no caderno. Consultar o caderno é instantâneo; o trabalho pesado foi feito uma vez, no horário certo. O Prometheus faz o mesmo: a cada `interval`, roda a `expr` e grava o resultado com um nome novo (`job:http_requests:rate5m`). Dashboards e alertas leem o "caderno".

**`promtool test rules` = simulador de voo.** Piloto não aprende a lidar com pane de motor num avião de verdade. No simulador você **inventa** o cenário ("contador sobe 60/min, reinicia aos 6 minutos, some aos 10") e confere o que o painel mostra em cada instante. `input_series` é o cenário, `eval_time` é o instante, `exp_samples`/`exp_alerts` é o que o painel **deveria** mostrar.

---

## 🏗️ Arquitetura

```
                    a cada interval do GRUPO
 ┌───────────┐   ┌──────────────────────────────────────────────────┐
 │   TSDB    │◄──┤ rule group "http-recording" (sequencial)         │
 │           │   │   1. record job:http_requests:rate5m        ─────┼──► grava série nova
 │ séries    │──►│   2. record job:http_requests_errors:rate5m ─────┼──► grava série nova
 │ cruas     │   │   3. record ..._errors:ratio_rate5m  (usa 1 e 2) ┼──► grava série nova
 └───────────┘   └──────────────────────────────────────────────────┘
       ▲         ┌──────────────────────────────────────────────────┐
       └─────────┤ rule group "http-alerts" (em paralelo com o outro)│
                 │   alert HighErrorRatio: expr sobre a série 3     ├──► ALERTS / Alertmanager
                 └──────────────────────────────────────────────────┘

 promtool test rules:
   input_series (cenário sintético) ─► TSDB em memória ─► avalia rule_files a cada evaluation_interval
                                                        ─► compara em eval_time com exp_samples / exp_alerts
```

- Regras **dentro de um grupo** rodam **em sequência**, na ordem do arquivo: a 3ª enxerga o que a 1ª e a 2ª acabaram de gravar.
- **Grupos diferentes** rodam **em paralelo**, cada um no seu `interval`.

---

## ▶️ Como rodar

Nada para subir. Um alias ajuda:

```bash
cd labs/recording-rules-testing
alias promtool='docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0'

promtool check rules examples/http.rules.yml
#   SUCCESS: 4 rules found
promtool test rules examples/http.test.yml
#   SUCCESS
./test.sh
#   PASS labs/recording-rules-testing (examples + 8 exercícios: soluções passam, iniciais falham)
```

> O alias monta o diretório atual em `/w`. Os caminhos passados ao promtool precisam estar **dentro** dele, e `rule_files` num `.test.yml` é **relativo ao arquivo de teste**.

---

## 🔍 Passo a passo

### 1. Quando usar recording rules

| Use quando... | Exemplo |
|---|---|
| A query é **cara** e roda o tempo todo (dashboard aberto numa TV, 30 painéis) | `sum by (job) (rate(http_requests_total[5m]))` sobre 50 mil séries |
| O **alerta** reaproveita um cálculo | SLO: `job:http_requests_errors:ratio_rate5m > 0.05` |
| Você precisa agregar **antes** de federar ou mandar via remote write | Prometheus global só puxa `job:*` |
| Consulta de **longo prazo** (30 dias de `rate`) é pesada | pré-agregar reduz cardinalidade |

Quando **não** usar: query rara/ad hoc, ou quando a regra teria a mesma cardinalidade da métrica original (não economiza nada).

### 2. Convenção de nome: `nível:métrica:operações`

| Parte | Significado | Exemplos |
|---|---|---|
| **nível** | labels que **sobram** após a agregação | `job`, `instance`, `job_code`, `cluster_namespace` |
| **métrica** | nome original; **tira `_total`** ao aplicar `rate`/`increase` | `http_requests`, `node_cpu_seconds` |
| **operações** | o que foi aplicado, **mais recente primeiro** | `rate5m`, `ratio_rate5m`, `sum_rate5m`, `avg_rate1m` |

```yaml
# sum by (job) (rate(http_requests_total[5m]))
- record: job:http_requests:rate5m
# sum by (job, code) (...)
- record: job_code:http_requests:rate5m
# razão de duas séries com o mesmo nível
- record: job:http_requests_errors:ratio_rate5m
# sem agregação nenhuma (mantém todos os labels do alvo)
- record: instance:node_cpu_utilisation:rate5m
```

Por que importa: o nível **documenta os labels**. Se você divide `job:a:rate5m` por `job:b:rate5m`, sabe que casa. Se um deles é `job_code:`, sabe que precisa de `on(job)`/`group_left` (exercício [07](exercises/07-razao-vector-matching/)).

### 3. Grupos e intervalo de avaliação

```yaml
groups:
  - name: http-recording
    interval: 1m          # padrão: global.evaluation_interval (1m no Prometheus)
    limit: 1000           # opcional: > 1000 séries resultantes => a avaliação do grupo FALHA
    query_offset: 30s     # opcional: avalia "30s no passado" (dados atrasados via remote write)
    labels:               # opcional: labels adicionados a TODAS as regras do grupo
      team: sre
    rules:
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
        labels:            # labels extras só desta regra
          source: recording
```

Todos esses campos foram validados com `promtool check rules` no v3.15.0. Recording rule aceita só `record`, `expr`, `labels`; `for`, `keep_firing_for` e `annotations` são exclusivos de **alerta**.

### 4. `promtool check rules`

```bash
promtool check rules examples/http.rules.yml
promtool check rules --lint-fatal regras/*.yml          # duplicadas => exit 3
promtool check config prometheus.yml                    # também checa os rule_files referenciados
```

O que pega: YAML inválido, PromQL que não parseia, campo inválido (`for` em recording rule), template de annotation que não compila, **regras duplicadas** (lint).

⚠️ Sem `--lint-fatal`, uma regra duplicada imprime `FAILED: lint error 1 duplicate rule(s) found` mas o comando **sai com 0** (testado). O que ele **não** pega: nome fora da convenção (no Prometheus 3 nomes UTF-8 como `job:http-requests:rate5m` são válidos). Para isso o lab traz [`lint-names.sh`](lint-names.sh).

### 5. `promtool test rules`: anatomia do arquivo de teste

```yaml
rule_files:
  - http.rules.yml            # relativo a este arquivo
evaluation_interval: 1m       # frequência de avaliação das regras no teste (padrão 1m)
# group_eval_order: [g1, g2]  # opcional
tests:
  - name: taxa e razão de erro do job api
    interval: 1m              # espaçamento das amostras de input_series (padrão = evaluation_interval)
    external_labels: { cluster: lab }   # opcional: vira $externalLabels nos templates
    input_series:
      - series: 'http_requests_total{job="api", instance="a", code="200"}'
        values: '0+240x30'
    promql_expr_test:         # "o que esta query devolve no instante X?"
      - expr: job:http_requests:rate5m
        eval_time: 10m
        exp_samples:
          - labels: 'job:http_requests:rate5m{job="api"}'
            value: 10
    alert_rule_test:          # "quais alertas estão FIRING no instante X?"
      - eval_time: 10m
        alertname: HighErrorRatio
        exp_alerts:
          - exp_labels: { severity: page, job: api }
            exp_annotations: { summary: "api com 10% de erros" }
```

Notação de `values` (todas testadas):

| Notação | Expande para | Uso |
|---|---|---|
| `'0+60x5'` | `0 60 120 180 240 300` (**n+1** amostras) | counter subindo |
| `'10-2x3'` | `10 8 6 4` | gauge descendo |
| `'5x3'` | `5 5 5 5` | constante |
| `'1 2 3'` | literal | valores à mão |
| `'1 _ 3'` | `_` = **sem amostra** naquele passo | buraco no scrape |
| `'_x3'` | 3 passos sem amostra | série sumiu |
| `'1 1 stale'` | **staleness marker** | alvo saiu do scrape: série some na hora |
| `'0+60x5 60+60x14'` | sobe, **cai** (reset), sobe de novo | counter reset |

Regras de ouro:
- `exp_alerts` lista **apenas alertas firing**. Pending = `exp_alerts: []`; para ver pending use `promql_expr_test` sobre `ALERTS{alertstate="pending"}`.
- `exp_labels` precisa de **todos** os labels do alerta (exceto `alertname`), inclusive os que vêm da série.
- `exp_samples: []` testa que a query devolve **vazio**.
- A comparação de `value` só tolera erro de arredondamento de float: esperar `0.3333333333` para um `rate` de `1/3` **falha** (testado). Monte cenários com números redondos (`0+60x10` → 1/s).

### 6. Lendo uma falha

```
  FAILED:
    expr: "job:jobs_processed:rate5m", time: 8m,
        exp: {__name__="job:jobs_processed:rate5m", job="worker"} 2E+00
        got: {__name__="job:jobs_processed:rate5m", job="worker"} 3.25E+00
```

`exp` = o que você esperava, `got` = o que a regra produziu. `got: nil` = a série nem existe (nome errado, labels não casam, vetor vazio). Para ver **tudo** que existe no TSDB do teste:

```bash
promtool test rules --debug examples/http.test.yml | head -20
# DEBUG: Dump of all data (input_series and rules) at 10m1s:
# {__name__="ALERTS", alertname="HighErrorRatio", alertstate="pending", ...} => 1 @[60000] ...
# {__name__="ALERTS_FOR_STATE", alertname="HighErrorRatio", ...} => 60 @[60000] ...
```

Outras flags úteis: `--run <regex>` (só os testes cujo `name` casa), `--junit=arquivo.xml` (relatório para CI), `--diff` (experimental).

### 7. Raciocinando como `promtool query instant`

Um `promql_expr_test` é literalmente uma **instant query** no instante `eval_time`. Contra um Prometheus de verdade, a mesma pergunta é:

```bash
# (usando o Prometheus do promql-functions-lab, se estiver no ar)
docker run --rm --network host --entrypoint promtool prom/prometheus:v3.15.0 \
  query instant http://localhost:9095 'count(up)'
# {} => 2 @[1790433336.756]
docker run --rm --network host --entrypoint promtool prom/prometheus:v3.15.0 \
  query instant --time=2026-09-26T12:00:00Z http://localhost:9095 'count(up)'
```

Faça a conta **antes** de rodar, como no exemplo: `'0+240x30'` com `interval: 1m` → 240 por minuto → `rate` = **4/s**. Somando `a` (4 + 1) e `b` (5) → `job:http_requests:rate5m{job="api"} = 10`. Erros 1/s ÷ 10/s = **0.1**. Linha do tempo do alerta (`for: 5m`), confirmada no `--debug`: condição verdadeira desde **1m** (primeiro instante com 2 amostras para o `rate`), pending de 1m a 5m, **firing a partir de 6m**.

### 8. CI: não deixe regra quebrada chegar em produção

[`ci/prometheus-rules.yml`](ci/prometheus-rules.yml) é um workflow do GitHub Actions pronto:

```yaml
env:
  PROMTOOL: docker run --rm -u 1001 -v ${{ github.workspace }}:/w -w /w --entrypoint promtool prom/prometheus:v3.15.0
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: $PROMTOOL check rules --lint-fatal rules/*.rules.yml
      - run: ./lint-names.sh rules/*.rules.yml
      - run: $PROMTOOL test rules --junit=promtool-junit.xml rules/tests/*.test.yml
      - run: docker run --rm -v $PWD:/w -w /w --entrypoint amtool prom/alertmanager:v0.34.1 check-config alertmanager/alertmanager.yml
```

O `./test.sh` deste lab é o mesmo raciocínio: **soluções têm que passar e os arquivos quebrados têm que falhar** (se o teste não falha com a regra errada, ele não testa nada).

---

## 🏭 Casos reais

### Caso 1: SLO multi-janela (padrão Google SRE / Sloth / Pyrra)

```yaml
groups:
  - name: slo-checkout-recording
    rules:
      - record: job:slo_errors_per_request:ratio_rate5m
        expr: |
          sum by (job) (rate(http_requests_total{job="checkout", code=~"5.."}[5m]))
          /
          sum by (job) (rate(http_requests_total{job="checkout"}[5m]))
      - record: job:slo_errors_per_request:ratio_rate1h
        expr: |
          sum by (job) (rate(http_requests_total{job="checkout", code=~"5.."}[1h]))
          /
          sum by (job) (rate(http_requests_total{job="checkout"}[1h]))
  - name: slo-checkout-alerts
    rules:
      - alert: CheckoutErrorBudgetBurn
        # 14.4x o orçamento de 99.9% = queima 2% do mês em 1h
        expr: |
          job:slo_errors_per_request:ratio_rate1h > (14.4 * 0.001)
          and
          job:slo_errors_per_request:ratio_rate5m > (14.4 * 0.001)
        labels: { severity: page }
```

Sem recording rules, esse alerta reavaliaria `rate(...[1h])` sobre todas as séries a cada ciclo.

### Caso 2: node-mixin / kube-prometheus

```yaml
- record: instance:node_cpu_utilisation:rate5m
  expr: 1 - avg without (cpu) (sum without (mode) (rate(node_cpu_seconds_total{job="node", mode=~"idle|iowait|steal"}[5m])))
- record: instance:node_memory_utilisation:ratio
  expr: 1 - (node_memory_MemAvailable_bytes{job="node"} / node_memory_MemTotal_bytes{job="node"})
- record: namespace_workload_pod:kube_pod_owner:relabel      # nível composto: namespace + workload + pod
  expr: max by (cluster, namespace, workload, pod) (...)
```

Os mixins do ecossistema publicam suas regras **junto com testes** (`tests.yaml` rodado com `promtool test rules` no CI do repositório).

### Caso 3: teste de alerta real (estilo kubernetes-mixin)

```yaml
tests:
  - interval: 1m
    input_series:
      - series: 'kube_pod_container_status_waiting_reason{namespace="ns1", pod="pod-1", container="c", reason="CrashLoopBackOff", job="kube-state-metrics"}'
        values: '1x20'
    alert_rule_test:
      - eval_time: 16m
        alertname: KubePodCrashLooping
        exp_alerts:
          - exp_labels: { severity: warning, namespace: ns1, pod: pod-1, container: c, reason: CrashLoopBackOff, job: kube-state-metrics }
            exp_annotations:
              description: 'Pod ns1/pod-1 (c) is in waiting state (reason: "CrashLoopBackOff").'
```

### Caso 4: federação só de agregados

```yaml
# Prometheus global: puxa só as recording rules dos Prometheus de cada cluster
scrape_configs:
  - job_name: federate
    honor_labels: true
    metrics_path: /federate
    params:
      "match[]": ['{__name__=~"job:.*|instance:.*"}']
```

A convenção de nomes vira um **filtro**: tudo que começa com `nível:` é agregado e barato de federar.

---

## 🧪 Exercícios

| # | Exercício | Tipo | Tema |
|---|---|---|---|
| 01 | [nomear-recording-rules](exercises/01-nomear-recording-rules/) | complete | `nível:métrica:operações`, `sum by` |
| 02 | [conserte-o-teste](exercises/02-conserte-o-teste/) | conserte o teste | unidade do `rate`, labels após agregação |
| 03 | [for-pending-firing](exercises/03-for-pending-firing/) | conserte + complete | `alert_rule_test`, `for`, `ALERTS` |
| 04 | [counter-reset](exercises/04-counter-reset/) | conserte a regra | reset, `resets()`, rate antes de sum |
| 05 | [templates-annotations](exercises/05-templates-annotations/) | conserte a regra | `exp_annotations`, `humanize*`, `query` |
| 06 | [absent-serie-sumiu](exercises/06-absent-serie-sumiu/) | conserte a regra | `absent()`, `_`, `stale`, lookback |
| 07 | [razao-vector-matching](exercises/07-razao-vector-matching/) | conserte a regra | nível do nome × `by()`, `got: nil` |
| 08 | [check-rules-lint](exercises/08-check-rules-lint/) | conserte | `check rules`, `--lint-fatal`, nomes |

---

## ⚠️ Pegadinhas

1. **`exp_alerts` só vê firing.** Pending testa-se com `ALERTS{alertstate="pending"}`.
2. **`'0+60x5'` são 6 amostras**, não 5.
3. **`rule_files` é relativo ao arquivo de teste**, não ao diretório atual.
4. **Recording rule não tem `for`** nem `annotations`.
5. **`check rules` sem `--lint-fatal`** deixa regra duplicada passar com exit 0.
6. **Nome fora da convenção passa no promtool** (UTF-8 no Prometheus 3).
7. **`sum` antes de `rate`** quebra na hora do reset (exercício 04).
8. **Regras do mesmo grupo** são sequenciais; entre grupos **não há ordem garantida**. Se a regra B usa o resultado de A, coloque as duas no mesmo grupo, A antes.
9. **`_` vs `stale`**: sem amostra a série ainda "existe" por 5m (lookback delta); com staleness marker ela some na hora.
10. `evaluation_interval` do teste ≠ `interval` das amostras: um controla a avaliação das regras, o outro o espaçamento dos dados.

---

## 🎓 Na prova PCA

**1.** Which recording rule name follows the Prometheus naming convention for `sum by (job) (rate(http_requests_total[5m]))`?
- A) `http_requests_total:rate5m:job`
- B) `job:http_requests:rate5m`
- C) `job:http_requests_total:sum`
- D) `rate5m:http_requests:job`

<details><summary>Resposta</summary>

**B.** `nível:métrica:operações`: o nível é `job` (label que sobrou), a métrica perde o `_total` depois do `rate`, a operação é `rate5m`.
</details>

**2.** What is the main purpose of recording rules?
- A) To send alerts to Alertmanager
- B) To precompute frequently used or expensive expressions and save the result as a new time series
- C) To delete old time series
- D) To relabel targets before scraping

<details><summary>Resposta</summary>

**B.** Recording rules pré-calculam e gravam o resultado como série nova, deixando dashboards e alertas rápidos.
</details>

**3.** Which command runs unit tests for alerting and recording rules?
- A) `promtool check rules tests.yml`
- B) `promtool test rules tests.yml`
- C) `amtool check-config tests.yml`
- D) `prometheus --test tests.yml`

<details><summary>Resposta</summary>

**B.** `check rules` só valida sintaxe/PromQL; `test rules` executa os cenários (`input_series`) e compara com `exp_samples`/`exp_alerts`.
</details>

**4.** In a `promtool` test, what does the input series notation `'1+2x3'` expand to?
- A) `1 2 3`  B) `1 3 5 7`  C) `1 3 5`  D) `3 3 3`

<details><summary>Resposta</summary>

**B.** Começa em 1 e soma 2, **3 vezes**: 4 amostras (`n+1`).
</details>

**5.** An alert has `for: 5m`. In an `alert_rule_test`, you evaluate at a time when the alert is still pending. What should `exp_alerts` be?
- A) The alert with `alertstate: pending`
- B) An empty list
- C) The alert with its annotations
- D) The test cannot evaluate pending alerts

<details><summary>Resposta</summary>

**B.** `exp_alerts` contém só alertas **firing**. Pending aparece apenas na série `ALERTS{alertstate="pending"}`, que pode ser checada com `promql_expr_test`.
</details>

**6.** How are rules within the same rule group evaluated?
- A) In parallel, in random order
- B) Sequentially, in the order they are defined
- C) Alphabetically by name
- D) Only when queried

<details><summary>Resposta</summary>

**B.** Dentro do grupo: **sequencial**, na ordem do arquivo, então uma regra pode usar o resultado da anterior no mesmo ciclo. Grupos diferentes rodam em paralelo.
</details>

**7.** Which field is valid in an alerting rule but **not** in a recording rule?
- A) `expr`  B) `labels`  C) `for`  D) `record`

<details><summary>Resposta</summary>

**C.** `for` (e `keep_firing_for`, `annotations`) são exclusivos de alerta. `promtool check rules` rejeita com `invalid field 'for' in recording rule`.
</details>

---

## 📝 Cola rápida

- Nome: `nível:métrica:operações` · tira `_total` após `rate` · nível = labels que sobram.
- Grupo: `interval`, `limit`, `query_offset`, `labels`; regras do grupo em sequência, grupos em paralelo.
- Recording: `record` + `expr` (+ `labels`). Alerta: `alert`, `expr`, `for`, `keep_firing_for`, `labels`, `annotations`.
- `promtool check rules [--lint-fatal] arq.yml` · `promtool test rules [--debug] [--run re] [--junit f.xml] t.yml`.
- Teste: `rule_files`, `evaluation_interval`, `tests[].interval`, `input_series`, `promql_expr_test` (`expr`, `eval_time`, `exp_samples`), `alert_rule_test` (`eval_time`, `alertname`, `exp_alerts` com `exp_labels`/`exp_annotations`).
- Valores: `a+bxn` (n+1 amostras) · `a-bxn` · `axn` · `_` · `stale`.
- `exp_alerts` = só firing. `exp_samples: []` = vazio. `got: nil` = série não existe.
- CI: `check rules --lint-fatal` + `test rules --junit` + `amtool check-config` + `amtool config routes test --verify.receivers`.

## 📚 Referências

- https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/
- https://prometheus.io/docs/practices/rules/
- https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/
- https://prometheus.io/docs/prometheus/latest/command-line/promtool/
- https://sre.google/workbook/alerting-on-slos/
