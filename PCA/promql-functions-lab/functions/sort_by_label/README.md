# `sort_by_label()`: ordenar pelo nome (label), em ordem natural

> **Em uma frase:** `sort_by_label(v, "label1", "label2", ...)` ordena as séries de forma **crescente pelos valores dos labels** indicados, usando **ordem natural** (`node-2` antes de `node-10`). Empates são desfeitos pelo conjunto completo de labels. Só afeta **consultas instantâneas**.

| | |
|---|---|
| **Assinatura** | `sort_by_label(v instant-vector, label string, ...) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Qualquer uma: float **e** histogram (diferente do `sort`, histogramas **não** são descartados) |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-sort_by_label |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a lista de arquivos do seu computador

Salve `foto1.jpg`, `foto2.jpg` ... `foto10.jpg` numa pasta. O explorador de arquivos mostra `foto1, foto2, ..., foto9, foto10`. Isso é **ordem natural**: os trechos numéricos são comparados **como números**.

Um programa "ingênuo" (ordem alfabética, byte a byte) mostraria `foto1, foto10, foto2, foto3...`, porque o caractere `1` vem antes do `2`, e ele nem olha o resto.

```
Alfabética (ingênua)     Natural (sort_by_label)
node-1                   node-1
node-10   ← estranho     node-2
node-11                  node-3
node-12                  ...
node-2                   node-9
node-3                   node-10
...                      node-11
node-9                   node-12
```

O `sort_by_label` é o `sort` que olha para o **nome** em vez do **valor**, e faz isso do jeito que um humano espera.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica (imita...) | Labels | Valor |
|---|---|---|
| `sort_by_label_node_load1` (`node_load1`) | `zone` (`us-east-1a` para ímpares, `us-east-1b` para pares), `node="node-1"` ... `node-12` | "embaralhado" de propósito (± 0.05): node-1 **1.8**, node-2 **0.4**, node-3 **2.6**, node-4 **1.1**, node-5 **0.7**, node-6 **2.9**, node-7 **1.5**, node-8 **0.2**, node-9 **2.2**, node-10 **3.1**, node-11 **0.9**, node-12 **1.3** |
| `sort_by_label_app_build_info` (estilo `*_build_info`) | `pod`, `version` | **1** por pod: 1 pod em `1.2.0`, 2 em `1.9.2`, 5 em `1.10.0`, 3 em `1.10.12` |

```bash
curl -s localhost:8088/metrics | grep '^sort_by_label_' | head -4
# sort_by_label_app_build_info{pod="shop-1.10.0-0",version="1.10.0"} 1
# sort_by_label_node_load1{node="node-10",zone="us-east-1b"} 3.12
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sort_by_label
```

Os dados aparecem em segundos.

---

## 🔍 Queries passo a passo

### 🟢 1. Um label: `node`

```promql
sort_by_label(sort_by_label_node_load1, "node")
```

**Resultado esperado (tabela):** `node-1` (≈1.8), `node-2` (≈0.4), `node-3` (≈2.6), ..., `node-9` (≈2.2), `node-10` (≈3.1), `node-11` (≈0.9), `node-12` (≈1.3). Doze linhas, com o número do nó crescendo **como número**, e a coluna de valor "pulando" (porque a ordem **não** é pelo valor).

---

### 🟡 2. Lado a lado: por valor (`sort_desc`)

```promql
sort_desc(sort_by_label_node_load1)
```

**Resultado esperado:** `node-10` (3.1), `node-6` (2.9), `node-3` (2.6), `node-9` (2.2), `node-1` (1.8), `node-7` (1.5), `node-12` (1.3), `node-4` (1.1), `node-11` (0.9), `node-5` (0.7), `node-2` (0.4), `node-8` (0.2).

São **perguntas diferentes**: `sort_desc` responde "quem está pior?" (e a ordem muda quando os valores mudam); `sort_by_label` responde "onde está o node-7?" (ordem **estável**).

---

### 🟡 3. Vários labels: `zone`, depois `node`

```promql
sort_by_label(sort_by_label_node_load1, "zone", "node")
```

**O que faz:** ordena por `zone`; dentro da mesma zona, por `node`. Como um `ORDER BY zone, node` do SQL.
**Resultado esperado:**

| zone | node (nesta ordem) |
|---|---|
| us-east-1a | node-1, node-3, node-5, node-7, node-9, node-11 |
| us-east-1b | node-2, node-4, node-6, node-8, node-10, node-12 |

---

### 🔴 4. Versões: ordem natural acerta (quase sempre)

```promql
sort_by_label(count by (version) (sort_by_label_app_build_info), "version")
```

**Resultado esperado:**

| version | pods |
|---|---|
| 1.2.0 | 1 |
| 1.9.2 | 2 |
| 1.10.0 | 5 |
| 1.10.12 | 3 |

Com ordem alfabética, `1.10.0` e `1.10.12` apareceriam **antes** de `1.2.0`. A ordem natural compara `2 < 9 < 10` como números. (Veja a pegadinha de pre-release em [`sort_by_label_desc()`](../sort_by_label_desc/).)

---

### 🟡 5. Num gráfico: sem efeito (o "uso errado")

```promql
sort_by_label(sort_by_label_node_load1, "node")   # num painel timeseries
```

**Resultado esperado:** as 12 linhas aparecem, mas a legenda vem em ordem **lexicográfica**: `node-1, node-10, node-11, node-12, node-2, node-3...`, e não na ordem natural. Para a legenda, use *Legend → Sort by* ou *Overrides* no Grafana.

> 🔎 **Olhe o ícone ⚠️ no título do painel:** o próprio Prometheus (v3.x) devolve um aviso na resposta da range query: `PromQL warning: sort is ineffective for range queries since results are always ordered by labels`. Ou seja: em range queries as séries voltam **sempre ordenadas pelo conjunto de labels, em ordem lexicográfica** (`node-1, node-10, node-11, node-12, node-2...`).

---

## 🏭 Casos reais

### 1. Tabela de inventário de nós estável (sem "pular" a cada refresh)

Uma tabela `node_load1` ordenada por **valor** (`sort_desc`) muda a posição das linhas a cada refresh, o que é ótimo para "quem está pior", mas péssimo para "quero achar o `worker-7`". Para inventário:

```promql
sort_by_label(
  node_load1 * on (instance) group_left (nodename) node_uname_info,
  "nodename"
)
```

**Decisão:** ordem **estável** e previsível (`worker-2` antes de `worker-10`), independentemente do valor. Como o join é caro e usado em vários painéis, vale gravar numa recording rule e ordenar só na hora de exibir (ordenação **não** se grava: recording rules não guardam ordem):

```yaml
groups:
  - name: node-inventory
    rules:
      - record: nodename:node_load1:with_name
        expr: node_load1 * on (instance) group_left (nodename) node_uname_info
