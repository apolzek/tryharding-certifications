# 07 · `remote_read`: consultar o storage remoto como se fosse local

**Cenário:** o `global` deve conseguir consultar os dados do **agent** (que só existem no receiver) sem federar nem copiar nada. A config parece certa, mas nada volta:

```bash
./load.sh exercises/07-remote-read/global.yml global
curl -s localhost:9162/api/v1/query --data-urlencode 'query=up{cluster="edge"}' | jq '.data.result | length'
# 0
curl -s localhost:9163/api/v1/query --data-urlencode 'query=up{cluster="edge"}' | jq '.data.result | length'
# 2   <- os dados estão no receiver
```

**Tarefa:** fazer `up{cluster="edge"}` responder no global.

> 💡 **Dica:** o `global` tem `external_labels: {tier: global}`. O que o Prometheus faz com os próprios external labels ao montar uma leitura remota?

<details><summary>Solução</summary>

Por padrão (`filter_external_labels: true`) o Prometheus **acrescenta seus external_labels como matchers** em toda leitura remota (pensado para "ler de volta o que eu mesmo escrevi"). A query vira `up{cluster="edge", tier="global"}` no receiver, que não tem `tier="global"`.

```yaml
remote_read:
  - url: http://receiver:9090/api/v1/read
    name: central
    read_recent: true
    required_matchers:
      cluster: edge
    filter_external_labels: false
```
```bash
./load.sh solutions/07-remote-read/global.yml global
curl -s localhost:9162/api/v1/query --data-urlencode 'query=up{cluster="edge"}' | jq -c '.data.result[].metric'
# {"__name__":"up","cluster":"edge","instance":"app-c:8000","job":"app","tier":"central"}
# {"__name__":"up","cluster":"edge","instance":"localhost:9090","job":"agent","tier":"central"}
curl -s localhost:9162/api/v1/query --data-urlencode 'query=app_build_info' | jq '.data.result | length'
# 0   <- sem cluster="edge" na query, o required_matchers impede a ida ao remoto
```
- `tier="central"` veio do **receiver**: o endpoint `/api/v1/read` acrescenta os external labels dele.
- `read_recent: false` (padrão) permite pular o remoto em intervalos que o TSDB local já cobre.
- Remote read é pouco usado hoje: Thanos/Mimir expõem a API de query do Prometheus direto (o Grafana consulta o Mimir, não o Prometheus).
</details>
