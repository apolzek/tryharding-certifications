# Service Discovery & Relabeling: como o Prometheus acha (e renomeia) o que raspar

> **Em uma frase:** o Prometheus **puxa** (`pull`) métricas de alvos que ele **descobre** (static, arquivo, HTTP, Kubernetes, Consul, EC2...); antes de cada scrape, `relabel_configs` decide **se** e **como** raspar cada alvo; depois do scrape, `metric_relabel_configs` decide **quais séries** são gravadas.

| | |
|---|---|
| **Prometheus** | http://localhost:9120 (UI: `/targets`, `/service-discovery`) |
| **2º Prometheus (sharding)** | http://localhost:9129 |
| **Alvos fake** | 9121 payments-api · 9122 payments-worker · 9123 checkout-api · 9124 search-api · 9125 legacy-batch · 9126 prober · 9127 sd-server · 9128 inventory-api |
| **Domínio da prova** | *Prometheus Fundamentals* e *Observability Concepts* (config, scraping, SD, relabel) |
| **Teste automático** | `./test.sh` (≈ 2 min) |

---

## 🧠 Analogia: o carteiro, a lista de endereços e a triagem

- O Prometheus é um **carteiro que vai buscar** as cartas (modelo **pull**). Ninguém entrega nada para ele (exceto via Pushgateway/remote write, outros assuntos).
- **Service discovery** é **de onde vem a lista de endereços**: um papel escrito à mão (`static_configs`), uma planilha que alguém atualiza (`file_sd_configs`), um site que publica a lista (`http_sd_configs`) ou a "prefeitura" (API do Kubernetes, Consul, AWS). Cada endereço chega com **etiquetas de rascunho** (`__meta_*`: bairro, dono, tipo de imóvel).
- **`relabel_configs`** é a **triagem antes de sair de casa**: "só vou aos endereços do time payments" (`keep`), "o número da casa está na etiqueta X" (monta `__address__`), "escreva no envelope o nome do dono" (`labelmap`/`replace`). Os rascunhos (`__*`) são jogados fora quando ele sai.
- **`metric_relabel_configs`** é a **triagem das cartas já coletadas**, antes de arquivar: "panfleto não entra no arquivo" (`drop` de métrica cara), "tira o carimbo inútil" (`labeldrop`).

---

## 🏗️ Arquitetura

```
                            ┌───────────────────────── docker network ─────────────────────────┐
  prometheus/prometheus.yml │                                                                  │
  prometheus/sd/*.json ────►│  ┌──────────────┐  static_configs   ┌─────────────────┐          │
   (file_sd: editado ao     │  │  Prometheus  │ ─────────────────►│ payments-api    │ :9121    │
    vivo, sem reload)       │  │    :9120     │                   │ payments-worker │ :9122    │
                            │  │              │  file_sd_configs  │ checkout-api    │ :9123    │
  ./load.sh X.yml ─────────►│  │ --web.enable-│ ─────────────────►│ search-api      │ :9124    │
   (promtool + /-/reload)   │  │   lifecycle  │                   │ inventory-api   │ :9128    │
                            │  │              │  http_sd_configs  ├─────────────────┤          │
                            │  │              │ ── GET /targets.json ─► sd-server   │ :9127    │
                            │  │              │                   ├─────────────────┤          │
                            │  │              │  /admin/metrics   │ legacy-batch    │ :9125    │
                            │  │              │  ?format=...      │ (expõe job=...) │          │
                            │  │              │  /probe?target=   │ prober          │ :9126    │
                            │  └──────────────┘                   └─────────────────┘          │
                            │  ┌──────────────┐  (exercício 06: hashmod, metade dos alvos)     │
                            │  │ Prometheus   │                                                │
                            │  │ shard1 :9129 │                                                │
                            │  └──────────────┘                                                │
                            └──────────────────────────────────────────────────────────────────┘
```

Todos os alvos são **a mesma imagem** (`python:3.14-alpine` + [`target/target.py`](target/target.py), só stdlib) com env vars diferentes. Cada um expõe:

