# `ts_of_last_over_time()`: quando foi a última amostra? (experimental)

> **Em uma frase:** `ts_of_last_over_time(v[janela])` devolve o **timestamp Unix (em segundos)** da amostra **mais recente** de cada série **dentro da janela**, mesmo que a série já tenha parado de ser reportada. É o "visto por último às..." das métricas.

| | |
|---|---|
| **Assinatura** | `ts_of_last_over_time(v range-vector) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Qualquer uma (gauge, counter; o valor não importa, só o timestamp) |
| **Unidade do resultado** | **segundos desde 1970** (timestamp Unix, com fração) |
| **Dashboard** | http://localhost:3300/d/fn-ts_of_last_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o "visto por último" do WhatsApp

Seu amigo não responde. Você abre a conversa e vê **"visto por último hoje às 14:02"**. Não importa **o que** ele escreveu por último, importa **quando** ele deu sinal de vida.

- `last_over_time(x[10m])` → **o que** foi dito por último (o valor).
- `ts_of_last_over_time(x[10m])` → **quando** foi dito (o timestamp).
- `time() - ts_of_last_over_time(x[10m])` → **há quantos segundos** ele está calado.

A diferença crucial para `timestamp(x)`: quando uma série **some** do scrape, o Prometheus grava um **marcador de staleness** e `x` (instant vector) passa a não retornar nada. `timestamp(x)` some junto. Já `ts_of_last_over_time(x[10m])` olha **para trás na janela** e ainda encontra a última amostra real.

```
flaky:  ●●●●●●●●●●●●●●●●●●                    ●●●●●●●●●
                         ^ última amostra      (volta)
timestamp(x):   ───────── (some) ─────────────
ts_of_last_over_time(x[10m]):  continua apontando para ^
```

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita um exporter de sensores IoT (ex.: MQTT/SNMP) em que um sensor tem Wi-Fi ruim:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `ts_of_last_over_time_sensor_temperature_celsius{sensor="healthy"}` | gauge | reporta **sempre** (~22°C) |
| `ts_of_last_over_time_sensor_temperature_celsius{sensor="flaky"}` | gauge | reporta por **90s** e **some por 90s** (ciclo de 3 min) |
| `ts_of_last_over_time_sensor_temperature_celsius{sensor="retired"}` | gauge | equipamento quase desativado: reporta só **60s a cada 15 min**. Fica mais tempo calado do que a janela `[10m]` |

```bash
curl -s localhost:8088/metrics | grep '^ts_of_last_over_time_'
# ts_of_last_over_time_sensor_temperature_celsius{sensor="healthy"} 22.4
# (a linha do flaky só aparece na metade "ligada" do ciclo)
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-ts_of_last_over_time
```

Espere **~3 min** para ver um ciclo completo do sensor flaky.

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
ts_of_last_over_time_sensor_temperature_celsius
```

**Resultado esperado:** `healthy` é uma linha contínua; `flaky` tem **buracos de 90s** a cada 3 min.

---

### 2. Segundos sem dado

```promql
time() - ts_of_last_over_time(ts_of_last_over_time_sensor_temperature_celsius[10m])
```

**O que faz:** para cada série que teve **pelo menos uma** amostra nos últimos 10 min, calcula a idade da última amostra.
**Resultado esperado** (unidade: s):
- `healthy`: sempre entre **0 e 5s** (a idade do último scrape, com scrape de 5s).
- `flaky`: 0 a 5s enquanto reporta; quando some, **cresce 1s por segundo** até ≈ **90s** e **despenca** para ~0 quando volta. Um dente-de-serra.
- `retired`: 0 a 5s durante o minuto em que reporta, depois sobe em rampa até ≈ **600s** e então **some do gráfico** (a última amostra saiu da janela de 10 min). Volta a aparecer só quando reporta de novo, 15 min depois do último ciclo. É a pegadinha "a janela é o limite da memória" ao vivo.

---

### 3. `timestamp(x)` some; `ts_of_last_over_time` não

```promql
time() - timestamp(ts_of_last_over_time_sensor_temperature_celsius{sensor="flaky"})
time() - ts_of_last_over_time(ts_of_last_over_time_sensor_temperature_celsius{sensor="flaky"}[10m])
```

