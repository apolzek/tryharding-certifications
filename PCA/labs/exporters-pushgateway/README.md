# Lab: Exporters (node, blackbox, textfile) e Pushgateway

> **Em uma frase:** quando você **não pode** colocar `/metrics` dentro do código, um **exporter** traduz o mundo (kernel, HTTP, DNS, bancos) para métricas Prometheus. E quando o processo **morre antes de ser raspado**, ele deixa um recado no **Pushgateway**.

| | |
|---|---|
| **Domínio da prova** | Instrumentation and Exporters (16%) + Prometheus Fundamentals (20%) |
| **Portas** | Prometheus http://localhost:9140 · node_exporter http://localhost:9141 · blackbox http://localhost:9142 · Pushgateway http://localhost:9143 |
| **Versões** | `prom/prometheus:v3.15.0` · `prom/node-exporter:v1.12.1` · `prom/blackbox-exporter:v0.28.0` · `prom/pushgateway:v1.11.3` |
| **Teste** | `./test.sh` (sobe o enunciado, mostra o bug do `honor_labels`, sobe o gabarito e verifica os 7 exercícios, ~1m30) |

---

## 🧠 Analogia: o intérprete, o cliente oculto e a caixa de recados

- **Exporter = intérprete.** O MySQL, o kernel Linux e o roteador não falam "Prometheus". O exporter fica ao lado, pergunta a eles na língua deles (`SHOW STATUS`, `/proc`, SNMP) e traduz para `/metrics`. O Prometheus continua **puxando** (pull), só que do intérprete.
- **node_exporter = o check-up da máquina**: CPU, memória, disco, rede. Um intérprete **por máquina**.
- **Textfile collector = um bilhete no quadro de avisos do check-up**: scripts que não têm porta própria deixam um arquivo `.prom` e o node_exporter lê junto.
- **blackbox_exporter = o cliente oculto**: não sabe nada por dentro do sistema; só entra pela porta da frente (HTTP, TCP, ICMP, DNS) e anota "fui atendido? em quanto tempo? o certificado vence quando?".
- **Pushgateway = a caixa de recados da portaria.** O entregador (batch job) passa às 3h da manhã, deixa o recado e vai embora. O Prometheus lê a caixa quando passa. Mas **ninguém tira o recado da caixa**: se o entregador nunca mais vier, o recado velho continua lá, com cara de novo.

---

## 🏗️ Arquitetura

```
                                   ┌───────────────────────────── Prometheus :9140 ─────────────────────────────┐
                                   │ job=node      job=blackbox-*          job=pushgateway (honor_labels!)       │
                                   └──────┬──────────────┬────────────────────────┬──────────────────────────────┘
                                          │ /metrics     │ /probe?module=..&target=..            │ /metrics
                                          ▼              ▼                                        ▼
  ┌── host ─────────────┐     ┌──────────────────┐  ┌──────────────────┐                ┌──────────────────┐
  │ /proc /sys /  ──────┼────▶│ node_exporter    │  │ blackbox_exporter│                │ Pushgateway      │
  │ ./textfile/*.prom ──┼────▶│ :9141 (9100)     │  │ :9142 (9115)     │                │ :9143 (9091)     │
  └─────────────────────┘     └──────────────────┘  └───┬───┬───┬───┬──┘                └────────▲─────────┘
                                                  http  │   │   │   │ dns                         │ curl POST/PUT/DELETE
                                     prometheus:9090 ◀──┘   │   │   └──▶ 127.0.0.11 (DNS do Docker) │
                                     pushgateway:9091 ◀─────┘   │ tcp/icmp                     ┌─────┴──────────┐
                                     https-site:8443 (TLS, CA do lab)                          │ batch job      │
                                     node-exporter:9999 ✘ (quebrado de propósito)              │ (você, via curl)│
                                                                                               └────────────────┘
```

## ▶️ Como rodar

```bash
cd labs/exporters-pushgateway
docker compose up -d --wait                                   # enunciado
PROM_DIR=./solutions/prometheus docker compose up -d --wait   # gabarito (prometheus.yml + regras resolvidos)
curl -X POST localhost:9140/-/reload                          # depois de editar prometheus/prometheus.yml ou regras
docker compose down -v
./test.sh            # teste completo
KEEP=1 ./test.sh     # deixa a stack do gabarito no ar
```

