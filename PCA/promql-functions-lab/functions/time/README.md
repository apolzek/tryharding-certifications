# `time()`: o relógio da consulta

> **Em uma frase:** `time()` devolve o número de **segundos desde 01/01/1970 UTC** no **instante em que a expressão é avaliada**. Sozinho é só um relógio; subtraindo dele uma métrica cujo *valor* é um timestamp, vira **"há quanto tempo?"** ou **"quanto falta?"**.

| | |
|---|---|
| **Assinatura** | `time() → scalar` |
| **Tipo de métrica** | nenhuma (não recebe argumento). Combina com gauges cujo **valor é um timestamp** (`*_timestamp_seconds`, `process_start_time_seconds`, `probe_ssl_earliest_cert_expiry`) |
| **Unidade do resultado** | segundos Unix (float, ex.: `1790373725.849`) |
| **Dashboard** | http://localhost:3300/d/fn-time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o relógio de parede e o carimbo na caixa de leite

`time()` é o **relógio de parede** da cozinha. Olhar para ele sozinho só diz "são 19h03".

O valor interessante está nas **caixas de leite** da geladeira: cada uma tem um **carimbo de data** (uma métrica cujo valor é um timestamp). A pergunta útil é sempre uma **subtração**:

```
há quanto tempo foi feito?   = relógio − carimbo de fabricação   →  time() - backup_last_success_timestamp_seconds
quanto falta para vencer?    = carimbo de validade − relógio      →  probe_ssl_earliest_cert_expiry - time()
```

E um detalhe importante: o relógio da cozinha do Prometheus está sempre em **Londres (UTC)**. Para `time()` isso não importa (segundos desde 1970 são iguais no mundo todo), mas importa muito para [`hour()`](../hour/), [`day_of_week()`](../day_of_week/) e amigos.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `time_backup_last_success_timestamp_seconds{backup="db-backup"}` | gauge (timestamp) | backup a cada **3 min** → idade 0 → 180 s |
| `time_backup_last_success_timestamp_seconds{backup="files-backup"}` | gauge (timestamp) | backup a cada **10 min** → idade 0 → 600 s |
| `time_backup_last_success_timestamp_seconds{backup="legacy-backup"}` | gauge (timestamp) | **quebrado**: último sucesso ontem 00:00 UTC → idade de **1 a 2 dias** |
| `time_process_start_time_seconds{pod="api-0"}` | gauge (timestamp) | imita `process_start_time_seconds`; **crashloop**: reinicia a cada 6 min |
| `time_process_start_time_seconds{pod="api-1"}` | gauge (timestamp) | no ar há **3 a 4 dias** |
| `time_probe_ssl_earliest_cert_expiry{domain}` | gauge (timestamp) | imita o blackbox exporter: `api` expira em ~45 dias, `shop` em ~12, `old` em ~3 |

```bash
curl -s localhost:8088/metrics | grep '^time_'
# time_backup_last_success_timestamp_seconds{backup="db-backup"} 1.79037366e+09
# time_probe_ssl_earliest_cert_expiry{domain="old.lab.local"} 1.7906112e+09
```

## ▶️ Como rodar

```bash
tools/deploy.sh          # na raiz do projeto
# Grafana: http://localhost:3300/d/fn-time    Prometheus: http://localhost:9095
```

Os dados são calculados a partir do relógio de parede: em ~1 minuto todos os painéis já têm dados; em ~10 min você vê um ciclo completo do `files-backup`.

---

## 🔍 Queries passo a passo

### 1. `time()` sozinho

```promql
time()
```

**O que faz:** devolve um **escalar** com o instante da avaliação.
**Resultado esperado:**
- **Gráfico (consulta range):** uma **rampa** subindo 1 por segundo. Numa janela de 15 min, a diferença entre a borda esquerda e a direita é ~**900**. Cada ponto do gráfico é uma avaliação separada, com o **seu próprio** `time()`.
- **Stat (consulta instantânea):** um número tipo `1790374556`. No painel 1b, `time() * 1000` com unidade `dateTimeAsIso` vira `2026-09-25 19:15:56`: o **Grafana** formata no fuso do **navegador** (Brasília), embora o número seja o mesmo no mundo todo (22:15:56 UTC).

> 💡 **Segundos vs milissegundos:** Prometheus trabalha em **segundos**; Grafana/JavaScript em **milissegundos**. Por isso o `* 1000` para formatar como data.

---

### 2 e 3. Idade do último backup

```promql
time_backup_last_success_timestamp_seconds{backup=~"db-backup|files-backup"}             # cru: escadinha
time() - time_backup_last_success_timestamp_seconds{backup=~"db-backup|files-backup"}    # idade em segundos
```

