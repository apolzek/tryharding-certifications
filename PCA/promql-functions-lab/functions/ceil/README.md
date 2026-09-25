# `ceil()`: arredondar pra cima (quantos eu preciso?)

> **Em uma frase:** `ceil(v)` arredonda cada valor para o **menor inteiro maior ou igual** a ele: `2.17 → 3`, `1.01 → 2`, `2 → 2`. É a função de "se passou um pouquinho, já precisa de mais uma unidade".

| | |
|---|---|
| **Assinatura** | `ceil(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (razões, `rate()/capacidade`) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada, mas agora **inteira** (pods, GB, horas...) |
| **Dashboard** | http://localhost:3300/d/fn-ceil |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: as vans da excursão

130 pessoas vão numa excursão e cada van leva 60. Quantas vans?

```
 130 / 60 = 2.17 vans
             │
             ├── round(2.17) = 2  → 10 pessoas ficam na calçada 😬
             ├── floor(2.17) = 2  → idem
             └── ceil(2.17)  = 3  → todo mundo vai ✅
```

Ninguém aluga **0.17 van**. Quando a unidade é indivisível (pod, nó, van, GB faturado, hora cobrada), a pergunta "quantos eu preciso?" sempre se responde com `ceil()`.

Outra imagem: `ceil` é um **elevador que só sobe**. De qualquer ponto entre o 2º e o 3º andar, ele leva você ao **3º**. Se você já está exatamente no 2º, fica no 2º.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `ceil_http_requests_total{service="checkout"}` | `http_requests_total` | counter | cresce a **~30 até ~450 req/s** (a velocidade é uma onda de 4 min) |
| `ceil_kube_deployment_spec_replicas{deployment="checkout"}` | `kube_deployment_spec_replicas` (kube-state-metrics) | gauge | fixo em **5** réplicas |
| `ceil_storage_used_bytes{bucket="backups"}` | uso de um bucket de objetos | gauge | cresce de **0.3 GB a 5.1 GB** em 5 min e recomeça |

A premissa do cenário: um **teste de carga** mostrou que **1 pod do checkout aguenta 60 req/s**.

```bash
curl -s localhost:8088/metrics | grep '^ceil_'
# ceil_http_requests_total{service="checkout"} 1.2034e+06
# ceil_kube_deployment_spec_replicas{deployment="checkout"} 5
# ceil_storage_used_bytes{bucket="backups"} 2.46e+09
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-ceil
```

Espere **~4 minutos** para ver uma volta inteira da onda de carga e **~5 min** para a escada de GB completa.

---

## 🔍 Queries passo a passo

### 1. A carga e a capacidade de um pod

```promql
sum by (service) (rate(ceil_http_requests_total[1m]))
vector(60)
```

**Resultado esperado:** uma onda entre **~50 e ~430 req/s** (o gerador vai de 30 a 450, mas a janela `[1m]` do `rate` suaviza os extremos) e uma linha reta em **60** (a capacidade de um pod).

---

### 2. Quantos pods eu preciso?

```promql
sum by (service) (rate(ceil_http_requests_total[1m])) / 60           # razão crua
ceil(sum by (service) (rate(ceil_http_requests_total[1m])) / 60)     # pods necessários
round(sum by (service) (rate(ceil_http_requests_total[1m])) / 60)    # ❌ para comparação
```

**O que faz:** `rate()` transforma o counter em req/s, a divisão por 60 dá "quantos pods de 60 req/s cabem nessa carga" e o `ceil` transforma isso em um número inteiro **seguro**.
**Resultado esperado** (exemplos):

| carga (req/s) | razão | `ceil` | `round` |
|---|---|---|---|
| 30 | 0.50 | **1** | 1 (empate sobe) |
| 130 | 2.17 | **3** | 2 ⚠️ falta 1 pod |
| 180 | 3.00 | **3** | 3 |
| 450 | 7.50 | **8** | 8 |

No gráfico, a linha do `ceil` é uma **escada** que fica **sempre em cima ou encostada** na razão crua. A do `round` às vezes fica **abaixo**, e esses são exatamente os momentos em que o serviço estaria sobrecarregado.

> 💡 É a mesma conta que o **HPA do Kubernetes** faz: `desiredReplicas = ceil(currentReplicas × currentMetricValue / desiredMetricValue)`.

---

### 3 e 4. Necessário × configurado, e o alerta

```promql
ceil(sum(rate(ceil_http_requests_total[1m])) / 60)                    # necessários
sum(ceil_kube_deployment_spec_replicas)                               # configurados (5)

ceil(sum(rate(ceil_http_requests_total[1m])) / 60)
  > scalar(sum(ceil_kube_deployment_spec_replicas))                   # alerta
