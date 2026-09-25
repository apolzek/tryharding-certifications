# `end()`: o fim da janela do gráfico (experimental)

> **Em uma frase:** `end()` devolve o **timestamp de fim** (segundos Unix, UTC) da consulta *range* em andamento, ou seja, a **borda direita do gráfico** (normalmente "agora"). É o mesmo número em todos os pontos. Numa consulta **instantânea**, `end()` = instante avaliado (= `time()`).

| | |
|---|---|
| **Assinatura** | `end() → scalar` |
| **Status** | 🧪 **experimental**: exige `--enable-feature=promql-experimental-functions` |
| **Tipo de métrica** | nenhuma. Brilha com `@`: `x @ end()`, `max_over_time(x[range()] @ end())` |
| **Unidade do resultado** | segundos Unix (UTC) |
| **Dashboard** | http://localhost:3300/d/fn-end |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o último quadro do filme

Como em [`start()`](../start/): o gráfico é um **filme** com um quadro por step. `end()` é o horário do **último quadro**.

Por que isso é útil? Porque com o modificador `@` você pode "pintar" o **final do filme** em todos os quadros:
- `x @ end()` → o **valor atual** desenhado como linha reta no gráfico inteiro ("como estamos agora em relação ao passado?").
- `max_over_time(x[range()] @ end())` → o **máximo de tudo que aparece na tela**, como linha reta.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `end_temperature_celsius{room="datacenter"}` | gauge | **24 ± 4 °C**, onda de 7 min |
| `end_temperature_celsius{room="escritorio"}` | gauge | **21 ± 2 °C**, onda de 5 min |

```bash
curl -s localhost:8088/metrics | grep '^end_'
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-end
```

---

## 🔍 Queries passo a passo

### 1. Contagem regressiva até a borda direita

```promql
end() - time()
```

**Resultado esperado:** rampa **descendo** de ~**900** (borda esquerda, janela de 15 min) até **0** (borda direita). É o espelho de `time() - start()`.

---

### 2. Valor atual como linha de referência

```promql
end_temperature_celsius{room="datacenter"}            # a onda
end_temperature_celsius{room="datacenter"} @ end()    # valor no último ponto, repetido
```

**Resultado esperado:** a onda do datacenter (entre ~20 e ~28 °C) e uma **linha reta** na temperatura **atual**. A linha encosta na onda exatamente na borda direita.

---

### 3. Diferença para o valor atual

```promql
end_temperature_celsius - end_temperature_celsius @ end()
```

**O que faz:** quanto cada ponto do passado estava **acima (+)** ou **abaixo (−)** do valor de agora.
**Resultado esperado:** duas ondas que **terminam em 0** na borda direita. A do datacenter oscila até ±8 °C, a do escritório até ±4 °C.

> Repare: o `@ end()` aqui está num seletor **sem** filtro de `room`. Cada série é casada com a sua própria versão `@ end()` pelos labels (one-to-one).

---

### 4. Máximo e mínimo da janela inteira

```promql
max_over_time(end_temperature_celsius{room="datacenter"}[range()] @ end())
min_over_time(end_temperature_celsius{room="datacenter"}[range()] @ end())
```

**O que faz:** `[range()]` é uma janela do tamanho do gráfico (900 s em 15 min); `@ end()` ancora essa janela no final. Resultado: o máximo/mínimo de **tudo que está visível**, como linhas retas.
**Resultado esperado:** duas linhas retas encostando no pico (~**28 °C**) e no vale (~**20 °C**) da onda. Mude o zoom para 5 min: as linhas se aproximam (menos da onda cabe na tela).

---

### 5. Numa consulta instantânea

```promql
end() - time()     # 0
```

**Resultado esperado:** **0**. Sem janela, `end()` é o próprio instante.

---

### 6. ❌ O que dá errado: `@` no lugar errado

```promql
max_over_time(end_temperature_celsius[range()]) @ end()    # ERRO de parse
max_over_time(end_temperature_celsius[range()] @ end())    # certo
```

O `@` precisa vir **logo depois de um seletor** (instant ou range) ou de uma subquery, nunca depois de uma função: *"@ modifier must be preceded by an instant vector selector or range vector selector or a subquery"*.

---

## 🏭 Casos reais

### 1. Dashboard de capacidade: "o pico da semana" como linha de referência

No painel de CPU do cluster, o time de capacidade quer uma linha com o **pico visível** para comparar com a linha atual, qualquer que seja o zoom:

