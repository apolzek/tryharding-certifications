# `abs()`: o tamanho do desvio, sem o sinal

> **Em uma frase:** `abs(v)` troca cada valor pelo seu **valor absoluto**: `-0.12` vira `0.12`, `3` continua `3`. Serve para medir **o quanto** algo se afastou do alvo, **não importa a direção**.

| | |
|---|---|
| **Assinatura** | `abs(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (diferenças, desvios) · ⚠️ Counter cru não faz sentido (já é ≥ 0) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada (segundos, °C, bytes...) |
| **Dashboard** | http://localhost:3300/d/fn-abs |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a régua do alvo

Você está jogando dardos. Um dardo caiu **2 cm à esquerda** do centro, outro **2 cm à direita**. Qual foi o erro de cada um? **2 cm**. A régua não tem sinal: ela mede **distância**.

```
      atrasado        alvo       adiantado
  ─────●────────────────┼────────────●─────
     -0.12s             0          +0.08s

  abs(-0.12) = 0.12     abs(+0.08) = 0.08
```

- O valor com sinal responde **"para que lado?"** (adiantado ou atrasado, acima ou abaixo do setpoint).
- O `abs()` responde **"quão longe?"**, que é o que um alerta de tolerância geralmente quer saber.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `abs_node_timex_offset_seconds{node="node-a"}` | gauge | onda entre **-0.08s e +0.08s** (período 4 min) |
| `abs_node_timex_offset_seconds{node="node-b"}` | gauge | onda entre **-0.03s e +0.03s** (sempre dentro da tolerância) |
| `abs_node_timex_offset_seconds{node="node-c"}` | gauge | parado em **-0.12s** (sempre **atrasado**) |
| `abs_temperature_celsius{room="datacenter"}` | gauge | onda **22 ± 3 °C** (período 3 min) |
| `abs_temperature_setpoint_celsius{room="datacenter"}` | gauge | constante **22 °C** (o alvo do ar-condicionado) |

```bash
curl -s localhost:8088/metrics | grep '^abs_'
# abs_node_timex_offset_seconds{node="node-a"} 0.0561
# abs_node_timex_offset_seconds{node="node-c"} -0.1204
# abs_temperature_celsius{room="datacenter"} 24.1
```

## ▶️ Como rodar

```bash
# na raiz do projeto
tools/deploy.sh
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-abs
```

Espere **~1 minuto** para os gráficos 1 a 5 e **~3 minutos** para o painel 6 (média de 3 min).

---

## 🔍 Queries passo a passo

### 1. O drift cru (com sinal)

```promql
abs_node_timex_offset_seconds
```

**O que faz:** mostra o desvio do relógio de cada nó em relação ao NTP.
**Resultado esperado:** node-a é uma onda grande (±0.08s), node-b uma onda pequena (±0.03s) e node-c uma **linha reta lá embaixo em -0.12s**. Olhando rápido, o node-c parece o "menor" valor, mas ele é o **pior** nó.

---

### 2. `abs()` do drift

```promql
abs(abs_node_timex_offset_seconds)
```

**O que faz:** remove o sinal de cada amostra, série por série. Os labels são mantidos (o nome da métrica é removido, como em toda função matemática).
**Resultado esperado:**

| node | cru | `abs()` |
|---|---|---|
| node-a | de -0.08 a +0.08 | "lombadas" de **0 a 0.08** (a onda é "rebatida" para cima) |
| node-b | de -0.03 a +0.03 | lombadas de **0 a 0.03** |
| node-c | **-0.12** | **0.12** → agora é a linha mais **alta** |

---

### 3 e 4. O alerta que não dispara × o alerta certo

```promql
abs_node_timex_offset_seconds > 0.05         # ingênuo
abs(abs_node_timex_offset_seconds) > 0.05    # correto
```

**Resultado esperado:**
- **Ingênuo:** só aparecem pontos do **node-a**, e só na metade da onda em que ele está **adiantado**. O **node-c**, 120 ms atrasado, **nunca** aparece. Um alerta assim deixaria o nó mais problemático passar despercebido.
- **Correto:** o **node-c aparece o tempo todo** (0.12) e o node-a aparece nas **duas** metades da onda (adiantado e atrasado). O node-b nunca passa de 0.03, então nunca aparece.

> 💡 É exatamente assim que alertas de NTP são escritos na vida real: `abs(node_timex_offset_seconds) > 0.05`.

---

### 5. Erro em relação a um setpoint

```promql
abs_temperature_celsius - abs_temperature_setpoint_celsius         # com sinal
abs(abs_temperature_celsius - abs_temperature_setpoint_celsius)    # distância até o alvo
```

**O que faz:** a subtração casa as séries pelo label `room` e dá o erro com sinal. O `abs()` transforma esse erro em "quantos graus longe de 22 °C".
**Resultado esperado:** com sinal, uma onda entre **-3 e +3**. Com `abs()`, "lombadas" entre **0 e 3**, que tocam o zero cada vez que a temperatura passa por 22 °C (a cada 90 s).

---

### 6. Erro médio: por que o sinal engana

```promql
avg_over_time((abs_temperature_celsius - abs_temperature_setpoint_celsius)[3m:5s])
avg_over_time(abs(abs_temperature_celsius - abs_temperature_setpoint_celsius)[3m:5s])
```

**O que faz:** calcula a média do erro nos últimos 3 minutos (uma volta completa da onda), usando uma subquery `[3m:5s]` porque o `avg_over_time` precisa de um range vector.
**Resultado esperado:**
- Média do erro **com sinal**: ≈ **0 °C**. Os +3 e os -3 se cancelam, e o painel "diz" que o ar-condicionado é perfeito.
- Média do **`abs(erro)`** (o famoso **MAE**, *mean absolute error*): ≈ **1.9 °C** (o painel mostra 1.87 a 1.91 por causa da amostragem a cada 5 s) (para uma senóide de amplitude 3, a média do valor absoluto é `3 × 2/π ≈ 1.91`). Esse é o erro que as pessoas **sentem** na sala.

---

### 7. Casos especiais

| Query | Resultado |
|---|---|
| `abs(vector(-5.5))` | **5.5** |
| `abs(vector(3))` | **3** (positivos não mudam) |
| `abs(vector(-0))` | **0** |
| `abs(vector(-Inf))` | **+Inf** |
| `abs(vector(NaN))` | **NaN** (NaN não tem sinal para tirar) |

---

## 🏭 Casos reais

### 1. Alerta de relógio fora de sincronia (node_exporter)

Relógios dessincronizados quebram TLS, tokens JWT, ordenação de logs e o próprio Prometheus (amostras "do futuro" são rejeitadas). O `node_exporter` expõe `node_timex_offset_seconds`, que pode ser **positivo ou negativo**:

```yaml
groups:
  - name: node-time
    rules:
      - alert: NodeClockSkewDetected
        expr: abs(node_timex_offset_seconds) > 0.05
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Relógio de {{ $labels.instance }} está {{ $value | humanizeDuration }} fora do NTP"
```

**Decisão:** sem o `abs()`, um nó **atrasado** (offset negativo) nunca dispararia, que é o caso do `node-c` neste lab. O `for: 10m` evita alertar em correções momentâneas do NTP.
(O `node-mixin` oficial usa uma variação: `(node_timex_offset_seconds > 0.05 and deriv(node_timex_offset_seconds[5m]) >= 0) or (node_timex_offset_seconds < -0.05 and deriv(node_timex_offset_seconds[5m]) <= 0)`, que só alerta se o desvio **não está sendo corrigido**. Repare que ela trata os dois sinais separadamente, que é exatamente o que o `abs()` resume.)

> No lab a métrica usa o label `node` e não `instance`, porque `instance` já é colocado pelo Prometheus no alvo raspado (o gerador).

### 2. Desbalanceamento entre zonas de disponibilidade

O SRE do e-commerce quer saber se o load balancer está distribuindo o tráfego igualmente entre duas AZs. Não importa **qual** AZ recebe mais, só **quanto** de diferença:

```promql
abs(
    sum(rate(http_requests_total{zone="sa-east-1a"}[5m]))
  - sum(rate(http_requests_total{zone="sa-east-1b"}[5m]))
)
/ sum(rate(http_requests_total[5m])) > 0.2      # diferença > 20% do total
```

### 3. Variação brusca de temperatura do hardware

```yaml
- alert: TemperaturaMudouBruscamente
  expr: abs(delta(node_hwmon_temp_celsius[5m])) > 10
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Sensor {{ $labels.chip }}/{{ $labels.sensor }} de {{ $labels.instance }} variou {{ $value }} °C em 5 min"
```

Uma queda de 10 °C em 5 minutos (ventoinha "voltou" ou sensor com defeito) é tão interessante quanto uma subida. `delta()` dá a variação com sinal; `abs()` pega as duas direções. É o mesmo raciocínio do painel 5 (erro em relação ao setpoint).

### 4. Relógio do alvo × relógio do Prometheus

```promql
abs(node_time_seconds - timestamp(node_time_seconds)) > 1
```

`node_time_seconds` é a hora **do servidor monitorado**; `timestamp()` é a hora em que o **Prometheus** fez o scrape. A diferença pode ter qualquer sinal, então o `abs()` é obrigatório.

## ✅ Quando usar

- **Drift / offset de relógio:** `abs(node_timex_offset_seconds) > 0.05`.
- **Desvio de um setpoint:** temperatura, pressão, tensão elétrica fora de uma faixa em torno do alvo.
- **Diferença entre réplicas ou ambientes:** `abs(rows_in_primary - rows_in_replica) > 100` (não importa quem está "na frente").
- **Diferença entre dois períodos:** `abs(rate(x[5m]) - rate(x[5m] offset 1d))` para detectar mudança de comportamento em qualquer direção.
- **Métricas de erro:** MAE de uma previsão (`abs(previsto - real)`).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Você quer saber **a direção** (subindo/descendo, adiantado/atrasado) | o valor cru, ou [`sgn()`](../sgn/) |
| Quer **forçar** valores negativos a virarem 0 (e não positivos) | [`clamp_min(x, 0)`](../clamp_min/) |
| Quer a **variação** de um gauge numa janela | [`delta()`](../delta/) / [`deriv()`](../deriv/) (e depois `abs()` se o sinal não importar) |
| O dado é counter cru | [`rate()`](../rate/) — counter já é ≥ 0, `abs()` não faz nada |

## ⚠️ Pegadinhas

1. **Média antes do `abs()`:** `abs(avg(erro))` ≠ `avg(abs(erro))`. O primeiro cancela os sinais e dá ≈ 0; o segundo dá o erro real (painel 6).
2. **`abs()` esconde a direção:** um alerta com `abs()` não diz se o relógio está adiantado ou atrasado. Coloque o valor cru na anotação do alerta.
3. **Não "conserta" counter com reset:** `abs(delta(counter[5m]))` numa queda de reset dá um número positivo **sem sentido**. Para counters, use [`rate()`](../rate/)/[`increase()`](../increase/).
4. **Nome da métrica some:** como em toda função matemática, o resultado não tem mais `__name__`. Os outros labels ficam.
5. **Histogramas nativos são ignorados em silêncio:** `abs()` só age em amostras float.

## 🎓 Na prova PCA

O que costuma cair:
- `abs()` recebe **instant vector** e devolve **instant vector**. `abs(-3)` é **erro de parse** (é escalar); use `abs(vector(-3))`.
- Funções matemáticas **removem o nome da métrica** (`__name__`) e mantêm os outros labels.
- Saber **quando** o sinal importa: alertas de desvio em torno de zero (offset, diferença entre réplicas, `delta()` de gauge) quase sempre precisam de `abs()`.
- Ordem de agregação: `avg(abs(x))` ≠ `abs(avg(x))`.

**1.** Qual expressão alerta quando o relógio de um nó está mais de 50 ms fora do NTP, **para qualquer lado**?

- A) `node_timex_offset_seconds > 0.05`
- B) `abs(node_timex_offset_seconds) > 0.05`
- C) `abs(rate(node_timex_offset_seconds[5m])) > 0.05`
- D) `sgn(node_timex_offset_seconds) > 0.05`

<details><summary>Resposta</summary>

**B.** A) só pega relógios adiantados. C) usa `rate()` num **gauge** (errado) e mede velocidade, não desvio. D) `sgn()` vale 1 para **qualquer** offset positivo (até 1 ns) e -1 para os negativos: ignora o tamanho e ainda perde os atrasados.
</details>

**2.** O que acontece ao executar `abs(-5)`?

- A) Retorna o escalar `5`
- B) Retorna um instant vector com um elemento de valor `5`
- C) Erro: `abs` espera um instant vector e recebeu um escalar
- D) Retorna vazio

<details><summary>Resposta</summary>

**C.** A assinatura é `abs(v instant-vector)`. Para testar com números literais, use `abs(vector(-5))`, que retorna um instant vector sem labels com valor 5.
</details>

**3.** Dado `erro{sala="a"} = -2` e `erro{sala="b"} = 2`, quanto valem `avg(abs(erro))` e `abs(avg(erro))`?

- A) 2 e 2
- B) 2 e 0
- C) 0 e 2
- D) 0 e 0

<details><summary>Resposta</summary>

**B.** `avg(abs(erro)) = avg(2, 2) = 2` (erro médio absoluto). `abs(avg(erro)) = abs(0) = 0`: a média com sinal cancela os desvios.
</details>

**4.** A query `abs(http_request_duration_seconds_bucket)` retorna séries com qual nome de métrica?

- A) `http_request_duration_seconds_bucket`
- B) `abs_http_request_duration_seconds_bucket`
- C) nenhum: o `__name__` é removido, os demais labels (inclusive `le`) ficam
- D) nenhum, e todos os labels são removidos

<details><summary>Resposta</summary>

**C.** Funções que transformam valores (abs, ceil, sqrt, ln...) descartam o nome da métrica, porque o valor não tem mais o mesmo significado. Os outros labels são preservados.
</details>

## 📝 Cola rápida

- `abs(v instant-vector)` → instant vector; tira o sinal; nome da métrica é removido.
- `abs(-3)` = erro de tipo; `abs(vector(-3))` = 3. `abs(-Inf) = +Inf`, `abs(NaN) = NaN`.
- Alerta de desvio "pros dois lados": `abs(x - alvo) > limite` (ex.: `abs(node_timex_offset_seconds) > 0.05`).
- `avg(abs(x))` = erro médio real; `abs(avg(x))` cancela os sinais.
- Quer direção? `sgn()`. Quer zerar negativos? `clamp_min(x, 0)`.

## 🔗 Relacionadas

[`sgn()`](../sgn/) · [`clamp_min()`](../clamp_min/) · [`clamp()`](../clamp/) · [`delta()`](../delta/) · [`deriv()`](../deriv/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#abs
