# `double_exponential_smoothing()`: suavizar o ruído sem perder a tendência

> **Em uma frase:** `double_exponential_smoothing(v[janela], sf, tf)` devolve uma versão **suavizada** do último valor de um gauge, levando em conta o **nível** (controlado por `sf`) e a **tendência** (controlada por `tf`). É o antigo `holt_winters` (método "Holt linear") e é **experimental**.

| | |
|---|---|
| **Assinatura** | `double_exponential_smoothing(v range-vector, sf scalar, tf scalar) → instant-vector` |
| **Parâmetros** | `sf` (smoothing factor) e `tf` (trend factor), ambos **entre 0 e 1** (exclusivo) |
| **Tipo de métrica** | ✅ Gauge (só floats) · ❌ Counter · ❌ native histograms (ignorados) |
| **Unidade do resultado** | a **mesma do gauge** (valor suavizado) |
| **Feature flag** | `--enable-feature=promql-experimental-functions` (ligada neste lab) |
| **Dashboard** | http://localhost:3300/d/fn-double_exponential_smoothing |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o piloto e o altímetro na turbulência

O altímetro de um avião treme sem parar na turbulência. O piloto não reage a cada tremida; ele mantém duas coisas na cabeça:

1. **Onde estou (nível):** "estamos a uns 3.000 pés". A cada leitura nova, ele ajusta só **um pouco** essa estimativa. Quanto ele confia na leitura nova é o **`sf`**.
   - `sf` **baixo** (0.1): "a leitura nova vale pouco, confio no que já sabia" → linha lisa, mas **atrasada**.
   - `sf` **alto** (0.9): "acredito em cada leitura" → quase igual ao dado cru, **nervoso**.
2. **Para onde estou indo (tendência):** "estamos subindo uns 500 pés/min". Quanto ele atualiza essa tendência a cada leitura é o **`tf`**.
   - `tf` **baixo**: a tendência muda devagar → demora a perceber que o avião começou a descer.
   - `tf` **alto**: percebe viradas rápido, mas pode **exagerar** (overshoot) depois de uma mudança brusca.

Em fórmula (por amostra, do começo ao fim da janela):

```
nível_t     = sf × valor_t + (1 − sf) × (nível_{t−1} + tendência_{t−1})
tendência_t = tf × (nível_t − nível_{t−1}) + (1 − tf) × tendência_{t−1}
resultado   = nível no fim da janela
```

Por que "double"? Porque são **duas** suavizações exponenciais (nível e tendência). O Prometheus 2 chamava isso de `holt_winters`, mas Holt-Winters "de verdade" tem uma terceira componente (sazonalidade), por isso o nome mudou no Prometheus 3.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `double_exponential_smoothing_node_load1{host="web-1"}` | gauge | imita o `node_load1`: **rampa em triângulo** de 6 min (sobe de **1.0 a 7.0** em 3 min, desce em 3 min) + **ruído de ±1.2** |
| `double_exponential_smoothing_probe_duration_seconds{target="https://checkout.exemplo.com"}` | gauge | imita o blackbox_exporter: **degrau** entre **0.1s e 0.3s** a cada 2 min + ruído de ±0.015s |

```bash
curl -s localhost:8088/metrics | grep '^double_exponential_smoothing_'
# double_exponential_smoothing_node_load1{host="web-1"} 5.87
# double_exponential_smoothing_probe_duration_seconds{target="https://checkout.exemplo.com"} 0.307
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-double_exponential_smoothing
```

Espere **~6 minutos** para um ciclo completo da rampa e a janela `[5m]` cheia.

---

## 🔍 Queries passo a passo

### 1. Cru × suavizado

```promql
double_exponential_smoothing_node_load1
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.3, 0.3)
```

**O que faz:** para cada ponto do gráfico, roda a suavização sobre os ~60 pontos dos últimos 5 min e devolve o nível final.
**Resultado esperado:** o cru é um "serrote peludo" subindo de ~1 a ~7 e voltando, tremendo ±1.2. A curva suavizada desenha o **triângulo** quase sem atraso (erro médio ≈ 0.5), com a tremedeira reduzida a uns ±0.3.

---

### 2. O efeito do `sf`

```promql
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.1, 0.1)   # sf baixo
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.8, 0.1)   # sf alto
```

**Resultado esperado:**
- `sf=0.1`: a linha **mais lisa** de todas, mas **atrasada**: "corta" os picos (não chega a 7) e vira bem depois da rampa (erro médio ≈ 1.5).
- `sf=0.8`: praticamente o dado cru, com quase todo o ruído (±0.7 de tremida entre pontos).

---

### 3. O efeito do `tf`

```promql
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.1, 0.1)   # tf baixo
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.1, 0.5)   # tf alto
```

