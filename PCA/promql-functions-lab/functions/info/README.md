# `info()`: enriquecer séries com os labels de um *info metric* (sem `group_left`)

> **Em uma frase:** `info(v, [seletor])` procura, para cada série de `v`, o `target_info` com o **mesmo `job` e `instance`** e **copia os data labels** dele (cluster, namespace, versão...) para a série. É o join `* on(job, instance) group_left(...) target_info` em versão curta e mais robusta.

| | |
|---|---|
| **Assinatura** | `info(v instant-vector, [data-label-selector instant-vector]) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Qualquer uma em `v` (counter com `rate`, gauge, histogram). O info metric é um gauge de valor **1** |
| **Unidade do resultado** | a mesma de `v` (o valor **não muda**, só os labels) |
| **Dashboard** | http://localhost:3300/d/fn-info |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o crachá e a ficha do RH

Cada série é um **funcionário com crachá**: o crachá tem só **matrícula** (`job` + `instance`) e o que ele faz (`route="/api/orders"`).

O **RH** guarda uma **ficha** para cada matrícula (o `target_info`): cluster, namespace, versão do app... Essas informações **não** vêm no crachá, porque repetir tudo em toda métrica custaria caro (cardinalidade, armazenamento).

Quando você precisa do cargo junto do crachá, tem dois caminhos:

- **Jeito antigo (`group_left`)**: você vai ao RH, diz "a ficha fica na gaveta `target_info`, a matrícula é `job`+`instance`, quero só o campo `k8s_cluster_name`" e grampeia manualmente.
- **`info()`**: você diz "grampeia a ficha". Ele já sabe qual é a gaveta (`target_info`), qual é a matrícula (`job` + `instance`) e, se a ficha foi **atualizada** (nova versão), usa a **mais recente**.

```
info_http_server_requests_total{job="lab", instance="generator:8080", route="/api/orders"}  ← crachá
target_info{job="lab", instance="generator:8080", k8s_cluster_name="prod-sa-east-1", version="1.5.0"} 1  ← ficha
                                  │ mesma matrícula (job + instance)
                                  ▼
{job="lab", instance="generator:8080", route="/api/orders", k8s_cluster_name="prod-sa-east-1", version="1.5.0"}
```

**Identifying labels** = a matrícula (`job`, `instance`). **Data labels** = o resto da ficha (tudo que não é identificador).

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Labels | Valor |
|---|---|---|
| `target_info` (a **única** métrica do lab sem prefixo: é o nome padrão que o `info()` procura) | `k8s_cluster_name="prod-sa-east-1"`, `k8s_namespace_name="shop"`, `version` = **1.4.0 ↔ 1.5.0 a cada 3 min** | **1** |
| `info_build_info` | `goversion="go1.25.1"`, `revision="a1b2c3d"` | **1** |
| `info_http_server_requests_total` (counter) | `route="/api/orders"` / `"/api/users"` | cresce ≈ **20/s** / **5/s** |

`job="lab"` e `instance="generator:8080"` **não** são expostos pelo gerador: quem coloca é o **scrape** do Prometheus, exatamente como acontece com uma aplicação instrumentada com **OpenTelemetry** exportando para o Prometheus (os *resource attributes* do OTel viram `target_info`).

```bash
curl -s localhost:8088/metrics | grep -E '^(target_info|info_)'
# info_build_info{goversion="go1.25.1",revision="a1b2c3d"} 1
# info_http_server_requests_total{route="/api/orders"} 51234
# target_info{k8s_cluster_name="prod-sa-east-1",k8s_namespace_name="shop",version="1.5.0"} 1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-info
```

Espere ~1 minuto para o `rate(...[1m])` e ~6 minutos para ver a `version` trocar duas vezes.

---

## 🔍 Queries passo a passo

### 1. A ficha: `target_info`

```promql
target_info
```

**Resultado esperado:** **1** série com valor `1`: `job="lab"`, `instance="generator:8080"`, `k8s_cluster_name="prod-sa-east-1"`, `k8s_namespace_name="shop"`, `version="1.4.0"` ou `"1.5.0"`.

---

### 2. O jeito clássico: `group_left`

```promql
rate(info_http_server_requests_total[1m])
  * on (job, instance) group_left (k8s_cluster_name)
