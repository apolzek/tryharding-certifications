# `histogram_avg()`: a média de um native histogram

> **Em uma frase:** `histogram_avg(v)` devolve a **média aritmética** das observações guardadas em cada **native histogram** de `v` (soma ÷ contagem). Séries float (classic `_bucket`, `_sum`, `_count`) são **ignoradas**.

| | |
|---|---|
| **Assinatura** | `histogram_avg(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ **Native** histogram · ❌ Classic (`_bucket`/`_sum`/`_count` são floats → resultado vazio) |
| **Unidade do resultado** | a mesma da métrica observada (segundos, bytes...) |
| **Equivalente** | `histogram_sum(x) / histogram_count(x)` (native) · `rate(x_sum[5m]) / rate(x_count[5m])` (classic) |
| **Dashboard** | http://localhost:3300/d/fn-histogram_avg |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a conta do restaurante

No fim do jantar chega a conta: **R$ 600** para **6 pessoas**. A média é R$ 100 por pessoa. Ninguém precisa saber quem pediu o quê.

O native histogram guarda, além dos buckets, **dois totais**:

- `sum`: a **conta inteira** (soma de todas as durações observadas);
- `count`: o **número de pessoas** (quantas observações).

`histogram_avg()` faz `sum ÷ count`. Mas cuidado com duas coisas que qualquer um que já dividiu conta sabe:

1. **"Média" esconde quem pediu lagosta.** Se 5 pessoas gastaram R$ 50 e uma gastou R$ 350, a média ainda é R$ 100. (A média **esconde a cauda**.)
2. **Não tire média de médias.** Mesa A: 10 pessoas, média R$ 50. Mesa B: 1 pessoa, média R$ 500. A média do restaurante **não** é R$ 275: é (500 + 500) ÷ 11 ≈ R$ 91.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_avg_request_duration_seconds{service="web"}` | histogram (native + classic) | ~**50 obs/s**, média **50ms**. **A cada 5 min, por 60s**, média sobe para **200ms** (cache frio) |
| `histogram_avg_request_duration_seconds{service="batch"}` | histogram (native + classic) | ~**2 obs/s**, média **1s** constante |

As durações são log-normais **com média fixa** (a mediana do web é ≈46ms e o p99 ≈117ms).

```bash
curl -s localhost:8088/metrics | grep -E '^histogram_avg_request_duration_seconds_(sum|count)'
# histogram_avg_request_duration_seconds_sum{service="web"} 1520.3
# histogram_avg_request_duration_seconds_count{service="web"} 30012
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_avg
```

Espere **~2 min** para `[1m]` e **~5 min** para ver um período lento do `web`.

---

## 🔍 Queries passo a passo

### 1. Média por serviço

```promql
histogram_avg(rate(histogram_avg_request_duration_seconds[1m]))
```

**O que faz:** `rate(...[1m])` transforma o histograma acumulado em "o histograma do último minuto (por segundo)"; `histogram_avg` divide sum por count.
**Resultado esperado** (segundos):

| service | normal | período lento |
|---|---|---|
| web | ≈ **0.05** | ≈ **0.2** (60s a cada 5 min) |
| batch | ≈ **1.0** | ≈ **1.0** |

> 💡 Dividir `rate(sum)/rate(count)` cancela o "por segundo", então o resultado é segundos por requisição, não "por segundo".

---

### 2. Três jeitos de escrever a mesma média

```promql
histogram_avg(rate(histogram_avg_request_duration_seconds{service="web"}[1m]))
histogram_sum(rate(histogram_avg_request_duration_seconds{service="web"}[1m]))
  / histogram_count(rate(histogram_avg_request_duration_seconds{service="web"}[1m]))
rate(histogram_avg_request_duration_seconds_sum{service="web"}[1m])
  / rate(histogram_avg_request_duration_seconds_count{service="web"}[1m])     # classic
```

**Resultado esperado:** **três linhas sobrepostas**. As duas primeiras são idênticas por definição (a documentação diz que são equivalentes). A terceira usa as séries classic `_sum`/`_count`, que carregam os mesmos totais.

---

### 3. ❌ Média das médias vs ✅ média global

```promql
avg(histogram_avg(rate(histogram_avg_request_duration_seconds[1m])))   # ❌
histogram_avg(sum(rate(histogram_avg_request_duration_seconds[1m])))   # ✅
```

**O que faz:**
- A errada calcula a média de cada serviço e depois tira a média **simples** entre eles, dando **o mesmo peso** ao `batch` (2 req/s) e ao `web` (50 req/s).
- A certa **soma os histogramas** (junta as contas das mesas) e só então divide.

**Resultado esperado:**