```

Painel: `sort_by_label(nodename:node_load1:with_name, "nodename")`.

### 2. Quantos pods em cada versão durante um rollout

```promql
sort_by_label(count by (version) (app_build_info{app="checkout"}), "version")
```

Durante um *rolling update* você vê `1.9.2: 2`, `1.10.0: 5`... e acompanha a versão nova ganhando pods. Ordem natural evita o `1.10.0` aparecendo antes do `1.9.2`. O alerta que acompanha a tabela:

```yaml
- alert: RolloutStuck
  expr: count(count by (version) (app_build_info{app="checkout"})) > 1
  for: 30m
  annotations:
    summary: "checkout com mais de uma versão rodando há 30 min (rollout travado?)"
```

O alerta não ordena nada; quando ele toca, o plantonista abre a tabela ordenada por versão (painel 4 do lab) para ver **quantos** pods ainda estão na versão velha.

### 3. Relatório por partição/shard

```promql
sort_by_label(sum by (topic, partition) (kafka_log_log_size{topic="orders"}), "topic", "partition")
```

Partições `0, 1, 2, ..., 10, 11` em ordem numérica, prontas para um relatório ou CSV exportado da API.

---

## ✅ Quando usar

- **Tabelas de inventário** (nós, pods, partições, shards) onde o usuário procura pelo **nome**.
- **Ordem estável** entre refreshes (não depende do valor, que muda o tempo todo).
- **Versões** e nomes numerados (`node-2` < `node-10`).
- **Desempate** determinístico de outros rankings.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Ordenar pelo **valor** | [`sort()`](../sort/) / [`sort_desc()`](../sort_desc/) |
| Mais novo / maior número primeiro | [`sort_by_label_desc()`](../sort_by_label_desc/) |
| Painel de gráfico (timeseries) | *Legend → Sort by* do Grafana |
| Precisa de **semver** correto com pre-releases (`-rc.1`) | ordene fora do Prometheus, ou use labels separados (`major`, `minor`, `patch`) |
| Prometheus sem a feature flag (ex.: serviço gerenciado sem experimentais) | a transformação *Sort by* do Grafana na tabela |

## ⚠️ Pegadinhas

1. **É experimental:** sem `--enable-feature=promql-experimental-functions` a query falha com erro de parse (função desconhecida). Pode mudar ou sumir em versões futuras.
2. **Só instant query:** em gráfico, nada acontece.
3. **Label inexistente** vai para o **fim** na ordem crescente (testado no v3.15.0: `sort_by_label` coloca as séries sem o label depois de todas as outras; no `_desc` elas vão para o começo).
4. **Natural ≠ semver:** `2.11.0-rc.1` é considerado **maior** que `2.11.0` (o prefixo igual + "mais caracteres").
5. **Maiúsculas vêm antes:** a comparação diferencia maiúsculas de minúsculas (`Node-4` aparece **antes** de `node-3`, porque `N` < `n` na tabela ASCII).
6. **Ao contrário do `sort`, histogramas NÃO são descartados**: a documentação diz que float e histogram são tratados do mesmo jeito.

## 🎓 Na prova PCA

O que costuma cair (funções experimentais aparecem menos, mas o conceito de ordenação sim):

- `sort_by_label` ordena por **label**; `sort`/`sort_desc` por **valor**.
- Usa **ordem natural**; aceita **vários labels** (desempate em cascata).
- Só em **instant queries**; exige feature flag.

**1.** Qual a saída de `sort_by_label(up, "instance")` para `instance` = `host-10`, `host-2`, `host-1`?

- A) host-1, host-10, host-2
- B) host-1, host-2, host-10
- C) host-10, host-2, host-1
- D) A ordem original

<details><summary>Resposta</summary>

**B.** Ordem natural: os números são comparados como números.
</details>

**2.** O que é necessário para usar `sort_by_label` no Prometheus 3.x?

- A) Nada, é estável
- B) `--enable-feature=promql-experimental-functions`
- C) `--enable-feature=native-histograms`
- D) Só funciona via recording rule

<details><summary>Resposta</summary>

**B.**
</details>

**3.** Em `sort_by_label(x, "zone", "node")`, qual o papel de `node`?

- A) Ignorado: só o primeiro label conta
- B) Critério de desempate dentro da mesma `zone`
- C) Filtra as séries que têm `node`
- D) Ordena `node` de forma decrescente

<details><summary>Resposta</summary>

**B.** Funciona como `ORDER BY zone, node`.
</details>

**4.** Você quer que uma tabela de nós **não mude de ordem** a cada refresh. Qual a melhor escolha?

- A) `sort_desc(node_load1)`
- B) `topk(100, node_load1)`
- C) `sort_by_label(node_load1, "instance")`
- D) `sort(node_load1)`

<details><summary>Resposta</summary>

**C.** A ordem por label é estável; por valor muda sempre que os valores mudam.
</details>

## 📝 Cola rápida

- `sort_by_label(v, "l1", "l2", ...)`: crescente por **label**, **ordem natural** (`2 < 10`).
- Desempate: próximo label da lista, depois o conjunto completo de labels.
- **Experimental** (feature flag) e **só instant query**.
- Natural ≠ semver (`-rc.1` > final).

## 🔗 Relacionadas

[`sort_by_label_desc()`](../sort_by_label_desc/) · [`sort()`](../sort/) · [`sort_desc()`](../sort_desc/) · [`label_replace()`](../label_replace/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#sort_by_label
