# Exercício 01: CPU %, memória % e disco % com o node_exporter

O job `node` já raspa o `node-exporter:9100`. Escreva as três queries clássicas e depois transforme-as em **recording rules**.

## O que fazer

1. **CPU ocupada (%)** por instância, a partir de `node_cpu_seconds_total` (counter de segundos por `cpu` e `mode`).
2. **Memória usada (%)**, a partir de `node_memory_MemAvailable_bytes` e `node_memory_MemTotal_bytes`.
3. **Disco usado (%)** do `/`, a partir de `node_filesystem_avail_bytes` e `node_filesystem_size_bytes`.
4. Crie `prometheus/rules/node.yml` com as três como recording rules (`instance:node_cpu_utilisation:ratio_rate1m` etc., em **ratio 0–1**) e recarregue: `curl -X POST localhost:9140/-/reload`.

## Como verificar

```bash
curl -s localhost:9140/api/v1/query --data-urlencode 'query=<sua query>'
# todos entre 0 e 100
```

> 💡 **Dica 1:** `rate()` de um counter de **segundos** dá "segundos por segundo", isto é, a **fração do tempo** (0–1) em cada modo.
> 💡 **Dica 2:** use `MemAvailable`, não `MemFree`: o Linux usa memória "livre" como cache, e `MemFree` baixo é normal.
> 💡 **Dica 3:** filtre `fstype` para não pegar `tmpfs`/`overlay`.

<details><summary>✅ Solução</summary>

```promql
# CPU %
100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m])))

# memória %
100 * (1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)

# disco / %
100 * (1 - node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|overlay"}
         / node_filesystem_size_bytes{mountpoint="/",fstype!~"tmpfs|overlay"})
```

Recording rules (arquivo completo em [`solutions/prometheus/rules/lab.rules.yml`](../../solutions/prometheus/rules/lab.rules.yml)):

```yaml
groups:
  - name: node
    rules:
      - record: instance:node_cpu_utilisation:ratio_rate1m
        expr: 1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m]))
      - record: instance:node_memory_utilisation:ratio
        expr: 1 - node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes
      - record: instance:node_filesystem_utilisation:ratio
        expr: |
          1 - node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|overlay"}
            / node_filesystem_size_bytes{mountpoint="/",fstype!~"tmpfs|overlay"}
```

Por que `avail` e não `free`? `node_filesystem_free_bytes` inclui os blocos reservados ao root (~5% no ext4); `avail` é o que um usuário comum consegue usar, que é o que enche e derruba a aplicação.
</details>
