# `histogram_quantiles()`: vários percentis numa query só

> **Em uma frase:** `histogram_quantiles(v, "label", φ1, φ2, ...)` faz o mesmo cálculo que [`histogram_quantile()`](../histogram_quantile/), mas para **até 10 quantis de uma vez**, devolvendo uma série por quantil identificada por um label com o nome que você escolher.

| | |
|---|---|
| **Assinatura** | `histogram_quantiles(v instant-vector, quantile_label string, φ_1 scalar, φ_2 scalar, ...) → instant-vector` |
| **Tipo de métrica** | ✅ Histogram classic (`*_bucket` com `le`) · ✅ Native histogram |
| **Unidade do resultado** | a mesma da métrica observada (segundos, bytes...) |
| **Feature flag** | ⚠️ **experimental**: requer `--enable-feature=promql-experimental-functions` (já ligado neste lab) |
| **Dashboard** | http://localhost:3300/d/fn-histogram_quantiles |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a pergunta em lista para o juiz da maratona

Com `histogram_quantile()`, você vai até o juiz da maratona **três vezes**:
"até que tempo chegaram 50% dos corredores?", depois "e 90%?", depois "e 99%?". A cada pergunta ele **recontaria todas as bolinhas das caixas**.

Com `histogram_quantiles()`, você entrega **uma lista**: "50%, 90% e 99%, por favor". Ele conta as caixas **uma vez** e devolve três respostas, cada uma com uma **etiqueta** (`quantile="0.5"`, `quantile="0.9"`, `quantile="0.99"`).

Mesma matemática, **uma query só**, e o resultado já vem pronto para virar legenda ou coluna de tabela.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) usa `NativeHistogramBucketFactor: 1.1` **e** `Buckets`, então o histograma existe como **native** (`x`) e como **classic** (`x_bucket{le}`).

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_quantiles_api_latency_seconds{service="catalog"}` | histogram | ~30 obs/s, mediana **80ms**, p99 ≈ **160ms**, sempre estável |
| `histogram_quantiles_api_latency_seconds{service="checkout"}` | histogram | ~30 obs/s, mediana **120ms**, p99 ≈ **270ms**. **A cada 4 min, por 90s**, 5% das requisições levam **~1.5s** (timeout no gateway de pagamento) |

Buckets classic: `0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, +Inf`.

```bash
curl -s localhost:8088/metrics | grep '^histogram_quantiles_api_latency_seconds_count'
# histogram_quantiles_api_latency_seconds_count{service="catalog"} 18000
# histogram_quantiles_api_latency_seconds_count{service="checkout"} 18000
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_quantiles
```

Espere **~2 min** para as janelas `[1m]` e **~4 min** para ver um "caminho lento" completo do checkout.

---

## 🔍 Queries passo a passo

### 1. Uma query, três linhas

```promql
histogram_quantiles(
  sum(rate(histogram_quantiles_api_latency_seconds{service="checkout"}[1m])),
  "quantile", 0.5, 0.9, 0.99
)
```

**O que faz:**
1. `rate(...[1m])`: o histograma das requisições do último minuto (native).
2. `sum(...)`: junta tudo num histograma só.
3. `histogram_quantiles(..., "quantile", 0.5, 0.9, 0.99)`: calcula os três quantis e cria o label `quantile` em cada resultado.

**Resultado esperado** (unidade: segundos):

| `quantile` | normal | caminho lento (90s a cada 4 min) |
|---|---|---|
| `"0.5"` | ≈ **0.12** | ≈ **0.12** (quase não muda!) |
| `"0.9"` | ≈ **0.19** | ≈ **0.21** |
| `"0.99"` | ≈ **0.27** | ≈ **1.8** 💥 |

**Moral:** só 5% das requisições ficaram lentas, então a mediana não percebe nada. É por isso que se olha **vários** quantis juntos, e esta função torna isso barato.

---

### 2. O jeito antigo: três `histogram_quantile()`

```promql
histogram_quantile(0.5,  sum(rate(histogram_quantiles_api_latency_seconds{service="checkout"}[1m])))
histogram_quantile(0.9,  sum(rate(histogram_quantiles_api_latency_seconds{service="checkout"}[1m])))
histogram_quantile(0.99, sum(rate(histogram_quantiles_api_latency_seconds{service="checkout"}[1m])))
```

**Resultado esperado:** **exatamente** as mesmas três linhas do painel 1. A diferença é operacional: três queries para manter, e o Prometheus seleciona e soma o histograma três vezes.

---

### 3. Com histograma **classic**

```promql
histogram_quantiles(
  sum by (le) (rate(histogram_quantiles_api_latency_seconds_bucket{service="checkout"}[1m])),
  "quantile", 0.5, 0.9, 0.99
)
```

**O que faz:** igual ao `histogram_quantile()` classic: o `le` **tem** que sobreviver ao `sum`.
**Resultado esperado:** números um pouco diferentes do native por causa da interpolação linear nos buckets largos: p50 ≈ **0.14**, p90 ≈ **0.23**, p99 ≈ **0.3 a 0.38** no normal e ≈ **2.0 a 2.3** no caminho lento (bucket `1 → 2.5`).

---

### 4. Por serviço, e com o **nome de label que você quiser**

```promql
histogram_quantiles(
  sum by (service) (rate(histogram_quantiles_api_latency_seconds[1m])),
  "pct", 0.5, 0.99
)
```

**O que faz:** o 2º argumento é o **nome** do label criado. Aqui `pct`. Os labels que sobraram do `sum by (service)` continuam no resultado.
**Resultado esperado:** 4 séries:

| service | pct | valor |
|---|---|---|
| catalog | 0.5 | ≈ 0.08 |
| catalog | 0.99 | ≈ 0.16 |
| checkout | 0.5 | ≈ 0.12 |
| checkout | 0.99 | ≈ 0.27 → ≈ 1.8 no caminho lento |

No Grafana, a legenda `{{service}} p{{pct}}` fica "checkout p0.99".

---

### 5. Tabela: 6 quantis por serviço

```promql
histogram_quantiles(
  sum by (service) (rate(histogram_quantiles_api_latency_seconds[2m])),
  "quantile", 0.5, 0.75, 0.9, 0.95, 0.99, 0.999
)
```

**Resultado esperado:** 12 linhas (2 serviços × 6 quantis). Ótimo para uma tabela de "perfil de latência" num relatório ou em um painel de SLO.

---

### 6. Mínimo e máximo estimados

```promql
histogram_quantiles(sum by (service) (rate(histogram_quantiles_api_latency_seconds[1m])), "quantile", 0, 1)
```

**O que faz:** o quantil `0` é o **menor** valor estimado e o `1` é o **maior**, pelos limites dos buckets.
**Resultado esperado:** mínimo ≈ **0.03–0.04s** nos dois; máximo ≈ **0.2s** no catalog e ≈ **0.4s** no checkout, saltando para **2 a 3s** no caminho lento.

---

## 🏭 Casos reais

> O cenário fake imita o caso 1: `histogram_quantiles_api_latency_seconds{service}` com um "caminho lento" de pagamento que só aparece no p99.

### Caso 1: painel "perfil de latência" do checkout

O gateway de pagamento dá timeout em ~5% das chamadas por alguns minutos. A mediana não muda, o p99 explode. Um único alvo no Grafana mostra p50/p90/p99 com legenda automática:

```promql
histogram_quantiles(
  sum by (http_route) (rate(http_server_request_duration_seconds{job="checkout"}[5m])),
  "quantile", 0.5, 0.9, 0.99)
