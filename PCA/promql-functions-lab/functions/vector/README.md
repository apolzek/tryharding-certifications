# `vector()`: transformar um número em vetor (o famoso `or vector(0)`)

> **Em uma frase:** `vector(s)` pega um **escalar** e devolve um **instant vector com um único elemento, sem labels** (`{} s`). O uso mais comum é `... or vector(0)`, que troca "No data" por **zero** em painéis e alertas.

| | |
|---|---|
| **Assinatura** | `vector(s scalar) → instant-vector` |
| **Tipo de métrica** | não se aplica: a entrada é um **escalar** (`0`, `time()`, `scalar(...)`) |
| **Unidade do resultado** | a do escalar |
| **Dashboard** | http://localhost:3300/d/fn-vector |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: escrever "0" em vez de deixar a célula vazia

Numa planilha de vendas, o dia em que ninguém comprou fica **em branco**. O gráfico da planilha "pula" esse dia, e quem lê fica na dúvida: vendeu zero ou **esqueceram de preencher**?

No Prometheus, um counter de erros que **nunca teve erro** muitas vezes **não existe** (a série só nasce no primeiro erro, ou some quando o pod reinicia). Então `sum(rate(errors_total[5m]))` não devolve **0**: devolve **nada**. O painel mostra "No data" e o alerta que depende dele fica em silêncio.

`vector(0)` é o "0" escrito na célula: um vetor de verdade, com valor 0 e **sem labels**. Com o operador `or`, ele só entra em cena quando o lado esquerdo está vazio:

```
lado esquerdo tem dados:   { } 4.0      or  { } 0   →  { } 4.0   (o vector(0) é ignorado)
lado esquerdo vazio:       (nada)       or  { } 0   →  { } 0     (o vector(0) "cobre" o buraco)
```

É o **inverso** do [`scalar()`](../scalar/): `scalar()` tira o número da caixa; `vector()` coloca um número numa caixa **sem etiqueta**.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Labels | Comportamento |
|---|---|---|
| `vector_http_requests_total` (counter, imita `http_requests_total`) | `route="/checkout"`, `code="200"` | cresce ≈ **15/s**, sempre existe |
| idem | `route="/search"`, `code="200"` | ≈ **25/s**, sempre existe |
| idem | `route="/checkout"`, `code="500"` | ≈ **3/s**, **só existe 2 min a cada 5 min** |
| idem | `route="/search"`, `code="500"` | ≈ **1/s**, **só existe 2 min a cada 5 min** |

As séries `code="500"` aparecem nos minutos 0-2 de cada bloco de 5 minutos do relógio (ex.: 10:00-10:02, 10:05-10:07...) e **somem** no resto do tempo.

```bash
curl -s localhost:8088/metrics | grep '^vector_'
# vector_http_requests_total{code="200",route="/checkout"} 612345
# vector_http_requests_total{code="500",route="/checkout"} 87     ← só em parte do tempo
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-vector
```

Espere ~5 minutos para ver pelo menos um ciclo completo (erros aparecem e somem).

---

## 🔍 Queries passo a passo

### 1. A taxa de erros "crua"

```promql
sum(rate(vector_http_requests_total{code=~"5.."}[1m]))
```

**Resultado esperado:** um gráfico com "morros" em forma de **trapézio** chegando a ≈ **4 erros/s** (3 + 1), um a cada 5 minutos, separados por **buracos** de ~2 minutos onde não há **nenhum** ponto. O trapézio sobe em ~1 min (o `rate[1m]` precisa de 1 minuto de dados da série recém-nascida para "enxergar" a velocidade toda), fica plano ~1 min e desce em ~1 min depois que a série some (a janela ainda contém as últimas amostras).

---

### 2. Com `or vector(0)`

```promql
sum(rate(vector_http_requests_total{code=~"5.."}[1m])) or vector(0)
```

**Resultado esperado:** uma linha **contínua**: ≈ **4** nos blocos de erro e **0** no resto. Nada de buracos.

**Por que funciona:** `a or b` devolve tudo de `a` **mais** os elementos de `b` cujo conjunto de labels **não existe** em `a`. O `sum(...)` sem `by` produz um vetor com labels `{}`; o `vector(0)` também é `{}`. Então:
- `sum` com dados → `{}` já existe em `a` → o `vector(0)` é descartado.
- `sum` vazio → nada em `a` → entra o `{} 0`.

---

### 3a e 3b. Stat lado a lado

