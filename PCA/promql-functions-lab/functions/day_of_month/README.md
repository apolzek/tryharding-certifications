# `day_of_month()`: que dia do mês é (em UTC)

> **Em uma frase:** `day_of_month(v)` interpreta cada valor de `v` como um timestamp Unix e devolve o **dia do mês (1 a 31) em UTC**. Sem argumento, usa o instante da avaliação (`vector(time())`) → "hoje".

| | |
|---|---|
| **Assinatura** | `day_of_month(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (hoje) ou um **gauge cujo valor é timestamp em segundos** (`*_timestamp_seconds`, `probe_ssl_earliest_cert_expiry`...) |
| **Unidade do resultado** | número inteiro 1-31 |
| **Dashboard** | http://localhost:3300/d/fn-day_of_month |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a folhinha de parede... pendurada em Londres

`day_of_month()` é a **folhinha** (aquele calendário de arrancar uma folha por dia): você entrega um instante e ela mostra **o número grande do dia**.

O detalhe: a folhinha do Prometheus está pendurada em **Londres (UTC)**. No Brasil (UTC-3), às **21h** alguém em Londres já arrancou a folha: para o Prometheus **já é amanhã**. Entre 21h00 e 23h59 de Brasília, `day_of_month()` está **1 dia à frente** do seu calendário.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `day_of_month_simulated_clock_timestamp_seconds` | gauge (timestamp) | **relógio acelerado**: 1 dia simulado a cada **10 s**, de 01/jan/2027 a 28/fev/2027 (ciclo de ~10 min) |
| `day_of_month_invoice_due_timestamp_seconds{customer="acme"}` | gauge (timestamp) | vence dia **5** do mês que vem, 12:00 UTC |
| `day_of_month_invoice_due_timestamp_seconds{customer="globex"}` | gauge (timestamp) | vence dia **15** do mês que vem, 12:00 UTC |
| `day_of_month_invoice_due_timestamp_seconds{customer="umbrella"}` | gauge (timestamp) | vence dia **1** do mês que vem, **01:00 UTC** (= 22:00 do último dia do mês em Brasília!) |
| `day_of_month_billing_pending_invoices` | gauge | ~42 faturas pendentes |
| `day_of_month_js_event_timestamp_milliseconds` | gauge (timestamp **em ms**) | vindo de `Date.now()` do JavaScript |

> 💡 **Por que um "relógio acelerado"?** O dia real só muda uma vez a cada 24 h; num gráfico de 15 min `day_of_month()` seria uma linha reta. O relógio acelerado é um gauge cujo valor é um timestamp que anda 8 640× mais rápido, para você ver o calendário "girar".

```bash
curl -s localhost:8088/metrics | grep '^day_of_month_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-day_of_month
```

Em ~5 min o relógio acelerado percorre janeiro inteiro.

---

## 🔍 Queries passo a passo

### 1. Hoje, em UTC e em Brasília

```promql
day_of_month()                                  # = day_of_month(vector(time()))
day_of_month(vector(time() - 3 * 3600))         # Brasília (UTC-3)
```

**O que faz:** sem argumento, usa o instante avaliado. Para ter o dia em Brasília, **desloque o timestamp** antes de extrair o dia.
**Resultado esperado:** duas linhas retas. Em 25/set às 19h (BRT) = 22h UTC, as duas mostram **25**. A partir das **21h BRT** (00h UTC do dia 26), a linha UTC pula para **26** e a de Brasília continua em **25** até a meia-noite local.

> Repare no `vector(...)`: as funções de calendário exigem **instant vector**; `time() - 10800` é escalar.

---

### 2. O relógio acelerado

```promql
day_of_month(day_of_month_simulated_clock_timestamp_seconds)
```

**Resultado esperado:** um dente-de-serra em **degraus**: 1, 2, 3 ... **31** (janeiro, ~5 min), cai para 1, sobe até **28** (fevereiro/2027), cai para 1 e recomeça.

---

### 3. Alerta só nos 5 primeiros dias do mês (fechamento)

```promql
day_of_month_billing_pending_invoices
  and on() day_of_month(day_of_month_simulated_clock_timestamp_seconds) <= 5