O `prometheus/` é montado inteiro em `/etc/prometheus` (config + `rules/*.yml`). O `textfile/` é montado em `/textfile` no node_exporter.

---

## 🔍 Passo a passo

### 1. O que é um exporter (e quando **não** usar)

| | Instrumentação direta | Exporter |
|---|---|---|
| Quando | o código é **seu** | software de terceiros, kernel, hardware, SaaS |
| Exemplo | `client_golang` no seu serviço | `mysqld_exporter`, `node_exporter`, `snmp_exporter` |
| Métricas de negócio | ✅ | ❌ (só o que o sistema já expõe) |
| Processo extra | não | sim (sidecar, DaemonSet, ou central) |
| `up` significa | o serviço respondeu | o **exporter** respondeu (o sistema por trás pode estar fora: veja `mysql_up`, `probe_success`) |

Os exporters seguem as mesmas regras: rodar **perto** do que monitoram, coletar **na hora do scrape** (sem cache próprio), um exporter por instância do sistema (não um exporter "central" para 100 bancos, com exceção dos multi-target como blackbox/snmp).

Tipos de exporter:
- **Por host** (um por máquina): node_exporter, windows_exporter.
- **Por aplicação** (sidecar do sistema): mysqld, postgres, redis, JMX.
- **Multi-target** (um central, alvos via `?target=`): blackbox, snmp.
- **Ponte de protocolo**: statsd_exporter, graphite_exporter, collectd_exporter.

Portas: cada exporter tem uma porta padrão reservada em https://github.com/prometheus/prometheus/wiki/Default-port-allocations (Prometheus **9090**, Pushgateway **9091**, Alertmanager **9093**, node **9100**, blackbox **9115**...).

### 2. node_exporter: coletores e métricas-chave

```bash
curl -s localhost:9141/metrics | grep -c '^node_'                           # centenas de séries
curl -s localhost:9141/metrics | grep node_scrape_collector_success | head   # coletores ativos
curl -s 'localhost:9141/metrics?collect[]=loadavg&collect[]=meminfo' | grep -E '^node_(load|memory_Mem)'  # só alguns coletores
```

Coletores são ligados/desligados por flag: `--collector.systemd`, `--no-collector.wifi`, `--collector.textfile.directory=...`. No lab, `--path.rootfs=/host` + o volume `/:/host:ro,rslave` fazem o container ler o disco **do host**.

| Área | Métrica | Tipo | Query típica |
|---|---|---|---|
| CPU | `node_cpu_seconds_total{cpu,mode}` | counter | `1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))` |
| Load | `node_load1`, `node_load5`, `node_load15` | gauge | `node_load1 / count without (cpu, mode) (node_cpu_seconds_total{mode="idle"})` |
| Memória | `node_memory_MemAvailable_bytes`, `node_memory_MemTotal_bytes` | gauge | `1 - MemAvailable / MemTotal` |
| Disco (espaço) | `node_filesystem_avail_bytes`, `node_filesystem_size_bytes` | gauge | `predict_linear(node_filesystem_avail_bytes[6h], 24*3600) < 0` |
| Disco (I/O) | `node_disk_read_bytes_total`, `node_disk_io_time_seconds_total` | counter | `rate(node_disk_io_time_seconds_total[5m])` (≈ utilização) |
| Rede | `node_network_receive_bytes_total{device}` | counter | `rate(...[5m]) * 8` (bits/s) |
| Sistema | `node_boot_time_seconds`, `node_uname_info`, `node_time_seconds` | gauge | `time() - node_boot_time_seconds` (uptime) |
| Textfile | `node_textfile_scrape_error`, `node_textfile_mtime_seconds` | gauge | `node_textfile_scrape_error == 1` |

> Neste lab o node_exporter roda em container **sem** `network_mode: host`, então `node_network_*` mostra a rede do container (`eth0`, `lo`). Em produção: binário no host, ou container com `network_mode: host` e `pid: host`.