**O que faz:** o valor cru só muda quando um backup termina (uma **escadinha**). Subtraindo de `time()`, cada ponto vira "segundos desde o último sucesso".
**Resultado esperado:**

| backup | forma no gráfico | faixa |
|---|---|---|
| `db-backup` | dente-de-serra | 0 → **180 s**, zera a cada 3 min |
| `files-backup` | dente-de-serra | 0 → **600 s**, zera a cada 10 min |
| `legacy-backup` | (fora dos painéis 2 e 3: achataria a escala) | 86 400 a 172 800 s (1 a 2 dias) |

---

### 4. Alerta: backup com mais de 5 min

```promql
time() - time_backup_last_success_timestamp_seconds > 300
```

**O que faz:** o `> 300` é um **filtro**: só sobram as séries com idade acima de 300 s.
**Resultado esperado (tabela):** `legacy-backup` **sempre** (~1,x dia); `files-backup` só na segunda metade do seu ciclo (idade entre 300 e 600 s); `db-backup` **nunca** (máximo 180 s).

---

### 5 e 6. Dias até o certificado expirar

```promql
(time_probe_ssl_earliest_cert_expiry - time()) / 86400
(time_probe_ssl_earliest_cert_expiry - time()) / 86400 < 7
```

**O que faz:** inverte a subtração (validade − agora = quanto **falta**) e divide por 86 400 para ter **dias**.
**Resultado esperado** (às 22h UTC, ~0,92 do dia já passou):

| domain | dias |
|---|---|
| `api.lab.local` | ≈ **44,1** |
| `shop.lab.local` | ≈ **11,1** |
| `old.lab.local` | ≈ **2,1** → único que passa no filtro `< 7` |

---

### 7 e 8. Uptime do processo (e crashloop)

```promql
time() - time_process_start_time_seconds             # uptime em segundos
(time() - time_process_start_time_seconds) < 600     # reiniciou nos últimos 10 min?
```

**Resultado esperado:** `api-0` faz um dente-de-serra 0 → **360 s** (reinicia a cada 6 min) e aparece **sempre** no alerta. `api-1` tem ~3 dias de uptime (≈ 260 000 s) e nunca aparece.

---

## 🏭 Casos reais

### 1. "O backup do banco rodou esta noite?" (textfile collector)

Um script de backup grava no fim, via node_exporter textfile collector:

```
# /var/lib/node_exporter/textfile/backup.prom
backup_last_success_timestamp_seconds{backup="postgres"} 1790373600
```

Regra de alerta (backup diário; toleramos até 26 h para absorver atrasos):

```yaml
groups:
  - name: backups
    rules:
      - alert: BackupAtrasado
        expr: time() - backup_last_success_timestamp_seconds > 26 * 3600
        for: 15m
        labels: {severity: critical}
        annotations:
          summary: "Backup {{ $labels.backup }} sem sucesso há {{ $value | humanizeDuration }}"
```

**Decisão:** a métrica guarda **quando** foi o último sucesso (e não "sucesso = 1"). Assim o alerta pega tanto o job que **falhou** quanto o job que **nem rodou** (cron desativado), coisa que um `backup_success == 0` não pega.

### 2. Certificado TLS perto de expirar (blackbox exporter)

```yaml
- alert: CertificadoExpirando
  expr: (probe_ssl_earliest_cert_expiry - time()) / 86400 < 14
  for: 1h
  labels: {severity: warning}
  annotations:
    summary: "Certificado de {{ $labels.instance }} expira em {{ $value | humanize }} dias"
```

### 3. Crashloop / deploy recente com `process_start_time_seconds`

Toda aplicação instrumentada com client_golang/client_python expõe `process_start_time_seconds`.

```yaml
- alert: ProcessoReiniciandoMuito
  expr: time() - process_start_time_seconds < 300
  for: 15m        # 15 min seguidos com uptime < 5 min = reinicia sem parar
  labels: {severity: warning}
```

Para dashboard, `time() - process_start_time_seconds` com unidade `s` vira o painel "Uptime" clássico (o `node_exporter` tem o equivalente `time() - node_boot_time_seconds`).

### 4. CronJob do Kubernetes que parou de rodar

```promql
time() - kube_cronjob_status_last_successful_time{cronjob="nightly-report"} > 25 * 3600
```

---

## ✅ Quando usar

