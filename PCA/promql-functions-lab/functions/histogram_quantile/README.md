# `histogram_quantile()`: percentis (p50, p90, p99) a partir de histogramas

> **Em uma frase:** `histogram_quantile(φ, b)` estima o valor abaixo do qual está a fração **φ** das observações de um histograma (ex.: `0.99` → "99% das requisições levaram até X segundos"). Funciona com histogramas **classic** (`x_bucket{le}`) e **native** (`x`).

| | |
|---|---|
| **Assinatura** | `histogram_quantile(φ scalar, b instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Histogram classic (`*_bucket` com label `le`) · ✅ Native histogram · ❌ Gauge/counter comum |
| **Unidade do resultado** | a mesma da métrica observada (segundos, bytes...) |
| **Dashboard** | http://localhost:3300/d/fn-histogram_quantile |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a chegada da maratona

Imagine a linha de chegada de uma maratona. Um fiscal não anota o tempo exato de cada corredor, ele só tem **caixas** etiquetadas: "chegou em até 2h", "até 2h30", "até 3h", "até 4h", "depois disso". Cada corredor joga uma bolinha na caixa certa.

No fim, alguém pergunta: **"até que tempo chegaram 99% dos corredores?"**

- O fiscal conta as bolinhas, acha a **caixa** onde está o corredor nº 99 de cada 100...
- ...e, como não sabe o tempo exato dentro da caixa, **estima** (interpola) um valor dentro dela.

Isso é o `histogram_quantile(0.99, ...)`:

- As **caixas** são os **buckets** do histograma.
- **Caixas largas** (classic com poucos buckets) = estimativa grosseira. **Caixas finas** (native, ~9% de largura aqui) = estimativa precisa.
- O `rate()` define **de quais corredores** estamos falando: "os que chegaram no último minuto", e não "todos desde que a prova começou".

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) usa `NativeHistogramBucketFactor: 1.1` **e** `Buckets`, então cada histograma aparece **duas vezes** no Prometheus: como **native** (`x`) e como **classic** (`x_bucket{le}`, `x_sum`, `x_count`).

| Métrica | Tipo | Comportamento |
|---|---|---|
| `histogram_quantile_http_request_duration_seconds{route="/api/users",pod="pod-a"}` | histogram | ~40 obs/s, log-normal, **mediana 50ms**, p99 ≈ **160ms** |
| `...{route="/api/users",pod="pod-b"}` | histogram | igual ao pod-a, mas **a cada 5 min, por 90s, fica lento**: mediana **400ms**, p99 ≈ **1.3s** |
| `...{route="/api/report",pod="pod-a"\|"pod-b"}` | histogram | ~5 obs/s cada, mediana **300ms**, p99 ≈ **0.76s** |
| `histogram_quantile_coarse_duration_seconds` | histogram | mesmo tráfego do `/api/users` pod-a, mas com buckets classic **grosseiros**: `0.1, 0.5, 1` |

Buckets classic da métrica principal: `0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, +Inf`.

Para ver o formato cru (classic, em texto):

```bash
curl -s localhost:8088/metrics | grep '^histogram_quantile_http_request_duration_seconds_bucket{pod="pod-a",route="/api/users"'
# ..._bucket{le="0.025",...} 1234
# ..._bucket{le="0.05",...}  5710      <- cumulativo: "até 50ms"
# ..._bucket{le="0.1",...}   10470
# ..._bucket{le="+Inf",...}  11420
```

> O native histogram só aparece no formato **protobuf** (o Prometheus negocia isso sozinho). No `curl` em texto você vê só o classic.

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-histogram_quantile
```

Espere **~2 min** para as janelas `[1m]` e **~5 min** para ver uma degradação completa do pod-b.

---

## 🔍 Queries passo a passo

### 1. O heatmap dos buckets (ver antes de calcular)

```promql
sum by (le) (rate(histogram_quantile_http_request_duration_seconds_bucket{route="/api/users"}[1m]))
```

**O que faz:** para cada bucket (`le`), quantas observações por segundo caem nele. No painel `heatmap` o Grafana transforma os buckets cumulativos em faixas.
**Resultado esperado:** uma faixa quente entre **25ms e 100ms** o tempo todo. A cada 5 min, por ~90s, **metade** do tráfego "sobe" para a faixa **250ms–1s** (é o pod-b).
**Moral:** o heatmap é a distribuição inteira; o quantil é **um número** tirado dela.

