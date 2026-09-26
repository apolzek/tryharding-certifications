# 03 · Teste de alerta: `for:`, pending vs firing (conserte e complete)

**Objetivo:** `InstanceDown` tem `for: 2m` e a série `up` vai a 0 em **t=2m**. Conserte e complete [`rules.test.yml`](rules.test.yml) para provar:

- em **1m**: nenhum alerta (inactive);
- em **3m**: **pending** (não aparece em `exp_alerts`, mas aparece em `ALERTS{alertstate="pending"}`);
- em **4m**: **firing**, com labels e annotations completos.

## ▶️ Rodar

```bash
cd labs/recording-rules-testing/exercises/03-for-pending-firing
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules rules.test.yml
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 test rules --debug rules.test.yml | head -30    # despeja todas as séries, inclusive ALERTS e ALERTS_FOR_STATE
```

## 💡 Dica

- `alert_rule_test` → `exp_alerts` só lista alertas **firing**. Pending = `exp_alerts: []`.
- Para enxergar o pending, use um `promql_expr_test` sobre a série `ALERTS`.
- `exp_labels` precisa de **todos** os labels do alerta, menos `alertname` (que já vem do campo `alertname:`).

<details><summary>✅ Solução</summary>

```yaml
    alert_rule_test:
      - eval_time: 1m
        alertname: InstanceDown
        exp_alerts: []
      - eval_time: 3m
        alertname: InstanceDown
        exp_alerts: []
      - eval_time: 4m
        alertname: InstanceDown
        exp_alerts:
          - exp_labels: { severity: critical, job: api, instance: a }
            exp_annotations: { summary: "a fora do ar" }
    promql_expr_test:
      - expr: ALERTS{alertname="InstanceDown"}
        eval_time: 3m
        exp_samples:
          - labels: '{__name__="ALERTS", alertname="InstanceDown", alertstate="pending", job="api", instance="a", severity="critical"}'
            value: 1
```

Linha do tempo: 2m condição verdadeira (pending, `activeAt=2m`) → 3m pending → 4m (2m desde activeAt) firing. Arquivo: [`solutions/03-for-pending-firing/rules.test.yml`](../../solutions/03-for-pending-firing/rules.test.yml).
</details>
