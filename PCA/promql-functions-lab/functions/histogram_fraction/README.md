# `histogram_fraction()`: que fração das observações ficou entre dois valores

> **Em uma frase:** `histogram_fraction(lower, upper, b)` estima a **fração (0 a 1)** das observações de cada histograma em `b` que caiu entre `lower` e `upper`. É a ferramenta natural para **SLO de latência** ("% de requisições ≤ 200ms") e **Apdex**. Funciona com **native** e **classic** (com `le`).

| | |
|---|---|
| **Assinatura** | `histogram_fraction(lower scalar, upper scalar, b instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Native histogram · ✅ Classic (`*_bucket` com `le`, igual ao `histogram_quantile`) |
| **Unidade do resultado** | fração de 0 a 1 (no Grafana, unidade `percentunit`) |
| **Dashboard** | http://localhost:3300/d/fn-histogram_fraction |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o radar de velocidade

Um radar numa avenida anota a velocidade de cada carro em faixas: "até 40", "até 60", "até 80", "acima". No fim do dia, a pergunta da prefeitura não é "qual a velocidade do 99º carro?" (isso seria o quantil), e sim:

> **"Que % dos carros respeitou o limite de 60 km/h?"**

Isso é o `histogram_fraction(0, 60, ...)`. É a **pergunta inversa** do [`histogram_quantile()`](../histogram_quantile/):

| função | você informa | ela responde |
|---|---|---|
| `histogram_quantile(0.99, h)` | a **porcentagem** (99%) | o **valor** (ex.: 0.35s) |
| `histogram_fraction(0, 0.2, h)` | o **valor** (200ms) | a **porcentagem** (ex.: 97%) |

Se o limite coincide com uma **divisória** do radar (limite de bucket), a resposta é exata. Se não coincide (ex.: 50 km/h num radar com faixas de 40 e 60), o radar **estima** interpolando.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_fraction_http_request_duration_seconds{service="stable"}` | histogram (native + classic) | ~40 obs/s, mediana **60ms** → ≈ **99%** ≤ 200ms, sempre |
| `histogram_fraction_http_request_duration_seconds{service="degrading"}` | histogram (native + classic) | ~40 obs/s, mediana **80ms** → ≈ **97%** ≤ 200ms. **A cada 5 min, por 90s**, mediana vai a **250ms** → só ≈ **33%** ≤ 200ms |

Buckets classic escolhidos **de propósito** para o SLO: `0.05, 0.1, 0.2, 0.4, 0.8, 1.6, 3.2, +Inf` (contêm **0.2** = T do Apdex e **0.8** = 4T).

```bash
curl -s localhost:8088/metrics | grep 'histogram_fraction_http_request_duration_seconds_bucket{le="0.2",service'
# histogram_fraction_http_request_duration_seconds_bucket{le="0.2",service="degrading"} 23011
# histogram_fraction_http_request_duration_seconds_bucket{le="0.2",service="stable"} 26780
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_fraction
```

Espere **~2 min** para `[1m]` e **~5 min** para ver uma degradação completa.

---

## 🔍 Queries passo a passo

### 1. % de requisições em até 200ms (native)

```promql
histogram_fraction(0, 0.2, rate(histogram_fraction_http_request_duration_seconds[1m]))
```

**O que faz:** `rate(...[1m])` = histograma do último minuto; `histogram_fraction(0, 0.2, ...)` = fração das observações entre 0 e 0.2s.
**Resultado esperado:**

| service | normal | degradação (90s a cada 5 min) |
|---|---|---|
| stable | ≈ **0.99** (99%) | ≈ 0.99 |
| degrading | ≈ **0.97** | ≈ **0.33** 💥 |

---

### 2. Classic: `histogram_fraction` com `le` ou divisão manual

```promql
histogram_fraction(0, 0.2,
  sum by (service, le) (rate(histogram_fraction_http_request_duration_seconds_bucket{service="degrading"}[1m])))

sum(rate(histogram_fraction_http_request_duration_seconds_bucket{service="degrading", le="0.2"}[1m]))
  / sum(rate(histogram_fraction_http_request_duration_seconds_count{service="degrading"}[1m]))
```

