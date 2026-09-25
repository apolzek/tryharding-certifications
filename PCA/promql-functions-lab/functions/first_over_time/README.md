# `first_over_time()`: a primeira amostra dentro da janela

> **Em uma frase:** `first_over_time(v[janela])` devolve, **para cada série**, a **amostra mais antiga que está dentro** da janela. Parece `v offset janela`, mas não é: o `offset` procura **fora e antes** da janela, e se não havia nada lá, devolve **nada**.

| | |
|---|---|
| **Assinatura** | `first_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer uma (gauge, counter, native histogram: float e histograma tratados igual) |
| **Unidade do resultado** | a **mesma** da entrada (e o `__name__` é **mantido**) |
| **Dashboard** | http://localhost:3300/d/fn-first_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a primeira foto do rolo vs "a foto de 1 hora atrás"

Você tem o rolo de fotos do seu celular.

- **`first_over_time(x[1h])`**: "abra as fotos **da última hora** e me mostre **a primeira**". Se você só começou a fotografar há 10 min, a resposta é a foto de 10 min atrás. Existe resposta.
- **`x offset 1h`**: "me mostre a foto **mais recente tirada até 1 hora atrás**" (olhando até 5 min para trás desse ponto, o lookback). Se você não tinha fotos naquela hora, a resposta é **"não tenho"**.

```
tempo ──────────────────────────────────────────────────────►
        [·····  lookback 5m  ·····]│◄──────── janela [1m] ────────►│
                                   t-1m                            t
                  x offset 1m ─────┘ (última amostra ≤ t-1m)
                                     └─ first_over_time(x[1m]) (1ª amostra > t-1m)
```

Com dados contínuos, as duas respostas diferem por no máximo 1 scrape. Com séries que **nascem** ou são **esparsas**, elas divergem bastante.

(Por série e no tempo →, como toda `*_over_time`; veja a planilha em [`avg_over_time`](../avg_over_time/).)

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `first_over_time_queue_depth` | gauge contínuo | senóide de **período 5 min**, entre **40 e 160** |
| `first_over_time_job_progress_percent{job_name="migration"}` | gauge efêmero | a série **existe só 100s a cada 3 min**: começa em **20%** (retomado do checkpoint) e sobe **0,8%/s** até ~99%; depois some |

```bash
curl -s localhost:8088/metrics | grep '^first_over_time_'
# first_over_time_queue_depth 131
# first_over_time_job_progress_percent{job_name="migration"} 44   <- só enquanto o job roda
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-first_over_time
```

Espere **~5 min** para ver várias execuções do job.

---

## 🔍 Queries passo a passo

### 1. Série contínua: praticamente iguais

```promql
first_over_time_queue_depth
first_over_time(first_over_time_queue_depth[2m])
first_over_time_queue_depth offset 2m
```

**Resultado esperado:** a onda crua e **duas cópias dela deslocadas 2 min para a direita**, uma em cima da outra. A diferença entre `first_over_time[2m]` e `offset 2m` é de no máximo **1 scrape (5s)**: a primeira pega a amostra logo **depois** de `t−2m`, o offset pega a logo **antes**.

---

### 2. Variação exata na janela

```promql
first_over_time_queue_depth - first_over_time(first_over_time_queue_depth[2m])
delta(first_over_time_queue_depth[2m])
```

**O que faz:** "quanto a fila mudou desde a primeira amostra da janela", **sem extrapolação**.
**Resultado esperado:** duas ondas parecidas entre ≈ **−110 e +110**, mas o [`delta()`](../delta/) fica **~4% maior** em módulo: ele extrapola a diferença dos ~115s cobertos pelas amostras para os 120s da janela (120/115 ≈ 1,04). `x - first_over_time(x[j])` é o valor "honesto" das amostras reais.

---

### 3. A série efêmera (cru)

```promql
first_over_time_job_progress_percent
```

**Resultado esperado:** rampas de **20% → ~99%** durando 100s, uma a cada 3 min, com 80s de vazio entre elas.

---

### 4. Aqui `first_over_time` e `offset` diferem

```promql
first_over_time(first_over_time_job_progress_percent[1m])
first_over_time_job_progress_percent offset 1m
```

**Resultado esperado, a cada execução:**

| momento (desde o início do job) | `first_over_time[1m]` | `offset 1m` |
|---|---|---|
| 0–60 s | **20** (a 1ª amostra da execução) | **vazio** (1 min antes o job não existia) |
| 60–100 s | sobe de ~20 a ~52 | sobe de 20 a ~52 |
| 100–160 s (job já morreu) | continua até a janela esvaziar | continua (mostra o passado) |
| 160–180 s | vazio | vazio |

---

### 5. "Progresso no último minuto"

```promql
first_over_time_job_progress_percent - first_over_time(first_over_time_job_progress_percent[1m])
first_over_time_job_progress_percent - (first_over_time_job_progress_percent offset 1m)
```

**Resultado esperado:**
- com `first_over_time`: começa em **0** no 1º scrape e sobe até **~45** (0,8%/s × ~55s), existindo durante **toda** a execução.
- com `offset`: **só aparece depois de 60s** de execução, fixo em **~48**. Os primeiros 60s de cada job ficam sem resposta.

> 💡 Por isso a documentação recomenda `first_over_time(m[step()])` em **range queries**: garante que a amostra usada em cada ponto esteja **dentro** daquele passo do gráfico, em vez de vir de um ponto anterior pelo lookback.

---

## 🏭 Casos reais

### 1. "Quanto este pod cresceu desde que subiu?" (memória de pod novo)

Um pod subiu há 20 min. Quer saber quanto a memória cresceu na última hora:

```promql
container_memory_working_set_bytes{pod="checkout-7d9f"}
  - first_over_time(container_memory_working_set_bytes{pod="checkout-7d9f"}[1h])
