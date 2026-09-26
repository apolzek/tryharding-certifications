# Federation, Remote Write & Agent: tirando métricas de dentro de um Prometheus

> **Em uma frase:** um Prometheus sozinho é **local e finito** (um servidor, retenção de dias/semanas). Para ter visão **global**, **longo prazo** ou **alta disponibilidade**, os dados saem dele por **federation** (outro Prometheus **puxa** `/federate`) ou por **remote write** (ele **empurra** para um storage remoto), sempre carimbados com **`external_labels`**.

| | |
|---|---|
| **prom-a** (cluster a) | http://localhost:9160 |
| **prom-b** (cluster b) | http://localhost:9161 |
| **global** (federação) | http://localhost:9162 |
| **receiver** (remote write) | http://localhost:9163 |
| **agent** (`--agent`) | http://localhost:9164 |
| **Domínio da prova** | *Prometheus Fundamentals* (arquitetura, federation, remote storage, agent) |
| **Teste automático** | `./test.sh` (≈ 2 min) |

---

## 🧠 Analogia: filiais, matriz e o arquivo central

Uma rede de lojas com uma **filial por cidade** (cada `prom-a`, `prom-b` é o caixa local de uma filial).

- **Federation = o gerente da matriz liga para cada filial e pede o resumo do dia.** A matriz (`global`) **puxa** (`GET /federate`), e pede só o que interessa ("total de vendas por loja", as recording rules `job:*`), não cada cupom fiscal. Se ele pedir *todos* os cupons de *todas* as filiais, a matriz afoga.
- **Remote write = cada filial manda, continuamente, uma cópia para o arquivo central** (Mimir, Thanos Receive, Cortex, VictoriaMetrics, Grafana Cloud...). É **push**, quase em tempo real, e dá para escolher o que vai na caixa (`write_relabel_configs`).
- **`external_labels` = o carimbo da filial no envelope** (`cluster="a"`). Sem ele, no arquivo central, "vendas: 18" da filial A e "vendas: 7" da filial B são indistinguíveis e uma sobrescreve a outra.
- **Agent mode = um quiosque sem cofre**: só registra e despacha para o central. Não guarda histórico consultável, não tem regras nem alertas.
- **HA pair = duas filiais gêmeas registrando a mesma coisa** (réplica A e B). O arquivo central precisa **deduplicar**, e para isso o carimbo tem que dizer "sou a réplica A" (`replica`/`__replica__`).

---

## 🏗️ Arquitetura

```
                 app-a:8000                      app-b:8000                   app-c:8000
                    │ scrape                        │ scrape                      │ scrape
                    ▼                               ▼                             ▼
   ┌──────────────────────────┐    ┌──────────────────────────┐     ┌──────────────────────────┐
   │ prom-a :9160             │    │ prom-b :9161             │     │ agent :9164  (--agent)   │
   │ external_labels:         │    │ external_labels:         │     │ external_labels:         │
   │   cluster: a             │    │   cluster: b             │     │   cluster: edge          │
   │ rules: job:*             │    │ rules: job:*             │     │ sem TSDB, sem rules      │
   └──────┬──────────────┬────┘    └────┬──────────────┬──────┘     └────────────┬─────────────┘
          │ GET /federate│              │ GET /federate│                         │
          │ match[]=job:*│              │              │ remote_write            │ remote_write
          │   (PULL)     │ remote_write │              │ keep job:*|up           │ (TUDO)
          ▼              │ keep job:*|up▼              │ (PUSH)                  │
   ┌──────────────────────────────────────┐            │                         │
   │ global :9162                         │            ▼                         ▼
   │ honor_labels: true                   │   ┌─────────────────────────────────────────────┐
   │ vê: job:*{cluster="a"|"b"}           │   │ receiver :9163                              │
   │ remote_read ─────────────────────────┼──►│ --web.enable-remote-write-receiver          │
   │   (só queries com cluster="edge")    │   │ (papel de Mimir / Thanos Receive / Cortex)  │
   └──────────────────────────────────────┘   │ vê: job:*{a,b} + up{a,b} + tudo{edge}       │
                                              └─────────────────────────────────────────────┘
```

