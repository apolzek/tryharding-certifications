# `ts_of_min_over_time()`: QUANDO aconteceu o mínimo (experimental)

> **Em uma frase:** `ts_of_min_over_time(v[janela])` devolve o **timestamp Unix (em segundos)** da **última** amostra que tem o **valor mínimo** da janela, para cada série. É o "quando" que falta no `min_over_time()`.

| | |
|---|---|
| **Assinatura** | `ts_of_min_over_time(v range-vector) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Gauge · ⚠️ Counter (funciona, mas o mínimo de um counter é quase sempre o início da janela ou um reset) · ❌ Histogram (ignorado) |
| **Unidade do resultado** | **segundos desde 1970** (timestamp Unix, com fração) |
| **Dashboard** | http://localhost:3300/d/fn-ts_of_min_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o extrato bancário

`min_over_time(saldo[30d])` responde "qual foi o **menor saldo** do mês?" → R$ 12,00.
`ts_of_min_over_time(saldo[30d])` responde "**em que dia** foi isso?" → `1790000000` (o dia 23, às 14:05).

Se o saldo ficou em R$ 12,00 por três dias seguidos, a função devolve o **último** desses momentos (a documentação diz: *"the timestamp of the last float sample that has the minimum value"*).

```
valor    100 ─╮
               ╲
                ╲______        ← mínimo (0) repetido por 60s
                       ╱
timestamp            ^ ts_of_min_over_time devolve ESTE (o último ponto do platô)
```

O truque mais útil: **`time() - ts_of_min_over_time(x[5m])`** = "**há quantos segundos** foi o mínimo".

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita o `node_power_supply_capacity` do **node_exporter** (nível de bateria em %):

| Métrica | Tipo | Comportamento |
|---|---|---|
| `ts_of_min_over_time_node_power_supply_capacity{power_supply="BAT0"}` | gauge | notebook, ciclo de **5 min**: descarrega 100→0% em 180s, fica **desligado em 0% por 60s**, recarrega 0→100% em 60s |
| `ts_of_min_over_time_node_power_supply_capacity{power_supply="UPS"}` | gauge | nobreak, senóide entre **30% e 90%**, período de **4 min** (um único vale por ciclo) |

```bash
curl -s localhost:8088/metrics | grep '^ts_of_min_over_time_'
# ts_of_min_over_time_node_power_supply_capacity{power_supply="BAT0"} 43.9
# ts_of_min_over_time_node_power_supply_capacity{power_supply="UPS"} 71.2
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-ts_of_min_over_time
```

Espere **~5 min** para ver um ciclo completo da bateria.

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
ts_of_min_over_time_node_power_supply_capacity
```

**Resultado esperado:** `BAT0` é uma rampa descendo de 100 a 0, um **platô em 0** de 1 min e uma subida rápida. `UPS` é uma onda suave entre 30 e 90.

---

### 2. QUAL foi o mínimo

```promql
min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m])
```

**Resultado esperado:** `BAT0` = **0** (a janela de 5 min sempre contém o platô), `UPS` ≈ **30**. Só o valor, sem o "quando".

---

### 3. HÁ QUANTO TEMPO foi o mínimo

```promql
time() - ts_of_min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m])
```

**O que faz:** `time()` é o instante de avaliação (em segundos); subtraindo o timestamp do mínimo, sobra a "idade" do mínimo.
**Resultado esperado** (unidade: s):
- `BAT0`: fica em ≈ **0 a 5s** durante todo o platô em 0% (cada nova amostra 0 é "o último mínimo") e, quando a recarga começa, **cresce 1s por segundo** até ≈ **240s**, zerando de novo no próximo platô.
- `UPS`: **dente-de-serra** de 0 a ~240s: zera no fundo de cada vale.

---

### 4. O valor cru (timestamp)

```promql
ts_of_min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m])
```

