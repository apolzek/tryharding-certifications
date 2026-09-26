# 10 · `http_sd_configs` + `labelmap`

**Cenário:** um serviço interno (`sd-server`) publica a lista de alvos em JSON (mesmo formato do file_sd). Queremos descobrir os alvos por HTTP e transformar `__meta_datacenter` e `__meta_owner_team` em labels `datacenter` e `owner_team`.

```bash
curl -s localhost:9127/targets.json | jq .
./load.sh exercises/10-http-sd-labelmap/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=http-sd' | jq '.data.activeTargets | length'
# 0
docker compose logs prometheus | grep 'Unable to refresh' | tail -1
# ... discovery=http config=http-sd err="server returned HTTP status 404 Not Found"
```

**Tarefa:** 2 alvos `up` com `datacenter="sa-east-1a"` e `owner_team`, e **sem** um label `url` indesejado.

<details><summary>Solução</summary>

```yaml
    http_sd_configs:
      - url: http://sd-server:8000/targets.json
        refresh_interval: 15s
    relabel_configs:
      - action: labelmap
        regex: __meta_(datacenter|owner_team)
```
O `http_sd` adiciona sozinho `__meta_url` (a URL consultada). Com `regex: __meta_(.+)` ele viraria o label `url="http://sd-server..."` em todas as séries.

```bash
./load.sh solutions/10-http-sd-labelmap/prometheus.yml
curl -s localhost:9120/api/v1/query --data-urlencode 'query=up{job="http-sd"}' | jq -c '.data.result[].metric'
# {"__name__":"up","datacenter":"sa-east-1a","env":"prod","instance":"checkout-api:8000","job":"http-sd","owner_team":"checkout"}
```
Regras do http_sd: resposta `200`, `Content-Type: application/json`, corpo = lista de `{targets, labels}`, UTF-8. Se a atualização falhar, o Prometheus **mantém** a última lista boa e incrementa `prometheus_sd_http_failures_total`.
</details>
