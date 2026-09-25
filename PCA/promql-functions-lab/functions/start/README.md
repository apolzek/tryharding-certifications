# `start()`: o início da janela do gráfico (experimental)

> **Em uma frase:** `start()` devolve o **timestamp de início** (segundos Unix, UTC) da consulta *range* em andamento, ou seja, a **borda esquerda do gráfico**. É o mesmo número em todos os pontos. Numa consulta **instantânea**, `start()` = instante avaliado (= `time()`).

| | |
|---|---|
| **Assinatura** | `start() → scalar` |
| **Status** | 🧪 **experimental**: exige `--enable-feature=promql-experimental-functions` |
| **Tipo de métrica** | nenhuma. Brilha com o modificador `@`: `x @ start()` |
| **Unidade do resultado** | segundos Unix (UTC) |
| **Dashboard** | http://localhost:3300/d/fn-start |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o filme e o primeiro quadro

Uma consulta *range* (o que o Grafana faz para desenhar um gráfico) é um **filme**: o Prometheus avalia a expressão várias vezes, uma por **quadro** (step), da borda esquerda até a direita.

- [`time()`](../time/) = o horário **do quadro atual** (muda a cada ponto).
- `start()` = o horário **do primeiro quadro** (igual para todos os pontos).
- [`end()`](../end/) = o horário do último quadro.
- [`range()`](../range/) = duração do filme. [`step()`](../step/) = intervalo entre quadros.

Uma **foto** (consulta instantânea: stat, tabela, regra de alerta) só tem **um** quadro: `start()` = `end()` = `time()`.

```
 start()                                                   end()
   |<------------------------ range() ------------------------>|
   •----•----•----•----•----•----•----•----•----•----•----•----•
        |<-->|  step()             ↑ time() deste ponto
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `start_orders_total` | counter | ~**3 pedidos/s** (calculado pelo relógio de parede: não zera com deploys) |
| `start_queue_depth` | gauge | tamanho de fila, onda lenta **100 ± 40** (período 10 min) |

```bash
curl -s localhost:8088/metrics | grep '^start_'
# start_orders_total 1.12345e+06
# start_queue_depth 117
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-start   (mude o zoom e veja os painéis se ajustarem)
```

---

## 🔍 Queries passo a passo

### 1. Segundos desde a borda esquerda

```promql
time() - start()
```

**O que faz:** em cada ponto, subtrai o início do gráfico do instante daquele ponto.
**Resultado esperado:** rampa de **0** (borda esquerda) até **~900** (borda direita) na janela de 15 min. Mude o seletor de tempo do Grafana para "Last 1 hour": a rampa passa a ir até **3600**.

---

### 2. Pedidos desde o início do gráfico

```promql
start_orders_total - start_orders_total @ start()
```

**O que faz:** `@ start()` "congela" o seletor no instante do primeiro quadro; o resultado é uma constante. Subtraindo do valor atual de cada ponto, você tem **quanto o counter andou desde a borda esquerda**.
**Resultado esperado:** rampa de **0** até ≈ **2 700** (3/s × 900 s) em 15 min. Sempre que você muda o zoom, o gráfico "zera" na nova borda esquerda.

> ⚠️ Com counters, isso só funciona se **não houver reset** na janela (senão fica negativo). Para counters que reiniciam, prefira `increase(x[range()] @ end())` (veja [`range()`](../range/)).

---

### 3. Linha de referência: "como estava no começo"

```promql
start_queue_depth               # a fila
start_queue_depth @ start()     # valor no primeiro quadro, repetido no gráfico todo
```

**Resultado esperado:** uma onda (a fila, entre ~60 e ~140) e uma **linha reta** no valor que a fila tinha na borda esquerda. Fica fácil ver "piorou ou melhorou desde o começo da janela".

---

### 4. Numa consulta instantânea (stat)

```promql
time() - start()                                      # 0
start_orders_total - start_orders_total @ start()     # 0
```

**Resultado esperado:** os dois stats mostram **0**. Numa consulta instantânea não existe janela: `start()` é o próprio instante avaliado.

> Esse é o "❌ o que dá errado" desta função: colocar `x - x @ start()` numa **regra de alerta** ou num **stat** esperando "variação no período" → sempre 0.

---

### 5. (Comparação) `@ start()` vs `offset`

```promql
start_queue_depth @ start()     # linha RETA (instante fixo)
start_queue_depth offset 5m     # a mesma onda deslocada 5 min (instante relativo a cada ponto)
```

`@` fixa um **instante absoluto**; `offset` desloca **relativamente** a cada ponto. **Resultado esperado (painel 5):** a linha `@ start()` é reta; a `offset 5m` é a mesma onda da fila, atrasada 5 min (e começa 5 min "depois" da borda se não houver dados antes).

---

## 🏭 Casos reais

### 1. Dashboard de incidente: "quanto piorou desde o início da janela?"

Durante um incidente, o SRE seleciona no Grafana o intervalo "desde o início do problema" e quer ver o **delta** da fila de mensagens do Kafka:

```promql
sum(kafka_consumergroup_lag{consumergroup="checkout"})
  - sum(kafka_consumergroup_lag{consumergroup="checkout"} @ start())
