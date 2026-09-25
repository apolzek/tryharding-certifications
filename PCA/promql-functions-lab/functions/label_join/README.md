# `label_join()`: juntar vários labels em um só

> **Em uma frase:** `label_join(v, "destino", "separador", "origem1", "origem2", ...)` cola os **valores** de vários labels com um separador e grava o resultado em um label de destino. O valor da série não muda.

| | |
|---|---|
| **Assinatura** | `label_join(v instant-vector, dst_label string, separator string, src_label_1 string, src_label_2 string, ...) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer uma (float e histogram são tratados igual): só mexe em **labels** |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-label_join |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o `CONCATENAR()` da planilha

Numa planilha você tem as colunas **Serviço** = `checkout` e **Ambiente** = `prod` e cria uma coluna nova com `=CONCATENAR(A2; "/"; B2)` → `checkout/prod`. Essa coluna vira uma **chave** para fazer `PROCV` em outra aba.

O `label_join` é esse `CONCATENAR`, feito em cada série:

```
entrada:  req{service="checkout", environment="prod"}  150
                    │                 │
                    └──── "/" ────────┘
                             ▼
saída:    req{service="checkout", environment="prod", key="checkout/prod"}  150
```

É o "irmão simples" do [`label_replace()`](../label_replace/): **sem regex**, só concatenação. Os labels de origem continuam na série.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Labels | Valor |
|---|---|---|
| `label_join_http_requests_total` (counter, imita `http_requests_total`) | `service="checkout"`, `environment="prod"`, `region="sa-east-1"` | cresce ≈ **150/s** |
| idem | `service="checkout"`, `environment="staging"`, **sem** `region` | ≈ **10/s** |
| idem | `service="payments"`, `environment="prod"`, `region="sa-east-1"` | ≈ **80/s** |
| idem | `service="payments"`, `environment="staging"`, **sem** `region` | ≈ **5/s** |
| `label_join_capacity_rps` (gauge) | `key="checkout/prod"` / `checkout/staging` / `payments/prod` / `payments/staging` | **200 / 50 / 100 / 20** |

A segunda métrica simula dados vindos de um **CMDB / planilha de capacity planning**, onde a capacidade é indexada por **uma** coluna composta `service/environment` (muito comum quando o dado vem de um exporter "genérico" de SQL/CSV).

> Por que `environment` e não `env`? Porque neste lab o próprio scrape já coloca `env="lab"` em tudo. Um `env` vindo do alvo seria renomeado para `exported_env`.

```bash
curl -s localhost:8088/metrics | grep '^label_join_'
# label_join_capacity_rps{key="checkout/prod"} 200
# label_join_http_requests_total{environment="prod",region="sa-east-1",service="checkout"} 91234
# label_join_http_requests_total{environment="staging",region="",service="checkout"} 6120
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-label_join
```

Espere ~1 minuto para o `rate(...[1m])` ter dados.

---

## 🔍 Queries passo a passo

### 1 e 2. Criando a chave `service/environment`

```promql
label_join(rate(label_join_http_requests_total[1m]), "key", "/", "service", "environment")
```

**O que faz:** primeiro o `rate` (label_join só aceita **instant vector**, e o `rate` devolve um), depois, para cada série, lê `service` e `environment` (nesta ordem), cola com `/` e grava em `key`.
**Resultado esperado (tabela):**

| service | environment | key (novo) | valor |
|---|---|---|---|
| checkout | prod | `checkout/prod` | ≈ 150 req/s |
| checkout | staging | `checkout/staging` | ≈ 10 |
| payments | prod | `payments/prod` | ≈ 80 |
| payments | staging | `payments/staging` | ≈ 5 |

A **ordem** dos labels de origem importa: `"environment", "service"` daria `prod/checkout`.

---

### 3. A chave na legenda

**Resultado esperado:** 4 linhas: `checkout/prod` ≈ 150, `payments/prod` ≈ 80, `checkout/staging` ≈ 10, `payments/staging` ≈ 5.

> 💡 Só para legenda, o Grafana já resolve com `{{service}}/{{environment}}`. O `label_join` brilha quando a chave precisa existir **como label**: para `on (...)`, `by (...)` ou para gravar numa recording rule.

---

### 4. O caso de uso matador: join por chave composta

```promql
label_join(rate(label_join_http_requests_total[1m]), "key", "/", "service", "environment")
  / on (key)
