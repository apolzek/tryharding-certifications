# `absent()`: o alarme de "a métrica sumiu"

> **Em uma frase:** `absent(v)` devolve **vazio** se o instant vector tiver **qualquer** série, e **uma série com valor 1** se estiver **vazio**. É como se escreve "alerte se essa métrica **não existir**" em PromQL.

| | |
|---|---|
| **Assinatura** | `absent(v instant-vector) → instant-vector` (vazio **ou** 1 elemento com valor `1`) |
| **Tipo de métrica** | ✅ Qualquer uma (só importa se existe ou não) |
| **Unidade do resultado** | nenhuma: é sempre `1` (ou nada) |
| **Dashboard** | http://localhost:3300/d/fn-absent |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a chamada na sala de aula

O professor pergunta: "**Alguém** da turma B está aqui?"

- Se **pelo menos um** aluno responder "presente" → silêncio, segue a aula (**vetor vazio**).
- Se **ninguém** responder → o professor anota "turma B **ausente**" (**1**).

Repare: `absent()` **não** diz **quem** faltou. Se 29 de 30 alunos vieram, ele fica em silêncio. Ele só grita quando **todos** faltam.

E o que vai escrito na anotação? Os labels que o professor **tinha certeza** ao perguntar. Se ele perguntou pela "turma B" (`{turma="B"}`), a anotação diz `{turma="B"}`. Se perguntou "alguém com nome começando com A?" (`=~"A.*"`), ele não tem um valor exato para anotar.

```
absent(nonexistent{job="myjob"})                       # => {job="myjob"} 1
absent(nonexistent{job="myjob",instance=~".*"})        # => {job="myjob"} 1   (regex não vira label)
absent(sum(nonexistent{job="myjob"}))                  # => {} 1              (agregação perde os labels)
```

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita um **heartbeat de jobs batch** (como o que se faz com `up` ou com métricas de "last success" de um cron):

| Métrica | Tipo | Comportamento |
|---|---|---|
| `absent_batch_heartbeat{batch="reports"}` | gauge (1) | **sempre** presente |
| `absent_batch_heartbeat{batch="billing"}` | gauge (1) | presente **2 min**, **some 1 min** (ciclo de 3 min): o job "morreu" |
| `absent_nonexistent_metric` | — | **nunca** exposta (para os exemplos da documentação) |

```bash
curl -s localhost:8088/metrics | grep '^absent_batch'
# absent_batch_heartbeat{batch="billing"} 1     <- some 1 min a cada 3
# absent_batch_heartbeat{batch="reports"} 1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-absent
```

Espere **~3 min** para ver um ciclo completo (um buraco do `billing`).

---

## 🔍 Queries passo a passo

### 1. O heartbeat cru

```promql
absent_batch_heartbeat{batch="billing"}
absent_batch_heartbeat{batch="reports"} * 2    # × 2 só para as linhas não se sobreporem no gráfico
```

**Resultado esperado:** `reports` é uma linha contínua (em 2, por causa do `* 2`); `billing` é uma linha em 1 que **some por 1 minuto** a cada 3.

---

### 2. `absent()` acende exatamente nos buracos

```promql
absent_batch_heartbeat{batch="billing"}
absent(absent_batch_heartbeat{batch="billing"})
```

**O que faz:** enquanto `billing` existe, `absent` devolve vazio. No scrape em que ele some, o Prometheus grava um **staleness marker**, o seletor passa a devolver vazio e `absent` devolve `{batch="billing"} 1`.
**Resultado esperado:** barras **verdes** (presente) e **vermelhas** (absent → 1) se **revezando**: 2 min verde, 1 min vermelho. **Nunca** aparecem juntas.
Labels do resultado: `{batch="billing"}` (só o matcher de igualdade; **sem** `job`, `instance`, `env`, porque não estavam no seletor).

---

### 3. Pegadinha: `absent(métrica)` sem filtro não dispara

```promql
absent(absent_batch_heartbeat)
```

**Resultado esperado:** **sempre vazio** ("No data" no painel). Enquanto **qualquer** série existir (`reports` está sempre lá), `absent` não tem o que reportar, mesmo com `billing` fora do ar.
**Moral:** sempre filtre o `absent` até o nível do que você quer vigiar.

---

### 4. Dedução de labels (exemplos da documentação)

```promql
absent(absent_nonexistent_metric{job="myjob"})
absent(absent_nonexistent_metric{job="myjob",instance=~".*"})
absent(sum(absent_nonexistent_metric{job="myjob"}))
```

