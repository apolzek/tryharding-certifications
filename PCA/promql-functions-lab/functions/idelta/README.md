# `idelta()`: o que mudou entre as 2 últimas amostras

> **Em uma frase:** `idelta(v[janela])` é a diferença entre a **última** e a **penúltima** amostra de um **gauge**. Responde "o que mudou desde o último scrape?", em unidades **por scrape** (não por segundo).

| | |
|---|---|
| **Assinatura** | `idelta(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (floats e native histograms) · ❌ Counter |
| **Unidade do resultado** | a **mesma do gauge**, variação entre 2 scrapes (depende do scrape interval!) |
| **Dashboard** | http://localhost:3300/d/fn-idelta |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: as duas últimas fotos do álbum

O Prometheus tira uma **foto** do gauge a cada scrape (aqui, a cada 5s).

- [`delta(x[1m])`](../delta/) compara a foto de **1 minuto atrás** com a de agora.
- `idelta(x[1m])` compara a **penúltima** foto com a **última**. A janela `[1m]` só diz **até onde procurar** essas duas fotos.

```
amostras:  ... 300  200  100    0  500
                                ↑    ↑
                         penúltima  última     → idelta = 500 − 0 = +500
```

É a versão "gauge" do [`irate()`](../irate/), mas **sem dividir pelo tempo** e **sem tratar resets**.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `idelta_rabbitmq_queue_messages{queue="emails"}` | gauge | imita o exporter do RabbitMQ: **lote de 500** mensagens chega a cada **30s**, consumidores drenam **20 msg/s** até zerar |
| `idelta_pg_stat_activity_count{datname="shop"}` | gauge | imita o postgres_exporter: conexões sobem em **degraus de +20 a cada 45s** e voltam a 10 a cada 3 min |
| `idelta_http_requests_total` | **counter** | +4/s, reseta a cada 2 min (para a pegadinha) |

```bash
curl -s localhost:8088/metrics | grep '^idelta_'
# idelta_http_requests_total 9
# idelta_pg_stat_activity_count{datname="shop"} 30
# idelta_rabbitmq_queue_messages{queue="emails"} 450
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-idelta
```

Em **~1 minuto** já dá para ver o padrão; espere ~3 min para o ciclo das conexões.

---

## 🔍 Queries passo a passo

### 1. A fila crua

```promql
idelta_rabbitmq_queue_messages
```

**Resultado esperado:** um "serrote" invertido: a cada 30s a fila **salta** para ~500 e depois **desce em rampa** (20 msg/s) até 0, onde fica uns 5s.

---

### 2. O que mudou no último scrape

```promql
idelta(idelta_rabbitmq_queue_messages[1m])
```

**Resultado esperado** (unidade: mensagens **por scrape de 5s**):

| momento | idelta |
|---|---|
| drenando | ≈ **−100** (20 msg/s × 5s) |
| fila chegou a 0 | um degrau menor (ex.: −60) e depois **0** |
| lote chegou | salto de ≈ **+400 a +500** (depende de onde o scrape caiu no ciclo) |

> 💡 Se o scrape interval mudasse para 15s, a drenagem viraria ≈ **−300** por scrape. O `idelta` **depende do scrape interval**. Para algo "por segundo", use `deriv()` ou divida pelo intervalo.

---

### 3. `idelta` × `delta[1m]`

```promql
idelta(idelta_rabbitmq_queue_messages[1m])
delta(idelta_rabbitmq_queue_messages[1m])
```

**Resultado esperado:**
- `idelta`: um pente: −100, −100, −100, −100, (−9), **+409** no lote, e assim por diante.
- `delta[1m]`: quase sempre ≈ **+110**, com picos de ≈ **+240** e ≈ **−450** duas vezes por minuto.

Por quê? A primeira e a última amostra de uma janela de 1m estão a ~55s de distância. Com um ciclo de 30s, isso é "quase o mesmo ponto do ciclo, 5s antes": uma diferença de +100 (≈ +110 com a extrapolação). Quando uma das pontas cai logo antes/depois da chegada do lote, o resultado salta (≈ +240 ou ≈ −450). A tendência real da fila é **zero** (ela sempre volta a 0), mas nenhum dos dois mostra isso: o `idelta` mostra cada passo, o `delta` mostra "em que fase caíram as pontas". Para tendência de algo periódico, use janelas bem maiores que o ciclo com [`deriv()`](../deriv/) ou `avg_over_time`.

---

### 4. Degraus: conexões no banco

```promql
idelta_pg_stat_activity_count             # cru: escada
idelta(idelta_pg_stat_activity_count[1m]) # +20 no degrau, 0 no resto
```

**Resultado esperado:** o cru é uma escada 10 → 30 → 50 → 70 → volta para 10. O `idelta` fica em **0** quase o tempo todo, com **+20** a cada degrau e **−60** quando o pool é reciclado. Ótimo para "marcar" o instante exato de uma mudança.

---

### 5. `idelta[1m]` × `idelta[5m]`

```promql
idelta(idelta_rabbitmq_queue_messages[1m])
idelta(idelta_rabbitmq_queue_messages[5m])
```

**Resultado esperado:** linhas **idênticas**. A janela não muda nada, desde que existam 2 amostras dentro dela.

---

### 6. Pegadinha: `idelta` num counter

```promql
idelta(idelta_http_requests_total[1m])      # errado
irate(idelta_http_requests_total[1m]) * 5   # certo (por scrape de 5s)
```

**Resultado esperado:** ambos ≈ **+20** por scrape (4/s × 5s), mas a cada 2 min o counter reseta e o `idelta` despenca para ≈ **−460** (ex.: `4 − 464`). O `irate` detecta o reset e continua positivo.

---

## 🏭 Casos reais

### 1. Chegada de lotes numa fila (imitado pelos painéis 1-3)

Um job de marketing despeja 50 mil e-mails na fila de uma vez. Com `rate`/`deriv` de 5 min o salto vira uma "rampa" suave; com `idelta` você vê **o scrape exato** em que o lote chegou e de que tamanho:

```yaml
# alerta "de evento": dispara no scrape em que o lote chega (sem for:)
- alert: LoteGrandeNaFila
  expr: idelta(rabbitmq_queue_messages{queue="emails"}[1m]) > 10000
  labels: {severity: info}
  annotations:
    summary: "Chegaram {{ $value }} mensagens de uma vez na fila {{ $labels.queue }}"
