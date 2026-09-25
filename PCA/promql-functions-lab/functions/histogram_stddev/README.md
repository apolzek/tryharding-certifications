# `histogram_stddev()`: o quanto os valores se espalham em volta da média

> **Em uma frase:** `histogram_stddev(v)` devolve o **desvio padrão estimado** das observações de cada **native histogram** em `v`. Mede **consistência**: dois serviços com a mesma média podem ter desvios muito diferentes. Séries float são **ignoradas**.

| | |
|---|---|
| **Assinatura** | `histogram_stddev(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ **Native** histogram · ❌ Classic (floats → resultado vazio) |
| **Unidade do resultado** | a mesma da métrica observada (segundos, bytes...) |
| **Como estima** | cada observação vale o **meio do seu bucket** (média **geométrica** dos limites nos buckets exponenciais; aritmética no zero bucket e em buckets customizados) |
| **Dashboard** | http://localhost:3300/d/fn-histogram_stddev |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: dois ônibus "pontuais em média"

Duas linhas de ônibus têm **atraso médio de zero**:

- **Linha A** chega sempre entre 1 min antes e 1 min depois. Você sai de casa no horário e pronto.
- **Linha B** às vezes chega 10 min antes (você perde), às vezes 15 min depois (você se atrasa). Na **média**, também é zero!

A média não te ajuda a escolher. O **desvio padrão** sim: é a "distância típica" de cada chegada até a média. Linha A: ~1 min. Linha B: ~10 min.

Em sistemas é igual: latência **previsível** (desvio baixo) é melhor para timeouts, retries, UX e capacidade do que latência que "na média é boa".

---

## 🔧 Setup: o que o gerador fake expõe

Os dois serviços têm **média de 100ms o tempo todo** (log-normal com média fixa). Só a dispersão muda.

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_stddev_request_duration_seconds{service="stable"}` | histogram (native + classic) | ~50 obs/s, σ ≈ **10ms** sempre |
| `histogram_stddev_request_duration_seconds{service="erratic"}` | histogram (native + classic) | ~50 obs/s, **alterna a cada 2 min**: calmo σ ≈ **31ms** ↔ caos σ ≈ **131ms** |

```bash
curl -s localhost:8088/metrics | grep -E '^histogram_stddev_request_duration_seconds_(sum|count)'
# ..._sum{service="erratic"} 3001.2     <- sum/count ≈ 0.1 nos dois serviços
# ..._count{service="erratic"} 30010
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_stddev
```

Espere **~2 min** para `[1m]` e **~4 min** para ver um ciclo calmo → caos completo.

---

## 🔍 Queries passo a passo

### 1. A média engana

```promql
histogram_avg(rate(histogram_stddev_request_duration_seconds[1m]))
```

**Resultado esperado:** **duas linhas juntas em ≈ 0.1s** o tempo todo. Olhando só a média, os serviços são idênticos.

---

### 2. O desvio padrão mostra a diferença

```promql
histogram_stddev(rate(histogram_stddev_request_duration_seconds[1m]))
```

**O que faz:** `rate(...[1m])` = histograma do último minuto; `histogram_stddev` estima o desvio padrão a partir dos buckets.
**Resultado esperado:**

| service | calmo | caos |
|---|---|---|
| stable | ≈ **0.010s** | ≈ 0.010s |
| erratic | ≈ **0.031s** | ≈ **0.13 a 0.15s** (teórico 0.131) |

A linha do `erratic` vira uma **onda quadrada** (período 4 min), suavizada nas bordas pela janela de 1 min.

---

### 3. Heatmap do erratic (buckets classic)

```promql
sum by (le) (rate(histogram_stddev_request_duration_seconds_bucket{service="erratic"}[1m]))
```

**Resultado esperado:** no **calmo** a mancha fica concentrada entre **50 e 250ms**; no **caos** ela se espalha de **< 25ms até 500ms+** (e fica mais "fraca" no meio, porque as mesmas requisições se distribuem por mais faixas).

---

### 4. Coeficiente de variação (σ / média)

```promql
histogram_stddev(rate(histogram_stddev_request_duration_seconds[1m]))
  / histogram_avg(rate(histogram_stddev_request_duration_seconds[1m]))
```

**O que faz:** normaliza o desvio pela média, para comparar serviços de escalas diferentes (um de 10ms e outro de 2s).
**Resultado esperado:** `stable` ≈ **0.10** (10%); `erratic` ≈ **0.31** ↔ ≈ **1.31** (131%!). Regra de bolso: CV > 1 = distribuição muito espalhada, com cauda longa.

