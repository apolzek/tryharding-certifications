# `step()`: a distância entre pontos do gráfico (experimental)

> **Em uma frase:** `step()` devolve o **step** (resolução, em segundos) da consulta *range* em andamento: a distância entre dois pontos do gráfico. No Grafana ele **muda com o zoom**. Numa consulta **instantânea**, `step()` = **0**.

| | |
|---|---|
| **Assinatura** | `step() → scalar` |
| **Status** | 🧪 **experimental**: exige `--enable-feature=promql-experimental-functions` |
| **Tipo de métrica** | nenhuma. Também pode ser usado como **duração**: `max_over_time(x[step()])` |
| **Unidade do resultado** | segundos |
| **Dashboard** | http://localhost:3300/d/fn-step |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o intervalo entre quadros do filme

O gráfico é um **filme** (veja [`start()`](../start/)); `step()` é o **intervalo entre dois quadros**.

- Filme em **câmera lenta** (zoom de 15 min): um quadro a cada **5 s**. Você vê cada scrape.
- Filme em **time-lapse** (zoom de 7 dias): um quadro a cada **~10 min**. Entre dois quadros aconteceram ~120 scrapes que **ninguém desenhou**.

Um pico que durou 5 s cai **entre dois quadros** do time-lapse e simplesmente **não aparece**. `max_over_time(x[step()])` resolve: cada quadro vira "o **máximo** do que aconteceu desde o quadro anterior".

Como o Grafana escolhe o step (aproximadamente):

```
step = max( Min interval do datasource (aqui 5s) , (to - from) / largura do painel em pixels )
          arredondado para um valor "bonito" (5s, 10s, 15s, 30s, 1m, 5m...)
```

| Zoom (painel de ~800 px) | step típico |
|---|---|
| 15 min | **5 s** (limitado pelo Min interval) |
| 1 h | 5 s |
| 6 h | ~30 s |
| 24 h | ~2 min |
| 7 dias | ~10-15 min |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `step_http_requests_total` | counter | ~**10 req/s** |
| `step_latency_seconds` | gauge | ~**0,12 s**, com **picos de 2,5 s que duram só 5 s** (um scrape) a cada **47 s** |

```bash
curl -s localhost:8088/metrics | grep '^step_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-step
```

Depois de ~1 h de dados, mude o zoom para "Last 6 hours" (ou, no painel, *Query options → Min interval = 1m*) e compare os painéis 1 e 3.

---

## 🔍 Queries passo a passo

### 1. `step()` num gráfico

```promql
step()
```

**Resultado esperado:** linha **reta em 5** no zoom de 15 min (o *Min interval* do datasource é 5 s). Em 6 h: ~**30**. (O `tools/validate.py` do lab usa step 15 s.)

---

### 2. Quantos scrapes cabem em cada ponto

```promql
count_over_time(step_latency_seconds[step()])
```

**Resultado esperado:** **1** com step 5 s (scrape de 5 s). Com step 30 s: **6**. Com step 15 s: **3**. É uma forma visual de entender o que o step "esconde".

---

### 3. Latência crua vs `max_over_time(x[step()])`

```promql
step_latency_seconds                          # amostra que caiu no ponto
max_over_time(step_latency_seconds[step()])   # pior valor desde o ponto anterior
```

**Resultado esperado:**
- **Zoom 15 min (step 5 s):** as duas linhas **coincidem**: base ~0,12 s e ~19 picos de **2,5 s** (um a cada 47 s).
- **Zoom 6 h (step 30 s):** a crua mostra só **~1 em cada 6** picos (os que calharam de cair em cima de um ponto); a `max_over_time[step()]` mostra **todos** em 2,5 s.

---

### 4. ❌ O que dá errado: `increase(x[step()])` com step = scrape

```promql
increase(step_http_requests_total[step()])        # step 5s → janela com 1 amostra → VAZIO
increase(step_http_requests_total[step() + 10s])  # janela de 15s → sempre 3 amostras
```

**Resultado esperado:** no zoom de 15 min a primeira linha **não aparece** (cada janela `[5s]` tem uma amostra só, e `increase`/`rate` precisam de **2**). A segunda aparece em ≈ **150** (10 req/s × 15 s), ondulando entre ~120 e ~180 porque o tráfego do cenário tem uma leve onda de 90 s. Com zoom maior (step ≥ 10 s), a primeira também aparece, em ≈ 10 × step.

> É exatamente por isso que o Grafana inventou o `$__rate_interval` = `max(4 × scrape_interval, step + scrape_interval)`: garante sempre amostras suficientes na janela. `step() + 10s` é uma versão "caseira" disso (duration expressions aceitam `+`, `-`, `*`, `/`, mas **não** `max()`).

---

### 5. Numa consulta instantânea

```promql
step()     # 0
```

**Resultado esperado:** **0**. E qualquer `x[step()]` numa consulta instantânea dá erro *"duration must be greater than 0"*.

