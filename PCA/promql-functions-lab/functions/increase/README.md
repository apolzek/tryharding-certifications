# `increase()`: quanto um counter cresceu na janela

> **Em uma frase:** `increase(v[janela])` diz **quanto** um counter aumentou dentro da janela ("quantos erros 500 nos últimos 5 min?"). Compensa resets e é exatamente `rate(v[janela]) × segundos da janela`.

| | |
|---|---|
| **Assinatura** | `increase(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Counter (`*_total`, `*_count`, `*_sum`, `*_bucket`, native histograms) · ❌ Gauge |
| **Unidade do resultado** | a **mesma do counter** (pedidos, bytes, erros...), total na janela |
| **Dashboard** | http://localhost:3300/d/fn-increase |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: quantos km eu rodei hoje?

O counter é o **hodômetro** do carro (`123.456 km`, só cresce).

- [`rate()`](../rate/) responde "**a que velocidade** eu andei?" (km **por segundo**).
- `increase()` responde "**quantos km** eu rodei na última hora?" (km **no total**).

```
increase(x[1h])  =  (hodômetro agora) − (hodômetro 1h atrás)      ← ideia
                 =  rate(x[1h]) × 3600                            ← como o Prometheus calcula de verdade
```

Se no meio da hora você **trocou de carro** (restart do pod e o counter voltou a 0), o `increase` percebe a queda e **soma os dois trechos** em vez de dar um número negativo.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `increase_http_requests_total{code="200"}` | counter | **+2 req/s** (120 por minuto) — imita o `http_requests_total` |
| `increase_http_requests_total{code="500"}` | counter | **+1 erro a cada 10s** (6 por minuto), incrementos **inteiros** |
| `increase_worker_jobs_processed_total{pod="worker-a"}` | counter | **2 jobs/s**, o pod **reinicia a cada 3 min** (volta a 0) |
| `increase_kube_pod_container_status_restarts_total{namespace="shop",pod="checkout-7d9f"}` | counter | imita o kube-state-metrics: **+1 restart a cada 2 min** |
| `increase_kube_pod_container_status_restarts_total{namespace="shop",pod="catalog-5b2c"}` | counter | nunca reinicia (**0**) |

```bash
curl -s localhost:8088/metrics | grep '^increase_'
# increase_http_requests_total{code="200"} 747844
# increase_http_requests_total{code="500"} 37392
# increase_worker_jobs_processed_total{pod="worker-a"} 124
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-increase
```

Espere **~2 minutos** para janelas de `[1m]` e ~5 min para `[5m]`.

---

## 🔍 Queries passo a passo

### 1. O counter cru

```promql
increase_http_requests_total
```

**Resultado esperado:** duas linhas que **parecem retas horizontais** (~750 mil e ~37 mil): elas sobem 120 e 6 por minuto, mas isso é invisível numa escala de centenas de milhares. O valor absoluto não responde nada útil: é o total "desde sempre". Esse é o motivo de existir `increase`.

---

### 2. Requisições por minuto

```promql
increase(increase_http_requests_total[1m])
```

**O que faz:** para cada série, calcula quanto o counter cresceu no último minuto (com compensação de reset e extrapolação).
**Resultado esperado:**

| code | valor |
|---|---|
| `200` | ≈ **120** |
| `500` | oscila entre ≈ **5.45** e ≈ **6.55** (ver query 4) |

---

### 3. `increase` é açúcar sintático para `rate × segundos`

```promql
increase(increase_http_requests_total{code="200"}[5m])
rate(increase_http_requests_total{code="200"}[5m]) * 300
```

**Resultado esperado:** as duas linhas **se sobrepõem perfeitamente** em ≈ **600** (2/s × 300s). O Prometheus calcula o `increase` exatamente assim.

> 💡 A documentação recomenda: use `increase` para **leitura humana** (dashboards, "quantos erros hoje?"), e `rate` em **recording rules**, para que tudo fique na mesma unidade (por segundo).

---

### 4. Extrapolação: por que `5.45` e não `6`?

```promql
increase(increase_http_requests_total{code="500"}[1m])
increase(increase_http_requests_total{code="500"}[5m])
```

O counter de erros `500` só sobe de **1 em 1**, a cada 10s. Então por que o resultado é fracionado?

1. Com scrape de 5s, a janela de 60s tem ~12 amostras, mas a **primeira** e a **última** estão separadas por só ~**55s**.
2. Nesses 55s o counter subiu **5** ou **6** (depende de onde caem as "viradas" de 10s).
3. O Prometheus **extrapola** para cobrir os 60s inteiros: `5 × 60/55 ≈ 5.45` ou `6 × 60/55 ≈ 6.55`.

**Resultado esperado:**
- `[1m]`: pula entre ≈ **5.45** e ≈ **6.55** (média 6).
- `[5m]`: pula entre ≈ **29.5** e ≈ **30.5** (média 30). Janela maior → erro relativo menor.

Não é bug: é o preço de estimar o que aconteceu **entre** os scrapes. Se precisar de números inteiros num relatório, arredonde no final (`round(...)`), sabendo que é uma estimativa.

---

### 5 e 6. Reset: `increase` × a subtração ingênua

```promql
increase(increase_worker_jobs_processed_total[1m])                              # certo
increase_worker_jobs_processed_total - increase_worker_jobs_processed_total offset 1m   # errado
```

**Resultado esperado:**
- **Cru:** dente-de-serra de 0 a ~**360**, reiniciando a cada 3 min.
- `increase[1m]`: linha estável em ≈ **120** jobs (2/s × 60s), **inclusive** no minuto do reset. Ele vê `358 → 4`, entende "reset" e soma `4` em vez de subtrair.
- `x - x offset 1m`: ≈ **+120** na maior parte do tempo, mas despenca para ≈ **-240** durante o minuto seguinte a cada reset. É por isso que você **não** calcula "quanto cresceu" com subtração.

---

### 7. Restarts de container (kube-state-metrics)

```promql
increase(increase_kube_pod_container_status_restarts_total[10m])
```

**Resultado esperado:** `checkout-7d9f` entre ≈ **4** e ≈ **5** (1 restart a cada 2 min; 4.03 ou 5.04 pela extrapolação), `catalog-5b2c` = **0**. É a forma canônica de alertar restarts no Kubernetes (ver Casos reais).

---

### 8. Taxa de erro nos últimos 5 min

```promql
sum(increase(increase_http_requests_total{code="500"}[5m]))
  / sum(increase(increase_http_requests_total[5m]))