**Resultado esperado:** um número enorme (≈ **1.79e9** em 2026), em forma de **escada**: fica parado enquanto o mínimo da janela não muda e dá um **salto** quando surge um mínimo mais novo. Sozinho é pouco legível; por isso usamos `time() - ...` ou `* 1000` para formatar como data.

---

### 5. Tabela: menor bateria e quando (e o erro clássico do `* 1000`)

```promql
min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m])
ts_of_min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m]) * 1000   # ✅
ts_of_min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m])          # ❌ sem * 1000
```

**Resultado esperado:**

| power_supply | mínimo em 5m | quando foi o mínimo | ❌ sem ×1000 |
|---|---|---|---|
| BAT0 | 0% | `2026-09-25 19:22:35` (o fim do último platô) | `1970-01-21 ...` |
| UPS | ~30% | horário do último vale | `1970-01-21 ...` |

> 💡 O Grafana espera **milissegundos** nas unidades de data (`dateTimeAsIso`). Sem o `* 1000`, 1.79 bilhão de **milissegundos** = ~20 dias depois de 1/1/1970.

---

### 6. 🔴 Caso real no dashboard: "bateria abaixo de 20% há menos de 60s"

```promql
min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m]) < 20
and
(time() - ts_of_min_over_time(ts_of_min_over_time_node_power_supply_capacity[5m])) < 60
```

**Resultado esperado:** pontos de `BAT0` durante o platô em 0% e por ~60s depois (≈ **2 min** por ciclo de 5). `UPS` nunca aparece (mínimo ≈ 30%). Um alerta "evento recente" que resolve sozinho quando o evento envelhece.

---

## 🏭 Casos reais

### 1. "Quando o disco esteve mais cheio?" (node_exporter)

Numa investigação pós-incidente, você quer saber **quando** o espaço livre chegou no fundo:

```promql
min_over_time(node_filesystem_avail_bytes{mountpoint="/"}[6h])            # quanto sobrou
ts_of_min_over_time(node_filesystem_avail_bytes{mountpoint="/"}[6h]) * 1000   # quando (Grafana: dateTimeAsIso)
```

Com o horário em mãos, você vai direto aos logs daquele minuto (rotação de log que falhou, dump que encheu o disco).

### 2. Bateria/UPS: alerta "o nível mínimo foi recente" (é o painel 6)

Um nobreak de rack teve queda de energia. Você quer alertar se o mínimo da bateria nas últimas 2h foi **abaixo de 20%** e **nos últimos 15 min** (evento em andamento, não de ontem):

```yaml
- alert: UPSBatteryLowRecently
  expr: |
    min_over_time(node_power_supply_capacity{power_supply=~"UPS.*"}[2h]) < 20
    and
    (time() - ts_of_min_over_time(node_power_supply_capacity{power_supply=~"UPS.*"}[2h])) < 900
  labels: {severity: critical}
```

### 3. Horário de menor tráfego (janela de manutenção)

Com uma recording rule `job:http_requests:rate5m`, o horário do vale do dia é:

```yaml
- record: job:http_requests:rate5m:ts_of_min_1d
  expr: ts_of_min_over_time(job:http_requests:rate5m{job="api"}[1d])
```

No Grafana, `job:http_requests:rate5m:ts_of_min_1d * 1000` com unidade `dateTimeAsIso`. Útil para escolher a janela de deploy/manutenção com base em dados, não em achismo.

---

## ✅ Quando usar

- **Investigação/postmortem**: "quando foi o fundo do poço?" (disco, memória livre, saldo, estoque, bateria).
- **Alertas "recentes"**: combinar `min_over_time(...) < X` com `time() - ts_of_min_over_time(...) < N`.
- **Achar o horário do vale** (tráfego mínimo, carga mínima).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Só precisa do valor mínimo | [`min_over_time()`](../min_over_time/) |
| Quer o horário do **pico** | [`ts_of_max_over_time()`](../ts_of_max_over_time/) |
| Quer o horário da amostra **mais recente** | [`ts_of_last_over_time()`](../ts_of_last_over_time/) ou [`timestamp()`](../timestamp/) |
| Ambiente sem a flag experimental | não há equivalente direto; aproxime inspecionando `min_over_time` num range query |

