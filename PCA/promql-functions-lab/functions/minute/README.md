# `minute()`: minuto da hora (0 a 59)

> **Em uma frase:** `minute(v)` interpreta cada valor de `v` como timestamp Unix e devolve o **minuto da hora** (0-59, UTC). Sem argumento, usa o instante avaliado. Ótimo para padrões tipo **cron** ("nos minutos 0-2 de cada dezena") e para conferir **em que minuto** um job rodou.

| | |
|---|---|
| **Assinatura** | `minute(v=vector(time()) instant-vector) → instant-vector` |
| **Tipo de métrica** | nenhuma (agora) ou gauge cujo **valor é timestamp em segundos** |
| **Unidade do resultado** | inteiro 0-59 |
| **Dashboard** | http://localhost:3300/d/fn-minute |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o ponteiro grande

`hour()` é o ponteiro pequeno; `minute()` é o **ponteiro grande** do relógio. Ele dá uma volta completa por hora e não liga para dia, mês ou fuso (quase: veja as pegadinhas).

Com o **resto da divisão** (`%`) ele vira um "cron" dentro do PromQL:

| expressão | significa | cron equivalente |
|---|---|---|
| `minute() % 10 < 3` | minutos x0, x1, x2 de cada dezena | `0-2,10-12,20-22,... * * * *` |
| `minute() % 15 == 0` | minutos 0, 15, 30, 45 | `*/15 * * * *` |
| `minute() < 5` | primeiros 5 min de toda hora | `0-4 * * * *` |

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `minute_kube_cronjob_status_last_schedule_time{cronjob="cleanup"}` | gauge (timestamp) | imita o kube-state-metrics: CronJob `*/5 * * * *` → minutos 0, 5, 10, ..., 55 |
| `minute_kube_cronjob_status_last_schedule_time{cronjob="report"}` | gauge (timestamp) | CronJob `2-59/15 * * * *` → minutos **2, 17, 32, 47** |
| `minute_api_errors_ratio` | gauge | taxa de erro ~**1%**, mas ~**20%** nos minutos **x0-x2** de cada dezena (deploy automático) |

```bash
curl -s localhost:8088/metrics | grep '^minute_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-minute
```

Em ~15 min você vê todos os padrões (um ciclo do `report`).

---

## 🔍 Queries passo a passo

### 1. `minute()` ao longo do tempo

```promql
minute()          # rampa 0 → 59
minute() % 10     # posição dentro da dezena: 0 → 9, 0 → 9...
```

**Resultado esperado:** `minute()` sobe 1 por minuto (em degraus) e despenca para 0 na virada da hora. `minute() % 10` é um dente-de-serra 0 → 9 a cada 10 min.

---

### 2. Em que minuto os CronJobs rodaram?

```promql
minute(minute_kube_cronjob_status_last_schedule_time)
```

**Resultado esperado:** `cleanup` = escada **0, 5, 10, ..., 55** (muda a cada 5 min); `report` = **2, 17, 32, 47** (muda a cada 15 min).

---

### 3 e 4. Erro cru vs erro fora da janela de deploy

```promql
minute_api_errors_ratio                                    # cru
minute_api_errors_ratio unless on() (minute() % 10 < 3)    # sem a janela de deploy
```

**O que faz:** `unless` é o "E NÃO": mantém o lado esquerdo **exceto** quando o lado direito tem resultado. O lado direito só existe nos minutos x0, x1, x2.
**Resultado esperado:**
- cru: ~1% com **platôs de ~20%** durando 3 min, a cada 10 min.
- filtrado: a mesma linha de ~1% com **buracos** onde estavam os platôs. Mas repare num **espigão de um único ponto** logo no fim de cada buraco: no instante x3:00 `minute()` já vale 3 (a janela "acabou"), porém a amostra mais recente da métrica foi raspada às x2:5x e ainda diz 20% (o PromQL usa a última amostra dentro do *lookback*). Por isso, na vida real, alargue a janela um pouco (`< 4`) e use `for:` no alerta: um ponto isolado nunca vira alerta com `for: 2m`.

---

### 5. O `report` rodou no minuto certo?

```promql
minute(minute_kube_cronjob_status_last_schedule_time{cronjob="report"}) % 15 == bool 2
```

**Resultado esperado:** **1** (ok). Com `bool`, a comparação devolve 1/0 em vez de filtrar, ótimo para um stat "ok/não ok".

---

### 6. ❌ O que dá errado: `unless` sem `on()`

```promql
minute_api_errors_ratio unless (minute() % 10 < 3)
```

**Resultado esperado:** igual ao **cru** (painel 3), **com** os picos de 20%. Sem `on()`, o `unless` só remove séries do lado esquerdo que tenham um par com **os mesmos labels** no lado direito; como o direito não tem labels, nada é removido. Compare os painéis 4 e 6 lado a lado.

---

## 🏭 Casos reais

