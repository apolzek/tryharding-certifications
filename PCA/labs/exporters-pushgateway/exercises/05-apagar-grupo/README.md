# Exercício 05: apague um grupo (e entenda PUT × POST)

## A pegadinha "o último valor fica para sempre"

O servidor `db-old` foi desligado mês passado, mas o último push dele continua no Pushgateway. O Prometheus raspa esse valor **a cada 5 s, para sempre**, como se fosse fresco. O Pushgateway **não expira** nada.

```bash
echo 'backup_last_success_timestamp_seconds 1000000000' | curl --data-binary @- localhost:9143/metrics/job/nightly-backup/instance/db-old
curl -s localhost:9143/api/v1/metrics | python3 -m json.tool | grep -E '"(job|instance)"'
```

## O que fazer

1. Apague **só** o grupo `{job="nightly-backup", instance="db-old"}` com a API.
2. Confirme que sumiu do Pushgateway e, após o próximo scrape, do Prometheus.
3. Empurre para `db01` com **POST** duas métricas; depois com **PUT** só uma. O que aconteceu com a outra?

## Como verificar

```bash
curl -s localhost:9143/api/v1/metrics | grep -c db-old     # 0
curl -s localhost:9140/api/v1/query --data-urlencode 'query={instance="db-old"}'   # "result":[]
```

<details><summary>✅ Solução</summary>

```bash
# apaga o grupo inteiro (todas as métricas com essa grouping key)
curl -X DELETE http://localhost:9143/metrics/job/nightly-backup/instance/db-old

# apaga TODOS os grupos (só com --web.enable-admin-api no pushgateway)
# curl -X PUT http://localhost:9143/api/v1/admin/wipe
```

Também dá para apagar pela UI (http://localhost:9143, botão "Delete Group").

| Método | Efeito no grupo |
|---|---|
| `POST /metrics/job/X/...` | substitui só as métricas **com o mesmo nome** que vieram no push; as outras do grupo ficam |
| `PUT /metrics/job/X/...` | substitui o **grupo inteiro** (o que não veio no push é apagado) |
| `DELETE /metrics/job/X/...` | apaga o grupo inteiro |

```bash
printf 'a_metric 1\nb_metric 2\n' | curl --data-binary @- localhost:9143/metrics/job/nightly-backup/instance/db01    # POST
echo 'a_metric 10' | curl -X PUT --data-binary @- localhost:9143/metrics/job/nightly-backup/instance/db01          # PUT
curl -s localhost:9143/metrics | grep -E '^(a|b)_metric'
# a_metric{instance="db01",job="nightly-backup"} 10      <- b_metric sumiu
```

Depois do `DELETE`, o Prometheus marca as séries como **stale** no scrape seguinte, e elas somem das queries instantâneas na hora (não esperam os 5 min de lookback).

Boas práticas: o próprio job (ou o processo de descomissionamento) faz o `DELETE`; alertas em `push_time_seconds` detectam grupos abandonados.
</details>
