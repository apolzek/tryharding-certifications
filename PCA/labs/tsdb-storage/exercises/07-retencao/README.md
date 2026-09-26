# 07 · Mudar a retenção (sem reiniciar)

**Objetivo:** o disco está enchendo. Reduza a retenção de **15d** para **6h** e veja os blocos velhos sumirem.

Pré-requisito: exercício 01 (o dia importado).

## 🎯 Tarefa

1. Descubra a retenção atual: `curl -s localhost:9150/api/v1/status/runtimeinfo | jq .data.storageRetention`.
2. Mude para 6h em [`prometheus/prometheus.yml`](../../prometheus/prometheus.yml) e faça `curl -XPOST localhost:9150/-/reload`.
3. Espere ~1 min e rode `promtool tsdb list -r /prometheus`. Quais blocos sumiram?
4. Pergunta difícil: os blocos que sobraram são das "últimas 6h do relógio"? Por quê?
5. Volte para 15d.

## 💡 Dica

`prometheus_tsdb_time_retentions_total` e `prometheus_tsdb_retention_limit_seconds` confirmam o que aconteceu.

<details><summary>✅ Solução</summary>

```yaml
storage:
  tsdb:
    retention:
      time: 6h
```

- No Prometheus 3.x a retenção vai no **arquivo de config** e muda com **reload**. As flags `--storage.tsdb.retention.time/size` ainda funcionam, mas estão *deprecated* (e flags só mudam com restart).
- A retenção apaga **blocos inteiros** (nunca amostras soltas), na checagem que roda a cada ~1 min.
- **Pegadinha:** a conta é relativa ao **bloco mais novo** no disco, não ao relógio. Aqui o bloco mais novo termina em `END` (2h atrás), então sobram os blocos que terminam depois de `END - 6h`. Os dados na head (últimas horas) não contam como bloco.
- `retention.time` e `retention.size` podem coexistir: o que estourar primeiro vence. `size` conta blocos + WAL + chunks da head.

Arquivo: [`solutions/07-retention.yml`](../../solutions/07-retention.yml).
</details>
