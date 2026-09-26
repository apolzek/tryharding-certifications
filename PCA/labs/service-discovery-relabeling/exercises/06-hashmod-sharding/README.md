# 06 · `hashmod`: sharding de alvos entre 2 Prometheus

**Cenário:** 6 alvos, 2 Prometheus (`:9120` e `:9129`). Cada um deve raspar **metade**, sem sobreposição e sem buraco. A ideia: `hashmod` calcula `hash(source_labels) % modulus` e grava em um label temporário (`__tmp_hash`); cada réplica faz `keep` do seu balde.

```bash
./load.sh exercises/06-hashmod-sharding/prometheus.yml
./load.sh exercises/06-hashmod-sharding/shard1.yml shard1
for p in 9120 9129; do echo "== $p"; curl -s "localhost:$p/api/v1/targets?scrapePool=sharded-apps" | jq -r '.data.activeTargets[].labels.instance' | sort; done
# os dois mostram OS MESMOS alvos e 2 alvos não são raspados por ninguém
```

**Tarefa:** conserte para que a união seja os 6 alvos e a interseção seja vazia.

<details><summary>Solução</summary>

`prometheus.yml` (shard 0) mantém `regex: "0"`; o `shard1.yml` usa:
```yaml
      - source_labels: [__address__]
        modulus: 2
        target_label: __tmp_hash
        action: hashmod
      - source_labels: [__tmp_hash]
        regex: "1"
        action: keep
```
```bash
./load.sh solutions/06-hashmod-sharding/prometheus.yml
./load.sh solutions/06-hashmod-sharding/shard1.yml shard1
```
Em produção cada réplica recebe um config gerado por templating (o Prometheus Operator faz isso com `spec.shards`) com seu número de balde no `regex`, e normalmente também um `external_labels: {shard: "N"}` para identificar a origem dos dados (no Prometheus 3, `${VAR}` de ambiente em `external_labels` é expandido por padrão; já dentro de `relabel_configs` **não** há expansão). O `__tmp_hash` some sozinho: todo label que começa com `__` é removido depois do relabel.

⚠️ Mudar `modulus` **re-embaralha** quase todos os alvos (não é consistent hashing). E cada shard só vê metade dos dados: para queries globais você precisa de federation/Thanos/Mimir por cima (veja o lab `federation-remote-write`).
</details>