**Resultado esperado:**
- `timestamp(x)`: linha em ~0-5s **com buracos** exatamente onde o sensor sumiu (staleness marker ⇒ instant vector vazio).
- `ts_of_last_over_time(x[10m])`: **linha contínua**, subindo até ~90 nos buracos.

**Moral:** para medir "há quanto tempo não chega dado", você precisa de uma função de **range vector**.

---

### 4. Alerta: sem dado há mais de 60s

```promql
(time() - ts_of_last_over_time(ts_of_last_over_time_sensor_temperature_celsius[10m])) > 60
```

**Resultado esperado:** pontos de `flaky` nos últimos **~30s** de cada buraco (quando a idade passa de 60s até ~90s) e uma faixa de `retired` de 60s até ~600s de idade (depois ele sai da janela e o alerta **resolve sozinho**, mesmo com o sensor ainda morto!). `healthy` nunca aparece.
Diferente de `absent()`, o resultado **mantém os labels** da série que sumiu (`sensor="flaky"`, `instance`, `job`), então o alerta diz **qual** sensor parou.

---

### 5. Tabela: último valor e quando

```promql
last_over_time(ts_of_last_over_time_sensor_temperature_celsius[10m])
ts_of_last_over_time(ts_of_last_over_time_sensor_temperature_celsius[10m]) * 1000
time() - ts_of_last_over_time(ts_of_last_over_time_sensor_temperature_celsius[10m])
```

**Resultado esperado:**

| sensor | último valor | visto por último | há quanto tempo |
|---|---|---|---|
| healthy | ~22°C | agora | 0–5s |
| flaky | ~25°C | agora **ou** até 90s atrás | 0–90s |
| retired | ~18°C | até 10 min atrás (depois some da tabela) | 0–600s |

---

## 🏭 Casos reais

### 1. "Qual série parou de reportar?" (com labels, diferente de `absent`)

Uma frota de 500 sensores/dispositivos; `absent(temperature{sensor="x"})` exigiria 500 regras. Com `ts_of_last_over_time`, **uma** regra cobre todos e o alerta sai com o label do dispositivo:

```yaml
- alert: SensorStale
  expr: (time() - ts_of_last_over_time(mqtt_sensor_temperature_celsius[30m])) > 300
  for: 1m
  labels: {severity: warning}
  annotations:
    summary: "Sensor {{ $labels.sensor }} sem dados há {{ $value | humanizeDuration }}"
```

A janela `[30m]` define o "esquecimento": depois de 30 min sem dado a série sai do resultado (e o alerta resolve sozinho). Combine com um `absent_over_time` se precisar de alerta eterno.

### 2. Exporter intermitente (SNMP/blackbox com timeout)

Um switch às vezes estoura o timeout do snmp_exporter e as séries daquele device somem por alguns scrapes. Um painel com `time() - ts_of_last_over_time(ifHCInOctets[15m])` por interface mostra quem está falhando e há quanto tempo, sem precisar de `up` (que é por target, não por interface). Como alerta:

```yaml
- alert: SNMPInterfaceStale
  expr: (time() - ts_of_last_over_time(ifHCInOctets{job="snmp"}[15m])) > 120
  for: 2m
  annotations:
    summary: "{{ $labels.instance }} ifIndex={{ $labels.ifIndex }} sem dados há {{ $value | humanizeDuration }}"
```

### 3. Métricas de job batch via Pushgateway

Métricas empurradas para o Pushgateway ficam **paradas** lá (não somem), então `ts_of_last_over_time` não serve: o scrape continua gerando amostras. Nesse caso use o `push_time_seconds` que o Pushgateway expõe: `time() - push_time_seconds{job="backup"} > 86400`. Saber **quando não usar** também cai na prova.

---

## ✅ Quando usar

