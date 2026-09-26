#!/bin/sh
# EX02: grava métricas de um "backup" para o textfile collector do node_exporter.
# Uso: solutions/textfile/backup-metrics.sh [diretório]   (padrão: ./textfile)
# Em produção: chamado no fim do script de backup, via cron/systemd timer.
set -eu
DIR=${1:-./textfile}
# escrita ATÔMICA: escreve num temporário (que não termina em .prom) e renomeia.
# Assim o node_exporter nunca lê um arquivo pela metade.
TMP=$(mktemp "$DIR/.backup.prom.XXXXXX")
cat > "$TMP" <<METRICS
# HELP backup_last_success_timestamp_seconds Unix time do último backup concluído com sucesso.
# TYPE backup_last_success_timestamp_seconds gauge
backup_last_success_timestamp_seconds{target="postgres"} $(date +%s)
# HELP backup_size_bytes Tamanho do último backup.
# TYPE backup_size_bytes gauge
backup_size_bytes{target="postgres"} 52428800
METRICS
chmod 644 "$TMP"
mv "$TMP" "$DIR/backup.prom"
echo "escrito $DIR/backup.prom"
