# `day_of_week()`: dia da semana (0 = domingo, em UTC)

> **Em uma frase:** `day_of_week(v)` interpreta cada valor de `v` como timestamp Unix e devolve o **dia da semana em UTC**: **0 = domingo**, 1 = segunda ... **6 = sábado**. Sem argumento, usa o instante da avaliação ("hoje").

| | |
|---|---|
| **Assinatura** | `day_of_week(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (hoje) ou gauge cujo **valor é timestamp em segundos** |
| **Unidade do resultado** | inteiro 0-6 (0 = domingo) |
| **Dashboard** | http://localhost:3300/d/fn-day_of_week |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a agenda do plantonista

Pense na **agenda de plantão** do time: de segunda a sexta tem gente acordada para atender alerta; no fim de semana, só emergência. `day_of_week()` é a pessoa que olha a agenda e responde **"hoje é sexta (5)"**.

Duas manias dessa pessoa:
1. Ela conta a partir de **domingo = 0** (não segunda = 1, como no ISO 8601).
2. Ela mora em **Londres (UTC)**: na sexta às 21h de Brasília, para ela **já é sábado**.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `day_of_week_simulated_clock_timestamp_seconds` | gauge (timestamp) | **relógio acelerado**: 1 dia simulado a cada **20 s**, começando no domingo 03/jan/2027 → uma semana a cada 140 s |
| `day_of_week_checkout_errors_ratio` | gauge | taxa de erro ~**8%** (acima do SLO de 5%) o tempo todo |
| `day_of_week_kube_cronjob_status_last_successful_time{cronjob="weekly-report"}` | gauge (timestamp) | imita o kube-state-metrics: o CronJob roda toda **segunda 06:00 UTC** |

```bash
curl -s localhost:8088/metrics | grep '^day_of_week_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-day_of_week
```

Em ~2,5 min o relógio acelerado dá uma volta completa na semana.

---

## 🔍 Queries passo a passo

### 1. O relógio acelerado: uma escada 0 → 6

```promql
day_of_week(day_of_week_simulated_clock_timestamp_seconds)
```

**Resultado esperado:** escada com 7 degraus de 20 s: **0 (Dom)**, 1, 2, 3, 4, 5, **6 (Sáb)**, e volta para 0. O painel usa *value mappings* do Grafana para mostrar "Dom/Seg/..." no tooltip.

---

### 2. Alerta só em dia útil

```promql
day_of_week_checkout_errors_ratio > 0.05
  and on() (day_of_week(day_of_week_simulated_clock_timestamp_seconds) > 0 < 6)
```

**O que faz:**
- `day_of_week(...) > 0 < 6` é uma **cadeia de filtros**: `> 0` tira o domingo, `< 6` tira o sábado. Sobra um vetor só de segunda a sexta.
- `and on()` mantém o erro apenas quando esse vetor existe.
**Resultado esperado:** linha em ~**0,08** (8%) por **100 s** (seg-sex) e um **buraco de 40 s** (sáb + dom) a cada volta de 140 s.

Com o relógio real, a regra seria:

```promql
checkout_errors_ratio > 0.05 and on() (day_of_week() > 0 < 6)
```

---

### 3. Hoje: UTC vs Brasília

```promql
day_of_week()                               # UTC
day_of_week(vector(time() - 3 * 3600))      # Brasília
```

**Resultado esperado:** numa sexta às 19h BRT: os dois dizem **Sexta** (5). Às **21h de sexta** em Brasília, UTC já é **Sábado** (6) → o filtro "dia útil" desliga **3 h antes** do seu fim de semana começar. E, simetricamente, volta a ligar domingo às 21h BRT.

---

### 4. Em que dia rodou o relatório semanal?

```promql
day_of_week(day_of_week_kube_cronjob_status_last_successful_time)
```

**Resultado esperado:** **1 (Segunda)**. Se aparecer outro número, o CronJob rodou fora do dia (ou alguém disparou manualmente).

---

### 5. ❌ O que dá errado: esquecer o `on()`

```promql
day_of_week_checkout_errors_ratio > 0.05 and (day_of_week() >= 0 <= 6)
```

**O que acontece:** o lado direito é verdadeiro **todos** os dias (0 a 6), e mesmo assim o resultado é **vazio** ("No data"). O `and` sem `on()` exige que os dois lados tenham **exatamente os mesmos labels**: o esquerdo tem `{env, instance, job}`, o direito não tem nenhum. A regra nunca dispara e ninguém percebe, porque "vazio" parece "tudo bem".

---

## 🏭 Casos reais

### 1. Alerta "não-crítico" só em horário comercial de dia útil

O SRE do e-commerce não quer ser acordado por um alerta de **warning** no fim de semana, mas quer vê-lo na segunda de manhã. Brasília 09-18h = 12-21h UTC:

```yaml
- alert: CheckoutErroAlto
  expr: |
    (
      sum(rate(http_requests_total{job="checkout",code=~"5.."}[5m]))
        / sum(rate(http_requests_total{job="checkout"}[5m]))
    ) > 0.05
    and on() (day_of_week() > 0 < 6)
    and on() (hour() >= 12 < 21)
  for: 10m
  labels: {severity: warning}