| Métrica | Observação |
|---|---|
| `app_info{service,team,version}` | 1 por alvo |
| `app_requests_total{route}` | counter, 3 rotas |
| `app_cache_entries{cache,pod_uid}` | `pod_uid` é o label "ruído" (exercício 05) |
| `app_requests_by_user_total{user_id}` | **só no payments-api**: 150 séries (alta cardinalidade) |
| `batch_*{job="nightly-batch"}`, `batch_heartbeat` | **só no legacy-batch** (caminho `/admin/metrics`): label `job` exposto + timestamp explícito |
| `app_scrape_param{name,value}` | ecoa os parâmetros de URL recebidos (`params:`) |
| `probe_success` em `/probe?target=` | **prober**: estilo blackbox |

### O pipeline de um alvo (decore isto)

```
  SD (static/file/http/k8s/consul/ec2...)
      │  gera: __address__, __meta_*, labels do grupo, e o Prometheus adiciona
      │        job, __scheme__, __metrics_path__, __scrape_interval__, __scrape_timeout__, __param_*
      ▼
  ┌───────────────────────┐   "discoveredLabels"  (API /api/v1/targets, UI /service-discovery)
  │   relabel_configs     │   keep/drop → alvo some (vai para droppedTargets)
  └───────────────────────┘   replace/labelmap/hashmod... → reescreve labels
      │  se instance não existir: instance = __address__
      │  todos os labels "__*" são REMOVIDOS  (__meta_*, __tmp_*, __address__...)
      ▼
  "labels" do alvo = job, instance + o que você criou   ──►  up, scrape_* usam ESTES labels
      │
      ▼  GET <__scheme__>://<__address__><__metrics_path__>?<__param_*>
  ┌───────────────────────┐
  │   scrape              │   honor_labels / honor_timestamps decidem conflitos
  └───────────────────────┘
      ▼
  ┌───────────────────────┐   por SÉRIE: drop de métricas caras, labeldrop...
  │ metric_relabel_configs│
  └───────────────────────┘
      ▼   sample_limit / label_limit conferidos AQUI (depois do metric_relabel)
    TSDB
```

---

## ▶️ Como rodar

```bash
cd labs/service-discovery-relabeling
docker compose up -d --wait
open http://localhost:9120/targets          # ou xdg-open
```

Para trocar a config, use o helper [`load.sh`](load.sh): ele roda `promtool check config`, copia para `prometheus/prometheus.yml` e chama `POST /-/reload` (habilitado por `--web.enable-lifecycle`):

```bash
./load.sh configs/base.yml                    # volta ao estado inicial
./load.sh exercises/01-keep-team-payments/prometheus.yml
```

Sem o helper, o equivalente é:
```bash
docker run --rm -v "$PWD/prometheus:/etc/prometheus:ro" --entrypoint promtool prom/prometheus:v3.15.0 check config /etc/prometheus/prometheus.yml
curl -X POST localhost:9120/-/reload          # ou: docker compose kill -s SIGHUP prometheus
```

> Os comandos abaixo usam `jq`. Rode tudo a partir de `labs/service-discovery-relabeling/`.

---

## 🔍 Passo a passo

### 1. Pull: o alvo só expõe, o Prometheus é quem busca

```bash
curl -s localhost:9121/metrics | head -12
# app_info{service="payments-api",team="payments",version="1.0.0"} 1
# app_requests_total{route="/"} 76
# ...
```
O alvo não sabe nada de Prometheus, `job` ou `instance`. Quem adiciona `job`/`instance` é o **servidor**, na hora do scrape.

### 2. A config base: 4 formas de achar alvos

[`configs/base.yml`](configs/base.yml) (carregada no start):

```yaml
global:
  scrape_interval: 5s      # de quanto em quanto tempo raspar (padrão: 1m)
  scrape_timeout: 4s       # padrão: 10s. Precisa ser <= scrape_interval
scrape_configs:
  - job_name: static-apps              # vira o label job
    static_configs:
      - targets: [payments-api:8000, payments-worker:8000]
        labels: { team: payments }
  - job_name: file-sd-apps
    file_sd_configs:
      - files: [/etc/prometheus/sd/apps*.json]
        refresh_interval: 30s
  - job_name: http-sd-apps
    http_sd_configs:
      - url: http://sd-server:8000/targets.json
        refresh_interval: 15s
  - job_name: legacy-batch
    metrics_path: /admin/metrics       # padrão: /metrics
    scheme: http                       # padrão: http
    params: { format: [prometheus] }   # vira ?format=prometheus
    static_configs:
      - targets: [legacy-batch:8000]
```

