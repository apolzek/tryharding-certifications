#!/usr/bin/env bash
# Exercício 04: snapshot (backup consistente) via Admin API e cópia para fora do container.
set -euo pipefail
cd "$(dirname "$0")/.."
name=$(curl -sf -XPOST localhost:9150/api/v1/admin/tsdb/snapshot | sed -E 's/.*"name":"([^"]+)".*/\1/')
echo "snapshot: $name"
docker compose exec -T prometheus promtool tsdb list -r "/prometheus/snapshots/$name"
rm -rf /tmp/pca-snapshot && docker compose cp "prometheus:/prometheus/snapshots/$name" /tmp/pca-snapshot
du -sh /tmp/pca-snapshot
echo "$name" > /tmp/pca-snapshot-name
