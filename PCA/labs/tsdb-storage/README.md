# Lab: TSDB e armazenamento local do Prometheus

> **Em uma frase:** o Prometheus guarda tudo num **TSDB local**: as últimas horas ficam na **head** (memória + WAL), a cada 2h viram **blocos** imutáveis em disco, que são **compactados** em blocos maiores e **apagados** pela retenção. Não é clusterizado nem replicado: para durabilidade e longo prazo, **remote write**.

| | |
|---|---|
| **Prometheus** | http://localhost:9150 (com `--web.enable-admin-api` e `--web.enable-lifecycle`) |
| **Gerador** | http://localhost:9151 (`/metrics`, `/control`, `/backfill.om`), código em [`generator/generator.py`](generator/generator.py) |
| **Teste automático** | [`./test.sh`](test.sh) (~2,5 min) |
| **Na prova** | domínio *Prometheus Fundamentals* (armazenamento, retenção, staleness) e *Observability Concepts* (cardinalidade) |

---

## 🧠 Analogia: o caderno do plantonista

Um plantonista anota **tudo** o que acontece no hospital:

- Ele escreve no **rascunho em cima da mesa** (a **head**, em memória): rápido de escrever e de consultar.
- Para não perder nada se desmaiar, ele **também copia cada linha num diário à caneta** (o **WAL**, *write-ahead log*). O diário não serve para consultar, só para **reconstruir o rascunho** depois de um apagão (restart/crash).
- De tempos em tempos o diário fica grosso e ele faz um **resumo** do que ainda importa (o **checkpoint**) e rasga as páginas velhas.
- A cada **2 horas**, ele passa o rascunho a limpo e **encaderna** (um **bloco**): um volume imutável com um **índice remissivo** no fim (o **index**) e as páginas de anotações compactadas (os **chunks**).
- Volumes pequenos são **re-encadernados juntos** em volumes maiores (a **compactação**: 2h → 6h → 18h → 54h...).
- Se alguém pede para **apagar** uma anotação, ele não arranca a página: cola um **post-it "ignore isto"** (o **tombstone**). A página só some quando o volume for re-encadernado (`clean_tombstones` ou compactação).
- O arquivo morto tem espaço limitado: os **volumes mais velhos vão para o lixo** (a **retenção**, por tempo ou por tamanho).
- E se o hospital pegar fogo? **Não há cópia.** Por isso, quem precisa de histórico durável manda uma cópia de tudo para fora (**remote write** para Thanos, Mimir, Cortex, VictoriaMetrics...).

---

## 🏗️ Arquitetura

```
                 scrape / regras / remote-write receiver
                                  │ amostras
                                  ▼
 ┌──────────────────────────── HEAD (memória) ─────────────────────────────┐
 │ séries ativas + chunk "aberto" de cada série (~120 amostras, XOR/Gorilla) │
 │ cobre ~ as últimas 2-3h                                                  │
 └──────┬──────────────────────────────┬───────────────────────────────────┘
        │ cada append vai antes para   │ chunk cheio → gravado e mmap-ado
        ▼                              ▼
   wal/00000012  (segmentos 128MB)   chunks_head/000003
   wal/checkpoint.00000011  ← resumo do WAL até o segmento 11 (feito no truncate da head)
   wbl/  ← WAL das amostras out-of-order
        │
        │ quando a head cobre 3h (1,5 × 2h), as 2h mais antigas viram um BLOCO
        ▼
 ┌─ 01J8Z...(ULID)/ ──────────────┐   compactação (a cada ~1 min checa)
 │ meta.json   (minTime, maxTime, │   2h + 2h + 2h ──► 6h ──► 18h ──► 54h ...
 │              stats, level)     │   (bloco máximo = 10% da retenção ou 31d)
 │ index       (symbols, postings │
 │              label→séries)     │   retenção: apaga BLOCOS inteiros
 │ chunks/000001 (amostras)       │   (time e/ou size, o que vier primeiro)
 │ tombstones  (deleções pendentes)│
 └────────────────────────────────┘
        ▲
        │ blocos prontos também podem chegar "de fora":
        │ promtool tsdb create-blocks-from openmetrics|rules  (backfill)
```

Peças que caem na prova:

| Peça | O que é | Onde ver |
|---|---|---|
| **Head block** | dados recentes em memória, mutável | `prometheus_tsdb_head_series`, `prometheus_tsdb_head_chunks`, `/api/v1/status/tsdb` |
| **WAL** | log sequencial para recuperar a head após crash; ≥ 3 segmentos são mantidos | `prometheus_tsdb_wal_storage_size_bytes`, diretório `wal/` |
| **Checkpoint** | "resumo" do WAL criado no truncate da head, para apagar segmentos velhos | `prometheus_tsdb_checkpoint_creations_total`, `wal/checkpoint.N` |
| **Bloco** | diretório imutável de 2h (ou mais, após compactação) com `index`, `chunks/`, `meta.json`, `tombstones` | `promtool tsdb list`, `prometheus_tsdb_blocks_loaded` |
| **Chunk** | até ~120 amostras de uma série, comprimidas (delta-of-delta no tempo, XOR no valor) → **~1-2 bytes/amostra** | `promtool tsdb list` (NUM CHUNKS) |
| **Index** | tabela de símbolos + *postings* (índice invertido `label=valor → séries`) | `promtool tsdb analyze` |
| **Tombstone** | marca de deleção; os dados só somem ao reescrever o bloco | arquivo `tombstones`, `clean_tombstones` |
| **Compactação** | junta blocos adjacentes em blocos maiores e aplica tombstones | `prometheus_tsdb_compactions_total` |

---

## ▶️ Como rodar

```bash
cd labs/tsdb-storage
docker compose up -d --build --wait
# Prometheus: http://localhost:9150   Gerador: http://localhost:9151/control
```

O gerador expõe `tsdb_demo_requests_total{path,user_id}` com **cardinalidade controlável**:

```bash
curl -s localhost:9151/control                 # estado atual (JSON)
curl -s 'localhost:9151/control?users=2000'    # 2000 séries de tsdb_demo_requests_total
curl -s 'localhost:9151/control?users=10'      # volta ao normal
```

Todos os comandos `promtool` rodam **dentro do container** (a imagem do Prometheus já traz o binário), apontando para o data dir `/prometheus`:

```bash
docker compose exec prometheus promtool tsdb list -r /prometheus
```

---

## 🔍 Passo a passo

### 1. Olhe o diretório de dados

```bash
docker compose exec prometheus sh -c 'find /prometheus -maxdepth 2 | sort'
```

**Resultado esperado** (Prometheus recém-subido): `wal/00000000`, `wbl/`, `chunks_head/`, `lock`, `queries.active` e **nenhum bloco ainda**. Todos os dados estão na head. O primeiro bloco "de verdade" só aparece quando a head cobrir ~3h. Neste lab você vai ver blocos antes disso porque vai **importá-los** (exercício 01).

### 2. O Prometheus se monitora: `prometheus_tsdb_*`

```promql
prometheus_tsdb_head_series                              # séries ativas na head (a métrica nº 1 de capacidade)
rate(prometheus_tsdb_head_samples_appended_total[1m])    # amostras ingeridas por segundo
prometheus_tsdb_wal_storage_size_bytes                   # tamanho do WAL
prometheus_tsdb_storage_blocks_bytes                     # tamanho dos blocos persistidos
prometheus_tsdb_blocks_loaded                            # quantos blocos estão carregados
rate(prometheus_tsdb_head_series_created_total[5m])      # churn: séries novas por segundo
prometheus_tsdb_retention_limit_seconds                  # retenção por tempo em vigor
```

Agora aumente a cardinalidade e veja `prometheus_tsdb_head_series` pular ~2000 no próximo scrape:

```bash
curl -s 'localhost:9151/control?users=2000'
```

> 💡 Quando o gerador volta para `users=10`, as 1990 séries **não** saem da head na hora: recebem stale marker e só são removidas no próximo **head GC** (truncate da head, a cada ~2h). Isso é **churn**, e é o que mata Prometheus em Kubernetes com pods efêmeros.

### 3. Cardinalidade: quem está gastando?

```bash
curl -s localhost:9150/api/v1/status/tsdb | python3 -m json.tool | head -30
```

Ou **Status → TSDB Status** na UI. Mostra `headStats` e o top-10 de séries por métrica, por label e por par label=valor. Para os **blocos** em disco, use o `promtool tsdb analyze` (exercício 02).

### 4. Regra de bolso de dimensionamento

**Disco** (fórmula da documentação oficial):

```
disco ≈ retenção_em_segundos × amostras_por_segundo × bytes_por_amostra (1 a 2)
```

