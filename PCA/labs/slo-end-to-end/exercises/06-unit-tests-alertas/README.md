# 06 · Teste unitário de alerta de burn rate (`promtool test rules`)

**Objetivo:** provar, **sem esperar horas**, que os alertas de [`rules-production/slo-rules.yml`](../../rules-production/slo-rules.yml) (janelas reais de 1h/6h/1d/3d) disparam quando devem e ficam quietos quando devem.

Crie `exercises/06-unit-tests-alertas/meu.test.yml` com `rule_files: [../../rules-production/slo-rules.yml]` e pelo menos estes casos:

1. **Saudável** (0,05% de erro): nenhum alerta dispara em 1 dia e `job:slo_error_budget_remaining:ratio` ≈ 0.5.
2. **48x**: `SLOErrorBudgetBurnFast` **não** está firing em 1m (pending por causa do `for: 2m`) e **está** em 5m.
3. **9.9x**: `SLOErrorBudgetBurnMedium` dispara e `SLOErrorBudgetBurnFast` **não**.

## 🎯 Critério de pronto

```bash
cd labs/slo-end-to-end
docker run --rm -v "$PWD:/w" -w /w/exercises/06-unit-tests-alertas --entrypoint promtool prom/prometheus:v3.15.0 test rules meu.test.yml
```

## 💡 Dica

`input_series` com notação de expansão: `'0+600x1500'` = começa em 0 e soma 600 a cada `interval`, 1500 vezes. Com `interval: 1m` isso é ~1 dia de counter. `exp_alerts` precisa bater **labels e annotations** exatamente (inclusive o texto renderizado dos templates).

<details><summary>✅ Solução</summary>

Veja [`rules-production/slo-rules.test.yml`](../../rules-production/slo-rules.test.yml): 5 cenários (saudável, incidente forte, degradação moderada, "incidente acabou" e latência). Rode com:

```bash
docker run --rm -v "$PWD/rules-production:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules slo-rules.test.yml
```

- `exp_alerts: []` = "não pode estar **firing**". Um alerta em **pending** não aparece em `exp_alerts`.
- Para testar sem amarrar as annotations, dá para usar `promql_expr_test` sobre a série sintética `ALERTS{alertname="...", alertstate="firing"}` (é o que os testes dos exercícios 02 e 05 fazem).
</details>
