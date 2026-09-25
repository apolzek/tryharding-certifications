# `rate()`: a velocidade média de um counter

> **Em uma frase:** `rate(v[janela])` diz quantas vezes **por segundo**, em média, um counter cresceu dentro da janela. Resets (restart do processo) são compensados automaticamente.

| | |
|---|---|
| **Assinatura** | `rate(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Counter (`*_total`, `*_count`, `*_sum`, `*_bucket`, native histograms) · ❌ Gauge |
| **Unidade do resultado** | "unidades por segundo" (req/s, bytes/s, jobs/s...) |
| **Dashboard** | http://localhost:3300/d/fn-rate |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: hodômetro e velocímetro

Pense no **hodômetro** do carro: ele só **aumenta** (`123.456 km`). Olhar pra esse número não diz se você está parado no trânsito ou a 120 km/h.

O **velocímetro** responde "a que velocidade estou indo?", e isso é o `rate()`:

```
                 (km no fim da janela) − (km no começo da janela)
velocidade  =   ─────────────────────────────────────────────────
                          duração da janela em segundos
```

- O counter `http_requests_total` é o **hodômetro** (total acumulado desde que o processo subiu).
- `rate(http_requests_total[1m])` é o **velocímetro**: requisições **por segundo**, na média do último minuto.
- Se você **troca o carro** (restart do pod), o hodômetro volta para zero. O `rate()` percebe a queda e **não** calcula uma velocidade negativa: ele entende que houve reset.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) cria duas métricas:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `rate_http_requests_total{route="/home"}` | counter | cresce ~**10/s** constante |
| `rate_http_requests_total{route="/checkout"}` | counter | cresce ~**2/s** constante |
| `rate_http_requests_total{route="/api/search"}` | counter | ~**5/s**, com **pico de 40/s durante 60s a cada 5 min** |
| `rate_worker_jobs_processed_total{pod="worker-a"}` | counter | cresce **2/s** e **volta a zero a cada 3 min** (simula restart) |

Para ver o formato cru que o Prometheus raspa:

```bash
curl -s localhost:8088/metrics | grep '^rate_'
# rate_http_requests_total{route="/home"} 18234
# rate_worker_jobs_processed_total{pod="worker-a"} 214
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-rate
```

Espere **~2 minutos** para ter dados suficientes nas janelas de `[1m]` e ~5 min para `[5m]`.

---

## 🔍 Queries passo a passo

### 1. O counter cru (o hodômetro)

```promql
rate_http_requests_total
```

**O que faz:** mostra o valor acumulado.
**Resultado esperado:** 3 linhas **sempre subindo**, cada uma com uma inclinação diferente. Dá pra ver que `/home` sobe mais rápido, mas é difícil ler números úteis disso.
**Moral:** gráfico de counter cru quase nunca é o que você quer.

---

### 2. Velocidade por rota

```promql
rate(rate_http_requests_total[1m])
```

**O que faz:** para cada série, pega as amostras do último minuto e calcula `(último − primeiro) / segundos`, extrapolando para cobrir a janela inteira.
**Resultado esperado** (unidade: req/s):

| route | valor |
|---|---|
| `/home` | ≈ **10** |
| `/checkout` | ≈ **2** |
| `/api/search` | ≈ **5**, subindo para ≈ **40** durante o pico |

> 💡 O resultado é **fracionado** (ex.: `9.87`) por causa do ruído e da **extrapolação**. É normal e esperado.

---

### 3. O tamanho da janela muda a história

```promql
rate(rate_http_requests_total{route="/api/search"}[30s])   # nervoso
rate(rate_http_requests_total{route="/api/search"}[5m])    # suave
```

**Analogia:** a janela é o **tempo de exposição de uma foto**. Exposição curta congela o movimento (mostra cada detalhe e cada tremida); exposição longa borra tudo numa média.

**Resultado esperado:**
- `[30s]`: o pico aparece como um **degrau alto e nítido** (~40) que dura ~60s.
- `[5m]`: o pico vira uma **lombada baixa e larga** (algo como 5→~12→5), porque 60s de pico ficam diluídos em 300s.

**Regra prática:** a janela deve ter **pelo menos 4× o `scrape_interval`**. Aqui o scrape é 5s, então `[20s]` é o mínimo seguro e `[1m]` é confortável. Com scrape de 15s (o padrão), use `[1m]` ou mais.

---

### 4. Somando rotas: primeiro `rate`, depois `sum`

```promql
sum(rate(rate_http_requests_total[1m]))
```

**Resultado esperado:** uma linha em ≈ **17 req/s** (10 + 2 + 5), subindo para ≈ **52** no pico.

⚠️ **Nunca** faça `rate(sum(rate_http_requests_total)[1m:])`. Se **um** pod reiniciar, a **soma** cai, e o `rate` enxerga isso como um reset do total inteiro, gerando um **pico falso** gigante. Regra de ouro: **rate primeiro, agregação depois**.

---

### 5 e 6. Counter com reset (restart de pod)

```promql
rate_worker_jobs_processed_total              # cru: dente-de-serra
rate(rate_worker_jobs_processed_total[1m])    # velocidade: linha reta em ~2
```

**Resultado esperado:**
- **Cru:** um **dente-de-serra**: sobe até ~360 e despenca para 0 a cada 3 minutos.
- **Com `rate`:** uma linha **estável em ≈ 2 jobs/s**. Quando o valor cai (ex.: `358 → 2`), o `rate()` entende que é um reset, assume que o counter recomeçou de 0 e soma o que veio depois.

Compare com [`resets()`](../resets/), que **conta** quantos desses resets aconteceram.

---

### 7. Porcentagem do tráfego por rota

```promql
sum by (route) (rate(rate_http_requests_total[1m]))
  / scalar(sum(rate(rate_http_requests_total[1m])))
