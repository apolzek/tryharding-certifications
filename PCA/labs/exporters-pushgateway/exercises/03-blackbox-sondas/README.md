# Exercício 03: sondas blackbox (2 alvos internos OK + 1 quebrado)

O blackbox_exporter já está no ar com módulos em [`blackbox/blackbox.yml`](../../blackbox/blackbox.yml). Teste na mão primeiro:

```bash
curl -s 'localhost:9142/probe?module=http_2xx&target=http://prometheus:9090/-/healthy' | grep -E '^probe_(success|http_status_code|duration_seconds)'
curl -s 'localhost:9142/probe?module=http_2xx&target=http://node-exporter:9999/' | grep ^probe_success
curl -s 'localhost:9142/probe?module=http_2xx&target=http://node-exporter:9999/&debug=true' | head -20   # log da sonda
```

## O que fazer

No `prometheus/prometheus.yml`, crie:

1. Job `blackbox-http` (módulo `http_2xx`) sondando:
   - `http://prometheus:9090/-/healthy` (deve dar `probe_success 1`)
   - `http://pushgateway:9091/-/healthy` (deve dar `1`)
   - `http://node-exporter:9999/` (porta fechada: deve dar `0`)
2. Bônus: `blackbox-https` (módulo `http_2xx_lab_ca`, alvo `https://https-site:8443/`), `blackbox-tcp` (`tcp_connect`), `blackbox-icmp` (`icmp`), `blackbox-dns` (`dns_pushgateway`, alvo `127.0.0.11`).
3. Um alerta `BlackboxProbeFailed` para `probe_success == 0`.

O label `instance` de cada série deve ser a **URL sondada**, não `blackbox:9115`.

## Como verificar

```bash
curl -X POST localhost:9140/-/reload
curl -s localhost:9140/api/v1/query --data-urlencode 'query=probe_success' | python3 -m json.tool | grep -E '"instance"|"1"|"0"'
curl -s localhost:9140/api/v1/query --data-urlencode 'query=(probe_ssl_earliest_cert_expiry - time()) / 86400'   # dias (≈3650)
```

> 💡 **Dica:** os `targets` do job **não são raspados**. Eles viram o parâmetro `?target=` via `relabel_configs`, e quem é raspado é o blackbox.

<details><summary>✅ Solução</summary>

```yaml
  - job_name: blackbox-http
    metrics_path: /probe
    params:
      module: [http_2xx]
    static_configs:
      - targets:
          - http://prometheus:9090/-/healthy
          - http://pushgateway:9091/-/healthy
          - http://node-exporter:9999/
    relabel_configs:
      - source_labels: [__address__]     # 1) alvo -> ?target=
        target_label: __param_target
      - source_labels: [__param_target]  # 2) alvo -> label instance
        target_label: instance
      - target_label: __address__        # 3) raspa o blackbox
        replacement: blackbox:9115
```

Os outros jobs (https/tcp/icmp/dns) e o alerta estão em [`solutions/prometheus/prometheus.yml`](../../solutions/prometheus/prometheus.yml) e [`solutions/prometheus/rules/lab.rules.yml`](../../solutions/prometheus/rules/lab.rules.yml).

Sem o passo 2, todas as séries teriam `instance="blackbox:9115"` e colidiriam. Sem o passo 3, o Prometheus tentaria raspar `http://prometheus:9090/-/healthy/probe`.

`up` do job `blackbox-http` é **1 mesmo para o alvo quebrado**: `up` diz se o *blackbox* respondeu. Quem diz se o *alvo* está bem é `probe_success`.
</details>