Resolva o [exercício 01](exercises/01-node-cpu-mem-disco/) (CPU %, memória %, disco %).

### 3. Textfile collector: métricas de scripts e cron

```bash
cat textfile/lab.prom
curl -s localhost:9141/metrics | grep -E '^lab_|^node_textfile'
# lab_textfile_demo_info{lab="exporters-pushgateway",owner="pca"} 1
# node_textfile_mtime_seconds{file="/textfile/lab.prom"} 1.79e+09
# node_textfile_scrape_error 0
```

Para "o que roda na máquina e não tem porta" (backup, cron, inventário de pacotes, versão do firmware). Escreva **atomicamente** (arquivo temporário + `mv`), sem timestamps nas amostras, só `*.prom`. Veja o [exercício 02](exercises/02-textfile-collector/).

Textfile × Pushgateway: se o job está **amarrado a uma máquina** (backup do host), prefira textfile (o node_exporter já dá `up` e o `instance` certo). Se o job **não tem máquina fixa** (job do Kubernetes, CI), Pushgateway.

### 4. blackbox_exporter: sondas de fora para dentro

```bash
curl -s 'localhost:9142/probe?module=http_2xx&target=http://prometheus:9090/-/healthy' | grep -E '^probe_(success|http_status_code|duration_seconds)'
# probe_duration_seconds 0.0008
# probe_http_status_code 200
# probe_success 1

curl -s 'localhost:9142/probe?module=http_2xx_lab_ca&target=https://https-site:8443/' | grep -E '^probe_(success|ssl_earliest)'
# probe_ssl_earliest_cert_expiry 2.105793359e+09      <- Unix time em que o 1º cert da cadeia vence
# probe_success 1

curl -s 'localhost:9142/probe?module=dns_pushgateway&target=127.0.0.11' | grep -E '^probe_(success|dns_answer_rrs)'
curl -s 'localhost:9142/probe?module=icmp&target=node-exporter' | grep ^probe_success
curl -s 'localhost:9142/probe?module=http_2xx&target=http://node-exporter:9999/&debug=true' | head -20   # por que falhou?
```

| Módulo (lab) | Prober | O que prova | Métricas úteis |
|---|---|---|---|
| `http_2xx` | http | responde 2xx | `probe_http_status_code`, `probe_http_duration_seconds{phase}` |
| `http_2xx_lab_ca` | http + TLS | HTTPS válido contra uma CA | `probe_ssl_earliest_cert_expiry`, `probe_tls_version_info` |
| `http_body_ok` | http | o corpo contém um regex | `probe_failed_due_to_regex` |
| `tcp_connect` | tcp | a porta aceita conexão | `probe_success` |
| `icmp` | icmp | responde ping | `probe_icmp_duration_seconds` |
| `dns_pushgateway` | dns | o servidor DNS responde o nome | `probe_dns_answer_rrs`, `probe_dns_lookup_time_seconds` |

Métricas que **toda** sonda tem: `probe_success` (0/1) e `probe_duration_seconds`.

**O padrão de relabel** (decore, cai na prova):

```yaml
  - job_name: blackbox-http
    metrics_path: /probe
    params:
      module: [http_2xx]
    static_configs:
      - targets: [http://prometheus:9090/-/healthy, http://node-exporter:9999/]
    relabel_configs:
      - source_labels: [__address__]     # 1) o alvo vira o parâmetro ?target=
        target_label: __param_target
      - source_labels: [__param_target]  # 2) e vira o label instance
        target_label: instance
      - target_label: __address__        # 3) quem é raspado de verdade é o blackbox
        replacement: blackbox:9115
```

`__param_<nome>` vira `?<nome>=` na URL do scrape. O resultado é um scrape de `http://blackbox:9115/probe?module=http_2xx&target=http://prometheus:9090/-/healthy` com `instance="http://prometheus:9090/-/healthy"`. Resolva no [exercício 03](exercises/03-blackbox-sondas/).

### 5. Pushgateway: só para batch jobs de vida curta