```

**Resultado esperado:** ≈ **4.76%** (30 erros / 630 requisições). Note que é o mesmo que fazer a conta com `rate` (os "× 300" se cancelam).

---

### 9. Requisições nos últimos 5 min, por código

```promql
sum by (code) (increase(increase_http_requests_total[5m]))
```

**Resultado esperado:** `200` ≈ **600**, `500` ≈ **30**. Leitura direta para humanos ("tivemos 30 erros nos últimos 5 min").

---

## 🏭 Casos reais

### 1. Pod reiniciando no Kubernetes (imitado pelo painel 7)

O kube-state-metrics expõe `kube_pod_container_status_restarts_total`, um counter por container. A pergunta do plantão é "**quantas vezes** reiniciou na última hora?", que é um `increase`:

```yaml
groups:
- name: kubernetes-apps
  rules:
  - alert: KubePodCrashLooping
    expr: increase(kube_pod_container_status_restarts_total[15m]) > 3
    for: 5m
    labels: {severity: warning}
    annotations:
      summary: "{{ $labels.namespace }}/{{ $labels.pod }} ({{ $labels.container }}) reiniciou {{ $value | humanize }}x em 15 min"
```

**Decisão:** `increase` (e não `resets`) porque aqui o counter **conta restarts**; queremos saber quanto ele **subiu**. `resets()` contaria restarts do próprio kube-state-metrics.

### 2. "Quantos 5xx tivemos na última hora?" (imitado pelos painéis 2, 4 e 9)

No post-mortem, o gerente quer um número humano, não "0.0083 req/s":

```promql
sum by (service) (increase(http_requests_total{code=~"5.."}[1h]))
```

Num painel Stat do Grafana, use `[$__range]` para "total no período selecionado".

### 3. Recording rule de SLO: `rate`, não `increase`

Para SLOs, a documentação recomenda **gravar `rate`** (tudo "por segundo") e calcular o resto depois:

```yaml
- record: job:http_requests:rate5m
  expr: sum by (job) (rate(http_requests_total[5m]))
- record: job:http_requests_errors:rate5m
  expr: sum by (job) (rate(http_requests_total{code=~"5.."}[5m]))
- record: job:http_requests_error_ratio:rate5m
  expr: job:http_requests_errors:rate5m / job:http_requests:rate5m
