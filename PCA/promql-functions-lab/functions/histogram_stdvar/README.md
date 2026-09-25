# `histogram_stdvar()`: a variância (desvio padrão ao quadrado)

> **Em uma frase:** `histogram_stdvar(v)` devolve a **variância estimada** das observações de cada **native histogram** em `v`. É exatamente o quadrado do [`histogram_stddev()`](../histogram_stddev/): `sqrt(histogram_stdvar(x)) = histogram_stddev(x)`. Séries float são **ignoradas**.

| | |
|---|---|
| **Assinatura** | `histogram_stdvar(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ **Native** histogram · ❌ Classic (floats → resultado vazio) |
| **Unidade do resultado** | a unidade observada **ao quadrado** (s², bytes²) |
| **Como estima** | igual ao `histogram_stddev`: cada observação vale o meio do seu bucket |
| **Dashboard** | http://localhost:3300/d/fn-histogram_stdvar |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o lado e a área do quadrado

Pense no **desvio padrão** como o **lado** de um quadrado: a distância típica de cada observação até a média (em segundos).

A **variância** é a **área** desse quadrado (lado × lado, em segundos²):

- Se o lado dobra (2×), a área **quadruplica** (4×).
- Se o lado sextuplica (6.6×), a área fica **43×** maior.

Por isso a variância:
- **amplifica** diferenças (bom para detectar mudança, ruim para ler num gráfico);
- tem **unidade estranha** (s²), que ninguém "sente" intuitivamente;
- tem uma propriedade matemática útil: variâncias de coisas independentes **se somam**, e é com ela que se fazem contas estatísticas (testes, intervalos de confiança, combinar grupos).

---

## 🔧 Setup: o que o gerador fake expõe

Tempo de espera numa fila. As duas filas têm **média de 0.5s** o tempo todo.

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_stdvar_queue_wait_seconds{queue="dedicated"}` | histogram (native + classic) | ~40 obs/s, σ ≈ 0.05s → **variância ≈ 0.0025 s²** |
| `histogram_stdvar_queue_wait_seconds{queue="shared"}` | histogram (native + classic) | ~40 obs/s, **alterna a cada 2 min**: vizinho quieto σ ≈ 0.10s (**var ≈ 0.010 s²**) ↔ vizinho barulhento σ ≈ 0.33s (**var ≈ 0.108 s²**) |

```bash
curl -s localhost:8088/metrics | grep -E '^histogram_stdvar_queue_wait_seconds_count'
# histogram_stdvar_queue_wait_seconds_count{queue="dedicated"} 24000
# histogram_stdvar_queue_wait_seconds_count{queue="shared"} 24000
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_stdvar
```

Espere **~2 min** para `[1m]` e **~4 min** para um ciclo quieto → barulhento completo.

---

## 🔍 Queries passo a passo

### 1. A variância, em s²

```promql
histogram_stdvar(rate(histogram_stdvar_queue_wait_seconds[1m]))
```

**Resultado esperado:**

| queue | vizinho quieto | vizinho barulhento |
|---|---|---|
| dedicated | ≈ **0.0025** | ≈ 0.0025 |
| shared | ≈ **0.010** | ≈ **0.108** (às vezes 0.09–0.11) |

Repare como o `dedicated` quase "some" no fundo do gráfico: a escala quadrática esmaga os valores pequenos.

---

### 2. `sqrt(stdvar)` = `stddev`

```promql
sqrt(histogram_stdvar(rate(histogram_stdvar_queue_wait_seconds{queue="shared"}[1m])))
histogram_stddev(rate(histogram_stdvar_queue_wait_seconds{queue="shared"}[1m]))
```

**Resultado esperado:** **linhas sobrepostas**, alternando ≈ **0.10s** ↔ ≈ **0.33s**. Em dashboards para humanos, prefira o desvio padrão: está na mesma unidade da métrica.

---

### 3. A variância amplifica: razões

```promql
histogram_stdvar(rate(histogram_stdvar_queue_wait_seconds{queue="shared"}[1m]))
  / ignoring(queue) histogram_stdvar(rate(histogram_stdvar_queue_wait_seconds{queue="dedicated"}[1m]))

histogram_stddev(rate(histogram_stdvar_queue_wait_seconds{queue="shared"}[1m]))
  / ignoring(queue) histogram_stddev(rate(histogram_stdvar_queue_wait_seconds{queue="dedicated"}[1m]))
```

**O que faz:** divide a fila compartilhada pela dedicada. `ignoring(queue)` é necessário porque os labels `queue` são diferentes dos dois lados.
**Resultado esperado:**

