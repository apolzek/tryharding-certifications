# `log2()`: logaritmo base 2 (quantas dobras? qual a próxima potência de 2?)

> **Em uma frase:** `log2(v)` responde "**2 elevado a quanto dá esse valor?**", ou seja, **quantas vezes você dobra 1 até chegar nele**: `log2(8) = 3`, `log2(1024) = 10`, `log2(0.5) = -1`. É a função natural para **dobras**, **bits** e **potências de 2** (tamanhos de memória, buckets de native histograms, HPA escalando em dobro).

| | |
|---|---|
| **Assinatura** | `log2(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões **positivos** · ⚠️ `0 → -Inf`, negativo → `NaN` · histogramas são ignorados |
| **Unidade do resultado** | "número de dobras" (ou **bits**) |
| **Dashboard** | http://localhost:3300/d/fn-log2 |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: dobrar a folha de papel

Pegue uma folha e dobre ao meio: 2 camadas. De novo: 4. De novo: 8. O `log2` é o **contador de dobras**:

```
 camadas:   1    2    4    8    16    32   ...  1024
 log2:      0    1    2    3     4     5   ...    10
            └─+1─┘└─+1─┘└─+1─┘└─+1─┘└─+1─┘
              cada DOBRA soma exatamente 1
```

E o caminho de volta é `2 ^ n`. Juntando com [`ceil()`](../ceil/), dá pra responder "**qual a menor potência de 2 que cabe esse valor?**": `2 ^ ceil(log2(700)) = 2 ^ 10 = 1024`.

Onde potências de 2 aparecem no dia a dia de SRE: memória (MiB, GiB = 2³⁰ bytes), tamanhos de buffer e de bloco, **buckets exponenciais** de histogramas (`prometheus.ExponentialBuckets(…, 2, …)` e os native histograms), backoff exponencial (1 s, 2 s, 4 s, 8 s...), HPA dobrando réplicas.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `log2_kube_deployment_status_replicas{deployment="checkout"}` | `kube_deployment_status_replicas` | gauge | HPA agressivo: **2 → 4 → 8 → 16 → 32** (dobra a cada minuto), ciclo de 5 min |
| `log2_container_memory_working_set_bytes{pod="api-7d9f"}` | `container_memory_working_set_bytes` | gauge | **~300 MiB** (±3%) |
| `...{pod="worker-5c2a"}` | idem | gauge | **~700 MiB** |
| `...{pod="search-9b1e"}` | idem | gauge | **~1.5 GiB** |
| `...{pod="cache-3f8d"}` | idem | gauge | **~3.1 GiB** |

```bash
curl -s localhost:8088/metrics | grep '^log2_'
# log2_kube_deployment_status_replicas{deployment="checkout"} 8
# log2_container_memory_working_set_bytes{pod="worker-5c2a"} 7.43e+08
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-log2
```

Espere **~5 minutos** para ver um ciclo completo do HPA.

---

## 🔍 Queries passo a passo

### 🟢 1. Réplicas cruas

```promql
log2_kube_deployment_status_replicas
```

**Resultado esperado:** degraus **2, 4, 8, 16, 32**, um por minuto, e volta a 2. Na escala linear os degraus ficam **cada vez mais altos** (+2, +4, +8, +16): o olho acha que "acelerou", mas o HPA fez **a mesma coisa** toda vez (dobrou).

---

### 🟢 2. `log2`: dobras viram degraus iguais

```promql
log2(log2_kube_deployment_status_replicas)          # 1, 2, 3, 4, 5
log2(log2_kube_deployment_status_replicas / 2)      # dobras desde o normal: 0, 1, 2, 3, 4
```

**Resultado esperado:**

| réplicas | `log2(x)` | `log2(x / 2)` (dobras desde 2) |
|---|---|---|
| 2 | **1** | **0** |
| 4 | **2** | **1** |
| 8 | **3** | **2** |
| 16 | **4** | **3** |
| 32 | **5** | **4** |

Os degraus agora são **todos do mesmo tamanho**. Dividir pelo valor "normal" antes do `log2` dá **quantas vezes** o sistema dobrou em relação ao baseline: `log2(a / b) = log2(a) - log2(b)`.

---

### 🟡 3 e 4. Memória em "bits"

```promql
log2_container_memory_working_set_bytes
log2(log2_container_memory_working_set_bytes)
```

**Resultado esperado:**

| pod | memória | `log2(bytes)` | leitura |
|---|---|---|---|
| api-7d9f | ~300 MiB | **≈ 28.2** | entre 2²⁸ (256 MiB) e 2²⁹ (512 MiB) |
| worker-5c2a | ~700 MiB | **≈ 29.5** | entre 512 MiB e 1 GiB |
| search-9b1e | ~1.5 GiB | **≈ 30.6** | entre 1 GiB (2³⁰) e 2 GiB |
| cache-3f8d | ~3.1 GiB | **≈ 31.6** | entre 2 GiB e 4 GiB |

Referências para decorar: **2¹⁰ = 1 Ki**, **2²⁰ = 1 Mi**, **2³⁰ = 1 Gi**. Um `log2(bytes)` de 30.6 é "um pouco mais de 1 GiB".

---

### 🔴 5. Próxima potência de 2: sugestão de limite de memória

```promql
2 ^ ceil(log2(log2_container_memory_working_set_bytes))
```

**O que faz:** `log2` diz "quantos bits", `ceil` arredonda para o próximo inteiro, e `2 ^` volta para bytes. Resultado: a **menor potência de 2 ≥ uso atual**.
**Resultado esperado:** api → **512 MiB**, worker → **1 GiB**, search → **2 GiB**, cache → **4 GiB**.

> ⚠️ Repare que o search (1.5 GiB) ganha 2 GiB (33% de folga), mas o worker (700 MiB) ganha 1 GiB (46%) e um pod com 1.01 GiB ganharia 2 GiB (98%!). Potência de 2 é uma **heurística** de arredondamento, não uma regra de capacidade.

---

### ⚠️ 6. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `log2(vector(1024))` | **10** | 2¹⁰ = 1024 |
| `log2(vector(3))` | **1.585...** | não precisa ser potência exata |
| `log2(vector(1))` | **0** | 2⁰ = 1 |
| `log2(vector(0.5))` | **-1** | valores entre 0 e 1 dão negativo |
| `log2(vector(0))` | **-Inf** ⚠️ | um deployment escalado a **0 réplicas** vira `-Inf` e quebra `avg`/gráficos |
| `log2(vector(-8))` | **NaN** | negativo |
| `log2(vector(+Inf))` | **+Inf** | |

(A doc diz que os casos especiais de `log2` são **os mesmos do `ln`**.)

---

## 🏭 Casos reais

### 1. "O HPA dobrou quantas vezes na última hora?"

```promql
log2(
  kube_deployment_status_replicas{deployment="checkout"}
  / kube_deployment_status_replicas{deployment="checkout"} offset 1h
)
```

`3` = o deployment está **8×** maior que há uma hora. Alerta para escalonamento explosivo (normalmente sinal de loop de retry ou ataque):

```yaml
- alert: EscalonamentoExplosivo
  expr: |
    log2(
      kube_deployment_status_replicas
      / (kube_deployment_status_replicas offset 30m > 0)
    ) >= 3
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "{{ $labels.deployment }} dobrou {{ $value }} vezes em 30 min"
```

**Decisão:** o `> 0` dentro do denominador evita divisão por zero (deployment escalado a 0 há 30 min → `+Inf` → `log2(+Inf) = +Inf`).

### 2. Right-sizing de limites de memória (recording rule)

```yaml
groups:
  - name: rightsizing
    rules:
      - record: pod:memory_limit_sugerido:pow2
        expr: |
          2 ^ ceil(log2(
            max_over_time(container_memory_working_set_bytes{container!=""}[7d]) * 1.2
          ))
