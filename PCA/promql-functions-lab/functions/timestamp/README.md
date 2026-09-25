# `timestamp()`: quando a amostra foi coletada

> **Em uma frase:** `timestamp(v)` troca o **valor** de cada amostra pelo **horário** (segundos Unix, UTC) em que ela foi coletada. `time() - timestamp(x)` = **"há quanto tempo esse dado não é atualizado?"**.

| | |
|---|---|
| **Assinatura** | `timestamp(v instant-vector) → instant-vector` |
| **Tipo de métrica** | qualquer uma (gauge, counter, float ou histogram: o valor é ignorado) |
| **Unidade do resultado** | segundos Unix (UTC), com milissegundos (`1790374320.123`) |
| **Dashboard** | http://localhost:3300/d/fn-timestamp |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a carta e o carimbo do correio

Cada amostra é uma **carta**: o **conteúdo** é o valor (`-18 °C`), o **carimbo do correio** é o timestamp (`22:05:00`).

- Ler a carta = consultar a métrica (`x`).
- Olhar o carimbo = `timestamp(x)`.
- "Há quanto tempo essa carta foi postada?" = `time() - timestamp(x)`.

Uma carta com conteúdo ótimo mas carimbada **ontem** é notícia velha. Olhando só o valor, você **nunca** percebe isso.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita um **gateway de IoT** (ou um Pushgateway / federate) que repassa leituras de sensores com o **timestamp da leitura**, e não o do scrape:

| Métrica | Timestamp | Comportamento |
|---|---|---|
| `timestamp_sensor_temperature_celsius{sensor="sala-servidores"}` | o do **scrape** (a cada 5 s) | 22 ± 1 °C, sempre fresco |
| `timestamp_sensor_temperature_celsius{sensor="estufa-lora"}` | **explícito**, 1 leitura a cada **60 s** | 28 ± 2 °C; idade 0 → 60 s |
| `timestamp_sensor_temperature_celsius{sensor="freezer-bateria"}` | **explícito**, 1 leitura a cada **7 min** (420 s) | −18 ± 2 °C; idade 0 → 300 s e depois **some** por 2 min |

No formato texto, o timestamp explícito é o 3º campo (em **ms**):

```bash
curl -s localhost:8088/metrics | grep '^timestamp_'
# timestamp_sensor_temperature_celsius{sensor="estufa-lora"} 29.3 1790374320000
# timestamp_sensor_temperature_celsius{sensor="sala-servidores"} 22.4
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-timestamp
```

Espere **~8 min** para ver um ciclo completo do freezer (inclusive o buraco).

---

## 🔍 Queries passo a passo

### 1. O dado cru

```promql
timestamp_sensor_temperature_celsius
```

**Resultado esperado:** 3 linhas de temperatura. A estufa aparece em **degraus de 60 s** (o valor só muda a cada leitura). O freezer é uma linha reta que **some** por ~2 min a cada 7 min. Pelo valor, nada indica que a estufa está 50 s atrasada.

---

### 2. Idade da última leitura

```promql
time() - timestamp(timestamp_sensor_temperature_celsius)
```

**O que faz:** para cada série, pega a amostra mais recente dentro do *lookback* (5 min), lê o **horário** dela e subtrai do instante avaliado.
**Resultado esperado** (segundos):

| sensor | forma | faixa |
|---|---|---|
| `sala-servidores` | quase zero | 0 a **5** (intervalo de scrape; um pouco mais se o gerador estiver sendo reiniciado por um deploy) |
| `estufa-lora` | dente-de-serra | 0 → **60** |
| `freezer-bateria` | dente-de-serra | 0 → **300** e então **desaparece** até a próxima leitura (aos 420 s) |

> 💡 Por que o freezer some aos 300 s? O Prometheus só "enxerga" uma amostra até **5 min** depois dela (`--query.lookback-delta`, padrão 5m). Depois disso, para o PromQL, a série **não existe**.

---

### 3. Alerta: sensor sem leitura há mais de 90 s

