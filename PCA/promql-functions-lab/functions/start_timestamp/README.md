# `start_timestamp()`: quando o counter nasceu

> **Em uma frase:** `start_timestamp(v)` devolve o **start timestamp** (também chamado *created timestamp*) de cada amostra: o instante, em segundos Unix UTC, em que o counter **começou do zero**. `time() - start_timestamp(x)` = **uptime** de quem expõe o counter.

| | |
|---|---|
| **Assinatura** | `start_timestamp(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Counter (e histogram/summary) com created timestamp exposto · ⚠️ Gauge/counter sem created → **0** |
| **Pré-requisito** | feature flag `use-start-timestamps` (+ `st-storage` para guardar) **e** alvo expondo o created (protobuf ou OpenMetrics `_created`) |
| **Unidade do resultado** | segundos Unix (UTC) |
| **Dashboard** | http://localhost:3300/d/fn-start_timestamp |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o hodômetro e a data de fabricação

Na lição de [`rate()`](../rate/), o counter é o **hodômetro** do carro. `start_timestamp()` é a **plaquinha com a data de fabricação** colada na porta: diz **desde quando** aquele hodômetro está contando.

- Trocou de carro (restart do processo)? Hodômetro volta a 0 **e** a data de fabricação muda.
- `time() - start_timestamp(x)` = **idade do carro** (uptime).
- `x / (time() - start_timestamp(x))` = **velocidade média desde que saiu da fábrica** (km totais ÷ idade).

Carros antigos (exporters sem created timestamp) **não têm plaquinha**: o Prometheus responde **0** (1º de janeiro de 1970).

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `start_timestamp_worker_jobs_total{pod="worker-a"}` | counter + created | 3 jobs/s; **reinicia a cada 4 min** (topo ≈ 720) |
| `start_timestamp_worker_jobs_total{pod="worker-b"}` | counter + created | 1,5 jobs/s; **reinicia a cada 7 min** (topo ≈ 630) |
| `start_timestamp_process_start_time_seconds{pod}` | gauge | imita `process_start_time_seconds` dos mesmos workers (o "jeito antigo") |
| `start_timestamp_http_requests_total{route}` | counter (client_golang) | created **automático** = quando o gerador subiu |
| `start_timestamp_legacy_jobs_total{pod="legacy-0"}` | counter **sem** created | exporter antigo → `start_timestamp()` = **0** |
| `start_timestamp_queue_depth` | gauge | gauges não têm created → **0** |

O created timestamp **não aparece** no texto que o `curl` mostra: ele viaja no formato **protobuf**, que é o que o Prometheus negocia neste lab. No `curl` você só vê o valor:

```bash
curl -s localhost:8088/metrics | grep '^start_timestamp_worker'
# start_timestamp_worker_jobs_total{pod="worker-a"} 654
# start_timestamp_worker_jobs_total{pod="worker-b"} 327
```

Em OpenMetrics texto ele pode aparecer como uma linha extra `start_timestamp_worker_jobs_total_created{pod="worker-a"} 1.79037432e+09` (o gerador deixa isso desligado).

> ℹ️ Neste lab o Prometheus roda com `--enable-feature=st-storage,use-start-timestamps` e raspa em **protobuf** (por causa dos native histograms), por onde o created timestamp chega.

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-start_timestamp
```

Em ~8 min você vê pelo menos um restart de cada worker.

---

## 🔍 Queries passo a passo

### 1. O dado cru

```promql
start_timestamp_worker_jobs_total
```

**Resultado esperado:** dois dentes-de-serra: worker-a sobe até ~**720** e zera a cada 240 s; worker-b até ~**630** a cada 420 s.

---

### 2. O start timestamp: uma escada

```promql
start_timestamp(start_timestamp_worker_jobs_total)
```

**O que faz:** em vez do valor, devolve o created timestamp guardado junto com cada amostra.
**Resultado esperado:** duas **escadas**. worker-a sobe **240** a cada degrau; worker-b sobe **420**. Entre restarts a linha é **reta** (o counter continua "nascido" no mesmo instante).

---

### 3. Uptime

```promql
time() - start_timestamp(start_timestamp_worker_jobs_total)
```

**Resultado esperado:** dente-de-serra 0 → **240 s** (worker-a) e 0 → **420 s** (worker-b), zerando **exatamente** no restart.

