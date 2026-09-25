# `hour()`: hora do dia (0 a 23, em UTC!)

> **Em uma frase:** `hour(v)` interpreta cada valor de `v` como timestamp Unix e devolve a **hora do dia em UTC** (0-23). Sem argumento, usa o instante avaliado. Brasília = UTC-3: **horário comercial 09h-18h BRT = 12h-21h UTC**.

| | |
|---|---|
| **Assinatura** | `hour(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (agora) ou gauge cujo **valor é timestamp em segundos** |
| **Unidade do resultado** | inteiro 0-23 |
| **Dashboard** | http://localhost:3300/d/fn-hour |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o relógio de Greenwich na parede da redação

Redações de jornal têm vários relógios na parede: "São Paulo", "Nova York", "Londres". O Prometheus só tem **um**: o de **Londres (UTC)**, e não aceita trocar.

`hour()` olha para esse relógio e diz a hora. Quando no Brasil são **19h**, `hour()` diz **22**. Quando no Brasil é **meia-noite**, diz **3**.

Tabela de conversão que vale decorar:

| Brasília (UTC-3) | `hour()` (UTC) |
|---|---|
| 00h | 3 |
| 09h (início do expediente) | **12** |
| 18h (fim do expediente) | **21** |
| 21h | 0 (e já é **o dia seguinte**!) |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `hour_simulated_clock_timestamp_seconds` | gauge (timestamp) | **relógio acelerado**: 1 hora simulada a cada **10 s** → um dia inteiro em 4 min |
| `hour_checkout_errors_ratio` | gauge | taxa de erro ~**8%** o tempo todo (SLO é 5%) |
| `hour_kube_cronjob_status_last_successful_time{cronjob="nightly-backup"}` | gauge (timestamp) | imita o kube-state-metrics: backup noturno às **03:00 UTC** (= meia-noite em Brasília) |

```bash
curl -s localhost:8088/metrics | grep '^hour_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-hour
```

Em 4 min o relógio acelerado dá uma volta completa no dia.

---

## 🔍 Queries passo a passo

### 1. O relógio acelerado (UTC e Brasília)

```promql
hour(hour_simulated_clock_timestamp_seconds)               # UTC
hour(hour_simulated_clock_timestamp_seconds - 3 * 3600)    # Brasília
```

**Resultado esperado:** dois dentes-de-serra 0 → **23**, cada degrau com 10 s. A linha de Brasília é a mesma deslocada: quando a UTC marca 3, Brasília marca 0; quando a UTC volta a 0, Brasília está em 21.

---

### 2. Alerta só em horário comercial

```promql
hour_checkout_errors_ratio > 0.05
  and on() (hour(hour_simulated_clock_timestamp_seconds) >= 12 < 21)
```

**O que faz:**
- `hour(...) >= 12 < 21` é uma **cadeia de filtros**: só sobra o vetor quando a hora UTC está entre 12 e 20 (inclusive), ou seja, 09h00-17h59 BRT.
- `and on()` mantém o erro apenas quando o filtro tem resultado (o `on()` vazio é necessário porque os lados têm labels diferentes).
**Resultado esperado:** a linha (~**8%**) aparece por **90 s** (9 "horas") e some por **150 s** (15 "horas"), a cada volta de 240 s.

---

### 3. Agora: `hour() - 3` vs `hour(vector(time() - 3h))`

```promql
hour()                             # UTC
hour() - 3                         # "Brasília" FRÁGIL
hour(vector(time() - 3 * 3600))    # Brasília CERTO
```

**Resultado esperado:** às 19h BRT: **22**, **19**, **19**. Parece igual... mas entre **00h e 02h UTC** (21h-23h BRT), `hour() - 3` dá **-3, -2, -1**! O jeito certo é **deslocar o timestamp** e só depois extrair a hora. (Alternativa aritmética: `(hour() + 21) % 24`.)

---

### 4. O mesmo alerta com o relógio real

```promql
hour_checkout_errors_ratio > 0.05 and on() hour() >= 12 < 21
```

**Resultado esperado:** entre 12h e 20h59 UTC → **~8%**. Fora disso (por exemplo, às 22h UTC = 19h BRT) → **"No data"**. Esse vazio **é** o comportamento esperado: o alerta está silenciado fora do expediente.

> Repare na precedência: `and` tem prioridade **menor** que as comparações, então `a > 0.05 and on() hour() >= 12 < 21` é lido como `(a > 0.05) and on() ((hour() >= 12) < 21)`.

---

### 5. A que horas rodou o backup noturno?

```promql
hour(hour_kube_cronjob_status_last_successful_time)              # 3 (UTC)
hour(hour_kube_cronjob_status_last_successful_time - 3 * 3600)   # 0 (Brasília)
```

---

## 🏭 Casos reais

### 1. SLO "só conta em horário comercial" (inibição de warning)

O time de checkout acorda para `critical` 24×7, mas `warning` só no expediente de São Paulo:

```yaml
- alert: CheckoutErroAltoExpediente
  expr: |
    sum(rate(http_requests_total{job="checkout",code=~"5.."}[5m]))
      / sum(rate(http_requests_total{job="checkout"}[5m])) > 0.05
    and on() (hour() >= 12 < 21)          # 09-18h BRT
    and on() (day_of_week() > 0 < 6)      # seg-sex (UTC!)
  for: 10m
  labels: {severity: warning}
