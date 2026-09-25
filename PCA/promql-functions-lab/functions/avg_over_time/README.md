# `avg_over_time()`: a média de cada série ao longo do tempo

> **Em uma frase:** `avg_over_time(v[janela])` pega **cada série separadamente**, olha todas as amostras dela dentro da janela e devolve a **média aritmética** dessas amostras. Uma série entra, uma série sai.

| | |
|---|---|
| **Assinatura** | `avg_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (CPU, memória, fila, temperatura, 0/1 de health check) · ✅ native histograms (média "de histograma") · ❌ Counter cru |
| **Unidade do resultado** | a **mesma** do dado de entrada (%, bytes, mensagens...) |
| **Dashboard** | http://localhost:3300/d/fn-avg_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a planilha (linhas × colunas)

Imagine os dados como uma **planilha**. Cada **linha** é uma série (um pod). Cada **coluna** é um scrape (um instante):

```
                 12:00:00  12:00:05  12:00:10  12:00:15   ...  12:00:55  │ avg_over_time([1m])
                ─────────────────────────────────────────────────────────┼────────────────────
 pod-a             28        11        19        25       ...     17     │  ≈ 20   ← média da LINHA
 pod-b             41        63        50        44       ...     58     │  ≈ 50
 pod-c             88        71        79        93       ...     74     │  ≈ 80
                ─────────────────────────────────────────────────────────┘
 avg(x)            52        48        49        54       ...     50
                   ↑ média da COLUNA (todos os pods, um instante)