## ⚠️ Pegadinhas

1. **Experimental**: sem a feature flag, erro de função desconhecida.
2. **Empate = ÚLTIMA amostra** com o valor mínimo, não a primeira (query 3, platô do `BAT0`).
3. **Segundos, não milissegundos**: Grafana precisa de `* 1000` para formatar data.
4. **É o timestamp da AMOSTRA** (hora do scrape), não do evento real: com scrape de 60s, erro de até 60s.
5. **Só enxerga dentro da janela**: se o mínimo "de verdade" foi antes da janela, você recebe o mínimo **da janela**.
6. **Histogramas são ignorados**.

---

## 🎓 Na prova PCA

Funções `ts_of_*` são **experimentais** (surgiram no Prometheus 3.x); a prova tende a cobrir o conceito vizinho: `timestamp()`, `time()`, `min_over_time()`. Saiba:
- Range vector → instant vector, **por série**; valor = timestamp Unix em **segundos**.
- `time()` = instante de avaliação; `time() - <timestamp>` = idade em segundos.
- Empate → **última** amostra.

**1.** Qual expressão retorna "há quantos segundos o espaço livre em disco atingiu o menor valor da última hora"?

- A) `min_over_time(node_filesystem_avail_bytes[1h]) - time()`
- B) `time() - ts_of_min_over_time(node_filesystem_avail_bytes[1h])`
- C) `timestamp(min_over_time(node_filesystem_avail_bytes[1h]))`
- D) `ts_of_min_over_time(node_filesystem_avail_bytes[1h]) - time()`

<details><summary>Resposta</summary>

**B.** `ts_of_min_over_time` dá o instante do mínimo; `time()` menos ele dá a idade (positiva). D daria negativo. C) `timestamp()` de um resultado de função retorna o instante de **avaliação**, não o do mínimo.
</details>

**2.** Na janela, o valor mínimo 0 aparece em 3 amostras (t=100, t=105, t=110). O que `ts_of_min_over_time` retorna?

- A) 100
- B) 105
- C) 110
- D) 0

<details><summary>Resposta</summary>

**C.** A documentação define: timestamp da **última** amostra com o valor mínimo.
</details>

**3.** Em que unidade vem o resultado de `ts_of_min_over_time`?

- A) milissegundos desde 1970
- B) segundos desde 1970
- C) segundos desde o início da janela
- D) mesma unidade da métrica

<details><summary>Resposta</summary>

**B.** Timestamps em PromQL são segundos Unix (com fração), como em `time()` e `timestamp()`.
</details>

**4.** Qual condição é necessária para usar `ts_of_min_over_time` no Prometheus 3.x?

- A) Nenhuma, é estável.
- B) `--enable-feature=promql-experimental-functions`
- C) `--enable-feature=native-histograms`
- D) Só funciona em recording rules.

<details><summary>Resposta</summary>

**B.** Todas as `ts_of_*_over_time` (e `mad_over_time`) são experimentais.
</details>

---

## 📝 Cola rápida

- `ts_of_min_over_time(x[w])` = **quando** (timestamp Unix em s) foi o mínimo da janela; **experimental**.
- Empate → a **última** amostra com o valor mínimo.
- `time() - ts_of_min_over_time(x[w])` = há quantos segundos foi o mínimo.
- Grafana: `* 1000` + unidade `dateTimeAsIso`.
- Par natural: `min_over_time` (o quê) + `ts_of_min_over_time` (quando).

## 🔗 Relacionadas

[`min_over_time()`](../min_over_time/) · [`ts_of_max_over_time()`](../ts_of_max_over_time/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/) · [`ts_of_first_over_time()`](../ts_of_first_over_time/) · [`timestamp()`](../timestamp/) · [`time()`](../time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
