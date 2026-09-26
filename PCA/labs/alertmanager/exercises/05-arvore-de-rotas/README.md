# 05 · Árvore de rotas errada: `continue`, ordem e regex (quebre e conserte)

**Objetivo:** fazer a árvore obedecer a esta tabela:

| Labels do alerta | Receivers esperados |
|---|---|
| `team=db severity=critical` | `dba,pager` |
| `team=db severity=warning` | `dba,slack` |
| `team=web severity=critical` | `pager` |
| `team=web severity=warning` | `slack` |
| `severity=info` | `default` |

## ▶️ Diagnóstico sem subir nada (`amtool config routes test`)

```bash
cd labs/alertmanager
at() { docker run --rm -v "$PWD/exercises/05-arvore-de-rotas:/c:ro" --entrypoint amtool prom/alertmanager:v0.34.1 "$@"; }
at config routes --config.file=/c/alertmanager.yml                                  # desenha a árvore
at config routes test --config.file=/c/alertmanager.yml team=db severity=critical   # -> pager   (errado!)
at config routes test --config.file=/c/alertmanager.yml team=web severity=warning   # -> default (errado!)
at config routes test --config.file=/c/alertmanager.yml --tree team=db severity=warning
at config routes test --config.file=/c/alertmanager.yml --verify.receivers=dba,pager team=db severity=critical; echo "exit=$?"
```

`--verify.receivers` faz o comando sair com **exit 1** quando o resultado difere: perfeito para CI.

## 🎯 Critério de pronto

As 5 linhas da tabela passam com `--verify.receivers`, e ao vivo:

```bash
EX=./exercises/05-arvore-de-rotas docker compose up -d --force-recreate --wait
curl -s -X POST localhost:9112/reset
curl -s 'localhost:9113/set?name=lab_up&value=0&instance=pg-1&cluster=eu&team=db'
sleep 15; curl -s localhost:9112/received | jq -c '.[] | {hook, a: [.alerts[].labels.instance]}'
# {"hook":"dba",...}  e  {"hook":"pager",...}
```

## 💡 Dica

Três bugs: (1) a busca **para** na primeira filha que casa, a não ser que ela tenha `continue: true`; (2) a **ordem** das filhas importa; (3) regex no Alertmanager é **ancorada** (`=~"warn"` equivale a `^warn$`).

<details><summary>✅ Solução</summary>

```yaml
route:
  receiver: default
  routes:
    - matchers: [team="db"]
      receiver: dba
      continue: true                 # segue testando as irmãs de baixo
    - matchers: [severity="critical"]
      receiver: pager
    - matchers: [severity=~"warn.*"] # ou severity="warning"
      receiver: slack
```

Arquivo: [`solutions/05-arvore-de-rotas/alertmanager.yml`](../../solutions/05-arvore-de-rotas/alertmanager.yml).
</details>