```promql
time() - timestamp(timestamp_sensor_temperature_celsius) > 90
```

**Resultado esperado:** só o `freezer-bateria`, e só enquanto a idade está **entre 90 e 300 s**. Depois dos 300 s ele some da consulta e **o alerta resolve sozinho** (falso "tudo bem"!). Por isso o próximo painel.

---

### 4. Idade robusta com subquery

```promql
time() - max_over_time(timestamp(timestamp_sensor_temperature_celsius)[10m:5s])
```

**O que faz:** avalia `timestamp(x)` a cada 5 s nos últimos 10 min e guarda o **maior** (o mais recente). Mesmo quando a amostra sai do lookback, a subquery ainda lembra dela.
**Resultado esperado:** o freezer agora sobe continuamente até **~420 s** e zera na leitura seguinte, sem buracos.

---

### 5. Pegadinha: `timestamp()` de uma expressão

```promql
time() - timestamp(last_over_time(timestamp_sensor_temperature_celsius[10m]))
```

**Resultado esperado:** **0** para todos os sensores, o tempo todo. O resultado de qualquer função/operação recebe o **timestamp da avaliação**, não o da amostra original. `timestamp()` só devolve o horário real da coleta quando é aplicado **diretamente num seletor** (`timestamp(metrica{...})`).

---

### 6. Stat: idade agora

```promql
time() - timestamp(timestamp_sensor_temperature_celsius)
```

Como consulta instantânea: sala ≈ 0-5 s, estufa entre 0 e 60 s, freezer entre 0 e 300 s (ou ausente).

---

## 🏭 Casos reais

### 1. Pushgateway: o batch job parou de empurrar métricas

Métricas no Pushgateway ficam lá **para sempre**, com o último valor. O Pushgateway expõe `push_time_seconds` (valor = horário do último push), mas quando o job empurra com timestamp explícito (ou você federa de outro Prometheus com `honor_timestamps`), `timestamp()` é o jeito de saber a idade:

```yaml
- alert: BatchSemAtualizar
  expr: time() - timestamp(batch_records_processed{job="pushgateway"}) > 2 * 3600
  for: 10m
  labels: {severity: warning}
```

### 2. Federation / remote-read: dado velho que "parece" fresco

Num Prometheus global que federa os regionais (`honor_timestamps: true`), se o regional travar, as séries continuam aparecendo até o lookback expirar. Painel de frescor por região:

```yaml
- alert: FederacaoAtrasada
  expr: max by (region) (time() - timestamp(up{job="federate"})) > 120
  for: 5m
  labels: {severity: warning}
  annotations:
    summary: "Dados da região {{ $labels.region }} com {{ $value | humanizeDuration }} de atraso"
```

### 3. Exporter com cache (ex.: exporter de API externa que só atualiza a cada N min)

Exporters de cloud (CloudWatch, Stackdriver) expõem dados com **atraso de minutos** e com timestamp explícito. Antes de alertar sobre um valor, confira a idade:

```promql
aws_sqs_approximate_number_of_messages_visible_average
  and on(queue_name) (time() - timestamp(aws_sqs_approximate_number_of_messages_visible_average) < 900)
```

---

## ✅ Quando usar

- Saber **quando** um dado foi coletado/produzido (frescor, *staleness*).
- Detectar **exporters travados**, gateways IoT atrasados, federação parada.
- Descobrir o horário de um evento em séries esparsas: `timestamp(x)` no último ponto.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| O **valor** da métrica já é um timestamp (`*_timestamp_seconds`) | `time() - x` com [`time()`](../time/) |
| Saber quando o **counter começou** (restart) | [`start_timestamp()`](../start_timestamp/) |
| Detectar que o alvo inteiro caiu | `up == 0` |
| Detectar que a série sumiu | [`absent()`](../absent/) / [`absent_over_time()`](../absent_over_time/) |
| Quando foi o máximo/último valor numa janela | [`ts_of_max_over_time()`](../ts_of_max_over_time/), [`ts_of_last_over_time()`](../ts_of_last_over_time/) |

