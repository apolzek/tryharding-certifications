# `sgn()`: só o sinal (+1, 0 ou -1)

> **Em uma frase:** `sgn(v)` troca cada valor pelo seu **sinal**: `1` se for positivo, `-1` se for negativo e `0` se for zero. Ela descarta o **tamanho** e guarda só a **direção**.

| | |
|---|---|
| **Assinatura** | `sgn(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (principalmente `deriv()`, `delta()`, diferenças entre períodos) · histogramas são ignorados |
| **Unidade do resultado** | nenhuma: só `-1`, `0`, `1` (ou `NaN`) |
| **Dashboard** | http://localhost:3300/d/fn-sgn |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a setinha do placar da bolsa

No painel da bolsa, ao lado de cada ação tem uma **setinha**: ▲ verde, ▼ vermelha ou ▬ cinza. Ela não diz **quanto** a ação subiu, só **para que lado** foi. É exatamente o `sgn()`:

```
 valor:    +4.17   +0.02    0    -0.03   -4.17
 sgn:        1       1      0     -1      -1
            ▲       ▲      ▬      ▼       ▼
```

Repare no `+0.02` e no `-0.03`: a setinha **não distingue** "subiu um tiquinho" de "subiu muito". Isso é poderoso (simplifica) e perigoso (amplifica ruído), e as duas coisas aparecem neste lab.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `sgn_kafka_consumergroup_lag{consumergroup="billing",topic="orders"}` | `kafka_consumergroup_lag` (kafka_exporter) | gauge | triângulo: **0 → 500 em 2 min** (atrasando) e **500 → 0 em 2 min** (recuperando) |
| `sgn_kafka_consumergroup_lag{consumergroup="emails",...}` | idem | gauge | parado em **42** |
| `sgn_kafka_consumergroup_lag{consumergroup="payments",...}` | idem | gauge | **~100 com ruído de ±2** (estável, mas "tremendo") |
| `sgn_http_requests_total{service="api"}` | `http_requests_total` | counter | velocidade em onda **100 ± 50 req/s**, período de **10 min** |

```bash
curl -s localhost:8088/metrics | grep '^sgn_'
# sgn_kafka_consumergroup_lag{consumergroup="billing",topic="orders"} 312
# sgn_http_requests_total{service="api"} 48213
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sgn
```

Espere **~2 minutos** para os painéis 1 a 4 (o `deriv(...[1m])` precisa de 1 min de dados) e **~6 min** para os painéis 5 e 6 (o `offset 5m` precisa de 5 min de histórico).

---

## 🔍 Queries passo a passo

### 1 e 2. O lag e a sua velocidade

```promql
sgn_kafka_consumergroup_lag
deriv(sgn_kafka_consumergroup_lag[1m])
```

**O que faz:** [`deriv()`](../deriv/) calcula a inclinação (msgs/s) do lag de cada consumer group no último minuto, por regressão linear. É a função certa para **gauges** (não use `rate()` em lag).
**Resultado esperado:**

| consumergroup | `deriv` |
|---|---|
| billing | ≈ **+4.17** atrasando (500 msgs / 120 s), ≈ **-4.17** recuperando |
| emails | exatamente **0** |
| payments | valores pequenos em volta de 0 (ex.: +0.03, -0.05) por causa do ruído |

---

### 3. A tendência: `sgn(deriv(...))`

```promql
sgn(deriv(sgn_kafka_consumergroup_lag[1m]))
```

**Resultado esperado:**
- **billing:** uma **onda quadrada** alternando entre **+1** (atrasando) e **-1** (recuperando) a cada 2 min.
- **emails:** reta em **0**.
- **payments:** ⚠️ **pisca** entre +1 e -1 sem parar. O lag está estável, mas o ruído deixa a inclinação levemente positiva ou negativa, e o `sgn()` transforma "+0.01" em "+1", **amplificando** o ruído até o tamanho máximo.

---

### 4. Tendência com "zona morta"

```promql
sgn(deriv(sgn_kafka_consumergroup_lag[1m]))
  * (abs(deriv(sgn_kafka_consumergroup_lag[1m])) > bool 0.5)
