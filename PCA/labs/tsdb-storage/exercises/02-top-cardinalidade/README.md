# 02 · Quem explodiu a cardinalidade? (`promtool tsdb analyze`)

**Objetivo:** um dev subiu um deploy que colocou `user_id` como label. A memória do Prometheus disparou. Descubra **qual métrica** e **qual label** são os culpados.

## ▶️ Rodar

```bash
curl -s 'localhost:9151/control?users=1500'   # o "deploy ruim"
```

## 🎯 Tarefa

1. Confira o efeito na head: `prometheus_tsdb_head_series` (antes/depois).
2. Rode `promtool tsdb analyze` e ache a métrica com mais séries e o label com mais valores.
3. Pegadinha: rodar `promtool tsdb analyze /prometheus` **não mostra** a métrica nova. Por quê? Como contornar?
4. Chegue na mesma resposta **sem promtool**, só pela API/UI.

## 💡 Dica

O `analyze` lê **blocos persistidos**. Dados recentes estão na **head** (memória + WAL), que ainda não é bloco. Existe uma chamada da Admin API que grava a head como bloco...

<details><summary>✅ Solução</summary>

```bash
name=$(curl -s -XPOST localhost:9150/api/v1/admin/tsdb/snapshot | sed -E 's/.*"name":"([^"]+)".*/\1/')
docker compose exec prometheus promtool tsdb analyze --limit=5 /prometheus/snapshots/$name
# Highest cardinality labels:
# 1500 user_id
# Highest cardinality metric names:
# 1500 tsdb_demo_requests_total
```

- O snapshot (sem `skip_head=true`) escreve a head como um bloco novo; o `analyze` pega por padrão o **último** bloco, que é justamente esse.
- Sem promtool: **Status → TSDB Status** na UI, ou `curl -s localhost:9150/api/v1/status/tsdb | jq '.data.seriesCountByMetricName[0]'`. Em PromQL: `topk(5, count by (__name__) ({__name__=~".+"}))` (cara em produção!).
- Conserto de verdade: tirar `user_id` do label (vai para log/trace) ou `metric_relabel_configs` com `action: labeldrop` enquanto o fix não sai.

Script: [`solutions/02-top-cardinality.sh`](../../solutions/02-top-cardinality.sh). Volte ao normal: `curl -s 'localhost:9151/control?users=10'`.
</details>
