# `absent_over_time()`: "nenhum dado nos últimos N minutos"

> **Em uma frase:** `absent_over_time(v[janela])` devolve **vazio** se houver **qualquer** amostra (de qualquer série do seletor) dentro da janela, e **uma série com valor 1** se a janela estiver **completamente vazia**. É o `absent()` com **tolerância**.

| | |
|---|---|
| **Assinatura** | `absent_over_time(v range-vector) → instant-vector` (vazio **ou** 1 elemento com valor `1`) |
| **Tipo de métrica** | ✅ Qualquer uma (só importa se há amostras ou não) |
| **Unidade do resultado** | nenhuma: é sempre `1` (ou nada) |
| **Dashboard** | http://localhost:3300/d/fn-absent_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o pai preocupado e o pai tranquilo

O filho saiu de casa e prometeu mandar mensagem.

- O pai **ansioso** (`absent()`) liga **no primeiro segundo** em que não há mensagem nova. Toda vez que o filho entra num túnel, alarme.
- O pai **tranquilo** (`absent_over_time(x[2m])`) só se preocupa se o filho ficar **2 minutos inteiros** sem dar sinal. Um túnel de 30s não gera ligação.

A janela é a **tolerância**: quanto tempo de silêncio você aceita antes de acordar alguém.

```
db:          ●●●●●●●●●●●●                                                  ●●●●●●●●●●●●
             |← 60s de dados →|←──────────────── 240s de silêncio ────────────→|
absent():                      ████████████████████████████████████████████████  (4 min)
absent_over_time([2m]):                                 ███████████████████████  (2 min)
                               ^ precisa de 2 min sem amostra antes de acender
```

Mesma dedução de labels do `absent()`:

```
absent_over_time(nonexistent{job="myjob"}[1h])                        # => {job="myjob"} 1
absent_over_time(nonexistent{job="myjob",instance=~".*"}[1h])         # => {job="myjob"} 1
absent_over_time(sum(nonexistent{job="myjob"})[1h:])                  # => {} 1
```

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita o heartbeat de **backups**:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `absent_over_time_backup_heartbeat{backup="files"}` | gauge (1) | **sempre** presente |
| `absent_over_time_backup_heartbeat{backup="db"}` | gauge (1) | ciclo de **5 min**: reporta por **60s** e fica **240s** em silêncio |
| `absent_over_time_nonexistent_metric` | — | **nunca** exposta (exemplos da documentação) |

```bash
curl -s localhost:8088/metrics | grep '^absent_over_time_'
# absent_over_time_backup_heartbeat{backup="db"} 1      <- só no 1º minuto de cada 5
# absent_over_time_backup_heartbeat{backup="files"} 1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-absent_over_time
```

Espere **~5 min** para ver um ciclo completo do `db`.

---

## 🔍 Queries passo a passo

### 1. O heartbeat cru

```promql
absent_over_time_backup_heartbeat{backup="db"}
absent_over_time_backup_heartbeat{backup="files"} * 2   # × 2 só para não sobrepor no gráfico
```

**Resultado esperado:** `files` contínuo (em 2); `db` (em 1) aparece só por **1 min** a cada 5.

---

### 2. `absent()` vs `absent_over_time([2m])`

```promql
absent(absent_over_time_backup_heartbeat{backup="db"}) * 2           # imediato
absent_over_time(absent_over_time_backup_heartbeat{backup="db"}[2m]) # tolerante
```

(O `* 2` é só para as barras não ficarem uma em cima da outra no gráfico.)
**Resultado esperado**, em cada ciclo de 5 min (t=0 quando o `db` volta):

| t (s) | `db` | `absent()` | `absent_over_time([2m])` |
|---|---|---|---|
| 0–60 | presente | vazio | vazio |
| 60–180 | sumiu | **2** (acende já) | vazio (ainda há amostra nos últimos 2 min) |
| 180–300 | sumiu | **2** | **1** |

`absent()` fica aceso **4 min** por ciclo; `absent_over_time([2m])` só **2 min**.

---

### 3. O tamanho da janela é a tolerância

```promql
absent_over_time(absent_over_time_backup_heartbeat{backup="db"}[1m]) * 3
absent_over_time(absent_over_time_backup_heartbeat{backup="db"}[2m]) * 2
absent_over_time(absent_over_time_backup_heartbeat{backup="db"}[4m])
```

