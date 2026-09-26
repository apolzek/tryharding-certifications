---
title: PCA · mapa mental com links
markmap:
  initialExpandLevel: 3
  maxWidth: 420
  colorFreezeLevel: 2
---

# PCA [🗓️ plano de estudo](../STUDY-PLAN.md) [🃏 flashcards](../flashcards/flashcards.html) [🎯 desafios](../challenges/) [📝 simulado](../exam/) [🔥 gamedays](../gamedays/) [📚 funções PromQL](../promql-functions-lab/)

## PromQL (28%) [🃏 cards](../flashcards/flashcards.html#d=promql)
### Data Types [🧪 promql-operators](../labs/promql-operators/) [🎯 selectors-and-types](../challenges/selectors-and-types/)
- instant vector (set of samples, one per series at one timestamp)
- range vector (e.g. `http_requests_total[5m]`)
- scalar (single numeric, e.g. `42` or `scalar(...)`) [📘 scalar](../promql-functions-lab/functions/scalar/)
- string (only in literal contexts)
### Selectors [🧪 promql-operators](../labs/promql-operators/) [🎯 selectors-and-types](../challenges/selectors-and-types/)
- metric name selector: `http_requests_total`
- label matchers: `{job="api", env="prod"}`
- matchers: `=`, `!=`, `=~` (regex), `!~` (negated regex)
- range selector duration units: ms, s, m, h, d, w, y (`[5m]`)
- offset modifier: `http_requests_total offset 1h` [🎯 time](../challenges/time/)
- @ modifier: `http_requests_total @ 1609746000` [🎯 time](../challenges/time/)
### Operators [🧪 promql-operators](../labs/promql-operators/) [🎯 operators](../challenges/operators/)
- arithmetic: `+ - * / % ^`
- comparison: `== != > < >= <=`, `> bool 0`
- logical/set: `and`, `or`, `unless`
- vector matching
  - `on(label)` / `ignoring(label)`
  - `group_left` / `group_right` (many-to-one / one-to-many)
### Aggregation Operators [🧪 promql-operators](../labs/promql-operators/) [🎯 aggregation](../challenges/aggregation/)
- `sum`, `min`, `max`, `avg`, `count`
- `stddev`, `stdvar`
- `topk(5, ...)`, `bottomk(5, ...)`
- `count_values("ver", build_info)`, `quantile(0.9, ...)`
- grouping: `sum by (job) (...)`, `sum without (instance) (...)`
### Functions
- `rate(http_requests_total[5m])` (per-second avg, counters) [📘 rate](../promql-functions-lab/functions/rate/) [🎯 counters](../challenges/counters/)
- `irate(...)` (instant rate, last 2 samples) [📘 irate](../promql-functions-lab/functions/irate/) [🎯 counters](../challenges/counters/)
- `increase(http_requests_total[1h])` [📘 increase](../promql-functions-lab/functions/increase/) [🎯 counters](../challenges/counters/)
- `delta()`, `idelta()`, `deriv()` (gauges) [📘 delta](../promql-functions-lab/functions/delta/) [📘 idelta](../promql-functions-lab/functions/idelta/) [📘 deriv](../promql-functions-lab/functions/deriv/) [🎯 gauges](../challenges/gauges/)
- `histogram_quantile(0.95, sum by(le)(rate(http_request_duration_seconds_bucket[5m])))` [📘 histogram_quantile](../promql-functions-lab/functions/histogram_quantile/) [📘 rate](../promql-functions-lab/functions/rate/) [🎯 histograms](../challenges/histograms/)
- `predict_linear(node_filesystem_free_bytes[1h], 4*3600)` [📘 predict_linear](../promql-functions-lab/functions/predict_linear/) [🎯 gauges](../challenges/gauges/)
- `label_replace(v, "new", "$1", "src", "(.*)")`, `label_join()` [📘 label_replace](../promql-functions-lab/functions/label_replace/) [📘 label_join](../promql-functions-lab/functions/label_join/) [🎯 labels](../challenges/labels/)
- `absent(up{job="api"})`, `absent_over_time()` [📘 absent](../promql-functions-lab/functions/absent/) [📘 absent_over_time](../promql-functions-lab/functions/absent_over_time/) [🎯 absence-and-staleness](../challenges/absence-and-staleness/) [🎯 over-time](../challenges/over-time/) [🔥 alvo-sumiu](../gamedays/02-alvo-sumiu/)
- `avg_over_time()`, `max_over_time()`, `min_over_time()`, `sum_over_time()`, `count_over_time()` [📘 avg_over_time](../promql-functions-lab/functions/avg_over_time/) [📘 max_over_time](../promql-functions-lab/functions/max_over_time/) [📘 min_over_time](../promql-functions-lab/functions/min_over_time/) [📘 sum_over_time](../promql-functions-lab/functions/sum_over_time/) [📘 count_over_time](../promql-functions-lab/functions/count_over_time/) [🎯 over-time](../challenges/over-time/)
- `clamp_max()`, `clamp_min()`, `round()`, `ceil()`, `floor()` [📘 clamp_max](../promql-functions-lab/functions/clamp_max/) [📘 clamp_min](../promql-functions-lab/functions/clamp_min/) [📘 round](../promql-functions-lab/functions/round/) [📘 ceil](../promql-functions-lab/functions/ceil/) [📘 floor](../promql-functions-lab/functions/floor/)
- `vector(1)`, `scalar(...)` [📘 vector](../promql-functions-lab/functions/vector/) [📘 scalar](../promql-functions-lab/functions/scalar/)
### Counters vs Gauges in Queries [🧪 instrumentation](../labs/instrumentation/) [🎯 counters](../challenges/counters/) [🔥 counter-negativo](../gamedays/04-counter-negativo/)
- `rate()`/`increase()` only valid on counters (monotonic) [📘 rate](../promql-functions-lab/functions/rate/) [📘 increase](../promql-functions-lab/functions/increase/)
- counter reset handling (rate auto-corrects on restart to 0)
- gauges: use value directly or `delta()`/`deriv()` [📘 delta](../promql-functions-lab/functions/delta/) [📘 deriv](../promql-functions-lab/functions/deriv/) [🎯 gauges](../challenges/gauges/)

