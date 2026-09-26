# 06 · `honor_labels` na federação

**Cenário:** no `global`, os painéis filtram `job:http_requests:rate1m{job="app"}` e ficaram vazios.

```bash
./load.sh exercises/06-honor-labels-federation/global.yml global
# espere ~15s
curl -s localhost:9162/api/v1/query --data-urlencode 'query=last_over_time(job:http_requests:rate1m[20s])' | jq -c '.data.result[].metric'
# {"__name__":"job:http_requests:rate1m","cluster":"a","exported_job":"app","instance":"prom-a:9090","job":"federate"}
```
Com `honor_labels: false`, o `job` do scrape do global (`federate`) vence e o original vira `exported_job`. O `instance` vazio que o `/federate` manda (`instance=""`) também é substituído por `prom-a:9090`.

**Tarefa:** preserve os labels originais.

<details><summary>Solução</summary>

```yaml
  - job_name: federate
    honor_labels: true
```
```bash
./load.sh solutions/06-honor-labels-federation/global.yml global
curl -s localhost:9162/api/v1/query --data-urlencode 'query=last_over_time(job:http_requests:rate1m[20s])' | jq -c '.data.result[].metric'
# {"__name__":"job:http_requests:rate1m","cluster":"a","job":"app"}
```
A doc da federação usa `honor_labels: true` em todos os exemplos: o Prometheus de origem é a "autoridade" sobre `job`/`instance`. (As séries antigas com `exported_job` somem da consulta instantânea após o lookback de 5 min.)
</details>