```promql
sum(rate(prometheus_tsdb_head_samples_appended_total[5m]))   # amostras/s
prometheus_tsdb_storage_blocks_bytes                         # quanto os blocos ocupam hoje
```

Exemplo: 1M séries raspadas a cada 15s = ~66.700 amostras/s. Com 15 dias: `66.700 × 1.296.000 s × 1,5 B ≈ 130 GB`, mais WAL.

**Memória:** o que manda é o número de **séries ativas** na head (e o churn), não a retenção. Ordem de grandeza: **alguns KiB por série ativa** (labels longos e muito churn aumentam). Meça no seu próprio servidor:

```promql
process_resident_memory_bytes{job="prometheus"} / prometheus_tsdb_head_series   # bytes por série (neste lab, com poucas séries, o overhead fixo domina)
```

Com 1M de séries ativas, conte com **vários GB de RAM** (algo como 4-8 GB é o que se vê na prática). Retenção maior = mais **disco**, quase nada a mais de RAM.

### 5. `promtool tsdb`: list, dump, dump-openmetrics, analyze

```bash
docker compose exec prometheus promtool tsdb list -r /prometheus          # blocos: ULID, min/max time, amostras, séries, tamanho
docker compose exec prometheus promtool tsdb dump --match='tsdb_demo_temperature_celsius{room="office"}' \
  --min-time=$(( ($(date +%s) - 60) * 1000 )) /prometheus                  # amostras cruas (ms), inclusive da head/WAL
docker compose exec prometheus promtool tsdb dump-openmetrics --match='tsdb_demo_config_users' \
  --min-time=$(( ($(date +%s) - 30) * 1000 )) /prometheus                  # mesmo dado em OpenMetrics (ts em segundos, termina em # EOF)
docker compose exec prometheus promtool tsdb analyze --limit=5 /prometheus # cardinalidade/churn do ÚLTIMO bloco
```

O `dump-openmetrics` é o caminho de volta do backfill: exporta de um Prometheus e importa em outro com `create-blocks-from openmetrics`.

### 6. Staleness: quando uma série "some"?

Há **dois** mecanismos, e a prova adora a diferença:

1. **Stale marker** (desde o 2.0): se uma série estava no scrape anterior e **não** está no atual (ou o alvo caiu, ou saiu do service discovery), o Prometheus grava um NaN especial. A série some das queries **imediatamente**.
2. **Lookback delta** (`--query.lookback-delta`, padrão **5m**): um seletor instantâneo em `t` pega a amostra mais recente em `[t-5m, t]`. Sem stale marker, uma série "morta" continua aparecendo por **até 5 min**.

Quem **não** recebe stale marker: séries expostas **com timestamp explícito** (federação, exportadores que "carimbam" o tempo), dados de **backfill**, amostras de remote write. Demonstração:

```bash
curl -s 'localhost:9151/control?ephemeral=0&timestamped=0'
# repita por ~1 min:
curl -s localhost:9150/api/v1/query --data-urlencode 'query={__name__=~"tsdb_demo_(ephemeral|timestamped)"}' | python3 -m json.tool | grep __name__
```

**Resultado esperado:** `tsdb_demo_ephemeral` some em ≤ 5s; `tsdb_demo_timestamped` continua lá por **5 minutos**. Detalhes e o `dump` com o `NaN` no exercício 06. Volte com `curl -s 'localhost:9151/control?ephemeral=1&timestamped=1'`.

### 7. Out-of-order

Por padrão o TSDB **rejeita** amostras com timestamp menor que a última da série (`out of order sample`). Com `storage.tsdb.out_of_order_time_window` (aqui `30m`) ele aceita atrasos até a janela, guardando-as numa head separada com WAL próprio (`wbl/`). Útil para remote write/OTLP de agentes que ficaram offline. Exercício 09.

### 8. Admin API (desligada por padrão!)

Só existe com `--web.enable-admin-api`:

| Chamada | O que faz |
|---|---|
| `POST /api/v1/admin/tsdb/snapshot[?skip_head=true]` | cria `snapshots/<nome>/` com hard links dos blocos + a head como bloco |
| `POST /api/v1/admin/tsdb/delete_series?match[]=...&start=&end=` | grava **tombstones** (os dados somem das queries na hora) |
| `POST /api/v1/admin/tsdb/clean_tombstones` | reescreve os blocos sem os dados apagados (libera disco) |