```

**O que faz:** compara a escada de pods necessários com as 5 réplicas do deployment. O `scalar()` transforma o lado direito num número para a comparação não depender de labels (os dois lados têm labels diferentes: `service` × `deployment`).
**Resultado esperado:**
- Painel 3: a escada sobe de **1 até 8** e cruza a linha reta em **5** quando a carga passa de **300 req/s**.
- Painel 4: **pontos só aparecem** nos momentos em que o alerta estaria ativo (valores **6, 7 ou 8**), por ~1 min a cada 4 min. Um filtro de comparação sem `bool` **remove** as séries que não passam.

---

### 5 e 6. GB cobrados "por GB iniciado"

```promql
ceil_storage_used_bytes / 1e9          # GB usados, com casas decimais
ceil(ceil_storage_used_bytes / 1e9)    # GB cobrados
```

**Resultado esperado:** a linha crua sobe em rampa de **0.3 a 5.1**; a linha do `ceil` é uma **escadinha 1 → 2 → 3 → 4 → 5 → 6**. Repare que `1.01 GB` já cobra **2 GB**, e que no último degrau (5.01..5.1 GB) você paga **6 GB**.

> ⚠️ **Divida antes, arredonde depois.** `ceil(ceil_storage_used_bytes) / 1e9` não faz nada útil: os bytes já são inteiros.

---

### 7. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `ceil(vector(1.49))` | **2** | exemplo da doc |
| `ceil(vector(1.78))` | **2** | exemplo da doc |
| `ceil(vector(2))` | **2** | inteiro não muda |
| `ceil(vector(-1.5))` | **-1** | "pra cima" é em direção ao **+∞**, não "pra longe do zero" |
| `ceil(vector(-0.5))` | **-0** | `ceil(±0) = ±0`; o resultado de -0.5 é o zero negativo (a API devolve `"-0"`; o Grafana exibe `0`) |
| `ceil(vector(+Inf))` | **+Inf** | infinito continua infinito |
| `ceil((vector(0.1) + 0.2) * 10)` | **4** ⚠️ | `0.1 + 0.2 = 0.30000000000000004` em ponto flutuante, então `×10 = 3.0000000000000004` e o `ceil` sobe para 4 |

---

## 🏭 Casos reais

### 1. "Réplicas insuficientes" antes da Black Friday

O SRE do e-commerce sabe, pelo teste de carga, que um pod do checkout aguenta 60 req/s. Ele cria uma recording rule com o número de pods necessário e um alerta que compara com o deployment:

```yaml
groups:
  - name: capacity
    rules:
      - record: service:pods_needed:ceil
        expr: ceil(sum by (service) (rate(http_requests_total{service="checkout"}[5m])) / 60)

      - alert: CheckoutReplicasInsuficientes
        expr: |
          max(service:pods_needed:ceil{service="checkout"})
            > on() max(kube_deployment_spec_replicas{deployment="checkout"})
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Checkout precisa de {{ $value }} pods, mas o deployment tem menos"
```

**Decisão:** `ceil` e não `round`, porque 2.17 pods precisam de **3** pods. `max(...) > on() max(...)` (ou `scalar()`) porque as duas séries têm labels diferentes.

### 2. Quantos nós o cluster precisa (memória)

```promql
ceil(
  sum(kube_pod_container_resource_requests{resource="memory"})
  / on() group_left max(kube_node_status_allocatable{resource="memory"})
)
```

```yaml
- record: cluster:nodes_needed_memory:ceil
  expr: |
    ceil(
      sum(kube_pod_container_resource_requests{resource="memory"})
      / scalar(max(kube_node_status_allocatable{resource="memory"}))
    )
- alert: ClusterSemFolgaDeMemoria
  expr: cluster:nodes_needed_memory:ceil >= scalar(count(kube_node_info))
  for: 30m
  labels:
    severity: warning