```bash
curl -s localhost:9120/api/v1/targets | jq -r '.data.activeTargets[] | "\(.scrapePool)\t\(.labels.instance)\t\(.health)"' | sort
# file-sd-apps   checkout-api:8000     up
# ...            (10 alvos)
# static-apps    payments-worker:8000  up
```

Os `params` chegam no alvo:
```bash
curl -s localhost:9120/api/v1/query --data-urlencode 'query=app_scrape_param' | jq -c '.data.result[].metric'
# {"__name__":"app_scrape_param","instance":"legacy-batch:8000","job":"legacy-batch","name":"format","value":"prometheus"}
```

### 3. As séries que o Prometheus cria sozinho para cada alvo

```promql
up                                   # 1 = scrape OK, 0 = falhou (a métrica nº 1 para alertar alvo fora)
scrape_duration_seconds              # quanto o scrape demorou
scrape_samples_scraped               # amostras que o alvo expôs
scrape_samples_post_metric_relabeling # amostras que sobraram depois do metric_relabel_configs
scrape_series_added                  # séries NOVAS neste scrape (churn!)
```
```bash
curl -s localhost:9120/api/v1/query --data-urlencode 'query=scrape_samples_scraped{job=~"static-apps|legacy-batch"}' | jq -r '.data.result[] | "\(.metric.instance) \(.value[1])"'
# legacy-batch:8000 9
# payments-api:8000 155      <- o culpado da cardinalidade
# payments-worker:8000 5
```

### 4. Antes x depois do relabel: `discoveredLabels` vs `labels`

```bash
curl -s 'localhost:9120/api/v1/targets?scrapePool=http-sd-apps' | jq '.data.activeTargets[0] | {discoveredLabels, labels}'
```
```json
{
  "discoveredLabels": {
    "__address__": "search-api:8000",
    "__meta_datacenter": "sa-east-1a",
    "__meta_owner_team": "search",
    "__meta_url": "http://sd-server:8000/targets.json",
    "__metrics_path__": "/metrics",
    "__scheme__": "http",
    "__scrape_interval__": "5s",
    "__scrape_timeout__": "4s",
    "env": "prod",
    "job": "http-sd-apps",
    "...": "..."
  },
  "labels": { "env": "prod", "instance": "search-api:8000", "job": "http-sd-apps" }
}
```
Tudo que começa com `__` **sumiu**. Se você quer um `__meta_*` na série, **copie** com relabel (`labelmap`/`replace`). A mesma visão existe na UI: **http://localhost:9120/service-discovery** (clique em *show more* em um alvo).

Alvos descartados pelo relabel:
```bash
curl -s 'localhost:9120/api/v1/targets?state=dropped' | jq '.data.droppedTargetCounts'
```

### 5. `file_sd`: hot reload ao vivo, sem `/-/reload`

Terminal 1:
```bash
watch -n1 "curl -s 'localhost:9120/api/v1/targets?scrapePool=file-sd-apps' | jq -r '.data.activeTargets[].labels.instance'"
```
Terminal 2: crie um arquivo que casa com o glob `apps*.json`:
```bash
cat > prometheus/sd/apps-inventory.json <<'EOF'
[{"targets": ["inventory-api:8000"], "labels": {"team": "inventory"}}]
EOF
```
Em ~1-2 s o `inventory-api:8000` aparece (o Prometheus observa o diretório com **inotify**; o `refresh_interval` é o plano B). Apague o arquivo e ele some:
```bash
rm prometheus/sd/apps-inventory.json
```
O file_sd adiciona `__meta_filepath` (qual arquivo gerou o alvo). Formato: JSON ou YAML, lista de `{targets: [...], labels: {...}}`. É o "SD genérico": qualquer script/CMDB/Ansible que escreva esse arquivo vira service discovery.