**Resultado esperado:** com o mesmo `sf=0.1` (bem liso), subir o `tf` para 0.5 faz a curva **perceber que está subindo** e alcançar a rampa: o erro médio cai de ≈ 1.5 para ≈ 0.7, sem ficar muito mais nervosa. É o ponto forte do método: **liso e sem atraso na tendência**.

---

### 4. `double_exponential_smoothing` × `avg_over_time`

```promql
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.3, 0.3)
avg_over_time(double_exponential_smoothing_node_load1[2m])
```

**Resultado esperado:** a média móvel de 2 min é lisa, mas fica **no meio da rampa**, atrasada ~1 min (numa rampa de 2 unidades/min, isso é ~2 unidades abaixo no topo). O DES fica em cima da rampa porque projeta a tendência. Média simples trata todos os pontos igual e não sabe que "está subindo".

---

### 5. Degrau: `tf` alto dá overshoot

```promql
double_exponential_smoothing_probe_duration_seconds
double_exponential_smoothing(double_exponential_smoothing_probe_duration_seconds[2m], 0.3, 0.1)
double_exponential_smoothing(double_exponential_smoothing_probe_duration_seconds[2m], 0.3, 0.9)
```

**Resultado esperado:**
- **Cru:** degraus 0.1s ↔ 0.3s a cada 2 min.
- `tf=0.1`: sobe/desce em rampa (~30s) até o novo patamar, sem exagerar.
- `tf=0.9`: depois de cada degrau, **passa do ponto**: chega a ≈ **0.38s** na subida e ≈ **0.03s** na descida, e oscila antes de acomodar. Tendência alta + mudança brusca = "achou que ia continuar subindo".

---

### 6. O que dá errado: janela curta demais

```promql
double_exponential_smoothing(double_exponential_smoothing_node_load1[30s], 0.3, 0.3)   # errado
double_exponential_smoothing(double_exponential_smoothing_node_load1[5m], 0.3, 0.3)    # certo
```

**O que acontece:** a suavização começa do zero **em cada avaliação**, usando só as amostras da janela. A tendência inicial é `x₁ − x₀`, ou seja, a diferença entre as duas primeiras amostras: **puro ruído** (±2.4 aqui). Com só 6 amostras (`[30s]`), não dá tempo de esse chute inicial ser "esquecido".
**Resultado esperado:** a curva `[30s]` erra **mais que o próprio dado cru** (erro médio ≈ 1.2 contra ≈ 0.7 do cru) e às vezes fica **negativa** (load < 0, impossível). A `[5m]` com os mesmos parâmetros fica em cima da rampa (erro ≈ 0.5).

**Regra prática:** a janela precisa ter **dezenas de amostras** (várias vezes `1/sf`).

---

## 🏭 Casos reais

### 1. Painel de "tendência" de load/CPU sem ruído (imitado pelos painéis 1-4)

O dashboard da equipe de plataforma mostra o load dos nós com muito ruído. Em vez de `avg_over_time` (que atrasa), usam:

```promql
double_exponential_smoothing(node_load1{instance="web-1:9100"}[10m], 0.3, 0.3)
```

E para CPU (que vem de counter), suavizando o **resultado** de um `rate` via **subquery**:

```promql
double_exponential_smoothing(
  (1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m])))[15m:30s],
  0.3, 0.3
)
```

**Decisão:** o DES recebe um **gauge**; como `node_cpu_seconds_total` é counter, primeiro vira gauge com `rate`, depois a subquery `[15m:30s]` cria o range vector. Para não pagar a subquery em todo refresh, grave a utilização numa recording rule e suavize a série gravada:

```yaml
groups:
- name: cpu
  rules:
  - record: instance:node_cpu_utilisation:rate1m
    expr: 1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[1m]))
  - record: instance:node_cpu_utilisation:smoothed
    expr: double_exponential_smoothing(instance:node_cpu_utilisation:rate1m[15m], 0.3, 0.3)
```

### 2. Alerta de latência menos "piscante" (imitado pelo painel 5)

```yaml
- alert: ProbeLento
  expr: double_exponential_smoothing(probe_duration_seconds{job="blackbox-http"}[10m], 0.2, 0.1) > 0.5
  for: 5m
  labels: {severity: warning}
```

`tf` baixo de propósito: queremos ignorar **picos isolados** e não ter overshoot depois que a latência volta ao normal.

> ⚠️ Como a função é **experimental**, muitas empresas evitam usá-la em regras críticas (ela pode mudar entre versões e exige a feature flag em todos os Prometheus/Thanos/Mimir que avaliam as regras). Alternativas estáveis: `avg_over_time`, `quantile_over_time`, `deriv`, `predict_linear`.