---

### 2. p50, p90 e p99 com histograma **classic**

```promql
histogram_quantile(0.5,  sum by (le) (rate(histogram_quantile_http_request_duration_seconds_bucket{route="/api/users"}[1m])))
histogram_quantile(0.9,  sum by (le) (rate(histogram_quantile_http_request_duration_seconds_bucket{route="/api/users"}[1m])))
histogram_quantile(0.99, sum by (le) (rate(histogram_quantile_http_request_duration_seconds_bucket{route="/api/users"}[1m])))
```

**O que faz (de dentro para fora):**
1. `rate(..._bucket[1m])`: observações/s em cada bucket, de **cada** pod (o bucket é um counter).
2. `sum by (le)`: junta os pods, **mantendo o `le`** (sem ele a função não sabe qual é o bucket).
3. `histogram_quantile(0.99, ...)`: acha o bucket onde está o 99º percentil e interpola **linearmente** dentro dele.

**Resultado esperado** (unidade: segundos):

| quantil | normal | pod-b degradado (90s a cada 5 min) |
|---|---|---|
| p50 | ≈ **0.05** | ≈ **0.14** |
| p90 | ≈ **0.095** | ≈ **0.6** |
| p99 | ≈ **0.23** ⚠️ (real: 0.16) | ≈ **1.6 a 1.7** ⚠️ (real: ≈1.1 a 1.2) |

> 💡 Por que o p99 classic "erra"? O 99º percentil real (160ms) cai no bucket `0.1 → 0.25`. A função supõe que as observações estão **espalhadas uniformemente** nesse bucket e chuta ~230ms. Veja o painel 3.

---

### 3. p99 **classic vs native** (os mesmos dados!)

```promql
# classic: precisa de _bucket e de "le"
histogram_quantile(0.99, sum by (le) (rate(histogram_quantile_http_request_duration_seconds_bucket{route="/api/users"}[1m])))
# native: a série é o histograma inteiro, sem _bucket e sem le
histogram_quantile(0.99, sum(rate(histogram_quantile_http_request_duration_seconds{route="/api/users"}[1m])))
```

**O que faz:** a mesma pergunta, feita aos dois formatos do mesmo histograma.
**Resultado esperado:**

| | normal | degradado |
|---|---|---|
| classic | ≈ 0.23s | ≈ 1.6 a 1.7s |
| native | ≈ **0.16s** | ≈ **1.1 a 1.2s** |

O native tem **buckets exponenciais finos** (fator 2^(1/8) ≈ 1.09, ou seja, cada bucket é ~9% maior que o anterior) e usa **interpolação exponencial**, então chega muito perto do valor real. **Repare:** no native não existe `le` e o `sum` fica **sem `by`**.

---

### 4. Quebrando por pod: quem está lento?

```promql
histogram_quantile(0.99, sum by (pod) (rate(histogram_quantile_http_request_duration_seconds{route="/api/users"}[1m])))
```

**Resultado esperado:** `pod-a` ≈ **0.16s** reto; `pod-b` ≈ **0.16s** que salta para ≈ **1.3 a 1.5s** por 90s a cada 5 min.
Com classic seria `sum by (pod, le) (rate(..._bucket[1m]))`.

---

### 5. Por rota (classic: `le` sempre no `by`)

```promql
histogram_quantile(0.99, sum by (route, le) (rate(histogram_quantile_http_request_duration_seconds_bucket[1m])))
```

**Resultado esperado:** `/api/report` ≈ **0.9s** (valor real ≈ 0.76s; de novo a interpolação linear no bucket `0.5 → 1`) e `/api/users` igual ao painel 2.
⚠️ Se você escrever `sum by (route)` (sem `le`) o resultado é **vazio**: a função ignora floats sem `le`.

---

### 6. ❌ Média de percentis não é percentil

```promql
avg(histogram_quantile(0.99, sum by (pod) (rate(histogram_quantile_http_request_duration_seconds{route="/api/users"}[1m]))))   # ❌
histogram_quantile(0.99, sum(rate(histogram_quantile_http_request_duration_seconds{route="/api/users"}[1m])))                  # ✅
```

