# 07 · O p99 mentiroso

> **Em uma frase:** 10% das compras levam quase 1 segundo, clientes reclamam, e o painel de SLO jura que o p99 do checkout está em 365ms. Alguém tirou a **média de quantis**.

| | |
|---|---|
| **Dificuldade** | ⭐⭐ |
| **Tempo-alvo** | 20 min |
| **Tópicos PCA** | histogramas, `_bucket` e `le`, `histogram_quantile`, ordem certa de `rate` → `sum by (le)` → quantil, por que quantis não se agregam |
| **Arquivos que você vai editar** | `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ TICKET #90417 · Customer Success → SRE                                   │
├──────────────────────────────────────────────────────────────────────────┤
│ Três clientes enterprise reclamando que "o botão Comprar demora".        │
│ Um mandou um vídeo: quase 1 segundo esperando.                           │
│                                                                          │
│ Olhamos o painel de SLO: "Checkout p99 = 365ms", abaixo do SLO de 500ms. │
│ O alerta CheckoutLatenciaP99Alta está verde.                             │
│                                                                          │
│ Pedido: se o p99 real do checkout estiver acima de 500ms, a recording    │
│ rule job:http_request_duration_seconds:p99 tem que mostrar isso e o      │
│ alerta CheckoutLatenciaP99Alta tem que disparar.                         │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 07
# 3 réplicas do checkout: app:9182, app:9184, app:9185
# edite work/prometheus/rules.yml  ->  ./reload.sh  ->  ./check.sh 07
```

## 🩺 Sintomas

- `job:http_request_duration_seconds:p99` ≈ **0.365**.
- O vídeo do cliente mostra ~0.9s.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> olhe o p99 de cada réplica.</summary>

```promql
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{job="checkout"}[1m]))
```

**Resultado esperado:**

| instance | p99 |
|---|---|
| app:9182 | ≈ 0.0495 |
| app:9184 | ≈ 0.0495 |
| app:9185 | ≈ **0.995** |

Uma réplica está lenta. A média desses três números é (0.0495 + 0.0495 + 0.995) / 3 ≈ **0.365**, exatamente o que o painel mostra.
</details>

<details>
<summary><b>Passo 2:</b> quanto tráfego cada réplica atende?</summary>

```promql
sum by (instance) (rate(http_request_duration_seconds_count{job="checkout"}[1m]))
```

**Resultado esperado:** ~45, ~45 e ~**10** req/s. A réplica lenta atende **10% do tráfego**. Se 10% das requisições levam ~0.75s, o p99 do serviço (a requisição na posição 99 de cada 100) está **dentro** dessas lentas. A média trata as três réplicas como se tivessem o mesmo peso e dilui a lenta.
</details>

<details>
<summary><b>Passo 3:</b> tente agregar os buckets "do jeito rápido".</summary>

```promql
histogram_quantile(0.99, sum by (job) (rate(http_request_duration_seconds_bucket{job="checkout"}[1m])))
```

**Resultado esperado:** **vazio**. O `sum by (job)` jogou fora o label `le`, e sem `le` o `histogram_quantile` não sabe qual série é qual bucket. (Outra forma clássica de quebrar este mesmo painel.)
</details>

<details>
<summary><b>Passo 4:</b> o jeito certo.</summary>

```promql
histogram_quantile(0.99, sum by (job, le) (rate(http_request_duration_seconds_bucket{job="checkout"}[1m])))
```

**Resultado esperado:** ≈ **0.95**. Somando os **buckets** (contagens), você reconstrói o histograma do serviço inteiro, e o quantil sai ponderado pelo tráfego real.
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

A recording rule calculava o p99 **por réplica** e depois tirava a **média** (`avg(histogram_quantile(...))`). Quantis não são aditivos nem "médiáveis": a média de p99s não é o p99 de nada. Com uma réplica lenta atendendo 10% do tráfego, a média ficou em 0.365s enquanto o p99 real era ~0.95s.
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/prometheus/rules.yml</code></summary>

```yaml
      - record: job:http_request_duration_seconds:p99
        expr: |
          histogram_quantile(0.99,
            sum by (job, le) (rate(http_request_duration_seconds_bucket{job="checkout"}[1m]))
          )
```

```bash
./reload.sh && ./check.sh 07
```
</details>

## 🛡️ Como evitar

- **A ordem é sempre:** `rate(_bucket[...])` → `sum by (<o que você quer manter>, le)` → `histogram_quantile`. Nunca agregue depois do quantil.
- **Recording rules de histograma guardam os buckets agregados**, não o quantil, se você vai querer re-agregar depois:

```yaml
- record: job:http_request_duration_seconds_bucket:rate1m
  expr: sum by (job, le) (rate(http_request_duration_seconds_bucket[1m]))
- record: job:http_request_duration_seconds:p99
  expr: histogram_quantile(0.99, job:http_request_duration_seconds_bucket:rate1m)
```

- **Para SLO, prefira a razão de "requisições rápidas"** em vez de um quantil: é aditiva, exata no limite do bucket e fácil de agregar.

```yaml
- alert: CheckoutSLOLatencia
  # menos de 99% das requisições abaixo de 0.5s
  expr: |
    sum(rate(http_request_duration_seconds_bucket{job="checkout", le="0.5"}[5m]))
      /
    sum(rate(http_request_duration_seconds_count{job="checkout"}[5m])) < 0.99
```

- **Native histograms** (Prometheus 3.x, quando a coleta deles está habilitada) eliminam o label `le` e a escolha de buckets, e `histogram_quantile(0.99, sum(rate(x[1m])))` funciona direto.
- **Summary não salva você aqui:** os quantis de um `summary` são calculados no cliente e **não podem** ser agregados entre réplicas de jeito nenhum.

## 📝 Postmortem (exemplo)

> **Resumo:** de 10:00 a 15:30 UTC, ~10% das requisições do checkout levaram 0.5-1s (SLO: 99% < 500ms). O SLO foi violado por 5h30 sem alerta; o painel mostrava p99 de 365ms.
>
> **Causa raiz:** a recording rule de p99 fazia `avg()` de quantis por réplica. Uma réplica degradada (node com disco lento) atendia 10% do tráfego e foi diluída na média.
>
> **Ações:**
> 1. (corrigir) p99 via `sum by (job, le)` antes do `histogram_quantile`. ✅
> 2. (detectar) alerta de SLO por razão de requisições `le="0.5"` (multi-window burn rate). **Dono:** SRE.
> 3. (detectar) painel de p99 **por instância** ao lado do agregado, para enxergar réplicas outliers. **Dono:** time checkout.
> 4. (prevenir) lint: `avg`/`sum` envolvendo `histogram_quantile` falha o PR. **Dono:** plataforma.

## 🎓 Na prova PCA

<details>
<summary>Q1. Which expression correctly computes the 95th percentile latency across all instances of a job?</summary>

`histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))`. O `le` tem que sobreviver à agregação, e o `rate` vem antes do `sum`.
</details>

<details>
<summary>Q2. Can you aggregate the quantiles exposed by a Summary metric across instances?</summary>

**Não** de forma estatisticamente válida. Summaries calculam quantis no cliente; para agregar, use histogramas.
</details>
