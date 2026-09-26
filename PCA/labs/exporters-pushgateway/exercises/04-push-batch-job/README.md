# Exercício 04: empurre o resultado de um batch job e alerte quando ficar velho

Um backup noturno roda por 40 s e morre. Não dá tempo de o Prometheus raspá-lo, então ele **empurra** as métricas para o Pushgateway no fim.

## O que fazer

1. Empurre com `curl` (grupo `job="nightly-backup"`, `instance="db01"`):

```bash
cat <<METRICS | curl --data-binary @- http://localhost:9143/metrics/job/nightly-backup/instance/db01
# TYPE backup_duration_seconds gauge
backup_duration_seconds 42.5
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds $(date +%s)
# TYPE backup_size_bytes gauge
backup_size_bytes 52428800
METRICS
curl -s localhost:9143/metrics | grep nightly
```

2. Note as métricas extras que o Pushgateway criou sozinho: `push_time_seconds` e `push_failure_time_seconds`.
3. Escreva o alerta `PushgatewayJobStale` em `prometheus/rules/batch.yml`: dispara quando o `nightly-backup` não empurra há mais de **26 h** (um dia + folga).
4. Prove que ele funciona com `promtool test rules` (não dá para esperar 26 h!).

## Como verificar

```bash
curl -s localhost:9140/api/v1/query --data-urlencode 'query=time() - push_time_seconds'
docker run --rm -v "$PWD/solutions/prometheus:/p:ro" -w /p --entrypoint promtool prom/prometheus:v3.15.0 \
  test rules tests/batch.test.yml
#   SUCCESS
```

<details><summary>✅ Solução</summary>

```yaml
- alert: PushgatewayJobStale
  expr: time() - push_time_seconds{job="nightly-backup"} > 26 * 3600
  labels: { severity: warning }
  annotations:
    summary: "{{ $labels.job }}/{{ $labels.instance }} não empurra métricas há {{ $value | humanizeDuration }}"
```

Teste unitário ([`solutions/prometheus/tests/batch.test.yml`](../../solutions/prometheus/tests/batch.test.yml)): uma série com o push parado no instante `0` e outra que empurra toda hora; em `25h` nada dispara, em `27h` só a `db01` dispara.

`push_time_seconds` diz que o job **rodou e empurrou**, não que **deu certo**! Por isso o job também empurra `backup_last_success_timestamp_seconds` (só atualizado no sucesso), com o alerta `BackupNotSucceededRecently`:

```yaml
- alert: BackupNotSucceededRecently
  expr: time() - backup_last_success_timestamp_seconds > 26 * 3600
```

Teste ao vivo (com o gabarito no ar): empurre um sucesso "antigo" e veja o alerta disparar em http://localhost:9140/alerts:

```bash
echo 'backup_last_success_timestamp_seconds 1000000000' | curl --data-binary @- localhost:9143/metrics/job/nightly-backup/instance/db-old
```
</details>