```

**O que faz:** `abs(deriv) > bool 0.5` vale `1` quando o lag anda mais rápido que 0.5 msg/s e `0` caso contrário (o modificador `bool` faz a comparação **devolver 0/1** em vez de filtrar). Multiplicando pelo sinal, velocidades pequenas viram **0** ("estável").
**Resultado esperado:** billing continua **+1/-1**, emails **0**, e payments agora fica **quieto em 0**. É assim que se escreve um indicador de tendência que não "pisca".

---

### 5 e 6. Mais ou menos tráfego que antes? (`offset`)

```promql
rate(sgn_http_requests_total[1m])                    # agora
rate(sgn_http_requests_total[1m] offset 5m)          # 5 minutos atrás
sgn(rate(sgn_http_requests_total[1m]) - rate(sgn_http_requests_total[1m] offset 5m))
```

**O que faz:** o modificador `offset 5m` desloca a janela para o passado. A diferença "agora − antes" é positiva se o tráfego cresceu e negativa se caiu; o `sgn` resume isso em ▲/▼.
**Resultado esperado:** no painel 5, duas ondas entre **50 e 150 req/s**, uma "atrasada" meia volta em relação à outra. No painel 6, uma **onda quadrada** entre **+1** e **-1** que troca de lado a cada ~5 min.

> 💡 Em produção, o mesmo padrão com `offset 1w` responde "o tráfego está acima ou abaixo do mesmo horário da semana passada?".

---

### 7. Contar quem está atrasando

```promql
count(sgn(deriv(sgn_kafka_consumergroup_lag[1m])) == 1) or vector(0)
```

**Resultado esperado:** **0, 1 ou 2** (billing quando está subindo; payments quando o ruído está pra cima). O `or vector(0)` garante um **0** em vez de "sem dados" quando nenhum grupo passa no filtro.

---

### 8. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `sgn(vector(42))` | **1** | positivo |
| `sgn(vector(-0.0001))` | **-1** | qualquer negativo, por menor que seja |
| `sgn(vector(0))` | **0** | zero |
| `sgn(vector(-Inf))` | **-1** | infinitos têm sinal |
| `sgn(vector(NaN))` | **NaN** | NaN não é nem positivo nem negativo |

---

## 🏭 Casos reais

### 1. "O consumidor está alcançando?" (Kafka)

Durante um incidente, o on-call quer saber se o consumer group está **se recuperando** depois de escalar os consumidores. O valor do lag (ex.: 2 milhões) importa menos do que a **direção**:

```promql
sgn(deriv(sum by (consumergroup) (kafka_consumergroup_lag)[10m:]))
```

Um painel "stat" com mapeamento de valores (`1 → ▲ atrasando`, `0 → ▬ estável`, `-1 → ▼ recuperando`) responde de relance. O alerta, por sua vez, usa o **tamanho** e a **direção**:

```yaml
- alert: KafkaLagCrescendo
  expr: |
    sum by (consumergroup) (kafka_consumergroup_lag) > 10000
      and
    deriv(sum by (consumergroup) (kafka_consumergroup_lag)[10m:]) > 0
  for: 15m
  labels:
    severity: warning
```

**Decisão:** no alerta, `deriv(...) > 0` é equivalente a `sgn(deriv(...)) == 1` e mais legível; o `sgn` brilha no **dashboard**.

### 2. Comparação com a semana passada (week-over-week)

```promql
sgn(
    sum(rate(http_requests_total{service="checkout"}[1h]))
  - sum(rate(http_requests_total{service="checkout"}[1h] offset 1w))
)
```

```yaml
- record: service:requests_trend_wow:sgn
  expr: |
    sgn(
        sum by (service) (rate(http_requests_total[1h]))
      - sum by (service) (rate(http_requests_total[1h] offset 1w))
    )
```

O time de produto quer uma setinha no painel executivo: "o checkout está com mais ou menos tráfego que na mesma hora da semana passada?". Os painéis 5 e 6 do lab fazem exatamente isso, com `offset 5m` para caber na aula.

### 3. Tendência de disco com zona morta

```promql
sgn(deriv(node_filesystem_avail_bytes{mountpoint="/"}[1h]))
  * (abs(deriv(node_filesystem_avail_bytes{mountpoint="/"}[1h])) > bool 1e5)