target_info
```

**O que faz:** multiplica cada série pelo `target_info` (que vale 1, então o valor não muda) casando por `job`+`instance`, e o `group_left(k8s_cluster_name)` copia esse label.
**Resultado esperado:** `/api/orders @ prod-sa-east-1` ≈ **20 req/s** e `/api/users @ prod-sa-east-1` ≈ **5 req/s**.

Funciona, mas repare quanto você precisa saber: o nome `target_info`, que a identidade é `job, instance`, a sintaxe `group_left`...

---

### 3. O mesmo com `info()` + seletor

```promql
info(rate(info_http_server_requests_total[1m]), {k8s_cluster_name=~".+"})
```

**O que faz:** o 2º argumento **não é um vetor de verdade**: é só uma lista de matchers entre chaves que diz **quais data labels** trazer (e de quais info metrics).
**Resultado esperado:** idêntico ao painel 2 (≈20 e ≈5 req/s, com `k8s_cluster_name="prod-sa-east-1"`). Só o `k8s_cluster_name` é adicionado; `version` e `k8s_namespace_name` não.

---

### 4. `info(v)` sem 2º argumento: todos os data labels

```promql
info(rate(info_http_server_requests_total[1m]))
```

**Resultado esperado (tabela):** 2 séries, cada uma com `route`, `job`, `instance` **mais** `k8s_cluster_name`, `k8s_namespace_name` e `version`.

---

### 5. Quando um data label muda (a "crise de identidade")

```promql
sum by (version) (info(rate(info_http_server_requests_total[1m])))
```

**O que faz:** a cada 3 min o `target_info` troca de `version` (1.4.0 → 1.5.0 → 1.4.0...). Para o Prometheus isso é **outra série** (a antiga termina, uma nova começa). O `info()` entende que é **a mesma ficha, com dados novos**, e usa sempre a mais recente.
**Resultado esperado:** gráfico empilhado com total ≈ **25 req/s**, em blocos de 3 minutos alternando entre `version=1.4.0` e `version=1.5.0`, como um bastão sendo passado.

> Com o `group_left` clássico, se a série antiga do `target_info` **não** for marcada como *stale* (acontece em alguns caminhos de ingestão, como remote-write/OTLP), durante até 5 min (o *lookback delta*) existem **duas** fichas para a mesma matrícula, e a query falha com `many-to-many matching not allowed`. O `info()` resolve o conflito escolhendo a mais nova. Neste lab o scrape marca a série antiga como stale corretamente, então o painel 2 também sobrevive à troca.

---

### 6. Usando outro info metric

```promql
info(rate(info_http_server_requests_total[1m]), {__name__=~"target_info|info_build_info"})
```

**Resultado esperado:** as 2 séries ganham os labels de **ambos**: `k8s_cluster_name`, `k8s_namespace_name`, `version` **e** `goversion="go1.25.1"`, `revision="a1b2c3d"`.
Por padrão, `info()` só considera `target_info`. Para outros, use um matcher em `__name__`.

---

### 7 e 8. Série sem ficha: mantém ou descarta?

```promql
info(up)                              # painel 7
info(up, {k8s_cluster_name=~".+"})    # painel 8
```

Aqui existem 2 alvos: `job="lab"` (tem `target_info`) e `job="prometheus"` (não tem).

**Resultado esperado:**

| query | `up{job="lab"}` | `up{job="prometheus"}` |
|---|---|---|
| `info(up)` | enriquecido | **mantido sem enriquecimento** |
| `info(up, {k8s_cluster_name=~".+"})` | só com `k8s_cluster_name` | **descartado** |

Regra da documentação: se o seletor tiver algum matcher que **não aceita string vazia** (como `=~".+"`), o enriquecimento vira **obrigatório** e séries sem info somem. Se todos os matchers aceitam vazio (`=~".*"`) ou não há seletor, a série volta como está.

---

### 9. O que dá errado sem `on`/`group_left`

```promql
rate(info_http_server_requests_total[1m]) * target_info                      # VAZIO
rate(info_http_server_requests_total[1m]) * on (job, instance) target_info   # ERRO
```

**Resultado esperado:**
- Sem `on (...)`: **vazio** ("No data"). O `*` tenta casar **todos** os labels; `route` só existe de um lado e `k8s_cluster_name` só do outro, então nenhum par é formado.
- Com `on (job, instance)` mas **sem** `group_left`: **erro** `multiple matches for labels: many-to-one matching must be explicit (group_left/group_right)`, porque há **2** séries (2 rotas) do lado esquerdo para **1** `target_info`.

São os dois erros mais comuns de quem escreve o join na mão, e o `info()` evita os dois.

---

## 🏭 Casos reais

### 1. Aplicações OpenTelemetry: taxa por cluster/namespace

Com o OTel Collector (ou OTLP direto no Prometheus), os *resource attributes* (`k8s.cluster.name`, `service.version`...) viram labels do `target_info`, e **não** de cada métrica. O dashboard multi-cluster:

```promql
sum by (k8s_cluster_name, k8s_namespace_name) (
  info(rate(http_server_request_duration_seconds_count[5m]), {k8s_cluster_name=~".+"})
)
```

E o alerta por cluster, com o nome do cluster na notificação:

```yaml
- alert: HighErrorRatePerCluster
  expr: |
    sum by (k8s_cluster_name) (info(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[5m]), {k8s_cluster_name=~".+"}))
      /
    sum by (k8s_cluster_name) (info(rate(http_server_request_duration_seconds_count[5m]), {k8s_cluster_name=~".+"}))
      > 0.05
  for: 10m
  annotations:
    summary: "Mais de 5% de erro no cluster {{ $labels.k8s_cluster_name }}"
