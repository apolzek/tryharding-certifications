# `sort_by_label_desc()`: ordenar pelo nome (label), decrescente

> **Em uma frase:** `sort_by_label_desc(v, "label", ...)` é o [`sort_by_label()`](../sort_by_label/) ao contrário: ordena as séries de forma **decrescente** pelos valores dos labels, em **ordem natural** (`11` > `10` > `9`). Só afeta **consultas instantâneas**.

| | |
|---|---|
| **Assinatura** | `sort_by_label_desc(v instant-vector, label string, ...) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Qualquer uma (float e histogram) |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-sort_by_label_desc |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: "mais recentes primeiro"

Na caixa de entrada do e-mail, na página de *Releases* do GitHub, no histórico de backups: o que você quer ver **primeiro** é o **mais novo**. Se o "mais novo" está codificado no **nome** (uma data `2026-09-25`, uma versão `2.11.0`, um número de partição `11`), é o `sort_by_label_desc` que resolve.

```
sort_by_label_desc(..., "date")        sort_by_label_desc(..., "partition")
2026-09-25   14 GiB  ← hoje            11   12 GiB
2026-09-24   13 GiB                    10   11 GiB
2026-09-23   12 GiB                     9   10 GiB
2026-09-22   11 GiB                    ...
2026-09-21   10 GiB                     0    1 GiB
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica (imita...) | Labels | Valor |
|---|---|---|
| `sort_by_label_desc_backup_last_size_bytes` (Velero / pgBackRest) | `date` = últimos 5 dias em `YYYY-MM-DD` (UTC, pelo relógio) | 10, 11, 12, 13, **14 GiB** (hoje) |
| `sort_by_label_desc_app_build_info` (estilo `*_build_info`) | `pod`, `version` | 1 por pod: `2.9.1` ×1, `2.10.3` ×4, `2.11.0` ×3, `2.11.0-rc.1` ×1 (canary esquecido) |
| `sort_by_label_desc_kafka_log_size_bytes` (`kafka_log_log_size`) | `topic="orders"`, `partition="0"` ... `"11"` | **(partição + 1) GiB** |

```bash
curl -s localhost:8088/metrics | grep '^sort_by_label_desc_' | head -3
# sort_by_label_desc_app_build_info{pod="api-2.10.3-0",version="2.10.3"} 1
# sort_by_label_desc_backup_last_size_bytes{date="2026-09-25"} 1.5032385536e+10
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sort_by_label_desc
```

Os dados aparecem em segundos.

---

## 🔍 Queries passo a passo

### 1. Backups: o de hoje primeiro

```promql
sort_by_label_desc(sort_by_label_desc_backup_last_size_bytes, "date")
```

**Resultado esperado:** 5 linhas, a primeira é a data de **hoje** (UTC) com **14 GiB**; depois ontem (13 GiB) ... até 4 dias atrás (10 GiB).

> 💡 Datas no formato **ISO 8601** (`YYYY-MM-DD`) já nascem ordenáveis como texto. Com `DD/MM/YYYY` nem ordem natural salva você (`25/09` > `01/10`).

---

### 2. Versões, mais nova primeiro... e a pegadinha do semver

```promql
sort_by_label_desc(count by (version) (sort_by_label_desc_app_build_info), "version")
```

**Resultado esperado:**

| version | pods |
|---|---|
| **2.11.0-rc.1** | 1 ← aparece **acima** do 2.11.0 |
| 2.11.0 | 3 |
| 2.10.3 | 4 |
| 2.9.1 | 1 |

A ordem natural acerta `2.10.3 > 2.9.1` (compara 10 e 9 como números), mas **não conhece semver**: para ela, `2.11.0-rc.1` é `2.11.0` **com mais coisa depois**, logo é "maior". Pelo semver, uma pre-release é **menor** que a versão final. Se a tabela for usada para dizer "qual é a versão mais nova em produção?", você responderia errado.

---

### 3 e 4. Partições Kafka: 11, 10, 9, ... 0

```promql
sort_by_label_desc(sort_by_label_desc_kafka_log_size_bytes, "partition")
```

**Resultado esperado:** 12 linhas: partição `11` (12 GiB), `10` (11 GiB), `9` (10 GiB), ..., `0` (1 GiB). No bar gauge, as barras **diminuem em escada**.
Com ordem alfabética decrescente o resultado seria `9, 8, 7, 6, 5, 4, 3, 2, 11, 10, 1, 0`, com a escada quebrada.

---

### 5. Lado a lado com a "vizinha" crescente

```promql
sort_by_label(count by (version) (sort_by_label_desc_app_build_info), "version")
```

**Resultado esperado:** `2.9.1` (1), `2.10.3` (4), `2.11.0` (3), `2.11.0-rc.1` (1): exatamente o **inverso** do painel 2. Repare que o `-rc.1` continua "depois" do `2.11.0`: o problema de semver não depende da direção.

---

## 🏭 Casos reais

### 1. Painel "últimos backups" (alerta + inventário)