| | normal | período lento do web |
|---|---|---|
| ❌ `avg(histogram_avg(...))` | (0.05 + 1.0) / 2 ≈ **0.52s** | (0.2 + 1.0) / 2 ≈ **0.6s** |
| ✅ `histogram_avg(sum(...))` | (50×0.05 + 2×1) / 52 ≈ **0.087s** | (50×0.2 + 2×1) / 52 ≈ **0.23s** |

A versão errada diz que "o usuário médio espera meio segundo", quando 96% das requisições são do web e levam 50ms.

---

### 4. A média esconde a cauda

```promql
histogram_avg(rate(histogram_avg_request_duration_seconds{service="web"}[1m]))
histogram_quantile(0.5,  rate(histogram_avg_request_duration_seconds{service="web"}[1m]))
histogram_quantile(0.99, rate(histogram_avg_request_duration_seconds{service="web"}[1m]))
```

**Resultado esperado (normal):** média ≈ **50ms**, p50 ≈ **46ms**, p99 ≈ **117ms**. No período lento todos sobem 4×: média ≈ 200ms, p99 ≈ 470ms.
**Moral:** "média de 50ms" soa ótimo, mas 1 em cada 100 usuários espera mais que o dobro disso. Para SLOs de latência, use [`histogram_quantile()`](../histogram_quantile/) ou [`histogram_fraction()`](../histogram_fraction/).

---

### 5. ❌ Sem `rate()`: média desde que o processo subiu

```promql
histogram_avg(histogram_avg_request_duration_seconds{service="web"})             # ❌ acumulado
histogram_avg(rate(histogram_avg_request_duration_seconds{service="web"}[1m]))   # ✅ último minuto
```

**Resultado esperado:** a sem `rate` fica quase **reta em ~0.05–0.07s**, porque um minuto lento se dilui em todo o histórico. A com `rate` mostra o degrau para 0.2s.

> 💡 O acumulado zera a cada restart do processo: logo após um restart as duas linhas ficam parecidas; com horas de uptime, a sem `rate` vira quase uma reta.

---

### 6. ❌ Em histograma classic: vazio

```promql
histogram_avg(rate(histogram_avg_request_duration_seconds_bucket[1m]))
```

**Resultado esperado:** **vazio** ("No data"). As séries `_bucket`, `_sum` e `_count` são **floats**, e `histogram_avg` ignora floats sem dar erro. Com classic, use `rate(x_sum[1m]) / rate(x_count[1m])` (painel 2).

---

## 🏭 Casos reais

> O cenário fake imita o caso 2: `histogram_avg_request_duration_seconds{service}` com um serviço de alto tráfego (`web`) e um de baixo tráfego e lento (`batch`).

### Caso 1: latência média no dashboard RED

No painel "Duration" do método RED, é comum ter a média ao lado do p99:

```promql
# native
histogram_avg(sum by (job) (rate(http_server_request_duration_seconds[5m])))
# classic (o mesmo número)
  sum by (job) (rate(http_server_request_duration_seconds_sum[5m]))
/ sum by (job) (rate(http_server_request_duration_seconds_count[5m]))
```

Recording rule (convenção `nível:métrica:operação`):

```yaml
- record: job:http_server_request_duration_seconds:mean5m
  expr: |2
      sum by (job) (rate(http_server_request_duration_seconds_sum[5m]))
    / sum by (job) (rate(http_server_request_duration_seconds_count[5m]))
```

### Caso 2: "a média da empresa" num relatório

O gerente pede "a latência média de todos os serviços". Se o dashboard faz `avg(job:http_server_request_duration_seconds:mean5m)`, um job batch com 2 req/s a 1s puxa a média para ~0.5s, embora 96% dos usuários vejam 50ms (painel 3). O certo é **somar sum e count** e dividir no fim:

```promql
histogram_avg(sum(rate(http_server_request_duration_seconds[5m])))
```

Recording rule que o relatório deve usar (soma numerador e denominador **antes** de dividir):

```yaml
- record: :http_server_request_duration_seconds:mean5m
  expr: histogram_avg(sum(rate(http_server_request_duration_seconds[5m])))
- alert: MeanLatencyRegression
  expr: :http_server_request_duration_seconds:mean5m > 1.5 * (:http_server_request_duration_seconds:mean5m offset 1d)
  for: 30m
  labels: { severity: ticket }
```

### Caso 3: tempo médio de GC / de compactação

Histogramas do próprio Prometheus e de runtimes:

```promql
# duração média das compactações do TSDB
rate(prometheus_tsdb_compaction_duration_seconds_sum[1h]) / rate(prometheus_tsdb_compaction_duration_seconds_count[1h])
```

Com native histogram seria `histogram_avg(rate(prometheus_tsdb_compaction_duration_seconds[1h]))`. A média aqui é a métrica certa: o que interessa é o custo típico de cada compactação, não a cauda.

