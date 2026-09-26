# 06 · Alerta de série ausente: `absent()`, `_` e `stale` (quebre e conserte)

**Objetivo:** o job `batch` sumiu do scrape (exporter morreu e saiu do service discovery). O alerta `BatchJobMissing` em [`rules.yml`](rules.yml) **nunca** dispara. Conserte a regra para que o teste [`rules.test.yml`](rules.test.yml) passe.

O teste tem **dois** cenários, e vale a pena entendê-los:

| `values` | O que simula | A série "some" em |
|---|---|---|
| `'1 1 1 1 1 1 _x14'` | amostras param de chegar (`_` = sem amostra) | 5m depois da última (lookback delta) → 10m |
| `'1 1 1 1 1 1 stale'` | alvo saiu do scrape: Prometheus grava um **staleness marker** | imediatamente → 6m |

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/06-absent-serie-sumiu
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
```

## 💡 Dica

`up{job="batch"} == 0` compara as séries que **existem**. Se não existe nenhuma, o resultado é vazio, e vazio nunca dispara alerta.

<details><summary>✅ Solução</summary>

```yaml
      - alert: BatchJobMissing
        expr: absent(up{job="batch"})
        for: 2m
```

`absent()` devolve `{job="batch"} 1` quando o seletor não acha nada (os labels vêm dos matchers de **igualdade**). Para "sumiu há X tempo" em janelas maiores, existe `absent_over_time(up{job="batch"}[10m])`. Arquivo: [`solutions/06-absent-serie-sumiu/rules.yml`](../../solutions/06-absent-serie-sumiu/rules.yml).
</details>