---

### 5. ⚠️ Faixa média ± 2σ: cuidado com latência

```promql
histogram_avg(rate(...{service="erratic"}[1m])) + 2 * histogram_stddev(rate(...{service="erratic"}[1m]))
histogram_avg(rate(...{service="erratic"}[1m])) - 2 * histogram_stddev(rate(...{service="erratic"}[1m]))
histogram_quantile(0.99, rate(...{service="erratic"}[1m]))
```

**Resultado esperado:**
- **Calmo:** faixa ≈ **38ms a 162ms**, p99 ≈ **190ms**. Até que razoável.
- **Caos:** faixa ≈ **−160 a −200ms** (!!) até **≈360–400ms**; p99 ≈ **550 a 720ms**, bem acima do "+2σ".

**Moral:** "média ± 2σ cobre ~95%" só vale para distribuições **simétricas** (normal). Latência é **assimétrica** (não existe latência negativa, e a cauda é longa para a direita). Use desvio padrão para **comparar consistência** e **detectar mudanças**; para limites e SLOs, use [`histogram_quantile()`](../histogram_quantile/).

---

### 6. ❌ Em histograma classic: vazio

```promql
histogram_stddev(rate(histogram_stddev_request_duration_seconds_bucket[1m]))
```

**Resultado esperado:** **vazio**. Não existe desvio padrão "de graça" para classic: `_sum` e `_count` não bastam (precisaria da soma dos **quadrados**). Para classic, compare quantis (ex.: `p90 − p10`) como medida de dispersão.

---

## 🏭 Casos reais

> O cenário fake imita o caso 1: `histogram_stddev_request_duration_seconds{service}`, dois serviços com a mesma média e dispersões diferentes.

### Caso 1: jitter de rede / "vizinho barulhento" em nuvem

Um serviço roda em nós compartilhados e às vezes fica instável sem que a média mude. Alerta de **mudança de dispersão** comparando com a semana passada:

```yaml
groups:
  - name: latency-jitter
    rules:
      - record: job:http_server_request_duration_seconds:stddev5m
        expr: histogram_stddev(sum by (job) (rate(http_server_request_duration_seconds[5m])))
      - alert: LatencyJitterIncreased
        expr: |2
            job:http_server_request_duration_seconds:stddev5m
          > 3 * job:http_server_request_duration_seconds:stddev5m offset 1w
        for: 15m
        labels: { severity: warning }
```

### Caso 2: canário com a mesma média

Deploy canário: média de latência igual à versão estável, mas o coeficiente de variação dobrou (a nova versão tem um cache que às vezes falha). Comparação lado a lado:

```promql
histogram_stddev(sum by (version) (rate(http_server_request_duration_seconds{job="api"}[5m])))
  / histogram_avg(sum by (version) (rate(http_server_request_duration_seconds{job="api"}[5m])))
```

É o painel 4 (CV). Um CV que salta de 0.3 para 1.3 é motivo para abortar o rollout, mesmo com média idêntica.

Regra usada pelo pipeline de canário (Argo Rollouts/Flagger consultam o Prometheus):

```yaml
- record: version:http_server_request_duration_seconds:cv5m
  expr: |2
      histogram_stddev(sum by (version) (rate(http_server_request_duration_seconds{job="api"}[5m])))
    / histogram_avg(sum by (version) (rate(http_server_request_duration_seconds{job="api"}[5m])))
- alert: CanaryLatencyUnstable
  expr: |2
      version:http_server_request_duration_seconds:cv5m{version="canary"}
    > ignoring(version) (2 * version:http_server_request_duration_seconds:cv5m{version="stable"})
  for: 10m
```

### Caso 3: disco com latência irregular

Em `etcd_disk_backend_commit_duration_seconds` (raspado como native histogram), o desvio indica se o disco é **consistente** — um disco de rede com σ alto causa eleições de líder mesmo com média boa:

```promql
histogram_stddev(rate(etcd_disk_backend_commit_duration_seconds[5m]))
```

---

## ✅ Quando usar