```

Útil em painéis de troubleshooting e em anotações do Grafana ("marque no gráfico quando chegou um lote grande").

### 2. Saltos de conexões no banco (imitado pelo painel 4)

Depois de um deploy, o pool de conexões dobra de uma vez. Para detectar "degraus" bruscos:

```yaml
- alert: SaltoDeConexoes
  expr: idelta(pg_stat_activity_count{datname="shop"}[2m]) > 50
  labels: {severity: info}
  annotations:
    summary: "+{{ $value }} conexões de uma vez em {{ $labels.datname }} (deploy? pool mal configurado?)"
```

**Decisão:** `idelta` porque interessa o **salto instantâneo**, não a média. Para alertas de saturação real, compare com o limite: `sum(pg_stat_activity_count) / pg_settings_max_connections > 0.8`.

### 3. Gauge que é "o último valor de um evento"

Exporters batch às vezes expõem um gauge com o total processado na última execução (`job_last_run_processed_items`). O `idelta` mostra a diferença entre uma execução e a anterior sem precisar de `offset`.

> ⚠️ Em alertas com `for:`, o `idelta` tem o mesmo problema do `irate`: pisca. Prefira `delta`/`deriv` com janela maior quando o alerta precisa se manter verdadeiro por minutos.

---

## ✅ Quando usar

- Ver **saltos instantâneos** de um gauge (lotes, degraus, recargas) em gráficos de alta resolução.
- Marcar **o momento exato** de uma mudança (config reload, resize de pool).
- Debug: "o que mudou no último scrape?".

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **counter** | [`irate()`](../irate/) (por segundo, com reset) |
| Quer a variação numa **janela** | [`delta()`](../delta/) |
| Quer a **tendência por segundo** de um gauge ruidoso | [`deriv()`](../deriv/) |
| **Alertas** que precisam ficar estáveis por minutos | [`delta()`](../delta/) / [`deriv()`](../deriv/) com janela maior |
| Quer **quantas vezes** mudou | [`changes()`](../changes/) |

## ⚠️ Pegadinhas

1. **Unidade = "por scrape":** o número muda se o scrape interval mudar. Não compare `idelta` entre jobs com intervalos diferentes.
2. **Não trata resets:** em counters, cada reset vira um valor negativo enorme.
3. **Zoom out esconde tudo:** com step do gráfico maior que o scrape, cada ponto só mostra o par de amostras antes do step; saltos no meio somem (mesmo problema do `irate`).
4. **Mistura float/histograma** nas 2 últimas amostras: série omitida (warning).
5. **Janela sem 2 amostras** → sem resultado.

## 🎓 Na prova PCA

O que costuma cair:
- `idelta` recebe **range vector**, devolve **instant vector**, usa **só as 2 últimas amostras**.
- É para **gauges**; o análogo para counters é `irate`.
- **Não** é por segundo (diferente de `irate`/`deriv`), e **não** extrapola.

**1.** Qual a diferença entre `idelta(x[5m])` e `delta(x[5m])`?
- A) `idelta` é por segundo; `delta` não
- B) `idelta` usa as duas últimas amostras; `delta` usa a primeira e a última da janela (extrapolado)
- C) `idelta` funciona com counters; `delta` com gauges
- D) Nenhuma, são sinônimos

<details><summary>Resposta</summary>

**B.** Ambas são para gauges e retornam a variação na unidade do gauge; o `idelta` só olha o último intervalo.
</details>

**2.** Amostras de um gauge (scrape de 15s): `40, 42, 47, 45`. Qual o resultado de `idelta(x[1m])`?
- A) 5
- B) −2
- C) −0.133
- D) 2

<details><summary>Resposta</summary>

**B.** Última (45) − penúltima (47) = −2. Não divide pelo tempo (C seria −2/15).
</details>

**3.** Qual é o equivalente de `idelta` para counters?
- A) `rate`
- B) `increase`
- C) `irate`
- D) `resets`

<details><summary>Resposta</summary>

**C.** `irate` também usa as 2 últimas amostras, mas divide pelo tempo e compensa resets.
</details>

## 📝 Cola rápida

- `idelta(gauge[janela])` = **última − penúltima** amostra (unidade do gauge, **por scrape**).
- Janela = só "até onde procurar".
- Gauge ↔ `idelta`; counter ↔ `irate`.
- Não trata reset, não extrapola, não divide pelo tempo.
- Ótimo para ver saltos; ruim para alertas.

## 🔗 Relacionadas

[`delta()`](../delta/) · [`irate()`](../irate/) · [`deriv()`](../deriv/) · [`changes()`](../changes/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#idelta
