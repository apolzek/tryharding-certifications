# 08 · `honor_labels`: conflito de `job` e o `exported_job`

**Cenário:** o `legacy-batch` expõe métricas que **já trazem** `job="nightly-batch"` (é o que acontece com Pushgateway e com `/federate`). Os dashboards filtram por `job="nightly-batch"`, mas nada aparece. E o alvo ainda está `down`.

```bash
./load.sh exercises/08-honor-labels/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=batch' | jq -r '.data.activeTargets[] | "\(.scrapeUrl) \(.health) \(.lastError)"'
# http://legacy-batch:8000/metrics down server returned HTTP status 404 Not Found
```
Consertado o 404 (e só ele), você verá `batch_records_processed_total{job="batch", exported_job="nightly-batch"}`: com `honor_labels: false` (padrão) o label do **servidor** vence e o do alvo é renomeado para `exported_<nome>`.

**Tarefa:** alvo `up` e a série com `job="nightly-batch"` sem `exported_job`.

<details><summary>Solução</summary>

```yaml
  - job_name: batch
    metrics_path: /admin/metrics
    honor_labels: true
    static_configs:
      - targets: [legacy-batch:8000]
```
```bash
./load.sh solutions/08-honor-labels/prometheus.yml
curl -s localhost:9120/api/v1/query --data-urlencode 'query=batch_records_processed_total' | jq -c '.data.result[].metric'
# {"__name__":"batch_records_processed_total","instance":"legacy-batch:8000","job":"nightly-batch"}
curl -s localhost:9120/api/v1/query --data-urlencode 'query=up{job="batch"}' | jq -r '.data.result[0].value[1]'
# 1   <- up/scrape_* sempre usam os labels do alvo, nunca os expostos
```
Use `honor_labels: true` **só** onde o alvo é "autoridade" sobre os labels (Pushgateway, federation). Em alvos comuns, deixaria qualquer app sobrescrever `job`/`instance`.
</details>