label_join_capacity_rps
```

**O que faz:** a capacidade só conhece `key="checkout/prod"`. Sem o `label_join`, não existe label em comum para o `on (...)`. Com ele, o pareamento é 1-para-1.
**Resultado esperado** (utilização):

| key | conta | utilização |
|---|---|---|
| `payments/prod` | 80 / 100 | ≈ **80%** |
| `checkout/prod` | 150 / 200 | ≈ **75%** |
| `payments/staging` | 5 / 20 | ≈ **25%** |
| `checkout/staging` | 10 / 50 | ≈ **20%** |

> Quando **o outro lado** tem a chave colada, quem se adapta é você, com `label_join`. Quando é você que tem a chave colada e quer **separar**, use [`label_replace()`](../label_replace/) com regex `(.*)/(.*)`.

---

### 5. Pegadinha: label ausente vira string vazia

```promql
label_join(rate(label_join_http_requests_total[1m]), "where", "/", "environment", "region")
```

**Resultado esperado:**

| environment | region | where |
|---|---|---|
| prod | sa-east-1 | `prod/sa-east-1` |
| staging | *(não existe)* | `staging/` ← separador sobrando! |

Não dá erro. O `label_join` trata label inexistente como `""` e **cola o separador mesmo assim**. Se **todos** os labels de origem estiverem ausentes, com separador `/` o resultado é `/` (e não vazio).

---

### 6. Sobrescrevendo o próprio label de origem

```promql
label_join(rate(label_join_http_requests_total[1m]), "service", "-", "service", "environment")
```

**Resultado esperado:** `service` passa a valer `checkout-prod`, `checkout-staging`, `payments-prod`, `payments-staging`. O destino pode ser um label que já existe, e ele é **substituído** (você perde o valor original).

---

## 🏭 Casos reais

### 1. Utilização contra capacidade de um CMDB (Black Friday)

O time de capacity planning mantém uma planilha "serviço/ambiente → req/s suportados", exportada por um `sql_exporter` como `service_capacity_rps{key="checkout/prod"}`. Na Black Friday o SRE quer um alerta de **80% da capacidade**:

```yaml
groups:
  - name: capacity
    rules:
      - record: service_environment:http_requests:rate5m
        expr: |
          label_join(
            sum by (service, environment) (rate(http_requests_total[5m])),
            "key", "/", "service", "environment"
          )
      - alert: ServiceNearCapacity
        expr: service_environment:http_requests:rate5m / on (key) service_capacity_rps > 0.8
        for: 10m
        annotations:
          summary: "{{ $labels.key }} usando {{ $value | humanizePercentage }} da capacidade"
```

**Decisão:** o `sum by` antes do `label_join` garante **uma** série por chave (senão o `on (key)` quebraria com "many-to-many"). O painel 4 deste lab é esse cálculo.

### 2. Identificador único de alvo para tabela de status

```promql
label_join(up == 0, "target", "@", "job", "instance")
```

Resultado: `target="node@10.0.0.5:9100"`, uma coluna só com "quem caiu", fácil de usar em tabela do Grafana ou em notificações. Como regra de alerta:

```yaml
- alert: TargetDown
  expr: label_join(up == 0, "target", "@", "job", "instance")
  for: 5m
  labels:
    severity: page
  annotations:
    summary: "Alvo {{ $labels.target }} fora do ar"