| painel | fora da janela de erros | durante os erros |
|---|---|---|
| 3a (sem tratamento) | **No data** | ≈ 4 erros/s |
| 3b (`or vector(0)`) | **0** | ≈ 4 erros/s |

Um painel de erro com "No data" gera a pergunta "o Prometheus caiu?". Com **0**, a mensagem é clara: **não há erros**.

---

### 4. SLO de taxa de erro + linha constante

```promql
(sum(rate(vector_http_requests_total{code=~"5.."}[1m])) or vector(0))
  / sum(rate(vector_http_requests_total[1m]))

vector(0.05)   # linha do limite de 5%
```

**Resultado esperado:**
- Taxa de erro ≈ **9%** (4 / 44) nos blocos de erro e **0%** no resto. Sem o `or vector(0)`, a razão também ficaria **vazia** fora dos blocos (nada / 40 = nada).
- `vector(0.05)` desenha uma **linha horizontal em 5%**: o limite do SLO. É assim que se desenha uma constante no Grafana a partir do PromQL.

> 💡 Sem `vector()`, a query `0.05` sozinha também funciona no Grafana (é um escalar); `vector(0.05)` é útil quando você precisa de um **vetor** para combinar com `or`, `and`, `label_replace` etc.

---

### 5. Pegadinha: `sum by (route) (...) or vector(0)`

```promql
sum by (route) (rate(vector_http_requests_total{code=~"5.."}[1m])) or vector(0)
```

**Resultado esperado:**
- **Durante os erros:** **3** séries: `route="/checkout"` ≈ 3, `route="/search"` ≈ 1 **e** uma série extra `{}` = **0** (os labels `{route=...}` ≠ `{}`, então o `or` também adiciona o `vector(0)`).
- **Fora dos erros:** **1** série `{}` = 0, **sem** nome de rota.

Não é "0 para cada rota": o `vector(0)` não sabe que rotas existem.

---

### 6. A correção para o caso "por rota"

```promql
sum by (route) (rate(vector_http_requests_total{code=~"5.."}[1m]))
  or
0 * sum by (route) (rate(vector_http_requests_total[1m]))
```

**O que faz:** usa como "molde" uma métrica que **sempre existe por rota** (o total de requisições), multiplicada por 0. Assim o fallback tem os **labels certos**.
**Resultado esperado:** **sempre 2** séries: `/checkout` (≈ 3 ou 0) e `/search` (≈ 1 ou 0).

---

## 🏭 Casos reais

### 1. Recording rule de SLO que não "some" quando não há erro

```yaml
groups:
  - name: slo-checkout
    rules:
      - record: checkout:http_requests_errors:ratio_rate5m
        expr: |
          (sum(rate(http_requests_total{job="checkout", code=~"5.."}[5m])) or vector(0))
            /
          sum(rate(http_requests_total{job="checkout"}[5m]))
```

**Decisões:** o `or vector(0)` garante que a recording rule grava **0** (e não nada) nas horas boas. Sem isso, cálculos de *error budget* com `avg_over_time` sobre essa série só enxergariam as horas **com** erro e ficariam **pessimistas** (a média de "só horas ruins"). Repare que **não** há `by (...)`: ambos os lados são `{}`, então a divisão casa. Se precisar de `by (job)`, troque o `vector(0)` pelo molde do painel 6: `or 0 * sum by (job) (rate(http_requests_total{job="checkout"}[5m]))`, porque o `{}` do `vector(0)` **não** casa com `{job="checkout"}`.

### 2. Alerta de "nenhum job de backup rodou" (contagem que pode ser vazia)

```yaml
- alert: NoSuccessfulBackupsToday
  expr: (sum(increase(backup_success_total[24h])) or vector(0)) == 0
  for: 30m
  annotations:
    summary: "Nenhum backup com sucesso nas últimas 24h"
```

**Decisão:** se o counter `backup_success_total` **nunca** foi criado (o job nunca teve sucesso desde o deploy), `sum(increase(...))` é **vazio** e `vazio == 0` também é vazio: o alerta **não dispararia** justamente no pior caso. O `or vector(0)` transforma "não existe" em 0 e o alerta dispara. (Alternativa: [`absent()`](../absent/).)

### 3. Linha de referência (SLO / capacidade) no Grafana

```promql
vector(0.999)    # objetivo de disponibilidade 99.9%
```

Desenha a meta ao lado da disponibilidade medida (painel 4 deste lab).

### 4. Criar uma série sintética com labels