---

### 4. Média desde o nascimento (sem janela)

```promql
start_timestamp_worker_jobs_total / (time() - start_timestamp(start_timestamp_worker_jobs_total))
```

**O que faz:** total acumulado ÷ segundos de vida = taxa média **desde o início**, sem escolher janela e sem extrapolação.
**Resultado esperado:** worker-a ≈ **3,0** jobs/s, worker-b ≈ **1,5** jobs/s (com um pequeno ruído logo após cada restart, quando o denominador é pequeno).

---

### 5. Quantos restarts nos últimos 10 min

```promql
changes(start_timestamp(start_timestamp_worker_jobs_total)[10m:5s])
```

**O que faz:** subquery que reavalia `start_timestamp()` a cada 5 s e conta quantas vezes o valor **mudou**.
**Resultado esperado:** worker-a = **2** ou **3** (600 ÷ 240), worker-b = **1** ou **2** (600 ÷ 420).

> 💡 Diferente de [`resets()`](../resets/): `resets()` só vê um restart se o valor **caiu**. Se o processo reiniciar e o counter já estiver em 0 (ou subir rápido entre scrapes), `resets()` não percebe; o start timestamp muda de qualquer jeito.

---

### 6. Uptime do próprio gerador

```promql
time() - start_timestamp(start_timestamp_http_requests_total{route="/home"})
```

**Resultado esperado:** o tempo desde o último `tools/deploy.sh` (que reinicia o gerador). O client_golang anota o created timestamp **sozinho** em todo `Counter`, `Histogram` e `Summary`.

---

### 7. Pegadinhas: 0 e vazio

```promql
start_timestamp(start_timestamp_legacy_jobs_total)              # → 0   (sem created)
start_timestamp(rate(start_timestamp_worker_jobs_total[1m]))    # → vazio (expressão)
time() - (start_timestamp({__name__=~"start_timestamp_(worker|legacy)_jobs_total"}) > 0)   # filtro seguro
```

**Resultado esperado:**
- legacy → **0**, não vazio! `time() - 0` daria um "uptime" de **56 anos**.
- em expressão → **vazio** (a doc: *"only works when used directly on an instant vector"*).
- o filtro `> 0` descarta o legacy: sobra só worker-a e worker-b com uptime válido.

---

### 8. Jeito novo vs jeito antigo

```promql
time() - start_timestamp(start_timestamp_worker_jobs_total{pod="worker-a"})
time() - start_timestamp_process_start_time_seconds{pod="worker-a"}
```

**Resultado esperado:** as duas linhas **se sobrepõem** (dente-de-serra 0 → 240 s). A vantagem do start timestamp: funciona para **qualquer counter**, até os que não vêm acompanhados de um `process_start_time_seconds` (ex.: counters de uma biblioteca, métricas por conexão/por sessão que nascem no meio da vida do processo).

---

## 🏭 Casos reais

### 1. Uptime de aplicações sem `process_start_time_seconds`

Apps em Java/.NET/Node instrumentados com OpenTelemetry → Prometheus (protobuf / OpenMetrics) expõem created timestamps nos counters, mas nem sempre expõem `process_start_time_seconds`. Com `use-start-timestamps` ligado:

```yaml
- record: app:uptime_seconds
  expr: time() - max by (job, instance) (start_timestamp(http_server_requests_total) > 0)
```

### 2. Detectar restart que o `resets()` não vê

Um pod reinicia e em 2 s já processou mais que antes (scrape de 30 s): o counter nunca "caiu" entre dois scrapes, `resets()` = 0. O start timestamp muda:

```yaml
- alert: ProcessoReiniciou
  expr: changes(start_timestamp(http_requests_total{job="checkout"})[15m:30s]) > 2
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.instance }} reiniciou mais de 2x em 15 min"
```

(Em Kubernetes, o equivalente clássico é `increase(kube_pod_container_status_restarts_total[15m]) > 2`.)

### 3. Precisão de `rate()`/`increase()` no primeiro scrape

Esse é o motivo **principal** da feature: com o start timestamp, o Prometheus sabe que um counter que apareceu com valor `50` nasceu em `T0`, e não precisa "chutar" (extrapolar) a partir do zero. Counters de vida curta (jobs, conexões) ficam com `increase()` bem mais preciso.