### 1. Janela de deploy / manutenção recorrente

Um pipeline faz deploy canário automaticamente nos minutos 0-2 de cada hora; os erros transitórios (conexões cortadas) não devem acordar ninguém:

```yaml
- alert: ApiErroAlto
  expr: |
    (
      sum(rate(http_requests_total{job="api",code=~"5.."}[2m]))
        / sum(rate(http_requests_total{job="api"}[2m]))
    ) > 0.05
    unless on() (minute() < 3)
  for: 5m
```

> Dica: com `for: 5m`, um pico de 3 min já não dispararia. O `unless` é para quando o pico é longo o bastante, ou para dashboards/SLOs que não devem contar a janela.

### 2. CronJob do Kubernetes atrasado (kube-state-metrics)

```yaml
- alert: CronJobAtrasado
  expr: time() - kube_cronjob_status_last_schedule_time{cronjob="report"} > 16 * 60
  labels: {severity: warning}
```

E para conferir que o agendamento mudou sem ninguém avisar:

```promql
minute(kube_cronjob_status_last_schedule_time{cronjob="report"}) % 15 != 2
```

### 3. Scrape "na hora cheia" (jobs pesados)

Exporters caros (ex.: um exporter de banco que roda `SELECT count(*)`) às vezes são consultados só no começo da hora. Painel:

```promql
db_table_rows and on() (minute() < 5)
```

---

## ✅ Quando usar

- Padrões de **cron** dentro de expressões (janelas recorrentes curtas).
- Conferir o **minuto** em que jobs agendados rodaram.
- Combinar com `hour()` para janelas precisas (`hour() == 3 and on() minute() < 30`).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Duração ("há quantos minutos?") | `(time() - x) / 60` |
| Silenciar alertas numa janela | Alertmanager `mute_time_intervals` (aceita `times: 00:00-00:03`) |
| Hora do dia | [`hour()`](../hour/) |

## ⚠️ Pegadinhas

1. **Fuso quase não importa... mas importa às vezes.** Brasília (UTC-3) tem offset de hora cheia: `minute()` é igual em UTC e BRT. Na **Índia (UTC+5:30)**, Nepal (+5:45) ou Terra Nova (-3:30), o minuto local é diferente!
2. **`unless` / `and` precisam de `on()`** para combinar com métricas que têm labels.
3. **Filtro vs `bool`:** `minute() < 3` some fora da janela; `minute() < bool 3` devolve 0/1.
4. **Borda da janela + lookback:** logo depois que a janela fecha, o valor avaliado ainda é o da última amostra (raspada antes). Dê uma folga de 1 min e use `for:`.
5. **Resolução do gráfico:** com step de 1 min ou mais, janelas de 1-2 min podem "pular" pontos.
6. **Timestamps em ms** → `/ 1000`.

## 🎓 Na prova PCA

- Faixa **0-59**, UTC, default `vector(time())`.
- Operadores de conjunto: `and`, `or`, `unless` (e o `on()` para ignorar labels).
- Operador `%` (módulo) em PromQL.

**1.** Qual expressão é verdadeira (não vazia) somente nos minutos 0, 15, 30 e 45?
- A) `minute() / 15 == 0`
- B) `minute() % 15 == 0`
- C) `minute() == 15`
- D) `minute(15)`

<details><summary>Resposta</summary>

**B.** Resto da divisão por 15 igual a zero. A só vale no minuto 0; C só no 15; D é erro de tipo: `15` é escalar e a função exige instant vector (e mesmo `minute(vector(15))` seria o timestamp 1970-01-01 00:00:15 → minuto 0).
</details>

**2.** O que faz `errors_ratio unless on() (minute() < 3)`?
- A) mostra o erro só nos minutos 0-2
- B) mostra o erro exceto nos minutos 0-2
- C) retorna 0 nos minutos 0-2
- D) erro de sintaxe

<details><summary>Resposta</summary>

**B.** `unless` remove do lado esquerdo os elementos que têm correspondência no direito; com `on()` a correspondência ignora labels.
</details>

**3.** Em que fuso `minute()` pode dar um valor diferente do minuto do relógio local?
- A) Brasília (UTC-3)
- B) Nova York (UTC-5)
- C) Índia (UTC+5:30)
- D) Londres (UTC+0)

<details><summary>Resposta</summary>

**C.** Offsets com meia hora (ou 45 min) mudam o minuto. Offsets de hora cheia não.
</details>

## 📝 Cola rápida

- `minute()` → 0-59 (UTC; igual ao local em fusos de hora cheia).
- "Cron": `minute() % N == k`, `minute() % 10 < 3`.
- Ignorar janela: `x unless on() (minute() < 3)`.
- 1/0 em vez de filtro: `bool`.

## 🔗 Relacionadas

[`hour()`](../hour/) · [`time()`](../time/) · [`day_of_week()`](../day_of_week/) · [`timestamp()`](../timestamp/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#minute
