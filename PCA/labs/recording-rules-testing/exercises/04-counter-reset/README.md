# 04 · Counter reset: `sum` antes de `rate` (quebre e conserte)

**Objetivo:** o worker `b` reinicia em t=6m e o counter dele volta a zero. A regra em [`rules.yml`](rules.yml) devolve **3.25** jobs/s quando o real é **2**. Conserte a **regra** (o teste está certo e também prova, com `resets()`, que houve exatamente 1 reset).

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/04-counter-reset
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
#   exp: {__name__="job:jobs_processed:rate5m", job="worker"} 2E+00
#   got: {__name__="job:jobs_processed:rate5m", job="worker"} 3.25E+00
```

## 💡 Dica

`rate()` detecta reset **por série** (valor caiu = reiniciou). Se você soma primeiro, a queda de `b` vira uma queda da **soma**, e o `rate` interpreta isso como se a soma inteira tivesse zerado.

Notação útil: `'0+60x5 60+60x14'` = 6 amostras subindo de 60 em 60 (0..300), depois recomeça em 60.

<details><summary>✅ Solução</summary>

```yaml
      - record: job:jobs_processed:rate5m
        expr: sum by (job) (rate(jobs_processed_total[5m]))
```

Regra de ouro: **rate primeiro, agregação depois**. Arquivo: [`solutions/04-counter-reset/rules.yml`](../../solutions/04-counter-reset/rules.yml).
</details>