```

Soma de memória **pedida** por todos os pods ÷ memória alocável de **um** nó = nós necessários. `ceil` pelo mesmo motivo: 4.1 nós = **5** nós. Compare com `count(kube_node_info)` para ver a folga.

### 3. Custo: horas de máquina e GB faturados

```promql
ceil(sum(node_filesystem_size_bytes{mountpoint="/data"} - node_filesystem_avail_bytes{mountpoint="/data"}) / 2^30)   # GiB faturados
ceil((time() - process_start_time_seconds{job="batch"}) / 3600)                                                         # horas cobradas de um job
```

Provedores cobram "por unidade iniciada". O dashboard de FinOps usa `ceil` para mostrar o número que vai para a fatura.

### 4. Shards/partições necessárias

```promql
ceil(sum(rate(kafka_server_brokertopicmetrics_bytesin_total{topic="events"}[5m])) / (10 * 2^20))
```

Se cada partição aguenta 10 MiB/s, o `ceil` diz quantas partições o tópico precisa.

---

## ✅ Quando usar

- **Capacity planning:** réplicas, nós, shards necessários: `ceil(sum(rate(requests_total[5m])) / 60)`.
- **Faturamento por unidade iniciada:** GB, horas de máquina, blocos de 1000 requisições.
- **Número de lotes/páginas:** "quantas páginas de 50 itens preciso para mostrar N itens?" → `ceil(N / 50)`.
- **Recording rules de "mínimo necessário"** que alimentam alertas.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer contar só as unidades **completas** (caixas cheias, lotes prontos) | [`floor()`](../floor/) |
| Quer o inteiro **mais próximo** ou arredondar para múltiplos (0.5, 10...) | [`round()`](../round/) |
| Quer só **exibir** menos casas decimais no Grafana | a opção **Decimals** do painel (não mude o dado) |
| Quer limitar o valor a uma faixa | [`clamp()`](../clamp/) |
| Quer **prever** quando vai faltar capacidade | [`predict_linear()`](../predict_linear/) |

## ⚠️ Pegadinhas

1. **Ponto flutuante:** valores que "deveriam" ser inteiros podem ter lixo na 16ª casa (`3.0000000000000004`) e o `ceil` sobe um degrau inteiro. Se necessário, subtraia uma tolerância: `ceil(x - 1e-9)`.
2. **Negativos:** `ceil(-1.5) = -1` (em direção ao +∞). Para "arredondar pra longe do zero", use `sgn(x) * ceil(abs(x))`.
3. **Zero negativo:** `ceil(-0.5)` devolve `-0`. Numericamente é igual a `0`, mas pode aparecer como `-0` em tabelas.
4. **`ceil` antes ou depois de agregar:** `sum(ceil(x))` (arredonda **cada** série, depois soma) ≥ `ceil(sum(x))`. Para capacity planning por serviço, arredonde por serviço (cada um tem seus pods).
5. **Não aplique em counter cru:** `ceil(http_requests_total)` não tem sentido; primeiro `rate()`, depois divida, depois `ceil()`.
6. **Nome da métrica some** e **histogramas nativos são ignorados**, como em toda função matemática.

## 🎓 Na prova PCA

O que costuma cair:
- `ceil()` recebe **instant vector** (não escalar, não range vector). `ceil(1.2)` é **erro de parse**; `ceil(rate(x[5m]))` é válido (o `rate` já devolve instant vector).
- Diferenciar `ceil` (pra cima, +∞), `floor` (pra baixo, -∞) e `round` (mais próximo, empate pra cima), **inclusive nos negativos**.
- A ordem certa em capacity planning: `rate` → `sum` → dividir → `ceil`.

**1.** Uma API recebe 250 req/s e cada pod aguenta 100 req/s. Qual query retorna o número mínimo de pods **sem subdimensionar**?

- A) `round(sum(rate(http_requests_total[5m])) / 100)`
- B) `floor(sum(rate(http_requests_total[5m])) / 100)`
- C) `ceil(sum(rate(http_requests_total[5m])) / 100)`
- D) `ceil(sum(http_requests_total) / 100)`

<details><summary>Resposta</summary>

**C** → `ceil(2.5) = 3`. A) `round(2.5) = 3` aqui por sorte (empate sobe), mas com 240 req/s daria 2 (falta pod). B) dá 2. D) usa o counter cru (total acumulado desde o start), não a taxa.
</details>

**2.** Quanto vale `ceil(vector(-2.7))`?

- A) -3
- B) -2
- C) 3
- D) -2.7

<details><summary>Resposta</summary>

**B.** `ceil` arredonda em direção ao **+∞**: o menor inteiro ≥ -2.7 é -2.
</details>

**3.** Qual das expressões abaixo é **inválida**?

- A) `ceil(node_load1)`
- B) `ceil(rate(node_cpu_seconds_total[5m]))`
- C) `ceil(node_memory_MemAvailable_bytes[5m])`
- D) `ceil(vector(3.2))`

<details><summary>Resposta</summary>

**C.** `node_memory_MemAvailable_bytes[5m]` é um **range vector** e `ceil` só aceita instant vector. Para usar uma janela, aplique antes uma função `_over_time` (ex.: `ceil(max_over_time(x[5m]))`).
</details>

**4.** `ceil((vector(0.1) + 0.2) * 10)` retorna 4. Por quê?

- A) Bug do Prometheus
- B) Em ponto flutuante IEEE 754, `0.1 + 0.2 = 0.30000000000000004`, e `ceil` sobe qualquer excesso
- C) `ceil` sempre soma 1
- D) `vector()` arredonda os valores

<details><summary>Resposta</summary>

**B.** O PromQL usa float64. O valor resultante é `3.0000000000000004`, e o menor inteiro ≥ a ele é 4.
</details>

## 📝 Cola rápida

- `ceil(v instant-vector)` → menor inteiro **≥** valor (direção +∞). `ceil(-1.5) = -1`, `ceil(+Inf) = +Inf`, `ceil(-0.5) = -0`.
- "Quantos eu **preciso**?" → `ceil`. "Quantos **completos**?" → `floor`. "Mais próximo?" → `round`.
- Capacity planning: `ceil(sum(rate(x[5m])) / capacidade_por_pod)`.
- Escalar literal não vale: `ceil(1.2)` é erro; `ceil(vector(1.2))` = 2.
- Cuidado com ponto flutuante logo acima de um inteiro.

## 🔗 Relacionadas

[`floor()`](../floor/) · [`round()`](../round/) · [`clamp()`](../clamp/) · [`predict_linear()`](../predict_linear/) · [`rate()`](../rate/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#ceil
