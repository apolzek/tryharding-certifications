# 03 · Inibição: `inhibit_rules` (quebre e conserte)

**Objetivo:** enquanto houver um alerta `severity="critical"` **firing** num cluster, os alertas `severity="warning"` **do mesmo cluster** não devem ser notificados (são sintoma, não causa). Warnings de **outros** clusters continuam chegando.

A regra em [`alertmanager.yml`](alertmanager.yml) está com três erros.

## ▶️ Rodar

```bash
EX=./exercises/03-inibicao docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset
curl -s 'localhost:9113/set?name=lab_up&value=0&instance=api-1&cluster=eu'              # critical (causa)
sleep 10
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-2&cluster=eu' # warning, mesmo cluster
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-3&cluster=us' # warning, outro cluster
sleep 15
curl -s localhost:9112/received | jq -c '.[] | {hook, alerts: [.alerts[].labels.instance]}'
```

## 🎯 Critério de pronto

- `pager` recebe `api-1`; `slack` recebe **só** `api-3`.
- `api-2` aparece como suprimido:

```bash
docker compose exec alertmanager amtool --alertmanager.url=http://localhost:9093 alert query -i
curl -s localhost:9111/api/v2/alerts | jq -c '.[] | {i: .labels.instance, state: .status.state, inhibitedBy: .status.inhibitedBy}'
# {"i":"api-2","state":"suppressed","inhibitedBy":["<fingerprint do api-1>"]}
```

## 💡 Dica

`source_matchers` = quem **inibe** (a causa). `target_matchers` = quem **é inibido**. `equal` = labels que precisam ter o **mesmo valor** nos dois.

<details><summary>✅ Solução</summary>

```yaml
inhibit_rules:
  - source_matchers: [severity="critical"]
    target_matchers: [severity="warning"]
    equal: [cluster]
```

Os erros eram: source/target **invertidos** e `equal: [instance]` (o critical é de `api-1` e o warning de `api-2`, então nunca teriam o mesmo `instance`). Importante: a inibição **não apaga** o alerta, só impede a notificação; ele continua visível na UI como *suppressed*. Arquivo: [`solutions/03-inibicao/alertmanager.yml`](../../solutions/03-inibicao/alertmanager.yml).
</details>
