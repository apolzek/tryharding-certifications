---
title: Prometheus Certified Associate (PCA)
markmap:
  colorFreezeLevel: 2
---

# PCA

## PromQL (28%)
### Data Types
- instant vector (set of samples, one per series at one timestamp)
- range vector (e.g. `http_requests_total[5m]`)
- scalar (single numeric, e.g. `42` or `scalar(...)`)
- string (only in literal contexts)
### Selectors
- metric name selector: `http_requests_total`
- label matchers: `{job="api", env="prod"}`
- matchers: `=`, `!=`, `=~` (regex), `!~` (negated regex)
- range selector duration units: ms, s, m, h, d, w, y (`[5m]`)
- offset modifier: `http_requests_total offset 1h`
- @ modifier: `http_requests_total @ 1609746000`
### Operators
- arithmetic: `+ - * / % ^`
- comparison: `== != > < >= <=`, `> bool 0`
- logical/set: `and`, `or`, `unless`
- vector matching
  - `on(label)` / `ignoring(label)`
  - `group_left` / `group_right` (many-to-one / one-to-many)
### Aggregation Operators
- `sum`, `min`, `max`, `avg`, `count`
- `stddev`, `stdvar`
- `topk(5, ...)`, `bottomk(5, ...)`
- `count_values("ver", build_info)`, `quantile(0.9, ...)`
- grouping: `sum by (job) (...)`, `sum without (instance) (...)`
### Functions
- `rate(http_requests_total[5m])` (per-second avg, counters)
- `irate(...)` (instant rate, last 2 samples)
- `increase(http_requests_total[1h])`
- `delta()`, `idelta()`, `deriv()` (gauges)
- `histogram_quantile(0.95, sum by(le)(rate(http_request_duration_seconds_bucket[5m])))`
- `predict_linear(node_filesystem_free_bytes[1h], 4*3600)`
- `label_replace(v, "new", "$1", "src", "(.*)")`, `label_join()`
- `absent(up{job="api"})`, `absent_over_time()`
- `avg_over_time()`, `max_over_time()`, `min_over_time()`, `sum_over_time()`, `count_over_time()`
- `clamp_max()`, `clamp_min()`, `round()`, `ceil()`, `floor()`
- `vector(1)`, `scalar(...)`
### Counters vs Gauges in Queries
- `rate()`/`increase()` only valid on counters (monotonic)
- counter reset handling (rate auto-corrects on restart to 0)
- gauges: use value directly or `delta()`/`deriv()`

## Prometheus Fundamentals (20%)
### Architecture
- Prometheus server: retrieval (scraper), TSDB, PromQL HTTP API (:9090)
- pull-based scraping over HTTP /metrics
- service discovery feeds target list
- Pushgateway (:9091) for short-lived batch jobs
- Alertmanager (:9093) handles alert routing
- exporters expose third-party metrics
- client libraries instrument app code
### Service Discovery
- `static_configs.targets`
- `kubernetes_sd_configs` (role: node, pod, endpoints, service, ingress)
- `consul_sd_configs`, `file_sd_configs`, `ec2_sd_configs`, `dns_sd_configs`
- `relabel_configs` (source_labels, regex, action, target_label)
- `__meta_*` labels (e.g. __meta_kubernetes_pod_label_app)
- `__address__`, `__scheme__`, `__metrics_path__`
### Scraping
- `scrape_interval` (global default 1m), `scrape_timeout`
- `job` label (from job_name) and `instance` label (from __address__)
- target /metrics endpoint exposition
- `honor_labels: true` (preserve exposed labels)
- `metric_relabel_configs` (drop after scrape)
### Configuration
- prometheus.yml top-level: global, scrape_configs, rule_files, alerting, remote_write/remote_read
- `global.scrape_interval`, `global.evaluation_interval`, `global.external_labels`
- reload: `kill -HUP <pid>` or POST `/-/reload` (requires --web.enable-lifecycle)
- validate: `promtool check config prometheus.yml`
### Storage (TSDB)
- local TSDB under --storage.tsdb.path
- blocks (2h default), chunks, WAL (write-ahead log), head block
- `--storage.tsdb.retention.time=15d`, `--storage.tsdb.retention.size=50GB`
- remote_write (to Cortex/Thanos/Mimir), remote_read
- compaction of blocks
### Metric Data Format & Labels
- naming: `<namespace>_<name>_<unit>_total` (e.g. http_requests_total)
- base units (seconds, bytes), `_total` suffix for counters
- labels add dimensions; cardinality = unique label combos
- series identity = metric name + sorted label set
### Limitations
- not for durable event logging / individual events
- cardinality explosion (high-cardinality labels like user_id)
- single-node by default; HA needs Thanos/Cortex/Mimir federation