```

**O que faz:** o lado direito só tem resultado quando o dia é ≤ 5 (comparação sem `bool` = **filtro**). O `and on()` mantém o lado esquerdo **só** quando o lado direito existe. O `on()` (lista vazia) é obrigatório: os dois lados têm labels diferentes.
**Resultado esperado:** barras em ~**42** durante os dias 1-5 (50 s reais) e **nada** no resto do mês.

Na vida real (relógio de verdade):

```promql
billing_pending_invoices > 0 and on() day_of_month() <= 5
```

---

### 4. Em que dia vence cada fatura?

```promql
day_of_month(day_of_month_invoice_due_timestamp_seconds)               # UTC
day_of_month(day_of_month_invoice_due_timestamp_seconds - 3 * 3600)    # Brasília
```

**Resultado esperado:**

| customer | UTC | Brasília |
|---|---|---|
| acme | **5** | 5 |
| globex | **15** | 15 |
| umbrella | **1** | **30** (último dia de setembro; 31 num mês de 31 dias) |

O `umbrella` vence 01:00 UTC do dia 1, que no Brasil ainda é 22:00 do **último dia do mês anterior**. Um relatório "faturas que vencem no dia 1" feito em UTC erra a data para o cliente brasileiro.

> Aqui não precisa de `vector()`: `x - 3*3600` já é um instant vector (vetor − escalar).

---

### 5. Pegadinha: timestamp em milissegundos

```promql
day_of_month(day_of_month_js_event_timestamp_milliseconds)          # ERRADO
year(day_of_month_js_event_timestamp_milliseconds)                  # ERRADO: denuncia o problema
day_of_month(day_of_month_js_event_timestamp_milliseconds / 1000)   # CERTO
year(day_of_month_js_event_timestamp_milliseconds / 1000)           # CERTO
```

**Resultado esperado:** o dia errado é um número "qualquer" entre 1 e 31 (muda a cada minuto e **às vezes acerta por acaso**, o que torna o bug traiçoeiro). O ano errado entrega tudo: ≈ **58 704**. Os certos mostram hoje (**25**, ou 26 depois das 21h BRT) e **2026**.

---

## 🏭 Casos reais

### 1. Alerta de faturamento só no fechamento do mês

O time financeiro quer ser acordado se o job de faturamento tiver faturas presas, **mas só** nos primeiros 3 dias úteis do mês (quando o fechamento acontece). No resto do mês, o mesmo número é normal (faturas avulsas).

```yaml
- alert: FaturamentoTravadoNoFechamento
  expr: |
    billing_pending_invoices > 100
      and on() day_of_month() <= 3
  for: 30m
  labels: {severity: critical, team: finance}
```

### 2. Janela de manutenção mensal (ex.: "todo dia 1, 00h-06h UTC")

Em vez de silence manual no Alertmanager, a própria regra ignora a janela:

```yaml
- alert: ApiDown
  expr: |
    up{job="api"} == 0
      unless on() (day_of_month() == 1 and on() hour() < 6)
  for: 5m