**Resultado esperado:** fora da degradação as duas ficam em ≈ 0.16s. **Na degradação:** a errada dá `(0.16 + 1.4) / 2 ≈ 0.8s`; a certa dá ≈ **1.2s**.
**Regra:** some (agregue) os **histogramas/buckets** primeiro e calcule o quantil **por último**. Nunca `avg`/`sum`/`max` de quantis.

---

### 7. ❌ Esqueceu o `rate()`

```promql
histogram_quantile(0.99, sum(histogram_quantile_http_request_duration_seconds{route="/api/users"}))            # ❌ acumulado
histogram_quantile(0.99, sum(rate(histogram_quantile_http_request_duration_seconds{route="/api/users"}[1m])))  # ✅ último minuto
```

**Resultado esperado:** a versão sem `rate` é o p99 de **tudo desde que o processo subiu**. Ela anda devagar e mal reage à degradação (sobe um pouco e demora a descer). A com `rate` mostra o degrau nítido.

> 💡 O "acumulado" começa do zero a cada restart do processo. Logo depois de um restart (ou de um deploy do lab) as duas linhas ficam parecidas; com o processo rodando por horas, a linha sem `rate` fica praticamente reta.

---

### 8. Buckets classic grosseiros = erro grande

```promql
histogram_quantile(0.99, rate(histogram_quantile_coarse_duration_seconds_bucket[1m]))   # classic: 0.1, 0.5, 1
histogram_quantile(0.99, rate(histogram_quantile_coarse_duration_seconds[1m]))          # native
```

**Resultado esperado:** p99 real ≈ **0.16s**. O classic diz ≈ **0.45s** (quase 3× mais!), porque o bucket `0.1 → 0.5` é enorme e a função supõe distribuição uniforme dentro dele. O native diz ≈ **0.16s**.
**Moral:** com classic, **escolha buckets perto dos valores que importam** (ex.: seu SLO). Ou use native.

---

### 9. 🔴 Caso real: o alerta de SLO por pod

```promql
histogram_quantile(0.99, sum by (pod, le) (rate(histogram_quantile_http_request_duration_seconds_bucket{route="/api/users"}[5m])))
vector(0.3)   # limite do SLO: p99 ≤ 300ms
```

**O que faz:** é exatamente a expressão de um alerta de produção (`... > 0.3` com `for: 10m`), agora com janela de **5 min** como se usa em regras.
**Resultado esperado:**
- `pod-a` ≈ **0.23s** (classic; o real é 0.16s), abaixo da linha: **nunca dispara**.
- `pod-b` ≈ **1.0 a 1.1s** **o tempo todo**, sem voltar ao normal entre as degradações!

**Por quê?** A degradação dura 90s e se repete a cada 5 min. Uma janela de `[5m]` **sempre** contém esses 90s, e 30% de requisições lentas bastam para dominar o p99. Com `[1m]` (painel 4) o pod-b volta a ~0.16s entre os episódios.
**Moral:** a janela do `rate` decide o que o alerta enxerga. Janela longa = alerta estável, mas que também "lembra" do problema por mais tempo (e aqui não esquece nunca). Pense no período dos seus incidentes ao escolher `[5m]`, `[30m]`, `[1h]`.

---

## 🏭 Casos reais

> O cenário fake imita esses casos: `histogram_quantile_http_request_duration_seconds{route,pod}` é o `http_request_duration_seconds` de um serviço com 2 réplicas, uma delas com "vizinho barulhento".

### Caso 1: SLO de latência do checkout (p99 < 300ms)

Serviço instrumentado com `promhttp`/Micrometer/OpenTelemetry expõe `http_server_request_duration_seconds_bucket{job, http_route, le}` (classic). O time grava o p99 numa **recording rule** e alerta sobre ela:

```yaml
groups:
  - name: checkout-latency
    rules:
      # 1) agrega os buckets (mantendo le) -> barato de consultar depois
      - record: job_route_le:http_server_request_duration_seconds_bucket:rate5m
        expr: sum by (job, http_route, le) (rate(http_server_request_duration_seconds_bucket[5m]))
      # 2) quantil sobre a série já agregada
      - record: job_route:http_server_request_duration_seconds:p99_5m
        expr: histogram_quantile(0.99, job_route_le:http_server_request_duration_seconds_bucket:rate5m)

      - alert: CheckoutLatencyHigh
        expr: job_route:http_server_request_duration_seconds:p99_5m{job="checkout"} > 0.3
        for: 10m
        labels: { severity: page }
        annotations:
          summary: "p99 de {{ $labels.http_route }} = {{ $value | humanizeDuration }}"
```

