# 05 · `labeldrop`: remover um label ruidoso

**Cenário:** `app_cache_entries` tem o label `pod_uid`, que muda a cada deploy e gera **churn** (séries novas a cada rollout). Queremos removê-lo. A config abaixo nem carrega:

```bash
./load.sh exercises/05-labeldrop/prometheus.yml
# FAILED: parsing YAML file ...: labeldrop action requires only 'regex', and no other fields
# ✘ config inválida: nada foi carregado
```

**Tarefa:** conserte e confirme que as 4 séries de `app_cache_entries{job="apps-labeldrop"}` existem **sem** `pod_uid`.

> 💡 **Dica:** `labeldrop`/`labelkeep` casam a regex contra o **nome** de cada label, não contra valores.

<details><summary>Solução</summary>

```yaml
    metric_relabel_configs:
      - regex: pod_uid
        action: labeldrop
```

```bash
./load.sh solutions/05-labeldrop/prometheus.yml
curl -s localhost:9120/api/v1/query --data-urlencode 'query=app_cache_entries{job="apps-labeldrop"}' | jq -c '.data.result[].metric'
# {"__name__":"app_cache_entries","cache":"sessions","instance":"checkout-api:8000","job":"apps-labeldrop"} ...
```
⚠️ Só dê `labeldrop` em label que **não** é o que diferencia as séries. Se dois samples ficarem com labels idênticos, o scrape falha/descarta amostras duplicadas. Aqui cada série continua única graças ao `instance`.
</details>