**Resultado esperado:**
- `[1m]`: acende de t≈120 a 300 → **3 min** por ciclo.
- `[2m]`: acende de t≈180 a 300 → **2 min** por ciclo.
- `[4m]`: praticamente **nunca** acende: o silêncio de 240s é do mesmo tamanho da janela, então sempre sobra uma amostra do minuto anterior dentro dela (no máximo um "piscar" de um ponto).

**Regra:** escolha a janela = **maior silêncio aceitável**. Para um job que roda a cada hora, `[2h]`; para um exporter com scrape de 15s, `[5m]`.

---

### 4. `files` nunca some → sempre vazio

```promql
absent_over_time(absent_over_time_backup_heartbeat{backup="files"}[2m])
```

**Resultado esperado:** **vazio** o tempo todo ("No data" é a resposta **certa**).

---

### 5. Dedução de labels (exemplos da documentação)

```promql
absent_over_time(absent_over_time_nonexistent_metric{job="myjob"}[1h])
absent_over_time(absent_over_time_nonexistent_metric{job="myjob",instance=~".*"}[1h])
absent_over_time(sum(absent_over_time_nonexistent_metric{job="myjob"})[10m:])
```

**Resultado esperado** (uma tabela por query): `{job="myjob"} 1`, `{job="myjob"} 1` e `{} 1`. O terceiro usa **subquery** (`[10m:]`), porque `sum(...)` devolve instant vector e a função precisa de range vector. (A doc usa `[1h:]`; aqui usamos `[10m:]` para a query ser mais leve.)

---

### 6. 🔴 Caso real no dashboard: ETL horário sem reportar há 2h

```promql
absent_over_time(up{job="etl-hourly"}[2h])   # job que não existe neste Prometheus
absent_over_time(up{job="lab"}[2h])          # job real
```

**Resultado esperado:** `{job="etl-hourly"} 1` (nenhuma amostra de `up` desse job nas últimas 2h) e **vazio** para `job="lab"`. Em produção, a primeira seria a regra `ETLJobNotReporting` disparando.

---

## 🏭 Casos reais

### 1. Job batch que roda de hora em hora

Um job de ETL é executado a cada hora e expõe métricas só enquanto roda (ou é raspado só nesse período). `absent()` dispararia 55 min por hora. Com `absent_over_time`, a tolerância é explícita:

```yaml
groups:
  - name: batch
    rules:
      - alert: ETLJobNotReporting
        expr: absent_over_time(etl_rows_processed_total{job="etl-hourly"}[2h])
        labels: {severity: warning}
        annotations:
          summary: "O job {{ $labels.job }} não reporta métricas há 2 horas (perdeu pelo menos 1 execução)"
```

`{{ $labels.job }}` funciona porque o matcher `job="etl-hourly"` é de igualdade.

### 2. Exporter instável: evitar flapping

Um exporter de hardware (IPMI/SNMP) às vezes falha 1 ou 2 scrapes. `absent(ipmi_temperature_celsius{instance="srv-42"})` alterna dispara/resolve a cada falha. Duas formas equivalentes de dar tolerância:

```yaml
- alert: IPMIMetricsMissing
  expr: absent_over_time(ipmi_temperature_celsius{instance="srv-42"}[10m])
# ou
- alert: IPMIMetricsMissing
  expr: absent(ipmi_temperature_celsius{instance="srv-42"})
  for: 10m
```

**Diferença:** com `for:`, o alerta fica **pending** no 1º scrape faltando e só dispara após 10 min **contínuos** de ausência; qualquer amostra no meio volta tudo para o início. Com `absent_over_time`, a condição é calculada direto dos dados ("zero amostras nos últimos 10 min"), então o resultado é o mesmo em qualquer query/dashboard, sem depender do estado interno da regra. Na prática os dois se comportam de forma muito parecida; `absent_over_time` deixa a tolerância **visível na própria expressão** (e funciona igual em painéis do Grafana, onde não existe `for:`).

### 3. Backup diário

```yaml
- alert: BackupMissing
  expr: absent_over_time(backup_last_success_timestamp_seconds{db="orders"}[26h])
  labels: {severity: critical}
```

26h = 24h de periodicidade + 2h de folga para o backup atrasar sem alarme.

---

## ✅ Quando usar

