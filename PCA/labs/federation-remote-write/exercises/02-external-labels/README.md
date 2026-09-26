# 02 · `external_labels`: quem é quem fora do cluster

**Cenário:** alguém criou o `prom-b` copiando a config do `prom-a` e esqueceu de trocar `cluster: a`. Localmente tudo parece normal (external labels **não** aparecem nas queries locais). Mas no receiver...

```bash
./load.sh exercises/02-external-labels/prom-b.yml prom-b
# espere ~30s
curl -s localhost:9163/api/v1/query --data-urlencode 'query=count by (cluster) (last_over_time(job:http_requests:rate1m[20s]))' | jq -c '.data.result[] | [.metric, .value[1]]'
# [{"cluster":"a"},"1"]          <- cadê o cluster b?
docker compose logs receiver 2>&1 | grep -m1 'duplicate sample'
# ... msg="Out of order sample from remote write" err="duplicate sample for timestamp ...; overrides not allowed:
#     existing 18.9, new value 7.5" series="{__name__=\"job:http_requests:rate1m\", cluster=\"a\", job=\"app\"}"
curl -s localhost:9161/api/v1/query --data-urlencode 'query=rate(prometheus_remote_storage_samples_failed_total[1m])' | jq -r '.data.result[0].value[1]'
# > 0   <- prom-b está tendo escritas rejeitadas
```
As séries `job:http_requests:rate1m{job="app"}` de a e b ficaram **com os mesmos labels**. O receiver aceita uma e rejeita a outra (ou, se os timestamps não coincidirem, **mistura os valores** na mesma série). No `global` o mesmo acontece: as duas chegam como `cluster="a"`.

**Tarefa:** conserte o `prom-b` para cada cluster ser distinguível no receiver e no global.

<details><summary>Solução</summary>

```yaml
global:
  external_labels:
    cluster: b
```
```bash
./load.sh solutions/02-external-labels/prom-b.yml prom-b
curl -s localhost:9163/api/v1/query --data-urlencode 'query=job:http_requests:rate1m' | jq -r '.data.result[] | "\(.metric.cluster) \(.value[1])"'
# a 18.9
# b 7.5
```
`external_labels` são adicionados a **tudo que sai** do Prometheus: `/federate`, remote write, alertas para o Alertmanager e remote read (como filtro). Em um par HA, use também um label de **réplica** (`replica: A`/`B` ou `__replica__`) para o Thanos/Mimir deduplicar; ele deve ser o **único** label diferente entre as réplicas.
</details>