```promql
sum(rate(node_cpu_seconds_total{mode!="idle"}[5m]))
max_over_time(sum(rate(node_cpu_seconds_total{mode!="idle"}[5m]))[range():] @ end())
```

(A segunda usa uma **subquery** `[range():]` porque dentro há uma expressão, não um seletor.)

Para alertas, a mesma ideia com janela fixa, via recording rule:

```yaml
groups:
  - name: capacity
    rules:
      - record: cluster:cpu_used_cores:rate5m
        expr: sum(rate(node_cpu_seconds_total{mode!="idle"}[5m]))
      - alert: CpuAcimaDoPicoDaSemana
        expr: cluster:cpu_used_cores:rate5m > 1.2 * max_over_time(cluster:cpu_used_cores:rate5m[7d] offset 1h)
        for: 30m
```

### 2. "Quanto a latência caiu depois do deploy?"

Com a janela do Grafana começando antes do deploy, o painel mostra o p99 relativo ao valor atual:

```promql
histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))
  / scalar(histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m] @ end()))))
```

Linha em 1,0 = igual a agora; 1,5 = 50% mais lento que agora.

Regra equivalente (compara com 1 h atrás, janela fixa):

```yaml
- alert: LatenciaPiorQueHaUmaHora
  expr: |
    histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))
      > 1.5 * histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m] offset 1h)))
  for: 15m
```

---

## ✅ Quando usar

- Linhas de referência em dashboards: valor atual (`@ end()`), máximo/mínimo/média visível (`[range()] @ end()`).
- "Diferença para agora" ao investigar incidentes.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Alertas e recording rules (instantâneas) | janelas fixas: `max_over_time(x[7d])`, `offset` |
| Horário do ponto atual | [`time()`](../time/) |
| Produção estável (experimental) | variáveis do Grafana `$__to`, `$__range` |

## ⚠️ Pegadinhas

1. **Experimental:** precisa de `--enable-feature=promql-experimental-functions`.
2. **Em instant query `end()` = `time()`**: `x - x @ end()` = 0 em stat/alerta.
3. **`@` só depois de seletor/subquery**, nunca depois de função.
4. **`[range()]` em instant query** = `[0s]` → erro *"duration must be greater than 0"*.
5. **Grafana alinha `end`** ao step; a borda direita pode estar alguns segundos antes de "agora".

## 🎓 Na prova PCA

`end()` é experimental; o que cai de verdade é o que está em volta:
- **Modificador `@`** e onde ele pode aparecer.
- **Subqueries** (`expr[range:resolution]`) para aplicar `_over_time` a expressões.
- Consultas **instant vs range** e seus parâmetros.

**1.** Qual expressão é **válida**?
- A) `rate(http_requests_total[5m]) @ 1700000000`
- B) `rate(http_requests_total[5m] @ 1700000000)`
- C) `rate(@ 1700000000 http_requests_total[5m])`
- D) `@1700000000 rate(http_requests_total[5m])`

<details><summary>Resposta</summary>

**B.** O `@` vem logo após o seletor (range selector aqui).
</details>

**2.** Para calcular `max_over_time` sobre `sum(rate(x[5m]))` na última hora, você precisa de:
- A) `max_over_time(sum(rate(x[5m]))[1h])`
- B) `max_over_time(sum(rate(x[5m]))[1h:])`
- C) `max(sum(rate(x[5m])))[1h]`
- D) `max_over_time(x[1h])`

<details><summary>Resposta</summary>

**B.** Uma expressão (não seletor) precisa de **subquery** `[1h:]` (resolução padrão = intervalo de avaliação global) para virar range vector.
</details>

**3.** Num painel *stat* (consulta instantânea), quanto vale `end() - time()`?
- A) a duração do painel
- B) 0
- C) vazio
- D) o step

<details><summary>Resposta</summary>

**B.** Instant query: `end()` = instante avaliado = `time()`.
</details>

## 📝 Cola rápida

- `end()` = borda **direita** do gráfico; em instant query = `time()`.
- `x @ end()` → valor atual como linha reta; `x - x @ end()` → diferença para agora.
- Máx./mín. da tela: `max_over_time(x[range()] @ end())` (expressão → subquery `[range():]`).
- `@` só logo após seletor/subquery. Experimental. Nunca em alertas.

## 🔗 Relacionadas

[`start()`](../start/) · [`range()`](../range/) · [`step()`](../step/) · [`time()`](../time/) · [`max_over_time()`](../max_over_time/) · [`min_over_time()`](../min_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#end