### 6. `http_sd`: mesma ideia, via HTTP

```bash
curl -s localhost:9127/targets.json | jq -c '.[]'
jq '. + [{"targets":["inventory-api:8000"],"labels":{"env":"prod","__meta_datacenter":"sa-east-1b","__meta_owner_team":"inventory"}}]' \
   sd-server/targets.json > /tmp/sd.json && cat /tmp/sd.json > sd-server/targets.json
# em até 15s (refresh_interval), o http-sd-apps passa a ter 3 alvos
curl -s 'localhost:9120/api/v1/targets?scrapePool=http-sd-apps' | jq '.data.activeTargets | length'
git checkout sd-server/targets.json    # (ou desfaça a edição à mão)
```
O http_sd **não** usa inotify: é polling a cada `refresh_interval`. Se o endpoint falhar, a última lista boa é mantida.

### 7. `honor_labels` e `honor_timestamps`

O `legacy-batch` expõe `batch_records_processed_total{job="nightly-batch"}`. O job do scrape é `legacy-batch`. Conflito! Com o padrão `honor_labels: false`, o servidor vence e o label do alvo vira `exported_job`:
```bash
curl -s localhost:9120/api/v1/query --data-urlencode 'query=batch_records_processed_total{job="legacy-batch"}' | jq -c '.data.result[].metric'
# {"__name__":"batch_records_processed_total","exported_job":"nightly-batch","instance":"legacy-batch:8000","job":"legacy-batch"}
```
`batch_heartbeat 1 <timestamp_ms>` vem com **timestamp explícito** (30s no passado). Com `honor_timestamps: true` (padrão) o Prometheus grava esse timestamp em vez do horário do scrape:
```bash
curl -s localhost:9120/api/v1/query --data-urlencode 'query=time() - timestamp(batch_heartbeat{job="legacy-batch"})' | jq -r '.data.result[0].value[1]'
# ~30
```
Com `honor_timestamps: false` o valor ficaria perto de 0. (Ex.: exporters que repassam dados de outro sistema usam timestamps; em geral, **evite** expor timestamps.)

### 8. A vitrine das actions

```bash
./load.sh configs/demo-actions.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=demo-actions' | jq -c '.data.activeTargets[].labels'
# {"env":"prod","host_env":"payments-api@prod","instance":"payments-api:8000","job":"demo-actions","region":"sa-east-1","team":"payments","team_code":"PAYMENTS"}
# ... (checkout-api foi descartado pelo keepequal)
./load.sh configs/base.yml
```
Leia [`configs/demo-actions.yml`](configs/demo-actions.yml): `keepequal`, `lowercase`, `uppercase`, `__tmp_*`, `separator` e `labelkeep`.

### 9. Referência: todas as actions

| action | Usa | O que faz | Onde é comum |
|---|---|---|---|
| `replace` (padrão) | `source_labels`, `separator`, `regex`, `target_label`, `replacement` | se a regex casar com os valores concatenados, grava `replacement` (com `$1`...) em `target_label` | montar `__address__`, `instance`, copiar `__meta_*` |
| `keep` | `source_labels`, `regex` | mantém só se casar (alvo ou série) | filtrar alvos por anotação/tag |
| `drop` | `source_labels`, `regex` | descarta se casar | descartar métricas caras |
| `keepequal` / `dropequal` | `source_labels`, `target_label` | mantém/descarta se `source_labels` concatenado == valor de `target_label` (sem regex) | porta do container == porta anotada |
| `hashmod` | `source_labels`, `modulus`, `target_label` | `target_label = hash(valores) % modulus` | sharding |
| `labelmap` | `regex`, `replacement` | para cada label cujo **nome** casa, copia com o nome novo (`$1`) | `__meta_kubernetes_pod_label_(.+)` |
| `labeldrop` | `regex` (só) | remove labels cujo **nome** casa | tirar `pod_uid`, `id` |
| `labelkeep` | `regex` (só) | remove labels cujo **nome** NÃO casa | whitelist de labels |
| `lowercase` / `uppercase` | `source_labels`, `target_label` | grava o valor em minúsculas/maiúsculas | padronizar `env="Prod"` |