### 9. Por que "local" é uma limitação

- **Um nó só.** Não há cluster, replicação nem consenso. Se o disco morrer, os dados morrem. Alta disponibilidade = **dois Prometheus iguais** raspando os mesmos alvos (os dados ficam parecidos, não idênticos).
- **Não use NFS** ou sistemas não-POSIX: a doc avisa que pode corromper.
- Retenção longa (meses/anos) e visão global = **remote write** para um backend (Thanos, Mimir, Cortex, VictoriaMetrics...) ou Thanos sidecar lendo os blocos.
- A doc é explícita: o armazenamento local **não** tem a pretensão de ser durável; trate-o como um cache das últimas semanas.

---

## 🏭 Casos reais

### Caso 1: retenção em produção (Prometheus 3.x)

"15 dias ou 400 GB, o que vier primeiro", num volume de 500 GB (sobra para WAL e compactação, que precisa de espaço temporário):

```yaml
# prometheus.yml
storage:
  tsdb:
    retention:
      time: 15d
      size: 400GB
    out_of_order_time_window: 10m   # agentes remote-write que atrasam um pouco
```

No **prometheus-operator** o equivalente é `spec.retention: 15d` e `spec.retentionSize: 400GB` no CRD `Prometheus`.

### Caso 2: alertas de saúde do TSDB (do `prometheus-mixin`)

```yaml
groups:
  - name: prometheus-tsdb
    rules:
      - alert: PrometheusTSDBReloadsFailing
        expr: increase(prometheus_tsdb_reloads_failures_total{job="prometheus"}[3h]) > 0
        for: 4h
        labels: { severity: warning }
        annotations:
          summary: Prometheus has issues reloading blocks from disk.
      - alert: PrometheusTSDBCompactionsFailing
        expr: increase(prometheus_tsdb_compactions_failed_total{job="prometheus"}[3h]) > 0
        for: 4h
        labels: { severity: warning }
        annotations:
          summary: Prometheus has issues compacting blocks.
      # não é do mixin, mas é comum: explosão de cardinalidade
      - alert: PrometheusHeadSeriesGrowing
        expr: prometheus_tsdb_head_series > 1.5 * prometheus_tsdb_head_series offset 1h
        for: 15m
        labels: { severity: warning }
        annotations:
          summary: "Séries ativas cresceram 50% em 1h ({{ $value }})"
```

### Caso 3: conter um label de alta cardinalidade enquanto o fix não sai

```yaml
scrape_configs:
  - job_name: checkout
    sample_limit: 50000          # scrape com mais amostras que isso FALHA inteiro (up=0): proteção
    label_limit: 30              # máximo de labels por série
    label_value_length_limit: 200
    static_configs:
      - targets: ["checkout:8080"]
    metric_relabel_configs:
      - action: labeldrop          # tira o user_id ANTES de gravar no TSDB
        regex: user_id
```

> ⚠️ `labeldrop` pode fazer duas séries virarem uma só (colisão). Aqui, com `path` sobrando, os counters de usuários diferentes colidiriam: o certo é agregar no código ou dropar a métrica inteira (`action: drop` com `source_labels: [__name__]`).

### Caso 4: migração com backfill

Importar histórico de outro Prometheus (ou de um CSV convertido):

```bash
promtool tsdb dump-openmetrics --min-time=1727740800000 --max-time=1727827200000 /old/data > day.om
promtool tsdb create-blocks-from openmetrics day.om /new/data
# o Prometheus novo carrega os blocos sozinho; sobreposição com blocos existentes é permitida (desde v2.39)
```

---

## 🧪 Exercícios

| # | Exercício | O que você pratica |
|---|---|---|
| 01 | [Backfill de 1 dia em OpenMetrics](exercises/01-backfill-openmetrics/) | `create-blocks-from openmetrics`, blocos, `tsdb list` |
| 02 | [Top cardinalidade](exercises/02-top-cardinalidade/) | `tsdb analyze`, snapshot da head, `/api/v1/status/tsdb` |
| 03 | [Apagar uma série](exercises/03-apagar-serie/) | `delete_series`, tombstones, `clean_tombstones` |
| 04 | [Snapshot](exercises/04-snapshot/) | backup consistente, `skip_head`, restauração |
| 05 | [Lookback delta](exercises/05-lookback-delta/) | query em timestamp, `lookback_delta` |
| 06 | [Staleness](exercises/06-staleness/) | stale markers vs timestamps explícitos vs alvo DOWN |
| 07 | [Retenção](exercises/07-retencao/) | `storage.tsdb.retention` via reload, retenção por bloco |
| 08 | [Backfill de recording rule](exercises/08-backfill-rules/) | `create-blocks-from rules` |
| 09 | [Out-of-order](exercises/09-out-of-order/) | `out_of_order_time_window`, `too_old` |