```

Em um único painel, o gráfico começa em 0 e mostra quanto o lag cresceu/diminuiu desde a borda esquerda, qualquer que seja o zoom.

### 2. Crescimento de disco no período selecionado

O `@` se aplica **a cada seletor**, não à expressão inteira (`(a - b) @ start()` é erro). Marque os dois:

```promql
  (node_filesystem_size_bytes{mountpoint="/"} - node_filesystem_avail_bytes{mountpoint="/"})
- (node_filesystem_size_bytes{mountpoint="/"} @ start() - node_filesystem_avail_bytes{mountpoint="/"} @ start())
```

O equivalente para **alerta** (sem janela de dashboard) usa uma janela fixa:

```yaml
- alert: DiscoCrescendoRapido
  expr: delta(node_filesystem_avail_bytes{mountpoint="/"}[6h]) < -50e9   # perdeu 50 GB em 6h
  for: 30m
  labels: {severity: warning}
```

### 3. Regras: **não use** `start()` em alertas

Em regras de alerta/recording, a avaliação é sempre **instantânea**: `start()` = `time()`. Uma regra "variação desde o início" deve usar uma janela explícita:

```yaml
- alert: LagCresceuMuito
  expr: delta(kafka_consumergroup_lag_sum{consumergroup="checkout"}[30m]) > 10000
  for: 5m
```

---

## ✅ Quando usar

- **Dashboards**: linha de referência no início da janela (`x @ start()`), "desde o início do gráfico" (`x - x @ start()`), "% da janela decorrida".
- Explorar dados ad hoc no Grafana/Explore com zoom variável.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Regras de alerta / recording rules | janela explícita: [`delta()`](../delta/), [`increase()`](../increase/) |
| Variação de counter com possível reset | `increase(x[range()] @ end())` |
| Horário do ponto atual | [`time()`](../time/) |
| Produção estável (feature experimental pode mudar) | `$__from` do Grafana: `x @ ${__from:date:seconds}` |

## ⚠️ Pegadinhas

1. **Experimental:** sem `--enable-feature=promql-experimental-functions` → erro de parse dizendo que a função experimental não está habilitada.
2. **Instantânea = `time()`**: em stat/tabela/alerta, `x - x @ start()` = 0.
3. **`@ start()` e o lookback**: `x @ start()` precisa de uma amostra nos 5 min anteriores à borda esquerda. Se a série nasceu depois, fica vazio.
4. **Grafana alinha o `start`** a múltiplos do step, então pode ser uns segundos antes da borda visível.
5. **`@` só vale para seletores e subqueries**: `rate(x[5m]) @ start()` dá erro; escreva `rate(x[5m] @ start())`.

## 🎓 Na prova PCA

`start()` é experimental e dificilmente aparece. O que **cai** e esta lição ajuda a fixar:
- **Consulta instantânea vs range** (`/api/v1/query` vs `/api/v1/query_range`, parâmetros `start`, `end`, `step`).
- O **modificador `@`** (`x @ 1700000000`, `@ start()`, `@ end()`) vs `offset`.
- Regras de alerta são avaliadas como consultas **instantâneas**.

**1.** Quais parâmetros a API `/api/v1/query_range` exige?
- A) `query`, `time`
- B) `query`, `start`, `end`, `step`
- C) `query`, `range`
- D) `query`, `start`, `duration`

<details><summary>Resposta</summary>

**B.** `start`/`end` delimitam a janela e `step` a resolução. `start()` e `end()` no PromQL leem exatamente esses valores.
</details>

**2.** O que retorna `http_requests_total @ 1700000000`?
- A) o valor do counter 1 700 000 000 segundos atrás
- B) o valor do counter no instante Unix 1 700 000 000, para todos os pontos
- C) erro: `@` só aceita `start()` e `end()`
- D) o counter multiplicado por 1 700 000 000

<details><summary>Resposta</summary>

**B.** `@` fixa o instante absoluto de avaliação do seletor. A descreveria `offset`.
</details>

**3.** Uma regra de alerta usa `x - x @ start() > 100`. O que acontece?
- A) alerta quando x cresceu 100 desde o início da janela do dashboard
- B) nunca dispara: em avaliação instantânea `start()` = instante avaliado, então a diferença é 0
- C) erro de sintaxe
- D) dispara sempre

<details><summary>Resposta</summary>

**B.** Regras são consultas instantâneas; não existe "janela do dashboard".
</details>

## 📝 Cola rápida

- `start()` = borda **esquerda** do gráfico (range query); em instant query = `time()`.
- `x @ start()` = valor no início da janela (linha reta).
- `x - x @ start()` = variação desde o início (só para gauges ou counters sem reset).
- Experimental: `--enable-feature=promql-experimental-functions`.
- Nunca em alertas.

## 🔗 Relacionadas

[`end()`](../end/) · [`range()`](../range/) · [`step()`](../step/) · [`time()`](../time/) · [`increase()`](../increase/) · [`delta()`](../delta/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#start