## Prometheus Fundamentals (20%) [🃏 cards](../flashcards/flashcards.html#d=fundamentals)
### Architecture
- Prometheus server: retrieval (scraper), TSDB, PromQL HTTP API (:9090) [🧪 tsdb-storage](../labs/tsdb-storage/)
- pull-based scraping over HTTP /metrics
- service discovery feeds target list
- Pushgateway (:9091) for short-lived batch jobs [🧪 exporters-pushgateway](../labs/exporters-pushgateway/)
- Alertmanager (:9093) handles alert routing [🧪 alertmanager](../labs/alertmanager/)
- exporters expose third-party metrics
- client libraries instrument app code [🧪 instrumentation](../labs/instrumentation/)
### Service Discovery [🧪 service-discovery-relabeling](../labs/service-discovery-relabeling/) [🔥 alvo-sumiu](../gamedays/02-alvo-sumiu/)
- `static_configs.targets`
- `kubernetes_sd_configs` (role: node, pod, endpoints, service, ingress)
- `consul_sd_configs`, `file_sd_configs`, `ec2_sd_configs`, `dns_sd_configs`
- `relabel_configs` (source_labels, regex, action, target_label)
- `__meta_*` labels (e.g. __meta_kubernetes_pod_label_app)
- `__address__`, `__scheme__`, `__metrics_path__`
### Scraping [🧪 service-discovery-relabeling](../labs/service-discovery-relabeling/)
- `scrape_interval` (global default 1m), `scrape_timeout`
- `job` label (from job_name) and `instance` label (from __address__)
- target /metrics endpoint exposition [🧪 instrumentation](../labs/instrumentation/) [🔥 alvo-sumiu](../gamedays/02-alvo-sumiu/)
- `honor_labels: true` (preserve exposed labels)
- `metric_relabel_configs` (drop after scrape)
### Configuration
- prometheus.yml top-level: global, scrape_configs, rule_files, alerting, remote_write/remote_read [🧪 federation-remote-write](../labs/federation-remote-write/) [🎯 alerting-expressions](../challenges/alerting-expressions/)
- `global.scrape_interval`, `global.evaluation_interval`, `global.external_labels` [🧪 federation-remote-write](../labs/federation-remote-write/)
- reload: `kill -HUP <pid>` or POST `/-/reload` (requires --web.enable-lifecycle) [🔥 reload-silencioso](../gamedays/10-reload-silencioso/)
- validate: `promtool check config prometheus.yml`
### Storage (TSDB) [🧪 tsdb-storage](../labs/tsdb-storage/)
- local TSDB under --storage.tsdb.path
- blocks (2h default), chunks, WAL (write-ahead log), head block
- `--storage.tsdb.retention.time=15d`, `--storage.tsdb.retention.size=50GB`
- remote_write (to Cortex/Thanos/Mimir), remote_read [🧪 federation-remote-write](../labs/federation-remote-write/)
- compaction of blocks
### Metric Data Format & Labels
- naming: `<namespace>_<name>_<unit>_total` (e.g. http_requests_total) [🧪 instrumentation](../labs/instrumentation/)
- base units (seconds, bytes), `_total` suffix for counters [🧪 instrumentation](../labs/instrumentation/)
- labels add dimensions; cardinality = unique label combos [🎯 labels](../challenges/labels/) [🔥 explosao-de-cardinalidade](../gamedays/03-explosao-de-cardinalidade/)
- series identity = metric name + sorted label set
### Limitations
- not for durable event logging / individual events
- cardinality explosion (high-cardinality labels like user_id) [🔥 explosao-de-cardinalidade](../gamedays/03-explosao-de-cardinalidade/)
- single-node by default; HA needs Thanos/Cortex/Mimir federation [🧪 federation-remote-write](../labs/federation-remote-write/)

