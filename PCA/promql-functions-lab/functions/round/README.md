# `round()`: o valor mais próximo (inteiro ou múltiplo que você escolher)

> **Em uma frase:** `round(v, to_nearest=1)` leva cada valor para o **múltiplo de `to_nearest` mais próximo**. Sem o 2º argumento, é o inteiro mais próximo (`61.37 → 61`). Com ele, você arredonda para 0.5, 5, 50, 0.001... **Empates sobem** (`2.5 → 3`, `-2.5 → -2`).

| | |
|---|---|
| **Assinatura** | `round(v instant-vector, to_nearest=1 scalar) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (conversões de unidade, razões) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada, agora em "degraus" de `to_nearest` |
| **Dashboard** | http://localhost:3300/d/fn-round |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o ímã e a régua

Imagine uma régua com **marcas**, e cada valor é uma bolinha de metal. O `round()` é um **ímã** em cada marca: a bolinha é puxada para a **marca mais próxima**.

```
to_nearest = 1        60      61      62
                  ────┼───────┼───────┼────
                              ▲ ●61.37 → vai pra 61

to_nearest = 0.5      61.0  61.5  62.0
                  ────┼─────┼─────┼────
                          ●61.37 → vai pra 61.5

to_nearest = 5        55      60      65
                  ────┼───────┼───────┼────
                              ●61.37 → vai pra 60
```

Se a bolinha está **exatamente no meio** (61.5 com marcas de 1 em 1), o PromQL desempata **para cima** (em direção ao +∞): `round(61.5) = 62`, `round(-2.5) = -2`.

Por dentro, a conta é: `floor(x / to_nearest + 0.5) * to_nearest`.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `round_node_hwmon_temp_celsius{chip="platform_coretemp_0",sensor="temp1"}` | `node_hwmon_temp_celsius` (node_exporter) | gauge | onda **62 ± 4 °C** (3 min) + ruído de ±0.4 °C: valores como `61.3718...` |
| `round_container_memory_working_set_bytes{pod="worker-01".."worker-12"}` | `container_memory_working_set_bytes` (cAdvisor) | gauge | 12 pods centrados em **100, 210, 320 ... 1310 MiB**, cada um oscilando **±40 MiB** (4 min) |

```bash
curl -s localhost:8088/metrics | grep '^round_'
# round_node_hwmon_temp_celsius{chip="platform_coretemp_0",sensor="temp1"} 61.3718
# round_container_memory_working_set_bytes{pod="worker-04"} 4.6e+08
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-round
```

Os painéis aparecem em **~1 minuto**; para ver os pods trocando de faixa, espere **~4 minutos**.

---

## 🔍 Queries passo a passo

### 1. Inteiro mais próximo

```promql
round_node_hwmon_temp_celsius
round(round_node_hwmon_temp_celsius)
```

**Resultado esperado:** a linha crua é "nervosa" (61.37, 61.82, 62.14...). A de `round()` vira **degraus de 1 °C** (58, 59 ... 66). Exemplos: `61.37 → 61`, `61.50 → 62`, `61.49 → 61`.

---

### 2. `to_nearest`: escolhendo o tamanho do degrau

```promql
round(round_node_hwmon_temp_celsius, 0.5)
round(round_node_hwmon_temp_celsius, 5)
```

**Resultado esperado:**

| cru | `round(x)` | `round(x, 0.5)` | `round(x, 5)` |
|---|---|---|---|
| 58.20 | 58 | **58.0** | **60** |
| 61.37 | 61 | **61.5** | **60** |
| 62.74 | 63 | **62.5** | **65** (já passou de 62.5, o meio entre 60 e 65) |
| 65.90 | 66 | **66.0** | **65** |

Com `0.5` a linha tem degraus finos de meio grau. Com `5`, ela fica **pulando entre 60 e 65**, porque a onda cruza 62.5 (o meio entre 60 e 65). Nunca chega a 55: o mínimo (~57.6 °C) ainda está acima de 57.5.

> 💡 `to_nearest` pode ser **fração**: `round(x, 0.001)` deixa 3 casas decimais (precisão de milissegundo para valores em segundos).

---

### 3 e 4. Memória dos pods arredondada para múltiplos de 256 MiB

```promql
round_container_memory_working_set_bytes / 2^20                 # bytes → MiB
round(round_container_memory_working_set_bytes / 2^20, 256)      # nível de 256 MiB mais próximo
```

**O que faz:** primeiro converte bytes para MiB (`2^20 = 1048576`), depois puxa cada valor para o múltiplo de 256 mais próximo.
**Resultado esperado:** 12 linhas onduladas viram 12 linhas **em degraus** nos níveis **0, 256, 512, 768, 1024, 1280**. Um pod que oscila entre 390 e 470 MiB fica em **512**; um que oscila entre 60 e 140 MiB **pula entre 0 e 256** toda vez que cruza **128 MiB** (o ponto do meio; exatamente 128 vai para **256**, porque o empate sobe).

> ⚠️ **Converta a unidade antes.** `round(x, 256)` em **bytes** arredondaria para múltiplos de 256 **bytes**, o que não muda nada visível.

---

### 5. Bucketing com `count_values`

```promql
sort_by_label(
  count_values("faixa_mib", round(round_container_memory_working_set_bytes / 2^20, 256)),
  "faixa_mib"
)
```

**O que faz:** `round(..., 256)` coloca cada pod na faixa mais próxima; `count_values("faixa_mib", ...)` cria um label `faixa_mib` com esse valor e **conta** quantas séries têm cada valor; `sort_by_label` (experimental, habilitada neste lab) só ordena para exibir.
**Resultado esperado:** 6 barras (`≈ 0`, `256`, `512`, `768`, `1024`, `1280 MiB`) com **1 a 3 pods** cada (por exemplo `1, 2, 2, 3, 2, 2`), somando sempre **12 pods**. As barras mudam um pouco com o tempo, conforme os pods oscilam entre faixas.

> ⚠️ `round(x, 256)` cria faixas **centradas** nas marcas (`512` = de 384 até 639.99). Se você quer faixas "de 512 até 767.99", use `floor(x / 256) * 256` (ver [`floor()`](../floor/)).

---

### 6. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `round(vector(2.5))` | **3** | empate sobe |
| `round(vector(-2.5))` | **-2** ⚠️ | empate sobe **em direção ao +∞**, não "pra longe do zero" (muita linguagem dá -3) |
| `round(vector(0.125), 0.25)` | **0.25** | 0.125 está no meio entre 0 e 0.25; empate sobe |
| `round(vector(1234), 100)` | **1200** | arredondar para centenas |
| `round(vector(1.005), 0.01)` | **1** ⚠️ | em float, `1.005` é `1.00499999...`, então vai para 1.00 e não 1.01 |
| `round(vector(7), 0)` | **NaN** | `to_nearest = 0` gera divisão por zero |
| `round(vector(7), -5)` | **5** ⚠️ | `to_nearest` negativo "funciona", mas com resultados estranhos. Não use |
| `round(vector(+Inf))` | **+Inf** | |

---

## 🏭 Casos reais

### 1. Tabela de "top consumidores de memória" legível

```promql
sort_desc(round(sum by (namespace) (container_memory_working_set_bytes{container!=""}) / 2^30, 0.1))
```

O painel de tabela do time de plataforma mostra GiB com **uma casa decimal** (`3.7 GiB`). Em painéis do Grafana prefira a opção *Decimals*, mas em **anotações de alerta**, **recording rules exportadas para outros sistemas** e respostas de API, arredondar na query evita números como `3.7182818284 GiB`.

### 2. Distribuição de pods por faixa de memória (right-sizing)

```promql
count_values("faixa_mib",
  round(max by (pod) (container_memory_working_set_bytes{namespace="batch", container!=""}) / 2^20, 256))