### 3. Migração do Prometheus 2 → 3

Dashboards antigos com `holt_winters(x[10m], 0.3, 0.3)` **quebram** no Prometheus 3. A migração é trocar o nome para `double_exponential_smoothing` e ligar `--enable-feature=promql-experimental-functions`.

---

## ✅ Quando usar

- **Gráficos** de gauges ruidosos com tendência (load, latência média, fila, temperatura), quando você quer lisura **sem** o atraso de uma média móvel.
- **Alertas** menos sensíveis a picos isolados (com `tf` baixo).
- Estudo/experimentação de parâmetros de suavização.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **counter** | primeiro `rate()`, depois DES via subquery; ou só [`rate()`](../rate/) com janela maior |
| Quer **prever** valor futuro | [`predict_linear()`](../predict_linear/) |
| Quer só a **média** da janela | [`avg_over_time()`](../avg_over_time/) |
| Série com **sazonalidade** (dia/noite) | DES não modela sazonalidade (é o "Holt-Winters" sem o "Winters") |
| Ambiente que **não pode** ligar features experimentais | `avg_over_time`, `deriv` |

## ⚠️ Pegadinhas

1. **Experimental:** sem `--enable-feature=promql-experimental-functions` a query falha ("function not found"/desabilitada).
2. **`sf` e `tf` devem estar entre 0 e 1 (exclusivo):** `0` ou `1` geram erro.
3. **Nome antigo:** `holt_winters` não existe mais no Prometheus 3.
4. **Só gauges e floats:** em counter, você suaviza um número que só cresce (inútil); histogramas são ignorados.
5. **Precisa de amostras:** a suavização começa do primeiro ponto da janela; com janela muito curta o "aquecimento" domina o resultado (a tendência inicial é `x₁ − x₀`, que é puro ruído).
6. **`tf` alto + degraus = overshoot** (painel 5).

## 🎓 Na prova PCA

O que costuma cair:
- Nome atual: `double_exponential_smoothing` (antes `holt_winters`, renomeado no Prometheus 3).
- Assinatura: `(v range-vector, sf scalar, tf scalar)`, `0 < sf, tf < 1`.
- `sf` **menor** = mais peso aos dados **antigos** (mais liso); `tf` **maior** = considera mais a **tendência**.
- É **experimental** (feature flag `promql-experimental-functions`) e só para **gauges**.

**1.** Como se chamava `double_exponential_smoothing` no Prometheus 2?
- A) `smooth_over_time`
- B) `holt_winters`
- C) `exp_smoothing`
- D) `predict_holt`

<details><summary>Resposta</summary>

**B.** Foi renomeada porque Holt-Winters normalmente se refere à suavização **tripla** (com sazonalidade).
</details>

**2.** O que acontece ao **diminuir** o smoothing factor (`sf`)?
- A) Os dados recentes ganham mais importância
- B) Os dados antigos ganham mais importância e a curva fica mais lisa
- C) A tendência é ignorada
- D) A query passa a retornar por segundo

<details><summary>Resposta</summary>

**B.** Documentação: "The lower the smoothing factor sf, the more importance is given to old data."
</details>

**3.** Qual chamada é **inválida**?
- A) `double_exponential_smoothing(x[10m], 0.5, 0.5)`
- B) `double_exponential_smoothing(x[10m], 0.1, 0.9)`
- C) `double_exponential_smoothing(x[10m], 1.5, 0.3)`
- D) `double_exponential_smoothing(x[10m], 0.9, 0.1)`

<details><summary>Resposta</summary>

**C.** `sf` e `tf` precisam estar entre 0 e 1.
</details>

**4.** O que é necessário para usar `double_exponential_smoothing` no Prometheus 3?
- A) Nada, é uma função estável
- B) Habilitar `--enable-feature=promql-experimental-functions`
- C) Instalar um plugin
- D) Usar apenas em recording rules

<details><summary>Resposta</summary>

**B.** A função é experimental.
</details>

## 📝 Cola rápida

- `double_exponential_smoothing(gauge[janela], sf, tf)` = Holt linear (ex-`holt_winters`).
- `sf` ↓ = mais liso/atrasado; `sf` ↑ = mais nervoso. `tf` ↑ = segue tendência (e pode dar overshoot).
- `0 < sf, tf < 1`; **experimental** (`promql-experimental-functions`).
- Só **gauges**; counter → `rate` + subquery primeiro.
- Suavizar ≠ prever: previsão → `predict_linear`.

## 🔗 Relacionadas

[`predict_linear()`](../predict_linear/) · [`deriv()`](../deriv/) · [`avg_over_time()`](../avg_over_time/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#double_exponential_smoothing