```bash
cat <<METRICS | curl --data-binary @- http://localhost:9143/metrics/job/nightly-backup/instance/db01
# TYPE backup_duration_seconds gauge
backup_duration_seconds 42.5
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds $(date +%s)
METRICS

curl -s localhost:9143/metrics | grep nightly
# backup_duration_seconds{instance="db01",job="nightly-backup"} 42.5
# backup_last_success_timestamp_seconds{instance="db01",job="nightly-backup"} 1.79e+09
# push_failure_time_seconds{instance="db01",job="nightly-backup"} 0
# push_time_seconds{instance="db01",job="nightly-backup"} 1.79e+09        <- criado pelo Pushgateway

curl -s localhost:9143/api/v1/metrics | python3 -m json.tool | head -30    # grupos em JSON
```

- **Grouping key** = o caminho da URL: `/metrics/job/<JOB>{/<LABEL>/<VALOR>}`. `job` é obrigatório; cada grupo é substituído/apagado como uma unidade.
- `POST` substitui as métricas de mesmo nome no grupo; `PUT` substitui o grupo inteiro; `DELETE` apaga o grupo ([exercício 05](exercises/05-apagar-grupo/)).
- `push_time_seconds` = último push **bem-sucedido** do grupo; `push_failure_time_seconds` = último push rejeitado (ex.: tipo conflitante).
- No Prometheus, o job do Pushgateway **precisa** de `honor_labels: true`, senão `job`/`instance` viram `exported_job`/`exported_instance` ([exercício 06](exercises/06-honor-labels/)).

Mesma coisa pelas bibliotecas cliente:

```python
from prometheus_client import CollectorRegistry, Gauge, push_to_gateway
reg = CollectorRegistry()
Gauge("backup_last_success_timestamp_seconds", "...", registry=reg).set_to_current_time()
push_to_gateway("localhost:9143", job="nightly-backup", grouping_key={"instance": "db01"}, registry=reg)
```

```go
push.New("http://localhost:9143", "nightly-backup").Grouping("instance", "db01").Collector(lastSuccess).Push()
```

**Por que NÃO usar Pushgateway para serviços** (a doc oficial é enfática):

1. **Ponto único de falha e gargalo**: todos os jobs passam por ele.
2. **Perde o `up`**: o Prometheus só sabe se o *Pushgateway* está vivo, não se o *job* está.
3. **Nunca esquece** ("last value stays forever"): o valor do último push é exposto para sempre, mesmo que o job tenha morrido há meses. Não há TTL. É preciso `DELETE` explícito.
4. **Não agrega**: dois pushes de instâncias diferentes para o mesmo grupo se sobrescrevem; counters não somam.
5. Timestamps: as amostras recebem o horário do **scrape**, não do push.

Uso legítimo: **batch jobs de nível de serviço** (não amarrados a uma máquina), de vida curta. Para o resto: instrumentação direta (serviços), textfile (cron amarrado a um host), ou `remote_write`/OTLP (quando o modelo push é inevitável).

---

## 🏭 Casos reais

### Caso 1: node_exporter + node-mixin (kube-prometheus)

```yaml
groups:
  - name: node-exporter
    rules:
      - alert: NodeFilesystemAlmostOutOfSpace
        expr: |
          (node_filesystem_avail_bytes{job="node",fstype!=""} / node_filesystem_size_bytes{job="node",fstype!=""} * 100 < 5
          and node_filesystem_readonly{job="node",fstype!=""} == 0)
        for: 30m
        labels: { severity: critical }
      - alert: NodeFilesystemSpaceFillingUp
        expr: |
          (node_filesystem_avail_bytes{job="node",fstype!=""} / node_filesystem_size_bytes{job="node",fstype!=""} * 100 < 15
          and predict_linear(node_filesystem_avail_bytes{job="node",fstype!=""}[6h], 4*60*60) < 0
          and node_filesystem_readonly{job="node",fstype!=""} == 0)
        for: 1h
        labels: { severity: critical }
      - alert: NodeTextFileCollectorScrapeError
        expr: node_textfile_scrape_error{job="node"} == 1
        labels: { severity: warning }
```

No Kubernetes, o node_exporter roda como **DaemonSet** com `hostNetwork: true`, `hostPID: true` e o `/` do host montado em `/host/root`.

