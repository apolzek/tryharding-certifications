# 04 · Snapshot (backup) via Admin API

**Objetivo:** fazer um backup **consistente** do TSDB sem parar o Prometheus e tirá-lo do container.

## 🎯 Tarefa

1. Crie um snapshot pela API.
2. Liste os blocos dentro dele com `promtool tsdb list`.
3. Copie o snapshot para o host (`/tmp/pca-snapshot`).
4. Responda: como você **restauraria** esse backup? E o que muda com `?skip_head=true`?

## 💡 Dica

`POST /api/v1/admin/tsdb/snapshot` devolve `{"data":{"name":"<nome>"}}`; o diretório fica em `<storage.tsdb.path>/snapshots/<nome>`.

<details><summary>✅ Solução</summary>

```bash
name=$(curl -s -XPOST localhost:9150/api/v1/admin/tsdb/snapshot | sed -E 's/.*"name":"([^"]+)".*/\1/')
docker compose exec prometheus promtool tsdb list -r /prometheus/snapshots/$name
docker compose cp prometheus:/prometheus/snapshots/$name /tmp/pca-snapshot
```

- O snapshot usa **hard links** para os blocos existentes (quase instantâneo e sem ocupar o dobro de disco) e grava a head como um bloco novo.
- `?skip_head=true`: não inclui a head, então os dados das últimas ~2h ficam de fora (mais rápido).
- **Restaurar:** pare o Prometheus e aponte `--storage.tsdb.path` para o diretório do snapshot (ou copie os blocos para o data dir). Um snapshot é um TSDB normal, só com blocos.
- Snapshots **não são apagados** automaticamente: limpe `snapshots/` você mesmo.

Script: [`solutions/04-snapshot.sh`](../../solutions/04-snapshot.sh).
</details>