Padrões: `regex: (.*)`, `separator: ;`, `replacement: $1`, `action: replace`. **A regex é sempre ancorada** (`^(?:...)$`, sintaxe RE2).

Labels especiais:

| Label | Papel |
|---|---|
| `__address__` | `host:porta` que será raspado. Vira `instance` se você não definir `instance` |
| `__scheme__` | `http`/`https` |
| `__metrics_path__` | caminho (`/metrics`) |
| `__param_<nome>` | parâmetro de URL `?<nome>=` |
| `__scrape_interval__`, `__scrape_timeout__` | intervalo/timeout **por alvo** (dá para mudar via relabel) |
| `__meta_*` | metadados do SD (só existem no relabel_configs) |
| `__tmp_*` | reservado para **você** usar como variável temporária |
| `__name__` | nome da métrica (só existe em `metric_relabel_configs`) |

### 10. Service discovery "de verdade": o que cada um entrega

| SD | Exemplos de `__meta_*` | Uso típico de relabel |
|---|---|---|
| `kubernetes_sd_configs` (roles: `node`, `pod`, `service`, `endpoints`, `endpointslice`, `ingress`) | `__meta_kubernetes_namespace`, `__meta_kubernetes_pod_name`, `__meta_kubernetes_pod_label_<l>`, `__meta_kubernetes_pod_annotation_<a>`, `__meta_kubernetes_pod_container_port_number`, `__meta_kubernetes_pod_ip` | keep por anotação `prometheus.io/scrape`, porta/caminho da anotação, `labelmap` dos labels do pod |
| `consul_sd_configs` | `__meta_consul_service`, `__meta_consul_tags` (`,tag1,tag2,`), `__meta_consul_node`, `__meta_consul_dc`, `__meta_consul_service_metadata_<k>` | `keep` com `regex: .*,prometheus,.*`; `service` vira `job` |
| `ec2_sd_configs` | `__meta_ec2_instance_id`, `__meta_ec2_private_ip`, `__meta_ec2_tag_<Tag>`, `__meta_ec2_availability_zone` | `__address__` = IP privado + porta; `instance` = tag `Name` |
| `file_sd_configs` | `__meta_filepath` | o que você colocar no arquivo |
| `http_sd_configs` | `__meta_url` | idem |
| `dns_sd_configs` | `__meta_dns_name` | registros SRV/A |

(Existem dezenas: `azure`, `gce`, `docker`, `dockerswarm`, `nomad`, `openstack`, `hetzner`, `linode`...; o padrão é sempre o mesmo: SD entrega `__meta_*`, você usa relabel.)

---

## 🏭 Casos reais

### Caso 1: Kubernetes, o job `kubernetes-pods` (chart `prometheus-community/prometheus`)

```yaml
- job_name: kubernetes-pods
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      action: keep
      regex: true
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scheme]
      action: replace
      regex: (https?)
      target_label: __scheme__
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
      action: replace
      target_label: __metrics_path__
      regex: (.+)
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_ip]
      action: replace
      regex: (\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})
      replacement: '[$2]:$1'          # IPv6
      target_label: __address__
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_ip]
      action: replace
      regex: (\d+);((([0-9]+?)(\.|$)){4})
      replacement: $2:$1              # IPv4
      target_label: __address__
    - action: labelmap
      regex: __meta_kubernetes_pod_annotation_prometheus_io_param_(.+)
      replacement: __param_$1         # annotation prometheus.io/param_x -> ?x=
    - action: labelmap
      regex: __meta_kubernetes_pod_label_(.+)
    - source_labels: [__meta_kubernetes_namespace]
      target_label: namespace
    - source_labels: [__meta_kubernetes_pod_name]
      target_label: pod
    - source_labels: [__meta_kubernetes_pod_phase]
      regex: Pending|Succeeded|Failed|Completed
      action: drop
```
O exercício [03](exercises/03-address-from-meta/) reproduz isto com file_sd.

### Caso 2: blackbox_exporter (doc oficial)

