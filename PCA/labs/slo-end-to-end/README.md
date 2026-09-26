# Lab: SLO de ponta a ponta (SLI → SLO → error budget → burn rate → alerta → dashboard)

> **Em uma frase:** um **SLI** mede o serviço (razão de eventos bons/ruins), o **SLO** é a meta para esse SLI numa janela (99,9% em 30d), o que sobra até a meta é o **error budget**, e os alertas de **burn rate multi-janela** avisam quando o budget está sendo gasto rápido demais, sem alarmes falsos e resetando logo depois do conserto.

| | |
|---|---|
| **Serviço fake** | http://localhost:9171 (`/api/checkout`, `/chaos`, `/metrics`), código em [`app/main.go`](app/main.go) (client_golang v1.24.1) |
| **Prometheus** | http://localhost:9170 (alertas: http://localhost:9170/alerts) |
| **Grafana** | http://localhost:3170/d/slo-checkout (anônimo, sem login) |
| **Regras do lab (escala 1:60)** | [`prometheus/rules/slo-lab.yml`](prometheus/rules/slo-lab.yml) + [testes](prometheus/rules/slo-lab.test.yml) |
| **Regras de produção (janelas reais)** | [`rules-production/slo-rules.yml`](rules-production/slo-rules.yml) + [testes](rules-production/slo-rules.test.yml) |
| **Teste automático** | [`./test.sh`](test.sh) (~2,5 min) |

---

## 🧠 Analogia: o plano de dados do celular

Seu plano tem **10 GB por mês**.

- O **SLI** é o medidor do app da operadora: "quanto eu gastei". Em SRE: *fração de requisições ruins*.
- O **SLO** é a sua regra pessoal: "não quero passar de 10 GB no mês". Em SRE: *99,9% das requisições boas em 30 dias*.
- O **SLA** é o contrato com a operadora, com **multa**: "se a velocidade cair abaixo de X, você ganha desconto". É externo, jurídico e sempre **mais frouxo** que o SLO (você quer perceber o problema **antes** de pagar multa).
- O **error budget** é a franquia: os 10 GB. 99,9% de sucesso = **0,1% de falhas permitidas**. Budget não é para ser guardado: é para ser **gasto** com deploys, experimentos, manutenção.
- O **burn rate** é a **velocidade** de consumo da franquia. Burn **1** = ela acaba exatamente no dia 30. Burn **14,4** = acaba em ~2 dias. Burn **0,5** = sobra metade.
- **Alerta ingênuo** ("avise se gastei mais de 0,1% hoje") apita toda vez que você assiste a um vídeo. **Alerta de burn rate multi-janela**: "avise se na **última hora** você gastou 2% da franquia **e** ainda está gastando **agora** (últimos 5 min)". A janela longa garante que é relevante; a curta garante que ainda está acontecendo.

---

## 📐 SLI, SLO, SLA e a matemática

| | O que é | Exemplo neste lab | Quem liga |
|---|---|---|---|
| **SLI** | medida: eventos bons / eventos válidos | `1 - rate(5xx) / rate(total)` | engenharia |
| **SLO** | meta para o SLI numa janela | 99,9% sem 5xx em 30d · 99% em até 300ms em 30d | engenharia + produto |
| **SLA** | contrato com consequência (multa, crédito) | "99,5% ou devolvemos 10%" | jurídico / clientes |
| **Error budget** | `1 - SLO` | 0,1% das requisições (≈ 43,2 min de indisponibilidade total em 30d) | todo mundo |

```
razão de erro (janela w)  =  Σ rate(5xx[w]) / Σ rate(total[w])
burn rate (w)             =  razão de erro (w) / (1 - SLO)                 ex.: 0,05 / 0,001 = 50x
budget gasto em w         =  burn rate × w / período do SLO                ex.: 14,4 × 1h / 720h = 2%
budget restante           =  1 - razão de erro (período) / (1 - SLO)       1 = intacto · 0 = gasto · <0 = violado
tempo até zerar           =  período do SLO / burn rate                    ex.: 30d / 14,4 ≈ 2 dias
```

**A tabela do SRE Workbook** (cap. 5, *Alerting on SLOs*), para um SLO de 30 dias:

| Alerta | Janela longa | Janela curta (1/12) | Burn rate | Budget gasto ao disparar | Ação |
|---|---|---|---|---|---|
| Fast | 1h | 5m | **14,4** | 2% | page |
| Medium | 6h | 30m | **6** | 5% | page |
| Slow | 1d | 2h | **3** | 10% | ticket |
| Very slow | 3d | 6h | **1** | 10% | ticket |

---

## 🏗️ Arquitetura

```
 curl /chaos?error_rate=0.05&latency_ms=300
            │
 ┌──────────▼───────────┐  scrape 1s   ┌──────────────────────── Prometheus :9170 ─────────────────────────┐
 │ app "checkout" :9171 │─────────────►│ recording rules (1s)                                                │
 │  /api/checkout       │              │   job:slo_errors_per_request:ratio_rate{5s,30s,1m,2m,6m,24m,72m}   │
 │  carga interna 30rps │              │   job:slo_latency_slow_per_request:ratio_rate{5s,1m}                │
 │  http_requests_total │              │ budget (5s): job:slo_errors_per_request:ratio_rate12h               │
 │  http_request_       │              │              job:slo_error_budget_remaining:ratio                   │
 │   duration_seconds   │              │ alertas (1s): SLOErrorBudgetBurn{Fast,Medium,Slow,VerySlow}         │
 └──────────────────────┘              │               SLOLatencyBudgetBurnFast   → /api/v1/alerts, ALERTS   │
                                       └───────────────────────────────┬────────────────────────────────────┘
                                                                       │ PromQL
                                                          ┌────────────▼─────────────┐
                                                          │ Grafana :3170            │
                                                          │ SLI · budget · burn rates│
                                                          │ · estado dos alertas     │
                                                          └──────────────────────────┘
```

O app gera erros de forma **determinística** (com `error_rate=0.05`, exatamente 1 a cada 20 requisições falha) e já nasce com `http_requests_total{code="500"} 0` inicializado (sem isso, a razão de erro fica **vazia** até o primeiro erro).

---

## ⏱️ A escala 1:60 (e o que ela esconde)

Janelas de 1h, 6h, 3 dias e um SLO de 30 dias não cabem numa aula. O Prometheus do lab carrega as **mesmas regras** de [`rules-production/slo-rules.yml`](rules-production/slo-rules.yml), com **só o relógio comprimido 60 vezes**:

| Produção | 5m | 30m | 1h | 2h | 6h | 1d | 3d | **30d** (SLO) | scrape 1m* | `for: 2m` |
|---|---|---|---|---|---|---|---|---|---|---|
| **Lab** | 5s | 30s | 1m | 2m | 6m | 24m | 72m | **12h** | scrape 1s | `for: 30s` ⚠️ |

\* em produção o scrape costuma ser 15-30s; o importante é ter amostras suficientes na menor janela.

**O que não muda:** o SLO (99,9%), o budget (0,001) e os limiares (14,4 / 6 / 3 / 1). Burn rate é uma razão **sem unidade de tempo** ("quantas vezes mais rápido que o sustentável"): se tudo acontece 60x mais rápido, 14,4x por 1 "hora" continua gastando 2% do budget.

**O que a escala esconde (seja crítico):**
1. **`for` não foi escalado de verdade.** 2m/60 = 2s, curto demais para você ver o estado `pending`. Usamos 30s (Fast/Medium), 1m (Slow), 3m (VerySlow).
2. **Prometheus "jovem".** Com a stack recém-criada, as janelas de 6m/24m/72m/12h só têm os minutos que existem. Elas se comportam como janelas curtas e **todos** os alertas disparam num incidente. Em produção, com dias de histórico, só o Fast (e depois o Medium) dispararia. Deixe o lab rodando ~15 min antes do chaos para ver a diferença.
3. **Budget muito sensível no começo.** Pelo mesmo motivo, 1 min de 5% de erro com 10 min de histórico derruba o "budget restante" para valores muito negativos.
4. **Janelas de 5s são ruidosas:** 150 requisições por janela. Um único erro já é 0,67% (6,7x). É por isso que a janela curta nunca alerta **sozinha**: ela só confirma a longa.
5. Os nomes das séries usam a janela **real do lab** (`ratio_rate1m`), não a de produção, para ninguém se enganar ao ler o gráfico.

---

## ▶️ Como rodar

```bash
cd labs/slo-end-to-end
docker compose up -d --build --wait
# Grafana: http://localhost:3170/d/slo-checkout   Prometheus: http://localhost:9170/alerts
```

