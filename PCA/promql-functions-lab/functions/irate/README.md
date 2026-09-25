# `irate()`: a velocidade instantânea de um counter

> **Em uma frase:** `irate(v[janela])` calcula a velocidade **por segundo** usando **só as 2 últimas amostras** da janela. Reage na hora a picos, mas é nervoso: ótimo para **gráficos** de counters rápidos, ruim para **alertas**.

| | |
|---|---|
| **Assinatura** | `irate(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Counter (`*_total`, `*_count`, `*_sum`, `*_bucket`, native histograms) · ❌ Gauge |
| **Unidade do resultado** | "unidades por segundo" (req/s, bytes/s, jobs/s...) |
| **Dashboard** | http://localhost:3300/d/fn-irate |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: velocidade média da viagem × ponteiro do velocímetro

Você fez 120 km em 2 horas. A **velocidade média** foi 60 km/h, isso é o [`rate()`](../rate/).

Mas no meio da viagem você **ultrapassou um caminhão a 110 km/h** por alguns segundos. A média não mostra isso. Quem mostra é o **ponteiro do velocímetro**, que olha só o **último instante**. Isso é o `irate()`:

```
                 (penúltima amostra → última amostra)
irate  =   ────────────────────────────────────────────────
             segundos entre a penúltima e a última amostra
```

- A janela `[1m]` **não** é "a média de 1 minuto". Ela só diz **até onde olhar para trás** para achar as 2 amostras. `irate(x[1m])` e `irate(x[5m])` dão **o mesmo número** (desde que existam 2 amostras no último minuto).
- Como o `rate()`, o `irate()` **compensa resets**: se a última amostra é menor que a penúltima, ele assume que o counter recomeçou do zero.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) calcula tudo a partir do relógio, então os padrões são determinísticos:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `irate_http_requests_total{route="/api/orders"}` | counter | **5 req/s** de base + **rajada de 100 req/s durante 15s a cada 60s** |
| `irate_node_network_transmit_errs_total{device="eth0"}` | counter | **+1 erro de transmissão a cada 40s** (counter lento, imita o node_exporter) |
| `irate_worker_jobs_processed_total{pod="worker-a"}` | counter | **3 jobs/s**, o pod **reinicia a cada 2 min** (volta a 0) |

```bash
curl -s localhost:8088/metrics | grep '^irate_'
# irate_http_requests_total{route="/api/orders"} 1.0750448e+07
# irate_node_network_transmit_errs_total{device="eth0"} 9348
# irate_worker_jobs_processed_total{pod="worker-a"} 207
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-irate
```

Espere **~2 minutos** para ver várias rajadas e ~5 min para o `rate(...[5m])` do painel 4.

---

## 🔍 Queries passo a passo

### 1. O counter cru

```promql
irate_http_requests_total
```

**O que faz:** mostra o total acumulado.
**Resultado esperado:** uma linha subindo sem parar. Olhando com atenção, a cada 60s a inclinação fica bem mais forte por 15s (a rajada). Difícil de ler: por isso existem `rate` e `irate`.

---

### 2. `irate` × `rate` no mesmo counter

```promql
irate(irate_http_requests_total[1m])   # ponteiro do velocímetro
rate(irate_http_requests_total[1m])    # velocidade média do último minuto
```

**O que faz:** o `irate` usa só o último intervalo de scrape (5s aqui); o `rate` usa todos os pontos do último minuto.

**Resultado esperado** (unidade: req/s):

| | fora da rajada | durante a rajada |
|---|---|---|
| `irate[1m]` | ≈ **5** | ≈ **100** (picos a cada 60s, com 1 ou 2 pontos intermediários, ex.: 91 e 14) |
| `rate[1m]` | ≈ **31** | ≈ **22 a 31** |

Por que o `rate[1m]` fica num **platô**? Porque toda janela de 60s contém **uma** rajada: `(45s × 5 + 15s × 100) / 60s = 1725 / 60 ≈ 28.75` em média. Na prática ele fica em ≈ **31** (extrapolação de 55s para 60s) e dá uma leve "entalhada" para ≈ **22** quando parte da rajada cai no intervalo antes da primeira amostra da janela. O `rate` diz "a média é 29 req/s" e **esconde** que o serviço alterna entre calmaria (5) e tempestade (100). O `irate` mostra a tempestade.

> 💡 Nenhum dos dois está "errado": são perguntas diferentes. "Qual a carga média?" → `rate`. "Como a carga se comporta segundo a segundo?" → `irate`.

---

### 3. A janela não muda o resultado do `irate`

```promql
irate(irate_http_requests_total[1m])
irate(irate_http_requests_total[5m])
```

**Resultado esperado:** as duas linhas ficam **exatamente sobrepostas**. Com o `rate`, trocar `[1m]` por `[5m]` suaviza tudo; com o `irate`, não muda nada, porque as 2 últimas amostras são as mesmas.

**Regra prática:** use uma janela **pequena, mas com folga** (ex.: `[1m]`, ou `$__rate_interval` no Grafana), só para garantir que sempre existam 2 amostras mesmo se um scrape falhar.

---

### 4. Counter lento: o `irate` vira um pente

```promql
irate(irate_node_network_transmit_errs_total[1m])   # pente
rate(irate_node_network_transmit_errs_total[5m])    # média honesta
```

**Resultado esperado:**
- `irate`: **0** quase o tempo todo e, a cada 40s, um **pico de 0.2** (1 erro dividido por 5s de intervalo). Parece que "às vezes tem 0.2 erros/s", o que é enganoso.
- `rate[5m]`: linha estável em ≈ **0.025 erros/s** (1/40), ou seja, **1.5 erros por minuto**. Isso é o que você quer num gráfico ou num alerta.

É exatamente o que a documentação avisa: "gráficos feitos só de picos raros são difíceis de ler".

---

### 5 e 6. Reset (restart de pod)

```promql
irate_worker_jobs_processed_total              # cru: dente-de-serra
irate(irate_worker_jobs_processed_total[1m])   # ≈ 3 jobs/s
```

**Resultado esperado:**
- **Cru:** sobe até ~**360** e despenca para **0** a cada 2 min.
- **`irate`:** linha em ≈ **3 jobs/s**, com uma **pequena queda** (nunca negativa) no instante do reset (aqui ≈ **2.6**). Quando a última amostra (ex.: `13`) é menor que a penúltima (ex.: `358`), o `irate` assume que o counter começou do zero e usa `13 / 5s = 2.6` naquele ponto. (Um pouco menos que 3 porque o reset aconteceu no meio do intervalo e o pedaço antes do reset se perdeu.)

---

### 7. Instantâneo: `irate` × `rate` agora

```promql
irate(irate_http_requests_total[1m])
rate(irate_http_requests_total[1m])
```

**Resultado esperado:** o `rate` ≈ **22 a 31**; o `irate` mostra **5** ou **100**, dependendo de você abrir o dashboard dentro ou fora de uma rajada. Recarregue algumas vezes e veja o valor pular.

---

## 🏭 Casos reais

### 1. Dashboard "tempo real" de rede do node_exporter (mesma ideia do painel 2)

O time de infra quer ver **micro-rajadas** de tráfego na placa de rede de um servidor de backup: o job de backup manda 1 GB em 10s e depois fica quieto. Com `rate(...[5m])` a rajada vira uma "lombadinha" de 3 MB/s; com `irate` aparece o pico real de 100 MB/s.

```promql
# Painel do Grafana, zoom de 15 min a 1h
irate(node_network_receive_bytes_total{device="eth0"}[$__rate_interval]) * 8   # bits/s
```

**Decisão:** `irate` **só no gráfico**, com zoom curto. O alerta de saturação (mesma métrica) usa `rate`:

```yaml
- alert: RedeSaturada
  expr: rate(node_network_receive_bytes_total{device="eth0"}[5m]) * 8 > 0.8 * 1e9   # > 80% de 1 Gbit/s
  for: 15m
  labels: {severity: warning}
