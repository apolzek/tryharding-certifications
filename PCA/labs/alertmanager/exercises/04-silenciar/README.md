# 04 · Silences com `amtool` (complete)

**Objetivo:** o `api-2` vai passar por um deploy. Crie um **silence** de 10 minutos só para `HighLatency` do `instance="api-2"`, confira que ele funciona e depois **expire** o silence.

A config do Alertmanager está correta: o exercício é só de `amtool`.

## ▶️ Rodar

```bash
EX=./exercises/04-silenciar docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset
alias amtool='docker compose exec -T alertmanager amtool --alertmanager.url=http://localhost:9093'

# 1) crie o silence ANTES do alerta disparar (complete o comando)
amtool silence add ______ --duration=10m --author=aluno --comment="deploy do api-2"

# 2) dispare nos dois
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-1&cluster=eu'
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-2&cluster=eu'
sleep 15; curl -s localhost:9112/received | jq -c '.[] | [.alerts[].labels.instance]'
```

## 🎯 Critério de pronto

1. A notificação do grupo `{HighLatency, eu}` contém **só** `api-1`.
2. `amtool silence query instance=api-2` mostra o seu silence.
3. `amtool alert query -s` mostra o `api-2` como silenciado.
4. Depois de `amtool silence expire <ID>`, em até `group_interval` (10s) chega uma nova notificação com `api-2`.

## 💡 Dica

`amtool silence add` recebe **matchers** como argumentos posicionais (`label=valor`, `label=~"regex"`, `label!=valor`). Ele imprime o ID; `silence query -q` imprime só os IDs (bom para scripts: `amtool silence expire $(amtool silence query -q instance=api-2)`).

<details><summary>✅ Solução</summary>

```bash
amtool silence add alertname=HighLatency instance=api-2 \
  --duration=10m --author=aluno --comment="deploy do api-2"
# 5f0c...   <- ID

amtool silence query                      # lista os ativos
amtool silence query instance=api-2       # filtra
amtool alert query -s                     # alertas silenciados
amtool silence expire 5f0c...             # encerra antes da hora
amtool silence query --expired            # histórico
```

Note que o silence filtra **dentro** do grupo: o grupo `{HighLatency, eu}` continuou existindo, só sem o `api-2`. Silences são guardados no Alertmanager (em disco, `--storage.path`) e, em HA, **replicados via gossip** entre as instâncias. Script usado pelo teste: [`solutions/04-silenciar/silence.sh`](../../solutions/04-silenciar/silence.sh).
</details>