```

Legenda: `{{http_route}} p{{quantile}}`.

### Caso 2: uma recording rule em vez de três

Antes era preciso uma regra por quantil (`...:p50_5m`, `...:p90_5m`, `...:p99_5m`). Com a função plural, **uma regra** grava as três séries, diferenciadas pelo label:

```yaml
groups:
  - name: latency-quantiles
    rules:
      - record: job:http_server_request_duration_seconds:quantiles_5m
        expr: |
          histogram_quantiles(
            sum by (job) (rate(http_server_request_duration_seconds[5m])),
            "quantile", 0.5, 0.9, 0.99)
      - alert: TailLatencyHigh
        expr: job:http_server_request_duration_seconds:quantiles_5m{quantile="0.99"} > 1
        for: 5m
```

Repare como o alerta filtra `{quantile="0.99"}`: o label criado pela função é um label comum.

### Caso 3: CoreDNS

```promql
histogram_quantiles(
  sum by (le, server) (rate(coredns_dns_request_duration_seconds_bucket[5m])),
  "quantile", 0.5, 0.99)
```

Funciona igual com **classic** (`le` no `by`). Útil para ver se a lentidão de DNS é geral (p50 sobe) ou de poucas consultas (só p99 sobe, ex.: upstream lento).

```yaml
- alert: CoreDNSLatencyTail
  # cauda lenta com mediana normal = problema em poucos upstreams/zonas
  # (não dá para pôr {quantile="0.99"} direto após a função: seletor só vale em nome de métrica.
  #  Em alertas, use o singular ou filtre uma recording rule que grava os quantis.)
  expr: |2
      histogram_quantile(0.99, sum by (le, server) (rate(coredns_dns_request_duration_seconds_bucket[5m]))) > 0.5
    and
      histogram_quantile(0.5, sum by (le, server) (rate(coredns_dns_request_duration_seconds_bucket[5m]))) < 0.05
  for: 10m
  labels: { severity: warning }
