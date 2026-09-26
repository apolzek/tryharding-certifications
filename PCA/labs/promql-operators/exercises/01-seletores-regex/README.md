# 01 · Seletores e regex ancorada

**Contexto:** o time de SRE quer ver o counter de CPU (`sort_desc_container_cpu_usage_seconds_total{namespace, pod}`)
só de alguns pods, e as requisições (`label_join_http_requests_total`) das séries que **não** têm região.

Complete as lacunas `___` e confira cada resposta com o corretor:

```promql
# a) pods cujo nome COMEÇA com "api-"                     -> desafio 201
sort_desc_container_cpu_usage_seconds_total{pod=~"___"}

# b) todos os namespaces MENOS kube-system e batch (um matcher só)   -> desafio 202
sort_desc_container_cpu_usage_seconds_total{namespace___"kube-system|batch"}

# c) séries SEM o label region                              -> desafio 203
label_join_http_requests_total{region___}
```

```bash
cd ../../../../challenges
./check.py 201 'sua query'
./check.py 202 'sua query'
./check.py 203 'sua query'
```

💡 **Dica:** regex no PromQL é **ancorada** (`=~"api-"` = `^api-$`). E para o Prometheus, label vazio = label ausente.

<details><summary>Solução</summary>

```promql
sort_desc_container_cpu_usage_seconds_total{pod=~"api-.*"}
sort_desc_container_cpu_usage_seconds_total{namespace!~"kube-system|batch"}
label_join_http_requests_total{region=""}
```

- a) sem o `.*` final nada casa (resultado vazio, sem erro — a pior pegadinha).
- b) `!~` = "não casa com a regex"; `!=` só aceitaria um valor.
- c) `{region=""}` seleciona quem **não tem** o label (as 2 séries de staging).
</details>