Soluções em [`solutions/`](solutions/). O [`test.sh`](test.sh) roda todas elas e confere o resultado pela API.

---

## ⚠️ Pegadinhas

1. **`promtool tsdb analyze` não vê a head.** Ele lê blocos. Métrica nova de 10 minutos atrás? Tire um snapshot e analise o snapshot, ou use `/api/v1/status/tsdb`.
2. **`delete_series` não libera disco.** Só grava tombstones. Precisa de `clean_tombstones` (ou esperar a compactação).
3. **Apagar série ainda raspada é inútil:** ela volta no próximo scrape. Corrija na origem ou com `metric_relabel_configs`.
4. **Retenção é por bloco e relativa ao bloco mais novo**, não ao relógio. E blocos só são apagados inteiros.
5. **Retenção não reduz RAM.** Quem manda na memória são as séries ativas na head.
6. **Admin API desligada por padrão** (`--web.enable-admin-api`). E `/-/reload` só com `--web.enable-lifecycle` (ou mande `SIGHUP`).
7. **Flags de retenção estão deprecated no 3.x** (ainda funcionam). O jeito novo é `storage.tsdb.retention` no `prometheus.yml`, que muda com reload.
8. **Unidades de tempo diferentes:** OpenMetrics usa **segundos** (com fração); o text format clássico do Prometheus usa **milissegundos**; `--min-time/--max-time` do `promtool tsdb dump` são em **ms**; a API HTTP (`time=`, `start=`, `end=`) aceita unix em **segundos** ou RFC3339.
9. **Lookback de 5m** explica "a série continua no gráfico depois que o pod morreu" (quando não há stale marker) e "buracos" com scrape/eval > 5m.
10. **Snapshots não se apagam sozinhos** e ficam dentro do data dir: limpe `snapshots/`.

---

## 🎓 Na prova PCA

O que costuma cair:
- Head em memória + **WAL**; blocos de **2h**; compactação em blocos maiores; ~**1-2 bytes/amostra**.
- Retenção padrão **15d**; `time` e `size` juntos: o que estourar primeiro.
- Armazenamento local **não é clusterizado/replicado**; longo prazo/HA = remote write/Thanos/Mimir + pares de Prometheus.
- **Staleness**: stale markers quando a série/alvo some; lookback padrão **5m** no resto.
- Admin API: snapshot, delete_series, clean_tombstones (precisa de flag).
- Cardinalidade = combinações de labels; labels com IDs de usuário/requisição são o erro clássico.

**1.** How long is the default local storage retention in Prometheus?
- A) 7d  B) 15d  C) 30d  D) Unlimited

<details><summary>Resposta</summary>

**B.** 15 dias, quando **nem** `retention.time` **nem** `retention.size` são configurados. Detalhe de prova: se você configurar **só** o `size`, o padrão de 15d deixa de valer e só o limite de tamanho é aplicado.
</details>

**2.** Which component allows Prometheus to recover in-memory data after a crash?
- A) Tombstones  B) The index file  C) The write-ahead log (WAL)  D) Compaction

<details><summary>Resposta</summary>

**C.** Toda amostra que entra na head é antes escrita no WAL. No restart, o Prometheus faz *replay* do WAL (e do checkpoint) para reconstruir a head. Tombstones marcam deleções; o index é por bloco; compactação junta blocos.
</details>

**3.** A target is removed from service discovery. When do its series stop being returned by an instant query?
- A) After 5 minutes (lookback delta)
- B) Immediately, because Prometheus writes stale markers
- C) After the next compaction
- D) Only after the retention period

<details><summary>Resposta</summary>

**B.** Alvo removido (ou scrape falhando, ou série que sumiu do `/metrics`) → stale markers. Os 5 minutos do lookback só valem quando **não** há stale marker, por exemplo séries expostas com timestamp explícito.
</details>

