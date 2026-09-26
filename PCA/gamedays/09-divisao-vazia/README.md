# 09 · A divisão vazia

> **Em uma frase:** o search está com 20% de erro e o alerta `SearchTaxaDeErroAlta` avalia para **nada**. `erros / requisições` não encontrou nenhum par de séries com os mesmos labels para dividir.

| | |
|---|---|
| **Dificuldade** | ⭐ |
| **Tempo-alvo** | 15 min |
| **Tópicos PCA** | vector matching (one-to-one), `on()`, `ignoring()`, agregação antes de operar, "resultado vazio não dispara" |
| **Arquivos que você vai editar** | `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 🔔 Slack #search-team · 11:20                                            │
├──────────────────────────────────────────────────────────────────────────┤
│ gente, o search tá devolvendo 500 pra uma a cada cinco buscas desde as   │
│ 10h (vi nos logs). o SearchTaxaDeErroAlta que a gente criou semana       │
│ passada não tocou. testei a query no Prometheus e... ela volta vazia?    │
│ mas as duas métricas existem, eu olhei!                                  │
│                                                                          │
│ Pedido: o alerta SearchTaxaDeErroAlta deve disparar enquanto a taxa de   │
│ erro (http_errors_total / http_requests_total) passar de 5%.             │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 09
# edite work/prometheus/rules.yml  ->  ./reload.sh  ->  ./check.sh 09
```

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> as duas métricas existem mesmo?</summary>

```promql
rate(http_errors_total{job="search"}[1m])
rate(http_requests_total{job="search"}[1m])
```

**Resultado esperado:** as duas têm dados (~2 e ~10 por segundo). Mas a divisão:

```promql
rate(http_errors_total{job="search"}[1m]) / rate(http_requests_total{job="search"}[1m])
```

volta **vazia**.
</details>

<details>
<summary><b>Passo 2:</b> compare os labels dos dois lados.</summary>

```bash
curl -s localhost:9182/metrics | grep -E '^http_(errors|requests)_total'
# http_requests_total{method="GET"} 1234
# http_errors_total{code="500"} 246
```

Depois do scrape (e sem `__name__`, que operações aritméticas descartam):

| lado | labels |
|---|---|
| esquerdo | `{code="500", instance="app:9182", job="search"}` |
| direito | `{instance="app:9182", job="search", method="GET"}` |

Operadores binários entre vetores fazem **one-to-one matching** por padrão: só combinam séries com **exatamente** o mesmo conjunto de labels. `code` só existe à esquerda e `method` só à direita, então nenhum par casa. Resultado: vazio. Vazio `> 0.05` é vazio. Alerta nunca dispara.
</details>

<details>
<summary><b>Passo 3:</b> três jeitos de consertar o matching.</summary>

```promql
# 1) dizer em QUAIS labels casar
rate(http_errors_total{job="search"}[1m]) / on(job, instance) rate(http_requests_total{job="search"}[1m])

# 2) dizer quais IGNORAR
rate(http_errors_total{job="search"}[1m]) / ignoring(code, method) rate(http_requests_total{job="search"}[1m])

# 3) agregar os dois lados para o mesmo conjunto de labels (o mais robusto)
sum by (job, instance) (rate(http_errors_total{job="search"}[1m]))
  /
sum by (job, instance) (rate(http_requests_total{job="search"}[1m]))
```

**Resultado esperado:** os três dão ≈ **0.2**.

Por que o 3 é o mais robusto? Amanhã o app começa a expor `http_requests_total{method="POST"}` também. Com `on(job, instance)`, o lado direito passa a ter **duas** séries para o mesmo `{job, instance}` e a query quebra com `many-to-many matching not allowed` (e, de novo, o alerta avalia para erro/vazio). Com `sum by`, os dois lados sempre têm uma série por `{job, instance}`.
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

A expressão dividia duas métricas com **labels diferentes** (`code` de um lado, `method` do outro) sem `on()`/`ignoring()` nem agregação. O one-to-one matching não encontrou nenhum par, a divisão voltou vazia e o alerta ficou *inactive* para sempre, sem nenhum erro visível (a regra tem `health: ok`).
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/prometheus/rules.yml</code></summary>

```yaml
      - alert: SearchTaxaDeErroAlta
        expr: |
          sum by (job, instance) (rate(http_errors_total{job="search"}[1m]))
            /
          sum by (job, instance) (rate(http_requests_total{job="search"}[1m]))
            > 0.05
        for: 10s
```

```bash
./reload.sh && ./check.sh 09
```
</details>

## 🛡️ Como evitar

- **Em razões, agregue os dois lados com o mesmo `by (...)`** antes de dividir. É o padrão das recording rules (`job:http_errors:ratio_rate5m`).
- **Melhor ainda, instrumente com uma métrica só:** `http_requests_total{code="200|500|..."}` e calcule `sum(rate(x{code=~"5.."}[5m])) / sum(rate(x[5m]))`. Mesmo nome, mesmos labels, sem armadilha de matching.
- **Teste o alerta com dados que deveriam dispará-lo** (`promtool test rules`). Uma regra que "nunca disparou" pode ser uma regra que **não consegue** disparar:

```yaml
tests:
  - interval: 15s
    input_series:
      - series: 'http_errors_total{job="search", instance="a", code="500"}'
        values: '0+30x20'
      - series: 'http_requests_total{job="search", instance="a", method="GET"}'
        values: '0+150x20'
    alert_rule_test:
      - eval_time: 4m
        alertname: SearchTaxaDeErroAlta
        exp_alerts:
          - exp_labels: {severity: page, job: search, instance: a}
            exp_annotations:
              summary: "search com 20% de erro em a"
```

Com a regra quebrada: `got:[]`. Com a corrigida: `SUCCESS`.

- **pint** avisa quando os dois lados de uma operação não têm labels em comum (check `promql/vector_matching`, que consulta o Prometheus de verdade).

## 📝 Postmortem (exemplo)

> **Resumo:** das 13:00 às 14:25 UTC, 20% das buscas retornaram HTTP 500. O alerta `SearchTaxaDeErroAlta`, criado 5 dias antes, não disparou. Detecção pelo próprio time, olhando logs.
>
> **Causa raiz:** a expressão dividia `http_errors_total{code}` por `http_requests_total{method}` sem `on()`/agregação; o one-to-one matching não casava nenhum par e a expressão sempre voltava vazia.
>
> **Por que passou:** a regra nunca foi testada com dados que deveriam fazê-la disparar; `promtool check rules` só valida sintaxe.
>
> **Ações:**
> 1. (corrigir) `sum by (job, instance)` nos dois lados. ✅
> 2. (prevenir) `promtool test rules` obrigatório com um caso "deve disparar" para todo alerta novo. **Dono:** time search.
> 3. (prevenir) pint `promql/vector_matching` no CI. **Dono:** plataforma.
> 4. (corrigir) padronizar a instrumentação: um único counter com label `code`. **Dono:** time search.

## 🎓 Na prova PCA

<details>
<summary>Q1. <code>a{x="1",y="2"} / b{x="1",z="3"}</code>: what is the result?</summary>

**Vazio.** One-to-one matching exige o mesmo conjunto de labels; `y` e `z` diferem. Use `on(x)` ou `ignoring(y, z)`.
</details>

<details>
<summary>Q2. When do you need <code>group_left</code> / <code>group_right</code>?</summary>

Quando um lado tem **várias** séries para cada série do outro lado (many-to-one / one-to-many), ex.: juntar `*_info` com métricas por instância. Sem isso, a query falha com erro de matching.
</details>
