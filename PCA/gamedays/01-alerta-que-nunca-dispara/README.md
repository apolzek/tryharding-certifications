# 01 · O alerta que nunca dispara

> **Em uma frase:** o checkout passou 20 minutos com erros intermitentes, o alerta `CheckoutErrosAltos` existe, está carregado, está "verde"... e nunca disparou.

| | |
|---|---|
| **Dificuldade** | ⭐ |
| **Tempo-alvo** | 15 min |
| **Tópicos PCA** | `rate()` e tamanho da janela, `scrape_interval`, `for:`, estados de alerta (inactive/pending/firing) |
| **Arquivos que você vai editar** | `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ TICKET #88213 · prioridade ALTA · aberto por: Suporte N2                 │
├──────────────────────────────────────────────────────────────────────────┤
│ Assunto: clientes com erro "Não foi possível finalizar a compra"         │
│                                                                          │
│ Desde ~14h vários clientes relatam erro ao finalizar compra. Ao tentar   │
│ de novo, às vezes funciona. O time de produto viu no painel que a taxa   │
│ de erro do checkout "fica pulando". O on-call NÃO recebeu page nenhum.   │
│                                                                          │
│ Pedido: entender por que o alerta CheckoutErrosAltos não disparou e      │
│ corrigir para que ele dispare enquanto o problema estiver acontecendo.   │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 01
# edite work/prometheus/rules.yml   ->  ./reload.sh  ->  ./check.sh 01
```

## 🩺 Sintomas