```promql
label_replace(vector(1), "status", "ok", "", "")
```

Útil para testes, para "semear" uma tabela ou para montar um *heartbeat* em regras: `vector(1)` sempre existe.

---

## ✅ Quando usar

- **`or vector(0)`** em painéis e regras de **erro/contagem** que podem não ter séries.
- **Constantes** em gráficos (meta de SLO, capacidade máxima).
- Converter um **escalar calculado** (`time()`, `scalar(...)`, `max_of(...)`) em vetor, para usar com operadores de conjunto (`or`, `and`, `unless`) ou `label_replace`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Fallback **por label** (por rota, por job) | `or 0 * <métrica que sempre existe> by (...)` (painel 6) |
| Detectar que uma métrica **sumiu** | [`absent()`](../absent/) / [`absent_over_time()`](../absent_over_time/) |
| Converter vetor → escalar | [`scalar()`](../scalar/) |
| Só desenhar uma constante no Grafana | um escalar puro (`0.05`) também serve |

## ⚠️ Pegadinhas

1. **`vector(0)` não tem labels:** `sum by (x) (...) or vector(0)` adiciona **uma** série `{}`, não um 0 por `x` (painel 5).
2. **Em divisões com `by`:** o `{}` do `vector(0)` não casa com `{job="..."}` do denominador → resultado vazio. Use o "molde" com `0 * ...`.
3. **Pode mascarar problemas:** se o scrape **falhou** e a métrica sumiu, `or vector(0)` mostra "0 erros" (tudo verde!). Tenha um alerta de `up == 0` / [`absent()`](../absent/) em paralelo.
4. **Escalar vs vetor:** `vector(time())` é um vetor; `time()` é escalar. Operadores de conjunto (`or`, `and`, `unless`) **só** funcionam entre vetores: `x or 0` é **erro de parse**.
5. **Nome da métrica:** `vector(...)` não tem `__name__`, então legendas `{{__name__}}` ficam vazias.

## 🎓 Na prova PCA

O que costuma cair:

- **Tipos:** `vector(s)` recebe **escalar** e devolve **instant vector** de 1 elemento **sem labels**.
- **Operador `or`** e o padrão `or vector(0)`.
- Diferença entre "série com valor 0" e "série inexistente" (e o efeito em alertas).

**1.** Qual o resultado de `vector(5)`?

- A) O escalar 5
- B) Um instant vector com um elemento `{}` de valor 5
- C) Um range vector de 5 minutos
- D) 5 séries com valor 1

<details><summary>Resposta</summary>

**B.**
</details>

**2.** Por que `sum(rate(errors_total[5m])) or vector(0)` é comum em dashboards?

- A) Para arredondar o resultado
- B) Para devolver 0 quando não existem séries de erro, em vez de "No data"
- C) Para converter o counter em gauge
- D) Para somar o valor 0 ao resultado

<details><summary>Resposta</summary>

**B.** Com dados, o `or` ignora o `vector(0)` (mesmo conjunto de labels `{}`).
</details>

**3.** Qual é **inválida**?

- A) `sum(x) or vector(0)`
- B) `sum(x) or 0`
- C) `vector(1) + 1`
- D) `scalar(sum(x))`

<details><summary>Resposta</summary>

**B.** Operadores de conjunto (`or`, `and`, `unless`) exigem vetores dos dois lados.
</details>

**4.** `sum by (job) (rate(errors_total[5m])) or vector(0)`, quando não há erros em nenhum job, retorna:

- A) Uma série com valor 0 para cada job
- B) Uma série `{}` com valor 0
- C) Nada
- D) Erro de execução

<details><summary>Resposta</summary>

**B.** `vector(0)` não conhece os jobs; ele só tem labels vazios.
</details>

## 📝 Cola rápida

- `vector(s)` → `{} s` (1 elemento, **sem labels**). Inverso de `scalar()`.
- `... or vector(0)`: "No data" vira 0. Só funciona bem **sem** `by`.
- Fallback por label: `or 0 * sum by (x) (<métrica que sempre existe>)`.
- `or`/`and`/`unless` só entre **vetores** (`x or 0` é erro).
- Cuidado: 0 pode esconder um scrape quebrado → tenha `absent()`/`up`.

## 🔗 Relacionadas

[`scalar()`](../scalar/) · [`absent()`](../absent/) · [`absent_over_time()`](../absent_over_time/) · [`label_replace()`](../label_replace/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#vector