```yaml
- job_name: blackbox-http
  metrics_path: /probe
  params:
    module: [http_2xx]
  static_configs:
    - targets: [https://prometheus.io, https://example.com]
  relabel_configs:
    - source_labels: [__address__]
      target_label: __param_target
    - source_labels: [__param_target]
      target_label: instance
    - target_label: __address__
      replacement: 127.0.0.1:9115     # onde o blackbox roda
```
Exercício [07](exercises/07-param-target/).

### Caso 3: Consul, só serviços com a tag `prometheus`

```yaml
- job_name: consul-services
  consul_sd_configs:
    - server: consul.service.consul:8500
      services: []                    # todos
  relabel_configs:
    - source_labels: [__meta_consul_tags]
      regex: .*,prometheus,.*         # tags vêm como ",a,b,c,"
      action: keep
    - source_labels: [__meta_consul_service]
      target_label: job
    - source_labels: [__meta_consul_dc]
      target_label: datacenter
```

### Caso 4: EC2, IP privado + tag Name como instance

```yaml
- job_name: ec2-node
  ec2_sd_configs:
    - region: sa-east-1
      port: 9100
      filters:
        - name: tag:Environment
          values: [prod]
  relabel_configs:
    - source_labels: [__meta_ec2_private_ip]
      regex: (.+)
      replacement: $1:9100
      target_label: __address__
    - source_labels: [__meta_ec2_tag_Name]
      target_label: instance
    - source_labels: [__meta_ec2_availability_zone]
      target_label: az
```

### Caso 5: cortar cardinalidade do cAdvisor/kubelet

```yaml
- job_name: kubernetes-cadvisor
  # ...
  metric_relabel_configs:
    - source_labels: [__name__]
      regex: container_(tasks_state|memory_failures_total|blkio_device_usage_total)
      action: drop
    - regex: id|name|image           # labels que explodem e ninguém usa
      action: labeldrop
```
Métricas para achar os vilões: `topk(10, count by (__name__)({__name__=~".+"}))`, `scrape_samples_scraped`, `scrape_series_added` e a página **TSDB Status** (`/tsdb-status`).

### Caso 6: Pushgateway (onde `honor_labels: true` é obrigatório)

```yaml
- job_name: pushgateway
  honor_labels: true      # o job/instance que o batch empurrou deve ser mantido
  static_configs:
    - targets: [pushgateway:9091]
```
Exercício [08](exercises/08-honor-labels/).

### Caso 7: limites como cinto de segurança

```yaml
- job_name: apps-multitenant
  sample_limit: 50000        # scrape inteiro falha (up=0) se passar
  label_limit: 30            # nº de labels por série
  label_value_length_limit: 200
  target_limit: 500          # SD devolveu mais? TODOS os alvos do job falham
  kubernetes_sd_configs: [{ role: pod }]
```

Todos os YAML acima passam no `promtool check config` com a v3.15.0.

---

## 🧪 Exercícios (quebre e conserte)

| # | Tema | Pasta |
|---|---|---|
| 01 | `keep` + regex ancorada | [01-keep-team-payments](exercises/01-keep-team-payments/) |
| 02 | `replace` de `instance` sem porta (colisão de séries) | [02-instance-sem-porta](exercises/02-instance-sem-porta/) |
| 03 | `__address__` a partir de `__meta_*` (+ `separator`, `__metrics_path__`) | [03-address-from-meta](exercises/03-address-from-meta/) |
| 04 | `metric_relabel_configs` para derrubar métrica de alta cardinalidade | [04-drop-high-cardinality](exercises/04-drop-high-cardinality/) |
| 05 | `labeldrop` | [05-labeldrop](exercises/05-labeldrop/) |
| 06 | `hashmod` sharding entre 2 Prometheus (`__tmp_`) | [06-hashmod-sharding](exercises/06-hashmod-sharding/) |
| 07 | padrão blackbox `__param_target` | [07-param-target](exercises/07-param-target/) |
| 08 | `honor_labels` e `exported_job` | [08-honor-labels](exercises/08-honor-labels/) |
| 09 | `sample_limit` | [09-sample-limit](exercises/09-sample-limit/) |
| 10 | `http_sd_configs` + `labelmap` | [10-http-sd-labelmap](exercises/10-http-sd-labelmap/) |