Controle o "incidente":

```bash
curl -s localhost:9171/chaos                                   # estado atual
curl -s 'localhost:9171/chaos?error_rate=0.05'                 # 5% de 5xx  -> burn 50x
curl -s 'localhost:9171/chaos?latency_ms=400&latency_ratio=0.3' # 30% das requisições +400ms
curl -s 'localhost:9171/chaos?reset=1'                          # volta ao normal (0,05% de erro, sem latência extra)
```

---

## 🔍 Passo a passo

### 1. O SLI "na mão"

```promql
sum by (code) (rate(http_requests_total{job="checkout"}[1m]))
```

**Resultado esperado:** `code="200"` ≈ **30 req/s**, `code="500"` ≈ **0,015 req/s** (0,05% de baseline = 1 erro a cada ~67s, então numa janela de 1m às vezes aparece **0**).

```promql
  sum(rate(http_requests_total{job="checkout",code=~"5.."}[1m]))
/
  sum(rate(http_requests_total{job="checkout"}[1m]))
```

**Resultado esperado:** oscila entre **0** e ~**0.0006** (média **0.0005**). É o SLI "ruim" (razão de erro). O SLI "bom" é `1 - isso` ≈ 99,95%. Troque `[1m]` por `[5m]` e veja o número estabilizar: janela maior = menos ruído, reação mais lenta.

### 2. As recording rules

```promql
{__name__=~"job:slo_errors_per_request:ratio_rate.*"}
```

Uma série por janela. Por que recording rules? (1) o alerta avalia **6 janelas a cada ciclo**; `rate[3d]` sobre milhões de amostras toda hora seria caro; (2) o dashboard e o alerta usam **exatamente** o mesmo número; (3) o nome (`nível:métrica:operações`) documenta o que é.

### 3. Burn rate

```promql
job:slo_errors_per_request:ratio_rate1m / 0.001
```

**Resultado esperado:** em repouso oscila entre 0 e ~1,1 (média **0,5**, metade do sustentável). Com `error_rate=0.05`: sobe para ≈ **50**.

### 4. Error budget restante

```promql
job:slo_error_budget_remaining:ratio
```

**Resultado esperado:** converge para ≈ **0,5** em repouso depois de alguns minutos (a baseline de 0,05% gasta metade). Depois de um chaos, cai (e pode ficar negativo: SLO violado na janela).

### 5. O incidente

```bash
./solutions/04-fast-burn.sh
# t+0s   SLOErrorBudgetBurnFast=inactive  (burn rate 1m ≈ 0.6x)
# t+18s  SLOErrorBudgetBurnFast=pending   (burn rate 1m ≈ 14.7x)
# t+47s  SLOErrorBudgetBurnFast=firing    (burn rate 1m ≈ 39.0x)
```

- **~18s até pending:** a janela de 1m (≈1h) ainda mistura tráfego bom; precisa que ~29% dela seja "ruim" (0,0144 / 0,05).
- **+30s até firing:** o `for`.
- Depois do `reset`, o Fast **some em segundos**, porque a janela de 5s volta ao normal, mesmo com a de 1m ainda alta. Isso é exatamente o que a janela curta faz.

No Grafana, o painel **Burn rate por janela** mostra as 6 curvas subindo em velocidades diferentes (as janelas curtas reagem primeiro) e o **Estado dos alertas** mostra as faixas amarelas (pending) virando vermelhas (firing).

### 6. SLO de latência

```promql
# fração de requisições em até 300ms (bucket cumulativo le="0.3" / total)
  sum(rate(http_request_duration_seconds_bucket{job="checkout",le="0.3"}[1m]))
/
  sum(rate(http_request_duration_seconds_count{job="checkout"}[1m]))

# o mesmo com histogram_fraction (Prometheus 3.x aceita classic)
histogram_fraction(0, 0.3, sum by (le) (rate(http_request_duration_seconds_bucket{job="checkout"}[1m])))
```

**Resultado esperado:** ≈ **1** (a latência base tem mediana 50ms e p99 ≈ 160-190ms). Com `latency_ms=400&latency_ratio=0.3` → ≈ **0,7** → 30% de lentas = burn **30x** do budget de 1% → `SLOLatencyBudgetBurnFast` dispara.

---

## 🏭 Casos reais