```

### 2. Tráfego esperado diferente no fim de semana

Um banco tem muito menos tráfego no fim de semana. Um alerta de "tráfego baixo demais" (sintoma de queda silenciosa) usa limites diferentes:

```yaml
- alert: InternetBankingTrafegoBaixo
  expr: |
    (sum(rate(http_requests_total{job="internet-banking"}[10m])) < 50
       and on() (day_of_week() > 0 < 6))
    or
    (sum(rate(http_requests_total{job="internet-banking"}[10m])) < 10
       and on() (day_of_week() == 0 or day_of_week() == 6))
  for: 15m
  labels: {severity: critical}
```

### 3. Jeito recomendado para silenciar: Alertmanager

A própria doc do Prometheus recomenda tratar **quando notificar** no Alertmanager, que tem **fuso horário**:

```yaml
# alertmanager.yml
time_intervals:
  - name: horario-comercial
    time_intervals:
      - weekdays: ['monday:friday']
        times: [{start_time: '09:00', end_time: '18:00'}]
        location: 'America/Sao_Paulo'
route:
  routes:
    - matchers: [severity="warning"]
      receiver: slack
      active_time_intervals: [horario-comercial]
```

Use `day_of_week()` no PromQL quando o **dado** (e não a notificação) depende do dia: limites diferentes, recording rules de "tráfego de dia útil", dashboards.

---

## ✅ Quando usar

- Limites/condições diferentes por **dia da semana** dentro da própria expressão.
- Recording rules e dashboards de padrão semanal ("pico de segunda").
- Conferir em que dia um job agendado rodou.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só **não notificar** em fins de semana | `time_intervals` / `mute_time_intervals` no Alertmanager (tem fuso) |
| Comparar com a semana passada | `x offset 1w` |
| Dia do mês / ano | [`day_of_month()`](../day_of_month/), [`day_of_year()`](../day_of_year/) |

## ⚠️ Pegadinhas

1. **0 = domingo**, não segunda. `day_of_week() == 7` nunca é verdade.
2. **UTC:** "sexta 21h BRT" já é sábado. Corrija com `day_of_week(vector(time() - 3*3600))`.
3. **`> 0 < 6` é filtro, não booleano.** Se quiser 1/0, use `bool`: `(day_of_week() > bool 0) * (day_of_week() < bool 6)`.
4. **`and on()`** é obrigatório para combinar com métricas que têm labels.
5. **Timestamps em ms** → divida por 1000 antes.

## 🎓 Na prova PCA

- Faixa: **0-6, 0 = domingo**, em UTC. Default: `vector(time())`.
- Cadeia de comparação como filtro (`> 0 < 6`) e `bool`.
- Onde cada coisa mora: condição no **PromQL** vs horário de notificação no **Alertmanager** (`time_intervals`).

**1.** Qual valor `day_of_week()` retorna num sábado (UTC)?
- A) 0
- B) 5
- C) 6
- D) 7

<details><summary>Resposta</summary>

**C.** 0 = domingo, 6 = sábado.
</details>

**2.** Qual expressão retorna um resultado somente de segunda a sexta (UTC)?
- A) `day_of_week() > 0 < 6`
- B) `day_of_week() >= 1 <= 7`
- C) `day_of_week() between 1 and 5`
- D) `day_of_week(1, 5)`

<details><summary>Resposta</summary>

**A.** 1..5 = segunda..sexta. B incluiria sábado (6); C e D não existem.
</details>

**3.** O time quer receber alertas `warning` só em horário comercial de São Paulo. Qual a abordagem mais robusta?
- A) `and on() hour() >= 9 < 18` em cada regra
- B) `active_time_intervals` com `location: America/Sao_Paulo` no Alertmanager
- C) mudar o fuso do servidor do Prometheus para America/Sao_Paulo
- D) `--query.timezone=America/Sao_Paulo`

<details><summary>Resposta</summary>

**B.** O Alertmanager entende fusos (e horário de verão). A usa horas em UTC (seria 12-21); C não muda nada no PromQL (sempre UTC); D não existe.
</details>

**4.** O que retorna `http_requests_total and day_of_week() == 1` numa segunda-feira?
- A) as séries de `http_requests_total`
- B) vazio
- C) 1
- D) erro de sintaxe

<details><summary>Resposta</summary>

**B.** Sem `on()`, o `and` compara todos os labels; o lado direito não tem labels, então nada casa.
</details>

## 📝 Cola rápida

- `day_of_week()` → **0 = domingo ... 6 = sábado**, UTC.
- Dia útil: `and on() (day_of_week() > 0 < 6)`.
- Brasília: `day_of_week(vector(time() - 3*3600))`. Sexta 21h BRT = sábado UTC.
- Notificação por horário → Alertmanager `time_intervals` (com `location`).

## 🔗 Relacionadas

[`hour()`](../hour/) · [`day_of_month()`](../day_of_month/) · [`day_of_year()`](../day_of_year/) · [`time()`](../time/) · [`vector()`](../vector/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#day_of_week