---

## 🏭 Casos reais

### 1. Dashboard de latência que não esconde picos

O SRE do checkout reclama: "o gráfico de 7 dias diz p99 < 300 ms, mas os clientes reclamaram de lentidão na terça". Com step de 10 min, picos de 1 min somem. Painel corrigido:

```promql
max_over_time(
  histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[1m])))[step():]
)
```

Para alerta (instantâneo), a janela é fixa e o `for:` substitui o step:

```yaml
- alert: CheckoutP99Alto
  expr: histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[5m]))) > 0.5
  for: 5m
  labels: {severity: page}
```

### 2. Gráfico de barras "eventos por ponto" que soma certo

Um painel de barras de erros: cada barra deve representar exatamente os erros daquele intervalo, e a soma das barras deve bater com o total do período.

```promql
increase(http_requests_total{code=~"5.."}[step()])     # sem sobreposição nem buracos (se step ≥ 2 scrapes)
```

(No Grafana tradicional isso se faz com `[$__interval]`.) Recording rule "por minuto" para relatórios:

```yaml
- record: job:http_errors:increase1m
  expr: sum by (job) (increase(http_requests_total{code=~"5.."}[1m]))
```

---

## ✅ Quando usar

- **Não perder picos** ao dar zoom out: `max_over_time(x[step()])`.
- Barras que "somam certo": `increase(x[step()])` / `sum_over_time(x[step()])`.
- Clientes fora do Grafana (sem `$__interval`) que fazem range queries.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Alertas / recording rules (instantâneas: step = 0) | janelas fixas + `for:` |
| Step menor que 2 scrapes com `rate`/`increase` | `$__rate_interval` no Grafana, ou `[step() + <folga>]` |
| Produção estável | `$__interval` / `$__rate_interval` do Grafana |

## ⚠️ Pegadinhas

1. **`step()` = 0 em instant query**: `x[step()]` → erro.
2. **`rate`/`increase` com `[step()]` pequeno** → vazio (menos de 2 amostras).
3. **Experimental**; e o valor depende de **quem** faz a consulta (Grafana, largura do painel, Min interval, Max data points).
4. **Picos invisíveis** num gráfico cru com step grande: a amostra desenhada é **a última antes do ponto**, não uma média nem um máximo.
5. **Duration expressions** aceitam aritmética (`step() * 2`, `step() + 10s`), não funções como `max()`.

## 🎓 Na prova PCA

`step()` é experimental; o que cai:
- Parâmetro **`step`** da API `query_range` (resolução) e o fato de uma range query ser uma sequência de instant queries.
- **Lookback delta** (5 min): cada ponto pega a amostra mais recente até 5 min antes.
- Janela de `rate()` deve ter pelo menos **2 amostras** (regra prática: ≥ 4 × scrape interval).

**1.** Numa range query com `step=60s` e scrape de 15 s, qual valor o gráfico mostra para `node_load1` em cada ponto?
- A) a média das 4 amostras do minuto
- B) a amostra mais recente anterior (ou igual) ao instante do ponto, dentro do lookback
- C) o máximo do minuto
- D) a soma das 4 amostras

<details><summary>Resposta</summary>

**B.** Seletores instantâneos pegam a última amostra (até 5 min de lookback). Não há agregação automática: para máximo, use `max_over_time(x[1m])`.
</details>

**2.** Por que `rate(x[15s])` com scrape de 15 s costuma retornar vazio?
- A) `rate()` exige janela ≥ 1 min
- B) a janela tem no máximo 1 amostra e `rate()` precisa de pelo menos 2
- C) `rate()` não aceita segundos
- D) o scrape interval precisa ser maior que a janela

<details><summary>Resposta</summary>

**B.** Regra prática: janela ≥ 4 × scrape interval.
</details>

**3.** O que controla o parâmetro `step` numa chamada a `/api/v1/query_range`?
- A) o tamanho da janela de `rate()`
- B) o intervalo entre os instantes em que a expressão é avaliada
- C) o scrape interval
- D) o retention

<details><summary>Resposta</summary>

**B.** `step` = resolução do resultado. Não muda a janela de `rate()` (ela é a que você escreve) nem nada no scrape.
</details>

## 📝 Cola rápida

- `step()` = distância entre pontos (s) da range query; **0** em instant query.
- Zoom out → step maior → picos somem → `max_over_time(x[step()])`.
- `increase(x[step()])` precisa de step ≥ 2 scrapes (ou `[step() + folga]`).
- Grafana: `$__interval` ≈ step; `$__rate_interval` = versão segura para `rate`.

## 🔗 Relacionadas

[`range()`](../range/) · [`start()`](../start/) · [`end()`](../end/) · [`max_over_time()`](../max_over_time/) · [`count_over_time()`](../count_over_time/) · [`increase()`](../increase/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#step