### Caso 1: as regras de produção (este lab)

[`rules-production/slo-rules.yml`](rules-production/slo-rules.yml) é o que você colocaria no Prometheus de verdade: SLIs em 7 janelas, budget em 30d (grupo com `interval: 5m`, porque `rate[30d]` é caro), os 4 alertas de disponibilidade e o de latência. Testado com [`promtool test rules`](rules-production/slo-rules.test.yml) simulando dias de tráfego em milissegundos.

### Caso 2: Sloth (gera as regras a partir de uma spec)

Ninguém escreve 30 recording rules na mão para cada serviço. O [Sloth](https://sloth.dev) gera tudo (SLIs, budget, alertas multi-janela) a partir disto:

```yaml
version: "prometheus/v1"
service: "checkout"
labels:
  owner: "payments-team"
slos:
  - name: "requests-availability"
    objective: 99.9
    description: "99,9% das requisições do checkout sem 5xx."
    sli:
      events:
        error_query: sum(rate(http_requests_total{job="checkout",code=~"5.."}[{{.window}}]))
        total_query: sum(rate(http_requests_total{job="checkout"}[{{.window}}]))
    alerting:
      name: CheckoutHighErrorRate
      labels:
        category: "availability"
      annotations:
        summary: "Checkout está queimando error budget rápido demais"
      page_alert:
        labels:
          severity: page
      ticket_alert:
        labels:
          severity: ticket
```

`sloth generate -i checkout.yml` produz `slo:sli_error:ratio_rate5m`, `...ratio_rate30m`, ..., `slo:error_budget:ratio` e os alertas page/ticket com as mesmas janelas do Workbook.

### Caso 3: Pyrra (CRD no Kubernetes)

```yaml
apiVersion: pyrra.dev/v1alpha1
kind: ServiceLevelObjective
metadata:
  name: checkout-availability
  namespace: monitoring
  labels:
    prometheus: k8s
    role: alert-rules
spec:
  target: "99.9"
  window: 4w
  indicator:
    ratio:
      errors:
        metric: http_requests_total{job="checkout",code=~"5.."}
      total:
        metric: http_requests_total{job="checkout"}
```

O operador do Pyrra transforma isso num `PrometheusRule` (regras + alertas multi-burn-rate) e dá uma UI de budget.

### Caso 4: roteamento page × ticket no Alertmanager

```yaml
route:
  receiver: slack-sre                 # padrão: ticket/aviso
  group_by: [alertname, job, slo]
  routes:
    - matchers: [severity="page"]     # Fast/Medium burn: acorda alguém
      receiver: pagerduty-sre
      group_wait: 10s
    - matchers: [severity="ticket"]   # Slow/VerySlow: vira ticket, sem acordar ninguém
      receiver: slack-sre
      repeat_interval: 24h
inhibit_rules:                        # se o Fast já está pagando, não precisa do Medium do mesmo job
  - source_matchers: [alertname="SLOErrorBudgetBurnFast"]
    target_matchers: [alertname="SLOErrorBudgetBurnMedium"]
    equal: [job]
receivers:
  - name: pagerduty-sre
    pagerduty_configs:
      - routing_key: <sua-integration-key>
  - name: slack-sre
    slack_configs:
      - api_url: https://hooks.slack.com/services/T000/B000/XXXX
        channel: "#sre-slo"
```

---

## 🧪 Exercícios

| # | Exercício | Como verificar |
|---|---|---|
| 01 | [Recording rules de SLI](exercises/01-sli-recording-rules/) | `promtool test rules` |
| 02 | [Alerta multi-window multi-burn-rate](exercises/02-burn-rate-alert/) | `promtool test rules` |
| 03 | [Budget restante (fração e minutos)](exercises/03-error-budget/) | `promtool test rules` |
| 04 | [Provocar um fast burn (pending → firing)](exercises/04-fast-burn-chaos/) | `/api/v1/alerts` + Grafana |
| 05 | [SLO de latência (bucket ratio e `histogram_fraction`)](exercises/05-latency-slo/) | `promtool test rules` + chaos de latência |
| 06 | [Teste unitário dos alertas de burn rate](exercises/06-unit-tests-alertas/) | `promtool test rules` |

Soluções em [`solutions/`](solutions/). O [`test.sh`](test.sh) roda os testes unitários de todas as soluções, sobe a stack, injeta chaos, confere que o `SLOErrorBudgetBurnFast` e o `SLOLatencyBudgetBurnFast` chegam a **firing** via `/api/v1/alerts`, que o fast burn **resolve** após o reset, e que o Grafana tem o datasource e o dashboard e consegue consultar os dados.

---

## ⚠️ Pegadinhas

1. **SLO de 100% não existe.** Budget zero = qualquer deploy é violação. E o usuário não percebe a diferença entre 99,99% e 100% (a rede dele falha mais que isso).
2. **SLA ≠ SLO.** SLA tem consequência contratual e é mais frouxo. O SLO é interno e mais apertado, para você agir antes do SLA.
3. **Série de erro inexistente.** Se `http_requests_total{code="500"}` só aparece no primeiro erro, a razão fica **vazia** (não 0) e o alerta nunca avalia. Inicialize os labels com 0 (o app deste lab faz `WithLabelValues("…","500")` no início).
4. **Média de razões ≠ razão de somas.** `avg_over_time(ratio_rate5m[30d])` dá peso igual à madrugada e ao pico. Use `sum(rate(bad[30d])) / sum(rate(total[30d]))`.
5. **4xx conta?** Normalmente não (erro do cliente). Mas `429` por *rate limiting* seu talvez conte. Decida e documente.
6. **Sem tráfego = sem SLI.** Serviço fora do ar que não recebe requisição nenhuma tem razão de erro **vazia**, e os alertas de burn rate ficam quietos. Complemente com `absent()`/`up == 0` ou SLI via blackbox (sondas sintéticas).
7. **Bucket no limiar.** O SLO de latência por razão de buckets só é exato se existir `le` **exatamente** no limiar (aqui, `0.3`). Mudou o SLO para 250ms e não tem bucket? Estimativa (ou native histograms).
8. **Percentil não é SLI.** `histogram_quantile(0.99, …) < 0.3` responde sim/não por janela; não conta eventos e não gera budget.
9. **Janela curta sozinha = barulho; janela longa sozinha = alerta que não para.** Por isso o `and` das duas.
10. **Burn rates dependem do período do SLO.** 14,4 vem de "2% de 30 dias em 1h". Para um SLO de 7 dias, recalcule (`0,02 × 168h / 1h = 3,36`).

---

## 🎓 Na prova PCA

O que costuma cair:
- Definições: **SLI** (medida), **SLO** (meta interna), **SLA** (contrato com consequência), **error budget** (`1 - SLO`).
- Cálculo de budget em tempo: 99,9% em 30d ≈ **43 min**; 99,99% ≈ **4,3 min**; 99% ≈ **7,2 h**.
- SLI de disponibilidade = razão de **counters** com `rate` (`sum(rate(errors)) / sum(rate(total))`); SLI de latência = razão de **buckets** de histograma (ou `histogram_fraction`), **não** summary (summary não agrega).
- Recording rules para janelas longas; convenção `level:metric:operations`.
- `for` + estados **inactive → pending → firing**; série sintética `ALERTS`; `promtool test rules`.

**1.** What is the error budget for an SLO of 99.9% availability over 30 days, expressed as time of total unavailability?
- A) ~4.3 minutes  B) ~43 minutes  C) ~7.2 hours  D) ~3 days