### Caso 2: blackbox para SLO de disponibilidade externa e certificado

```yaml
scrape_configs:
  - job_name: blackbox-sites
    scrape_interval: 30s
    metrics_path: /probe
    params: { module: [http_2xx] }
    static_configs:
      - targets: [https://www.exemplo.com.br, https://api.exemplo.com.br/health]
        labels: { team: web }
    relabel_configs:
      - { source_labels: [__address__], target_label: __param_target }
      - { source_labels: [__param_target], target_label: instance }
      - { target_label: __address__, replacement: blackbox-exporter.monitoring:9115 }

rule_files: [blackbox.rules.yml]
# blackbox.rules.yml
groups:
  - name: blackbox
    rules:
      - alert: SiteDown
        expr: probe_success{job="blackbox-sites"} == 0
        for: 2m
      - alert: SSLCertExpiringSoon
        expr: probe_ssl_earliest_cert_expiry - time() < 14 * 86400
        for: 1h
      - record: job:probe_success:avg_over_time30d     # disponibilidade de 30 dias
        expr: avg_over_time(probe_success{job="blackbox-sites"}[30d])
```

No Prometheus Operator, o mesmo vira um recurso `Probe` (`spec.prober.url`, `spec.targets.staticConfig.static`, `spec.module`).

Dica de produção: rode blackbox em **mais de uma região** e alerte quando `probe_success` falhar em **várias** (`count by (instance) (probe_success == 0) >= 2`), evitando alarme falso por problema de rede do próprio probe.

### Caso 3: CronJob do Kubernetes empurrando para o Pushgateway

```yaml
apiVersion: batch/v1
kind: CronJob
metadata: { name: nightly-backup }
spec:
  schedule: "0 3 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: backup
              image: postgres:17
              command: ["/bin/sh", "-c"]
              args:
                - |
                  start=$(date +%s)
                  pg_dump "$DATABASE_URL" > /backup/db.sql || exit 1
                  cat <<EOM | curl --data-binary @- http://pushgateway.monitoring:9091/metrics/job/nightly-backup/db/orders
                  # TYPE backup_last_success_timestamp_seconds gauge
                  backup_last_success_timestamp_seconds $(date +%s)
                  # TYPE backup_duration_seconds gauge
                  backup_duration_seconds $(( $(date +%s) - start ))
                  EOM
```

Note a grouping key `db/orders` (e não o nome do pod!): cada execução **substitui** o grupo anterior em vez de criar um grupo novo por pod, que ficaria para sempre.

```yaml
# prometheus.yml
  - job_name: pushgateway
    honor_labels: true
    static_configs: [{ targets: ['pushgateway.monitoring:9091'] }]
# alerta
      - alert: NightlyBackupMissing
        expr: time() - backup_last_success_timestamp_seconds{job="nightly-backup"} > 26 * 3600
```

### Caso 4: textfile collector com cron (sem Pushgateway)

```cron
# /etc/cron.d/apt-metrics  (o node_exporter do Debian/Ubuntu já traz scripts assim)
*/15 * * * * root /usr/share/prometheus-node-exporter-collectors/apt_info.py | sponge /var/lib/prometheus/node-exporter/apt.prom
```

`sponge` (moreutils) só escreve quando a entrada termina, o que dá o mesmo efeito do "temporário + mv".

---

## 🧪 Exercícios

| # | Exercício |
|---|---|
| 01 | [CPU %, memória % e disco % com o node_exporter](exercises/01-node-cpu-mem-disco/) |
| 02 | [Métrica própria via textfile collector](exercises/02-textfile-collector/) |
| 03 | [Sondas blackbox: 2 alvos internos OK + 1 quebrado](exercises/03-blackbox-sondas/) |
| 04 | [Empurre um batch job e alerte quando `push_time_seconds` ficar velho](exercises/04-push-batch-job/) |
| 05 | [Apague um grupo (e PUT × POST)](exercises/05-apagar-grupo/) |
| 06 | [Conserte o `honor_labels` que falta](exercises/06-honor-labels/) |
| 07 | [Ecossistema de exporters e portas padrão](exercises/07-ecossistema-e-portas/) |

