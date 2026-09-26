# Exercício 06: conserte o `honor_labels` que falta

## O problema

Empurre algo (exercício 04) e consulte:

```bash
echo 'backup_size_bytes 52428800' | curl --data-binary @- localhost:9143/metrics/job/nightly-backup/instance/db01
curl -s localhost:9140/api/v1/query --data-urlencode 'query=backup_size_bytes' | python3 -m json.tool | grep -A6 '"metric"'
```

```
"__name__": "backup_size_bytes",
"exported_instance": "db01",
"exported_job": "nightly-backup",
"instance": "pushgateway:9091",
"job": "pushgateway"
```

O Prometheus põe `job` e `instance` do **alvo raspado** (o Pushgateway). Como a série já tinha `job` e `instance`, houve **conflito**, e com o padrão `honor_labels: false` o Prometheus renomeia os labels que vieram nos dados para `exported_*`. Resultado: `backup_size_bytes{job="nightly-backup"}` não existe, e todos os batch jobs parecem ser "o pushgateway".

## O que fazer

Conserte o job `pushgateway` no `prometheus/prometheus.yml` e recarregue.

## Como verificar

```bash
curl -X POST localhost:9140/-/reload
curl -s localhost:9140/api/v1/query --data-urlencode 'query=backup_size_bytes{job="nightly-backup",instance="db01"}'
# 1 resultado; e nenhum exported_job:
curl -s localhost:9140/api/v1/query --data-urlencode 'query={exported_job!=""}'
```

<details><summary>✅ Solução</summary>

```yaml
  - job_name: pushgateway
    honor_labels: true
    static_configs:
      - targets: ['pushgateway:9091']
```

`honor_labels: true` = "em caso de conflito, **os labels que vieram nos dados vencem**". É usado para Pushgateway e **federação**. Não use em scrapes comuns: um alvo poderia se passar por outro job.

As métricas do próprio Pushgateway (`pushgateway_*`, `go_*`, `process_*`) não trazem `job`/`instance` na exposição, então continuam com `job="pushgateway"` normalmente.

Se o push não tiver `instance` na grouping key, o Pushgateway expõe `instance=""`; com `honor_labels: true`, label vazio = label ausente, e a série fica **sem** `instance` (não herda `pushgateway:9091`). Isso é o desejado: o dado não é "da instância do Pushgateway".
</details>
