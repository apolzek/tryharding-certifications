# `label_replace()`: criar ou reescrever um label usando regex

> **Em uma frase:** `label_replace(v, "destino", "substituição", "origem", "regex")` aplica uma regex no valor do label `origem` de cada série e, **se casar**, grava a `substituição` (com `$1`, `$nome`...) no label `destino`. Se não casar, a série volta **intacta**.

| | |
|---|---|
| **Assinatura** | `label_replace(v instant-vector, dst_label string, replacement string, src_label string, regex string) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer uma (counter, gauge, histogram, native histogram): só mexe em **labels**, nunca no valor |
| **Unidade do resultado** | a mesma da entrada (o valor não muda) |
| **Dashboard** | http://localhost:3300/d/fn-label_replace |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o "Localizar e substituir" com regex

Todo editor de texto tem um **Localizar e substituir** com expressões regulares. Você escreve `(.*):.*`, manda substituir por `$1`, e `10.0.0.5:9100` vira `10.0.0.5`.

O `label_replace()` é exatamente isso, com duas diferenças importantes:

1. Ele **não apaga o original**: o resultado vai para um label de **destino** (que pode ser novo ou pode sobrescrever um existente).
2. A regex é **ancorada**: é como se ela tivesse `^...$` implícito. Ela precisa casar o valor **inteiro**, não um pedaço.

```
série de entrada:   cpu{endpoint="10.0.0.5:9100"}  70
                              │
             regex "(.*):.*"  │  $1 = "10.0.0.5"
                              ▼
série de saída:     cpu{endpoint="10.0.0.5:9100", internal_ip="10.0.0.5"}  70   ← valor igual!
```

Pense no `label_replace` como um **adaptador de tomada**: a métrica chega com um "plugue" (label) no formato errado e você converte para o formato que o resto da query (legenda, `sum by`, join) espera.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) imita três exporters reais de um cluster Kubernetes:

| Métrica (imita...) | Labels | Valor |
|---|---|---|
| `label_replace_node_cpu_usage_percent` (node_exporter) | `endpoint="10.0.0.5:9100"` / `10.0.0.6:9100` / `10.0.0.7:9100` | ≈ **70 / 40 / 20** % (onda ±5) |
| `label_replace_kube_node_info` (`kube_node_info`) | `node="worker-1"`, `internal_ip="10.0.0.5"` (e worker-2/.6, worker-3/.7) | **1** |
| `label_replace_container_memory_working_set_bytes` (cAdvisor) | `pod="checkout-7d9f8b6c4-x2k9p"`, `image="registry.local/shop/checkout:1.8.2"` | 3 pods checkout (200+220+180 MiB), 2 pods payments (300+340 MiB), `redis-0` (512 MiB) |
| `label_replace_http_requests_per_second` (app) | `service="checkout"` / `payments` / `cart` | ≈ **120 / 60 / 30** req/s |
| `label_replace_kube_deployment_status_replicas_available` (kube-state-metrics) | `deployment="checkout"` / `payments` / `cart` ⚠️ label **diferente** | **3 / 2 / 1** |

> Por que `endpoint` e não `instance`? Porque o Prometheus já coloca o label `instance` no scrape (aqui `instance="generator:8080"`); um `instance` vindo do alvo seria renomeado para `exported_instance`. A técnica é idêntica: `label_replace(up, "host", "$1", "instance", "(.*):.*")` funciona igualzinho no `instance` real.

```bash
curl -s localhost:8088/metrics | grep '^label_replace_'
# label_replace_kube_node_info{internal_ip="10.0.0.5",node="worker-1"} 1
# label_replace_container_memory_working_set_bytes{image="registry.local/shop/checkout:1.8.2",pod="checkout-7d9f8b6c4-x2k9p"} 2.09e+08
# label_replace_node_cpu_usage_percent{endpoint="10.0.0.5:9100"} 71.3
```

## ▶️ Como rodar

```bash
# na raiz do projeto
tools/deploy.sh
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-label_replace
```

Os dados aparecem em **segundos** (não há janela de tempo envolvida).

---

## 🔍 Queries passo a passo

### 1 e 2. Extrair o IP de `IP:porta`

```promql
label_replace_node_cpu_usage_percent                                                             # antes
label_replace(label_replace_node_cpu_usage_percent, "internal_ip", "$1", "endpoint", "(.*):.*")  # depois
```

**O que faz:** para cada série, pega o valor de `endpoint` e aplica `(.*):.*`. O grupo 1 (`(.*)`) captura tudo antes do `:`; `$1` grava isso em `internal_ip`.
**Resultado esperado (tabela):**

| endpoint | internal_ip (novo) | valor |
|---|---|---|
| `10.0.0.5:9100` | `10.0.0.5` | ≈ 70 |
| `10.0.0.6:9100` | `10.0.0.6` | ≈ 40 |
| `10.0.0.7:9100` | `10.0.0.7` | ≈ 20 |

Repare que `endpoint` **continua lá**: `label_replace` adiciona, não remove.

> 💡 Por que `(.*):.*` e não `(.*):9100`? Porque assim funciona com qualquer porta. E como o `.*` é "guloso", num IPv6 como `[::1]:9100` você pegaria `[::1]`: tudo antes do **último** `:`.

---

### 3. O join clássico: node_exporter × kube_node_info

```promql
label_replace(label_replace_node_cpu_usage_percent, "internal_ip", "$1", "endpoint", "(.*):.*")
  * on (internal_ip) group_left (node)
