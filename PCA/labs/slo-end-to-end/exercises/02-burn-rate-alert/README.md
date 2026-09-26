# 02 · Alerta multi-window multi-burn-rate

**Objetivo:** escrever os dois alertas de **página** do SRE Workbook para o SLO de 99,9% (budget = 0,001):

| Alerta | Janela longa | Janela curta | Burn rate | `for` | Budget gasto quando dispara |
|---|---|---|---|---|---|
| `SLOErrorBudgetBurnFast` | 1h | 5m | 14.4 | 2m | 2% |
| `SLOErrorBudgetBurnMedium` | 6h | 30m | 6 | 15m | 5% |

Labels: `severity: page` e `burn_rate: "14.4"` / `"6"`. Os SLIs já estão prontos em [`sli.yml`](sli.yml). Edite [`alerts.yml`](alerts.yml).

## 🎯 Critério de pronto

```bash
cd exercises/02-burn-rate-alert
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules alerts.test.yml
```

Os testes cobrem: 48x (os dois disparam, cada um depois do seu `for`), 9.9x (só o Medium) e "incidente acabou" (a janela curta desliga o Fast).

## 💡 Dica

Burn rate = razão de erro / budget. "Burn rate > 14.4" ⇔ "razão de erro > 14.4 × 0.001". As duas janelas se juntam com `and`.

<details><summary>✅ Solução</summary>

```yaml
- alert: SLOErrorBudgetBurnFast
  expr: |
    job:slo_errors_per_request:ratio_rate1h > (14.4 * 0.001)
    and
    job:slo_errors_per_request:ratio_rate5m > (14.4 * 0.001)
  for: 2m
  labels: { severity: page, burn_rate: "14.4" }
```

- **Por que 14.4?** 2% do budget de 30 dias em 1 hora: `0.02 × 30d / 1h = 0.02 × 720 = 14.4`.
- **Por que duas janelas?** A longa (1h) garante que o problema é **significativo**; a curta (5m, 1/12 da longa) garante que ele **ainda está acontecendo**: sem ela o alerta ficaria firing até ~1h depois do conserto.
- O `and` casa pelos labels (`job`), por isso as regras de SLI mantêm `by (job)`.

Arquivo: [`solutions/02-burn-rate-alerts.yml`](../../solutions/02-burn-rate-alerts.yml).
</details>