```

(Alternativa moderna: `time_intervals` + `mute_time_intervals` no Alertmanager, veja abaixo.)

### 3. Log rotation / relatório mensal: "o job do dia 1 rodou?"

```promql
day_of_month(kube_cronjob_status_last_successful_time{cronjob="monthly-report"}) != 1
```

Se o último sucesso do relatório mensal **não** foi num dia 1, algo pulou.

---

## ✅ Quando usar

- Regras/painéis que dependem do **dia do mês**: fechamento, faturamento, janelas mensais.
- Extrair o dia de uma métrica de timestamp (vencimentos, última execução).
- Projeções mensais junto com [`days_in_month()`](../days_in_month/).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| "Há quantos dias foi?" (duração) | `(time() - x) / 86400` com [`time()`](../time/) |
| Silenciar alertas em horários/dias fixos, com fuso | `time_intervals` no **Alertmanager** (aceita `location: America/Sao_Paulo`) |
| Dia da semana | [`day_of_week()`](../day_of_week/) |
| Tamanho do mês | [`days_in_month()`](../days_in_month/) |

## ⚠️ Pegadinhas

1. **UTC sempre.** Brasil: das 21h à meia-noite o Prometheus já está no dia seguinte. Corrija com `day_of_month(vector(time() - 3*3600))`. (O Brasil não tem mais horário de verão; em países que têm, o offset muda duas vezes por ano e o PromQL **não sabe disso**.)
2. **Argumento em segundos.** Timestamps em ms (JavaScript, Java) → `/ 1000`.
3. **Precisa de instant vector:** `day_of_month(time())` dá erro de tipo; use `day_of_month(vector(time()))` ou só `day_of_month()`.
4. **Combinar com outras métricas exige `on()`:** o resultado de `day_of_month()` não tem labels, então `x and day_of_month() <= 5` não casa com nada. Use `and on()`.
5. **Remove o nome da métrica** e mantém os labels: `day_of_month(x{customer="acme"})` → `{customer="acme"}`.

## 🎓 Na prova PCA

- Funções de data/hora (`day_of_month`, `day_of_week`, `hour`, `minute`, `month`, `year`, `days_in_month`, `day_of_year`) recebem **instant vector** (default `vector(time())`) e trabalham em **UTC**.
- Faixa de retorno: `day_of_month` 1-31.
- Padrão de uso com `and on()` para condicionar alertas.
- Silenciar por horário "de verdade" é trabalho do **Alertmanager** (`time_intervals`/`mute_time_intervals`).

**1.** Qual expressão retorna o dia do mês atual?
- A) `day_of_month(time())`
- B) `day_of_month()`
- C) `day_of_month(now())`
- D) `day_of_month[1d]`

<details><summary>Resposta</summary>

**B.** Sem argumento, o padrão é `vector(time())`. A falha porque `time()` é escalar e a função exige instant vector; C não existe; D é sintaxe inválida.
</details>

**2.** São 22h30 de 31/março em São Paulo (UTC-3). Quanto vale `day_of_month()`?
- A) 31
- B) 1
- C) 30
- D) depende do fuso configurado no Prometheus

<details><summary>Resposta</summary>

**B.** 22h30 BRT = 01h30 UTC de 1º de abril. Não existe configuração de fuso no PromQL: é sempre UTC.
</details>

**3.** Qual expressão mantém `billing_pending_invoices` apenas nos dias 1 a 5?
- A) `billing_pending_invoices and day_of_month() <= 5`
- B) `billing_pending_invoices and on() day_of_month() <= 5`
- C) `billing_pending_invoices * (day_of_month() <= 5)`
- D) `billing_pending_invoices if day_of_month() <= 5`

<details><summary>Resposta</summary>

**B.** Sem `on()`, o `and` tenta casar todos os labels e o lado direito (sem labels) nunca casa. C também não casa labels (e sem `bool` o valor seria multiplicado pelo dia). D não existe em PromQL.
</details>

**4.** Uma métrica guarda `Date.now()` do JavaScript. Como extrair o dia do mês?
- A) `day_of_month(x)`
- B) `day_of_month(x / 1000)`
- C) `day_of_month(x * 1000)`
- D) `day_of_month(timestamp(x))`

<details><summary>Resposta</summary>

**B.** `Date.now()` está em milissegundos; o PromQL espera segundos. D daria o dia do **scrape**, não o do evento.
</details>

## 📝 Cola rápida

- `day_of_month(v=vector(time()))` → 1-31, **UTC**.
- Hoje: `day_of_month()`; de uma métrica: `day_of_month(x_timestamp_seconds)`.
- Brasília: `day_of_month(vector(time() - 3*3600))` / `day_of_month(x - 3*3600)`.
- Condicionar alerta: `alerta and on() day_of_month() <= 5`.
- ms → `/1000`; escalar → `vector()`.

## 🔗 Relacionadas

[`days_in_month()`](../days_in_month/) · [`day_of_week()`](../day_of_week/) · [`day_of_year()`](../day_of_year/) · [`hour()`](../hour/) · [`month()`](../month/) · [`time()`](../time/) · [`vector()`](../vector/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#day_of_month
