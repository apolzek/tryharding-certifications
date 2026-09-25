# `sort()`: ordenar as séries pelo valor (crescente)

> **Em uma frase:** `sort(v)` devolve as mesmas séries, **na ordem do menor para o maior valor**. Não muda nenhum número nem label, e **só faz efeito em consultas instantâneas** (tabelas, bar gauges, API `/api/v1/query`).

| | |
|---|---|
| **Assinatura** | `sort(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer float (gauge, resultado de `rate`, razão...) · ⚠️ amostras de **histogram** são ignoradas (somem do resultado) |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-sort |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o botão "Classificar" da planilha

Você tem uma planilha com o espaço livre de cada disco. Clicar em **Classificar do menor para o maior** não altera nenhuma célula: só muda a **ordem das linhas**, e o disco mais cheio vai para o topo. Isso é o `sort()`.

Agora imagine que você tira **uma foto da planilha a cada 15 segundos** e monta um filme (é isso que um **gráfico** faz: uma consulta por passo de tempo). No filme cada disco tem **uma linha** que atravessa o tempo todo, com a mesma cor. Não existe "a linha que está em primeiro": em cada instante a ordem seria diferente. Por isso o Prometheus **ignora** o `sort()` em range queries: a ordem da saída é fixa.

```
Consulta instantânea (tabela)          Range query (gráfico)
┌───────────────┬──────┐               100%┤  ╭─╮     node-b:/
│ node-b:/data  │  8 % │ ← mais cheio        │ ╭╯ ╰╮  ╭─ node-c:/
│ node-a:/      │ 21 % │                     │─╯   ╰──╯  ...
│ node-a:/data  │ 38 % │                   0%┼──────────────▶ t
│ node-c:/      │ 55 % │               sort() aqui = sem efeito
│ node-b:/      │ 81 % │
└───────────────┴──────┘
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica (imita...) | Labels | Comportamento |
|---|---|---|
| `sort_node_filesystem_avail_bytes` + `sort_node_filesystem_size_bytes` (node_exporter) | `node="node-a"`, `mountpoint="/"` | livre = **50 ± 35 %** (onda de 5 min), 100 GiB |
| idem | `node-a`, `/data` | **40 ± 10 %** (4 min), 500 GiB |
| idem | `node-b`, `/` | **70 ± 15 %** (3 min), 50 GiB |
| idem | `node-b`, `/data` | **20 ± 12 %** (5 min), 1 TiB |
| idem | `node-c`, `/` | **60 ± 30 %** (3m20s), 200 GiB |
| `sort_probe_ssl_earliest_cert_expiry` (blackbox_exporter) | `target="https://legacy.example.com"` ... | vence em **3 / 12 / 27 / 45 / 90** dias (legacy / api / status / shop / admin) |
| `sort_cronjob_last_duration_seconds` | `cronjob="backup"` / `report` / `cleanup` / `sync` | **120 / 45 / 8 / NaN** s |

Como os períodos são diferentes, as curvas de disco se **cruzam**: o ranking de "disco mais cheio" muda a cada minuto. O `sync` nunca rodou, então a duração dele é `NaN` (0/0).

```bash
curl -s localhost:8088/metrics | grep '^sort_'
# sort_node_filesystem_avail_bytes{mountpoint="/",node="node-a"} 2.5e+10
# sort_probe_ssl_earliest_cert_expiry{target="https://legacy.example.com"} 1.79064e+09
# sort_cronjob_last_duration_seconds{cronjob="sync"} NaN
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sort
```

Os dados aparecem em segundos. Para ver o ranking mudar, deixe a tela aberta alguns minutos (o dashboard atualiza a cada 10s).

---

## 🔍 Queries passo a passo

### 1. As curvas no tempo

```promql
100 * sort_node_filesystem_avail_bytes / sort_node_filesystem_size_bytes
```

**Resultado esperado:** 5 linhas ondulando entre ~8% e ~95%, **se cruzando**. Exemplo: `node-b:/data` passa boa parte do tempo lá embaixo (8% a 32%), mas quando `node-a:/` chega no vale (≈15%) ele vira o mais cheio.

---

### 2. Sem `sort`: ordem arbitrária

**Resultado esperado:** as 5 linhas numa ordem que **não tem significado** (costuma seguir a ordem interna das séries). Não confie nela: pode mudar entre versões e entre execuções.

---

### 3. `sort()`: mais cheio primeiro

```promql
sort(100 * sort_node_filesystem_avail_bytes / sort_node_filesystem_size_bytes)
```

**O que faz:** ordena pelo valor, crescente. Para "espaço livre", isso significa **o disco mais cheio no topo**, exatamente o que um plantonista quer ver.
**Resultado esperado** (exemplo de um instante):