Gabarito: [`solutions/prometheus/`](solutions/prometheus/) (config, regras e teste de regras) e [`solutions/textfile/backup-metrics.sh`](solutions/textfile/backup-metrics.sh).

---

## ⚠️ Pegadinhas

1. **`up` do blackbox é 1 mesmo com o site fora do ar.** `up` = "o blackbox respondeu". Alerta em `probe_success == 0`.
2. **Esquecer o passo 2 do relabel** (`__param_target → instance`): todas as sondas ficam com `instance="blackbox:9115"` e colidem.
3. **`honor_labels` faltando** no job do Pushgateway: `exported_job` / `exported_instance`.
4. **Pushgateway nunca expira nada.** Job descomissionado = `DELETE` explícito. Monitore `time() - push_time_seconds`.
5. **Grouping key com valor único por execução** (pod name, run id) = um grupo novo a cada execução, acumulando para sempre.
6. **`push_time_seconds` recente ≠ job com sucesso.** Empurre um `*_last_success_timestamp_seconds` e alerte nele.
7. **Textfile:** só `*.prom`, sem timestamp, escrita atômica, e `node_textfile_scrape_error` para pegar erro de sintaxe.
8. **node_exporter em container** sem `--path.rootfs` e sem os volumes mede o **container**, não o host.
9. **`MemFree` ≠ memória disponível.** Use `MemAvailable`.
10. **`rate()` no `node_cpu_seconds_total`**: é counter; `node_cpu_seconds_total{mode="idle"}` cru não diz nada.

## 🎓 Na prova PCA

O que costuma cair:
- Quando usar Pushgateway (batch de serviço, vida curta) e quando **não** (serviços, "para atravessar firewall", "para ter push").
- `honor_labels: true` para Pushgateway e federação.
- O padrão `__param_target` / `__address__` do blackbox.
- `probe_success`, `probe_duration_seconds`, `probe_ssl_earliest_cert_expiry`.
- Textfile collector.
- Portas padrão (9090, 9091, 9093, 9100, 9115).
- Exporter × instrumentação direta.

**1.** Which use case is appropriate for the Pushgateway?
- A) A long-running web service behind a firewall
- B) A service-level batch job that runs for 30 seconds every night
- C) Replacing the pull model for all microservices
- D) Aggregating counters from many instances of a service

<details><summary>Resposta</summary>

**B.** A doc: *"the only valid use case for the Pushgateway is for capturing the outcome of a service-level batch job"*. A, C e D são antipadrões: serviços devem ser raspados, e o Pushgateway não agrega (pushes do mesmo grupo se sobrescrevem).
</details>

**2.** Metrics pushed to the Pushgateway appear in Prometheus with `exported_job="backup"` and `job="pushgateway"`. What fixes this?
- A) `honor_timestamps: true`
- B) `honor_labels: true` in the Pushgateway scrape config
- C) Pushing to `/metrics/job/pushgateway`
- D) `metric_relabel_configs` with `action: labeldrop` on `job`

<details><summary>Resposta</summary>

**B.** Com `honor_labels: true`, em caso de conflito os labels **dos dados** vencem os labels do alvo. Sem ele, os labels conflitantes dos dados são renomeados para `exported_<label>`.
</details>

**3.** In a blackbox exporter scrape config, what is the purpose of `source_labels: [__address__]` → `target_label: __param_target`?
- A) It sets the address Prometheus connects to
- B) It passes the original target as the `target` URL parameter to the blackbox exporter
- C) It renames the `instance` label
- D) It selects the blackbox module

<details><summary>Resposta</summary>

**B.** `__param_<nome>` vira parâmetro de URL (`?target=...`). O endereço de conexão (A) é trocado depois, com `__address__ = blackbox:9115`. O módulo (D) vem de `params: module: [...]`.
</details>

**4.** A blackbox HTTP probe target is down. What are `up` and `probe_success` for that target?
- A) `up=0`, `probe_success=0`
- B) `up=1`, `probe_success=0`
- C) `up=0`, `probe_success=1`
- D) `up=1`, `probe_success=1`

