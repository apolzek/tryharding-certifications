# 01 · Recording rules de SLI (várias janelas)

**Objetivo:** escrever as regras que calculam a **razão de erro** do checkout em cada janela usada pelos alertas de burn rate.

- SLO: **99,9%** das requisições sem 5xx em 30 dias.
- Métrica: `http_requests_total{job="checkout", code}` (counter). Pode haver **várias instâncias** e **outros jobs** no mesmo Prometheus.
- Nome: `job:slo_errors_per_request:ratio_rate<janela>` (convenção `nível:métrica:operação`).

Complete o [`rules.yml`](rules.yml) (janelas 5m, 30m, 1h, 6h; desafio: 2h, 1d, 3d).

## 🎯 Critério de pronto

```bash
cd exercises/01-sli-recording-rules
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
# SUCCESS
```

## 💡 Dica

"Ruim / total", **rate primeiro, soma depois**, e o `by (job)` dos dois lados para o `/` casar. O filtro do job vai **nos dois** seletores.

<details><summary>✅ Solução</summary>

```yaml
- record: job:slo_errors_per_request:ratio_rate1h
  expr: |
    sum by (job) (rate(http_requests_total{job="checkout",code=~"5.."}[1h]))
    /
    sum by (job) (rate(http_requests_total{job="checkout"}[1h]))
```

- `code=~"5.."`: 4xx normalmente é erro **do cliente** e não conta contra o SLO (decisão de produto: documente!).
- Por que `rate` e não `increase`? Os dois dão a mesma razão; `rate` é a convenção para regras.
- Por que não `avg_over_time(job:...:ratio_rate5m[1h])`? Média de razões ≠ razão das somas: uma hora de pouco tráfego pesaria igual a uma de pico.

Arquivo completo: [`solutions/01-sli-recording-rules.yml`](../../solutions/01-sli-recording-rules.yml).
</details>
