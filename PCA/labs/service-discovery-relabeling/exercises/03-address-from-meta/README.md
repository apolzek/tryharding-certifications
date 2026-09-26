# 03 · Montar `__address__` a partir de `__meta_*` (o job `kubernetes-pods`)

**Cenário:** `prometheus/sd/pods.json` imita o que o `kubernetes_sd_configs` com `role: pod` entrega: um `__address__` inútil (`10.244.1.17:9999`) e vários `__meta_kubernetes_*`. A convenção (usada no chart oficial do Prometheus) é:

| Annotation no pod | Meta label | Uso |
|---|---|---|
| `prometheus.io/scrape: "true"` | `__meta_kubernetes_pod_annotation_prometheus_io_scrape` | só raspa se `true` |
| `prometheus.io/port: "8000"` | `__meta_kubernetes_pod_annotation_prometheus_io_port` | porta do scrape |
| `prometheus.io/path: /admin/metrics` | `__meta_kubernetes_pod_annotation_prometheus_io_path` | caminho (se existir) |

(Aqui o `__meta_kubernetes_pod_ip` é o nome DNS do container, para funcionar no Docker.)

```bash
./load.sh exercises/03-address-from-meta/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=kubernetes-pods' | jq -r '.data.activeTargets[] | "\(.scrapeUrl) \(.health) \(.lastError)"'
# http://checkout-api;8000/metrics down ... lookup checkout-api;8000: no such host
# http://legacy-batch;8000/metrics down ...
```

**Tarefa:** os 2 pods com `scrape=true` devem ficar `up`; o `search-api` (scrape=false) fica fora. Há **2 bugs**.

> 💡 **Dica 1:** quando há vários `source_labels`, os valores são **concatenados com `separator`**, cujo padrão é `;`.
> 💡 **Dica 2:** o `legacy-batch` tem a annotation `prometheus.io/path`. Quem controla o caminho é `__metrics_path__`.

<details><summary>Solução</summary>

```yaml
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        regex: "true"
        action: keep
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        regex: (.+)                      # não casa com string vazia: pods sem annotation mantêm /metrics
        target_label: __metrics_path__
      - source_labels: [__meta_kubernetes_pod_ip, __meta_kubernetes_pod_annotation_prometheus_io_port]
        separator: ":"
        target_label: __address__
      - action: labelmap
        regex: __meta_kubernetes_pod_label_(.+)
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
```
A forma que aparece nos charts antigos também vale: `regex: ([^:]+)(?::\d+)?;(\d+)` + `replacement: $1:$2` sobre `[__address__, ...port]`.

```bash
./load.sh solutions/03-address-from-meta/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=kubernetes-pods' | jq -c '.data.activeTargets[] | {url: .scrapeUrl, health, labels}'
# {"url":"http://checkout-api:8000/metrics","health":"up","labels":{"app":"checkout","instance":"checkout-api:8000","job":"kubernetes-pods","namespace":"shop","pod":"checkout-api-7d9f8b6c4-x2klp","tier":"backend"}}
# {"url":"http://legacy-batch:8000/admin/metrics", ... }
```
</details>
