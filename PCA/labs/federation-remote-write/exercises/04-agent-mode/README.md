# 04 · Prometheus em modo agent

**Cenário:** o time de borda quer só "coletar e mandar" do `app-c` para o receiver central. Alguém copiou a config de um Prometheus servidor para o container que roda com `--agent`:

```bash
./load.sh exercises/04-agent-mode/agent.yml agent
# FAILED: field alerting is not allowed in agent mode
# ✘ config inválida: nada foi carregado
FORCE=1 ./load.sh exercises/04-agent-mode/agent.yml agent
# failed to reload config: ... field alerting is not allowed in agent mode
# ✘ reload falhou: o Prometheus continua com a config ANTERIOR
curl -s localhost:9164/metrics | grep '^prometheus_config_last_reload_successful'
# prometheus_config_last_reload_successful 0
```

**Tarefa:** escreva uma config válida para `--agent` que raspe `app-c:8000` (job `app`) e o próprio agent, e mande tudo para `http://receiver:9090/api/v1/write` com `cluster="edge"`.

> 💡 **Dica:** no modo agent **não existem** `rule_files`, `alerting` e `remote_read`, nem TSDB consultável. O que sobra: `global`, `scrape_configs`, `remote_write`. Valide com `promtool check config --agent`.

<details><summary>Solução</summary>

```yaml
global:
  scrape_interval: 5s
  external_labels:
    cluster: edge
scrape_configs:
  - job_name: app
    static_configs: [{ targets: [app-c:8000] }]
  - job_name: agent
    static_configs: [{ targets: [localhost:9090] }]
remote_write:
  - url: http://receiver:9090/api/v1/write
    name: central
```
```bash
./load.sh solutions/04-agent-mode/agent.yml agent
curl -s localhost:9163/api/v1/query --data-urlencode 'query=up{cluster="edge"}' | jq -r '.data.result[] | "\(.metric.job) \(.metric.instance) \(.value[1])"'
# agent localhost:9090 1
# app app-c:8000 1
curl -s 'localhost:9164/api/v1/query?query=up'
# {"status":"error","errorType":"execution","error":"unavailable with Prometheus Agent"}
```
A UI do agent (http://localhost:9164) mostra só *Targets*, *Service Discovery*, *Configuration* etc. Não há aba de alertas/regras e as consultas retornam erro. Ele guarda só um **WAL** (`--storage.agent.path`) até conseguir enviar.
</details>
