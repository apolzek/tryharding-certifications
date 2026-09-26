# 05 · Compare as contagens de séries

Exercício de **leitura** (sem config). Responda com queries, antes de abrir a solução:

1. Quantas séries o `prom-a` tem na head (`prometheus_tsdb_head_series`)?
2. Quantas dessas chegam ao `global` via federation?
3. Quantas chegam ao `receiver` via remote write?
4. Quantas séries o **agent** está mantendo ativas e quantas chegam ao receiver com `cluster="edge"`?
5. Quantas amostras o `prom-a` **descartou** no remote write e por quê?

> 💡 Use `last_over_time(...[20s])` para contar só séries "vivas" (ignorar o lookback de 5 min de séries que já pararam).

<details><summary>Solução</summary>

```bash
q(){ curl -s "localhost:$1/api/v1/query" --data-urlencode "query=$2" | jq -r '.data.result[] | "\(.metric) \(.value[1])"'; }
q 9160 'prometheus_tsdb_head_series'                                  # 1) ~840 (tudo: app + self-scrape + rules)
q 9162 'count(last_over_time({cluster="a"}[20s]))'                    # 2) 3   (só job:*)
q 9163 'count(last_over_time({cluster="a"}[20s]))'                    # 3) 5   (job:* + up)
curl -s localhost:9164/metrics | grep '^prometheus_agent_active_series' # 4a) ~550
q 9163 'count(last_over_time({cluster="edge"}[20s]))'                 # 4b) ~550 (o agent manda TUDO)
q 9160 'sum by (reason) (prometheus_remote_storage_samples_dropped_total)'  # 5) reason="dropped_series" (write_relabel)
```
Conclusões:
- **Federation** e **write_relabel** são as ferramentas para enviar "o resumo" em vez do "tudo". De ~840 séries, o central recebe 3-5.
- O **agent** sem filtro manda 100% do que raspa. Em produção, combine agent + `write_relabel_configs` (ou `metric_relabel_configs`).
- O `global` também tem suas próprias séries (`prometheus_*` do self-scrape), por isso a `prometheus_tsdb_head_series` dele não é só "o que foi federado".
</details>