Soluções em [`solutions/`](solutions/); o [`test.sh`](test.sh) carrega cada uma e confere pela API.

---

## ⚠️ Pegadinhas

1. **Regex ancorada:** `regex: payment` não casa com `payments`. Use `payment.*`.
2. **`relabel_configs` ≠ `metric_relabel_configs`:** o primeiro age em **alvos** (antes do scrape; não existe `__name__`), o segundo em **séries** (depois). Drop de métrica em `relabel_configs` não faz nada (e não dá erro!).
3. **`separator` padrão é `;`**: `[a, b]` vira `"a;b"`.
4. **Labels `__*` somem** depois do relabel. Para guardar um `__meta_*`, copie para um label sem `__`.
5. **Ordem importa:** as regras rodam em sequência. Trocar `__address__` antes de copiá-lo para `__param_target` perde o valor original.
6. **Colisão silenciosa:** reescrever `instance`/remover labels até dois alvos/séries ficarem iguais mistura dados sem erro visível.
7. **`labeldrop`/`labelkeep` aceitam só `regex`** (casam com o **nome** do label). `promtool` rejeita `source_labels` ali.
8. **`labelkeep` em `relabel_configs`** também apaga `__address__`, `__metrics_path__`...: use em `metric_relabel_configs` (e inclua `__name__`, `job`, `instance`).
9. **`sample_limit` estourado = scrape inteiro perdido** (`up=0`), não "só o excesso". O limite é conferido **depois** do `metric_relabel_configs`.
10. **`scrape_timeout` > `scrape_interval`** é erro de config.
11. **`up` e `scrape_*` sempre usam os labels do alvo**, nunca os expostos (mesmo com `honor_labels: true`).
12. **Reload falho não derruba o Prometheus:** ele segue com a config anterior; `prometheus_config_last_reload_successful` vira 0.
13. **file_sd recarrega sozinho** (inotify + `refresh_interval`); `prometheus.yml` **não** (precisa de `SIGHUP` ou `POST /-/reload` com `--web.enable-lifecycle`).

---

## 🎓 Na prova PCA

**1.** Which relabeling stage can drop a metric by its `__name__` before it is stored?
- A) `relabel_configs`
- B) `metric_relabel_configs`
- C) `write_relabel_configs`
- D) `alert_relabel_configs`

<details><summary>Resposta</summary>

**B.** `metric_relabel_configs` roda depois do scrape, sobre cada série, e é onde `__name__` existe. `relabel_configs` age sobre alvos. `write_relabel_configs` também veria `__name__`, mas só filtra o que vai para o **remote write** (a série continua gravada localmente).
</details>

**2.** A target was discovered with `__address__="10.0.0.5:9100"` and no relabeling touches `instance`. What will the `instance` label be?
- A) empty
- B) `10.0.0.5`
- C) `10.0.0.5:9100`
- D) the value of `__meta_*` hostname

<details><summary>Resposta</summary>

**C.** Se `instance` não for definido no relabel, o Prometheus copia `__address__` para ele depois do relabel.
</details>

**3.** Given `source_labels: [__meta_a, __meta_b]` with values `x` and `y`, and no `separator`, what string does `regex` match against?
- A) `xy`  B) `x;y`  C) `x,y`  D) `x:y`

<details><summary>Resposta</summary>

**B.** O `separator` padrão é `;`.
</details>

**4.** A scraped target exposes `foo{job="batch"}` while the scrape config's `job_name` is `pushgw`, with default settings. What is stored?
- A) `foo{job="batch"}`
- B) `foo{job="pushgw"}` (the exposed label is dropped)
- C) `foo{job="pushgw", exported_job="batch"}`
- D) the scrape fails

<details><summary>Resposta</summary>

**C.** Com `honor_labels: false` (padrão), em conflito o label do servidor vence e o exposto é renomeado para `exported_<label>`. Com `honor_labels: true`, seria A.
</details>

**5.** Which relabel configuration keeps only targets whose `team` label is exactly `payments`?
- A) `{source_labels: [team], regex: "pay", action: keep}`
- B) `{source_labels: [team], regex: "payments", action: keep}`
- C) `{target_label: team, regex: "payments", action: keep}`
- D) `{source_labels: [team], regex: "payments", action: labelkeep}`