| Mecanismo | Direção | Quem inicia | Latência | Para quê |
|---|---|---|---|---|
| **Federation** (`/federate`) | pull | Prometheus de cima | 1 scrape interval | visão global de **agregados**, hierarquias pequenas |
| **Remote write** | push | Prometheus de origem | segundos | long-term storage, visão global, multi-tenant |
| **Remote read** | pull (query) | Prometheus que consulta | por query | consultar storage remoto via PromQL local (pouco usado hoje) |
| **Agent** | push | agent | segundos | coletores leves na borda, sem storage local |

---

## ▶️ Como rodar

```bash
cd labs/federation-remote-write
docker compose up -d --wait
# espere ~20s para as recording rules (rate[1m]) terem dados
```

As configs "vivas" ficam em `prometheus/<serviço>/prometheus.yml`. Para trocar, use o [`load.sh`](load.sh) (roda `promtool check config`, com `--agent` quando o destino é o agent, copia e faz `POST /-/reload`):

```bash
./load.sh exercises/01-federate-job-rules/global.yml global
./load.sh reset          # volta os 5 Prometheus para configs/
```

---

## 🔍 Passo a passo

### 1. O `/federate` na mão

É um endpoint HTTP comum. Você diz **quais séries** quer com um ou mais `match[]` (seletores PromQL) e recebe o **último valor** de cada uma, no formato de exposição:

```bash
curl -s -G localhost:9160/federate --data-urlencode 'match[]={__name__=~"job:.*"}'
# # TYPE job:http_requests:rate1m untyped
# job:http_requests:rate1m{job="app",cluster="a",instance=""} 18.89 1790433922886
# ...
```
Repare:
- `cluster="a"` foi **adicionado** pelo `prom-a`: são os `external_labels`.
- `instance=""`: o `/federate` manda `instance` vazio quando a série não tem, para o Prometheus de cima (com `honor_labels: true`) **não** colar o `instance` dele.
- Vem **com timestamp** (o da amostra original).
- Sem `match[]` a resposta é vazia.

### 2. O global federa só agregados

[`configs/global.yml`](configs/global.yml):
```yaml
- job_name: federate
  scrape_interval: 10s
  honor_labels: true             # mantém job/instance/cluster da origem
  metrics_path: /federate
  params:
    'match[]':
      - '{__name__=~"job:.*"}'   # só recording rules "job:"
  static_configs:
    - targets: [prom-a:9090, prom-b:9090]
```
```bash
curl -s localhost:9162/api/v1/query --data-urlencode 'query={__name__=~"job:.*"}' | jq -r '.data.result[] | "\(.metric.__name__) \(.metric.cluster) \(.value[1])"'
# job:http_requests:rate1m a 18.9
# job:http_requests:rate1m b 7.5
# ... (3 métricas x 2 clusters)
curl -s localhost:9162/api/v1/query --data-urlencode 'query=http_requests_total' | jq '.data.result | length'
# 0   <- séries cruas ficam nas filiais
```
Agora dá para somar os clusters no global:
```promql
sum(job:http_requests:rate1m)                      # req/s globais
sum by (cluster) (job:http_requests_error_ratio:rate1m)
```

**Quando usar federation:** hierarquia de agregados (um global com `job:`/`cluster:` de N Prometheus), ou puxar **algumas** séries de outro Prometheus.
**Quando NÃO usar:** para copiar "tudo" (federation não é replicação), para long-term storage, ou quando o volume é grande: o `/federate` é um scrape gigante, com timeout e sem retry. Para isso: remote write + Thanos/Mimir.

### 3. Remote write: o `prom-a` empurra para o receiver

[`configs/prom-a.yml`](configs/prom-a.yml):
```yaml
remote_write:
  - url: http://receiver:9090/api/v1/write
    name: central
    queue_config:
      capacity: 2500             # amostras bufferizadas por shard
      max_shards: 10             # paralelismo máximo (ajustado sozinho)
      max_samples_per_send: 500  # tamanho do lote
      batch_send_deadline: 5s    # envia mesmo com o lote incompleto após 5s
    write_relabel_configs:
      - source_labels: [__name__]
        regex: 'job:.*|up'
        action: keep             # só estes saem; o TSDB local continua com tudo
```
Como funciona por dentro: o remote write **lê o WAL** (write-ahead log) do Prometheus e envia em lotes, com *shards* paralelos e retry com backoff. Se o destino cair, ele segura enquanto o WAL existir (~2h) e depois **perde** dados.