- **Idade/frescor** de algo registrado como timestamp: último backup, último deploy, último sucesso de job, última sincronização.
- **Contagem regressiva:** certificados, licenças, tokens, `kube_secret` com expiração.
- **Uptime:** `time() - process_start_time_seconds`, `time() - node_boot_time_seconds`.
- Como **argumento padrão** das funções de calendário: `hour()` = `hour(vector(time()))`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer saber **quando a amostra foi coletada** (e não o valor) | [`timestamp()`](../timestamp/) |
| Quer o início/fim/duração da janela do **gráfico** | [`start()`](../start/), [`end()`](../end/), [`range()`](../range/) |
| Quer a **hora do dia** / dia da semana | [`hour()`](../hour/), [`day_of_week()`](../day_of_week/) |
| Quer o uptime de um counter pelo created timestamp | [`start_timestamp()`](../start_timestamp/) |
| Detectar que o alvo **caiu** | `up == 0` ou [`absent()`](../absent/) |

## ⚠️ Pegadinhas

1. **`time()` é o instante AVALIADO, não o "agora" do servidor.** Num gráfico, cada ponto tem o seu `time()`; numa regra, é o horário da avaliação da regra; com `@`, o que o `@` mandar.
2. **É um escalar**, não um vetor: `sum(time())` dá erro. Para virar vetor: `vector(time())`.
3. **Unidade:** segundos. Métricas vindas de JavaScript/Java (`Date.now()`, `System.currentTimeMillis()`) estão em **ms** → divida por 1000.
4. **Sempre UTC** por definição (época Unix). O fuso só vira problema quando você extrai hora/dia com as funções de calendário.
5. **Não use o label `job` nas suas métricas:** o Prometheus já coloca `job` (do scrape) e renomeia o seu para `exported_job` (a não ser com `honor_labels: true`). Por isso o cenário usa `backup="..."`.
6. **Gauge "timestamp" parado vs alvo fora do ar:** se o exporter cair, a série some e `time() - x` fica **vazio** (o alerta não dispara!). Combine com `up == 0` ou `absent()`.

## 🎓 Na prova PCA

- `time()` **não recebe argumento** e retorna **scalar**.
- Padrão "idade": `time() - <métrica de timestamp>`. Padrão "quanto falta": `<métrica de timestamp> - time()`.
- Diferença clássica: `time()` (relógio da avaliação) **vs** `timestamp(v)` (horário da amostra).
- Unidades: **segundos** desde a época Unix, em UTC.

**1.** Qual expressão retorna há quantos segundos o processo foi iniciado?
- A) `process_start_time_seconds - time()`
- B) `time() - process_start_time_seconds`
- C) `timestamp(process_start_time_seconds)`
- D) `rate(process_start_time_seconds[5m])`

<details><summary>Resposta</summary>

**B.** Idade = agora − início. A dá um número negativo; C dá o horário do **scrape** (não o de início); D mede variação de um gauge que praticamente nunca muda.
</details>

**2.** Qual é o tipo de retorno de `time()`?
- A) instant vector com 1 elemento
- B) range vector
- C) scalar
- D) string

<details><summary>Resposta</summary>

**C.** `time()` é escalar. Para usar onde se exige vetor (ex.: `hour()`), use `vector(time())`.
</details>

**3.** Num gráfico de 1 h com step de 15 s, qual é o comportamento de `time()`?
- A) linha reta com o horário de agora
- B) rampa: cada ponto recebe o horário da sua própria avaliação
- C) linha reta com o horário do início do gráfico
- D) erro: `time()` só funciona em consultas instantâneas

<details><summary>Resposta</summary>

**B.** Uma consulta range é uma sequência de avaliações instantâneas, uma por step, e `time()` retorna o instante de **cada** avaliação.
</details>

**4.** Um exporter expõe `job_last_success_timestamp_seconds`. O job roda de hora em hora. Qual alerta é mais adequado?
- A) `job_last_success_timestamp_seconds < 3600`
- B) `time() - job_last_success_timestamp_seconds > 2 * 3600`
- C) `rate(job_last_success_timestamp_seconds[1h]) == 0`
- D) `absent(job_last_success_timestamp_seconds)`

<details><summary>Resposta</summary>

**B.** Compara a idade do último sucesso com uma tolerância (2 execuções). A compara um timestamp de 1970 com 1 h (sempre falso); C é frágil e sem sentido semântico; D só pega a métrica sumindo.
</details>

## 📝 Cola rápida

- `time()` → **scalar**, segundos Unix (UTC) **do instante avaliado**.
- Idade: `time() - x_timestamp_seconds` · Quanto falta: `x_expiry - time()` · Dias: `/ 86400`.
- Uptime: `time() - process_start_time_seconds`.
- Num gráfico, `time()` é uma rampa (1 por segundo), não "agora".
- ms → `/ 1000`. Métrica sumiu → `time() - x` fica vazio: combine com `up`/`absent()`.

## 🔗 Relacionadas

[`timestamp()`](../timestamp/) · [`start_timestamp()`](../start_timestamp/) · [`vector()`](../vector/) · [`hour()`](../hour/) · [`start()`](../start/) · [`end()`](../end/) · [`absent()`](../absent/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#time