Decisão: o bucket **0.3** precisa existir na instrumentação (senão o p99 perto de 300ms é interpolado com erro grande, painel 8). Por isso a regra de ouro: **coloque buckets perto do seu SLO**.

### Caso 2: etcd com disco lento (alerta real do etcd-mixin)

```yaml
- alert: etcdHighFsyncDurations
  expr: histogram_quantile(0.99, rate(etcd_disk_wal_fsync_duration_seconds_bucket{job=~".*etcd.*"}[5m])) > 0.5
  for: 10m
  labels: { severity: warning }
```

Sem `sum by (le)`: aqui se quer o p99 **por instância** do etcd (cada membro tem seu disco), então os labels originais (`instance`, `le`) são mantidos. É exatamente o painel 4 (p99 por pod): o membro com disco ruim aparece sozinho.

### Caso 3: API server do Kubernetes

```promql
histogram_quantile(0.99,
  sum by (le, verb, resource) (
    rate(apiserver_request_duration_seconds_bucket{job="apiserver", verb=~"GET|LIST"}[5m])))
```

É a base das regras `cluster_quantile:apiserver_request_duration_seconds:histogram_quantile` do kube-prometheus. Note o `le` junto dos labels de agrupamento.

### Caso 4: migração para native histograms

Com native histograms (Prometheus 3.x, `scrape_native_histograms: true`), a mesma consulta vira:

```promql
histogram_quantile(0.99, sum by (job) (rate(http_server_request_duration_seconds[5m])))
```

Sem `_bucket`, sem `le`, e com precisão muito maior (painel 3). Enquanto houver dashboards antigos, dá para manter os dois formatos com `always_scrape_classic_histograms: true` (como neste lab).

---

## ✅ Quando usar

- **Latência p50/p95/p99** de APIs, filas, queries de banco: `histogram_quantile(0.99, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))`.
- **Alertas de SLO de latência** ("p99 > 500ms por 10 min").
- **Comparar pods/rotas/regiões** para achar quem degrada (painel 4).
- **Tamanhos** (bytes de resposta, tamanho de lote): qualquer histograma serve, não só tempo.
- `histogram_quantile(0, ...)` e `histogram_quantile(1, ...)` dão o **mínimo e o máximo estimados**.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer **vários quantis** de uma vez (p50, p90, p99) | [`histogram_quantiles()`](../histogram_quantiles/) |
| Quer "**que % ficou abaixo de 200ms**" (SLO, Apdex) | [`histogram_fraction()`](../histogram_fraction/) (a pergunta inversa) |
| Quer a **média** | [`histogram_avg()`](../histogram_avg/) ou `rate(x_sum)/rate(x_count)` |
| A métrica é um **summary** (já vem com `quantile="0.99"`) | Use a série direto; summaries **não** são agregáveis |
| Quer um quantil de uma **gauge** ao longo do tempo | [`quantile_over_time()`](../quantile_over_time/) |

## ⚠️ Pegadinhas

1. **Esquecer o `le` no `by`** (classic): `sum by (job) (rate(x_bucket[5m]))` → resultado vazio. Tem que ser `sum by (job, le)`.
2. **Colocar `le` com native**: native não tem `le`; use `sum by (job) (rate(x[5m]))`.
3. **Sem `rate()`**: vira o quantil "desde sempre" (painel 7).
4. **Média de quantis** (painel 6): matematicamente errado.
5. **Quantil no bucket `+Inf`** (classic): a função devolve o **limite do penúltimo bucket** (ex.: `5`). Se o seu p99 fica "grudado" num número redondo, seus buckets são curtos demais.
6. **Resultado é estimativa**: classic interpola **linearmente**; native interpola **exponencialmente** (bem mais preciso). Nenhum dos dois é o valor exato.
7. **Sem observações na janela** → `NaN` (ex.: rota sem tráfego). No Grafana aparece como buraco.
8. **φ fora de 0..1**: `φ < 0` → `-Inf`, `φ > 1` → `+Inf`. Use `0.99`, não `99`!
9. **Mensagem `input to histogram_quantile needed to be fixed for monotonicity`**: buckets classic não cumulativos, geralmente por mistura de séries diferentes. Investigue a origem.

## 🎓 Na prova PCA