**Resultado esperado** (um painel de tabela para cada query):

| query | resultado |
|---|---|
| `{job="myjob"}` | `{job="myjob"} 1` |
| `{job="myjob", instance=~".*"}` | `{job="myjob"} 1` (regex **não** vira label) |
| `sum(...)` | `{} 1` (agregação **perde** os labels) |

---

### 5. Regra de alerta típica

```promql
absent(absent_batch_heartbeat{batch="billing"})
```

Num stat: **1** durante o buraco e o texto "vazio = billing presente, sem alerta" no resto do tempo. Em alertas, "No data" = "sem alerta".

---

### 6. 🟡 O que dá errado: `count(x) == 0`

```promql
count(absent_batch_heartbeat{batch="billing"}) == 0
```

**Resultado esperado:** **sempre vazio**. Fora do buraco, `count` = 1 e `1 == 0` é falso. **Dentro** do buraco, `count` de um vetor vazio é **vazio** (não 0!), e vazio `== 0` continua vazio. Essa regra nunca dispara. Use `absent()`.

---

### 7. 🔴 Caso real no dashboard: `absent(up{job=...})`

```promql
absent(up{job="lab"})              # o job real deste lab
absent(up{job="billing-batch"})    # job que ninguém configurou no Prometheus
```

**Resultado esperado:**
- `absent(up{job="lab"})`: **vazio** (o gerador está sendo raspado; `up{job="lab"}` existe).
- `absent(up{job="billing-batch"})`: `{job="billing-batch"} 1`. É exatamente o que aconteceria se o job saísse do service discovery: a regra `TargetMissing` dispara, e o label `job` já vem pronto para a anotação.

---

## 🏭 Casos reais

### 1. `TargetMissing` / job inteiro sumiu do service discovery

`up == 0` alerta quando o scrape **falha**. Mas se o target **some** do service discovery (deployment deletado, label errado no ServiceMonitor, anotação removida), **não existe** `up` para ele, e `up == 0` fica em silêncio. É para isso que existe `absent`:

```yaml
groups:
  - name: absent
    rules:
      - alert: APIJobMissing
        expr: absent(up{job="api"})
        for: 5m
        labels: {severity: critical}
        annotations:
          summary: "Nenhum target do job api está sendo raspado"
          description: "Service discovery não encontra targets do job {{ $labels.job }} há 5 min."
```

O label `job="api"` vem da dedução automática de `absent`, então dá para usar `{{ $labels.job }}` na anotação. O kube-prometheus-stack tem regras exatamente assim (`KubeAPIDown`, `KubeletDown`, `PrometheusOperatorDown`...: `absent(up{job="apiserver"} == 1)`).

### 2. `absent(up{job="x"} == 1)`: "nenhum target **saudável**"

```yaml
- alert: KubeAPIDown
  expr: absent(up{job="apiserver"} == 1)
  for: 15m
```

`up{job="apiserver"} == 1` filtra só os targets de pé. Se todos caírem **ou** sumirem, o vetor fica vazio e `absent` dispara. Uma query só cobre os dois casos.

### 3. Heartbeat de cron job / job batch

Um cron de faturamento expõe `billing_last_success_timestamp_seconds` (via Pushgateway ou textfile collector). Duas regras complementares:

```yaml
- alert: BillingJobMetricMissing        # a métrica nem existe (job nunca rodou / textfile apagado)
  expr: absent(billing_last_success_timestamp_seconds{job="billing"})
  for: 10m
- alert: BillingJobNotSucceeded          # existe, mas o último sucesso é velho
  expr: time() - billing_last_success_timestamp_seconds{job="billing"} > 26 * 3600
```

### 4. Métrica de negócio que precisa existir

`absent(checkout_orders_total{env="prod"})`: se a métrica some depois de um deploy (alguém renomeou!), todos os alertas baseados em `rate(checkout_orders_total[5m])` ficam **mudos**. `absent` é o "alerta dos alertas".

---

## ✅ Quando usar

- **Alertar que um job/target/métrica inteira sumiu** (`absent(up{job="x"})`).
- **Proteger alertas** que dependem de uma métrica existir (renome, deploy quebrado).
- **Heartbeats** de jobs batch/cron.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Tolerar falhas curtas de scrape (evitar flapping) | [`absent_over_time()`](../absent_over_time/) ou `for:` na regra |
| Saber **qual** de várias séries sumiu | [`ts_of_last_over_time()`](../ts_of_last_over_time/), ou comparar com `offset`: `x offset 10m unless x` |
| Scrape falhando (target existe mas não responde) | `up == 0` |
| Contar quantas séries existem | `count(x)` (mas repare que `count` de vetor vazio é **vazio**, não 0) |

