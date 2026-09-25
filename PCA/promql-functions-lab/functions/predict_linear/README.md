# `predict_linear()`: onde o gauge vai estar daqui a N segundos

> **Em uma frase:** `predict_linear(v[janela], t)` passa uma **reta** (regressão linear simples) pelos pontos da janela e diz **quanto o gauge valerá daqui a `t` segundos**. É a base do alerta mais famoso do Prometheus: "**o disco vai encher em menos de 4 horas**".

| | |
|---|---|
| **Assinatura** | `predict_linear(v range-vector, t scalar) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (só floats) · ❌ Counter · ❌ native histograms (ignorados) |
| **Unidade do resultado** | a **mesma do gauge** (bytes, %, mensagens...), valor previsto |
| **Dashboard** | http://localhost:3300/d/fn-predict_linear |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o computador de bordo do carro

O painel do carro mostra "**autonomia: 40 km**". Ele não sabe o futuro: só olha **como o combustível caiu nos últimos minutos** e estica essa linha até o zero.

```
 livre
   │•  •
   │    •• •
   │        • •  •               ← pontos da janela [5m]
   │              ╲ •
   │                ╲            ← a reta (mesma regressão do deriv)
   │                  ╲
 0 ┼───────────────────╳──────── ← cruza o zero = "vai encher"
   │   agora ──── t ────▶ previsão = valor da reta em (agora + t)
```

- A reta é a **mesma** do [`deriv()`](../deriv/): `previsão = valor_da_reta_agora + deriv × t`.
- `t` é em **segundos**: `4*3600` = 4 horas.
- Se a previsão é **< 0** para espaço livre, o disco vai encher antes de `t`.
- É **linear**: se o consumo acelera (crescimento exponencial), a previsão erra para o lado otimista.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário imita `node_filesystem_avail_bytes` do node_exporter:

| Métrica | Comportamento | Alerta "cheio em 4h"? |
|---|---|---|
| `predict_linear_node_filesystem_avail_bytes{mountpoint="/"}` | ~**40 GiB** parado, com **ruído de ±300 MiB** | não |
| `...{mountpoint="/var/log"}` | 20 GiB → perde **0.5 MiB/s** (logrotate a cada 3h). Enche em **~8 a 11 h** | não (mas o de 24h sim) |
| `...{mountpoint="/data"}` | 12 GiB → perde **1 MiB/s** (ciclo de 3h). Enche em **~0.4 a 3.4 h** | **sim** |
| `...{mountpoint="/scratch"}` | job batch: 20 GiB → perde **40 MiB/s**, zera em ~**8.5 min**, limpo a cada **10 min** | sim, enquanto enche |

```bash
curl -s localhost:8088/metrics | grep '^predict_linear_'
# predict_linear_node_filesystem_avail_bytes{mountpoint="/"} 4.2670751918e+10
# predict_linear_node_filesystem_avail_bytes{mountpoint="/data"} 5.835862745e+09
# predict_linear_node_filesystem_avail_bytes{mountpoint="/scratch"} 1.633744323e+10
# predict_linear_node_filesystem_avail_bytes{mountpoint="/var/log"} 1.7947682393e+10
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-predict_linear
```

Espere **~10 minutos** para ver um ciclo inteiro do `/scratch` e a janela `[10m]` do painel 6 cheia.

---

## 🔍 Queries passo a passo

### 1. Espaço livre cru

```promql
predict_linear_node_filesystem_avail_bytes
```

**Resultado esperado:** `/` reto em ~40 GiB (tremendo), `/var/log` e `/data` descendo devagar (quase retas em 15 min), e `/scratch` despencando de 20 GiB a 0 em ~8.5 min, ficando em 0 um pouco e voltando a 20 GiB (limpeza).

---

### 2. `/scratch`: agora × daqui a 5 minutos

```promql
predict_linear_node_filesystem_avail_bytes{mountpoint="/scratch"}
predict_linear(predict_linear_node_filesystem_avail_bytes{mountpoint="/scratch"}[1m], 300)
```

**O que faz:** a reta do último minuto é projetada **300s** à frente.
**Resultado esperado:** a linha "previsão" é a linha "agora" **deslocada 11.7 GiB para baixo** (40 MiB/s × 300s). Ela cruza o **zero** por volta de **3.5 min** de ciclo, enquanto o disco real só zera em **~8.5 min**: **5 minutos de aviso antecipado**. É exatamente o objetivo da função.

> ⚠️ Logo depois da limpeza (salto de 0 para 20 GiB) a reta da janela fica "de ponta-cabeça" e a previsão dá um salto enorme para cima. O eixo do painel foi cortado em −15/+25 GiB para não achatar o resto. Mudanças bruscas enganam a regressão.

---

### 3. O alerta clássico: "cheio em menos de 4h"

```promql
predict_linear(predict_linear_node_filesystem_avail_bytes[5m], 4*3600) < 0
```

**Resultado esperado (barras, só quem dispara):**

| mountpoint | previsão para daqui 4h |
|---|---|
| `/data` | ≈ **−2 a −12 GiB** (sempre aparece: 1 MiB/s × 14400s = 14 GiB > livre) |
| `/scratch` | muito negativo (≈ −500 GiB) enquanto o job está enchendo |
| `/`, `/var/log` | **não aparecem** (previsão positiva) |

---

### 4. Horas até encher

```promql
predict_linear_node_filesystem_avail_bytes{mountpoint!="/"}
  / -deriv(predict_linear_node_filesystem_avail_bytes{mountpoint!="/"}[5m]) / 3600