```

Antes do `info()`, a query do painel era:

```promql
sum by (k8s_cluster_name, k8s_namespace_name) (
  rate(http_server_request_duration_seconds_count[5m])
  * on (job, instance) group_left (k8s_cluster_name, k8s_namespace_name) target_info
)
```

### 2. Taxa de erro por versão durante um deploy (canary)

```yaml
groups:
  - name: rollout
    rules:
      - record: version:http_server_errors:ratio_rate5m
        expr: |
          sum by (version) (info(rate(http_server_requests_total{code=~"5.."}[5m]), {version=~".+"}))
            /
          sum by (version) (info(rate(http_server_requests_total[5m]), {version=~".+"}))
```

**Decisão:** `{version=~".+"}` torna o enriquecimento **obrigatório**: alvos sem `target_info` não entram na razão por engano com `version=""`. E como `info()` sempre pega a ficha mais nova, a troca de versão no meio do deploy não quebra a regra com "many-to-many".

### 3. Juntar build info de exporters clássicos

```promql
info(up, {__name__="node_uname_info", nodename=~".+"})
```

Funciona **só** porque `node_uname_info` também é identificado por `job` + `instance`. Na versão atual, `info()` **sempre** usa `job` e `instance` como identificadores; para info metrics com outra identidade (ex.: `kube_pod_info`, identificado por `namespace`+`pod`), continue com `group_left`.

---

## ✅ Quando usar

- Métricas **OpenTelemetry** / OTLP, onde o contexto (cluster, versão, região) fica no `target_info`.
- Qualquer lugar onde você faria `* on(job, instance) group_left(...) target_info`.
- Quando os **data labels mudam** (versão, pod IP) e o `group_left` sofreria com "many-to-many" durante a transição.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Info metric identificado por **outros** labels (`kube_pod_info` → `namespace, pod`) | `* on (namespace, pod) group_left (node) kube_pod_info` |
| Prometheus sem feature flag / serviço gerenciado sem experimentais | `group_left` clássico |
| Só precisa renomear/extrair um label | [`label_replace()`](../label_replace/) |
| O label deveria estar em **todas** as métricas sempre | `promote_resource_attributes` (OTLP) ou relabeling no scrape |

## ⚠️ Pegadinhas

1. **Experimental:** o comportamento pode mudar e a função pode até ser removida.
2. **Identificadores fixos:** só `job` e `instance`. Não dá para dizer "use `pod`".
3. **Só `target_info` por padrão:** outros info metrics só com `{__name__=~"..."}`.
4. **Matcher negado sozinho** (`{__name__!="target_info"}`) faz o `info()` considerar **todas** as métricas terminadas em `_info` (menos a negada). Em ambientes grandes isso pode trazer labels inesperados, e dois info metrics com o **mesmo** data label (ex.: dois `version`) entram em conflito.
5. **Mantém ou descarta:** sem seletor → séries sem info são **mantidas** sem enriquecimento; com `=~".+"` → **descartadas**. Escolha de propósito.
6. **Séries de `v` que casam com o seletor** (ex.: você passou o próprio `target_info` em `v`) são tratadas como info series e voltam inalteradas.

## 🎓 Na prova PCA

Por ser experimental, `info()` aparece pouco; o que **cai muito** é o conceito por trás:

- O que é um **info metric** (gauge valor 1 com labels de metadados; ex.: `node_uname_info`, `kube_pod_info`, `target_info`, `*_build_info`).
- O join clássico `* on(...) group_left(...)`: por que `group_left` (muitos à esquerda, **um** info à direita) e por que multiplicar por 1 não muda o valor.
- Erros "many-to-many" quando a identidade do lado "um" não é única.

**1.** Por que se multiplica por um info metric (`* on(job, instance) group_left(version) build_info`)?

- A) Para converter a unidade
- B) Porque o info metric vale 1, então o valor não muda e o `group_left` copia labels
- C) Para somar as séries
- D) Para filtrar séries com valor 0

<details><summary>Resposta</summary>

**B.**
</details>

**2.** Quais labels o `info()` usa hoje como identificadores?

- A) `__name__`
- B) Todos os labels em comum
- C) `job` e `instance`
- D) Os definidos no 2º argumento

<details><summary>Resposta</summary>

**C.** O 2º argumento seleciona **data labels** e info metrics, não identificadores.
</details>

**3.** O alvo `X` não tem `target_info`. O que `info(up, {k8s_cluster_name=~".+"})` retorna para `up` de `X`?

- A) `up` sem enriquecimento
- B) `up` com `k8s_cluster_name=""`
- C) Nada: a série é descartada
- D) Erro

<details><summary>Resposta</summary>

**C.** O matcher `.+` não aceita vazio, então o enriquecimento é obrigatório.
</details>

**4.** Em `x * on(job, instance) group_left(k8s_cluster_name) target_info`, a versão do app (data label) mudou e a série antiga não ficou stale. O que pode acontecer?

- A) Nada
- B) Erro de matching many-to-many por até ~5 min (lookback delta)
- C) Valores dobrados
- D) O Prometheus apaga a série antiga

<details><summary>Resposta</summary>

**B.** Duas séries de `target_info` com o mesmo `job`/`instance` do lado "um". É exatamente o problema que o `info()` resolve.
</details>

## 📝 Cola rápida

- `info(v)` = `v * on(job, instance) group_left(<todos data labels>) target_info`, só que mais curto e robusto.
- 2º argumento `{...}`: escolhe data labels (`{k8s_cluster_name=~".+"}`) e info metrics (`{__name__=~"target_info|build_info"}`).
- Sem info: sem seletor → mantém; com `=~".+"` → descarta.
- Identificadores fixos `job` + `instance`; padrão só `target_info`; **experimental**.
- Info metric = gauge valor 1 com metadados nos labels.

## 🔗 Relacionadas

[`label_replace()`](../label_replace/) · [`label_join()`](../label_join/) · [`vector()`](../vector/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#info
