# `sqrt()`: raiz quadrada (desfazendo o "ao quadrado")

> **Em uma frase:** `sqrt(v)` calcula a **raiz quadrada** de cada valor. No monitoramento, ela quase sempre aparece para **desfazer um quadrado**: variância → desvio padrão, média dos quadrados → RMS, `x² + y² + z²` → magnitude.

| | |
|---|---|
| **Assinatura** | `sqrt(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (variâncias, somas de quadrados) · histogramas são ignorados |
| **Unidade do resultado** | a raiz da unidade de entrada: `%² → %`, `V² → V`, `(mm/s)² → mm/s` |
| **Dashboard** | http://localhost:3300/d/fn-sqrt |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o lado do quadrado

Se um terreno **quadrado** tem **100 m²**, quanto mede cada lado? **10 m**. A área está em m² (unidade "quadrada"); a raiz devolve o **lado**, em metros.

```
    ┌──────────┐
    │          │   área = 100 m²
    │  100 m²  │   lado = sqrt(100) = 10 m
    │          │
    └──────────┘
       10 m
```

A estatística adora elevar coisas ao quadrado (para que positivos e negativos não se cancelem), e o resultado fica numa unidade estranha: "**75 %²**" não significa nada para ninguém. O `sqrt()` traz de volta para "**8.66 %**", que dá pra comparar com o próprio uso de CPU.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `sqrt_cpu_usage_percent{host="calmo"}` | gauge | **40 ± 2 %** (ruído uniforme → desvio padrão teórico `2/√3 ≈ 1.15`) |
| `sqrt_cpu_usage_percent{host="agitado"}` | gauge | **50 ± 15 %** (desvio padrão teórico `15/√3 ≈ 8.66`) |
| `sqrt_ac_voltage_volts{phase="L1"}` | gauge | senóide de pico **±311 V** em "câmera lenta" (período 60 s) |
| `sqrt_vibration_mm_s{motor="bomba-1",axis="x"}` | gauge | **3** mm/s |
| `sqrt_vibration_mm_s{motor="bomba-1",axis="y"}` | gauge | **4** mm/s |
| `sqrt_vibration_mm_s{motor="bomba-1",axis="z"}` | gauge | **12** mm/s por 2 min, **0** por 2 min |

```bash
curl -s localhost:8088/metrics | grep '^sqrt_'
# sqrt_ac_voltage_volts{phase="L1"} -269.3
# sqrt_cpu_usage_percent{host="agitado"} 61.4
# sqrt_vibration_mm_s{axis="z",motor="bomba-1"} 12
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-sqrt
```

Espere **~2 minutos** (as janelas são de `[2m]`).

---

## 🔍 Queries passo a passo

### 1. A variância: número "ao quadrado"

```promql
stdvar_over_time(sqrt_cpu_usage_percent[2m])
```

**O que faz:** [`stdvar_over_time()`](../stdvar_over_time/) calcula a **variância** (média dos desvios **ao quadrado**) de cada série na janela.
**Resultado esperado:** calmo ≈ **1.3 %²** e agitado ≈ **75 %²**. O agitado parece "57 vezes pior", o que exagera a diferença real.

---

### 2. `sqrt(variância)` = desvio padrão

```promql
sqrt(stdvar_over_time(sqrt_cpu_usage_percent[2m]))
stddev_over_time(sqrt_cpu_usage_percent[2m])
```

**Resultado esperado:**

| host | variância | `sqrt()` = desvio padrão |
|---|---|---|
| calmo | ≈ 1.33 %² | ≈ **1.15 %** |
| agitado | ≈ 75 %² | ≈ **8.66 %** |

(Os valores **flutuam** em volta do teórico, por exemplo de ~6 a ~9.5 % no agitado, porque 2 min são só 24 amostras. Estimar dispersão com poucas amostras é ruidoso.)

As duas queries dão **exatamente** o mesmo número (as linhas se sobrepõem): por definição, `stddev = sqrt(stdvar)`. Agora dá pra ler: "a CPU do agitado varia uns **±9 pontos** em torno da média".

> 💡 Isso é útil quando a aplicação **exporta a variância** pronta (alguns SDKs de métricas fazem isso), ou quando você soma variâncias de fontes independentes: `sqrt(var_a + var_b)`. Desvios padrão **não se somam**; variâncias sim.

---

### 3 e 4. RMS: por que a tomada é "220 V" se o pico é 311 V?

```promql
avg_over_time(sqrt_ac_voltage_volts[2m])                      # média
sqrt(avg_over_time((sqrt_ac_voltage_volts ^ 2)[2m:5s]))       # RMS
```

**O que faz:** o RMS (*root mean square*) é literalmente o nome da conta: **raiz** (`sqrt`) da **média** (`avg_over_time`) dos **quadrados** (`^ 2`). A subquery `[2m:5s]` é necessária porque `avg_over_time` precisa de um range vector e `x ^ 2` é uma expressão.
**Resultado esperado:**
- **Média:** ≈ **0 V**. A onda passa metade do tempo positiva e metade negativa: média inútil.
- **RMS:** ≈ **220 V** (`311 / √2 = 219.9`). É o valor "efetivo" que aquece um chuveiro, e é o que a companhia elétrica chama de "220 V".

O mesmo padrão serve para **erro quadrático médio (RMSE)** de previsões: `sqrt(avg_over_time(((previsto - real) ^ 2)[1h:]))`.

---

### 5 e 6. Magnitude de um vetor (Pitágoras)

```promql
sqrt(sum by (motor) (sqrt_vibration_mm_s ^ 2))   # magnitude
sum by (motor) (sqrt_vibration_mm_s)              # soma simples (errado)
```

**O que faz:** eleva cada eixo ao quadrado, soma os eixos do mesmo motor e tira a raiz: `sqrt(x² + y² + z²)`.
**Resultado esperado:**

| z | magnitude `sqrt(9 + 16 + z²)` | soma simples |
|---|---|---|
| 0 | **5** (triângulo 3-4-5) | 7 ❌ |
| 12 | **13** (3-4-12-13) | 19 ❌ |

A soma simples **superestima** a vibração, porque os eixos são perpendiculares.

---

### 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `sqrt(vector(16))` | **4** | |
| `sqrt(vector(2))` | **1.414...** | |
| `sqrt(vector(0.25))` | **0.5** | ⚠️ para valores entre 0 e 1, a raiz é **maior** que o número |
| `sqrt(vector(0))` | **0** | |
| `sqrt(vector(-1))` | **NaN** | não existe raiz real de negativo |
| `sqrt(vector(+Inf))` | **+Inf** | |

---

## 🏭 Casos reais

### 1. Detecção de anomalia com z-score (e o "desvio padrão da semana")

Um padrão bem conhecido (popularizado pelo time de SRE do GitLab) grava a **média** e a **variância** do tráfego em recording rules e alerta quando o tráfego atual está a mais de 3 desvios padrão da média:

```yaml
groups:
  - name: anomaly
    rules:
      - record: job:http_requests:rate5m
        expr: sum by (job) (rate(http_requests_total[5m]))
      - record: job:http_requests:rate5m:avg_1w
        expr: avg_over_time(job:http_requests:rate5m[1w])
      - record: job:http_requests:rate5m:stdvar_1w
        expr: stdvar_over_time(job:http_requests:rate5m[1w])

      - alert: TrafegoAnomalo
        expr: |
          abs(job:http_requests:rate5m - job:http_requests:rate5m:avg_1w)
            / sqrt(job:http_requests:rate5m:stdvar_1w) > 3
        for: 10m
```

**Decisão:** guardar a **variância** (e não o desvio padrão) permite **combinar** janelas e instâncias depois: variâncias se somam/tiram média, desvios padrão não. A raiz é tirada só no final, com `sqrt()`.

### 2. Juntando o desvio padrão de várias réplicas

```promql
sqrt(avg by (job) (stdvar_over_time(instance:latency_seconds:avg1m[1h])))
```

O "desvio padrão típico" das réplicas é a **raiz da média das variâncias** (desvio padrão agrupado), e não a média dos desvios padrão. É o mesmo raciocínio do painel 2.

### 3. RMS em telemetria industrial (Modbus/SNMP exporters)

```promql
sqrt(avg_over_time((vibration_velocity_mm_s ^ 2)[10m:10s]))
```

Normas de vibração (ISO 10816) e medidas elétricas trabalham com **RMS**. Quando o exporter só entrega o valor instantâneo, a query acima calcula o RMS (painéis 3 e 4). Magnitude de 3 eixos: `sqrt(sum by (motor) (x ^ 2))` (painel 6).

### 4. "Esse pico é ruído?" com tráfego baixo

```promql
sqrt(increase(http_requests_total{code=~"5.."}[5m])) / increase(http_requests_total{code=~"5.."}[5m])
```

```yaml
- alert: TaxaDeErroAlta
  expr: |
    sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 0.05
      and sum(increase(http_requests_total[5m])) > 100    # ruído relativo < sqrt(100)/100 = 10%
  for: 10m
```

Contagens de eventos raros seguem aproximadamente uma distribuição de Poisson, cujo desvio padrão é `sqrt(N)`. Com 4 erros em 5 min, o ruído relativo é `sqrt(4)/4 = 50%`: um alerta de taxa de erro com tão pouco tráfego vai ser instável. Isso justifica exigir um mínimo de requisições no alerta (`and increase(http_requests_total[5m]) > 100`).

## ✅ Quando usar

- **Desvio padrão a partir de variância** (exportada pela aplicação ou somada de fontes independentes).
- **RMS / RMSE:** tensão/corrente elétrica, vibração, erro de previsão.
- **Magnitude de vetores:** vibração em 3 eixos, aceleração, distância euclidiana.
- **Comprimir escala** para visualização de valores muito desiguais (menos agressivo que log): `sqrt(x)`.
- **Regra "raiz de N"**: erro de amostragem ~ `1/sqrt(n)`; capacidade ~ `sqrt(carga)` em alguns modelos de fila.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer desvio padrão de uma série no tempo | [`stddev_over_time()`](../stddev_over_time/) direto |
| Quer desvio padrão **entre séries** | o agregador `stddev by (...)` |
| Desvio padrão de histograma | [`histogram_stddev()`](../histogram_stddev/) |
| Valores cobrem várias **ordens de grandeza** | [`log10()`](../log10/) |
| Pode haver valores negativos | `sgn(x) * sqrt(abs(x))` (preserva o sinal), ou [`clamp_min(x, 0)`](../clamp_min/) antes |

## ⚠️ Pegadinhas

1. **Negativos viram `NaN`.** Uma "variância" calculada à mão com cancelamento numérico pode dar `-0.0000001` e o `sqrt` devolve `NaN`. Use `sqrt(clamp_min(x, 0))`.
2. **Entre 0 e 1 a raiz aumenta:** `sqrt(0.25) = 0.5`. Cuidado ao aplicar em razões (`0..1`).
3. **Não some desvios padrão:** `stddev_a + stddev_b` está errado. Some as variâncias e tire a raiz: `sqrt(var_a + var_b)`.
4. **Média de raízes ≠ raiz da média:** `avg(sqrt(x))` ≠ `sqrt(avg(x))`. Para RMS, a ordem é **quadrado → média → raiz**.
5. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- `sqrt()` recebe **instant vector**; `sqrt(-1)` → **NaN** (não é erro).
- Relação `stddev = sqrt(stdvar)` (funções `stddev_over_time`/`stdvar_over_time`, agregadores `stddev`/`stdvar`, `histogram_stddev`/`histogram_stdvar`).
- Para aplicar uma função `_over_time` sobre uma **expressão** (ex.: `x ^ 2`), é preciso **subquery** `[janela:resolução]`.
- Operador de potência `^` (e sua associatividade à direita: `2 ^ 3 ^ 2 = 2 ^ 9`).

**1.** Qual expressão é equivalente a `stddev_over_time(node_load1[10m])`?

- A) `sqrt(avg_over_time(node_load1[10m]))`
- B) `sqrt(stdvar_over_time(node_load1[10m]))`
- C) `stdvar_over_time(sqrt(node_load1)[10m:])`
- D) `sqrt(node_load1)`

<details><summary>Resposta</summary>

**B.** Desvio padrão é a raiz quadrada da variância.
</details>

**2.** Quanto vale `sqrt(vector(-4))`?

- A) -2
- B) 2
- C) NaN
- D) erro de execução

<details><summary>Resposta</summary>

**C.** Não existe raiz real de número negativo; o PromQL segue o float64 e devolve NaN sem erro.
</details>

**3.** Como calcular o RMS da tensão `v` na última hora?

- A) `sqrt(avg_over_time(v[1h]))`
- B) `avg_over_time(sqrt(v)[1h:])`
- C) `sqrt(avg_over_time((v ^ 2)[1h:1m]))`
- D) `sqrt(sum_over_time(v[1h]) ^ 2)`

<details><summary>Resposta</summary>

**C.** Raiz (`sqrt`) da média (`avg_over_time`) dos quadrados (`v ^ 2`). Como `v ^ 2` é uma expressão, é preciso subquery. A) é a raiz da média (≈ 0 para AC).
</details>

**4.** Você tem os desvios padrão de latência de 3 réplicas independentes: 3 ms, 4 ms e 12 ms. Qual o desvio padrão da **soma** das três?

- A) 19 ms
- B) 13 ms (`sqrt(9 + 16 + 144)`)
- C) 6.33 ms (média)
- D) 12 ms (máximo)

<details><summary>Resposta</summary>

**B.** Para variáveis independentes, **variâncias** se somam: `9 + 16 + 144 = 169`, e `sqrt(169) = 13`.
</details>

## 📝 Cola rápida

- `sqrt(v instant-vector)`; `sqrt(-x) = NaN`, `sqrt(+Inf) = +Inf`, `sqrt(0.25) = 0.5` (entre 0 e 1 a raiz cresce).
- `stddev = sqrt(stdvar)`. Some/tire média de **variâncias**, e só no fim aplique `sqrt`.
- RMS = `sqrt(avg_over_time((x ^ 2)[janela:passo]))` (subquery porque `x ^ 2` é expressão).
- Magnitude: `sqrt(sum by (grupo) (x ^ 2))`.
- Ruído de Poisson: `sqrt(N)`; com poucos eventos, taxas são instáveis.

## 🔗 Relacionadas

[`stddev_over_time()`](../stddev_over_time/) · [`stdvar_over_time()`](../stdvar_over_time/) · [`histogram_stddev()`](../histogram_stddev/) · [`abs()`](../abs/) · [`log10()`](../log10/) · [`exp()`](../exp/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#sqrt