- **Detectar séries paradas/intermitentes** mantendo os labels (qual dispositivo, qual pod).
- **Painéis de "freshness"**: idade do último dado por série.
- **Diagnóstico de scrape/exporter** instável por série.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Alertar que **nenhuma** série existe para um seletor | [`absent()`](../absent/) / [`absent_over_time()`](../absent_over_time/) |
| Só precisa do **valor** mais recente | [`last_over_time()`](../last_over_time/) |
| A série está presente agora e você quer o timestamp dela | [`timestamp()`](../timestamp/) (estável, sem flag) |
| Target inteiro caiu | `up == 0` |
| Métrica vem de Pushgateway | `time() - push_time_seconds` |

## ⚠️ Pegadinhas

1. **Experimental**: sem a feature flag, erro de função desconhecida.
2. **A janela é o limite da memória**: se a última amostra é mais velha que a janela, a série **some** do resultado (não aparece "idade infinita").
3. **Timestamp do scrape**: com scrape de 5s, uma série saudável tem idade de 0 a 5s, nunca exatamente 0.
4. **Pushgateway/exporters que "congelam" valores** continuam gerando amostras novas a cada scrape, então a idade fica sempre ~0.
5. **Segundos Unix**: `* 1000` para o Grafana formatar como data.

---

## 🎓 Na prova PCA

`ts_of_last_over_time` é **experimental**; a prova cobra os conceitos por trás: **staleness** (a série some ~imediatamente quando desaparece do scrape, ou após 5 min de lookback se não houver marcador), `timestamp()`, `last_over_time`, `absent`.

**1.** Uma série parou de aparecer no scrape há 2 minutos. O que retorna `timestamp(minha_metrica)`?

- A) O timestamp da última amostra.
- B) Vazio, porque o marcador de staleness faz o instant vector não retornar a série.
- C) O instante atual.
- D) 0.

<details><summary>Resposta</summary>

**B.** Quando uma série some de um scrape bem-sucedido, o Prometheus grava um *staleness marker*; o seletor instantâneo deixa de retorná-la.
</details>

**2.** Qual query devolve, para cada sensor que reportou nos últimos 30 minutos, **há quantos segundos** ele reportou pela última vez?

- A) `time() - timestamp(temp_celsius)`
- B) `time() - ts_of_last_over_time(temp_celsius[30m])`
- C) `absent_over_time(temp_celsius[30m])`
- D) `last_over_time(temp_celsius[30m])`

<details><summary>Resposta</summary>

**B.** A) some para sensores parados. C) só diz se **nenhuma** série existe (e perde os labels das que sumiram). D) devolve o **valor**, não o horário.
</details>

**3.** Qual a diferença entre `last_over_time(x[10m])` e `ts_of_last_over_time(x[10m])`?

- A) Nenhuma.
- B) O primeiro devolve o **valor** da amostra mais recente; o segundo, o **timestamp** dela.
- C) O primeiro é experimental.
- D) O segundo junta todas as séries.

<details><summary>Resposta</summary>

**B.** `last_over_time` é estável e devolve o valor; `ts_of_last_over_time` é experimental e devolve o timestamp.
</details>

**4.** Um sensor está parado há 45 minutos. `time() - ts_of_last_over_time(temp[30m])` retorna para ele:

- A) ~2700
- B) 1800
- C) Nada: não há amostras dele na janela de 30m.
- D) +Inf

<details><summary>Resposta</summary>

**C.** A função só enxerga a janela. Sem amostras nos últimos 30 min, a série não aparece no resultado.
</details>

---

## 📝 Cola rápida

- `ts_of_last_over_time(x[w])` = timestamp (s) da amostra mais recente **na janela**; **experimental**.
- `time() - ts_of_last_over_time(x[w])` = idade do último dado; mantém os **labels** (bom para alertas por série).
- `timestamp(x)` some quando a série fica stale; a versão `_over_time` "lembra" até o tamanho da janela.
- Série mais velha que a janela → some do resultado. Nenhuma série → use `absent`/`absent_over_time`.
- Pushgateway: use `push_time_seconds`.

## 🔗 Relacionadas

[`last_over_time()`](../last_over_time/) · [`timestamp()`](../timestamp/) · [`absent_over_time()`](../absent_over_time/) · [`present_over_time()`](../present_over_time/) · [`ts_of_first_over_time()`](../ts_of_first_over_time/) · [`ts_of_max_over_time()`](../ts_of_max_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