<details><summary>Resposta</summary>

**B.** A falha porque a regex é ancorada. C usa `target_label` (keep olha `source_labels`). D: `labelkeep` casa com **nomes** de labels.
</details>

**6.** What happens when a scrape returns more samples than `sample_limit`?
- A) Only the first N samples are stored
- B) The whole scrape is treated as failed and `up` becomes 0
- C) Prometheus drops the oldest series in the TSDB
- D) The limit is ignored for counters

<details><summary>Resposta</summary>

**B.** O scrape inteiro é descartado e marcado como falho (`up=0`, erro "sample limit exceeded"). O limite é aplicado após o `metric_relabel_configs`.
</details>

**7.** You want to split scraping of 1000 targets between 4 Prometheus servers. Which action helps?
- A) `labelmap`  B) `hashmod`  C) `keepequal`  D) `replace` with `modulus`

<details><summary>Resposta</summary>

**B.** `hashmod` com `modulus: 4` grava o balde (0-3) em um label temporário (`__tmp_hash`), e cada servidor faz `keep` do seu número.
</details>

**8.** Which statement about `file_sd_configs` is TRUE?
- A) Changes require `POST /-/reload`
- B) Files are watched and re-read automatically; `refresh_interval` is a fallback
- C) Only YAML is supported
- D) It adds `__meta_url` to each target

<details><summary>Resposta</summary>

**B.** O Prometheus observa os arquivos (inotify) e relê periodicamente. Aceita JSON e YAML. Adiciona `__meta_filepath` (`__meta_url` é do http_sd).
</details>

**9.** Which labels are removed from the target label set after relabeling?
- A) Only `__meta_*`
- B) All labels starting with `__`
- C) `job` and `instance`
- D) None

<details><summary>Resposta</summary>

**B.** Tudo com prefixo `__` (inclui `__meta_*`, `__tmp_*`, `__address__`, `__param_*`) é removido. `job` e `instance` ficam.
</details>

---

## 📝 Cola rápida

- **Pull**: Prometheus busca `<__scheme__>://<__address__><__metrics_path__>?<__param_*>` a cada `scrape_interval` (padrão 1m; timeout padrão 10s).
- **SD** gera `__address__` + `__meta_*`. static (à mão) · file_sd (arquivo, hot reload) · http_sd (JSON por HTTP, `__meta_url`) · kubernetes/consul/ec2...
- **`relabel_configs`** = alvos, antes do scrape. **`metric_relabel_configs`** = séries, depois do scrape. **`write_relabel_configs`** = só remote write.
- Actions: `replace` (padrão) · `keep`/`drop` · `keepequal`/`dropequal` · `hashmod` · `labelmap` · `labeldrop`/`labelkeep` (só `regex`, casam **nomes**) · `lowercase`/`uppercase`.
- Regex **ancorada**, RE2. `separator` padrão `;`. `replacement` padrão `$1`.
- `__*` somem depois do relabel. `instance` = `__address__` se não definido. `__tmp_*` = rascunho seu.
- Conflito de labels: `honor_labels: false` → `exported_job`; `true` → alvo vence (Pushgateway, federation).
- `up`, `scrape_duration_seconds`, `scrape_samples_scraped`, `scrape_samples_post_metric_relabeling`, `scrape_series_added`.
- Limites: `sample_limit`/`label_limit` falham o scrape; `target_limit` falha o job inteiro.
- API: `/api/v1/targets` (`discoveredLabels` x `labels`), `?state=dropped`, `?scrapePool=<job>`. UI: `/targets`, `/service-discovery`.
- Reload: `POST /-/reload` (com `--web.enable-lifecycle`) ou `SIGHUP`. Validar: `promtool check config`.

## 📚 Referências

- https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
- https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
- https://prometheus.io/docs/prometheus/latest/http_sd/
- https://prometheus.io/docs/guides/file-sd/
- https://github.com/prometheus/blackbox_exporter#prometheus-configuration
- https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus (job `kubernetes-pods`)
