# `resets()`: quantas vezes um counter voltou para trás

> **Em uma frase:** `resets(v[janela])` conta quantas vezes, dentro da janela, uma amostra foi **menor** que a anterior. Num counter, cada queda é um **reset**, quase sempre um restart do processo.

| | |
|---|---|
| **Assinatura** | `resets(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Counter (floats e native histograms) · ❌ Gauge |
| **Unidade do resultado** | número de resets (inteiro, sem extrapolação) |
| **Dashboard** | http://localhost:3300/d/fn-resets |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: quantas vezes você trocou de carro?

O counter é o **hodômetro**: só cresce. Ele só volta a zero quando você **troca de carro** (o processo reinicia e a memória com o valor do counter se perde).

`resets(x[10m])` responde: "**quantas vezes eu troquei de carro** nos últimos 10 minutos?".

```
amostras:  100  110  120   3   13   23   2   12
                           ↑             ↑
                        queda         queda        → resets = 2
```

- Não importa o **tamanho** da queda: `120 → 3` e `120 → 119` contam igual (1 reset cada).
- Não há extrapolação: o resultado é sempre um **inteiro**.
- Em native histograms, conta como reset qualquer queda em algum bucket ou no count, sumiço de bucket, mudança de schema incompatível etc.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) calcula os valores a partir do relógio, então os "restarts" são programados (e um restart real do gerador **não** cria resets extras):

| Métrica | Tipo | Comportamento |
|---|---|---|
| `resets_process_cpu_seconds_total{pod="estavel"}` | counter | imita `process_cpu_seconds_total`: sobe 0.5/s, **nunca** reinicia |
| `resets_process_cpu_seconds_total{pod="instavel"}` | counter | sobe 0.5/s, reinicia a **cada 2 min** |
| `resets_process_cpu_seconds_total{pod="crashloop"}` | counter | sobe 0.5/s, reinicia a **cada 30s** (CrashLoopBackOff) |
| `resets_kube_pod_container_status_restarts_total{pod="checkout"}` | counter | imita o kube-state-metrics: **conta** restarts, +1 a cada 2 min (nunca cai) |
| `resets_rabbitmq_queue_messages` | **gauge** | fila que sobe e desce (onda de 2 min, 200 ↔ 800) para a pegadinha |

```bash
curl -s localhost:8088/metrics | grep '^resets_'
# resets_process_cpu_seconds_total{pod="crashloop"} 11.5
# resets_process_cpu_seconds_total{pod="estavel"} 187261.5
# resets_process_cpu_seconds_total{pod="instavel"} 41.5
# resets_rabbitmq_queue_messages 541
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-resets
```

Espere **~10 minutos** para a janela `[10m]` ficar "cheia" (antes disso os números são menores).

---

## 🔍 Queries passo a passo

### 1. Os counters crus

```promql
resets_process_cpu_seconds_total{pod!="estavel"}
```

**Resultado esperado:** `instavel` é um dente-de-serra que vai até ~**60** e cai a 0 a cada 2 min; `crashloop` é um serrote fino que vai até ~**15** e cai a cada 30s. (O `estavel` foi escondido do gráfico porque está em ~190 mil e achataria os outros.)

---

### 2. Restarts nos últimos 10 minutos

```promql
resets(resets_process_cpu_seconds_total[10m])
```

**O que faz:** percorre as ~120 amostras de cada série na janela e conta as quedas.
**Resultado esperado:**

| pod | resets em 10 min | conta |
|---|---|---|
| `estavel` | **0** | nunca cai |
| `instavel` | **5** | 600s / 120s |
| `crashloop` | **20** | 600s / 30s |

> 💡 O número "escada" (ex.: 19 → 20 → 19) conforme restarts entram e saem da janela. É inteiro sempre.

---

### 3. Alerta de crashloop

```promql
resets(resets_process_cpu_seconds_total[5m]) > 3
```

**Resultado esperado:** só `crashloop` aparece (≈ **10** resets em 5 min). `instavel` tem **2 ou 3** e fica de fora; `estavel` tem 0.

Numa regra de alerta:

```yaml
- alert: PodEmCrashLoop
  expr: resets(process_cpu_seconds_total[5m]) > 3
  for: 5m