**4.** You called `/api/v1/admin/tsdb/delete_series` but disk usage did not change. Why?
- A) The admin API is read-only
- B) Deletion only writes tombstones; space is reclaimed by `clean_tombstones` or compaction
- C) You need to restart Prometheus
- D) Deleted data is moved to the WAL

<details><summary>Resposta</summary>

**B.** Os blocos são imutáveis: a deleção é registrada num arquivo `tombstones` e aplicada quando o bloco é reescrito (`POST /api/v1/admin/tsdb/clean_tombstones` ou compactação).
</details>

**5.** Which statement about Prometheus local storage is correct?
- A) It replicates data across Prometheus servers in the same cluster
- B) It is not clustered or replicated; use remote storage integrations for durability and long-term storage
- C) It requires an external database such as Cassandra
- D) It is recommended to put it on NFS for durability

<details><summary>Resposta</summary>

**B.** Texto quase literal da doc: *"Prometheus's local storage is limited to a single node's scalability and durability."* NFS não é recomendado (não-POSIX, risco de corrupção).
</details>

**6.** What is the approximate on-disk cost of a sample in the Prometheus TSDB?
- A) 1-2 bytes  B) 16 bytes  C) 64 bytes  D) 1 KiB

<details><summary>Resposta</summary>

**A.** Graças à compressão delta-of-delta (timestamps) + XOR (valores) dos chunks. Uma amostra "crua" seria 16 bytes (int64 + float64).
</details>

**7.** Which tool imports historical data from an OpenMetrics file into Prometheus?
- A) `promtool tsdb dump`
- B) `promtool tsdb create-blocks-from openmetrics`
- C) `promtool check metrics`
- D) The `/api/v1/admin/tsdb/snapshot` endpoint

<details><summary>Resposta</summary>

**B.** Ele gera blocos TSDB que você coloca no data dir. `dump`/`dump-openmetrics` fazem o caminho inverso; `create-blocks-from rules` faz backfill de recording rules.
</details>

**8.** Your Prometheus memory doubled after a deploy. What is the most likely cause?
- A) Retention was increased from 15d to 30d
- B) A new label with high cardinality (e.g. user ID) multiplied the number of active series
- C) The scrape interval changed from 15s to 30s
- D) Remote write was enabled to a slow endpoint

<details><summary>Resposta</summary>

**B.** RAM é função das **séries ativas na head**. Retenção custa disco; scrape mais lento reduz amostras, não séries. (Remote write lento também gasta memória, mas em fila, e não "dobra" por deploy de aplicação.)
</details>

---

## 📝 Cola rápida

- Head (RAM) + WAL (`wal/`, segmentos, checkpoint) → blocos de **2h** (`index`, `chunks/`, `meta.json`, `tombstones`) → compactação (2h→6h→18h...; máx 10% da retenção / 31d) → retenção apaga blocos.
- Retenção: `storage.tsdb.retention.{time,size}` no config (3.x, com reload) ou flags `--storage.tsdb.retention.*` (deprecated). Padrão **15d**. Relativa ao bloco mais novo.
- Disco ≈ retenção(s) × amostras/s × 1-2 B. RAM ∝ séries ativas (+ churn).
- `promtool tsdb list | analyze | dump | dump-openmetrics | create-blocks-from openmetrics|rules`.
- Admin API (`--web.enable-admin-api`): `snapshot`, `delete_series`, `clean_tombstones`.
- Staleness: stale marker quando série/alvo some; senão lookback **5m** (`--query.lookback-delta`, ou `lookback_delta=` na API).
- OOO: `storage.tsdb.out_of_order_time_window` (padrão 0 = rejeita).
- Self-metrics: `prometheus_tsdb_head_series`, `..._head_samples_appended_total`, `..._wal_storage_size_bytes`, `..._storage_blocks_bytes`, `..._compactions_failed_total`, `..._time_retentions_total`.
- Local = 1 nó, sem replicação. Longo prazo/HA global = remote write / Thanos / Mimir.

## 📚 Referências

- https://prometheus.io/docs/prometheus/latest/storage/
- https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-admin-apis
- https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness
- https://prometheus.io/docs/prometheus/latest/command-line/promtool/#promtool-tsdb
- https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tsdb
- https://github.com/prometheus/prometheus/blob/main/tsdb/docs/format/README.md
- https://ganeshvernekar.com/blog/prometheus-tsdb-the-head-block/ (série de posts sobre a head, WAL e mmap)