```

**Decisão:** o label `target` pode ser usado no `group_by` do Alertmanager para agrupar/silenciar "por alvo" com **uma** chave.

### 3. Chave para multi-cluster (Thanos / federação)

Com vários clusters, `namespace="shop"` existe em todos. Para um painel "top namespaces do planeta":

```promql
topk(10,
  label_join(sum by (cluster, namespace) (rate(container_cpu_usage_seconds_total[5m])),
             "cluster_ns", "/", "cluster", "namespace")
)
```

A legenda `{{cluster_ns}}` fica `prod-sa/shop`, `prod-us/shop`: sem ambiguidade.

---

## ✅ Quando usar

- **Chave composta para joins** com dados externos (CMDB, SLO por `service/env`, custos por `team:project`).
- **Criar um identificador único** para tabela ou alerta: `label_join(up, "target", "@", "job", "instance")`.
- **Recording rules** que precisam de um label "achatado" para outro sistema consumir.
- **Copiar um label** com outro nome: `label_join(v, "novo", "", "antigo")` (uma origem só, separador vazio).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Precisa **extrair um pedaço** de um label (porta, sufixo, versão) | [`label_replace()`](../label_replace/) |
| É só para a **legenda** do Grafana | `{{service}}/{{environment}}` no *Legend* |
| A chave composta deveria existir **sempre** | `relabel_configs` (`action: replace` com vários `source_labels` + `separator`) |
| Adicionar metadados de um info metric | [`info()`](../info/) |

## ⚠️ Pegadinhas

1. **Label ausente = `""`**, com o separador sobrando (`staging/`). Filtre antes (`{region!=""}`) ou trate depois com `label_replace(..., "where", "$1", "where", "(.*)/")`.
2. **A ordem das origens importa:** `service, environment` ≠ `environment, service`.
3. **Colisão:** se, depois de sobrescrever um label, duas séries ficarem idênticas, a query falha com `vector cannot contain metrics with the same labelset`.
4. **Separador ambíguo:** se os valores podem conter o separador (`a/b` + `c` e `a` + `b/c` dão `a/b/c`), escolha um separador que não aparece nos dados (`|`, `::`).
5. **Só instant vector:** `label_join(x[5m], ...)` é erro de tipo. Aplique **depois** do `rate`/`sum`.

## 🎓 Na prova PCA

O que costuma cair:

- **Assinatura:** `dst_label, separator, src_labels...` (número **variável** de origens).
- Diferença **label_join × label_replace** (concatenação × regex).
- Comportamento com **label ausente** (vira string vazia) e com **destino existente** (sobrescreve).
- Equivalente no **relabeling** (`source_labels` + `separator`, cujo separador padrão é `;`).

**1.** Qual o resultado de `label_join(up{job="api", instance="a:80"}, "id", "-", "job", "instance")`?

- A) `id="api-a:80"`
- B) `id="a:80-api"`
- C) `id="api;a:80"`
- D) Erro: `instance` contém `:`

<details><summary>Resposta</summary>

**A.** Concatena na ordem das origens com o separador dado.
</details>

**2.** A série `m{a="x"}` (sem label `b`). O que `label_join(m, "c", ",", "a", "b")` produz?

- A) A série sem o label `c`
- B) `c="x"`
- C) `c="x,"`
- D) Erro, label `b` inexistente

<details><summary>Resposta</summary>

**C.** Label ausente conta como string vazia e o separador é colado mesmo assim.
</details>

**3.** Você precisa de `version="1.2"` a partir de `image="app:1.2"`. Qual função?

- A) `label_join`
- B) `label_replace`
- C) `info`
- D) `sort_by_label`

<details><summary>Resposta</summary>

**B.** Extrair parte de um valor exige regex (`.*:(.*)`). `label_join` só concatena.
</details>

**4.** O que acontece em `label_join(m{service="a"}, "service", "-", "service", "zone")` com `zone="z1"`?

- A) Cria `service_1="a-z1"`
- B) `service` passa a ser `a-z1`
- C) Erro: destino já existe
- D) Nada muda

<details><summary>Resposta</summary>

**B.** O destino pode ser um label existente; ele é sobrescrito.
</details>

## 📝 Cola rápida

- `label_join(v, DESTINO, SEPARADOR, ORIGEM1, ORIGEM2, ...)`: sem regex, só concatena.
- Label ausente = `""` → separador sobrando (`staging/`).
- Destino existente é **sobrescrito**; ordem das origens importa.
- Uso nº 1: **chave composta** para `on (key)` com dados externos.
- Recebe **instant vector**: aplique depois de `rate`/`sum`.

## 🔗 Relacionadas

[`label_replace()`](../label_replace/) · [`info()`](../info/) · [`sort_by_label()`](../sort_by_label/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#label_join
