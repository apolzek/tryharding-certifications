# 07 · Notificação de *resolved* e templates de annotation (quebre e conserte)

**Objetivo:**
1. O time quer ser avisado quando o problema **acabar** (notificação com `"status": "resolved"`).
2. O `summary` do alerta `CheckoutErrors` deve ficar **exatamente** `checkout com 25% de erros` quando `lab_checkout_error_ratio{service="checkout"} = 0.25`.

Arquivos: [`alertmanager.yml`](alertmanager.yml) e [`checkout.rules.yml`](checkout.rules.yml).

## ▶️ Rodar

```bash
EX=./exercises/07-resolved-e-templates docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset
curl -s 'localhost:9113/set?name=lab_checkout_error_ratio&value=0.25&service=checkout&cluster=eu'
sleep 15; curl -s localhost:9112/received | jq -c '.[] | {status, s: .alerts[0].annotations.summary}'
# com o bug: {"status":"firing","s":" com 0.25 de erros"}
curl -s 'localhost:9113/set?name=lab_checkout_error_ratio&value=0&service=checkout&cluster=eu'
sleep 25; curl -s localhost:9112/received | jq -c '.[] | .status'   # com o bug: nunca chega "resolved"
```

## 💡 Dica

- Label inexistente em `$labels.x` vira **string vazia** (sem erro!).
- Funções de template do Prometheus: `humanize`, `humanize1024`, `humanizeDuration`, `humanizePercentage`, `humanizeTimestamp`, `printf "%.2f"`...
- Qual campo do receiver liga o envio de resolvidos?

<details><summary>✅ Solução</summary>

```yaml
# checkout.rules.yml
annotations:
  summary: "{{ $labels.service }} com {{ $value | humanizePercentage }} de erros"
```

```yaml
# alertmanager.yml
receivers:
  - name: slack
    webhook_configs:
      - url: http://receiver:8080/hook/slack
        send_resolved: true
```

`humanizePercentage` multiplica por 100 e formata (`0.25` → `25%`). Como resolvidos funcionam: quando a `expr` deixa de ser verdadeira, o Prometheus envia o alerta com `endsAt` = agora; o Alertmanager marca como resolvido e, no próximo `group_interval`, manda a notificação com `status: resolved` (se `send_resolved: true`). O padrão de `send_resolved` é `true` para webhook, PagerDuty e OpsGenie, mas **`false` para `slack_configs` e `email_configs`** (pegadinha clássica: no Slack você precisa ligar explicitamente). Arquivos: [`solutions/07-resolved-e-templates/`](../../solutions/07-resolved-e-templates/).
</details>