**O que faz:** a primeira usa a função sobre os buckets classic (o `le` precisa sobreviver ao `sum`, como no `histogram_quantile`). A segunda é o jeito "clássico" de sempre: bucket `le="0.2"` (cumulativo: tudo até 0.2) ÷ total.
**Resultado esperado:** **linhas sobrepostas**, iguais à linha `degrading` do painel 1. Como **0.2 é limite de bucket**, não há interpolação: o valor é exato.
> A própria documentação recomenda, com classic, **preferir a divisão direta de buckets** (segunda forma): é robusta e deixa claro se o limite existe.

---

### 3. Fração **lenta** e o orçamento de erro

```promql
histogram_fraction(0.2, +Inf, rate(histogram_fraction_http_request_duration_seconds[1m]))
vector(0.05)   # SLO: no máximo 5% acima de 200ms
```

**O que faz:** `+Inf` é um limite válido. "Entre 0.2 e infinito" = **mais lentas que 200ms** (= `1 − fração do painel 1`).
**Resultado esperado:** `stable` ≈ **1%**; `degrading` ≈ **3%** (abaixo da linha de 5%, SLO ok) e ≈ **67%** na degradação (SLO estourado).

---

### 4. Apdex (T = 200ms)

```promql
# native
( histogram_fraction(0, 0.2, rate(histogram_fraction_http_request_duration_seconds[1m]))
+ histogram_fraction(0, 0.8, rate(histogram_fraction_http_request_duration_seconds[1m])) ) / 2

# classic (como na doc de boas práticas do Prometheus)
( sum by (service) (rate(histogram_fraction_http_request_duration_seconds_bucket{le="0.2"}[1m]))
+ sum by (service) (rate(histogram_fraction_http_request_duration_seconds_bucket{le="0.8"}[1m])) )
/ 2 / sum by (service) (rate(histogram_fraction_http_request_duration_seconds_count[1m]))
```

**O que faz:** Apdex = (satisfeitos + tolerantes/2) / total, com **satisfeito ≤ T** e **tolerante entre T e 4T**. Como `F(0.8)` já inclui os satisfeitos, a conta vira `(F(T) + F(4T)) / 2`.
**Resultado esperado:** `stable` ≈ **1.0**; `degrading` ≈ **0.98** normal e ≈ **0.66** na degradação ((0.33 + 0.99) / 2). Native e classic batem, porque 0.2 e 0.8 são limites de bucket.

---

### 5. Limite **fora** dos buckets classic: estimativa

```promql
histogram_fraction(0, 0.3, rate(histogram_fraction_http_request_duration_seconds{service="degrading"}[1m]))            # native
histogram_fraction(0, 0.3, sum by (le) (rate(histogram_fraction_http_request_duration_seconds_bucket{service="degrading"}[1m])))  # classic
```

**O que faz:** 0.3 fica **entre** os buckets classic 0.2 e 0.4. O classic supõe distribuição **uniforme** no bucket e interpola linearmente. O native tem buckets de ~9% de largura e erra bem menos.
**Resultado esperado:** normal: native ≈ **99.6%**, classic ≈ **98.3%**. Na degradação: native ≈ **64%** (valor real), classic ≈ **58%**. Com buckets classic mais esparsos o erro seria ainda maior.

---

### 6. Requisições lentas **por segundo**

```promql
histogram_count(rate(histogram_fraction_http_request_duration_seconds[1m]))
  * histogram_fraction(0.2, +Inf, rate(histogram_fraction_http_request_duration_seconds[1m]))
```

**Resultado esperado:** `stable` ≈ **0.4 req/s** lentas; `degrading` ≈ **1.3 req/s** e ≈ **27 req/s** (de 40) na degradação. "% ruim" vira "quantos usuários afetados".

---

### 7. SLO "agora" (bargauge, janela de 5 min)

```promql
histogram_fraction(0, 0.2, rate(histogram_fraction_http_request_duration_seconds[5m]))
```

**Resultado esperado:** `stable` ≈ 99%; `degrading` entre ≈ **75% e 97%**, dependendo de quanto da degradação está dentro dos últimos 5 min.

---

## 🏭 Casos reais

> O cenário fake imita os casos 1 e 2: `histogram_fraction_http_request_duration_seconds{service}` com buckets classic alinhados ao SLO (0.2 e 0.8), um serviço estável e um que degrada.

### Caso 1: SLO "99% das requisições em até 200ms" com burn rate

A forma mais usada em SLOs (Google SRE Workbook, Sloth, Pyrra) é a **fração ruim** como "taxa de erro":