## ⚠️ Pegadinhas

1. **Só funciona "de verdade" sobre um seletor.** `timestamp(rate(x[1m]))`, `timestamp(sum(x))`, `timestamp(last_over_time(x[5m]))` devolvem o **instante da avaliação**.
2. **Lookback de 5 min:** amostras mais velhas que 5 min somem da consulta, e `time() - timestamp(x) > 600` **nunca** dispara. Use `max_over_time(timestamp(x)[1h:])` ou `absent_over_time`.
3. **Target normal (sem timestamp explícito):** o timestamp é o do **scrape**, então a idade fica sempre entre 0 e `scrape_interval`. `timestamp()` só é interessante com timestamps explícitos (Pushgateway, federation, gateways, exporters de cloud) ou para ver *jitter* de scrape.
4. **Series com timestamp explícito não recebem staleness markers**: quando o exporter para, elas continuam visíveis até o lookback acabar.
5. **Resultado com ms:** `1790374320.123`. Na maioria dos usos isso não importa.

## 🎓 Na prova PCA

- `timestamp(v)` recebe **instant vector** e retorna **instant vector** (mesmos labels, sem `__name__`), com o **horário da amostra** como valor.
- Diferença: `time()` = relógio da avaliação (scalar) × `timestamp(v)` = horário da coleta (vetor).
- Lookback delta padrão = **5 min**; amostras mais antigas não aparecem em consultas instantâneas.

**1.** O que retorna `time() - timestamp(up)` num target raspado a cada 15 s, saudável?
- A) sempre 0
- B) um valor entre 0 e ~15 segundos
- C) o uptime do target
- D) 1, porque `up == 1`

<details><summary>Resposta</summary>

**B.** É a idade do último scrape: entre 0 e o scrape interval. C seria `time() - process_start_time_seconds`.
</details>

**2.** Qual das expressões retorna o horário real de coleta das amostras?
- A) `timestamp(sum(node_load1))`
- B) `timestamp(node_load1)`
- C) `timestamp(rate(node_cpu_seconds_total[5m]))`
- D) `timestamp(avg_over_time(node_load1[5m]))`

<details><summary>Resposta</summary>

**B.** Só aplicado diretamente a um seletor de vetor instantâneo. Nos outros casos o resultado de uma expressão carrega o timestamp da **avaliação**.
</details>

**3.** Uma métrica do Pushgateway foi empurrada pela última vez há 20 min, com timestamp explícito. O que `metrica` retorna numa consulta instantânea agora (lookback padrão)?
- A) o último valor, normalmente
- B) resultado vazio
- C) NaN
- D) 0

<details><summary>Resposta</summary>

**B.** Com timestamp de 20 min atrás, a amostra está fora do lookback de 5 min: a série não aparece. (Se o Pushgateway expõe **sem** timestamp, cada scrape gera uma amostra nova e a série aparece sempre; aí o frescor se mede com `push_time_seconds`.)
</details>

**4.** Qual o tipo de retorno de `timestamp(x)`?
- A) scalar
- B) instant vector
- C) range vector
- D) string

<details><summary>Resposta</summary>

**B.** Um elemento por série de entrada, com os mesmos labels (o nome da métrica é removido).
</details>

## 📝 Cola rápida

- `timestamp(v)` → horário da **coleta** (segundos Unix, UTC); `time()` → horário da **avaliação**.
- Idade do dado: `time() - timestamp(x)`.
- Só vale sobre **seletor**; sobre expressão = hora da avaliação.
- Lookback 5 min: dado mais velho some. Para lembrar mais: `max_over_time(timestamp(x)[1h:])`.
- Útil com timestamps explícitos: Pushgateway, federation, IoT, exporters de cloud.

## 🔗 Relacionadas

[`time()`](../time/) · [`start_timestamp()`](../start_timestamp/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/) · [`absent_over_time()`](../absent_over_time/) · [`last_over_time()`](../last_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#timestamp
