# 01 · Roteamento por severidade (quebre e conserte)

**Objetivo:** alertas `severity="critical"` vão para o **pager**; todo o resto vai para o **slack**.

Hoje o [`alertmanager.yml`](alertmanager.yml) manda **tudo** para o slack, inclusive o `ServiceDown`.

## ▶️ Rodar

```bash
cd labs/alertmanager
EX=./exercises/01-roteamento-severidade docker compose up -d --force-recreate --wait
curl -s localhost:9113/reset; curl -s -X POST localhost:9112/reset

# um critical e um warning, em clusters diferentes
curl -s 'localhost:9113/set?name=lab_up&value=0&instance=api-1&cluster=eu'
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-2&cluster=us'

# ~15s depois: quem recebeu o quê?
curl -s localhost:9112/received | jq -c '.[] | {hook, alerts: [.alerts[].labels.alertname]}'
```

Resultado com o bug: os dois chegam em `"hook":"slack"`.

## 🎯 Critério de pronto

```bash
docker run --rm -v "$PWD/exercises/01-roteamento-severidade:/c:ro" --entrypoint amtool \
  prom/alertmanager:v0.34.1 config routes test --config.file=/c/alertmanager.yml severity=critical
# esperado: pager
```

E no `/received`: `ServiceDown` só no hook `pager`, `HighLatency` só no hook `slack`.

Depois de editar o arquivo, recarregue sem reiniciar: `curl -X POST localhost:9111/-/reload`.

## 💡 Dica

Olhe o label que a regra de alerta coloca: `grep -A3 'alert: ServiceDown' prometheus/rules/base.rules.yml`. O matcher compara **strings exatas**.

<details><summary>✅ Solução</summary>

```yaml
route:
  receiver: slack                      # rota raiz = "todo o resto"
  group_by: [alertname, cluster]
  routes:
    - matchers: [severity="critical"]  # era "page": nenhum alerta tem esse valor
      receiver: pager
```

A rota raiz **sempre** casa com tudo; as filhas são testadas em ordem e a primeira que casa ganha. Como nenhuma regra usa `severity="page"`, a filha nunca casava e tudo caía no receiver da raiz. Arquivo completo: [`solutions/01-roteamento-severidade/alertmanager.yml`](../../solutions/01-roteamento-severidade/alertmanager.yml).
</details>