| | quieto | barulhento |
|---|---|---|
| razão das **variâncias** | ≈ **4** | ≈ **36 a 43** |
| razão dos **desvios** | ≈ **2** | ≈ **6 a 6.6** |

A variância é o **quadrado** da razão dos desvios. Um alerta em "variância 10× maior" dispara bem antes de um em "desvio 10× maior".

---

### 4. ...e a média não muda

```promql
histogram_avg(rate(histogram_stdvar_queue_wait_seconds[1m]))
```

**Resultado esperado:** duas linhas em ≈ **0.5s**. O vizinho barulhento não deixou a fila mais lenta **em média**, deixou-a **imprevisível**.

---

### 5. Variância do conjunto: `histogram_stdvar(sum(rate(x)))`

```promql
histogram_stdvar(sum(rate(histogram_stdvar_queue_wait_seconds[1m])))   # variância de TODAS as esperas juntas
avg(histogram_stdvar(rate(histogram_stdvar_queue_wait_seconds[1m])))   # média das variâncias
```

**O que faz:** a primeira **soma os histogramas** das duas filas (como se fossem uma só) e mede a variância de todas as esperas. A segunda tira a média das variâncias de cada fila.
**Resultado esperado:** as duas coincidem aqui: ≈ **0.006 s²** (quieto) ↔ ≈ **0.055 s²** (barulhento), porque **as médias são iguais (0.5s) e o tráfego também**.
**Em geral elas NÃO coincidem:** se uma fila tivesse média 0.5s e outra 2s, a variância do conjunto incluiria também a diferença **entre** as médias (lei da variância total) e seria bem maior. Para "a variância que o usuário sente", use a primeira.

---

### 6 e 7. Heatmap da fila compartilhada e ❌ classic

```promql
sum by (le) (rate(histogram_stdvar_queue_wait_seconds_bucket{queue="shared"}[1m]))   # heatmap
histogram_stdvar(rate(histogram_stdvar_queue_wait_seconds_bucket[1m]))              # vazio
```

**Resultado esperado:** o heatmap "respira": concentrado entre **0.25s e 1s** com o vizinho quieto, espalhado de **0.1s até 2.5s** com o barulhento. A segunda query dá **vazio**: `_bucket` é float, e `histogram_stdvar` só aceita native.

---

## 🏭 Casos reais

> O cenário fake imita o caso 1: `histogram_stdvar_queue_wait_seconds{queue}`, fila em máquina dedicada vs compartilhada.

### Caso 1: comparar variância de filas (teste estatístico simples)

Um time de plataforma quer provar que mover um consumidor de fila para nós compartilhados piorou a previsibilidade. Razão de variâncias (base de um teste F) entre os dois pools:

```promql
  histogram_stdvar(sum(rate(queue_wait_seconds{pool="shared"}[30m])))
/ histogram_stdvar(sum(rate(queue_wait_seconds{pool="dedicated"}[30m])))
```

Um valor como **40** (painel 3) é uma diferença enorme; o mesmo dado em desvio padrão (≈ 6.5×) soa menos dramático.

```yaml
- record: pool:queue_wait_seconds:stdvar30m
  expr: histogram_stdvar(sum by (pool) (rate(queue_wait_seconds[30m])))
- alert: SharedPoolNoisyNeighbor
  # variância 10x maior que a do pool dedicado (≈ desvio 3.2x)
  expr: |2
      pool:queue_wait_seconds:stdvar30m{pool="shared"}
    > ignoring(pool) (10 * pool:queue_wait_seconds:stdvar30m{pool="dedicated"})
  for: 15m
  labels: { severity: warning }
```

### Caso 2: recording rule para combinar depois

Variâncias de componentes **independentes** se somam (desvios não). Se uma requisição passa por fila + processamento, o time grava as variâncias e estima a da jornada inteira:

```yaml
groups:
  - name: variance
    rules:
      - record: job:queue_wait_seconds:stdvar5m
        expr: histogram_stdvar(sum by (job) (rate(queue_wait_seconds[5m])))
      - record: job:processing_seconds:stdvar5m
        expr: histogram_stdvar(sum by (job) (rate(processing_seconds[5m])))
      # σ da jornada (supondo independência) = sqrt(var1 + var2)
      - record: job:end_to_end_seconds:stddev_estimate5m
        expr: sqrt(job:queue_wait_seconds:stdvar5m + job:processing_seconds:stdvar5m)
```

### Caso 3: z-score para detectar outlier de latência

