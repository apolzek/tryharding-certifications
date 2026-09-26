# 02 · offset e subquery

**Contexto:** você quer saber quanto um counter cresceu na última hora **sem** `increase()`, e achar o
**pico** de uma taxa que tem rajadas curtas (a rota `/api/search` de `rate_http_requests_total` tem
~5 req/s com picos de ~40 req/s por 60 s a cada 5 min).

```promql
# a) crescimento de increase_http_requests_total na última hora, por code   -> desafio 208
increase_http_requests_total - increase_http_requests_total ___

# b) maior rate[1m] da rota /api/search nos últimos 10 min, avaliado a cada 30s   -> desafio 211
max_over_time(rate(rate_http_requests_total{route="/api/search"}[1m])___)

# c) em quantos pontos (1/min, últimos 30 min) essa taxa passou de 30 req/s   -> desafio 212
count_over_time((rate(rate_http_requests_total{route="/api/search"}[1m]) ___)[30m:1m])
```

```bash
cd ../../../../challenges && ./check.py 208 'sua query'   # idem 211, 212
```

💡 **Dica:** `offset` vem colado no seletor. Subquery = `<expressão>[<range>:<resolução>]`.

<details><summary>Solução</summary>

```promql
increase_http_requests_total - increase_http_requests_total offset 1h
max_over_time(rate(rate_http_requests_total{route="/api/search"}[1m])[10m:30s])
count_over_time((rate(rate_http_requests_total{route="/api/search"}[1m]) > 30)[30m:1m])
```

- a) ≈ 7200 (200) e ≈ 360 (500). Diferente de `increase()`, não compensa reset de counter.
- b) ≈ 40. `rate(...[10m])` daria ≈ 12 (a média dilui o pico) e `max_over_time(counter[10m])` pega o valor acumulado.
- c) o filtro dentro da subquery remove os pontos abaixo de 30; `count_over_time` conta o que sobrou.
</details>