label_replace_kube_node_info
```

**O que faz:** `kube_node_info` vale sempre **1** (é um *info metric*), então multiplicar por ele **não muda o valor**; só serve para "puxar" o label `node`. O `on (internal_ip)` só funciona porque o `label_replace` criou um `internal_ip` **sem a porta**, no mesmo formato do outro lado.
**Resultado esperado:** 3 linhas com legenda **`worker-1` ≈ 70 %, `worker-2` ≈ 40 %, `worker-3` ≈ 20 %**. Ninguém no plantão pensa em "10.0.0.6:9100"; todo mundo sabe o que é o `worker-2`.

---

### 4. De pod para deployment (e somar)

```promql
sum by (deployment) (
  label_replace(label_replace_container_memory_working_set_bytes,
                "deployment", "$1", "pod", "(.*)-[a-z0-9]+-[a-z0-9]+")
)
```

**O que faz:** pods de Deployment no Kubernetes têm o formato `<deployment>-<hash do ReplicaSet>-<sufixo aleatório>`. A regex joga fora os dois últimos pedaços e guarda o resto em `deployment`. Depois o `sum by (deployment)` agrupa.
**Resultado esperado:**

| deployment | memória |
|---|---|
| `checkout` | ≈ **600 MiB** (200 + 220 + 180) |
| `payments` | ≈ **640 MiB** (300 + 340) |
| *(vazio)* | ≈ **512 MiB**: é o `redis-0` |

⚠️ **Por que o redis ficou sem nome?** `redis-0` é um pod de **StatefulSet** (`<nome>-<ordinal>`), só tem **um** hífen. A regex exige dois, então **não casa**, e a série volta sem o label `deployment`. No `sum by (deployment)` ela cai no grupo "sem deployment". Solução: uma segunda regex, ex. `(.*)-[0-9]+` para StatefulSets, ou usar o label `created_by_name` do `kube_pod_info`.

---

### 5. Grupos de captura nomeados

```promql
label_replace(label_replace_container_memory_working_set_bytes,
              "app_version", "$name@$tag",
              "image", ".*/(?P<name>[^:]+):(?P<tag>.+)")
```

**O que faz:** `(?P<name>...)` dá **nome** ao grupo. Na substituição você usa `$name` em vez de `$1`, o que fica muito mais legível quando há vários grupos.
**Resultado esperado:** `app_version="checkout@1.8.2"` (3 pods), `payments@2.1.0` (2 pods), `redis@7.2` (`docker.io/library/redis:7.2`).

⚠️ `$name_v` **não** é "`$name` seguido de `_v`": o Go lê como um grupo chamado `name_v`, que não existe, e expande para **vazio** (o label nem é criado). Use chaves: `${name}_v`.

---

### 6. A regex é ancorada

```promql
label_replace(label_replace_container_memory_working_set_bytes, "team", "loja", "pod", "checkout")
```

**Resultado esperado:** as 6 séries voltam **sem** o label `team`. `checkout` não casa com `checkout-7d9f8b6c4-x2k9p` porque a regex vira `^(?s:checkout)$`. Com `"checkout.*"` os 3 pods do checkout ganhariam `team="loja"`.

---

### 7 e 8. Join entre métricas com nomes de label diferentes

```promql
label_replace_http_requests_per_second                    # labels: service="checkout"
  / on (service)
label_replace(label_replace_kube_deployment_status_replicas_available,   # labels: deployment="checkout"
              "service", "$1", "deployment", "(.*)")