```promql
# quantos "desvios" a média atual está acima da média da última hora
  (histogram_avg(rate(http_server_request_duration_seconds[5m])) - histogram_avg(rate(http_server_request_duration_seconds[1h])))
/ sqrt(histogram_stdvar(rate(http_server_request_duration_seconds[1h])))
```

Útil para alertas adaptativos, mas lembre que latência é assimétrica: trate como heurística.

---

## ✅ Quando usar

- **Estatística**: testes de hipótese, intervalos de confiança, z-score (`(x − média) / sqrt(var)`), combinar grupos, onde a fórmula pede variância.
- **Detectar instabilidade** com mais sensibilidade que o desvio padrão (painel 3).
- **Recording rules** que depois são somadas/combinadas: variâncias de fontes independentes se somam; desvios não.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Mostrar dispersão para **humanos** | [`histogram_stddev()`](../histogram_stddev/) (mesma unidade da métrica) |
| Limites e SLOs | [`histogram_quantile()`](../histogram_quantile/) · [`histogram_fraction()`](../histogram_fraction/) |
| Métrica **classic** | diferença de quantis (`p90 − p10`) |
| Variância de uma **gauge** ao longo do tempo | [`stdvar_over_time()`](../stdvar_over_time/) |
| Variância **entre séries** | [`stdvar()`](../stdvar/) (agregador) |

## ⚠️ Pegadinhas

1. **Unidade ao quadrado**: 0.108 **s²** não é "108ms". Tire a raiz antes de comparar com latências.
2. **Escala esmagada**: valores pequenos somem no gráfico; considere escala log no Grafana ou use stddev.
3. **É estimativa** (valores no meio do bucket), igual ao stddev.
4. **Só native**: em floats → vazio, sem erro.
5. **Sem `rate()`** = variância desde o start do processo.
6. **Variância do conjunto ≠ média das variâncias** quando as médias ou os tráfegos diferem (painel 5).

## 🎓 Na prova PCA

O que costuma cair:
- **Variância = desvio padrão²**. Unidade ao quadrado.
- Família: `stdvar()` (agregador entre séries), `stdvar_over_time()` (amostras float no tempo), `histogram_stdvar()` (dentro de um native histogram).
- Só **native**; em floats → vazio. Estimativa pelos buckets, como `histogram_stddev`.

**1.** `histogram_stdvar(rate(x[5m]))` retorna `0.09` para uma métrica em segundos. Qual é o desvio padrão?
- A) 0.09 s
- B) 0.3 s
- C) 0.0081 s
- D) 9 ms

<details><summary>Resposta</summary>

**B.** √0.09 = 0.3. A variância está em s².
</details>

**2.** Qual expressão é equivalente a `histogram_stddev(h)`?
- A) `histogram_stdvar(h) ^ 2`
- B) `sqrt(histogram_stdvar(h))`
- C) `histogram_stdvar(h) / histogram_count(h)`
- D) `stdvar(h)`

<details><summary>Resposta</summary>

**B.** O desvio é a raiz da variância (painel 2).
</details>

**3.** O desvio padrão de uma fila passou de 0.1s para 0.3s. Quanto a variância aumentou?
- A) 3×
- B) 6×
- C) 9×
- D) 0.2 s²

<details><summary>Resposta</summary>

**C.** (0.3/0.1)² = 9. A variância amplifica as mudanças (painel 3).
</details>

**4.** Qual função calcula a variância **entre** as séries de `node_load1` de todos os nós, num instante?
- A) `histogram_stdvar(node_load1)`
- B) `stdvar_over_time(node_load1[5m])`
- C) `stdvar(node_load1)`
- D) `var(node_load1)`

<details><summary>Resposta</summary>

**C.** `stdvar` é o operador de agregação. A retorna vazio (gauge float), B é por série ao longo do tempo, D não existe.
</details>

## 📝 Cola rápida

- `histogram_stdvar(rate(x[5m]))` = variância estimada, unidade², só **native**.
- `sqrt(histogram_stdvar(x)) == histogram_stddev(x)`.
- Variância amplifica diferenças (quadrado); boa para detecção/estatística, ruim para leitura humana.
- Variâncias de partes independentes se somam; desvios não.
- `stdvar()` (entre séries) · `stdvar_over_time()` (no tempo) · `histogram_stdvar()` (dentro do histograma).

## 🔗 Relacionadas

[`histogram_stddev()`](../histogram_stddev/) · [`histogram_avg()`](../histogram_avg/) · [`histogram_quantile()`](../histogram_quantile/) · [`stdvar_over_time()`](../stdvar_over_time/) · [`stdvar()`](../stdvar/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_stddev-and-histogram_stdvar