## ⚠️ Pegadinhas

1. **Só dispara quando TODAS as séries do seletor somem** (query 3). `absent(up)` quase nunca dispara.
2. **Labels do resultado vêm só dos matchers `=`** do seletor. Regex (`=~`), `!=` e agregações não geram labels (query 4).
3. **Não serve para "qual instância caiu"**: o resultado tem no máximo 1 elemento.
4. **Staleness**: a série some **no scrape seguinte** ao sumiço (staleness marker). Se o target inteiro cai, o `up` continua existindo (valor 0), e as outras séries do target somem.
5. **Resultado vazio ≠ 0**: em painéis, "No data" é o estado "tudo bem".
6. **`count(x) == 0` não funciona** para detectar ausência: `count` de vetor vazio é vazio. Use `absent(x)`.

---

## 🎓 Na prova PCA

Cai **muito**. Saiba de cor:
- Retorno: **vazio** se existe alguma série; **`1`** com labels deduzidos se não existe.
- Labels deduzidos: só de matchers de **igualdade**; agregação → `{}`.
- `absent` (instant vector) vs `absent_over_time` (range vector, "sumiu há X tempo").
- Uso clássico: `absent(up{job="..."})` para job que sumiu do service discovery; `up == 0` para scrape falhando.

**1.** O que retorna `absent(http_requests_total{job="api", instance=~"10.*"})` quando **nenhuma** série corresponde?

- A) `{job="api", instance=~"10.*"} 1`
- B) `{job="api"} 1`
- C) `{} 1`
- D) Vetor vazio

<details><summary>Resposta</summary>

**B.** Só matchers de igualdade viram labels do resultado. O regex é descartado.
</details>

**2.** Existem 3 séries `up{job="node"}`: duas com valor 1 e uma com valor 0. O que retorna `absent(up{job="node"})`?

- A) `{job="node"} 1`
- B) Vetor vazio
- C) 1 série para a instância com valor 0
- D) `0`

<details><summary>Resposta</summary>

**B.** As séries **existem** (o valor 0 não importa). Para "nenhum target saudável", use `absent(up{job="node"} == 1)`.
</details>

**3.** Um deployment foi deletado e seus pods sumiram do service discovery. Qual alerta dispara?

- A) `up{job="myapp"} == 0`
- B) `absent(up{job="myapp"})`
- C) `rate(up{job="myapp"}[5m]) == 0`
- D) `count(up{job="myapp"}) == 0`

<details><summary>Resposta</summary>

**B.** Sem targets, não existe série `up{job="myapp"}`: A e C ficam vazias (sem alerta) e D também (count de vetor vazio é vazio, não 0).
</details>

**4.** O que retorna `absent(sum(nonexistent{job="myjob"}))`?

- A) `{job="myjob"} 1`
- B) `{} 1`
- C) Vetor vazio
- D) Erro de sintaxe

<details><summary>Resposta</summary>

**B.** Exemplo literal da documentação: agregações não preservam labels para a dedução.
</details>

**5.** Qual a diferença entre `absent(x)` e `absent_over_time(x[10m])`?

- A) Nenhuma.
- B) `absent` olha só o instante atual; `absent_over_time` dispara só se não houver **nenhuma** amostra nos últimos 10 min.
- C) `absent_over_time` retorna a contagem de amostras ausentes.
- D) `absent` aceita range vector.

<details><summary>Resposta</summary>

**B.** `absent_over_time` é a versão com "tolerância" (range vector). Ambas devolvem vazio ou `1`.
</details>

---

## 📝 Cola rápida

- `absent(v)` → vazio se existe **qualquer** série; `{labels =} 1` se não existe nenhuma.
- Labels deduzidos só de matchers `=`; regex e agregação → não viram labels (`sum(...)` → `{}`).
- `absent(up{job="x"})` = job sumiu do SD; `absent(up{job="x"} == 1)` = nenhum target saudável; `up == 0` = scrape falhando.
- Filtre até o nível que quer vigiar: `absent(métrica)` sem filtro quase nunca dispara.
- `count(x) == 0` **não** detecta ausência. Quer tolerância? `absent_over_time(x[w])` ou `for:`.

## 🔗 Relacionadas

[`absent_over_time()`](../absent_over_time/) · [`present_over_time()`](../present_over_time/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/) · [`count_over_time()`](../count_over_time/) · [`vector()`](../vector/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#absent