and deriv(predict_linear_node_filesystem_avail_bytes{mountpoint!="/"}[5m]) < 0
```

**O que faz:** "livre ÷ velocidade de consumo" = tempo até zerar. O `and deriv(...) < 0` descarta quem não está perdendo espaço (divisão por zero/negativa).
**Resultado esperado:** `/var/log` ≈ **8-11 h**, `/data` ≈ **0.4-3.4 h** (depende da hora do dia no ciclo de 3h), `/scratch` ≈ **0.0x h** (minutos) ou ausente logo após a limpeza.

---

### 5. O `t` define "com quanta antecedência": 4h × 24h

```promql
predict_linear(predict_linear_node_filesystem_avail_bytes{mountpoint="/var/log"}[5m], 4*3600)
predict_linear(predict_linear_node_filesystem_avail_bytes{mountpoint="/var/log"}[5m], 24*3600)
```

**Resultado esperado:** daqui a 4h ≈ **+8 a +13 GiB** (tranquilo: livre − 7 GiB); daqui a 24h ≈ **−22 a −27 GiB** (livre − 42 GiB → vai encher amanhã). É comum ter **dois alertas**: `warning` com horizonte de 24h (abre ticket) e `critical` com 4h (acorda alguém).

---

### 6. Pegadinha: janela curta + ruído

```promql
predict_linear(predict_linear_node_filesystem_avail_bytes{mountpoint="/"}[1m], 4*3600)
predict_linear(predict_linear_node_filesystem_avail_bytes{mountpoint="/"}[10m], 4*3600)
```

**Resultado esperado:** o disco `/` está **parado** em 40 GiB, só com ruído de ±300 MiB.
- `[1m]`: com só 12 pontos, a inclinação da reta varia muito, e multiplicada por 14400s vira uma previsão que oscila **dezenas de GiB** (desvio padrão ≈ 40 GiB), às vezes **negativa** → **alarme falso**.
- `[10m]`: 120 pontos seguram a reta; a previsão fica perto de **40 GiB** (±~1-2 GiB).

**Regra prática:** a janela deve ser **proporcional ao horizonte**. O clássico usa `[6h]` para prever `4h`; nunca preveja horas olhando só 1 minuto.

---

## 🏭 Casos reais

### 1. O alerta de disco do node_exporter (imitado pelos painéis 3-5)

É praticamente o `NodeFilesystemAlmostOutOfSpace`/`NodeFilesystemFillingUp` do kube-prometheus / node-mixin:

```yaml
groups:
- name: node-disk
  rules:
  - alert: NodeFilesystemFillingUpIn24h
    expr: |
      (
        node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes < 0.40
      and
        predict_linear(node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"}[6h], 24*3600) < 0
      and
        node_filesystem_readonly == 0
      )
    for: 1h
    labels: {severity: warning}
    annotations:
      summary: "{{ $labels.mountpoint }} em {{ $labels.instance }} deve encher nas próximas 24h"
  - alert: NodeFilesystemFillingUpIn4h
    expr: |
      (
        node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes < 0.20
      and
        predict_linear(node_filesystem_avail_bytes{fstype!~"tmpfs|overlay"}[6h], 4*3600) < 0
      and
        node_filesystem_readonly == 0
      )
    for: 1h
    labels: {severity: critical}
```

**Decisões:** (a) janela de **6h** para uma previsão de 4h/24h, para o ruído não gerar alarmes falsos (painel 6); (b) o `and ... < 0.40` evita alertar um disco de 2 TB com 90% livre só porque ele está "descendo"; (c) `for: 1h` absorve picos de escrita.

### 2. Certificado / quota / licença acabando

Qualquer gauge que "gasta" linearmente serve: quota de API (`api_quota_remaining`), créditos de burst de EBS, conexões disponíveis:

```promql
predict_linear(aws_ebs_burst_balance_average[1h], 2*3600) < 10
```

### 3. Memória chegando no limite do container

```yaml
- alert: ContainerVaiLevarOOM
  expr: |
    predict_linear(container_memory_working_set_bytes{container!=""}[30m], 3600)
      > on(namespace, pod, container) kube_pod_container_resource_limits{resource="memory"}
  for: 15m
```

"Se continuar nesse ritmo, em 1h passa do limit" → OOMKill evitado com um restart planejado ou aumento de limit.

### 4. Job batch enchendo disco temporário (imitado pelo painel 2)

Para processos rápidos, janela e horizonte encolhem juntos: `predict_linear(node_filesystem_avail_bytes{mountpoint="/scratch"}[5m], 15*60) < 0` → "em 15 min o /scratch acaba, mate o job".

---

## ✅ Quando usar

- **Alertas preditivos** de esgotamento: disco, inodes (`node_filesystem_files_free`), memória, quotas.
- **Capacity planning:** "quando vamos precisar de mais disco?".
- Qualquer gauge com **tendência aproximadamente linear** no horizonte que interessa.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Métrica é **counter** | `predict_linear` sobre `rate()` via subquery, ou pense de novo: counters crescem sempre |
| Só quer a **velocidade** atual | [`deriv()`](../deriv/) |
| Série com **sazonalidade** (dia/noite) | janela ≥ 1 ciclo, ou compare com `offset 1w` |
| Crescimento **exponencial** | a reta subestima; use limites absolutos também |
| Quer suavizar ruído mantendo tendência (não prever) | [`double_exponential_smoothing()`](../double_exponential_smoothing/) |

## ⚠️ Pegadinhas

1. **Janela curta** para horizonte longo = alarmes falsos (painel 6).
2. **Saltos** (limpeza de disco, logrotate, restart) entortam a reta por um tempo igual à janela (painel 2).
3. **`t` em segundos:** `predict_linear(x[1h], 4)` prevê 4 **segundos**, não 4 horas. Use `4*3600` ou `4 * 60 * 60`.
4. **Só floats:** native histograms são ignorados; `+Inf`/`-Inf` na janela → `NaN`.
5. **Série nova** com menos de 2 amostras → sem resultado (disco recém-montado não alerta).

## 🎓 Na prova PCA

O que costuma cair:
- Assinatura: `predict_linear(v range-vector, t scalar)`; `t` em **segundos**; retorna **instant vector** com o valor previsto.
- Usa **regressão linear simples** (a mesma do `deriv`); só **gauges**.
- O exemplo clássico: `predict_linear(node_filesystem_avail_bytes[6h], 4*3600) < 0`.
- Diferença para `deriv`: `deriv` = inclinação (por segundo); `predict_linear` = valor futuro.

**1.** Qual expressão alerta se o disco vai encher nas próximas 4 horas, com base nas últimas 6 horas?
- A) `predict_linear(node_filesystem_avail_bytes[4h], 6*3600) < 0`
- B) `predict_linear(node_filesystem_avail_bytes[6h], 4*3600) < 0`
- C) `deriv(node_filesystem_avail_bytes[6h]) * 4 < 0`
- D) `predict_linear(node_filesystem_avail_bytes[6h], 4) < 0`

<details><summary>Resposta</summary>

**B.** A janela `[6h]` é o histórico usado na regressão; o segundo argumento é o horizonte em segundos (4 × 3600). A troca os papéis, C só multiplica a inclinação por 4 segundos, D prevê 4 segundos.
</details>

**2.** Qual método matemático o `predict_linear` usa?
- A) Média móvel exponencial
- B) Regressão linear simples (mínimos quadrados)
- C) Holt-Winters
- D) Interpolação entre a primeira e a última amostra

<details><summary>Resposta</summary>

**B.** Suavização exponencial dupla é o `double_exponential_smoothing`; primeira/última amostra é o `delta`.
</details>

**3.** Qual é o tipo do segundo argumento de `predict_linear`?
- A) Duração (`4h`)
- B) Range vector
- C) Scalar (segundos)
- D) String

<details><summary>Resposta</summary>

**C.** `t scalar`. `predict_linear(x[1h], 4h)` é inválido; escreva `4*3600` (ou `14400`).
</details>

**4.** Por que alertas reais combinam `predict_linear(...) < 0` com `avail/size < 0.4`?
- A) Porque `predict_linear` só funciona com percentuais
- B) Para evitar alertar discos grandes e quase vazios só porque tiveram uma tendência de queda momentânea
- C) Porque `predict_linear` não aceita labels
- D) Para converter bytes em porcentagem

<details><summary>Resposta</summary>

**B.** A condição extra garante que só discos já razoavelmente cheios alertem, reduzindo falsos positivos.
</details>

## 📝 Cola rápida

- `predict_linear(gauge[janela], t_segundos)` = valor da **reta de regressão** daqui a `t` s.
- Clássico: `predict_linear(node_filesystem_avail_bytes[6h], 4*3600) < 0`.
- Janela proporcional ao horizonte (6h → 4h); janela curta + ruído = falso positivo.
- Mesma reta do `deriv`; só **gauges** e floats.
- `t` é **scalar em segundos** (não `4h`).

## 🔗 Relacionadas

[`deriv()`](../deriv/) · [`delta()`](../delta/) · [`double_exponential_smoothing()`](../double_exponential_smoothing/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#predict_linear