```

E se o time quiser um painel "tempo real" barato para muitos hosts, a recording rule grava o `rate` curto (nunca o `irate`):

```yaml
- record: instance:node_network_receive_bytes:rate1m
  expr: sum by (instance) (rate(node_network_receive_bytes_total{device!~"lo|veth.*"}[1m]))
```

### 2. CPU por modo, no painel de troubleshooting

```promql
sum by (mode) (irate(node_cpu_seconds_total{instance="db-01:9100", mode!="idle"}[1m]))
```

**Decisão:** o `irate` vem **dentro** do `sum` (primeiro irate, depois agrega), para que um reboot do node (counter volta a zero) seja tratado como reset de cada série, e não como uma queda da soma.

### 3. O alerta que ficava "piscando" (imitado pelo painel 4, mesma métrica)

Um time escreveu:

```yaml
# ❌ ruim: irate + for
- alert: ErrosDeRedeAltos
  expr: irate(node_network_transmit_errs_total[5m]) > 0.1
  for: 5m
```

O counter de erros sobe de vez em quando: o `irate` fica em 0 quase sempre e pula para 0.2 por um instante. O `for: 5m` **nunca** é satisfeito (a condição volta a ser falsa no scrape seguinte). A correção:

```yaml
# ✅ bom: rate com janela ≥ for
- alert: ErrosDeRedeAltos
  expr: rate(node_network_transmit_errs_total[5m]) > 0.01
  for: 10m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.instance }} {{ $labels.device }}: {{ $value | humanize }} erros/s"