```

**O que faz:** a aplicação chama o serviço de `service`, o kube-state-metrics chama de `deployment`. Para o `on (service)` parear, copiamos `deployment` → `service` (regex `(.*)` = "copie tudo").
**Resultado esperado** (req/s por réplica):

| service | conta | resultado |
|---|---|---|
| `checkout` | 120 / 3 | ≈ **40** |
| `payments` | 60 / 2 | ≈ **30** |
| `cart` | 30 / 1 | ≈ **30** |

**Painel 8:** e sem o `label_replace`? Depende de quantas séries há do lado direito:

- `... / on (service) label_replace_kube_deployment_status_replicas_available{deployment="checkout"}` → **vazio** ("No data"). Do lado direito `service` não existe (vale `""`); do esquerdo vale `checkout`/`payments`/`cart`: nenhum par. É o bug **silencioso** mais comum de join.
- `... / on (service) label_replace_kube_deployment_status_replicas_available` (as 3 séries) → **erro**: `found duplicate series for the match group {} on the right hand-side`. As 3 séries da direita têm o mesmo `service=""`, então o Prometheus não sabe qual usar.

---

## 🏭 Casos reais

### 1. Dashboard de nós com nome em vez de IP (node_exporter + kube-state-metrics)

O `instance` do node_exporter é `10.0.0.5:9100`; o `kube_node_info` tem `node="worker-1"` e `internal_ip="10.0.0.5"`. Para o plantonista ver "worker-1":

```promql
label_replace(
  1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])),
  "internal_ip", "$1", "instance", "(.*):.*"
)
* on (internal_ip) group_left (node)
kube_node_info
```

É exatamente o painel 3 deste lab. **Decisão:** multiplicar por um info metric (valor 1) é o padrão para "importar" labels; o `label_replace` só existe para alinhar o formato da chave.

### 2. Custo de memória por Deployment (FinOps)

cAdvisor só conhece `pod`. O time de FinOps quer "memória por aplicação". Recording rule:

```yaml
groups:
  - name: finops
    rules:
      - record: deployment:container_memory_working_set_bytes:sum
        expr: |
          sum by (namespace, deployment) (
            label_replace(
              container_memory_working_set_bytes{container!="", container!="POD"},
              "deployment", "$1", "pod", "(.*)-[a-z0-9]{8,10}-[a-z0-9]{5}"
            )
          )
```

**Decisão:** a regex mais específica (`{8,10}` / `{5}`, formato real dos sufixos do Kubernetes) evita cortar nomes que têm hífen, como `order-api-v2`. Em clusters com kube-state-metrics, o join com `kube_pod_owner` é ainda mais robusto (não depende do formato do nome).

### 3. Alerta com legenda humana: disco por host sem porta

```yaml
- alert: NodeDiskAlmostFull
  expr: |
    label_replace(
      node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes < 0.10,
      "host", "$1", "instance", "(.*):.*"
    )
  for: 10m
  annotations:
    summary: "{{ $labels.host }}:{{ $labels.mountpoint }} com menos de 10% livre"