<details><summary>Resposta</summary>

**B.** 30d = 43.200 min; 0,1% disso = **43,2 min**. 99,99% → 4,3 min (A); 99% → 7,2 h (C).
</details>

**2.** Which statement best describes the relationship between SLI, SLO and SLA?
- A) The SLA is an internal target; the SLO is a legal contract
- B) The SLI is a measurement; the SLO is a target for that measurement; the SLA is an agreement with consequences, usually looser than the SLO
- C) SLO and SLA are synonyms
- D) The SLI is the percentage of the error budget that remains

<details><summary>Resposta</summary>

**B.** SLI mede, SLO é a meta (interna), SLA é o contrato (externo, com multa/crédito) e deve ser mais frouxo que o SLO para dar margem de reação.
</details>

**3.** Your SLO is 99.9%. Over the last hour, 1.44% of requests failed. What is the burn rate?
- A) 1.44  B) 14.4  C) 0.144  D) 144

<details><summary>Resposta</summary>

**B.** Burn rate = razão de erro / budget = 0,0144 / 0,001 = **14,4**. Nesse ritmo, 2% do budget de 30 dias vai embora em 1 hora e o budget inteiro em ~2 dias.
</details>

**4.** Why do multi-window burn-rate alerts combine a long window (1h) with a short window (5m)?
- A) The short window makes the alert fire faster
- B) The long window ensures the problem is significant; the short window ensures it is still happening, so the alert resets quickly after recovery
- C) Prometheus cannot evaluate windows longer than 1h
- D) To reduce the cardinality of the alert