```

```yaml
- record: pod:memory_working_set_mib:round256
  expr: round(max by (namespace, pod) (container_memory_working_set_bytes{container!=""}) / 2^20, 256)
```

O FinOps quer saber se os `requests` de 1 GiB fazem sentido: se a maioria dos pods está na faixa de ~256 MiB, dá pra reduzir o request. `count_values` + `round` é um **histograma improvisado de um gauge**. É exatamente o painel 5 deste lab.

### 3. Temperatura "sem tremedeira" num painel de datacenter

```promql
round(avg by (instance) (node_hwmon_temp_celsius{chip=~"platform_coretemp.*"}), 0.5)
```

O NOC exibe a temperatura das CPUs num painel de status. `round(x, 0.5)` evita que o número "dance" a cada 5s (`61.37 → 61.82 → 61.44`). **Mas** o alerta continua no valor cru:

```yaml
- alert: NodeTemperaturaAlta
  expr: avg by (instance) (node_hwmon_temp_celsius{chip=~"platform_coretemp.*"}) > 85
  for: 5m
  annotations:
    summary: "CPU de {{ $labels.instance }} a {{ $value | printf \"%.1f\" }} °C"
```

Repare que, no texto do alerta, o arredondamento é feito no **template** (`printf "%.1f"`), não na expressão: arredondar a expressão com `round(x, 5)` faria 83 virar 85 e disparar antes da hora.

### 4. Alinhar timestamps para agrupar eventos

```promql
count_values("hora_do_deploy", round(kube_deployment_created, 3600))
```

Agrupa deployments pela **hora cheia mais próxima** de criação (útil para descobrir "janelas de deploy").

## ✅ Quando usar

- **Reduzir ruído visual** ou cardinalidade de valores em tabelas/alertas: `round(temperatura, 0.5)`.
- **Bucketing/faixas:** `count_values("faixa", round(latencia_ms, 100))` para um histograma "caseiro" de valores de gauges.
- **Precisão fixa** em recording rules ou anotações de alerta: `round(x, 0.01)`.
- **Agrupar versões/valores numéricos** em labels com `count_values` (ex.: quantos pods em cada faixa de uso de memória).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| "Quantos eu **preciso**?" (pods, GB cobrados) | [`ceil()`](../ceil/) — `round` pode subdimensionar |
| "Quantos estão **completos**?" (caixas, minutos) | [`floor()`](../floor/) |
| Só quer **mostrar** menos casas decimais no Grafana | a opção **Decimals** do painel (não perde dado) |
| Quer faixas `[a, a+passo)` começando no limite | `floor(x / passo) * passo` |
| Quer limitar a uma faixa mínima/máxima | [`clamp()`](../clamp/) |

## ⚠️ Pegadinhas

1. **Empate sobe, sempre em direção ao +∞:** `round(-2.5) = -2`. Não é o "arredondamento bancário" nem o "pra longe do zero".
2. **Ponto flutuante:** `round(1.005, 0.01) = 1`. Valores "exatos" em decimal nem sempre são exatos em binário.
3. **`to_nearest` precisa ser > 0:** `0` dá `NaN`; negativos dão resultados sem sentido.
4. **Arredondar antes de somar acumula erro:** `sum(round(x))` pode diferir bastante de `round(sum(x))` com muitas séries. Arredonde **no final**.
5. **Alertas com `round` ficam "pulando"**: um valor oscilando perto do meio (62.5) alterna 60/65 a cada scrape. Para alertas, compare o valor cru com o limite.
6. **Nome da métrica some** e **histogramas nativos são ignorados**.

## 🎓 Na prova PCA

O que costuma cair:
- Assinatura: `round(v instant-vector, to_nearest=1 scalar)`. O 2º argumento é **escalar** e **opcional**; `round(x, vector(5))` é erro de tipo.
- **Empate:** "ties are resolved by rounding up" → `round(2.5) = 3`, `round(-2.5) = -2`.
- `to_nearest` pode ser fração (`0.1`, `0.001`) ou maior que 1 (`10`, `100`).
- Diferença entre `round`, `ceil` e `floor`.

**1.** Quanto vale `round(vector(-0.5))`?

- A) -1
- B) 0
- C) 0.5
- D) NaN

<details><summary>Resposta</summary>

**B.** No PromQL, empates são resolvidos **para cima** (em direção ao +∞): `floor(-0.5 + 0.5) = 0`. Em muitas linguagens o resultado seria -1 ("half away from zero").
</details>

**2.** Qual expressão arredonda a latência (em segundos) para o **milissegundo** mais próximo?

- A) `round(latency_seconds, 1000)`
- B) `round(latency_seconds, 0.001)`
- C) `round(latency_seconds * 0.001)`
- D) `round(latency_seconds, 3)`

<details><summary>Resposta</summary>

**B.** `to_nearest` é o **múltiplo** desejado (0.001 s = 1 ms), não o número de casas decimais. D) arredondaria para múltiplos de 3 segundos. A) para múltiplos de 1000 s.
</details>

**3.** Qual é o tipo esperado do segundo argumento de `round()`?

- A) instant vector
- B) range vector
- C) scalar
- D) string

<details><summary>Resposta</summary>

**C.** `to_nearest` é um escalar (ex.: `5`, `0.5`, ou `scalar(algum_vetor)`).
</details>

**4.** Você precisa exibir quantos pods estão em cada faixa de ~100 MiB de memória. Qual combinação resolve?

- A) `histogram_quantile(0.5, container_memory_working_set_bytes)`
- B) `count_values("faixa", round(container_memory_working_set_bytes / 2^20, 100))`
- C) `round(count(container_memory_working_set_bytes), 100)`
- D) `sum by (le) (container_memory_working_set_bytes)`

<details><summary>Resposta</summary>

**B.** `round(..., 100)` leva cada pod para o múltiplo de 100 MiB mais próximo e `count_values` conta quantas séries têm cada valor, criando o label `faixa`. A) exige um histograma com buckets `le`. C) arredonda a **contagem** total. D) não há label `le` num gauge.
</details>

**5.** Por que `round(vector(1.005), 0.01)` retorna `1` e não `1.01`?

- A) `round` sempre arredonda pra baixo
- B) `to_nearest` menor que 1 é ignorado
- C) `1.005` não é representável exatamente em float64; vale `1.00499999...`
- D) bug de versão

<details><summary>Resposta</summary>

**C.** Limitação do ponto flutuante IEEE 754. O valor armazenado é ligeiramente menor que 1.005.
</details>

## 📝 Cola rápida

- `round(v, to_nearest=1)`: múltiplo de `to_nearest` mais próximo; `to_nearest` é **escalar**, pode ser fração.
- Empate **sobe** (para +∞): `round(2.5) = 3`, `round(-2.5) = -2`, `round(-0.5) = 0`.
- `to_nearest = 0` → NaN; não use valores ≤ 0.
- Bucketing de gauge: `count_values("faixa", round(x, passo))`.
- Para alertas e capacity: compare o valor **cru** (ou use `ceil`), não o arredondado.

## 🔗 Relacionadas

[`ceil()`](../ceil/) · [`floor()`](../floor/) · [`clamp()`](../clamp/) · [`count_values`](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#round
