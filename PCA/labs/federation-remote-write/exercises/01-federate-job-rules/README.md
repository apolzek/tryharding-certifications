# 01 · Federar só as recording rules `job:*`

**Cenário:** o `global` deve puxar de `prom-a` e `prom-b` **apenas** as séries agregadas (`job:*`), nunca as cruas. A config quebrada deixa os alvos `up=1`... e o global continua vazio.

```bash
./load.sh exercises/01-federate-job-rules/global.yml global
# espere ~15s
curl -s localhost:9162/api/v1/query --data-urlencode 'query=scrape_samples_scraped{job="federate"}' | jq -r '.data.result[] | "\(.metric.instance) \(.value[1])"'
# prom-a:9090 0
# prom-b:9090 0
```
`up=1` com **0 amostras** é o sintoma clássico de `match[]` que não casa com nada: o `/federate` responde `200` vazio.

Teste o seletor direto no `prom-a`, sem passar pelo global:
```bash
curl -s -G localhost:9160/federate --data-urlencode 'match[]={__name__=~"job:"}' | wc -c
# 0
```

**Tarefa:** conserte o `match[]`.

> 💡 **Dica:** regex em seletores PromQL também é **ancorada**.

<details><summary>Solução</summary>

```yaml
    params:
      'match[]':
        - '{__name__=~"job:.*"}'
```
```bash
./load.sh solutions/01-federate-job-rules/global.yml global
curl -s localhost:9162/api/v1/query --data-urlencode 'query={__name__=~"job:.*"}' | jq -r '.data.result[] | "\(.metric.__name__) cluster=\(.metric.cluster)"'
# job:http_requests:rate1m cluster=a   (x3 métricas x 2 clusters)
```
Por que só agregados? Federar séries cruas de muitos Prometheus para um só = o global vira um gargalo com **toda** a cardinalidade de todo mundo. A doc recomenda federação hierárquica **de agregados** (`job:`, `cluster:`), com intervalo maior.
</details>