O alerta usa o valor (não precisa de ordem):

```yaml
- alert: BackupTooOld
  expr: time() - max(backup_last_success_timestamp_seconds) > 26 * 3600
  for: 15m
  annotations:
    summary: "Nenhum backup com sucesso há mais de 26h"
```

E a **tabela** do dashboard lista os mais recentes primeiro:

```promql
sort_by_label_desc(backup_last_size_bytes, "date")
```

**Decisão:** ordenar por label (data), e não por valor, porque o que importa na tabela é a **cronologia**; um backup pequeno demais de hoje (sinal de problema!) aparece em primeiro, onde todo mundo vê.

### 2. Qual versão está rodando onde (e a armadilha da pre-release)

```promql
sort_by_label_desc(count by (version) (app_build_info{app="api"}), "version")
```

Útil em *post-mortems* e em rollouts: "a 2.11.0 já está em todos os pods?". **Lembre:** se houver canaries com `-rc`, eles aparecem **acima** da versão final. Para versão "mais nova" de verdade, exporte labels numéricos (`major`, `minor`, `patch`) e ordene por eles, ou filtre `version!~".*-.*"`.

### 3. Tamanho por partição, maiores números primeiro

```promql
sort_by_label_desc(sum by (partition) (kafka_log_log_size{topic="orders"}), "partition")
```

Partições recém-adicionadas (números maiores) no topo, para ver se já estão recebendo dados depois de um aumento de partições. O alerta correspondente (sem ordenação, claro):

```yaml
- alert: KafkaPartitionEmpty
  expr: sum by (topic, partition) (kafka_log_log_size{topic="orders"}) == 0
  for: 1h
  annotations:
    summary: "Partição {{ $labels.partition }} de {{ $labels.topic }} vazia há 1h (produtor não está distribuindo?)"
```

---

## ✅ Quando usar

- Listas onde **o mais novo** está no nome: datas ISO, versões, números de build, partições/shards novos.
- **Ordem estável** decrescente em tabelas de inventário.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Ordem crescente por label | [`sort_by_label()`](../sort_by_label/) |
| Ordenar por **valor** | [`sort_desc()`](../sort_desc/) / [`sort()`](../sort/) |
| **Semver** com pre-releases | labels numéricos separados ou ordenação fora do Prometheus |
| Datas em formato não-ISO (`DD/MM/YYYY`) | corrija o formato na origem (ou `label_replace` para `YYYY-MM-DD`) |
| Gráficos | *Legend → Sort by* do Grafana |

## ⚠️ Pegadinhas

1. **Experimental:** precisa da feature flag; pode mudar/sumir.
2. **Só instant query**.
3. **Natural ≠ semver:** `2.11.0-rc.1` > `2.11.0`.
4. **Séries sem o label** vão para o **começo** na ordem decrescente (testado no v3.15.0).
5. **Maiúsculas/minúsculas:** comparação sensível a caixa (`a` > `Z` em ASCII).

## 🎓 Na prova PCA

- `sort_by_label_desc` = decrescente por label, ordem natural, experimental, só instant query.
- Saber diferenciar de `sort_desc` (valor) e de `topk` (seleção).

**1.** Partições `"0"`, `"2"`, `"10"`. Qual a saída de `sort_by_label_desc(x, "partition")`?

- A) 2, 10, 0
- B) 10, 2, 0
- C) 0, 2, 10
- D) 2, 0, 10

<details><summary>Resposta</summary>

**B.** Ordem natural decrescente compara os números como números.
</details>

**2.** Versões `2.11.0` e `2.11.0-rc.1`. Qual aparece primeiro em `sort_by_label_desc(x, "version")`?

- A) `2.11.0`, porque é a versão final
- B) `2.11.0-rc.1`, porque ordem natural não é semver
- C) Empate: a ordem é aleatória
- D) A query falha com labels não numéricos

<details><summary>Resposta</summary>

**B.**
</details>

**3.** Qual função retorna os backups do **mais recente para o mais antigo**, dado o label `date="YYYY-MM-DD"`?

- A) `sort_desc(backup_size_bytes)`
- B) `sort_by_label_desc(backup_size_bytes, "date")`
- C) `topk(1, backup_size_bytes)`
- D) `sort_by_label(backup_size_bytes, "date")`

<details><summary>Resposta</summary>

**B.** A) ordena pelo **tamanho**, não pela data; D) é crescente (mais antigo primeiro).
</details>

## 📝 Cola rápida

- `sort_by_label_desc(v, "l1", ...)`: **decrescente** por label, **ordem natural**.
- Datas ISO + `_desc` = "mais recente primeiro".
- `-rc.1` fica **acima** da versão final (natural ≠ semver).
- Experimental, só instant query.

## 🔗 Relacionadas

[`sort_by_label()`](../sort_by_label/) · [`sort_desc()`](../sort_desc/) · [`sort()`](../sort/) · [`label_replace()`](../label_replace/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#sort_by_label_desc