```

---

### 4. Barras: restarts por pod (5m e 10m)

```promql
resets(resets_process_cpu_seconds_total[5m])
resets(resets_process_cpu_seconds_total[10m])
```

**Resultado esperado:** `estavel` 0 / 0 · `instavel` 2-3 / 5 · `crashloop` 10 / 20.

---

### 5 e 6. Pegadinha: `resets()` num gauge

```promql
resets_rabbitmq_queue_messages               # gauge: sobe e desce
resets(resets_rabbitmq_queue_messages[5m])   # "resets" que não existem
```

**Resultado esperado:** a fila é uma onda entre ~200 e ~800. Metade do tempo ela **desce**, e cada descida entre dois scrapes conta como "reset": em 5 min (60 amostras) o resultado fica entre ≈ **23 e 35**. Esse número não significa nada.

Para gauges, a pergunta certa costuma ser "quantas vezes **mudou**?" → [`changes()`](../changes/), ou "quanto mudou?" → [`delta()`](../delta/).

---

### 7. O que dá errado: `resets()` num counter que **conta** restarts

```promql
resets(resets_kube_pod_container_status_restarts_total[10m])     # errado
increase(resets_kube_pod_container_status_restarts_total[10m])   # certo
```

**O que acontece:** o `kube_pod_container_status_restarts_total` já **é** a contagem de restarts: ele **sobe** 1 a cada restart e nunca cai. Não há "queda" para o `resets()` contar.
**Resultado esperado:** `resets` = **0** o tempo todo; `increase[10m]` ≈ **5** (entre 4.0 e 5.0 pela extrapolação), que é o número real de restarts em 10 min.

---

## 🏭 Casos reais

### 1. Detectar restarts de qualquer app instrumentado (imitado pelos painéis 1-4)

Toda client library do Prometheus expõe `process_cpu_seconds_total` (e `process_start_time_seconds`). Se o processo reinicia, o counter de CPU volta a zero. Então, mesmo sem Kubernetes, dá para saber quem está reiniciando:

```yaml
groups:
- name: process
  rules:
  - alert: ProcessoReiniciandoMuito
    expr: resets(process_cpu_seconds_total{job="payments"}[15m]) > 2
    for: 5m
    labels: {severity: warning}
    annotations:
      summary: "{{ $labels.instance }} reiniciou {{ $value }}x nos últimos 15 min"
```

**Alternativa equivalente e muito usada:** `changes(process_start_time_seconds[15m]) > 2` (o timestamp de start muda a cada restart; ver [`changes()`](../changes/)).

### 2. Kubernetes: `resets` × `increase` no counter de restarts (imitado pelo painel 7)

No k8s, o kube-state-metrics já **conta** restarts:

```promql
increase(kube_pod_container_status_restarts_total[1h]) > 3   # ✅ quantos restarts
resets(kube_pod_container_status_restarts_total[1h])         # ❌ quantas vezes o próprio counter zerou (restart do kube-state-metrics!)
```

**Decisão:** se a métrica **é** um contador de restarts, use `increase`. Use `resets` quando você só tem um counter "qualquer" do processo. Uma regra típica (combinando os dois sinais do kube-state-metrics):

```yaml
- alert: KubePodCrashLooping
  expr: increase(kube_pod_container_status_restarts_total[10m]) > 0
        and on(namespace, pod, container) kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff"} == 1
  for: 15m
  labels: {severity: warning}