---

## ✅ Quando usar

- **Latência média** de um native histogram: `histogram_avg(rate(http_request_duration_seconds[5m]))`.
- **Tamanho médio** de resposta/payload/lote.
- **Planejamento de capacidade**: média × taxa = trabalho total (veja [`histogram_sum()`](../histogram_sum/)).
- Comparar a média com a mediana: se média ≫ p50, a distribuição tem **cauda longa**.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| A métrica é **classic** (só `_bucket`/`_sum`/`_count`) | `rate(x_sum[5m]) / rate(x_count[5m])` |
| SLO de latência ("99% abaixo de 300ms") | [`histogram_quantile()`](../histogram_quantile/) · [`histogram_fraction()`](../histogram_fraction/) |
| Quer saber o quanto os valores **variam** | [`histogram_stddev()`](../histogram_stddev/) |
| Média de uma **gauge** ao longo do tempo | [`avg_over_time()`](../avg_over_time/) |

## ⚠️ Pegadinhas

1. **Só native**: em floats o resultado é vazio **sem erro** (painel 6). Se o painel "sumiu", confira se você não está consultando `x_bucket`.
2. **Média de médias** (painel 3): agregue os histogramas com `sum` **antes** de `histogram_avg`.
3. **Esquecer o `rate()`** (painel 5).
4. **A média esconde a cauda** (painel 4).
5. **Não há estimativa aqui**: diferente de quantis, a média usa `sum` e `count` **exatos** (não depende da resolução dos buckets).
6. **Janela sem observações** → `NaN` (0/0).

## 🎓 Na prova PCA

O que costuma cair:
- **Média de um histograma (ou summary) classic** = `rate(x_sum[5m]) / rate(x_count[5m])`. Clássico de prova.
- `histogram_avg` é o atalho **para native histograms**; em floats devolve vazio.
- Divisão de dois `rate` com a mesma janela: o "por segundo" se cancela, sobra a unidade da métrica (segundos por requisição).
- Agregar: some numerador e denominador **antes** de dividir (`sum(rate(_sum)) / sum(rate(_count))`).

**1.** Qual query dá a latência média dos últimos 5 min de um histograma classic?
- A) `avg(http_request_duration_seconds_bucket)`
- B) `rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])`
- C) `avg_over_time(http_request_duration_seconds_sum[5m])`
- D) `histogram_avg(http_request_duration_seconds_bucket[5m])`

<details><summary>Resposta</summary>

**B.** `_sum` e `_count` são counters; a razão dos `rate` é a média das observações da janela. D é inválido (range vector) e, mesmo com `rate`, daria vazio em floats.
</details>

**2.** `histogram_avg(rate(x_bucket[5m]))` retorna...
- A) A média correta
- B) Erro de tipo
- C) Um resultado vazio
- D) NaN

<details><summary>Resposta</summary>

**C.** `histogram_avg` ignora amostras float silenciosamente. `x_bucket` é float, então nada volta.
</details>

**3.** Serviço A: 100 req/s com média 10ms. Serviço B: 1 req/s com média 1s. Qual é a latência média de todas as requisições?
- A) ≈ 505ms
- B) ≈ 20ms
- C) ≈ 10ms
- D) ≈ 1s

<details><summary>Resposta</summary>

**B.** (100×0.01 + 1×1) / 101 = 2/101 ≈ 0.0198s. A resposta A é a "média das médias", o erro do painel 3.
</details>

**4.** Qual afirmação sobre `histogram_avg` está correta?
- A) É uma estimativa baseada nos buckets
- B) Usa sum e count, que são exatos, então não depende da resolução dos buckets
- C) Funciona igualmente em summaries
- D) Precisa de feature flag

<details><summary>Resposta</summary>

**B.** Diferente de quantis e desvio padrão, a média usa os totais exatos. Summaries são floats (`_sum`/`_count`) → use a divisão manual.
</details>

## 📝 Cola rápida

- `histogram_avg(rate(x[5m]))` = `histogram_sum(...)/histogram_count(...)` (só **native**).
- Classic/summary: `rate(x_sum[5m]) / rate(x_count[5m])`.
- Agregue antes de dividir: `histogram_avg(sum(rate(x[5m])))`. Nunca `avg(histogram_avg(...))`.
- A média é **exata** (não é estimativa), mas **esconde a cauda**: para SLO use quantis/fração.
- Sem `rate` = média desde o start do processo.

## 🔗 Relacionadas

[`histogram_sum()`](../histogram_sum/) · [`histogram_count()`](../histogram_count/) · [`histogram_quantile()`](../histogram_quantile/) · [`histogram_stddev()`](../histogram_stddev/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_avg