```

**Resultado esperado:** `/home` ≈ 59%, `/api/search` ≈ 29%, `/checkout` ≈ 12% (fora do pico).

---

## 🏭 Casos reais

### Caso 1: e-commerce na Black Friday (RED: Rate, Errors, Duration)

O time de SRE quer saber **requisições por segundo** e **taxa de erro** do checkout. A métrica real vem de bibliotecas como `promhttp` e Spring Actuator: `http_requests_total{job, handler, code}`.

```promql
# Throughput por serviço
sum by (job) (rate(http_requests_total[5m]))

# Taxa de erro (0..1): 5xx sobre o total
  sum by (job) (rate(http_requests_total{code=~"5.."}[5m]))
/
  sum by (job) (rate(http_requests_total[5m]))
```

Regra de alerta usada de verdade (padrão dos runbooks do kube-prometheus / SLO):

```yaml
groups:
  - name: checkout-slo
    rules:
      # recording rule: pré-calcula a taxa "por segundo" (convenção nível:métrica:operação)
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
      - record: job:http_requests_errors:rate5m
        expr: sum by (job) (rate(http_requests_total{code=~"5.."}[5m]))

      - alert: HighErrorRate
        expr: job:http_requests_errors:rate5m / job:http_requests:rate5m > 0.05
        for: 10m
        labels: { severity: page }
        annotations:
          summary: "{{ $labels.job }} com {{ $value | humanizePercentage }} de erros"
```

> Por que `rate` e não `increase` na recording rule? A doc recomenda: **`rate` em regras** (sempre por segundo, comparável entre janelas); `increase` para leitura humana.

### Caso 2: uso de CPU com o node_exporter

`node_cpu_seconds_total{cpu, mode}` é um **counter de segundos**. O `rate` de "segundos por segundo" dá a **fração de tempo** em cada modo:

```promql
# % de CPU ocupada por instância (100 - idle)
100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))
```

Resultado típico: `23.4` (23% ocupada). É **a query mais famosa do Prometheus** e cai muito em prova.

Como aparece nas regras do **node-mixin** (recording rule + alerta):

```yaml
groups:
  - name: node-cpu
    rules:
      - record: instance:node_cpu_utilisation:rate5m
        expr: 1 - avg without (cpu) (sum without (mode) (rate(node_cpu_seconds_total{mode=~"idle|iowait|steal"}[5m])))
      - alert: NodeHighCPU
        expr: instance:node_cpu_utilisation:rate5m > 0.9
        for: 15m
        labels: { severity: warning }
        annotations:
          summary: "CPU de {{ $labels.instance }} em {{ $value | humanizePercentage }}"
