# Exercício 02: métrica própria via textfile collector

Você tem um script de backup rodando no cron. Ele não é um serviço (não tem porta para expor `/metrics`), mas a máquina já tem um node_exporter. Solução clássica: o script **escreve um arquivo `.prom`** e o node_exporter o expõe junto com as métricas da máquina.

O lab já monta `./textfile` em `/textfile` e o node_exporter roda com `--collector.textfile.directory=/textfile`. Veja o exemplo que já existe:

```bash
cat textfile/lab.prom
curl -s localhost:9141/metrics | grep -E '^lab_|^node_textfile'
```

## O que fazer

Escreva um script que grave `textfile/backup.prom` com:

- `backup_last_success_timestamp_seconds{target="postgres"}` = agora (Unix time)
- `backup_size_bytes{target="postgres"}` = 52428800

Requisitos: `# HELP` e `# TYPE`, e escrita **atômica** (o node_exporter nunca pode ler o arquivo pela metade).

## Como verificar

```bash
./meu-script.sh textfile        # ou: solutions/textfile/backup-metrics.sh textfile
docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics < textfile/backup.prom
curl -s localhost:9141/metrics | grep ^backup_
curl -s localhost:9140/api/v1/query --data-urlencode 'query=time() - backup_last_success_timestamp_seconds'
# alguns segundos
```

<details><summary>✅ Solução</summary>

[`solutions/textfile/backup-metrics.sh`](../../solutions/textfile/backup-metrics.sh):

```sh
TMP=$(mktemp "$DIR/.backup.prom.XXXXXX")     # não termina em .prom -> o node_exporter ignora
cat > "$TMP" <<METRICS
# HELP backup_last_success_timestamp_seconds Unix time do último backup concluído com sucesso.
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds{target="postgres"} $(date +%s)
# HELP backup_size_bytes Tamanho do último backup.
# TYPE backup_size_bytes gauge
backup_size_bytes{target="postgres"} 52428800
METRICS
chmod 644 "$TMP"
mv "$TMP" "$DIR/backup.prom"                 # rename é atômico no mesmo filesystem
```

Alerta correspondente:

```yaml
- alert: BackupNotSucceededRecently
  expr: time() - backup_last_success_timestamp_seconds > 26 * 3600
```

Regras do textfile collector:
- Só lê arquivos terminados em **`.prom`**.
- **Sem timestamp** nas amostras (o arquivo é rejeitado).
- Erro de sintaxe → `node_textfile_scrape_error 1` (monitore isso!).
- `node_textfile_mtime_seconds{file=...}` diz quando cada arquivo mudou: bom para detectar script que parou de rodar.
- Mesma métrica em dois arquivos precisa ter o mesmo HELP/TYPE e labels diferentes.
</details>