```

A taxa de erro (painel 8) dá **o mesmo número** com `rate` ou `increase`, porque o fator "× segundos" se cancela.

### 4. Volume de dados processados por dia

```promql
increase(node_network_transmit_bytes_total{device="eth0"}[1d])   # bytes enviados nas últimas 24h
increase(backup_bytes_written_total[1d])                         # quanto o backup escreveu hoje
```

## ✅ Quando usar

- **Números para humanos:** "quantos erros 5xx na última hora?" → `sum(increase(http_requests_total{code=~"5.."}[1h]))`.
- **Painéis de negócio:** pedidos, cadastros, pagamentos por período (`increase(orders_total[1d])`).
- **Alertas de volume absoluto:** `increase(payment_failures_total[10m]) > 20`.
- **Contagens de histogramas:** `increase(http_request_duration_seconds_count[1h])` = quantas requisições na última hora.
- **Com `$__range` no Grafana** num painel stat: "total no período selecionado".

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer **velocidade** (req/s) | [`rate()`](../rate/) |
| **Recording rules** / SLOs (padronizar em "por segundo") | [`rate()`](../rate/) |
| Métrica é **gauge** (memória, fila, temperatura) | [`delta()`](../delta/) |
| Quer contar **quantos restarts** aconteceram | [`resets()`](../resets/) |
| Precisa de um número **exato** e inteiro (faturamento) | Prometheus não é banco contábil; use a fonte de verdade |

## ⚠️ Pegadinhas

1. **Resultado fracionado** (query 4): extrapolação. Normal.
2. **Janela curta demais:** com menos de 2 amostras, **nenhum resultado**. Use janela ≥ 4× o scrape interval.
3. **`increase(sum(...))` é errado:** primeiro `increase`, depois `sum` (mesma regra do `rate`).
4. **Somar `increase` de janelas coladas** não bate exatamente com o `increase` da janela inteira (cada janela extrapola por conta própria).
5. **Counter que nasce no meio da janela:** o Prometheus não extrapola para antes do "zero" provável da série, mas o primeiro incremento (de "não existe" para o 1º valor) **não** é contado. Um counter que já nasce com `1` pode "sumir" com esse evento.
6. **Usar em gauge:** qualquer queda vira "reset" e o número fica inflado.

## 🎓 Na prova PCA

O que costuma cair:
- `increase` recebe **range vector**, devolve **instant vector**, e é **só para counters**.
- `increase(x[5m])` ≡ `rate(x[5m]) * 300` (açúcar sintático). Use `increase` para leitura humana e `rate` em recording rules.
- **Compensa resets** e **extrapola** → resultado pode ser **fracionado** mesmo com incrementos inteiros.
- Para **gauges**, a função equivalente é `delta`.
- Ordem com agregação: `sum(increase(...))`.

**1.** Um counter só incrementa de 1 em 1. Por que `increase(x[1m])` retornou `5.45`?
- A) Bug de precisão de ponto flutuante
- B) O Prometheus extrapola o aumento para cobrir a janela inteira
- C) O counter foi resetado no meio da janela
- D) O `increase` divide o resultado pelo scrape interval

<details><summary>Resposta</summary>

**B.** A primeira e a última amostras não caem exatamente nas bordas da janela, então o aumento observado é extrapolado para a janela completa. A documentação cita isso explicitamente.
</details>

**2.** Qual expressão é equivalente a `increase(http_requests_total[5m])`?
- A) `rate(http_requests_total[5m]) * 5`
- B) `rate(http_requests_total[5m]) * 300`
- C) `delta(http_requests_total[5m])`
- D) `irate(http_requests_total[5m]) * 300`

<details><summary>Resposta</summary>

**B.** `rate` é por segundo; multiplicado pelos 300 segundos da janela vira o aumento total. C não compensa resets; D usa só as 2 últimas amostras.
</details>

**3.** Qual é a melhor query para "quantos restarts o container teve na última hora" com kube-state-metrics?
- A) `resets(kube_pod_container_status_restarts_total[1h])`
- B) `delta(kube_pod_container_status_restarts_total[1h])`
- C) `increase(kube_pod_container_status_restarts_total[1h])`
- D) `rate(kube_pod_container_status_restarts_total[1h])`

<details><summary>Resposta</summary>

**C.** A métrica é um counter que **conta** restarts; o que queremos é quanto ele cresceu. A contaria resets do próprio counter (restarts do exporter). B não trata resets. D dá "restarts por segundo".
</details>

**4.** Em uma recording rule que alimenta vários dashboards e SLOs, o que a documentação recomenda gravar?
- A) `increase(x[5m])`
- B) `rate(x[5m])`
- C) `irate(x[5m])`
- D) o counter cru

<details><summary>Resposta</summary>

**B.** "Use `rate` em recording rules para que os aumentos sejam rastreados de forma consistente por segundo." `increase` é para legibilidade humana.
</details>

## 📝 Cola rápida

- `increase(counter[janela])` = aumento total na janela = `rate × segundos`.
- Compensa **resets**; **extrapola** (resultado fracionado é normal).
- Só **counters**. Gauge → `delta`.
- Humano/dashboards → `increase`; recording rules/alertas por taxa → `rate`.
- Restarts no k8s: `increase(kube_pod_container_status_restarts_total[15m]) > 3`.

## 🔗 Relacionadas

[`rate()`](../rate/) · [`irate()`](../irate/) · [`resets()`](../resets/) · [`delta()`](../delta/) · [`sum_over_time()`](../sum_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#increase
