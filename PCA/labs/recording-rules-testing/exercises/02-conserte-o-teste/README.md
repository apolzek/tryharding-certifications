# 02 · Conserte um teste unitário que falha (quebre e conserte)

**Objetivo:** a regra em [`rules.yml`](rules.yml) está **certa**. O teste [`rules.test.yml`](rules.test.yml) tem **2 erros de raciocínio**. Conserte o **teste**.

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/02-conserte-o-teste
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
#   FAILED:
#     expr: "job:http_requests:rate5m", time: 10m,
#         exp: {__name__="job:http_requests:rate5m", instance="a", job="api"} 9E+01
#         got: {__name__="job:http_requests:rate5m", job="api"} 1.5E+00
```

Leia a saída: `exp` é o que o teste espera, `got` é o que a regra produziu.

## 💡 Dica

- `'0+60x20'` com `interval: 1m` = sobe 60 **por minuto**. Em que unidade o `rate` responde?
- O que o `sum by (job)` faz com o label `instance`?

<details><summary>✅ Solução</summary>

```yaml
        exp_samples:
          - labels: 'job:http_requests:rate5m{job="api"}'   # sem instance: o by (job) remove
            value: 1.5                                     # 60/min + 30/min = 1/s + 0.5/s
```

Lição: quando o teste falha, primeiro pergunte **quem está errado**: a regra ou a expectativa? Arquivo: [`solutions/02-conserte-o-teste/rules.test.yml`](../../solutions/02-conserte-o-teste/rules.test.yml).
</details>
