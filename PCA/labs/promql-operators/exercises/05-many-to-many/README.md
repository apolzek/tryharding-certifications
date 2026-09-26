# 05 · many-to-one e many-to-many

**Contexto:** `sort_desc_http_requests_total{service, code}` tem 2 séries por serviço (200 e 500).
Taxas (5m): 200 → cart 99, checkout 95, payments 88, recommendations 0 · 500 → 1, 5, 12, 0.

```promql
# a) fração de CADA série (service × code) no total do seu serviço   -> desafio 233
rate(sort_desc_http_requests_total[5m]) / on(service) ___ sum by(service)(rate(sort_desc_http_requests_total[5m]))

# b) esta query dá "many-to-many matching not allowed". Conserte para dividir
#    cada série pela taxa de 200 do mesmo serviço                     -> desafio 237
rate(sort_desc_http_requests_total[5m]) / on(service) rate(sort_desc_http_requests_total[5m])
```

```bash
cd ../../../../challenges && ./check.py 233 'sua query'   # idem 237
```

💡 **Dica:** pelo menos **um** lado precisa ter 1 série por chave. Quem tem **mais** séries define se é `group_left` ou `group_right`.

<details><summary>Solução</summary>

```promql
rate(sort_desc_http_requests_total[5m]) / on(service) group_left sum by(service)(rate(sort_desc_http_requests_total[5m]))
rate(sort_desc_http_requests_total[5m]) / on(service) group_left rate(sort_desc_http_requests_total{code="200"}[5m])
```

- a) cart 0.99/0.01, checkout 0.95/0.05, payments 0.88/0.12, recommendations NaN (0/0).
- b) filtrando `{code="200"}` a direita vira "one"; aí `group_left` resolve o many-to-one.
</details>
