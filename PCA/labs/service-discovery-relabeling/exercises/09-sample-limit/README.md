# 09 · `sample_limit`: o limite que derruba o scrape inteiro

**Cenário:** para se proteger de explosões de cardinalidade, o time colocou `sample_limit: 100`. O `payments-api` passou a ficar `up=0`:

```bash
./load.sh exercises/09-sample-limit/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=payments-limited' | jq -r '.data.activeTargets[] | "\(.health) \(.lastError)"'
# down sample limit exceeded
```
Quando o limite estoura, **o scrape inteiro falha** (nenhuma amostra daquele scrape é gravada) e `up` vira 0. A métrica `prometheus_target_scrapes_exceeded_sample_limit_total` sobe.

**Tarefa:** deixar o alvo `up` **sem** aumentar o limite.

> 💡 **Dica:** o limite é verificado **depois** do `metric_relabel_configs`.

<details><summary>Solução</summary>

```yaml
    sample_limit: 100
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: app_requests_by_user_total
        action: drop
```
```bash
./load.sh solutions/09-sample-limit/prometheus.yml
curl -s localhost:9120/api/v1/query --data-urlencode 'query=scrape_samples_post_metric_relabeling{job="payments-limited"}' | jq -r '.data.result[0].value[1]'
# 5
```
Irmãos do `sample_limit`: `label_limit`, `label_name_length_limit`, `label_value_length_limit` (também falham o scrape) e `target_limit` (se o SD devolver mais alvos que o limite, **todos** os alvos do job falham).
</details>
