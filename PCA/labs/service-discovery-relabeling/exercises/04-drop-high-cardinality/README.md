# 04 · Descartar uma métrica de alta cardinalidade

**Cenário:** o `payments-api` expõe `app_requests_by_user_total{user_id=...}`: **uma série por usuário** (150 aqui, milhões em produção). Alguém tentou descartá-la, mas ela continua lá:

```bash
./load.sh exercises/04-drop-high-cardinality/prometheus.yml
curl -s localhost:9120/api/v1/query --data-urlencode 'query=count(app_requests_by_user_total{job="payments-api"})' | jq -r '.data.result[0].value[1]'
# 150
```

**Tarefa:** a métrica deve sumir; `app_requests_total` e as outras continuam.

> 💡 **Dica:** em qual **momento** existe o label `__name__`? Antes do scrape só existem labels de **alvo**.

<details><summary>Solução</summary>

```yaml
  - job_name: payments-api
    static_configs:
      - targets: [payments-api:8000]
    metric_relabel_configs:        # depois do scrape, antes de gravar
      - source_labels: [__name__]
        regex: app_requests_by_user_total
        action: drop
```
Em `relabel_configs` a regra avalia `__name__` = "" (não existe no alvo), a regex não casa e **nada** é descartado.

```bash
./load.sh solutions/04-drop-high-cardinality/prometheus.yml
# espere ~10s (o próximo scrape grava staleness markers para as séries que sumiram)
curl -s localhost:9120/api/v1/query --data-urlencode 'query=count(app_requests_by_user_total{job="payments-api"})' | jq '.data.result'
# []
curl -s localhost:9120/api/v1/query --data-urlencode 'query=scrape_samples_scraped{job="payments-api"} - scrape_samples_post_metric_relabeling{job="payments-api"}' | jq -r '.data.result[0].value[1]'
# 150   <- raspadas, mas descartadas antes de gravar
```
Note: o **scrape continua trazendo** as 150 amostras (custo de rede/CPU no alvo). O ideal é corrigir na instrumentação; `metric_relabel_configs` é o "torniquete".
</details>