- **Consistência/jitter** de latência: serviços, filas, discos, rede.
- **Detectar mudanças de comportamento** que a média não pega (ex.: vizinho barulhento, GC, cache instável): alerta em "σ dobrou em relação à semana passada".
- **Comparar** duas versões (canário vs estável) com a mesma média.
- **Coeficiente de variação** (painel 4) para comparar serviços de escalas diferentes.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Definir **limites/SLO** ("99% abaixo de X") | [`histogram_quantile()`](../histogram_quantile/) · [`histogram_fraction()`](../histogram_fraction/) |
| Métrica **classic** | diferença de quantis (`p90 − p10`) |
| Precisa da **variância** (s²) para estatística | [`histogram_stdvar()`](../histogram_stdvar/) |
| Desvio de uma **gauge** ao longo do tempo | [`stddev_over_time()`](../stddev_over_time/) |
| Desvio **entre séries** num instante | [`stddev()`](../stddev/) (agregador) |

## ⚠️ Pegadinhas

1. **É estimativa**: todas as observações de um bucket são tratadas como se valessem o meio dele. Com native (buckets de ~9%) o erro é pequeno; no `stable` o valor sai ≈ 10.3ms em vez de 10.0ms.
2. **Só native**: em floats → vazio, sem erro.
3. **Média ± kσ em dados assimétricos** (painel 5) gera limites absurdos.
4. **Sem `rate()`** = desvio de tudo desde o start do processo.
5. **Agregação**: `histogram_stddev(sum(rate(x[1m])))` é o desvio do **conjunto** de observações (misturando serviços, o desvio cresce se as médias forem diferentes). `avg(histogram_stddev(...))` é outra coisa.
6. **Não confunda** com [`stddev()`](../stddev/) (entre séries) e [`stddev_over_time()`](../stddev_over_time/) (ao longo do tempo de uma série float).

## 🎓 Na prova PCA

O que costuma cair (sobre dispersão em PromQL em geral):
- Três "desvios padrão" diferentes: **`stddev()`** (agregador, entre séries no mesmo instante), **`stddev_over_time()`** (amostras float de uma série numa janela) e **`histogram_stddev()`** (observações dentro de um native histogram).
- `histogram_stddev` é **estimativa** (valor no meio do bucket) e só para **native**.
- Média igual não significa comportamento igual; para SLO, quantis.

**1.** Você quer o desvio padrão da latência das requisições dos últimos 5 min, a partir de um native histogram `x`. Qual query?
- A) `stddev(x)`
- B) `stddev_over_time(x[5m])`
- C) `histogram_stddev(rate(x[5m]))`
- D) `stddev(rate(x[5m]))`

<details><summary>Resposta</summary>

**C.** A e D calculam o desvio **entre séries**; B é para amostras float ao longo do tempo (e ignora histogramas).
</details>

**2.** Como `histogram_stddev` estima o valor de cada observação?
- A) Usa o valor exato de cada observação
- B) Assume que todas as observações de um bucket valem o meio do bucket (média geométrica nos buckets exponenciais)
- C) Usa só `_sum` e `_count`
- D) Usa o limite superior de cada bucket

<details><summary>Resposta</summary>

**B.** Está na documentação. Por isso é uma estimativa; com buckets finos (native) o erro é pequeno.
</details>

**3.** Dois serviços têm média de 100ms; o A tem σ = 10ms e o B, σ = 130ms. Qual afirmação é verdadeira?
- A) São equivalentes para o usuário
- B) B tem latência bem menos previsível; o p99 de B tende a ser bem maior
- C) A é mais lento
- D) B tem mais tráfego

<details><summary>Resposta</summary>

**B.** Mesma média, dispersões muito diferentes. No lab: p99 ≈ 125ms no stable e ≈ 600ms no erratic em caos.
</details>

**4.** `histogram_stddev(rate(x_bucket[5m]))` retorna...
- A) O desvio padrão estimado pelos buckets classic
- B) Vazio
- C) Erro de parse
- D) Zero

<details><summary>Resposta</summary>

**B.** Floats são ignorados; não há suporte a classic.
</details>

## 📝 Cola rápida

- `histogram_stddev(rate(x[5m]))` → σ estimado das observações (só **native**; floats → vazio).
- Estimativa: cada observação = meio do seu bucket (geométrico nos exponenciais).
- Mesma média ≠ mesmo comportamento. CV = σ / média.
- Latência é assimétrica: média ± 2σ pode dar negativo. Para limites use quantis.
- `stddev()` (entre séries) · `stddev_over_time()` (no tempo) · `histogram_stddev()` (dentro do histograma).

## 🔗 Relacionadas

[`histogram_stdvar()`](../histogram_stdvar/) · [`histogram_avg()`](../histogram_avg/) · [`histogram_quantile()`](../histogram_quantile/) · [`stddev_over_time()`](../stddev_over_time/) · [`stddev()`](../stddev/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_stddev-and-histogram_stdvar