<details><summary>Resposta</summary>

**B.** O scrape do **blackbox** funcionou (`up=1`); a **sonda** falhou (`probe_success=0`). `up=0` só se o próprio blackbox estiver fora.
</details>

**5.** A cron script on a host needs to expose the timestamp of its last successful run. The host already runs node_exporter. What is the simplest recommended approach?
- A) Push to the Pushgateway with the hostname as grouping key
- B) Write a `.prom` file to the node_exporter textfile collector directory
- C) Start an HTTP server in the script
- D) Use remote write

<details><summary>Resposta</summary>

**B.** Textfile collector: o job é amarrado ao host, o node_exporter já é raspado, e o `instance` fica certo. A funciona, mas traz os problemas do Pushgateway (não expira, perde `up`).
</details>

**6.** What is the default port of node_exporter? And of the Pushgateway?
- A) 9100 and 9091
- B) 9090 and 9093
- C) 9115 and 9100
- D) 9091 and 9100

<details><summary>Resposta</summary>

**A.** node_exporter 9100, Pushgateway 9091. (9090 Prometheus, 9093 Alertmanager, 9115 blackbox.)
</details>

**7.** Which metric can be used to alert on a TLS certificate about to expire, using the blackbox exporter?
- A) `probe_http_ssl`
- B) `probe_ssl_earliest_cert_expiry - time()`
- C) `ssl_cert_not_after`
- D) `probe_tls_version_info`

<details><summary>Resposta</summary>

**B.** `probe_ssl_earliest_cert_expiry` é o Unix time de expiração do certificado que vence primeiro na cadeia; subtraindo `time()` temos os segundos restantes. `probe_http_ssl` só diz se foi usado SSL (0/1).
</details>

**8.** A batch job that ran on a decommissioned server pushed metrics to the Pushgateway 3 months ago. What happens to those metrics?
- A) They expire after 5 minutes (staleness)
- B) They are deleted after 24h
- C) They remain exposed and scraped indefinitely until explicitly deleted
- D) Prometheus drops them because the timestamps are old

<details><summary>Resposta</summary>

**C.** O Pushgateway não tem TTL. As amostras recebem o timestamp do **scrape**, então parecem frescas para sempre. Apague com `curl -X DELETE .../metrics/job/<job>/instance/<inst>`.
</details>

## 📝 Cola rápida

- **Exporter** = tradutor para sistemas que você não controla. Código seu → instrumentação direta.
- **node_exporter :9100**: `node_cpu_seconds_total` (counter, `rate`), `node_memory_MemAvailable_bytes`, `node_filesystem_avail_bytes`, `node_network_*_bytes_total`, `node_load1`. Textfile: `--collector.textfile.directory`, `*.prom`, escrita atômica, sem timestamp.
- **blackbox :9115**: `/probe?module=X&target=Y`; `probe_success`, `probe_duration_seconds`, `probe_http_status_code`, `probe_ssl_earliest_cert_expiry`. Relabel: `__address__→__param_target→instance`, `__address__=blackbox:9115`.
- **Pushgateway :9091**: só batch de serviço de vida curta. `/metrics/job/<j>/<label>/<valor>`. POST (mesmo nome) · PUT (grupo todo) · DELETE. `push_time_seconds`. **`honor_labels: true`**. Nunca expira.
- `up` = o alvo raspado respondeu (o exporter), não o sistema por trás.
- Portas: 9090 Prometheus · 9091 Pushgateway · 9093 Alertmanager · 9100 node · 9115 blackbox.

## 📚 Referências

- https://prometheus.io/docs/instrumenting/exporters/
- https://prometheus.io/docs/instrumenting/writing_exporters/
- https://github.com/prometheus/node_exporter#textfile-collector
- https://github.com/prometheus/blackbox_exporter/blob/master/CONFIGURATION.md
- https://prometheus.io/docs/practices/pushing/
- https://github.com/prometheus/pushgateway/blob/master/README.md
- https://github.com/prometheus/prometheus/wiki/Default-port-allocations
- https://prometheus.io/docs/guides/node-exporter/