```

### Caso 3: tráfego de rede

```promql
# bits por segundo recebidos (bytes * 8)
rate(node_network_receive_bytes_total{device!~"lo|veth.*"}[5m]) * 8
```

### Caso 4: pod em CrashLoop (restarts como counter)

`kube_pod_container_status_restarts_total` só sobe. Versões antigas do alerta `KubePodCrashLooping` (kubernetes-mixin) usavam `rate(kube_pod_container_status_restarts_total[10m]) * 60 * 5 > 0`, ou seja, "restarts por segundo × 300s = restarts em 5 min". Depois o alerta passou a usar `increase(...)`, que é o mesmo cálculo mais legível, e as versões atuais olham `kube_pod_container_status_waiting_reason{reason="CrashLoopBackOff"}`. Veja [`increase()`](../increase/).

---

## ✅ Quando usar

- **Throughput:** req/s, mensagens/s, bytes/s (`rate(node_network_receive_bytes_total[5m])`).
- **Taxa de erro:** `sum(rate(errors_total[5m])) / sum(rate(requests_total[5m]))`.
- **Alertas e recording rules:** o `rate()` é estável e bem-comportado. É **a** escolha para alertas.
- **Base para histogramas:** `histogram_quantile(0.99, sum by (le) (rate(x_bucket[5m])))`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **gauge** (temperatura, memória, fila) | [`deriv()`](../deriv/) ou [`delta()`](../delta/) |
| Quer o **total** na janela ("quantos erros na última hora?") | [`increase()`](../increase/) |
| Gráfico **muito** reativo de counter volátil | [`irate()`](../irate/) |
| Alertar que a métrica **sumiu** | [`absent()`](../absent/) |

## ⚠️ Pegadinhas

1. **Janela curta demais** (`[10s]` com scrape de 15s): se houver < 2 amostras na janela, **nenhum resultado**.
2. **Extrapolação:** `rate` e `increase` extrapolam até as bordas da janela, então valores inteiros viram fracionados. Não é bug.
3. **`rate(sum(...))`:** errado, ver query 4.
4. **Usar em gauge:** qualquer queda do gauge vira "reset" e o número fica sem sentido.
5. **`$__rate_interval` no Grafana:** em dashboards, prefira `rate(x[$__rate_interval])`, que ajusta a janela ao zoom e ao scrape interval automaticamente.

## 🎓 Na prova PCA

O que costuma cair:
- `rate()` recebe **range vector** e devolve **instant vector**. `rate(x)` sem `[janela]` é **erro de parse**.
- Só faz sentido com **counters**. Com gauge, use `deriv`/`delta`.
- Resultado é **por segundo**, sempre. `increase` = `rate × segundos da janela`.
- **Ordem**: `sum(rate(x[5m]))` ✔ · `rate(sum(x)[5m:])` ✘.
- `irate` usa as **2 últimas** amostras; `rate` usa **todas** as da janela (média).
- O nome da métrica (`__name__`) é **removido** do resultado.

**1.** Qual expressão é válida?
- A) `rate(http_requests_total)`
- B) `rate(http_requests_total[5m])`
- C) `rate(sum(http_requests_total))`
- D) `sum(http_requests_total)[5m]`

<details><summary>Resposta</summary>

**B.** `rate` exige um *range vector* (`[5m]`). A e C passam instant vectors (erro de parse), e D aplica o seletor de range sobre uma expressão sem ser subquery (`[5m:]`), o que também é erro.
</details>

**2.** Um counter tinha valor `1000` e, após um restart do pod, passou a `20`, `40`, `60`... O que `rate()` faz?
- A) Retorna um valor negativo
- B) Descarta a série
- C) Trata a queda como reset e soma o que veio depois do zero
- D) Retorna NaN

<details><summary>Resposta</summary>

**C.** Qualquer queda num counter é interpretada como reset. O `rate` assume que o counter recomeçou em 0 e continua a conta. Por isso **nunca** use `rate` em gauges.
</details>

**3.** Você quer a % de CPU ocupada. Qual é a melhor query?
- A) `100 - node_cpu_seconds_total{mode="idle"}`
- B) `100 * (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m])))`
- C) `rate(avg(node_cpu_seconds_total)[5m])`
- D) `deriv(node_cpu_seconds_total{mode="idle"}[5m])`

<details><summary>Resposta</summary>

**B.** `node_cpu_seconds_total` é counter. O `rate` do modo idle dá a fração ociosa (0..1) por CPU, `avg by (instance)` junta as CPUs e `1 -` inverte.
</details>

**4.** Por que as recording rules usam `rate` em vez de `increase`?
- A) `increase` não funciona em regras
- B) `rate` é sempre por segundo, então resultados de janelas diferentes ficam comparáveis
- C) `rate` não extrapola
- D) `increase` só funciona com gauges

<details><summary>Resposta</summary>

**B.** A própria doc: *"Use `rate` in recording rules so that increases are tracked consistently on a per-second basis."* Os dois extrapolam, e ambos são para counters.
</details>

**5.** Com `scrape_interval: 1m`, qual janela é problemática?
- A) `[5m]`  B) `[4m]`  C) `[1m]`  D) `[10m]`

<details><summary>Resposta</summary>

**C.** Com `[1m]` há no máximo ~1 amostra na janela, e o `rate` precisa de **no mínimo 2**. Resultado: buracos ou vazio. Regra prática: janela ≥ **4× o scrape interval**.
</details>

## 📝 Cola rápida

- `rate(counter[janela])` → média **por segundo** na janela, compensa resets, extrapola as bordas.
- Janela ≥ 4× scrape interval. No Grafana use `$__rate_interval`.
- **rate → depois sum/avg** (nunca o contrário).
- Alertas e recording rules: `rate`. Gráfico nervoso: `irate`. "Quantos no total?": `increase`.
- Gauge? Não. Use `deriv` / `delta`.

## 🔗 Relacionadas

[`irate()`](../irate/) · [`increase()`](../increase/) · [`resets()`](../resets/) · [`deriv()`](../deriv/) · [`histogram_quantile()`](../histogram_quantile/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#rate