```

- **`avg_over_time(x[1m])`** anda na **horizontal** (→): para **cada pod**, média das 12 amostras do último minuto. Resultado: **3 séries** (uma por pod), bem mais lisas.
- **`avg(x)`** (operador de agregação) anda na **vertical** (↓): num **único instante**, média entre os pods. Resultado: **1 série**, ainda com todo o ruído, porque não olha o passado.

Outra imagem: `avg_over_time` é a **média das suas notas no semestre** (um aluno, várias provas). `avg` é a **média da turma numa prova** (vários alunos, uma prova). Todas as funções `*_over_time` são "média do semestre"; todos os operadores `sum/avg/min/max/count` são "média da turma".

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `avg_over_time_cpu_usage_percent{pod="pod-a"}` | gauge | **20%** ± 15 de ruído a cada scrape |
| `avg_over_time_cpu_usage_percent{pod="pod-b"}` | gauge | **50%** ± 15 |
| `avg_over_time_cpu_usage_percent{pod="pod-c"}` | gauge | **80%** ± 15 |
| `avg_over_time_rabbitmq_queue_messages_ready` | gauge | senóide de **período 4 min**, entre 20 e 180 (centro **100**) |
| `avg_over_time_probe_success{service="checkout"}` | gauge 0/1 | sempre **1** |
| `avg_over_time_probe_success{service="search"}` | gauge 0/1 | **0 durante 30s a cada 2 min** (segundos 90..120 do ciclo) |

```bash
curl -s localhost:8088/metrics | grep '^avg_over_time_'
# avg_over_time_cpu_usage_percent{pod="pod-a"} 27.3
# avg_over_time_rabbitmq_queue_messages_ready 143
# avg_over_time_probe_success{service="search"} 1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-avg_over_time
```

Espere **~2 min** para as janelas de `[1m]`, **~5 min** para `[4m]` e **~10 min** para a disponibilidade em `[10m]` estabilizar.

---

## 🔍 Queries passo a passo

### 1. O dado cru

```promql
avg_over_time_cpu_usage_percent
```

**Resultado esperado:** 3 linhas "tremidas", cada uma pulando ±15 em volta de 20, 50 e 80. Os valores de cada scrape são ruído puro: é difícil dizer se um pod "está em 65%" ou "estava em 65% só naquele scrape".

---

### 2. `avg_over_time`: a média horizontal

```promql
avg_over_time(avg_over_time_cpu_usage_percent[1m])
```

**O que faz:** para **cada pod**, soma as amostras de `(agora − 1m, agora]` e divide pela quantidade. Com scrape de 5s são **12 amostras**.
**Resultado esperado:**

| pod | valor |
|---|---|
| `pod-a` | ≈ **20** (oscila uns ±3) |
| `pod-b` | ≈ **50** |
| `pod-c` | ≈ **80** |

Continuam sendo **3 séries**, com os mesmos labels. O `__name__` é removido (o resultado não é mais "a métrica crua").

---

### 3. `avg()`: a média vertical

```promql
avg(avg_over_time_cpu_usage_percent)
```

**O que faz:** em cada instante, junta **os 3 pods** e tira a média. Os labels somem.
**Resultado esperado:** **1 série** em ≈ **50** (= (20+50+80)/3), ainda ruidosa (±9), porque cada ponto só enxerga aquele instante.

---

### 4. Combinando os dois eixos

```promql
avg(avg_over_time(avg_over_time_cpu_usage_percent[1m]))
```

**O que faz:** primeiro suaviza cada pod no tempo (→), depois tira a média entre pods (↓).
**Resultado esperado:** uma linha **lisa** em ≈ **50**, comparada no mesmo painel com a linha tremida do `avg()` cru.

> 💡 A ordem importa pouco para `avg` de `avg` com séries completas, mas importa MUITO para `max(avg_over_time(...))` vs `avg(max_over_time(...))`: pense sempre "o que eu quero por série no tempo" e "o que eu quero entre séries".

---

### 5. O tamanho da janela contra o período do sinal

```promql
avg_over_time_rabbitmq_queue_messages_ready                          # cru: onda 20..180
avg_over_time(avg_over_time_rabbitmq_queue_messages_ready[1m])       # segue a onda, com atraso
avg_over_time(avg_over_time_rabbitmq_queue_messages_ready[4m])       # janela = 1 período → reta
```

**Resultado esperado:**
- **Cru:** onda completa entre ~20 e ~180 a cada 4 min.
- **`[1m]`:** a mesma onda, um pouco **menor** (picos em ~170 em vez de 180) e **atrasada** ~30s: é uma média móvel, então o valor "de agora" é o centro da janela, 30s atrás.
- **`[4m]`:** uma linha praticamente **reta em ≈ 100**. Quando a janela cobre um período inteiro, sobe e desce se cancelam.

**Moral:** média móvel com janela grande "apaga" padrões. Ótimo para tirar ruído, péssimo se o padrão é o que você quer ver.

---

### 6, 7 e 8. Disponibilidade (SLI) com um 0/1

```promql
avg_over_time(avg_over_time_probe_success[10m])
```

**O que faz:** a média de um sinal 0/1 é a **fração do tempo em que ele foi 1**.
**Resultado esperado** (depois de 10 min de dados):

| service | valor |
|---|---|
| `checkout` | **1** (100%) |
| `search` | ≈ **0.75** (75%): 6 scrapes em 0 a cada 24 = 30s fora a cada 2 min |

É exatamente o clássico `avg_over_time(up{job="api"}[1d])` para calcular disponibilidade de um target.

> Se outro agente/pessoa rodar `tools/deploy.sh`, o gerador reinicia e perde 1 ou 2 scrapes. Isso não vira 0 (a amostra simplesmente não existe), então a média continua certa.

### 9. ❌ O que dá errado: alertar limite com a média

```promql
max_over_time(avg_over_time_rabbitmq_queue_messages_ready[4m])   # ≈ 180
avg_over_time(avg_over_time_rabbitmq_queue_messages_ready[4m])   # ≈ 100
```

**Resultado esperado:** com um alerta "fila > 150", a versão com `avg_over_time[4m]` fica em **~100** e **nunca dispara**, embora a fila passe de 150 durante ~1 min a cada 4 min. O `max_over_time` fica em **~180** e dispara. Média é para **tendência/ruído**; limite é com **max/min**.

---

## 🏭 Casos reais

### 1. SLI de disponibilidade com o blackbox exporter

O time de SRE do e-commerce mede a disponibilidade do checkout com uma sonda HTTP a cada 15s (`probe_success`, 0/1). O SLO é 99,9% em 30 dias:

```yaml
groups:
  - name: slo-checkout
    rules:
      - record: probe:availability:avg5m
        expr: avg_over_time(probe_success{job="blackbox", instance="https://loja/checkout"}[5m])
      - alert: CheckoutIndisponivel
        expr: avg_over_time(probe_success{job="blackbox", instance="https://loja/checkout"}[5m]) < 0.9
        for: 2m
        labels: {severity: critical}
        annotations:
          summary: "Checkout falhou em mais de 10% das sondas nos últimos 5 min"