```

**Decisão:** o label `host` aparece na notificação do Alertmanager e pode ser usado em `group_by`. Se isso vale para **todas** as queries, o melhor é fazer no `relabel_configs` do scrape (uma vez só, na ingestão).

### 4. SLO de API com versão extraída do path

`http_requests_total{handler="/api/v2/orders"}` → quero erro **por versão da API**:

```promql
sum by (api_version) (
  label_replace(rate(http_requests_total{code=~"5.."}[5m]), "api_version", "$1", "handler", "/api/(v[0-9]+)/.*")
)
```

---

## ✅ Quando usar

- **Legendas legíveis:** tirar porta do `instance`, encurtar nomes enormes.
- **Agrupar por algo que está "escondido" dentro de outro label:** pod → deployment, `topic="orders.v2.dlq"` → `domain="orders"`, `handler="/api/v1/users"` → `api_version="v1"`.
- **Join entre métricas de exporters diferentes** (`app` vs `service`, `node` vs `instance`, `internal_ip` vs `instance`).
- **Renomear um label:** copie para o novo nome com `"(.*)"`/`$1`.
- **Remover um label:** substituição vazia, `label_replace(v, "pod", "", "pod", ".*")` (label com valor vazio = label inexistente).
- **Criar um label constante:** `label_replace(vector(1), "status", "ok", "", "")`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só **concatenar** vários labels com um separador | [`label_join()`](../label_join/) (mais simples, sem regex) |
| O label errado vem de **todos** os dados, sempre | `relabel_configs` / `metric_relabel_configs` no scrape: corrija na **ingestão**, uma vez só |
| Pegar metadados (cluster, versão) de um "info metric" | [`info()`](../info/) ou `* on(...) group_left(...)` |
| Filtrar séries por regex | um seletor: `metric{pod=~"checkout-.*"}` |

## ⚠️ Pegadinhas

1. **Regex ancorada:** `"checkout"` só casa com o valor exato `checkout`. Use `.*` nas pontas quando quiser "contém" (painel 6).
2. **Não casou = volta igual:** não dá erro e não some. As séries que "escaparam" (como o `redis-0`) aparecem sem o label e bagunçam agrupamentos (painel 4).
3. **`$1_suffix` vira vazio:** use `${1}_suffix` / `${name}_suffix`.
4. **Escape em strings PromQL:** dentro de `"..."`, `\d` precisa ser escrito `\\d`. Alternativas: usar classes (`[0-9]`) ou crases: `` `(\d+)` ``.
5. **Colisão de séries:** se o `label_replace` sobrescrever um label e duas séries ficarem com o **mesmo** conjunto de labels, a query falha com `vector cannot contain metrics with the same labelset`.
6. **Custo:** roda em **cada avaliação** e em **cada série**. Em dashboards com milhares de séries, prefira resolver no relabeling do scrape.
7. **O nome da métrica é mantido:** diferente das funções matemáticas, `label_replace` preserva o `__name__` (e você pode até mexer nele com `dst_label="__name__"`).

## 🎓 Na prova PCA

O que costuma cair:

- **Ordem dos argumentos:** `dst_label, replacement, src_label, regex` (destino **antes** da origem, é o contrário do que muita gente espera).
- **Regex ancorada** e comportamento quando **não casa** (série volta inalterada, sem erro).
- **Entrada e saída são instant vectors**; `label_replace` não aceita range vector (`label_replace(x[5m], ...)` é erro de tipo).
- Diferença para `label_join` e para **relabeling** (`relabel_configs` acontece na ingestão, `label_replace` na consulta).

**1.** Qual o resultado de `label_replace(up{instance="db:5432"}, "port", "$1", "instance", "[a-z]+")`?

- A) Série com `port="db"`
- B) Série com `port="5432"`
- C) A série sem o label `port`
- D) Erro de execução

<details><summary>Resposta</summary>

**C.** A regex é ancorada: `[a-z]+` precisaria casar `db:5432` inteiro, e não casa. Quando não casa, a série volta **inalterada**, sem erro.
</details>

**2.** Qual a ordem correta dos parâmetros?

- A) `label_replace(v, src_label, regex, dst_label, replacement)`
- B) `label_replace(v, dst_label, replacement, src_label, regex)`
- C) `label_replace(v, regex, replacement, src_label, dst_label)`
- D) `label_replace(v, dst_label, src_label, regex, replacement)`

<details><summary>Resposta</summary>

**B.** Destino e substituição primeiro, depois origem e regex.
</details>

**3.** Você quer somar memória por deployment, mas só tem o label `pod="api-6d4cf56db6-2xk8p"`. Qual query?

- A) `sum by (deployment) (label_join(mem, "deployment", "-", "pod"))`
- B) `sum by (deployment) (label_replace(mem, "deployment", "$1", "pod", "(.*)-[^-]+-[^-]+"))`
- C) `sum by (pod) (mem)`
- D) `sum(label_replace(mem, "pod", "$1", "deployment", "(.*)"))`

<details><summary>Resposta</summary>

**B.** Extrai tudo antes dos dois últimos segmentos. A) só copiaria o pod inteiro; C) não agrupa por deployment; D) inverte origem/destino (e `deployment` nem existe).
</details>

**4.** Qual a forma de **remover** o label `pod` de todas as séries com `label_replace`?

- A) `label_replace(v, "pod", "", "pod", ".*")`
- B) `label_replace(v, "", "pod", ".*", "pod")`
- C) `label_replace(v, "pod", "null", "pod", "")`
- D) Não é possível

<details><summary>Resposta</summary>

**A.** Gravar string vazia em um label equivale a removê-lo. (C criaria `pod="null"`... só nas séries onde `pod` é vazio.)
</details>

**5.** `x / on (service) y` retorna vazio. `x` tem `service`, `y` tem `app` com os mesmos valores. O que resolve?

- A) Trocar `on (service)` por `ignoring (service)`
- B) `x / on (service) label_replace(y, "service", "$1", "app", "(.*)")`
- C) `x / on (app) y`
- D) `label_join(x / y, "service", "", "app")`

<details><summary>Resposta</summary>

**B.** Copia `app` → `service` em `y`, criando a chave comum para o `on`.
</details>

## 📝 Cola rápida

- `label_replace(v, DESTINO, SUBSTITUIÇÃO, ORIGEM, REGEX)`: destino primeiro!
- Regex **ancorada**; não casou → série **inalterada** (sem erro).
- `$1`, `$nome`, `${1}_x`; substituição vazia **remove** o label.
- Uso nº 1: alinhar labels para **join** (`on (...) group_left (...)`); uso nº 2: `sum by` de algo escondido num label.
- Se é sempre necessário, faça no **relabel_configs** (ingestão), não na query.

## 🔗 Relacionadas

[`label_join()`](../label_join/) · [`info()`](../info/) · [`sort_by_label()`](../sort_by_label/) · [`vector()`](../vector/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#label_replace
