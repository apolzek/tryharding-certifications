# 04 · PROCV: `group_left(label)`

**Contexto:** duas situações clássicas de "JOIN" em produção.

1. `info_http_server_requests_total{route}` precisa ganhar o label `version` que está em
   `target_info{version, k8s_cluster_name}` (valor 1, mesmo `job`/`instance`) — padrão OpenTelemetry.
2. `label_replace_node_cpu_usage_percent{endpoint="10.0.0.5:9100"}` só tem o IP; o nome do nó está em
   `label_replace_kube_node_info{internal_ip="10.0.0.5", node="worker-1"}`.

```promql
# a) -> desafio 234
info_http_server_requests_total * on(___) group_left(___) target_info

# b) -> desafio 236
label_replace(label_replace_node_cpu_usage_percent, "internal_ip", "$1", "endpoint", "___") * on(___) group_left(___) label_replace_kube_node_info
```

```bash
cd ../../../../challenges && ./check.py 234 'sua query'   # idem 236
```

💡 **Dica:** `on(...)` = a chave do PROCV (tem que existir com o mesmo nome **e** valor dos dois lados).
`group_left(...)` = as colunas que você quer **trazer** do lado "one".

<details><summary>Solução</summary>

```promql
info_http_server_requests_total * on(job, instance) group_left(version) target_info
label_replace(label_replace_node_cpu_usage_percent, "internal_ip", "$1", "endpoint", "(.*):.*") * on(internal_ip) group_left(node) label_replace_kube_node_info
```

- Multiplicar por uma info metric (valor 1) não muda o valor — só adiciona labels.
- Em b), a regex `(.*):.*` é ancorada e o `$1` captura o IP sem a porta. Sem essa chave comum, o JOIN não acha par e o resultado é vazio.
</details>