O receptor precisa aceitar escrita: `--web.enable-remote-write-receiver` (Prometheus 3) habilita `POST /api/v1/write`. Sem a flag:
```bash
curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:9160/api/v1/write
# 404   (prom-a não é receiver)
```

No receiver:
```bash
curl -s localhost:9163/api/v1/query --data-urlencode 'query=count by (cluster) ({cluster!=""})' | jq -c '.data.result[] | [.metric, .value[1]]'
# [{"cluster":"edge"},"556"]   <- o agent manda tudo
# [{"cluster":"a"},"5"]        <- 3 job:* + 2 up
# [{"cluster":"b"},"5"]
```

Monitorando o remote write (no **emissor**):
```promql
rate(prometheus_remote_storage_samples_total[1m])                       # amostras enviadas/s
rate(prometheus_remote_storage_samples_failed_total[1m])                 # falhas não recuperáveis
sum by (reason) (prometheus_remote_storage_samples_dropped_total)       # descartadas (ex.: write_relabel)
prometheus_remote_storage_samples_pending                                # fila
prometheus_remote_storage_shards                                         # shards em uso
# atraso (s) entre o que entrou no WAL e o que já foi enviado:
prometheus_remote_storage_queue_highest_timestamp_seconds - prometheus_remote_storage_queue_highest_sent_timestamp_seconds
```
```bash
curl -s localhost:9160/api/v1/query --data-urlencode 'query=sum by (reason) (prometheus_remote_storage_samples_dropped_total)' | jq -c '.data.result[] | [.metric, .value[1]]'
# [{"reason":"dropped_series"},"3275"]
```

### 4. `external_labels`: por que são obrigatórios fora de casa

```bash
curl -s localhost:9160/api/v1/query --data-urlencode 'query=job:http_requests:rate1m' | jq -c '.data.result[].metric'
# {"__name__":"job:http_requests:rate1m","job":"app"}         <- LOCALMENTE não aparece cluster
curl -s localhost:9163/api/v1/query --data-urlencode 'query=job:http_requests:rate1m' | jq -c '.data.result[].metric'
# {"__name__":"job:http_requests:rate1m","cluster":"a","job":"app"}
# {"__name__":"job:http_requests:rate1m","cluster":"b","job":"app"}
```
Os `external_labels` são aplicados em **tudo que sai**: `/federate`, remote write, alertas enviados ao Alertmanager e (como filtro) remote read. Um `sum by (job)` num Prometheus de cluster não tem `instance` para diferenciar; sem `cluster`, as séries de dois clusters colidem (exercício [02](exercises/02-external-labels/)).

### 5. Agent mode

```bash
docker compose exec agent ps    # /bin/prometheus --agent --config.file=/etc/prometheus/prometheus.yml --storage.agent.path=/prometheus ...
curl -s 'localhost:9164/api/v1/query?query=up'
# {"status":"error","errorType":"execution","error":"unavailable with Prometheus Agent"}
curl -s localhost:9164/metrics | grep '^prometheus_agent_active_series'
# prometheus_agent_active_series 556
```
- Só `global`, `scrape_configs` e `remote_write` (e SD/relabel). **Não** tem `rule_files`, `alerting`, `remote_read`, nem queries.
- Guarda só um **WAL** (`--storage.agent.path`); apaga o que já foi enviado. Consome bem menos memória/disco.
- Valide com `promtool check config --agent`.
- É o que roda por trás do "Grafana Agent/Alloy em modo Prometheus" e em clusters de borda que mandam tudo para um Mimir central.

### 6. Remote read

```yaml
remote_read:
  - url: http://receiver:9090/api/v1/read
    read_recent: true
    required_matchers: { cluster: edge }   # só vai ao remoto se a query tiver cluster="edge"
    filter_external_labels: false          # não acrescentar tier="global" na leitura
```
O Prometheus faz a query PromQL **localmente**, mas busca as séries cruas também no remoto (**o processamento é local**: puxa os dados crus pela rede). Detalhes e pegadinhas no exercício [07](exercises/07-remote-read/).

### 7. HA pairs e deduplicação (conceito)

Prometheus não tem cluster/replicação nativa. HA = **dois Prometheus idênticos** raspando os mesmos alvos:

```
      alvos ──► prometheus-0  (external_labels: cluster=prod, replica=0) ──┐
        └─────► prometheus-1  (external_labels: cluster=prod, replica=1) ──┼──► Thanos Querier / Mimir
                                                                           │    dedup por "replica"
      Alertmanager (cluster de 3) ◄─────────── os dois mandam alertas ─────┘    (Alertmanager deduplica alertas)
```
- **Alertas:** os dois enviam; o **Alertmanager deduplica** (por isso os alertas não podem depender do label `replica`: use `alert_relabel_configs` para removê-lo, ou o Alertmanager verá dois alertas diferentes).
- **Dados:** as séries diferem **só** no label de réplica.
  - **Thanos Querier:** `--query.replica-label=replica` junta as duas em tempo de query.
  - **Mimir/Cortex (HA tracker):** aceita só a réplica "eleita" por `cluster` + `__replica__` e descarta a outra na ingestão; o `__replica__` não é gravado.
- Consultar os dois Prometheus direto no Grafana (sem dedup) mostra **dados em dobro** em `sum()`.

### 8. Onde guardar por meses/anos

| Opção | Como recebe | Observação |
|---|---|---|
| Prometheus local | scrape | retenção por tempo/tamanho (`--storage.tsdb.retention.time`, `.size`), um nó só |
| **Thanos** | *sidecar* sobe blocos para object storage (S3/GCS) **ou** *Receive* (remote write) | Querier global + dedup, Compactor faz downsampling |
| **Grafana Mimir** / Cortex | remote write | multi-tenant (`X-Scope-OrgID`), HA tracker, object storage |
| **VictoriaMetrics** | remote write (e scrape próprio) | single-node ou cluster, compressão alta |
| **Grafana Cloud / AMP (AWS) / GMP (Google)** | remote write | gerenciado |
| M3, InfluxDB, TimescaleDB (Promscale, descontinuado)... | remote write | lista em *Remote Endpoints and Storage* na doc |

---

## 🏭 Casos reais

### Caso 1: Prometheus em Kubernetes mandando para o Mimir (multi-tenant)

```yaml
global:
  external_labels:
    cluster: prod-sa-east-1
    __replica__: prometheus-0        # Mimir HA tracker: dedup por cluster + __replica__
remote_write:
  - url: https://mimir.example.com/api/v1/push
    headers:
      X-Scope-OrgID: team-payments   # tenant
    queue_config:
      max_samples_per_send: 2000
      max_shards: 50
      capacity: 10000
    write_relabel_configs:
      - source_labels: [__name__]
        regex: 'go_gc_.*|go_sched_.*|prometheus_sd_.*'
        action: drop                 # não pagar por métricas que ninguém olha
```

### Caso 2: Grafana Cloud (basic auth) a partir de um agent

```yaml
global:
  scrape_interval: 60s
  external_labels:
    cluster: edge-store-042
scrape_configs:
  - job_name: node
    static_configs: [{ targets: ['localhost:9100'] }]
remote_write:
  - url: https://prometheus-prod-13-prod-us-east-0.grafana.net/api/prom/push
    basic_auth:
      username: "123456"
      password_file: /etc/prometheus/grafana-cloud.token
```
(rodando com `prometheus --agent`)

### Caso 3: federação hierárquica clássica (doc oficial)

```yaml
- job_name: federate
  scrape_interval: 15s
  honor_labels: true
  metrics_path: /federate
  params:
    'match[]':
      - '{job="prometheus"}'
      - '{__name__=~"job:.*"}'
  static_configs:
    - targets:
        - source-prometheus-1:9090
        - source-prometheus-2:9090
        - source-prometheus-3:9090
```

### Caso 4: HA pair + Thanos sidecar

```yaml
# prometheus-0 (o prometheus-1 é idêntico, com replica: "1")
global:
  external_labels:
    cluster: prod
    replica: "0"
alerting:
  alert_relabel_configs:
    - regex: replica           # o Alertmanager precisa ver os alertas das 2 réplicas como IGUAIS
      action: labeldrop
```
```bash
# Thanos Querier deduplicando
thanos query --endpoint=prometheus-0-sidecar:10901 --endpoint=prometheus-1-sidecar:10901 --query.replica-label=replica
```
(O sidecar exige `--storage.tsdb.min-block-duration=2h --storage.tsdb.max-block-duration=2h` no Prometheus para subir blocos de 2h.)

### Caso 5: remote write com fila resistente a quedas longas