```yaml
groups:
  - name: checkout-latency-slo
    rules:
      # fração de requisições LENTAS (o "erro" do SLI de latência)
      - record: job:http_server_request_duration_seconds:slow_ratio_5m
        expr: histogram_fraction(0.2, +Inf, sum by (job) (rate(http_server_request_duration_seconds[5m])))
      - record: job:http_server_request_duration_seconds:slow_ratio_1h
        expr: histogram_fraction(0.2, +Inf, sum by (job) (rate(http_server_request_duration_seconds[1h])))

      # SLO 99% => orçamento de 1%. Burn rate 14.4 = gasta 2% do orçamento mensal em 1h.
      - alert: LatencySLOFastBurn
        expr: |2
              job:http_server_request_duration_seconds:slow_ratio_1h > (14.4 * 0.01)
          and job:http_server_request_duration_seconds:slow_ratio_5m > (14.4 * 0.01)
        labels: { severity: page }
```

Em **classic**, o SLI costuma ser escrito direto com o bucket do SLO:

```promql
1 - (
    sum by (job) (rate(http_server_request_duration_seconds_bucket{le="0.2"}[5m]))
  / sum by (job) (rate(http_server_request_duration_seconds_count[5m])))
```

É por isso que o `0.2` **precisa** existir nos buckets.

### Caso 2: Apdex do e-commerce

Apdex com T = 200ms, a fórmula da doc de boas práticas do Prometheus (classic):

```promql
(
  sum(rate(http_request_duration_seconds_bucket{le="0.2"}[5m])) by (job)
+
  sum(rate(http_request_duration_seconds_bucket{le="0.8"}[5m])) by (job)
) / 2 / sum(rate(http_request_duration_seconds_count[5m])) by (job)
```

Native: `(histogram_fraction(0, 0.2, h) + histogram_fraction(0, 0.8, h)) / 2` (painel 4).

```yaml
- record: job:http_request_duration_seconds:apdex5m
  expr: |
    (
        histogram_fraction(0, 0.2, sum by (job) (rate(http_request_duration_seconds[5m])))
      + histogram_fraction(0, 0.8, sum by (job) (rate(http_request_duration_seconds[5m])))
    ) / 2
- alert: ApdexLow
  expr: job:http_request_duration_seconds:apdex5m < 0.85
  for: 10m
  labels: { severity: warning }
```

### Caso 3: SLO do API server do Kubernetes

O kube-prometheus calcula o SLI de latência do apiserver contando requisições **abaixo** do limite (bucket `le="1"` para verbos de leitura de um recurso), como fração do total. Com native histograms isso vira:

```promql
histogram_fraction(0, 1, sum by (verb) (rate(apiserver_request_duration_seconds{verb=~"GET"}[5m])))
```

---

## ✅ Quando usar

- **SLO de latência**: "99% das requisições em até 300ms" → `histogram_fraction(0, 0.3, rate(x[5m])) >= 0.99`.
- **Burn rate de SLO**: `histogram_fraction(0.3, +Inf, rate(x[1h]))` / orçamento.
- **Apdex** (painel 4).
- **Faixas de tamanho**: "% de payloads acima de 1 MB" → `histogram_fraction(1e6, +Inf, ...)`.
- **Contar eventos numa faixa** junto com [`histogram_count()`](../histogram_count/) (painel 6).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer o **valor** de um percentil ("qual é o p99?") | [`histogram_quantile()`](../histogram_quantile/) |
| Classic e o limite **é** um bucket | Divisão direta: `rate(x_bucket{le="0.2"}[5m]) / rate(x_count[5m])` (mais robusto) |
| Classic e o limite **não é** um bucket | Adicione o bucket na instrumentação, ou migre para native |
| Quer % de **amostras** de uma gauge acima de um limite | `avg_over_time((x > bool 0.2)[5m:])` |

## ⚠️ Pegadinhas

1. **Limite desalinhado com o bucket** (classic): o resultado é uma **estimativa** que pode ser bem ruim (painel 5). Com native o erro é pequeno.
2. **`lower = 0` vs `-Inf`**: para durações, 0 basta. Se o histograma tiver valores **negativos**, use `-Inf` para incluir "tudo até o limite".
3. **Inclusivo/exclusivo** só importa se o limite cai **exatamente** num limite de bucket. Em schemas exponenciais padrão, o limite superior do bucket é inclusivo.
4. **`NaN` observados** ficam fora de qualquer bucket no native: `histogram_fraction(-Inf, +Inf, b)` pode dar **menos que 1**.
5. **Sem `rate()`** = fração desde que o processo subiu (quase não reage).
6. **Classic: esqueceu o `le` no `by`** → vazio.
7. **Janela sem observações** → `NaN`.