```

Com `offset 1h`, o resultado seria **vazio** (o pod não existia 1h atrás). Com `first_over_time`, compara com a **primeira amostra do pod** e responde "cresceu 300 MiB desde o boot".

Como alerta de vazamento de memória que funciona **também para pods recém-criados**:

```yaml
- alert: PossivelMemoryLeak
  expr: |
    (container_memory_working_set_bytes{container!=""}
      - first_over_time(container_memory_working_set_bytes{container!=""}[1h]))
    > 500 * 1024 * 1024
  for: 30m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.pod }} cresceu mais de 500 MiB na última hora"
```

### 2. Consumo de um gauge "acumulado" com reset diário (sem extrapolação)

Alguns medidores (energia, cota de API) expõem um gauge que acumula desde 00:00. Para um relatório "consumo das últimas 24h" em recording rule:

```yaml
- record: tenant:api_quota_used:increase1d
  expr: api_quota_used - first_over_time(api_quota_used[1d])
```

Diferente do `delta()`, o resultado é exatamente a diferença entre amostras reais (sem extrapolação), bom para cobrança/relatório.

### 3. Painéis Grafana com dados esparsos: `first_over_time(x[$__interval])`

Num painel de 7 dias, cada ponto representa ~10 min. Com `x` puro, o ponto pega a amostra do lookback, que pode ser do intervalo **anterior**. Com:

```promql
first_over_time(node_hwmon_temp_celsius[$__interval])
```

cada ponto usa a primeira amostra **daquele** intervalo (equivalente ao `first_over_time(m[step()])` da documentação).

### 4. Valor de abertura (estilo "candlestick")

Para um painel de "abertura/fechamento/máx/mín" de latência por hora: `first_over_time(x[1h])` (abertura), `last_over_time(x[1h])` (fechamento), `max_over_time`, `min_over_time`.

---

## ✅ Quando usar

- **Diferença desde o começo da janela**, sem extrapolação: `x - first_over_time(x[j])`.
- **Séries que nasceram há pouco** (pods novos, jobs), onde `offset` devolve vazio.
- **Range queries com dados esparsos:** `first_over_time(x[step()])` / `first_over_time(x[$__interval])`.
- **Valor inicial** de uma execução/sessão (com janela cobrindo só ela).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer o valor **exatamente** de X tempo atrás, em série contínua | `x offset X` (mais barato: não lê a janela inteira) |
| Quer o valor **mais recente** | seletor puro `x` ou [`last_over_time()`](../last_over_time/) |
| Variação de counter | [`increase()`](../increase/) (trata resets) |
| Variação de gauge com extrapolação "por segundo" | [`delta()`](../delta/) / [`deriv()`](../deriv/) |

## ⚠️ Pegadinhas

1. **`first_over_time(x[1m])` ≠ `x offset 1m`.** O primeiro olha **dentro** de `(t−1m, t]`; o segundo, a última amostra **em ou antes** de `t−1m` (até 5 min antes, o lookback).
2. **Não trata resets:** em counters, `x - first_over_time(x[1h])` fica **negativo** após um restart. Use `increase()`.
3. **O "primeiro" depende do começo da janela**, não do começo da série: com a janela deslizando, o "valor inicial" muda a cada passo.
4. **Mantém `__name__`** (como `last_over_time`).
5. **Custo:** lê a janela toda; em janelas enormes (`[30d]`) é mais caro que `offset`.

## 🎓 Na prova PCA

O que costuma cair:
- Diferença entre **range vector + função** e **modificador `offset`** (e o `@`).
- Comportamento com séries que **não existiam** no ponto do offset.
- Que `first_over_time` e `last_over_time` retornam **uma amostra** (não agregam valores) e preservam o nome.

**1.** Um pod subiu há 10 min. O que retorna `container_memory_working_set_bytes{pod="p"} offset 30m`?

- A) O valor de 10 min atrás
- B) Nenhum resultado
- C) Zero
- D) Erro

<details><summary>Resposta</summary>

**B.** Não havia amostra em (ou até 5 min antes de) `t−30m`. `first_over_time(...[30m])` retornaria a primeira amostra do pod.
</details>

**2.** Qual a principal diferença entre `first_over_time(x[1h])` e `x offset 1h`?

- A) Nenhuma
- B) `first_over_time` pega a primeira amostra **dentro** da janela; `offset` pega a última amostra **até** `t−1h`
- C) `offset` só funciona com counters
- D) `first_over_time` extrapola o valor

<details><summary>Resposta</summary>

**B.** É exatamente o que diz a documentação oficial. Nenhuma das duas extrapola.
</details>

**3.** Qual expressão é válida?

- A) `first_over_time(rate(x[5m]))`
- B) `first_over_time(x)`
- C) `first_over_time(rate(x[5m])[1h:])`
- D) `first_over_time(x[1h], 5m)`

<details><summary>Resposta</summary>

**C.** `first_over_time` exige um **range vector**; `rate(...)` é instant vector, então precisa de **subquery** `[1h:]`. A e B passam instant vector; D tem argumento a mais.
</details>

**4.** `x - first_over_time(x[1h])` em um **counter** que reiniciou há 10 min dá:

- A) O aumento correto
- B) Um número negativo ou menor que o real
- C) Sempre zero
- D) Erro de tipo

<details><summary>Resposta</summary>

**B.** Não há tratamento de reset. Para counters use `increase(x[1h])`.
</details>

## 📝 Cola rápida

- `first_over_time(x[j])` = **1ª amostra dentro** de `(t−j, t]`; `x offset j` = última amostra **até** `t−j` (com lookback).
- Série nova/esparsa: `offset` → **vazio**; `first_over_time` → **valor**.
- `x - first_over_time(x[j])` = variação **sem extrapolação** (≠ `delta`), **sem** tratar reset (≠ `increase`).
- `first_over_time(m[step()])` em range queries mantém a amostra **dentro do passo**.
- Mantém `__name__`.

## 🔗 Relacionadas

[`last_over_time()`](../last_over_time/) · [`delta()`](../delta/) · [`increase()`](../increase/) · [`ts_of_first_over_time()`](../ts_of_first_over_time/) · [`present_over_time()`](../present_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