```yaml
remote_write:
  - url: https://central.example.com/api/v1/write
    remote_timeout: 30s
    queue_config:
      capacity: 10000
      max_shards: 30
      min_backoff: 30ms
      max_backoff: 5s
      retry_on_http_429: true     # respeitar rate limit do destino
    metadata_config:
      send: true
```

Todos os YAML acima passam no `promtool check config` com a v3.15.0 (o caso 2 com `--agent`).

---

## 🧪 Exercícios

| # | Tema | Pasta |
|---|---|---|
| 01 | federar só `job:*` (`match[]` ancorado) | [01-federate-job-rules](exercises/01-federate-job-rules/) |
| 02 | `external_labels` duplicados → colisão no remote write | [02-external-labels](exercises/02-external-labels/) |
| 03 | `write_relabel_configs` (keep x drop) | [03-write-relabel](exercises/03-write-relabel/) |
| 04 | montar um agent (`--agent`) | [04-agent-mode](exercises/04-agent-mode/) |
| 05 | comparar contagens de séries | [05-compare-series](exercises/05-compare-series/) |
| 06 | `honor_labels` na federação | [06-honor-labels-federation](exercises/06-honor-labels-federation/) |
| 07 | `remote_read` e `filter_external_labels` | [07-remote-read](exercises/07-remote-read/) |

O [`test.sh`](test.sh) carrega cada versão quebrada (confere o sintoma) e depois a solução (confere com dados frescos).

---

## ⚠️ Pegadinhas

1. **`match[]` é obrigatório** no `/federate`, e a regex é ancorada (`job:` não casa com `job:x`).
2. **Federation sem `honor_labels: true`** troca `job`/`instance` da origem por `exported_job`/`exported_instance`.
3. **Federar tudo** (`{__name__=~".+"}`) é anti-padrão: scrape gigante, timeouts, e o global vira ponto único de falha com toda a cardinalidade.
4. **`external_labels` não aparecem em queries locais**, só no que sai. Colocar o mesmo `cluster` em dois Prometheus = colisão silenciosa ou rejeição (`duplicate sample`).
5. **`write_relabel_configs` afeta só o remote write**; `metric_relabel_configs` afeta o local **e** o remoto.
6. **Remote write lê o WAL:** se o destino ficar fora mais que a janela do WAL (~2h), dados se perdem. Monitore `..._samples_failed_total`, `..._samples_pending` e o atraso `highest_timestamp - highest_sent_timestamp`.
7. **O receptor precisa de `--web.enable-remote-write-receiver`**, senão `404` em `/api/v1/write`.
8. **Agent não consulta, não avalia regras, não alerta.** `rule_files`/`alerting`/`remote_read` na config = erro.
9. **Remote read acrescenta os `external_labels` do leitor** como filtro (`filter_external_labels: true` padrão) e é processado localmente (puxa dados crus).
10. **HA sem dedup** = `sum()` em dobro. O label de réplica deve ser o **único** diferente entre as réplicas.

---

## 🎓 Na prova PCA

**1.** A global Prometheus federates from several datacenter Prometheus servers. Which setting keeps the original `job` and `instance` labels from the source servers?
- A) `honor_timestamps: true`
- B) `honor_labels: true`
- C) `external_labels`
- D) `metric_relabel_configs` with `labelmap`

<details><summary>Resposta</summary>

**B.** Sem `honor_labels: true` os labels do Prometheus que raspa vencem e os originais viram `exported_job`/`exported_instance`.
</details>

**2.** Which flag lets a Prometheus 3 server accept samples pushed by other Prometheus servers via remote write?
- A) `--enable-feature=remote-write`
- B) `--web.enable-lifecycle`
- C) `--web.enable-remote-write-receiver`
- D) `--storage.remote.enable`

<details><summary>Resposta</summary>

**C.** Habilita `POST /api/v1/write`. (Em versões 2.x antigas era `--enable-feature=remote-write-receiver`; na v3 só existe a flag.)
</details>

**3.** Where are `external_labels` applied?
- A) To every series stored in the local TSDB
- B) Only to alerts
- C) To data leaving the server: federation, remote write, alerts sent to Alertmanager
- D) Only to recording rules

<details><summary>Resposta</summary>

**C.** Eles não são gravados localmente; são anexados em tudo que "sai" do servidor (e usados como filtro no remote read).
</details>