---

## ✅ Quando usar

- **Uptime** a partir de qualquer counter com created timestamp.
- **Contar restarts** (`changes(start_timestamp(x)[..:])`), inclusive os invisíveis para `resets()`.
- **Taxa média desde o início** do processo, sem escolher janela.
- Auditoria: "este counter começou a contar quando?".

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Exporter não expõe created (text format antigo) ou a flag está desligada | `time() - process_start_time_seconds` com [`time()`](../time/) |
| Quer o horário da **coleta** da amostra | [`timestamp()`](../timestamp/) |
| Quer contar quedas do counter | [`resets()`](../resets/) |
| Restarts de container no Kubernetes | `kube_pod_container_status_restarts_total` + [`increase()`](../increase/) |
| Métrica é gauge | não existe start timestamp para gauge |

## ⚠️ Pegadinhas

1. **Sem created → 0, não vazio.** Filtre com `> 0` antes de calcular uptime.
2. **Só sobre seletor:** `start_timestamp(rate(x[1m]))`, `start_timestamp(sum(x))` → **vazio**.
3. **Precisa das flags** (`use-start-timestamps`; `st-storage` para persistir). Sem elas → vazio.
4. **Formato do scrape:** text format 0.0.4 não carrega created. Precisa de **protobuf** (ou OpenMetrics com `_created`).
5. **Experimental / recente:** a feature ainda está evoluindo. Não baseie alertas críticos só nela; tenha `process_start_time_seconds` como plano B.

## 🎓 Na prova PCA

A PCA foca nos fundamentos; `start_timestamp()` é recente. O que vale saber:
- A ideia de **created timestamp** (OpenMetrics `_created`) e por que ele existe: saber quando um counter/histogram começou (melhora `rate()` e detecta resets).
- Uptime "clássico": `time() - process_start_time_seconds`.
- Funções de timestamp operam em **instant vectors** e retornam instant vectors.

**1.** Em OpenMetrics, qual sufixo expõe o instante em que um counter começou a contar?
- A) `_total`
- B) `_created`
- C) `_start`
- D) `_timestamp`

<details><summary>Resposta</summary>

**B.** `x_created`. O `_total` é o próprio valor do counter.
</details>

**2.** Qual expressão clássica dá o uptime de um processo instrumentado com client_golang, **sem** feature flags?
- A) `time() - process_start_time_seconds`
- B) `start_timestamp(up)`
- C) `timestamp(process_start_time_seconds)`
- D) `increase(process_start_time_seconds[1h])`

<details><summary>Resposta</summary>

**A.** `process_start_time_seconds` é um gauge cujo valor é o horário de boot.
</details>

**3.** Com `use-start-timestamps` ligado, o que retorna `start_timestamp(rate(http_requests_total[5m]))`?
- A) o created timestamp de cada série
- B) o horário da avaliação
- C) vazio
- D) 0

<details><summary>Resposta</summary>

**C.** A função só funciona diretamente sobre um seletor de instant vector; sobre expressões retorna vazio.
</details>

**4.** Um exporter antigo expõe um counter sem created timestamp. Com a flag ligada, `start_timestamp(x)` retorna:
- A) vazio
- B) 0
- C) o horário do primeiro scrape
- D) erro

<details><summary>Resposta</summary>

**B.** (neste lab/versão 3.15.0.) Por isso `time() - start_timestamp(x)` precisa do filtro `> 0`.
</details>

## 📝 Cola rápida

- `start_timestamp(v)` → quando o counter **começou do zero** (created timestamp), em segundos Unix.
- Uptime: `time() - start_timestamp(x)`; restarts: `changes(start_timestamp(x)[10m:])`.
- Precisa de `use-start-timestamps` (+ `st-storage`) e scrape em protobuf/OpenMetrics.
- Sem created → **0** (filtre `> 0`); sobre expressão → **vazio**.
- Plano B clássico: `time() - process_start_time_seconds`.

## 🔗 Relacionadas

[`time()`](../time/) · [`timestamp()`](../timestamp/) · [`resets()`](../resets/) · [`changes()`](../changes/) · [`rate()`](../rate/) · [`increase()`](../increase/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#start_timestamp