- **Jobs periódicos** (cron, batch, backup) que naturalmente ficam sem métricas entre execuções.
- **Alertas de ausência com tolerância** a falhas curtas de scrape.
- Quando você quer a tolerância **dentro da expressão** (funciona igual em dashboards, onde não existe `for:`).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer reagir **imediatamente** à ausência | [`absent()`](../absent/) |
| Quer saber **qual** série parou (manter labels) | [`ts_of_last_over_time()`](../ts_of_last_over_time/) ou `x offset 10m unless x` |
| Quer um 1 **por série** que teve dados na janela | [`present_over_time()`](../present_over_time/) |
| Quer contar amostras | [`count_over_time()`](../count_over_time/) |

## ⚠️ Pegadinhas

1. **Só dispara quando TODAS as séries do seletor** ficam sem amostras na janela. Filtre até o nível que importa.
2. **Janela ≥ silêncio**: com janela igual ou maior que o intervalo entre execuções, **nunca** dispara (query 3, `[4m]`).
3. **Labels deduzidos só de matchers `=`**; agregação → `{}` (e exige subquery).
4. **Precisa de range vector**: `absent_over_time(sum(x)[1h:])`, não `absent_over_time(sum(x))`.
5. **Resultado vazio = tudo bem**. Painéis mostram "No data".
6. **Staleness marker não é amostra**: ele não "conta" como dado dentro da janela.

---

## 🎓 Na prova PCA

Saiba:
- Recebe **range vector**; devolve **vazio** ou `{labels deduzidos} 1`.
- Diferença para `absent` (instant vector, reação imediata).
- Diferença para `present_over_time` (1 **por série** que tem amostras) e `count_over_time` (número de amostras por série).
- Com agregação dentro, precisa de **subquery**.

**1.** Um job roda a cada 30 min e só é raspado enquanto executa (~2 min). Qual expressão alerta se ele perder **uma** execução, sem falsos positivos entre execuções?

- A) `absent(job_metric{job="etl"})`
- B) `absent_over_time(job_metric{job="etl"}[5m])`
- C) `absent_over_time(job_metric{job="etl"}[45m])`
- D) `absent_over_time(job_metric{job="etl"}[24h])`

<details><summary>Resposta</summary>

**C.** Entre execuções há ~28 min de silêncio normal; a janela precisa ser maior que isso e menor que 2 intervalos (60 min). A e B disparariam entre toda execução; D só dispararia depois de um dia sem dados.
</details>

**2.** Qual expressão é **válida**?

- A) `absent_over_time(sum(up{job="api"}))`
- B) `absent_over_time(sum(up{job="api"})[10m:])`
- C) `absent_over_time(up{job="api"})`
- D) `absent(up{job="api"}[10m])`

<details><summary>Resposta</summary>

**B.** `absent_over_time` exige range vector; com agregação, isso se obtém via subquery. A e C passam instant vector; D passa range vector para `absent`, que exige instant vector.
</details>

**3.** O que retorna `absent_over_time(nonexistent{job="myjob",instance=~".*"}[1h])`?

- A) `{job="myjob", instance=~".*"} 1`
- B) `{job="myjob"} 1`
- C) `{} 1`
- D) Vetor vazio

<details><summary>Resposta</summary>

**B.** Mesma dedução do `absent`: só matchers de igualdade viram labels.
</details>

**4.** Qual a diferença entre `absent_over_time(x[10m])` e `present_over_time(x[10m])`?

- A) São opostos exatos e retornam as mesmas séries.
- B) `present_over_time` retorna **1 por série** que teve amostras; `absent_over_time` retorna **no máximo 1** elemento, só se **nenhuma** série teve amostras.
- C) `present_over_time` é experimental.
- D) `absent_over_time` retorna 0 quando há dados.

<details><summary>Resposta</summary>

**B.** Não são simétricos: `present_over_time` preserva as séries (e seus labels); `absent_over_time` é "tudo ou nada". Nenhum dos dois retorna 0.
</details>

---

## 📝 Cola rápida

- `absent_over_time(x[w])` → vazio se houve **qualquer** amostra na janela; `{labels =} 1` se nenhuma.
- A janela é a **tolerância** ao silêncio: escolha > intervalo normal entre dados.
- Mesmas regras de labels do `absent` (só `=`; agregação → `{}` e precisa de subquery `[w:]`).
- vs `for:` na regra: efeito parecido, mas a janela fica na própria expressão (serve também para dashboards).
- `present_over_time` = 1 por série presente; `absent_over_time` = 1 se **ninguém** apareceu.

## 🔗 Relacionadas

[`absent()`](../absent/) · [`present_over_time()`](../present_over_time/) · [`count_over_time()`](../count_over_time/) · [`last_over_time()`](../last_over_time/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#absent_over_time
