# 03 · Filtrar o remote write com `write_relabel_configs`

**Cenário:** o storage central cobra por série ativa. O `prom-a` deve mandar **só** `job:*` e `up`. A config quebrada manda **todo o resto**, e nada do que queremos:

```bash
./load.sh exercises/03-write-relabel/prom-a.yml prom-a
# espere ~20s
curl -s localhost:9163/api/v1/query --data-urlencode 'query=count(last_over_time({cluster="a"}[20s]))' | jq -r '.data.result[0].value[1]'
# ~880   (deveria ser 5)
curl -s localhost:9163/api/v1/query --data-urlencode 'query=last_over_time(job:http_requests:rate1m{cluster="a"}[20s])' | jq '.data.result | length'
# 0
```

**Tarefa:** consertar `write_relabel_configs` sem afetar o que o `prom-a` guarda localmente.

<details><summary>Solução</summary>

```yaml
remote_write:
  - url: http://receiver:9090/api/v1/write
    write_relabel_configs:
      - source_labels: [__name__]
        regex: 'job:.*|up'
        action: keep
```
```bash
./load.sh solutions/03-write-relabel/prom-a.yml prom-a
curl -s localhost:9163/api/v1/query --data-urlencode 'query=count(last_over_time({cluster="a"}[20s]))' | jq -r '.data.result[0].value[1]'
# 5    (3 job:* + 2 up)
curl -s localhost:9160/api/v1/query --data-urlencode 'query=count(prometheus_http_requests_total)' | jq -r '.data.result[0].value[1]'
# > 0  <- local continua com tudo
curl -s localhost:9160/api/v1/query --data-urlencode 'query=sum by (reason) (prometheus_remote_storage_samples_dropped_total)' | jq -c '.data.result'
# [{"metric":{"reason":"dropped_series"},"value":[...,"3275"]}]   <- descartadas pelo write_relabel
```
| Estágio | Afeta o TSDB local? | Afeta o remote write? |
|---|---|---|
| `metric_relabel_configs` | ✅ | ✅ (o que não foi gravado não é enviado) |
| `write_relabel_configs` | ❌ | ✅ (só aquele `remote_write`) |
</details>
