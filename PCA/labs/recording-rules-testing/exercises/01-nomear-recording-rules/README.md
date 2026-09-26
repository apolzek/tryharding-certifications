# 01 · Escreva e nomeie recording rules (complete)

**Objetivo:** complete [`rules.yml`](rules.yml) com duas recording rules que façam o teste [`rules.test.yml`](rules.test.yml) (já correto, **não mexa**) passar:

1. requisições por segundo **por job** (média de 5m);
2. requisições por segundo **por job e code** (média de 5m).

Os nomes têm que seguir a convenção `nível:métrica:operações` (o teste já diz quais são).

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/01-nomear-recording-rules
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 check rules rules.yml
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
../../lint-names.sh rules.yml
```

Hoje: `got: nil` (a série esperada não existe) e o lint reclama de `http_requests_per_second`.

## 💡 Dica

- **nível** = os labels que **sobram** depois da agregação, unidos por `_` (`job`, `job_code`, `instance_mode`...).
- **métrica** = o nome original **sem `_total`** quando você aplica `rate`/`increase`.
- **operações** = o que foi feito, mais recente primeiro (`rate5m`, `ratio_rate5m`, `sum_rate5m`...).

<details><summary>✅ Solução</summary>

```yaml
groups:
  - name: ex01
    rules:
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
      - record: job_code:http_requests:rate5m
        expr: sum by (job, code) (rate(http_requests_total[5m]))
```

`sum(...)` sem `by` junta **tudo** numa série sem labels: `{job="api"}` e `{job="web"}` viram um só número (5). Arquivo: [`solutions/01-nomear-recording-rules/rules.yml`](../../solutions/01-nomear-recording-rules/rules.yml).
</details>
