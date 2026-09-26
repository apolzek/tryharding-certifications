# 06 · `for:` e o estado *pending* (quebre e conserte)

**Objetivo:** a fila `orders` passa de 100 mensagens por alguns segundos em todo pico normal, e o plantão é acordado à toa. Faça o alerta `QueueBacklog` só **notificar** se a condição ficar verdadeira por **1 minuto seguido**.

O problema está na regra em [`queue.rules.yml`](queue.rules.yml) (o Prometheus carrega os `*.rules.yml` da pasta `EX`).

## ▶️ Rodar

```bash
EX=./exercises/06-pending-for docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset
curl -s 'localhost:9113/set?name=lab_queue_depth&value=500&queue=orders&cluster=eu'
watch -n2 "curl -s localhost:9110/api/v1/alerts | jq -c '.data.alerts[] | {a: .labels.alertname, state, activeAt}'"
```

Com o bug: `firing` em ~5s e notificação no `/received` imediatamente.

## 🎯 Critério de pronto

Depois de editar, recarregue o Prometheus (`curl -X POST localhost:9110/-/reload`), zere e dispare de novo:

1. Por ~60s o estado é **`pending`** e nada chega em `/received`.
2. No Prometheus: `ALERTS{alertname="QueueBacklog", alertstate="pending"}` existe; `ALERTS_FOR_STATE{alertname="QueueBacklog"}` tem como **valor** o timestamp Unix de quando ficou ativo.
3. Após ~1m: `firing` e a notificação chega.
4. Se você baixar a fila (`value=10`) durante o pending, o alerta volta a `inactive` e o relógio zera.

## 💡 Dica

Os 3 estados de uma regra de alerta são `inactive` → `pending` → `firing`. O que controla quanto tempo fica em `pending` é um campo da regra.

<details><summary>✅ Solução</summary>

```yaml
      - alert: QueueBacklog
        expr: lab_queue_depth > 100
        for: 1m
        labels:
          severity: warning
```

Detalhes que caem na prova:
- O Prometheus **só envia ao Alertmanager alertas `firing`** (e os resolvidos). `pending` não sai do Prometheus.
- A contagem do `for` usa as avaliações da regra: com `evaluation_interval: 5s`, o firing acontece na 1ª avaliação em que já se passou ≥ 1m desde `activeAt`.
- `ALERTS_FOR_STATE` é o que permite ao Prometheus **restaurar** o tempo de `for` depois de um restart (`--rules.alert.for-outage-tolerance`, padrão 1h).

Arquivo: [`solutions/06-pending-for/queue.rules.yml`](../../solutions/06-pending-for/queue.rules.yml).
</details>