| node | mountpoint | valor |
|---|---|---|
| node-b | /data | 9 % |
| node-a | / | 24 % |
| node-a | /data | 41 % |
| node-c | / | 63 % |
| node-b | / | 82 % |

Os números exatos dependem do momento, mas a coluna de valor está **sempre crescente**.

---

### 4. Certificados: o que vence primeiro

```promql
sort((sort_probe_ssl_earliest_cert_expiry - time()) / 86400)
```

**O que faz:** transforma o timestamp de expiração em "dias até vencer" e ordena. O menor número (mais urgente) vem primeiro.
**Resultado esperado:** `legacy` ≈ **3** → `api` ≈ **12** → `status` ≈ **27** → `shop` ≈ **45** → `admin` ≈ **90** (menos a fração do dia de hoje que já passou, ex.: 2.4, 11.4...).

---

### 5. `sort(bottomk(3, ...))`: os 3 piores, em ordem

```promql
sort(bottomk(3, 100 * sort_node_filesystem_avail_bytes / sort_node_filesystem_size_bytes))
```

**O que faz:** `bottomk` **escolhe** as 3 séries de menor valor, mas **não promete a ordem** de saída. O `sort` por fora garante que o bar gauge mostre do mais cheio para o menos cheio.
**Resultado esperado:** 3 barras, a menor em cima (ex.: 9% → 24% → 41%).

---

### 6. E o NaN?

```promql
sort(sort_cronjob_last_duration_seconds)
```

**Resultado esperado:**

| cronjob | valor |
|---|---|
| cleanup | 8 s |
| report | 45 s |
| backup | 120 s |
| sync | **NaN** ← sempre no fim |

`NaN` não é "menor que tudo" nem "maior que tudo": o Prometheus o coloca **no final**, tanto no `sort` quanto no [`sort_desc`](../sort_desc/).

---

### 7. `sort()` num gráfico

**Resultado esperado:** **idêntico** ao painel 1. A documentação é explícita: *"sort only affects the results of instant queries, as range query results always have a fixed output ordering"*. Para ordenar a **legenda** de um gráfico, use *Legend → Sort by* do Grafana.

> 🔎 **Olhe o ícone ⚠️ no título do painel:** o próprio Prometheus (v3.x) devolve um aviso na resposta da range query: `PromQL warning: sort is ineffective for range queries since results are always ordered by labels`. Ou seja: em range queries as séries voltam **sempre ordenadas pelo conjunto de labels, em ordem lexicográfica** (aqui: primeiro todos os `mountpoint="/"`, depois os `/data`, porque `mountpoint` vem antes de `node` em ordem alfabética de nome de label).

---

## 🏭 Casos reais

### 1. Painel "discos mais cheios" do plantão (node_exporter)

```promql
sort(
  bottomk(10,
    100 * node_filesystem_avail_bytes{fstype!~"tmpfs|overlay|squashfs"}
        / node_filesystem_size_bytes{fstype!~"tmpfs|overlay|squashfs"}
  )
)
```

Tabela com os 10 filesystems com **menos** espaço livre, o pior em cima. **Decisão:** o filtro de `fstype` tira os pseudo-filesystems (senão `tmpfs` de 64 MiB cheio domina a lista). O `sort` é o que garante a ordem na tabela; o `bottomk` só limita a quantidade.

A razão é usada no painel **e** no alerta, então vale uma recording rule (sem `sort`: regras não guardam ordem):

```yaml
groups:
  - name: disk
    rules:
      - record: instance_mountpoint:node_filesystem_avail:ratio
        expr: |
          node_filesystem_avail_bytes{fstype!~"tmpfs|overlay|squashfs"}
            / node_filesystem_size_bytes{fstype!~"tmpfs|overlay|squashfs"}
      - alert: DiskAlmostFull
        expr: instance_mountpoint:node_filesystem_avail:ratio < 0.10
        for: 15m
```

Painel: `sort(bottomk(10, 100 * instance_mountpoint:node_filesystem_avail:ratio))`.

### 2. Inventário de certificados que vão vencer (blackbox_exporter)

```promql
sort((probe_ssl_earliest_cert_expiry - time()) / 86400)
```

O alerta **não** precisa de `sort` (alertas são avaliados série a série); ele é só para o **painel** de inventário:

```yaml
- alert: SSLCertExpiringSoon
  expr: (probe_ssl_earliest_cert_expiry - time()) / 86400 < 14
  for: 1h
  annotations:
    summary: "Certificado de {{ $labels.instance }} vence em {{ $value | humanize }} dias"
```

### 3. Réplicas disponíveis: quem está mais degradado

```promql
sort(
  kube_deployment_status_replicas_available / kube_deployment_spec_replicas
)
```

