# 03 · Quanto budget ainda resta?

**Objetivo:** responder "podemos fazer deploy hoje?" com números: a fração do error budget que sobra e quantos **minutos de indisponibilidade total** ainda cabem no mês.

Complete o [`budget.yml`](budget.yml) com três regras:

1. `job:slo_errors_per_request:ratio_rate30d`: razão de erro na janela inteira do SLO (30d).
2. `job:slo_error_budget_remaining:ratio`: `1` = intacto, `0` = gasto, negativo = SLO violado.
3. `job:slo_error_budget_remaining:minutes`.

## 🎯 Critério de pronto

```bash
cd exercises/03-error-budget
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules budget.test.yml
```

E no lab rodando (janela de 12h ≈ 30d): `curl -s localhost:9170/api/v1/query --data-urlencode 'query=job:slo_error_budget_remaining:ratio'`.

<details><summary>✅ Solução</summary>

```yaml
- record: job:slo_error_budget_remaining:ratio
  expr: 1 - (job:slo_errors_per_request:ratio_rate30d / (1 - 0.999))
- record: job:slo_error_budget_remaining:minutes
  expr: job:slo_error_budget_remaining:ratio * (1 - 0.999) * 30 * 24 * 60
```

- Budget de 99,9% em 30d = 0,1% × 43.200 min = **43,2 minutos** de indisponibilidade total. Com 0,05% de erro constante, metade vai embora: sobram **21,6 min**.
- "Minutos" é uma simplificação: o budget é de **requisições**. 43 min de 100% de erro de madrugada (pouco tráfego) gastam menos budget do que 43 min no pico.
- Grupo com `interval: 5m`: uma query de 30d é cara e não muda rápido.

Arquivo: [`solutions/03-error-budget.yml`](../../solutions/03-error-budget.yml).
</details>
