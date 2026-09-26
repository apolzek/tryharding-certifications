# 06 · Agregações sem armadilhas

```promql
# a) soma das durações de sort_cronjob_last_duration_seconds IGNORANDO o NaN do "sync"   -> desafio 245
sum(sort_cronjob_last_duration_seconds ___)

# b) soma de sort_by_label_desc_kafka_log_size_bytes removendo SÓ o label partition
#    (job, instance, env e topic continuam)                                               -> desafio 248
sum ___(partition)(sort_by_label_desc_kafka_log_size_bytes)

# c) taxa de erro GLOBAL (500 / tudo) de sort_desc_http_requests_total, rate 5m, 1 série  -> desafio 249
___(rate(sort_desc_http_requests_total{code="500"}[5m])) / ___(rate(sort_desc_http_requests_total[5m]))
```

```bash
cd ../../../../challenges && ./check.py 245 'sua query'   # idem 248, 249
```

💡 **Dica:** `sum` propaga NaN; `by` mantém só o que você listou, `without` remove só o que você listou;
e "média de médias" não é a média global.

<details><summary>Solução</summary>

```promql
sum(sort_cronjob_last_duration_seconds >= 0)
sum without(partition)(sort_by_label_desc_kafka_log_size_bytes)
sum(rate(sort_desc_http_requests_total{code="500"}[5m])) / sum(rate(sort_desc_http_requests_total[5m]))
```

- a) **173** (120 + 8 + 45). `x == x` também serve. Sem filtro: NaN.
- b) 78 GiB, mantendo `topic`, `job`, `instance`, `env`. `sum by(topic)` jogaria fora os labels de alvo.
- c) 18 / 300 = **0.06**. `avg(razão por serviço)` dá NaN (recommendations 0/0) e pesa igual serviços com tráfego diferente.
</details>