Deployments com menor proporção de réplicas prontas no topo (0.33 = 1 de 3 prontas). **Cuidado:** um Deployment com `spec_replicas = 0` gera `0/0 = NaN`, que vai para o **fim** da lista, e não para o topo.

---

## ✅ Quando usar

- **Tabelas de "piores primeiro"** onde o menor é o problema: disco com menos espaço, certificado que vence antes, SLO com menos *error budget*, réplicas disponíveis.
- **Junto com `bottomk`/`topk`** para garantir a ordem de exibição.
- **Scripts e integrações via API** (`/api/v1/query`) que esperam uma lista ordenada (relatórios, ChatOps).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer o **maior primeiro** (Top N) | [`sort_desc()`](../sort_desc/) |
| Quer ordenar por **nome** (label), não por valor | [`sort_by_label()`](../sort_by_label/) |
| Quer só os N piores | `bottomk(N, v)` (e aí `sort()` por fora) |
| Painel de **gráfico** (timeseries) | *Legend → Sort by* no Grafana; o `sort` não tem efeito |
| Regras de **alerta** / recording rules | não use: a ordem não significa nada lá |

## ⚠️ Pegadinhas

1. **Range query ignora:** colocar `sort()` em gráfico não quebra nada, mas também não faz nada.
2. **NaN vai para o fim**, mesmo no `sort` crescente. Para escondê-lo: `sort(v == v)` (só o NaN é diferente de si mesmo).
3. **Histograms somem:** amostras de native histogram são ignoradas silenciosamente (removidas do resultado).
4. **Empates:** séries com o mesmo valor não têm ordem garantida entre si. Se precisa de estabilidade, use [`sort_by_label()`](../sort_by_label/).
5. **O Grafana pode reordenar:** algumas transformações (*Join by field*, *Reduce*...) ou um clique no cabeçalho da tabela mudam a ordem depois que o Prometheus respondeu.
6. **`topk`/`bottomk` não ordenam** de forma garantida: sempre combine com `sort`/`sort_desc` quando a ordem importa.

## 🎓 Na prova PCA

O que costuma cair:

- `sort` só afeta **instant queries**; em **range queries** a ordem é fixa.
- Ordem **crescente** (`sort`) × **decrescente** (`sort_desc`) × por **label** (`sort_by_label`, experimental).
- `sort` **não** filtra, **não** agrega, **não** altera valores: só ordena. Para limitar a N use `topk`/`bottomk`.
- Onde fica o **NaN** (fim).

**1.** O que `sort(node_load1)` faz num painel **timeseries** do Grafana (range query)?

- A) Ordena as linhas pelo último valor
- B) Ordena a legenda
- C) Nada: range queries têm ordem fixa
- D) Retorna erro

<details><summary>Resposta</summary>

**C.** A documentação diz que `sort` só afeta instant queries.
</details>

**2.** Qual query devolve os 5 filesystems com menos espaço livre, do mais cheio para o menos cheio?

- A) `sort(topk(5, node_filesystem_avail_bytes))`
- B) `sort(bottomk(5, node_filesystem_avail_bytes / node_filesystem_size_bytes))`
- C) `sort_desc(bottomk(5, node_filesystem_avail_bytes / node_filesystem_size_bytes))`
- D) `bottomk(5, sort(node_filesystem_size_bytes))`

<details><summary>Resposta</summary>

**B.** `bottomk` escolhe os 5 menores; `sort` coloca o menor (mais cheio) primeiro. A) pega os mais **vazios**; C) ordena ao contrário; D) usa o tamanho, não o espaço livre.
</details>

**3.** Em `sort(x)`, onde aparece uma série com valor `NaN`?

- A) Primeiro
- B) Último
- C) É removida
- D) Posição indefinida

<details><summary>Resposta</summary>

**B.** O Prometheus coloca NaN no final tanto no `sort` quanto no `sort_desc`.
</details>

**4.** Qual o tipo de entrada e de saída de `sort`?

- A) range vector → instant vector
- B) instant vector → instant vector
- C) instant vector → scalar
- D) scalar → instant vector

<details><summary>Resposta</summary>

**B.** Ex.: `sort(rate(x[5m]))` funciona; `sort(x[5m])` é erro de tipo.
</details>

## 📝 Cola rápida

- `sort(v)` = crescente pelo **valor**; `sort_desc(v)` = decrescente.
- **Só instant query** (tabela, bar gauge, API). Gráfico = ignorado.
- **NaN no fim**; histogramas são descartados.
- Não limita: combine `sort(bottomk(N, ...))` / `sort_desc(topk(N, ...))`.
- Não serve para alertas/recording rules.

## 🔗 Relacionadas

[`sort_desc()`](../sort_desc/) · [`sort_by_label()`](../sort_by_label/) · [`sort_by_label_desc()`](../sort_by_label_desc/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#sort