```

> ⚠️ Em produção, lembre que é **experimental**: o Prometheus precisa de `--enable-feature=promql-experimental-functions`, e ferramentas externas (Thanos, Mimir, Grafana Cloud) podem ainda não suportar.

---

## ✅ Quando usar

- **Painéis de latência com p50/p90/p99** num único alvo (menos queries, legendas automáticas).
- **Tabelas de perfil de latência** (painel 5).
- **Recording rules** que precisam gravar vários quantis: uma regra só, com o label diferenciando.
- Quando você quer os quantis **como dado** (label) em vez de como "nome da query".

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só precisa de **um** quantil | [`histogram_quantile()`](../histogram_quantile/) (estável, não é experimental) |
| Prometheus **sem** a feature flag `promql-experimental-functions` | [`histogram_quantile()`](../histogram_quantile/) repetido |
| Quer "**que % ficou abaixo de X**" | [`histogram_fraction()`](../histogram_fraction/) |
| Quer a **média** ou a **dispersão** | [`histogram_avg()`](../histogram_avg/) · [`histogram_stddev()`](../histogram_stddev/) |

## ⚠️ Pegadinhas

1. **É experimental**: pode mudar de nome/comportamento em versões futuras. Em alertas críticos, prefira `histogram_quantile()`.
2. **Máximo de 10 quantis** por chamada.
3. **O nome do label não pode colidir** com um label que já existe no resultado (ex.: não use `"service"` se você fez `sum by (service)`). Prefira `"quantile"` ou algo único como `"pct"`.
4. **O valor do label é texto**: `quantile="0.5"`, não `"0.50"`. Filtre depois com `{quantile="0.99"}` exatamente como aparece.
5. **Classic: `le` no `by`**, native: **sem** `le`. Mesma regra do `histogram_quantile()`.
6. **Sem `rate()`** = quantis "desde que o processo subiu".
7. Todas as pegadinhas de interpolação de [`histogram_quantile()`](../histogram_quantile/) valem aqui também.

## 🎓 Na prova PCA

O que costuma cair (a função é nova e experimental; a prova foca nos **conceitos** que ela reaproveita):
- Mesmas regras de `histogram_quantile`: classic precisa de `le` no `by`; native não; `rate` dentro.
- Funções **experimentais** exigem feature flag (`--enable-feature=promql-experimental-functions`).
- Por que olhar **vários** quantis: a mediana descreve o caso típico, o p99 descreve a **cauda**.
- Labels: a função **adiciona** um label ao resultado (como `label_replace` faria).

**1.** O que `histogram_quantiles(sum(rate(x[5m])), "q", 0.5, 0.99)` retorna?
- A) Um escalar com a média entre p50 e p99
- B) Duas séries: `{q="0.5"}` e `{q="0.99"}`
- C) Uma série com dois valores
- D) Erro: o nome do label deve ser `quantile`

<details><summary>Resposta</summary>

**B.** Uma série por quantil, identificada pelo label cujo nome é o 2º argumento (qualquer nome válido).
</details>

**2.** Durante um incidente, o p50 continua em 120ms e o p99 vai de 270ms para 1.8s. O que isso indica?
- A) Todas as requisições ficaram lentas
- B) Uma pequena fração das requisições ficou muito lenta
- C) O tráfego caiu
- D) O histograma está com buckets errados

<details><summary>Resposta</summary>

**B.** Se a mediana não se move, a maioria está normal. Só a cauda (≤ 5% aqui) mudou. Exatamente o cenário deste lab.
</details>

**3.** O que é necessário para usar `histogram_quantiles` no Prometheus 3.x?
- A) Nada, é estável
- B) `--enable-feature=promql-experimental-functions`
- C) `--enable-feature=native-histograms`
- D) Só funciona com recording rules

<details><summary>Resposta</summary>

**B.** Está marcada como experimental na documentação. Sem a flag, a query dá erro de função desconhecida/desabilitada.
</details>

**4.** Qual é o equivalente com funções estáveis?
- A) `quantile(0.5, x) or quantile(0.99, x)`
- B) Várias chamadas a `histogram_quantile(φ, ...)`, uma por quantil
- C) `quantile_over_time(0.99, x[5m])`
- D) Não existe equivalente

<details><summary>Resposta</summary>

**B.** Mesmo cálculo, uma query por φ. `quantile()` é agregação **entre séries** e `quantile_over_time` é sobre amostras float no tempo: outras coisas.
</details>

## 📝 Cola rápida

- `histogram_quantiles(v, "label", φ1, ..., φ10)`: até 10 quantis, uma série por φ.
- Experimental: precisa de `promql-experimental-functions`.
- Mesmas regras do singular: `rate` dentro, `le` no `by` (classic), sem `le` (native).
- Valor do label é texto (`"0.99"`); não use um nome de label que já exista no resultado.
- Olhe p50 **e** p99: mediana = típico, p99 = cauda.

## 🔗 Relacionadas

[`histogram_quantile()`](../histogram_quantile/) · [`histogram_fraction()`](../histogram_fraction/) · [`histogram_avg()`](../histogram_avg/) · [`quantile_over_time()`](../quantile_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantiles
