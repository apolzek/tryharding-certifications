# `stddev_over_time()`: o quanto um gauge "balança" dentro da janela

> **Em uma frase:** `stddev_over_time(v[janela])` calcula o **desvio padrão populacional** das amostras de cada série na janela: um número, na **mesma unidade** da métrica, que diz "tipicamente, os valores ficam a ± quanto da média".

| | |
|---|---|
| **Assinatura** | `stddev_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (ou o resultado de uma recording rule de `rate`) · ❌ Counter cru · ❌ Histogram (amostras de histograma são ignoradas) |
| **Unidade do resultado** | a **mesma** da métrica (req/s → req/s) |
| **Dashboard** | http://localhost:3300/d/fn-stddev_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: dois ônibus com a mesma média

Duas linhas de ônibus passam **em média** a cada 10 minutos.

- A linha **A** passa sempre entre 9 e 11 minutos.
- A linha **B** passa entre 2 e 18 minutos.

A **média** é igual; a **confiabilidade** é totalmente diferente. O desvio padrão é o número que captura essa diferença: A ≈ 0,6 min, B ≈ 4,6 min.

```
             √  Σ (xᵢ − média)²
  σ   =         ───────────────        (populacional: divide por N, não por N−1)
                       N
```

E com ele nasce o detector de anomalia mais clássico do mundo, o **z-score**:

```
  z = (valor_agora − média) / σ        "a quantos desvios padrão da média estou?"
```

|z| < 2 é normal; |z| > 3 é raro (em dados "bem-comportados", < 0,3% das vezes).

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita o resultado de uma **recording rule de tráfego** (algo como `job:http_requests:rate1m = sum by (job) (rate(http_requests_total[1m]))`), exposto como gauge em req/s:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `stddev_over_time_http_requests_per_second{service="checkout"}` | gauge | **100 req/s ± 5** (ruído uniforme → σ ≈ 5/√3 ≈ **2,9**). A cada **4 min**, por **20s** (~4 amostras), **ataque de bot**: ~**160 req/s** |
| `stddev_over_time_http_requests_per_second{service="search"}` | gauge | **100 req/s ± 40** (ruído uniforme → σ ≈ 40/√3 ≈ **23**). Barulhento por natureza |
| `stddev_over_time_http_requests_per_second{service="legacy"}` | gauge | **5 req/s constante** (sistema legado só com health-check). σ = **0**: serve para mostrar o z-score quebrando |

```bash
curl -s localhost:8088/metrics | grep '^stddev_over_time_'
# stddev_over_time_http_requests_per_second{service="checkout"} 101.7
# stddev_over_time_http_requests_per_second{service="search"} 73.2
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-stddev_over_time
```

Espere **~5 min** para a janela `[5m]` encher, e até 4 min para ver uma anomalia.

---

## 🔍 Queries passo a passo

### 1. O gauge cru

```promql
stddev_over_time_http_requests_per_second
```

**Resultado esperado:** `checkout` é uma faixa **estreita** em torno de 100 req/s, com um **degrau** de ~160 por 20s a cada 4 min. `search` é uma **nuvem larga** entre 60 e 140. `legacy` é uma linha reta em 5.

---

### 2. As médias são iguais

```promql
avg_over_time(stddev_over_time_http_requests_per_second{service!="legacy"}[5m])
```

**Resultado esperado:** as duas linhas em ≈ **100 req/s** (checkout sobe para ~104 enquanto a anomalia está na janela). Pela média, os serviços parecem idênticos.

---

### 3. O desvio padrão mostra a diferença

```promql
stddev_over_time(stddev_over_time_http_requests_per_second[5m])
```

**Resultado esperado:**

| service | valor |
|---|---|
| `search` | ≈ **21 a 23 req/s**, estável |
| `checkout` | ≈ **2,5 a 3** fora da anomalia; sobe para ≈ **15 a 20** durante os 5 min em que a anomalia está dentro da janela |
| `legacy` | **0** (nunca varia) |

**Moral:** o `checkout` é **previsível** (e por isso uma anomalia nele é notável); o `search` é **sempre** imprevisível.

---

### 4. Z-score: o detector de anomalia

```promql
(stddev_over_time_http_requests_per_second{service!="legacy"} - avg_over_time(stddev_over_time_http_requests_per_second{service!="legacy"}[5m]))
  / stddev_over_time(stddev_over_time_http_requests_per_second{service!="legacy"}[5m])
```

(O `legacy` fica de fora aqui; ele tem painel próprio, o 7.)

**O que faz:** para cada série, mede a quantos σ o valor **atual** está da média dos últimos 5 min. O resultado é **adimensional** (funciona igual para req/s, bytes, °C).
**Resultado esperado:**
- `search`: oscila entre **−1,7 e +1,7** (com ruído uniforme ±40 e σ≈23, o máximo é 40/23). **Nunca** alarma, apesar de barulhento: ele é "normalmente barulhento".
- `checkout`: fica entre −1,7 e +1,7 e, quando a anomalia começa, **salta para ≈ +6 a +7**. Nos pontos seguintes cai para ≈ **+3,7**. Por quê? Porque a própria anomalia entra na janela e **puxa a média para cima e infla o σ**. Com 4 anomalias em 60 amostras: média ≈ 104, σ ≈ 15, z = (160 − 104)/15 ≈ 3,7.
- Logo depois que a anomalia acaba, o `checkout` fica em ≈ **−0,3**: valores normais parecem "um pouco abaixo" da média inflada.

---

### 5. Faixa "normal" (bandas de Bollinger)

```promql
stddev_over_time_http_requests_per_second{service="checkout"}
avg_over_time(stddev_over_time_http_requests_per_second{service="checkout"}[5m]) + 2 * stddev_over_time(stddev_over_time_http_requests_per_second{service="checkout"}[5m])
avg_over_time(stddev_over_time_http_requests_per_second{service="checkout"}[5m]) - 2 * stddev_over_time(stddev_over_time_http_requests_per_second{service="checkout"}[5m])
```

**Resultado esperado:** faixa apertada **~94 a ~106** em volta do valor. Quando a anomalia chega, o valor **fura** a faixa e a faixa em seguida **se alarga** para ~75 a ~135 por 5 min (efeito da anomalia dentro da janela).

---

### 6. Regra de alerta: |z| > 3

```promql
abs(
  (stddev_over_time_http_requests_per_second - avg_over_time(stddev_over_time_http_requests_per_second[5m]))
    / stddev_over_time(stddev_over_time_http_requests_per_second[5m])
) > 3
```

**Resultado esperado:** **vazio** quase o tempo todo; aparece `service="checkout"` como pontos durante os ~20s de anomalia. Nunca aparece `search`.

---

### 7. 🟡 O que dá errado: série constante → σ = 0 → z = NaN

```promql
stddev_over_time(stddev_over_time_http_requests_per_second{service="legacy"}[5m])          # 0
(stddev_over_time_http_requests_per_second{service="legacy"} - avg_over_time(stddev_over_time_http_requests_per_second{service="legacy"}[5m]))
  / stddev_over_time(stddev_over_time_http_requests_per_second{service="legacy"}[5m])       # NaN
(stddev_over_time_http_requests_per_second - avg_over_time(stddev_over_time_http_requests_per_second[5m]))
  / (stddev_over_time(stddev_over_time_http_requests_per_second[5m]) > 0)                  # legacy some
```

**Resultado esperado** (tabela):

| service | σ | z sem proteção | z com `(σ > 0)` |
|---|---|---|---|
| legacy | **0** | **NaN** (5 − 5) / 0 | *(ausente)* |
| checkout | — | — | ≈ −1,7..+1,7 (ou alto na anomalia) |
| search | — | — | ≈ −1,7..+1,7 |

**Moral:** `NaN` não é maior nem menor que nada, então `abs(z) > 3` com NaN simplesmente **não** dispara, mas polui dashboards e recording rules. E se o numerador não for zero (ex.: a média vem de uma janela com `offset` ou de outro tamanho, e o σ dessa janela é 0), o resultado é `±Inf` e o alerta **dispara**. Proteja sempre o divisor com `> 0`.

---

## 🏭 Casos reais

### 1. Detecção de anomalia de tráfego (padrão "z-score com recording rules")

É o padrão popularizado pelo time de infraestrutura da GitLab: em vez de um limiar fixo ("alerte se passar de 500 req/s", que é errado às 3h e às 15h), compare o tráfego atual com o **comportamento recente dele mesmo**:

```yaml
groups:
  - name: traffic-anomaly
    rules:
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
      - record: job:http_requests:rate5m:avg_over_time_1w
        expr: avg_over_time(job:http_requests:rate5m[1w])
      - record: job:http_requests:rate5m:stddev_over_time_1w
        expr: stddev_over_time(job:http_requests:rate5m[1w])

      - alert: TrafficAnomaly
        expr: |
          abs(
            (job:http_requests:rate5m - job:http_requests:rate5m:avg_over_time_1w)
              / job:http_requests:rate5m:stddev_over_time_1w
          ) > 3
        for: 10m
        labels: {severity: warning}
        annotations:
          summary: "Tráfego de {{ $labels.job }} a mais de 3σ da média da semana"
```

**Decisões:** `stddev_over_time` precisa de **gauge**, por isso primeiro se grava o `rate` numa recording rule (ou se usa subquery). Janela longa (`1w`) para que uma anomalia de minutos não contamine a linha de base (ver query 4). `for: 10m` para não disparar por uma rajada.

### 2. Jitter de latência de rede / probe

`probe_duration_seconds` do blackbox_exporter (módulo ICMP) mede a latência de um link a cada scrape. O desvio padrão é o **jitter**: média boa com desvio alto = VoIP/vídeo ruim.

```yaml
- alert: LinkHighJitter
  expr: stddev_over_time(probe_duration_seconds{job="blackbox-icmp"}[10m]) > 0.03
  for: 10m
  labels: {severity: warning}
  annotations:
    summary: "Jitter de {{ $labels.instance }} acima de 30ms (média pode estar ok)"
```

**Decisão:** limiar na **mesma unidade** da métrica (segundos), diferente de `stdvar_over_time` (que seria 0.0009 s²).

### 3. Flapping de temperatura / sensores

`stddev_over_time(node_hwmon_temp_celsius[15m]) > 5` detecta um sensor ou ventoinha "oscilando" (liga/desliga), algo que nem `max_over_time` nem `avg_over_time` revelam.

---

## ✅ Quando usar

- **Detecção de anomalia sem limiar fixo**: z-score em tráfego, latência, temperatura, fila.
- **Medir estabilidade/jitter**: variação de latência de rede, "o quanto o tempo de resposta oscila".
- **Comparar consistência** de servidores/regiões com médias iguais.
- **Bandas dinâmicas** em dashboards (média ± 2σ).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Dados com **outliers** que você não quer que dominem a medida | [`mad_over_time()`](../mad_over_time/) (robusto) |
| Quer "95% das vezes fica abaixo de X" | [`quantile_over_time()`](../quantile_over_time/) |
| Precisa **somar** dispersões de fontes independentes | [`stdvar_over_time()`](../stdvar_over_time/) (variâncias somam, desvios não) |
| Desvio padrão **entre séries** num instante (entre pods agora) | operador de agregação `stddev(x)` |
| Métrica é **counter** | `stddev_over_time(rate(x[1m])[30m:])` (subquery) ou recording rule |
| Desvio padrão de um **histogram** (distribuição de requisições) | [`histogram_stddev()`](../histogram_stddev/) |

## ⚠️ Pegadinhas

1. **A anomalia contamina a própria linha de base** (query 4): o z-score cai conforme ela fica na janela. Alternativa: base com `offset` (`avg_over_time(x[1h] offset 5m)`) ou janela **bem maior** que o evento.
2. **σ = 0 ⇒ divisão por zero**: se a série ficou constante na janela (ex.: um gauge parado), o z-score vira `+Inf`, `-Inf` ou `NaN`. Proteja com `... / (stddev_over_time(x[5m]) > 0)`.
3. **Populacional, não amostral**: divide por N, não por N−1. Com poucas amostras, o valor é um pouco menor que o de planilhas (`DESVPAD.A`).
4. **Pressupõe distribuição "em sino"**: com dados bimodais ou cauda longa, "3σ" não significa 0,3%.
5. **Poucas amostras** (janela curta) → σ instável, z-scores exagerados.
6. **Histogramas nativos são ignorados** (só amostras float entram).

---

## 🎓 Na prova PCA

O que costuma cair:
- Recebe **range vector**, devolve **instant vector** (uma saída por série; **não** junta séries).
- Diferença para o **operador** `stddev(...)` (entre séries, num instante).
- É **populacional**; `stdvar_over_time` = `stddev_over_time`².
- Montar z-score: `(x - avg_over_time(x[w])) / stddev_over_time(x[w])`.

**1.** Qual query calcula o desvio padrão do uso de memória **de cada pod** ao longo da última hora?

- A) `stddev(container_memory_working_set_bytes)`
- B) `stddev_over_time(container_memory_working_set_bytes[1h])`
- C) `stddev(container_memory_working_set_bytes[1h])`
- D) `stddev_over_time(sum(container_memory_working_set_bytes))`

<details><summary>Resposta</summary>

**B.** `_over_time` com range vector, uma saída por série. A) é entre pods num instante. C) operador de agregação não aceita range vector (erro). D) falta o range (e ainda somaria os pods).
</details>

**2.** Você quer o desvio padrão da **taxa** de requisições (`http_requests_total` é counter) nos últimos 30 min. Qual é válida?

- A) `stddev_over_time(http_requests_total[30m])`
- B) `stddev_over_time(rate(http_requests_total[1m])[30m:])`
- C) `rate(stddev_over_time(http_requests_total[30m]))`
- D) `stddev_over_time(rate(http_requests_total[30m]))`

<details><summary>Resposta</summary>

**B.** Primeiro transforma o counter em taxa (`rate`), depois usa **subquery** `[30m:]` para gerar um range vector da taxa. A) mede a dispersão do total acumulado (sem sentido). D) `rate(...)` devolve instant vector, e `stddev_over_time` exige range vector (erro de tipo).
</details>

**3.** Se `stddev_over_time(x[5m])` = 4, quanto vale `stdvar_over_time(x[5m])`?

- A) 2
- B) 4
- C) 8
- D) 16

<details><summary>Resposta</summary>

**D.** Variância = desvio padrão ao quadrado: 4² = 16.
</details>

**4.** Uma série ficou **constante** (valor 7) nos últimos 5 minutos. O que retorna `(x - avg_over_time(x[5m])) / stddev_over_time(x[5m])`?

- A) 0
- B) 1
- C) NaN
- D) Vetor vazio

<details><summary>Resposta</summary>

**C.** σ = 0 e o numerador também é 0 → 0/0 = `NaN`. (Se o valor atual fosse diferente da média, daria `±Inf`.) Por isso se protege o divisor com `> 0`.
</details>

---

## 📝 Cola rápida

- `stddev_over_time(gauge[janela])` → desvio padrão **populacional** por série, na unidade da métrica.
- **z-score** = `(x − avg_over_time(x[w])) / stddev_over_time(x[w])`; |z| > 3 = anômalo.
- Counter? `rate` primeiro (subquery ou recording rule), depois `stddev_over_time`.
- `stddev(x)` = entre séries; `stddev_over_time(x[w])` = no tempo. `stdvar = stddev²`.
- Outliers? Prefira `mad_over_time`. Proteja divisão por σ = 0.

## 🔗 Relacionadas

[`stdvar_over_time()`](../stdvar_over_time/) · [`mad_over_time()`](../mad_over_time/) · [`avg_over_time()`](../avg_over_time/) · [`quantile_over_time()`](../quantile_over_time/) · [`histogram_stddev()`](../histogram_stddev/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
