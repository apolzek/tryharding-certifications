# `clamp_min()`: um piso (nada fica abaixo do mínimo)

> **Em uma frase:** `clamp_min(v, min)` troca todo valor **menor que `min`** por `min` e deixa o resto intacto. Os três usos clássicos: **zerar negativos depois de uma subtração**, **evitar divisão por zero** (`x / clamp_min(y, 1)`) e **cortar previsões impossíveis** (`clamp_min(predict_linear(...), 0)`).

| | |
|---|---|
| **Assinatura** | `clamp_min(v instant-vector, min scalar) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (diferenças, divisores, previsões) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada |
| **Dashboard** | http://localhost:3300/d/fn-clamp_min |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o chão do prédio

O elevador pode subir até o último andar, mas **não afunda abaixo do térreo**. Se alguém apertar "-6", ele para no térreo.

```
  valor:    -6    -1     0     3     8
             │     │
  clamp_min(v, 0):
             0     0     0     3     8
             └─────┴── "chão" em 0
```

Em PromQL isso aparece o tempo todo porque **subtrações e previsões** geram números negativos que **não existem fisicamente**: "sobram -6 cores", "haverá -20 GiB livres", "faltam -3 minutos". E porque **dividir por zero** dá `+Inf` (ou `NaN` para `0/0`).

Regras especiais **da documentação**:
- `min = NaN` → resultado **NaN**;
- `min = -Inf` → valores **inalterados**;
- `min = +Inf` → **todos** os valores viram `+Inf`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `clamp_min_kube_resourcequota{namespace="loja",resource="requests.cpu",type="hard"}` | `kube_resourcequota` | gauge | **20** cores (quota recém-reduzida) |
| `...{type="used"}` | idem | gauge | oscila entre **12 e 26** cores (4 min) |
| `clamp_min_rabbitmq_queue_messages{queue="emails"}` | `rabbitmq_queue_messages` | gauge | **~600** mensagens |
| `clamp_min_kube_deployment_status_replicas_available{deployment="email-worker"}` | `kube_deployment_status_replicas_available` | gauge | **3**, mas **0 por 30 s a cada 3 min** (rollout ruim) |
| `clamp_min_node_filesystem_avail_bytes{mountpoint="/var/lib/postgresql"}` | `node_filesystem_avail_bytes` | gauge | cai de **40 GiB a 4 GiB** em 15 min (**2.4 GiB/min**) e recomeça (limpeza) |

```bash
curl -s localhost:8088/metrics | grep '^clamp_min_'
# clamp_min_kube_resourcequota{namespace="loja",resource="requests.cpu",type="used"} 24.1
# clamp_min_kube_deployment_status_replicas_available{deployment="email-worker"} 0
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-clamp_min
```

Espere **~4 minutos** para ver um ciclo da quota e **~15 min** para o ciclo completo do disco.

---

## 🔍 Queries passo a passo

### 🟢 1 e 2. Quota restante (subtração que fica negativa)

```promql
clamp_min_kube_resourcequota
clamp_min_kube_resourcequota{type="hard"} - ignoring(type) clamp_min_kube_resourcequota{type="used"}
clamp_min(clamp_min_kube_resourcequota{type="hard"} - ignoring(type) clamp_min_kube_resourcequota{type="used"}, 0)
```

**O que faz:** `ignoring(type)` casa as duas séries mesmo tendo `type` diferente. A diferença é "quantos cores ainda posso pedir". Como a quota foi **reduzida** abaixo do uso atual, a conta fica negativa.
**Resultado esperado:**

| used | `hard - used` (cru) | `clamp_min(…, 0)` |
|---|---|---|
| 12 | **8** | **8** |
| 19 | **1** | **1** |
| 20 | **0** | **0** |
| 26 | **-6** ⚠️ | **0** |

No gráfico, a linha crua é uma onda de **+8 a -6**; a do `clamp_min` é a mesma onda com o **fundo achatado em 0**.

> 💡 Qual está "certo"? Depende da pergunta. "Quanto ainda posso pedir?" → **0** (`clamp_min`). "Quão acima da quota estou?" → o **cru** (ou `used - hard > 0`), que é o que o alerta deve olhar.

---

### 🟡 3 e 4. Divisão por zero

```promql
sum(clamp_min_rabbitmq_queue_messages) / sum(clamp_min_kube_deployment_status_replicas_available)                 # quebra
sum(clamp_min_rabbitmq_queue_messages) / clamp_min(sum(clamp_min_kube_deployment_status_replicas_available), 1)   # finito
```

**Resultado esperado:**

| réplicas | `msgs / réplicas` | `msgs / clamp_min(réplicas, 1)` |
|---|---|---|
| 3 | **~200** | **~200** |
| 0 | **+Inf** ⚠️ (o gráfico "some" ou dispara) | **~600** |

Durante os 30 s sem réplicas, a divisão crua vira `+Inf`, o que quebra o eixo do gráfico, médias (`avg` com `+Inf` = `+Inf`) e alertas com `>`. Com `clamp_min(réplicas, 1)`, o número continua **finito e interpretável**: "a fila inteira está esperando".

> ⚠️ `clamp_min` vai **no divisor**, depois do `sum`. `clamp_min` no numerador não resolve nada.

---

### 🔴 5 e 6. Previsão de disco que fica negativa

```promql
clamp_min_node_filesystem_avail_bytes
predict_linear(clamp_min_node_filesystem_avail_bytes[2m], 600)
clamp_min(predict_linear(clamp_min_node_filesystem_avail_bytes[2m], 600), 0)
```

**O que faz:** [`predict_linear()`](../predict_linear/) extrapola a reta dos últimos 2 min para daqui a 10 min (600 s). Com o disco perdendo 2.4 GiB/min, a previsão desconta **~24 GiB**.
**Resultado esperado:**

| livre agora | previsto +10 min (cru) | `clamp_min(…, 0)` |
|---|---|---|
| 40 GiB | **~16 GiB** | **~16 GiB** |
| 24 GiB | **~0** | **0** |
| 10 GiB | **~-14 GiB** ⚠️ | **0** |
| 4 GiB | **~-20 GiB** ⚠️ | **0** |

"-20 GiB livres" é fisicamente impossível; o `clamp_min(…, 0)` transforma isso em "**o disco vai encher**". No dashboard, as duas previsões têm um `and (clamp_min_node_filesystem_avail_bytes < clamp_min_node_filesystem_avail_bytes offset 2m)`: elas ficam ocultas nos 2 min seguintes a cada limpeza. Sem isso, a janela de 2 min pegaria o salto de +36 GiB e a previsão dispararia para **~290 GiB** (num disco de 40!), achatando o resto do gráfico. É o comportamento normal do `predict_linear` quando a janela contém uma mudança brusca.

---

### ⚠️ 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `clamp_min(vector(-6), 0)` | **0** | abaixo do piso |
| `clamp_min(vector(8), 0)` | **8** | acima do piso: intacto |
| `clamp_min(vector(0), 1)` | **1** | o truque do divisor |
| `clamp_min(vector(5), -Inf)` | **5** | doc: `-Inf` → inalterado |
| `clamp_min(vector(5), +Inf)` | **+Inf** | doc: `+Inf` → tudo vira +Inf |
| `clamp_min(vector(5), NaN)` | **NaN** | doc: `min` NaN → NaN |
| `clamp_min(vector(NaN), 0)` | **NaN** | NaN na entrada continua NaN |

---

## 🏭 Casos reais

### 1. Quota restante por namespace (kube-state-metrics)

```yaml
groups:
  - name: quotas
    rules:
      - record: namespace:resourcequota_remaining:clamped
        expr: |
          clamp_min(
            kube_resourcequota{type="hard"} - ignoring(type) kube_resourcequota{type="used"},
            0
          )

      - alert: NamespaceAcimaDaQuota
        expr: kube_resourcequota{type="used"} - ignoring(type) kube_resourcequota{type="hard"} > 0
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.namespace }} usa {{ $value }} de {{ $labels.resource }} acima da quota"
```

**Decisão:** a recording rule com `clamp_min` alimenta o painel "quanto ainda cabe"; o alerta usa a subtração **invertida e crua**, porque o interessante é **o quanto** passou.

### 2. Proteção contra divisão por zero em razões

```promql
# mensagens por consumidor (0 consumidores não vira +Inf)
sum by (queue) (rabbitmq_queue_messages)
  / clamp_min(sum by (queue) (rabbitmq_queue_consumers), 1)