- Na UI (http://localhost:9180/alerts) o alerta `CheckoutErrosAltos` aparece como **inactive**. Ele nunca nem passa por *pending* de forma estável.
- O app realmente está com problema: `curl -s localhost:9182/metrics | grep http_requests_total` mostra o counter de `code="500"` crescendo.

---

## 🔍 Investigação guiada

Tente sozinho primeiro. Abra uma dica por vez.

<details>
<summary><b>Passo 1:</b> o problema é real? Confirme a taxa de erro "na mão".</summary>

Rode no Graph (http://localhost:9180) com uma janela confortável:

```promql
sum(rate(http_requests_total{job="checkout", code="500"}[1m]))
  /
sum(rate(http_requests_total{job="checkout"}[1m]))
```

**Resultado esperado:** ≈ **0.2** (20%) o tempo todo. Muito acima do limite de 10% do alerta. Então o problema não é "não tem erro": é o alerta.
</details>

<details>
<summary><b>Passo 2:</b> rode a expressão EXATA do alerta.</summary>

Copie a `expr` de `work/prometheus/rules.yml` (ou de Status → Rules) e rode:

```promql
sum(rate(http_requests_total{job="checkout", code="500"}[5s]))
  /
sum(rate(http_requests_total{job="checkout"}[5s]))
```

**Resultado esperado:** **vazio** (`Empty query result`). Via API:

```bash
curl -s localhost:9180/api/v1/query \
  --data-urlencode 'query=sum(rate(http_requests_total{job="checkout"}[5s]))' | jq '.data.result'
# []
```

Uma comparação sobre vazio é vazio, e vazio **nunca** dispara.
</details>

<details>
<summary><b>Passo 3:</b> quantas amostras cabem numa janela de 5s?</summary>

```promql
count_over_time(http_requests_total{job="checkout", code="500"}[5s])
```

**Resultado esperado:** **1**. O `scrape_interval` é 5s, então numa janela de 5s cabe **uma** amostra (a janela é aberta à esquerda: `(t-5s, t]`). O `rate()` precisa de **pelo menos duas** amostras para calcular uma inclinação. Com uma só, a série simplesmente some do resultado.

```bash
curl -s localhost:9180/api/v1/status/config | jq -r '.data.yaml' | grep -A2 global
```
</details>

<details>
<summary><b>Passo 4:</b> e se a janela estivesse certa, o <code>for: 2m</code> deixaria disparar?</summary>

Olhe o padrão do erro com uma janela curta mas válida:

```promql
sum(rate(http_requests_total{job="checkout", code="500"}[15s]))
  /
sum(rate(http_requests_total{job="checkout"}[15s]))
```

**Resultado esperado:** uma onda quadrada: ~40% por 15s, ~0% por 15s, e assim por diante (ciclo de 30s). Com `for: 2m`, o alerta precisa ficar **2 minutos seguidos** acima do limite; a cada 15s a condição fica falsa, o *pending* é descartado e o cronômetro do `for` **zera**. Um alerta com `for` maior que o "período bom" de um problema intermitente **nunca** dispara.
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

Dois erros que se somam:

1. **Janela do `rate()` menor que 2 scrapes.** `rate(x[5s])` com `scrape_interval: 5s` tem 1 amostra por janela → resultado vazio → a regra avalia para "nada" a cada ciclo.
2. **`for:` maior que o intervalo em que a condição fica verdadeira.** O erro é intermitente (15s ruim / 15s bom). Mesmo com a janela corrigida para algo curto, o `for: 2m` reiniciaria a cada 15s.

A intenção era boa ("janela curta para reagir rápido", "for longo para não acordar ninguém à toa"), mas as duas escolhas juntas criaram um alerta que é **estruturalmente incapaz** de disparar para este tipo de falha.
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/prometheus/rules.yml</code></summary>

A janela maior faz a **média** sobre o ciclo inteiro da oscilação: 20% constante em vez de uma onda que vai a zero.

```yaml
      - alert: CheckoutErrosAltos
        expr: |
          sum(rate(http_requests_total{job="checkout", code="500"}[1m]))
            /
          sum(rate(http_requests_total{job="checkout"}[1m]))
            > 0.1
        for: 30s
```

```bash
./reload.sh && ./check.sh 01
```

Ou `./solve.sh 01`.
</details>

## 🛡️ Como evitar

- **Janela ≥ 4× `scrape_interval`.** Com scrape de 15s (o padrão), `[1m]` é o mínimo; `[5m]` é o mais comum em alertas. Uma falha de scrape já não quebra o `rate`.
- **Use `$__rate_interval` no Grafana**, que já calcula `max(4 × scrape, intervalo do painel)`.
- **Para sinais que oscilam, alise a entrada, não aumente o `for`.** Janela maior (ou `avg_over_time` sobre uma recording rule) deixa o sinal estável; o `for` só deve filtrar picos de segundos. A partir do Prometheus 2.42+ existe `keep_firing_for` para o problema inverso (alerta que "pisca" ao resolver).
- **Teste o alerta com a falha real** usando `promtool test rules` (padrão intermitente nos `input_series`):

```yaml
# tests/checkout_test.yml
rule_files: [../rules.yml]
evaluation_interval: 5s
tests:
  - interval: 5s
    input_series:
      # 15s com erro (sobe 20 a cada 5s), 15s sem erro (parado), repetindo
      - series: 'http_requests_total{job="checkout", code="500"}'
        values: '0+20x2 60+0x2 60+20x2 120+0x2 120+20x2 180+0x2 180+20x2 240+0x2 240+20x2 300+0x2 300+20x2 360+0x2 360+20x2 420+0x2 420+20x2 480+0x2 480+20x2 540+0x2'
      - series: 'http_requests_total{job="checkout", code="200"}'
        values: '0+30x107'
    promql_expr_test:
      # o alerta PRECISA estar firing com esse padrão de falha
      - expr: count(ALERTS{alertname="CheckoutErrosAltos", alertstate="firing"})
        eval_time: 2m
        exp_samples:
          - value: 1
```

Com a regra quebrada, esse teste falha (`got: nil`); com a corrigida, `SUCCESS`.

- **Meta-alerta de regra quebrada:** `prometheus_rule_evaluation_failures_total` pega erro de avaliação, mas **não** pega "regra que sempre volta vazio". Para isso, revise alertas que nunca dispararam em 90 dias (`ALERTS` histórico / `count_over_time(ALERTS{alertname="X"}[90d])`).

## 📝 Postmortem (exemplo)

> **Resumo:** entre 14:02 e 14:25 UTC, ~20% das finalizações de compra falharam de forma intermitente. O alerta `CheckoutErrosAltos` não disparou; o incidente foi detectado por tickets de clientes, 38 min após o início.
>
> **Causa raiz:** a regra usava `rate(...[5s])` com `scrape_interval` de 5s (1 amostra por janela → resultado vazio) e `for: 2m`, maior que os períodos de 15s em que a falha ficava ativa.
>
> **Por que a mudança passou:** a regra foi revisada visualmente, sem teste; não havia `promtool test rules` no CI.
>
> **Ações:**
> 1. (corrigir) janela `[1m]`, `for: 30s`. **Dono:** SRE. ✅
> 2. (prevenir) lint no CI: janela de `rate`/`increase` < 4× `scrape_interval` falha o PR. **Dono:** plataforma.
> 3. (prevenir) teste `promtool` com padrão intermitente para todo alerta `severity: page`. **Dono:** times donos dos alertas.
> 4. (detectar) relatório mensal de alertas `page` que nunca dispararam. **Dono:** SRE.

## 🎓 Na prova PCA

<details>
<summary>Q1. With <code>scrape_interval: 15s</code>, which range is the smallest safe choice for <code>rate()</code> in an alert? (a) [15s] (b) [30s] (c) [1m] (d) [5s]</summary>

**(c) [1m].** `rate` precisa de ≥ 2 amostras; a regra prática é janela ≥ 4× o scrape interval para tolerar falhas de scrape. `[15s]` e `[5s]` dão ≤ 1 amostra; `[30s]` dá 2 no melhor caso e zero margem.
</details>

<details>
<summary>Q2. An alert has <code>for: 5m</code>. The condition is true for 4 minutes, false for one evaluation, then true for 4 more minutes. Does it fire?</summary>

**Não.** Uma avaliação falsa devolve o alerta para *inactive* e o contador do `for` recomeça do zero.
</details>