```

Espaço livre caindo (`-1`), subindo (`+1`, limpeza de logs) ou estável (`0`, variação < 100 kB/s). A zona morta evita que pequenas escritas façam a setinha piscar (painel 4).

### 4. Reaplicar o sinal depois de transformar o tamanho

```promql
sgn(node_timex_offset_seconds) * log10(abs(node_timex_offset_seconds) * 1e6)
```

Para plotar offsets de relógio que variam de microssegundos a segundos numa escala log **sem perder o lado** (adiantado/atrasado): `log10` só aceita positivos, então tira-se o sinal com `abs`, transforma-se, e devolve-se o sinal com `sgn`.

---

## ✅ Quando usar

- **Indicador de tendência** em dashboards: `sgn(deriv(x[10m]))` (com zona morta).
- **Direção de mudança** comparando com o passado: `sgn(rate(x[1h]) - rate(x[1h] offset 1w))`.
- **Contar por direção:** `count_values("sinal", sgn(delta(estoque[1h])))` → quantos subiram/desceram.
- **Reaplicar o sinal** depois de mexer no tamanho: `sgn(x) * floor(abs(x))` (truncar), `sgn(x) * sqrt(abs(x))` (comprimir escala mantendo o sinal).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| O **tamanho** importa (quão rápido cresce?) | o valor cru de [`deriv()`](../deriv/) / [`delta()`](../delta/) |
| Quer o tamanho sem o sinal | [`abs()`](../abs/) |
| Só quer filtrar positivos/negativos (alertas) | comparação direta: `x > 0`, `x < 0` |
| Quer transformar negativos em 0 | [`clamp_min(x, 0)`](../clamp_min/) |

## ⚠️ Pegadinhas

1. **`sgn` amplifica ruído:** `+0.001` vira `+1`. Use zona morta (`abs(x) > bool limite`) ou janelas maiores no `deriv`.
2. **Zero exato é raro com floats:** uma série "estável" com ruído quase nunca dá `0`. Na prática, `0` só aparece em séries realmente constantes (como `emails`).
3. **`sgn(NaN) = NaN`**, não 0. Se a divisão de origem pode dar NaN (0/0), trate antes.
4. **Não use em counter cru:** o counter é sempre ≥ 0, então `sgn()` dá 1 (ou 0). Para tendência de tráfego, compare **taxas** (`rate`) como no painel 6.
5. **`deriv` × `rate`:** tendência de **gauge** (lag, disco, fila) é com `deriv()`; `rate()` é só para counters.
6. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- `sgn()` recebe **instant vector** e retorna `-1`, `0`, `1` (ou `NaN`).
- Combinações clássicas: `sgn(deriv(gauge[5m]))`, `sgn(x - x offset 1w)`.
- Diferença entre comparação que **filtra** (`x > 0`) e comparação com **`bool`** (`x > bool 0`, devolve 0/1), usada para zonas mortas.
- `rate` é para counter; `deriv` para gauge.

**1.** Qual expressão indica se o **espaço livre** do disco está aumentando (+1), diminuindo (-1) ou estável (0)?

- A) `sgn(rate(node_filesystem_avail_bytes[1h]))`
- B) `sgn(deriv(node_filesystem_avail_bytes[1h]))`
- C) `sgn(node_filesystem_avail_bytes)`
- D) `deriv(sgn(node_filesystem_avail_bytes)[1h:])`

<details><summary>Resposta</summary>

**B.** `node_filesystem_avail_bytes` é **gauge**, então a inclinação vem de `deriv()`. A) `rate()` em gauge trata quedas como resets. C) o espaço livre é sempre positivo → sempre 1. D) `sgn` do valor cru é constante 1, a derivada é 0.
</details>

**2.** Quanto vale `sgn(vector(-0.000001))`?

- A) 0
- B) -1
- C) -0.000001
- D) NaN

<details><summary>Resposta</summary>

**B.** Qualquer valor negativo, por menor que seja, vira -1.
</details>

**3.** Qual é o resultado de `x > bool 0.5` quando `x = 0.2`?

- A) A série é removida do resultado
- B) `0.2`
- C) `0`
- D) `false`

<details><summary>Resposta</summary>

**C.** Com o modificador `bool`, a comparação **não filtra**: devolve `1` (verdadeiro) ou `0` (falso). Sem `bool`, a série seria removida (A). É o que permite montar a "zona morta" `sgn(d) * (abs(d) > bool 0.5)`.
</details>

**4.** `sgn(vector(NaN))` retorna:

- A) 0
- B) 1
- C) NaN
- D) resultado vazio

<details><summary>Resposta</summary>

**C.** NaN não é positivo, negativo nem zero; o sinal de NaN é NaN.
</details>

## 📝 Cola rápida

- `sgn(v instant-vector)` → `1` (positivo), `-1` (negativo), `0` (zero), `NaN` (NaN).
- Tendência de gauge: `sgn(deriv(x[5m]))`. Comparação com o passado: `sgn(rate(x[1h]) - rate(x[1h] offset 1w))`.
- `sgn` amplifica ruído → zona morta com `* (abs(d) > bool limite)`.
- `x > 0` filtra; `x > bool 0` devolve 0/1.
- Tamanho sem sinal → `abs()`; sinal sem tamanho → `sgn()`; `x = sgn(x) * abs(x)`.

## 🔗 Relacionadas

[`abs()`](../abs/) · [`deriv()`](../deriv/) · [`delta()`](../delta/) · [`clamp_min()`](../clamp_min/) · [`floor()`](../floor/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#sgn
