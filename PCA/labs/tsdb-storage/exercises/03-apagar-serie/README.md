# 03 · Apagar uma série com a Admin API

**Objetivo:** os dados de `curitiba` importados no exercício 01 vieram de um sensor quebrado. Apague **só** essa série e libere o espaço em disco.

Pré-requisito: exercício 01 feito. A stack já sobe com `--web.enable-admin-api`.

## 🎯 Critério de pronto

```bash
curl -s localhost:9150/api/v1/series --data-urlencode 'match[]=tsdb_backfill_temperature_celsius' \
  --data-urlencode "start=$(( $(date +%s) - 3*86400 ))"
# só sobra {"city":"sao_paulo",...}
```

## 💡 Dica

São **duas** chamadas `POST`: uma marca os dados como apagados (tombstone) e a outra reescreve os blocos. Liste os blocos (`promtool tsdb list`) antes e depois da segunda: os ULIDs mudam.

<details><summary>✅ Solução</summary>

```bash
curl -XPOST localhost:9150/api/v1/admin/tsdb/delete_series \
  --data-urlencode 'match[]=tsdb_backfill_temperature_celsius{city="curitiba"}'      # 204
curl -XPOST localhost:9150/api/v1/admin/tsdb/clean_tombstones                       # 204
```

- `delete_series` aceita `start`/`end` para apagar só um intervalo. Sem eles, apaga **tudo** daquela série.
- Depois do `delete_series` a query já não acha nada, mas o disco **não** diminuiu: só um arquivo `tombstones` foi escrito em cada bloco. O `clean_tombstones` reescreve os blocos (ULIDs novos) sem os dados marcados. Se você não chamar, a compactação faz isso com o tempo.
- Apagar uma série que **ainda está sendo raspada** é inútil: ela volta no próximo scrape. Para isso use `metric_relabel_configs` + `drop`.

Script: [`solutions/03-delete-series.sh`](../../solutions/03-delete-series.sh).
</details>