```

Pico de 7 dias + 20% de margem, arredondado para a próxima potência de 2. O time de plataforma compara com `kube_pod_container_resource_limits{resource="memory"}` para achar limites exagerados (4× a sugestão) ou apertados.

### 3. Native histograms: em qual bucket cai esse valor?

Native histograms com **schema 0** têm fronteiras em potências de 2 (…, 0.25, 0.5, 1, 2, 4, …). Com schema `s`, o fator de crescimento é `2^(2^-s)`. O índice do bucket de um valor `x` no schema 0 é `ceil(log2(x))`:

```promql
ceil(log2(histogram_quantile(0.99, sum(rate(http_request_duration_seconds[5m])))))
```

Se o p99 = 0.3 s → `log2(0.3) = -1.74` → bucket **-1** (`(0.25, 0.5]`). Útil para entender a **resolução** do quantil: com schema 0, o erro relativo pode chegar a 100%; o schema 3 (fator ≈ 1.09) é o padrão do client_golang para `NativeHistogramBucketFactor: 1.1`.

### 4. Backoff exponencial: em qual tentativa está o cliente?

```promql
log2(retry_backoff_seconds / 0.1)      # backoff base de 100 ms → número da tentativa
```

Um backoff de 6.4 s com base 0.1 s = `log2(64) = 6` → está na 6ª dobra.

---

## ✅ Quando usar

- **Contar dobras** (escalonamento, crescimento, backoff): `log2(atual / base)`.
- **Próxima potência de 2**: `2 ^ ceil(log2(x))` para tamanhos de buffer, limites, shards.
- **Bits/entropia**: quantos bits para representar N valores → `ceil(log2(N))`.
- **Entender buckets exponenciais** de histogramas (clássicos com fator 2 e native histograms).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer "ordem de grandeza" decimal (ms, s, dezenas...) | [`log10()`](../log10/) |
| Quer taxa de crescimento contínua / tempo de dobra | [`ln()`](../ln/) (`ln(2)/r`) |
| Só quer **visualizar** em escala log | eixo **Logarithmic (base 2)** do Grafana |
| O valor pode ser 0 (deployment escalado a zero) | filtre antes: `log2(x > 0)` |

## ⚠️ Pegadinhas

1. **Zero vira `-Inf`** (ex.: 0 réplicas, 0 bytes) e **negativo vira `NaN`**. Filtre com `x > 0`.
2. **Divisão por zero no ratio:** `log2(a / b)` com `b = 0` → `log2(+Inf) = +Inf`. Filtre o denominador.
3. **`2 ^ ceil(log2(x))` numa potência exata:** `log2(1024) = 10` exato, então fica 1024. Mas ponto flutuante pode dar `10.000000000000002` e pular para 2048. Use `ceil(log2(x) - 1e-9)` se precisar de exatidão.
4. **Precedência:** `2 ^ ceil(log2(x))` precisa do `^` fora; `^` é associativo à **direita** (`2 ^ 3 ^ 2 = 2 ^ 9 = 512`).
5. **Nome da métrica some** e **histogramas nativos são ignorados** (o `log2` não mexe em buckets; ele age em valores float).

## 🎓 Na prova PCA

O que costuma cair:
- Casos especiais **iguais aos do `ln`**: `log2(0) = -Inf`, `log2(<0) = NaN`, `log2(+Inf) = +Inf`, `log2(NaN) = NaN`.
- Operador de potência `^` como inverso (`2 ^ log2(x) = x`).
- Native histograms: buckets exponenciais com fator `2^(2^-schema)`.

**1.** Quanto vale `log2(vector(0.25))`?

- A) 2
- B) -2
- C) 0.5
- D) NaN

<details><summary>Resposta</summary>

**B.** 2⁻² = 1/4 = 0.25. Valores entre 0 e 1 têm logaritmo negativo.
</details>

**2.** Qual expressão retorna a **menor potência de 2 maior ou igual** a `x`?

- A) `log2(ceil(x))`
- B) `2 ^ ceil(log2(x))`
- C) `ceil(2 ^ x)`
- D) `2 * ceil(x / 2)`

<details><summary>Resposta</summary>

**B.** `log2` dá o expoente, `ceil` arredonda o expoente para cima, `2 ^` volta para a escala original. D) dá o próximo número **par**.
</details>

**3.** Um deployment tinha 4 réplicas há 1 hora e agora tem 64. Quanto vale `log2(agora / (agora offset 1h))`?

- A) 60
- B) 16
- C) 4
- D) 6

<details><summary>Resposta</summary>

**C.** 64 / 4 = 16 = 2⁴: o deployment dobrou 4 vezes. D) seria `log2(64)`.
</details>

**4.** Em um native histogram com **schema 0**, qual é o fator de crescimento entre fronteiras de buckets consecutivos?

- A) 1.1
- B) 2
- C) 10
- D) e

<details><summary>Resposta</summary>

**B.** O fator é `2^(2^-schema)`; com schema 0, `2^(2^0) = 2^1 = 2`. Cada schema a mais divide a largura (em log2) pela metade.
</details>

## 📝 Cola rápida

- `log2(v)`: "2 elevado a quanto?"; `log2(1024) = 10`, `log2(1) = 0`, `log2(0.5) = -1`, `log2(0) = -Inf`, `log2(<0) = NaN`.
- Dobras desde a base: `log2(atual / base)`.
- Próxima potência de 2: `2 ^ ceil(log2(x))`.
- Referências: 2¹⁰ = Ki, 2²⁰ = Mi, 2³⁰ = Gi.
- Native histograms: fator `2^(2^-schema)`; schema 0 → fronteiras em potências de 2.

## 🔗 Relacionadas

[`ln()`](../ln/) · [`log10()`](../log10/) · [`exp()`](../exp/) · [`ceil()`](../ceil/) · [`histogram_quantile()`](../histogram_quantile/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#log2