## 🎓 Na prova PCA

O que costuma cair:
- **"% de requisições abaixo de X"** num histograma classic = `rate(x_bucket{le="X"}[5m]) / rate(x_count[5m])`. Só é exato se `X` for um limite de bucket.
- **Apdex** com buckets (satisfeito ≤ T, tolerante ≤ 4T): `(bucket{le=T} + bucket{le=4T}) / 2 / count`.
- `histogram_fraction(lower, upper, b)` funciona em **classic e native**; o resultado vai de **0 a 1**.
- É a pergunta **inversa** de `histogram_quantile`.
- Summaries **não** permitem calcular isso (não têm buckets).

**1.** Os buckets são `0.1, 0.25, 0.5, 1, +Inf`. Qual query dá a fração exata de requisições em até 250ms?
- A) `histogram_quantile(0.25, rate(x_bucket[5m]))`
- B) `sum(rate(x_bucket{le="0.25"}[5m])) / sum(rate(x_count[5m]))`
- C) `sum(rate(x_bucket{le="0.25"}[5m])) / sum(rate(x_sum[5m]))`
- D) `rate(x_bucket{le="0.25"}[5m])`

<details><summary>Resposta</summary>

**B.** Bucket cumulativo "até 0.25" dividido pelo total. A é o p25 (outra pergunta). C divide por segundos somados. D é req/s rápidas, não fração.
</details>

**2.** Com os mesmos buckets, você calcula `histogram_fraction(0, 0.3, sum by (le) (rate(x_bucket[5m])))`. O resultado é...
- A) Exato
- B) Uma estimativa por interpolação linear entre os buckets 0.25 e 0.5
- C) Vazio, porque 0.3 não é um bucket
- D) Sempre 1

<details><summary>Resposta</summary>

**B.** Limites fora dos buckets são estimados, com erro que pode ser grande em classic (painel 5).
</details>

**3.** Qual é o Apdex quando 90% das requisições são ≤ T, 8% estão entre T e 4T e 2% são > 4T?
- A) 0.90
- B) 0.94
- C) 0.98
- D) 0.86

<details><summary>Resposta</summary>

**B.** (satisfeitos + tolerantes/2) = 0.90 + 0.08/2 = 0.94. Com a fórmula de buckets: (F(T) + F(4T)) / 2 = (0.90 + 0.98) / 2 = 0.94.
</details>

**4.** Qual é o equivalente de `histogram_fraction(0.2, +Inf, h)`?
- A) `histogram_fraction(0, 0.2, h)`
- B) `1 - histogram_fraction(0, 0.2, h)` (para histogramas sem valores negativos nem NaN)
- C) `histogram_quantile(0.2, h)`
- D) `histogram_count(h) - 0.2`

<details><summary>Resposta</summary>

**B.** Tudo acima de 0.2 = 1 − tudo até 0.2 (desde que não haja observações negativas ou NaN).
</details>

**5.** Por que não dá para calcular "% abaixo de 200ms" a partir de um **summary**?
- A) Summaries não têm `_count`
- B) Summaries só expõem quantis pré-calculados e `_sum`/`_count`, sem buckets
- C) Dá sim, com `histogram_fraction`
- D) Porque summaries são gauges

<details><summary>Resposta</summary>

**B.** Sem buckets não há distribuição para consultar. Se você precisa de SLO por limite, instrumente com **histogram**.
</details>

## 📝 Cola rápida

- `histogram_fraction(lo, hi, rate(x[5m]))` → fração 0..1 entre `lo` e `hi` (classic com `le` ou native).
- SLO classic: `rate(x_bucket{le="0.2"}[5m]) / rate(x_count[5m])`. O bucket do SLO **tem que existir**.
- Apdex = `(F(T) + F(4T)) / 2`.
- Fração lenta: `histogram_fraction(T, +Inf, ...)`. Quantidade: × `histogram_count`.
- Inverso de `histogram_quantile`. Limite desalinhado = estimativa.

## 🔗 Relacionadas

[`histogram_quantile()`](../histogram_quantile/) · [`histogram_count()`](../histogram_count/) · [`histogram_avg()`](../histogram_avg/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_fraction
· Apdex: https://prometheus.io/docs/practices/histograms/#apdex-score