```

### 3. Qualidade do `rate()` em dashboards de throughput

Um SRE nota que o `rate(http_requests_total[5m])` de um serviço tem "buracos". Ele confere:

```promql
sum by (instance) (resets(http_requests_total{job="api"}[1h]))
```

`instance="api-3"` tem 40 resets na última hora: o pod está em crashloop e cada restart perde as requisições entre o último scrape e o crash.

## ✅ Quando usar

- **Detectar restarts** de um processo a partir de qualquer counter dele: `resets(process_cpu_seconds_total[1h]) > 0`.
- **Crashloop:** `resets(x[15m]) > 3`.
- **Auditar a qualidade de um `rate()`:** se `resets()` está alto, o `rate`/`increase` estão compensando muitos resets (e perdendo o que aconteceu entre o último scrape e o crash).
- **Detectar bugs de instrumentação:** um counter que "desce" sem restart (alguém usou `Set()` ou `Add(-1)` num counter).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **gauge** | [`changes()`](../changes/) |
| Quer a **velocidade** do counter (com resets compensados) | [`rate()`](../rate/) |
| Quer o **total** na janela | [`increase()`](../increase/) |
| Kubernetes: já existe um counter de restarts | `increase(kube_pod_container_status_restarts_total[1h])` (ver [`increase()`](../increase/)) |

## ⚠️ Pegadinhas

1. **Restart entre dois scrapes que "volta alto":** se o processo reinicia e, antes do próximo scrape, o counter já passou do valor anterior, a queda **nunca é vista**. Com counters lentos e scrape espaçado isso é raro; com counters rápidos pode acontecer.
2. **Série nova ≠ reset:** se o restart muda algum label (ex.: `pod="api-7f9c"` → `pod="api-8a1b"`), são **duas séries diferentes**; nenhuma delas tem queda, `resets()` = 0. Em Kubernetes isso é comum: agregue por algo estável ou use `kube_pod_container_status_restarts_total`.
3. **Não tem extrapolação** (diferente de `increase`), então janelas diferentes dão resultados proporcionais só "em média".
4. **Qualquer queda conta:** `1000 → 999` é um reset. Em gauges, isso vira lixo (query 6).
5. **Troca float ↔ histogram** também conta como reset.

## 🎓 Na prova PCA

O que costuma cair:
- `resets` recebe **range vector**, devolve **instant vector** com o **número de resets** (inteiro).
- **Qualquer queda** entre duas amostras consecutivas é um reset.
- É para **counters**. Num gauge o número não significa nada (cada descida conta).
- Diferença com `changes`: `changes` conta **toda** mudança (subida ou descida); `resets` só **quedas**.
- `rate`/`increase`/`irate` **já compensam** resets automaticamente: você não precisa de `resets()` para corrigi-los.

**1.** Dadas as amostras `5, 8, 2, 4, 4, 1, 7` numa janela, qual é o resultado de `resets()`?
- A) 1
- B) 2
- C) 3
- D) 5

<details><summary>Resposta</summary>

**B.** Quedas: `8 → 2` e `4 → 1`. `4 → 4` não é queda.
</details>

**2.** E qual seria o resultado de `changes()` para as mesmas amostras `5, 8, 2, 4, 4, 1, 7`?
- A) 2
- B) 4
- C) 5
- D) 6

<details><summary>Resposta</summary>

**C.** Mudanças: 5→8, 8→2, 2→4, 4→1, 1→7 = 5. O par `4 → 4` não conta.
</details>

**3.** Para qual tipo de métrica `resets()` deve ser usada?
- A) Gauge
- B) Counter
- C) Summary quantile (`{quantile="0.9"}`)
- D) Qualquer tipo, o resultado é sempre significativo

<details><summary>Resposta</summary>

**B.** A documentação diz que `resets` deve ser usada apenas com counters. Em gauges e quantis de summary, quedas são normais.
</details>

**4.** Você precisa alertar quando um container do Kubernetes reiniciar mais de 3 vezes em 1 hora. Qual query?
- A) `resets(kube_pod_container_status_restarts_total[1h]) > 3`
- B) `increase(kube_pod_container_status_restarts_total[1h]) > 3`
- C) `changes(kube_pod_container_status_restarts_total[1h]) > 3`
- D) `rate(kube_pod_container_status_restarts_total[1h]) > 3`

<details><summary>Resposta</summary>

**B.** A métrica conta restarts, então queremos o aumento. A mede restarts do exporter; C funcionaria "quase", mas é semanticamente errado e pode divergir (dois restarts entre scrapes contam 1 mudança); D dá restarts por segundo.
</details>

## 📝 Cola rápida

- `resets(counter[janela])` = nº de **quedas** entre amostras consecutivas (inteiro, sem extrapolação).
- Sinal de **restart** do processo; alerta de crashloop: `resets(x[15m]) > 3`.
- Não use em gauge. Para gauges: `changes` (quantas mudanças) / `delta` (quanto mudou).
- `rate`/`increase` já compensam resets sozinhos.
- k8s: counter de restarts → `increase(kube_pod_container_status_restarts_total[...])`.

## 🔗 Relacionadas

[`changes()`](../changes/) · [`rate()`](../rate/) · [`increase()`](../increase/) · [`irate()`](../irate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#resets