<details><summary>Resposta</summary>

**B.** Só com 1h, o alerta continuaria firing até ~1h depois do conserto. Só com 5m, qualquer pico de segundos acordaria alguém. O `and` das duas dá precisão **e** reset rápido.
</details>

**5.** Which PromQL expression is a correct availability SLI (ratio of failed requests) across all instances of a job?
- A) `avg(rate(http_requests_total{code=~"5.."}[5m]) / rate(http_requests_total[5m]))`
- B) `sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))`
- C) `rate(sum(http_requests_total{code=~"5.."})[5m]) / rate(sum(http_requests_total)[5m])`
- D) `sum(http_requests_total{code=~"5.."}) / sum(http_requests_total)`

<details><summary>Resposta</summary>

**B.** Rate primeiro, soma depois, e **razão das somas**. Em A o `/` casa série a série **inclusive pelo label `code`**, então cada resultado vira `rate(500)/rate(500) = 1` (e ainda seria média de razões); C faz `rate(sum)` (quebra com resets); D usa os counters crus desde o início do processo.
</details>

**6.** You want a latency SLI "requests served in under 300ms". Which instrumentation makes this easy and aggregatable across instances?
- A) A summary with quantile 0.99
- B) A histogram with a bucket boundary at 0.3 seconds
- C) A gauge with the last request latency
- D) A counter of total request time

<details><summary>Resposta</summary>

**B.** `rate(x_bucket{le="0.3"}[w]) / rate(x_count[w])` dá a fração de requisições rápidas, e buckets **somam** entre instâncias. Quantis de summary não podem ser agregados; gauge perde eventos; counter de tempo total só dá média.
</details>

**7.** An alert has `for: 2m`. Its expression becomes true at 10:00:00 and stays true. Rules are evaluated every 30s. What is the alert state at 10:01:00?
- A) inactive  B) pending  C) firing  D) resolved

<details><summary>Resposta</summary>

**B.** Enquanto a expressão é verdadeira há menos que o `for`, o alerta fica **pending** (aparece em `ALERTS{alertstate="pending"}`, não é enviado ao Alertmanager). Vira **firing** a partir de ~10:02:00.
</details>

---

## 📝 Cola rápida

- SLI = bons/válidos (ou ruins/válidos) · SLO = meta numa janela · SLA = contrato · budget = `1 - SLO`.
- 99% → 7,2h/30d · 99,9% → 43,2 min · 99,95% → 21,6 min · 99,99% → 4,3 min.
- Razão de erro: `sum by (job)(rate(bad[w])) / sum by (job)(rate(total[w]))` em recording rules `job:slo_errors_per_request:ratio_rate<w>`.
- Burn rate = razão / budget. Gasto = burn × janela / período. Tempo até zerar = período / burn.
- Workbook (30d): **14,4x 1h&5m page** · **6x 6h&30m page** · **3x 1d&2h ticket** · **1x 3d&6h ticket**. Janela curta = longa/12.
- Latência: `rate(x_bucket{le="0.3"}) / rate(x_count)` ou `histogram_fraction(0, 0.3, ...)`. Precisa de bucket no limiar.
- Estados: inactive → pending (`for`) → firing. `ALERTS{alertname, alertstate}`. Teste com `promtool test rules`.
- Ferramentas que geram tudo isso: Sloth, Pyrra, OpenSLO, Grafana SLO.

## 📚 Referências

- Google SRE Workbook, cap. 5 *Alerting on SLOs*: https://sre.google/workbook/alerting-on-slos/
- Google SRE Book, cap. 4 *Service Level Objectives*: https://sre.google/sre-book/service-level-objectives/
- https://prometheus.io/docs/practices/histograms/#apdex-score (SLI de latência com buckets)
- https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_fraction
- https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/
- https://sloth.dev · https://github.com/pyrra-dev/pyrra
