# 08 · `time_intervals`: `mute_time_intervals` vs `active_time_intervals` (quebre e conserte)

**Objetivo:** warnings do `team="batch"` **não** devem ser notificados durante a janela `manutencao`. Warnings de outros times continuam chegando normalmente.

> Para o teste ser determinístico, a janela `manutencao` cobre **todos os dias, o dia inteiro** (`sunday:saturday`). Na vida real seria algo como "sábado 02:00–04:00".

## ▶️ Rodar

```bash
EX=./exercises/08-janela-manutencao docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=b-1&cluster=eu&team=batch'
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=w-1&cluster=us&team=web'
sleep 15; curl -s localhost:9112/received | jq -c '.[] | [.alerts[].labels.instance]'
# com o bug: chegam b-1 e w-1
```

## 🎯 Critério de pronto

Só `w-1` chega. O `b-1` continua ativo no Alertmanager (`amtool alert query`), só não é notificado.

## 💡 Dica

- `mute_time_intervals`: **não** notifica **dentro** da janela.
- `active_time_intervals`: **só** notifica **dentro** da janela (fora dela, fica mudo).
- Pegadinha: `weekdays: ["monday:sunday"]` é **inválido** (o intervalo começa em domingo): `amtool check-config` acusa `start day cannot be before end day`.

<details><summary>✅ Solução</summary>

```yaml
  routes:
    - matchers: [team="batch"]
      receiver: slack
      mute_time_intervals: [manutencao]

time_intervals:
  - name: manutencao
    time_intervals:
      - weekdays: ["sunday:saturday"]
```

Diferença para silence: silence é **ad hoc** (criado na hora, por matchers, com fim) e vale para qualquer rota; time interval é **recorrente**, fica na config e vale por **rota**. Arquivo: [`solutions/08-janela-manutencao/alertmanager.yml`](../../solutions/08-janela-manutencao/alertmanager.yml).
</details>