O que costuma cair:
- Um histograma **classic** gera `<nome>_bucket{le="..."}`, `<nome>_sum` e `<nome>_count`. Os buckets são **cumulativos** e o `le="+Inf"` é igual ao `_count`.
- Fórmula canônica: `histogram_quantile(0.95, sum by (le) (rate(x_bucket[5m])))`. **`rate` dentro**, **`le` no `by`**.
- φ vai de **0 a 1** (`0.95`, não `95`).
- **Histogram vs Summary**: summary calcula quantis **no cliente** (`x{quantile="0.99"}`) e **não pode ser agregado** entre instâncias; histogram calcula no servidor e **pode** (somando buckets).
- O resultado é uma **estimativa** (interpolação dentro do bucket).

**1.** Qual query calcula corretamente o p95 de latência de todas as instâncias de um job?
- A) `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))`
- B) `histogram_quantile(0.95, sum by (job) (rate(http_request_duration_seconds_bucket[5m])))`
- C) `histogram_quantile(0.95, sum by (job, le) (rate(http_request_duration_seconds_bucket[5m])))`
- D) `avg by (job) (histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])))`

<details><summary>Resposta</summary>

**C.** Soma os buckets de todas as instâncias mantendo `le` e `job`. A dá um p95 **por instância**. B perde o `le` (resultado vazio). D tira média de quantis, o que é matematicamente errado (painel 6).
</details>

**2.** Os buckets de um histograma classic são `0.1, 0.5, 1, +Inf`. O p99 real está em 2s. O que `histogram_quantile(0.99, ...)` retorna?
- A) 2
- B) +Inf
- C) 1
- D) NaN

<details><summary>Resposta</summary>

**C.** Se o quantil cai no bucket `+Inf`, a função devolve o limite superior do **penúltimo** bucket (1). É o sinal de que os buckets não cobrem a faixa de valores.
</details>

**3.** Por que não se pode agregar `http_request_duration_seconds{quantile="0.99"}` de 10 pods com `avg`?
- A) Porque é um counter
- B) Porque é um summary: quantis calculados no cliente não são agregáveis
- C) Porque falta o label `le`
- D) Pode sim, `avg` de quantis é correto

<details><summary>Resposta</summary>

**B.** A média de 10 p99 não é o p99 do conjunto. Summaries são precisos por instância, mas não se combinam. Para agregar, use **histogram**.
</details>

**4.** O que acontece com `histogram_quantile(0.9, sum by (le) (http_request_duration_seconds_bucket))` (sem `rate`)?
- A) Erro de parse
- B) Retorna o p90 de todas as requisições desde que cada processo subiu
- C) Retorna o p90 dos últimos 5 minutos
- D) Retorna vazio

<details><summary>Resposta</summary>

**B.** É válido, mas os buckets são counters acumulados: o resultado é o quantil "desde sempre" e quase não reage a mudanças (painel 7).
</details>

**5.** Com native histograms, como fica a query do p99 agregado por `job`?
- A) `histogram_quantile(0.99, sum by (job, le) (rate(x[5m])))`
- B) `histogram_quantile(0.99, sum by (job) (rate(x[5m])))`
- C) `histogram_quantile(0.99, sum by (job) (rate(x_bucket[5m])))`
- D) Native histograms não suportam `histogram_quantile`

<details><summary>Resposta</summary>

**B.** Native histogram é **uma série** com todos os buckets dentro; não existe `le` nem `_bucket`.
</details>

## 📝 Cola rápida

- Classic: `histogram_quantile(φ, sum by (<labels>, le) (rate(x_bucket[5m])))`. Native: `histogram_quantile(φ, sum by (<labels>) (rate(x[5m])))`.
- φ entre 0 e 1. Resultado é **estimativa** (linear no classic, exponencial no native).
- Agregue **buckets**, nunca quantis. Summary não agrega; histogram agrega.
- Quantil no bucket `+Inf` → devolve o maior limite finito. Coloque buckets perto do SLO.
- Sem observações → `NaN`. Sem `rate` → "desde sempre".

## 🔗 Relacionadas

[`histogram_quantiles()`](../histogram_quantiles/) · [`histogram_fraction()`](../histogram_fraction/) · [`histogram_avg()`](../histogram_avg/) · [`histogram_count()`](../histogram_count/) · [`histogram_stddev()`](../histogram_stddev/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantile
· https://prometheus.io/docs/practices/histograms/