```

## ✅ Quando usar

- **Gráficos de alta resolução** de counters rápidos e voláteis: tráfego de rede (`irate(node_network_receive_bytes_total[1m])`), req/s em dashboards de "tempo real", CPU (`irate(node_cpu_seconds_total[1m])`).
- **Investigar incidentes:** ver micro-rajadas que o `rate` achata ("o p99 subiu às 14:03:25 porque veio uma rajada de 10× o tráfego").
- **Zoom curto** no Grafana (últimos 5-15 min), onde cada pixel corresponde a poucos scrapes.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| **Alertas** (`for: 5m`): um único ponto baixo "zera" o `for` | [`rate()`](../rate/) |
| Counter **lento** (erros raros, jobs por hora) | [`rate()`](../rate/) com janela maior, ou [`increase()`](../increase/) |
| Recording rules / SLOs | [`rate()`](../rate/) |
| Métrica é **gauge** | [`idelta()`](../idelta/) (diferença entre as 2 últimas amostras) ou [`deriv()`](../deriv/) |
| Quer o **total** na janela | [`increase()`](../increase/) |

## ⚠️ Pegadinhas

1. **Zoom out esconde picos:** num gráfico de 24h, o Grafana calcula um ponto a cada ~minutos (step). O `irate` só olha os 2 últimos scrapes **antes de cada step**, então tudo o que aconteceu entre um step e outro é **ignorado** (aliasing). O `rate` com janela ≥ step não tem esse problema.
2. **Alertas com `for:`** ficam piscando: basta um intervalo calmo para a condição virar falsa e o contador do `for` recomeçar.
3. **`irate(sum(...))` é errado:** como no `rate`, primeiro `irate`, depois `sum`. Senão o restart de um pod vira um "reset" da soma inteira.
4. **Janela sem 2 amostras = sem resultado:** `irate(x[5s])` com scrape de 5s frequentemente não acha 2 pontos. Use pelo menos 2-4× o scrape interval.
5. **Não é "mais preciso" que o `rate`:** é mais **recente**, não mais correto. Ele descarta todas as amostras menos duas.

## 🎓 Na prova PCA

O que costuma cair:
- `irate` recebe **range vector** e devolve **instant vector**; usa **só as 2 últimas amostras** da janela.
- Serve para **counters** (compensa resets), como o `rate`.
- **`rate` para alertas e counters lentos; `irate` para gráficos de counters voláteis** (frase quase literal da documentação).
- A janela do `irate` só define "até onde procurar"; `[1m]` e `[5m]` dão o mesmo resultado se houver amostras recentes.
- Ordem com agregação: **`sum(irate(x[1m]))`**, nunca `irate(sum(x)[1m:])`.

**1.** Qual é a diferença essencial entre `rate(x[5m])` e `irate(x[5m])`?
- A) `irate` devolve o aumento total; `rate`, o aumento por segundo
- B) `irate` usa apenas as duas últimas amostras da janela; `rate` usa a primeira e a última (com extrapolação)
- C) `irate` funciona com gauges; `rate` só com counters
- D) `irate` não trata resets de counter

<details><summary>Resposta</summary>

**B.** Ambos são "por segundo" (A errada), ambos são para counters (C errada) e ambos compensam resets (D errada). O `irate` olha só o último intervalo entre amostras.
</details>

**2.** Para uma regra de alerta com `for: 10m` sobre taxa de erros, qual expressão é mais adequada?
- A) `irate(http_requests_total{code=~"5.."}[1m]) > 1`
- B) `rate(http_requests_total{code=~"5.."}[5m]) > 1`
- C) `increase(http_requests_total{code=~"5.."}[10s]) > 1`
- D) `delta(http_requests_total{code=~"5.."}[5m]) > 1`

<details><summary>Resposta</summary>

**B.** A documentação recomenda `rate` para alertas: mudanças breves no `irate` "resetam" a cláusula `for`. C usa janela curta demais (pode não ter 2 amostras) e D usa `delta` em counter (não compensa resets).
</details>

**3.** Com scrape a cada 15s, qual é o resultado de trocar `irate(x[1m])` por `irate(x[10m])`?
- A) O resultado fica 10× mais suave
- B) O resultado é praticamente o mesmo
- C) O resultado vira o total de 10 minutos
- D) A query falha

<details><summary>Resposta</summary>

**B.** O `irate` só usa as 2 últimas amostras; a janela maior só permite achá-las mesmo se houver falhas de scrape.
</details>

**4.** Qual expressão está correta para obter a taxa instantânea total de requisições de um serviço com vários pods?
- A) `irate(sum(http_requests_total)[1m:])`
- B) `sum(irate(http_requests_total[1m]))`
- C) `irate(sum by (pod) (http_requests_total))`
- D) `sum(http_requests_total) / 60`

<details><summary>Resposta</summary>

**B.** Primeiro `irate`, depois agregação; senão o restart de um pod derruba a soma e o `irate` não consegue detectar o reset corretamente. C nem é válida (falta range vector).
</details>

## 📝 Cola rápida

- `irate(counter[janela])` = (última − penúltima) / Δt, **por segundo**, compensa reset.
- Janela = só "até onde procurar"; use `[1m]` ou `$__rate_interval`.
- **Gráfico de counter nervoso → `irate`. Alerta / recording rule / counter lento → `rate`.**
- Zoom out (step > scrape) **perde picos** com `irate`.
- Sempre `sum(irate(...))`, nunca `irate(sum(...))`.

## 🔗 Relacionadas

[`rate()`](../rate/) · [`increase()`](../increase/) · [`idelta()`](../idelta/) · [`resets()`](../resets/) · [`deriv()`](../deriv/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#irate