**4.** Which statement about Prometheus Agent mode is TRUE?
- A) It supports recording rules but not alerting rules
- B) It can be queried with PromQL through `/api/v1/query`
- C) It scrapes targets and forwards samples with remote write, keeping only a WAL
- D) It replaces Alertmanager

<details><summary>Resposta</summary>

**C.** O agent desliga TSDB consultável, regras, alertas e remote read. Só raspa e envia (com WAL local).
</details>

**5.** You only want recording-rule results (`job:*`) to be sent to a remote storage while keeping everything locally. What do you use?
- A) `metric_relabel_configs` with `action: keep`
- B) `write_relabel_configs` with `action: keep` on `__name__`
- C) `relabel_configs` with `action: drop`
- D) `alert_relabel_configs`

<details><summary>Resposta</summary>

**B.** `write_relabel_configs` filtra só aquele `remote_write`. A (`metric_relabel_configs`) descartaria localmente também. C age em alvos.
</details>

**6.** What is the recommended use of federation?
- A) Replicating all series from one Prometheus to another for backup
- B) Pulling aggregated series (e.g. recording rules) from lower-level Prometheus servers into a global one
- C) Long-term storage for years
- D) Deduplicating HA pairs

<details><summary>Resposta</summary>

**B.** Federação hierárquica **de agregados**. Backup/long-term/dedup são papéis de remote write + Thanos/Mimir/etc.
</details>

**7.** Two identical Prometheus replicas remote-write to Mimir. What must differ between them so that Mimir's HA tracker can deduplicate?
- A) `job_name`
- B) A replica external label (e.g. `__replica__`), while `cluster` stays the same
- C) `scrape_interval`
- D) Nothing; Mimir detects duplicates automatically

<details><summary>Resposta</summary>

**B.** O HA tracker elege uma réplica por `cluster` usando o label `__replica__` e descarta as amostras da outra. Com Thanos Querier o equivalente é `--query.replica-label`.
</details>

**8.** Which metric best shows that remote write is falling behind?
- A) `up`
- B) `prometheus_remote_storage_queue_highest_timestamp_seconds - prometheus_remote_storage_queue_highest_sent_timestamp_seconds`
- C) `prometheus_tsdb_head_series`
- D) `scrape_duration_seconds`

<details><summary>Resposta</summary>

**B.** É o atraso (em segundos) entre a amostra mais nova que entrou na fila e a mais nova já enviada. Crescendo = o destino não está dando conta (veja também `samples_pending` e `shards`).
</details>

---

## 📝 Cola rápida

- **Federation** = pull de `/federate?match[]=...`, `honor_labels: true`, só agregados (`job:*`), intervalo maior. Não é backup nem long-term.
- **Remote write** = push a partir do WAL, `queue_config` (shards, capacity, max_samples_per_send, batch_send_deadline), `write_relabel_configs` para filtrar só o envio.
- Receptor: `--web.enable-remote-write-receiver` → `POST /api/v1/write`.
- **Remote read** = PromQL local sobre dados crus do remoto; `read_recent`, `required_matchers`, `filter_external_labels`.
- **Agent** = `--agent`: scrape + remote write + WAL. Sem queries/regras/alertas. `promtool check config --agent`.
- **`external_labels`** = carimbo de origem em tudo que sai (`cluster`, `replica`). Obrigatório com federation/remote write/HA.
- **HA** = 2 réplicas idênticas + dedup (Thanos `--query.replica-label`, Mimir HA tracker `cluster`+`__replica__`); alertas deduplicados pelo Alertmanager (remova `replica` com `alert_relabel_configs`).
- Long-term: Thanos, Mimir/Cortex, VictoriaMetrics, serviços gerenciados.
- Métricas: `prometheus_remote_storage_samples_{total,failed_total,dropped_total,pending}`, `..._shards`, `..._queue_highest_(sent_)timestamp_seconds`.

## 📚 Referências

- https://prometheus.io/docs/prometheus/latest/federation/
- https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write
- https://prometheus.io/docs/practices/remote_write/
- https://prometheus.io/docs/prometheus/latest/feature_flags/ e https://prometheus.io/blog/2021/11/16/agent/
- https://prometheus.io/docs/operating/integrations/#remote-endpoints-and-storage
- https://grafana.com/docs/mimir/latest/configure/configure-high-availability-deduplication/
- https://thanos.io/tip/components/query.md/#deduplication
