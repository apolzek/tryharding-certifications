# 02 · Agrupamento: `group_by` (quebre e conserte)

**Objetivo:** alertas com o mesmo `alertname` **e** o mesmo `cluster` chegam juntos, numa só notificação.

Hoje o [`alertmanager.yml`](alertmanager.yml) usa `group_by: ['...']`, e cada instância gera uma mensagem separada. Com 300 pods caindo, seriam 300 mensagens no Slack.

## ▶️ Rodar

```bash
EX=./exercises/02-agrupamento docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset
for i in 1 2; do curl -s "localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-$i&cluster=eu"; done
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-3&cluster=us'
sleep 15
curl -s localhost:9112/received | jq -c '.[] | {groupLabels, n: (.alerts|length)}'
```

Com o bug: **3** notificações, cada uma com 1 alerta e `groupLabels` contendo todos os labels.

## 🎯 Critério de pronto

Exatamente **2** grupos:

```json
{"groupLabels":{"alertname":"HighLatency","cluster":"eu"},"n":2}
{"groupLabels":{"alertname":"HighLatency","cluster":"us"},"n":1}
```

## 💡 Dica

`'...'` é um valor especial: "agrupar por **todos** os labels", o que na prática **desliga** o agrupamento. Veja também `groupKey` no `/received`.

<details><summary>✅ Solução</summary>

```yaml
route:
  receiver: slack
  group_by: [alertname, cluster]
  group_wait: 5s        # espera 5s pra juntar os alertas do grupo antes da 1a mensagem
  group_interval: 10s   # novidades no MESMO grupo esperam 10s
  repeat_interval: 1h
```

Se um alerta novo do cluster `eu` chegar **depois** da 1ª notificação, ele não gera mensagem imediata: entra na próxima, após `group_interval`. Arquivo: [`solutions/02-agrupamento/alertmanager.yml`](../../solutions/02-agrupamento/alertmanager.yml).
</details>