# latência média a partir de _sum/_count sem NaN quando não há tráfego
sum(rate(http_request_duration_seconds_sum[5m]))
  / clamp_min(sum(rate(http_request_duration_seconds_count[5m])), 1e-9)
```

**Decisão:** no primeiro caso, `1` faz sentido ("uma réplica imaginária"). No segundo, `0/0 = NaN`; com um divisor mínimo minúsculo, o resultado vira `0` em vez de `NaN`. Avalie se "0 s de latência" é aceitável ou se é melhor deixar a série sumir (`> 0` no divisor).

### 3. "Horas até o disco encher" sem números negativos

```yaml
- alert: DiscoVaiEncherEm4h
  expr: |
    clamp_min(predict_linear(node_filesystem_avail_bytes{fstype!="tmpfs"}[6h], 4 * 3600), 0) == 0
      and node_filesystem_avail_bytes{fstype!="tmpfs"} / node_filesystem_size_bytes < 0.2
  for: 30m
  labels:
    severity: warning
```

(A regra clássica do node-mixin é `predict_linear(...) < 0`; o `clamp_min` é útil principalmente no **dashboard** "espaço livre previsto", para não mostrar "-20 GiB".)

### 4. Antes de `ln` / `log10` / `sqrt`

```promql
log10(clamp_min(rate(http_requests_total[5m]), 1e-3))
```

Evita `-Inf` quando a taxa é 0 (serviço ocioso) num gráfico em escala log.

---

## ✅ Quando usar

- **Subtrações** que não fazem sentido negativas: capacidade restante, quota restante, "tempo restante".
- **Divisores** que podem ser 0: `x / clamp_min(y, 1)`.
- **Previsões** (`predict_linear`, `deriv * t`) que extrapolam para valores impossíveis.
- **Antes de funções com domínio restrito:** `ln`, `log2`, `log10`, `sqrt`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer **descartar** as séries negativas (não achatar) | filtro: `x >= 0` |
| Quer o **tamanho** do negativo | [`abs()`](../abs/) |
| Precisa de piso **e** teto | [`clamp()`](../clamp/) |
| Quer um teto | [`clamp_max()`](../clamp_max/) |
| O limite muda por série (outra métrica) | `max` com matching não existe como função; use `x > y or y` (para cada série, o maior) |

## ⚠️ Pegadinhas

1. **Esconde o "quanto passou":** `-6` e `-0.1` viram `0`. Alerte na conta crua.
2. **`clamp_min(y, 1)` como divisor muda o significado** quando `y` é fracionário legítimo (ex.: 0.5 CPU): `x / clamp_min(0.5, 1) = x / 1`. Use um piso minúsculo (`1e-9`) se `y` pode ser fracionário.
3. **`min` é escalar:** `clamp_min(x, other_metric)` é erro de tipo.
4. **`clamp_min(x, +Inf)` = tudo `+Inf`** (caso da doc, geralmente um typo).
5. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- Assinatura `clamp_min(v instant-vector, min scalar)`.
- Casos da doc: `min` NaN → NaN; `-Inf` → inalterado; `+Inf` → tudo `+Inf`.
- Divisão por zero no PromQL **não dá erro**: `x/0 = +Inf` (ou `-Inf`), `0/0 = NaN`.
- `ignoring(label)` para subtrair séries que diferem num label.

**1.** O que acontece com `sum(queue_messages) / sum(consumers)` quando `consumers = 0` e há mensagens?

- A) Erro de divisão por zero
- B) O resultado é `+Inf`
- C) O resultado é `0`
- D) A série é removida

<details><summary>Resposta</summary>

**B.** O PromQL segue IEEE 754: número positivo / 0 = `+Inf`. `0/0` seria `NaN`. Para evitar, `clamp_min(sum(consumers), 1)` no divisor.
</details>

**2.** Quanto vale `clamp_min(vector(3), +Inf)`?

- A) 3
- B) +Inf
- C) NaN
- D) vazio

<details><summary>Resposta</summary>

**B.** Pela documentação, se `min` é `+Inf`, todos os valores viram `+Inf`.
</details>

**3.** `kube_resourcequota` tem os labels `type="hard"` e `type="used"`. Qual expressão dá a quota restante **sem valores negativos**?

- A) `clamp_min(kube_resourcequota{type="hard"} - kube_resourcequota{type="used"}, 0)`
- B) `clamp_min(kube_resourcequota{type="hard"} - ignoring(type) kube_resourcequota{type="used"}, 0)`
- C) `clamp_max(kube_resourcequota{type="hard"} - ignoring(type) kube_resourcequota{type="used"}, 0)`
- D) `abs(kube_resourcequota{type="hard"} - ignoring(type) kube_resourcequota{type="used"})`

<details><summary>Resposta</summary>

**B.** Sem `ignoring(type)` (A), os conjuntos de labels não casam e o resultado é **vazio**. C) limitaria o máximo em 0 (tudo ≤ 0). D) transformaria "-6" em "6 sobrando", o que é falso.
</details>

**4.** Qual é o resultado de `clamp_min(vector(-2), -Inf)`?

- A) -Inf
- B) -2
- C) 0
- D) NaN

<details><summary>Resposta</summary>

**B.** Com `min = -Inf`, os valores ficam inalterados.
</details>

## 📝 Cola rápida

- `clamp_min(v, min)`: valores `< min` viram `min`; `min` é **escalar**.
- Casos: `min` NaN → NaN; `-Inf` → inalterado; `+Inf` → tudo `+Inf`.
- Negativos pós-subtração: `clamp_min(a - b, 0)`. Divisor seguro: `x / clamp_min(y, 1)`.
- Previsão impossível: `clamp_min(predict_linear(...), 0)`.
- `x/0 = ±Inf`, `0/0 = NaN` (sem erro).

## 🔗 Relacionadas

[`clamp()`](../clamp/) · [`clamp_max()`](../clamp_max/) · [`predict_linear()`](../predict_linear/) · [`abs()`](../abs/) · [`ln()`](../ln/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#clamp_min
