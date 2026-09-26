# 07 · Razão vazia: nível do nome × `by()` (quebre e conserte)

**Objetivo:** `job:http_requests_errors:ratio_rate5m` sai **vazio** (`got: nil`). Conserte a **regra** em [`rules.yml`](rules.yml); o teste espera `0.25`.

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/07-razao-vector-matching
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules --debug rules.test.yml | grep -A2 'errors:rate5m'   # veja os labels de cada série gravada
```

## 💡 Dica

Divisão entre vetores casa séries com **exatamente o mesmo conjunto de labels** (fora o nome). Compare os labels de `job:http_requests_errors:rate5m` e de `job:http_requests:rate5m` no `--debug`. O nível no nome (`job:`) promete o quê?

<details><summary>✅ Solução</summary>

```yaml
      - record: job:http_requests_errors:rate5m
        expr: sum by (job) (rate(http_requests_total{code=~"5.."}[5m]))
```

Com `by (job, code)`, o numerador tinha `{job, code}` e o denominador só `{job}`: sem `on()`/`group_left`, nada casa. A convenção de nomes serve justamente para isso: o nível `job` avisa quais labels a série tem. Arquivo: [`solutions/07-razao-vector-matching/rules.yml`](../../solutions/07-razao-vector-matching/rules.yml).
</details>