```

### 2. Backup noturno não rodou (kube-state-metrics)

```yaml
- alert: BackupNoturnoNaoRodou
  expr: time() - kube_cronjob_status_last_successful_time{cronjob="nightly-backup"} > 26 * 3600
  labels: {severity: critical}
- alert: BackupNoturnoForaDeHora       # rodou, mas não na janela 02-05 UTC
  expr: hour(kube_cronjob_status_last_successful_time{cronjob="nightly-backup"}) < 2
        or hour(kube_cronjob_status_last_successful_time{cronjob="nightly-backup"}) > 5
```

### 3. Limites diferentes de dia e de noite (tráfego)

"Menos de 100 req/s entre 08h e 23h de Brasília é anormal" → 11h-02h UTC. Esse intervalo **atravessa a meia-noite UTC**, então `hour() >= 11 < 2` nunca é verdade (nenhum número é ≥ 11 e < 2 ao mesmo tempo). Faça dois ramos com `or`:

```promql
  (sum(rate(http_requests_total[5m])) < 100 and on() (hour() >= 11))
or
  (sum(rate(http_requests_total[5m])) < 100 and on() (hour() < 2))
```

### 4. Jeito recomendado para *notificar* por horário: Alertmanager

```yaml
time_intervals:
  - name: expediente-sp
    time_intervals:
      - weekdays: ['monday:friday']
        times: [{start_time: '09:00', end_time: '18:00'}]
        location: America/Sao_Paulo
```

Com `location`, o Alertmanager entende fuso e horário de verão, coisa que `hour()` **não** entende.

---

## ✅ Quando usar

- Condições que dependem da hora **dentro da expressão**: limites dia/noite, janelas de manutenção fixas.
- Conferir **a que horas** um job rodou (`hour(x_timestamp)`).
- Recording rules de "tráfego em horário de pico".

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só não **notificar** fora do expediente | Alertmanager `active_time_intervals` / `mute_time_intervals` |
| Comparar com o mesmo horário de ontem | `x offset 1d` |
| Minuto / dia da semana | [`minute()`](../minute/), [`day_of_week()`](../day_of_week/) |

## ⚠️ Pegadinhas

1. **UTC sempre.** 09-18h BRT = **12-21h UTC**.
2. **`hour() - 3` fica negativo** entre 00h-02h UTC. Use `hour(vector(time() - 3*3600))` ou `(hour() + 21) % 24`.
3. **Horário de verão:** PromQL não sabe. Um offset fixo erra 1 h em metade do ano nos países com DST.
4. **Intervalos que cruzam a meia-noite UTC** (ex.: 22h-02h) não cabem numa única cadeia `>= 22 < 2`; use `or`.
5. **`and on()`** para combinar com métricas com labels.
6. **Grafana mostra o eixo X no fuso do navegador**, mas `hour()` devolve UTC: num gráfico, às "19:00" do eixo o valor de `hour()` é **22**.

## 🎓 Na prova PCA

- Faixa **0-23**, UTC, default `vector(time())`.
- `hour()` exige **instant vector**: `hour(time())` dá erro de tipo.
- Filtro vs `bool`: `hour() >= 12` filtra; `hour() >= bool 12` devolve 0/1.
- Silenciar por horário: Alertmanager (`time_intervals`), com fuso.

**1.** Qual expressão retorna a hora atual em Brasília (UTC-3) sem nunca dar número negativo?
- A) `hour() - 3`
- B) `hour(vector(time() - 3 * 3600))`
- C) `hour(time() - 10800)`
- D) `hour() offset -3h`

<details><summary>Resposta</summary>

**B.** Desloca o timestamp antes. A fica negativo entre 00-02h UTC; C é erro de tipo (escalar); D é sintaxe inválida para função.
</details>

**2.** Uma regra tem `and on() hour() >= 9 < 18`. Um SRE em São Paulo percebe que o alerta só dispara das 06h às 15h locais. Por quê?
- A) bug do Prometheus
- B) `hour()` é UTC; 9-18 UTC = 6-15 BRT
- C) o servidor está com fuso errado
- D) o `for:` atrasa o alerta em 3 horas

<details><summary>Resposta</summary>

**B.** Para 09-18h BRT use `>= 12 < 21`, ou trate no Alertmanager com `location`.
</details>

**3.** O que retorna `hour() >= bool 12` às 10h UTC?
- A) vazio
- B) 0
- C) 1
- D) 10

<details><summary>Resposta</summary>

**B.** Com `bool` a comparação devolve 0/1 em vez de filtrar. Sem `bool`, o resultado seria vazio.
</details>

**4.** Qual o valor de `hour(vector(0))`?
- A) 0
- B) 21
- C) 1
- D) erro

<details><summary>Resposta</summary>

**A.** Timestamp 0 = 01/01/1970 00:00:00 UTC.
</details>

## 📝 Cola rápida

- `hour()` → 0-23 **UTC**. 09-18h BRT = `hour() >= 12 < 21`.
- Brasília: `hour(vector(time() - 3*3600))` (nunca `hour() - 3`).
- Condição: `alerta and on() (hour() >= 12 < 21)`.
- Intervalo que cruza 0h → dois ramos com `or`.
- Notificar por horário (com fuso/DST) → Alertmanager `time_intervals`.

## 🔗 Relacionadas

[`minute()`](../minute/) · [`day_of_week()`](../day_of_week/) · [`day_of_month()`](../day_of_month/) · [`time()`](../time/) · [`vector()`](../vector/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#hour
