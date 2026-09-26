# 01 · `keep`: só os alvos do time payments

**Cenário:** o Prometheus do time de pagamentos deve raspar **só** os alvos com `team="payments"`. O arquivo `sd/apps.json` tem 4 alvos (2 de payments, 1 checkout, 1 search). Alguém escreveu a regra abaixo e... **nenhum** alvo aparece.

```bash
./load.sh exercises/01-keep-team-payments/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=payments-only' | jq '.data.activeTargets[].labels.instance'
# (vazio)
curl -s 'localhost:9120/api/v1/targets?state=dropped' | jq '.data.droppedTargetCounts'
# { "payments-only": 4, "prometheus": 0 }
```

**Tarefa:** conserte `exercises/01-keep-team-payments/prometheus.yml` para ficar com exatamente `payments-api:8000` e `payments-worker:8000`.

> 💡 **Dica:** toda regex de relabel é **ancorada** nas duas pontas. `regex: payment` na verdade é `^(?:payment)$`.

<details><summary>Solução</summary>

```yaml
    relabel_configs:
      - source_labels: [team]
        regex: payments          # ou payment.* / payments|billing
        action: keep
```

Arquivo completo: [`solutions/01-keep-team-payments/prometheus.yml`](../../solutions/01-keep-team-payments/prometheus.yml).

Verifique:
```bash
./load.sh solutions/01-keep-team-payments/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=payments-only' | jq -r '.data.activeTargets[].labels.instance'
# payments-api:8000
# payments-worker:8000
curl -s 'localhost:9120/api/v1/targets?state=dropped' | jq '.data.droppedTargetCounts["payments-only"]'
# 2
```
Na UI, http://localhost:9120/service-discovery mostra os 2 descartados com os **discovered labels** (antes do relabel).
</details>