## Observability Concepts (18%)
### Pillars of Observability
- metrics (Prometheus core)
- logs (Loki/ELK, out of scope for storage)
- traces (Tempo/Jaeger)
### Metric Types
- Counter (only increases, e.g. requests_total)
- Gauge (up/down, e.g. memory_usage_bytes)
- Histogram (`_bucket{le="..."}`, `_sum`, `_count`)
- Summary (precomputed `{quantile="0.99"}`, `_sum`, `_count`)
### Monitoring Concepts
- white-box (instrumented internals) vs black-box (probe externally)
- push (Pushgateway) vs pull (default scrape)
- USE method: Utilization, Saturation, Errors (resources)
- RED method: Rate, Errors, Duration (services)
- Four Golden Signals: latency, traffic, errors, saturation
### SLI / SLO / SLA
- SLI: measured ratio (e.g. good_requests / total_requests)
- SLO: target (e.g. 99.9% over 30d)
- SLA: contractual consequence
- error budget = 1 - SLO (e.g. 0.1%)

## Alerting & Dashboarding (18%)
### Alerting Rules
- rule fields: `alert`, `expr`, `for`, `labels`, `annotations`
- example: `expr: up == 0`, `for: 5m`
- states: inactive -> pending (during `for`) -> firing
- recording rules: `record: job:http_requests:rate5m`, `expr: sum by(job)(rate(...))`
- validate: `promtool check rules rules.yml`
### Alertmanager
- routing tree: `route.receiver`, `route.match`, `route.group_by`, `route.routes`
- `group_wait`, `group_interval`, `repeat_interval`
- inhibition (inhibit_rules: source_match, target_match, equal)
- silences (via UI/amtool, matchers + duration)
- receivers: email_configs, slack_configs, pagerduty_configs, webhook_configs
- deduplication across HA Alertmanager peers
- HA via `--cluster.peer` gossip
### Dashboarding
- Prometheus expression browser (:9090/graph)
- console templates (Go templating under consoles/)
- Grafana integration
  - add Prometheus data source (URL :9090)
  - panels with PromQL queries + `$__rate_interval`
- visualization: rate over counters, not raw counter values

## Instrumentation and Exporters (16%)
### Instrumentation
- client libraries: client_golang, client_python, client_java, client_ruby
- direct instrumentation: prometheus.NewCounterVec(...).WithLabelValues().Inc()
- exposition format / OpenMetrics (text format, `# HELP`, `# TYPE`)
- best practices: snake_case, base units, low-cardinality labels
### Exporters
- node_exporter (host: node_cpu_seconds_total, node_memory_*)
- blackbox_exporter (probe modules: http_2xx, icmp, tcp_connect)
- cAdvisor (container_cpu_usage_seconds_total)
- mysqld_exporter, postgres_exporter, snmp_exporter
- custom exporter when no library + scrape /metrics
### Pushgateway
- use case: cron/batch job pushes before exit
- push via `echo "metric 1" | curl --data-binary @- :9091/metrics/job/<job>`
- caveats: metrics persist until deleted (stale), not for service-level, single point