```

E o número do mês: `avg_over_time(probe:availability:avg5m[30d])`. É o que o cenário imita com `avg_over_time_probe_success`.

### 2. Alerta de load average sem "pisca-pisca"

`node_load1` pula muito. Um alerta direto (`node_load1 > 8`) dispara e resolve a cada minuto. Suavizando:

```yaml
- alert: NodeLoadAlto
  expr: |
    avg_over_time(node_load1[10m])
      / count without (cpu, mode) (node_cpu_seconds_total{mode="idle"}) > 2
  for: 15m
```

Load médio de 10 min maior que 2× o número de CPUs, sustentado por 15 min.

### 3. Tamanho médio de fila para dimensionar consumidores

`avg_over_time(rabbitmq_queue_messages_ready{queue="orders"}[1h])` ≈ backlog típico. Se a média de 1h cresce dia após dia, faltam consumidores. (O cenário imita isso com `avg_over_time_rabbitmq_queue_messages_ready`.)

### 4. `avg_over_time(up[1d])` no relatório de disponibilidade de targets

```promql
sort(avg_over_time(up{job="node"}[1d]))
```

Lista os nodes do menos disponível para o mais disponível no último dia.

---

## ✅ Quando usar

- **Suavizar um gauge ruidoso** num painel: `avg_over_time(node_load1[5m])`, `avg_over_time(queue_size[2m])`.
- **Disponibilidade/SLI** a partir de um 0/1: `avg_over_time(up{job="api"}[30d])`, `avg_over_time(probe_success[1h])`.
- **Alertas que não disparam por um único scrape ruim:** `avg_over_time(temperature_celsius[5m]) > 80`.
- **Recording rules de "média horária"** para dashboards longos.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer a média **entre** séries (pods, instâncias) num instante | operador `avg()` / `avg by (...)` |
| Métrica é **counter** (`_total`) | [`rate()`](../rate/) (a média de um counter cru é um número sem sentido) |
| Quer saber o **pior** momento (pico de memória, latência) | [`max_over_time()`](../max_over_time/), [`quantile_over_time()`](../quantile_over_time/) |
| Quer o **pior** vale (conexões livres, disco livre) | [`min_over_time()`](../min_over_time/) |
| Latência de **histograma** | `histogram_quantile(0.99, rate(x_bucket[5m]))` (média esconde a cauda) |

## ⚠️ Pegadinhas

1. **Média esconde picos.** Um pico de 15s numa janela de 5 min pesa só 5% (3 de 60 amostras) na média: 400 MiB com pico de 950 MiB vira uma média de ~430 MiB. Para limites (OOM, disco cheio), use `max_over_time`/`min_over_time`.
2. **Todas as amostras têm o mesmo peso**, mesmo se não estiverem igualmente espaçadas (diz a documentação). Se o scrape falhou por 1 min e depois voltou, os pontos antes e depois pesam igual; não há ponderação por tempo.
3. **Janela nunca é "vazia = 0".** Se não há amostra na janela, a série **some** do resultado (não vira 0). Em SLI, um target que nunca respondeu não aparece com 0%.
4. **`avg_over_time(sum(x)[5m:])` ≠ `sum(avg_over_time(x[5m]))`** quando séries entram e saem no meio da janela. O primeiro exige subquery (`[5m:]`) e custa mais.
5. **Histogramas:** com native histograms, `avg_over_time` faz a "média de histogramas". Se a janela mistura amostras float e histogram, a série é **removida** do resultado (com aviso).
6. **Range selector é aberto à esquerda** no Prometheus 3: `[1m]` com scrape de 5s tem **12** amostras, não 13.

## 🎓 Na prova PCA

O que costuma cair:
- `avg_over_time` recebe **range vector** (`x[5m]`) e devolve **instant vector**; `avg` recebe **instant vector**. `avg_over_time(x)` sem `[ ]` dá **erro**.
- A diferença entre agregar **no tempo** (`*_over_time`, por série) e **entre séries** (operadores `avg/sum/max...` com `by/without`).
- Usar em **gauges**; para counters, `rate()`.
- Disponibilidade com `avg_over_time(up[...])`.
- Subquery para aplicar `*_over_time` sobre o resultado de outra função: `avg_over_time(rate(x[5m])[1h:])`.

**1.** Você tem 3 pods com a métrica `cpu_usage`. O que `avg_over_time(cpu_usage[5m])` retorna?

- A) Uma série com a média dos 3 pods nos últimos 5 min
- B) Três séries, cada uma com a média do seu pod nos últimos 5 min
- C) Três séries com o último valor de cada pod
- D) Erro: falta `by (pod)`

<details><summary>Resposta</summary>

**B.** Funções `*_over_time` trabalham **por série**, ao longo do tempo. Para juntar os pods seria `avg(avg_over_time(cpu_usage[5m]))`. `by` só existe em operadores de agregação.
</details>

**2.** Qual expressão é **inválida**?

- A) `avg_over_time(up[1h])`
- B) `avg(up)`
- C) `avg_over_time(up)`
- D) `avg_over_time(rate(http_requests_total[5m])[1h:])`

<details><summary>Resposta</summary>

**C.** `avg_over_time` exige range vector. D é válida graças à subquery `[1h:]`.
</details>

**3.** `probe_success` (0/1) de um site teve 1080 amostras 1 e 120 amostras 0 na última hora. Quanto vale `avg_over_time(probe_success[1h])`?

- A) 0.1
- B) 0.9
- C) 1080
- D) 1

<details><summary>Resposta</summary>

**B.** 1080 / 1200 = 0.9: a média de um 0/1 é a fração do tempo em 1 (90% de disponibilidade).
</details>

**4.** Por que `avg_over_time(http_requests_total[5m])` não é útil?

- A) Porque counters não podem ser lidos com range vector
- B) Porque a média de um counter acumulado não representa taxa nem quantidade; use `rate()`
- C) Porque retorna sempre 0
- D) Porque só funciona com histogramas

<details><summary>Resposta</summary>

**B.** O counter só cresce (e reseta): a média dos valores acumulados não tem significado operacional.
</details>

## 📝 Cola rápida

- `avg_over_time(x[j])` = média **das amostras de cada série** na janela (→ tempo). `avg(x)` = média **entre séries** agora (↓).
- Entrada **range vector**, saída **instant vector**, sem `__name__`.
- **Gauges** e 0/1 (disponibilidade = `avg_over_time(up[...])`). Counter → `rate`.
- Todas as amostras pesam igual; série sem amostra na janela **some** (não vira 0).
- Média esconde picos: para limites use `max_over_time`/`min_over_time`.

## 🔗 Relacionadas

[`sum_over_time()`](../sum_over_time/) · [`count_over_time()`](../count_over_time/) · [`min_over_time()`](../min_over_time/) · [`max_over_time()`](../max_over_time/) · [`quantile_over_time()`](../quantile_over_time/) · [`stddev_over_time()`](../stddev_over_time/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