## Observability Concepts (18%) [🃏 cards](../flashcards/flashcards.html#d=observability)
### Pillars of Observability
- metrics (Prometheus core)
- logs (Loki/ELK, out of scope for storage)
- traces (Tempo/Jaeger)
### Metric Types [🧪 instrumentation](../labs/instrumentation/)
- Counter (only increases, e.g. requests_total) [🎯 counters](../challenges/counters/)
- Gauge (up/down, e.g. memory_usage_bytes) [🎯 gauges](../challenges/gauges/)
- Histogram (`_bucket{le="..."}`, `_sum`, `_count`) [🎯 histograms](../challenges/histograms/)
- Summary (precomputed `{quantile="0.99"}`, `_sum`, `_count`) [🎯 histograms](../challenges/histograms/)
### Monitoring Concepts
- white-box (instrumented internals) vs black-box (probe externally) [🧪 exporters-pushgateway](../labs/exporters-pushgateway/)
- push (Pushgateway) vs pull (default scrape) [🧪 exporters-pushgateway](../labs/exporters-pushgateway/)
- USE method: Utilization, Saturation, Errors (resources) [🧪 slo-end-to-end](../labs/slo-end-to-end/)
- RED method: Rate, Errors, Duration (services) [🧪 slo-end-to-end](../labs/slo-end-to-end/)
- Four Golden Signals: latency, traffic, errors, saturation [🧪 slo-end-to-end](../labs/slo-end-to-end/)
### SLI / SLO / SLA [🧪 slo-end-to-end](../labs/slo-end-to-end/)
- SLI: measured ratio (e.g. good_requests / total_requests)
- SLO: target (e.g. 99.9% over 30d)
- SLA: contractual consequence
- error budget = 1 - SLO (e.g. 0.1%)

## Alerting & Dashboarding (18%) [🎯 alerting-expressions](../challenges/alerting-expressions/) [🃏 cards](../flashcards/flashcards.html#d=alerting)
### Alerting Rules [🧪 alertmanager](../labs/alertmanager/) [🔥 alerta-que-nunca-dispara](../gamedays/01-alerta-que-nunca-dispara/)
- rule fields: `alert`, `expr`, `for`, `labels`, `annotations`
- example: `expr: up == 0`, `for: 5m`
- states: inactive -> pending (during `for`) -> firing
- recording rules: `record: job:http_requests:rate5m`, `expr: sum by(job)(rate(...))` [📘 rate](../promql-functions-lab/functions/rate/) [🧪 recording-rules-testing](../labs/recording-rules-testing/)
- validate: `promtool check rules rules.yml` [🧪 recording-rules-testing](../labs/recording-rules-testing/)
### Alertmanager [🧪 alertmanager](../labs/alertmanager/) [🔥 ninguem-foi-paginado](../gamedays/05-ninguem-foi-paginado/)
- routing tree: `route.receiver`, `route.match`, `route.group_by`, `route.routes`
- `group_wait`, `group_interval`, `repeat_interval`
- inhibition (inhibit_rules: source_match, target_match, equal)
- silences (via UI/amtool, matchers + duration)
- receivers: email_configs, slack_configs, pagerduty_configs, webhook_configs
- deduplication across HA Alertmanager peers
- HA via `--cluster.peer` gossip
### Dashboarding [🧪 slo-end-to-end](../labs/slo-end-to-end/)
- Prometheus expression browser (:9090/graph)
- console templates (Go templating under consoles/)
- Grafana integration
  - add Prometheus data source (URL :9090)
  - panels with PromQL queries + `$__rate_interval`
- visualization: rate over counters, not raw counter values

## Instrumentation and Exporters (16%) [🃏 cards](../flashcards/flashcards.html#d=instrumentation)
### Instrumentation [🧪 instrumentation](../labs/instrumentation/)
- client libraries: client_golang, client_python, client_java, client_ruby
- direct instrumentation: prometheus.NewCounterVec(...).WithLabelValues().Inc()
- exposition format / OpenMetrics (text format, `# HELP`, `# TYPE`)
- best practices: snake_case, base units, low-cardinality labels [🔥 explosao-de-cardinalidade](../gamedays/03-explosao-de-cardinalidade/)
### Exporters [🧪 exporters-pushgateway](../labs/exporters-pushgateway/)
- node_exporter (host: node_cpu_seconds_total, node_memory_*)
- blackbox_exporter (probe modules: http_2xx, icmp, tcp_connect)
- cAdvisor (container_cpu_usage_seconds_total)
- mysqld_exporter, postgres_exporter, snmp_exporter
- custom exporter when no library + scrape /metrics
### Pushgateway [🧪 exporters-pushgateway](../labs/exporters-pushgateway/) [🔥 backup-congelado](../gamedays/06-backup-congelado/)
- use case: cron/batch job pushes before exit
- push via `echo "metric 1" | curl --data-binary @- :9091/metrics/job/<job>`
- caveats: metrics persist until deleted (stale), not for service-level, single point

