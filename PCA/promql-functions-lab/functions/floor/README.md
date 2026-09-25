# `floor()`: arredondar pra baixo (quantos estão completos?)

> **Em uma frase:** `floor(v)` arredonda cada valor para o **maior inteiro menor ou igual** a ele: `9.99 → 9`, `2.98 → 2`, `-1.5 → -2`. É a função de "só conta o que está **completo**".

| | |
|---|---|
| **Assinatura** | `floor(v instant-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge e resultados de expressões (divisões, `time() - x`) · histogramas são ignorados |
| **Unidade do resultado** | a mesma da entrada, agora **inteira** (lotes, caixas, minutos, dias...) |
| **Dashboard** | http://localhost:3300/d/fn-floor |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a esteira de embalagem

Uma fábrica coloca peças em caixas de **12**. Com **99 peças** na esteira:

```
 99 / 12 = 8.25
            │
            ├── floor(8.25) = 8   → 8 caixas CHEIAS prontas para despachar ✅
            ├── 99 % 12     = 3   → 3 peças avulsas esperando a próxima caixa
            └── ceil(8.25)  = 9   → 9 caixas que você precisa ter no estoque
```

A caixa com 3 peças **não pode** ser despachada: ela não está completa. O `floor()` é o **chão**: de qualquer ponto entre o 8º e o 9º andar, você cai para o **8º**.

`floor` e o operador `%` (módulo) andam juntos: **quociente inteiro** e **resto**. No lab, a "esteira" é uma fila do RabbitMQ e a "caixa" é um lote de 100 mensagens.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Imita | Tipo | Comportamento |
|---|---|---|---|
| `floor_rabbitmq_queue_messages_ready{queue="invoices"}` | `rabbitmq_queue_messages_ready` | gauge | mensagens **inteiras** de **0 a 999** em 3 min, depois zera (fila drenada) |
| `floor_process_start_time_seconds{service="api"}` | `process_start_time_seconds` | gauge | timestamp Unix do start; o processo "reinicia" a cada **15 min** |

Premissa: o consumidor da fila `invoices` só processa **lotes cheios de 100** mensagens.

```bash
curl -s localhost:8088/metrics | grep '^floor_'
# floor_rabbitmq_queue_messages_ready{queue="invoices"} 572
# floor_process_start_time_seconds{service="api"} 1.7903736e+09
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-floor
```

Espere **~3 minutos** para ver um ciclo inteiro da fila.

---

## 🔍 Queries passo a passo

### 1. As mensagens cruas

```promql
floor_rabbitmq_queue_messages_ready
```

**Resultado esperado:** uma rampa de **0 a 999** que despenca para 0 a cada 3 min.

---

### 2. Lotes cheios e sobra

```promql
floor_rabbitmq_queue_messages_ready / 100                 # razão crua
floor(floor_rabbitmq_queue_messages_ready / 100)          # lotes cheios
(floor_rabbitmq_queue_messages_ready % 100) / 100         # fração do próximo lote
```

**O que faz:** o `floor` descarta a parte fracionária, então sobram só os lotes **completos**. O `%` devolve o resto (dividido por 100 só para caber na mesma escala do gráfico).
**Resultado esperado:**

| msgs | `/ 100` | `floor` (lotes cheios) | `% 100` (sobra) |
|---|---|---|---|
| 99 | 0.99 | **0** | 99 |
| 100 | 1.00 | **1** | 0 |
| 572 | 5.72 | **5** | 72 |
| 999 | 9.99 | **9** | 99 |

No gráfico, o `floor` é uma **escada** de 0 a 9 que fica **sempre abaixo (ou encostada)** da linha crua, e a sobra é um **dente-de-serra** de 0 a 0.99 que zera a cada lote completado.

> 💡 Verificação: `100 × floor(x/100) + x % 100 = x`, sempre (para x ≥ 0).

---

### 3 e 4. Minutos completos de uptime

```promql
time() - floor_process_start_time_seconds                     # uptime em segundos
floor((time() - floor_process_start_time_seconds) / 60)       # minutos COMPLETOS
round((time() - floor_process_start_time_seconds) / 60)       # ❌ para comparação
```

**O que faz:** `time()` é o "agora" da avaliação; menos o horário de start dá o uptime. Dividindo por 60 e aplicando `floor`, ficam só os minutos **inteiros**.
**Resultado esperado:** o uptime é uma rampa de **0 a 900 s**; os minutos crus vão de **0 a 15** e a do `floor` é uma escada **0, 1, 2 ... 14**. Com `uptime = 179 s`: cru = 2.98, `floor` = **2** (o processo ainda não completou 3 minutos). A linha do `round` (painel 4) diria **3**, o que é mentira: ela sobe sempre **30 s antes** de o minuto completar (em 150 s ela já mostra 3).

É o mesmo raciocínio de "idade": uma pessoa com 29 anos e 11 meses **tem 29 anos**. Idade é sempre `floor`.

---

### 5. Casos especiais

| Query | Resultado | Por quê |
|---|---|---|
| `floor(vector(1.49))` | **1** | exemplo da doc |
| `floor(vector(1.78))` | **1** | exemplo da doc |
| `floor(vector(-1.5))` | **-2** ⚠️ | "pra baixo" é em direção ao **-∞**, não "em direção ao zero" |
| `sgn(vector(-1.5)) * floor(abs(vector(-1.5)))` | **-1** | truque para **truncar** (cortar as casas decimais) também nos negativos |
| `floor(vector(-0))` | **-0** | `floor(±0) = ±0` |
| `floor(vector(+Inf))` | **+Inf** | infinito continua infinito |
| `floor((vector(0.7) + 0.1) * 10)` | **7** ⚠️ | `0.7 + 0.1 = 0.7999999999999999` em ponto flutuante, `×10 = 7.999...` e o `floor` desce para 7 |

---

## 🏭 Casos reais

### 1. "Dias de uptime" no dashboard de frota

```promql
floor((time() - process_start_time_seconds{job="api"}) / 86400)
```

O painel "Uptime (dias)" do time de plataforma mostra dias **completos** desde o último restart. Com `floor`, um processo que subiu há 6.9 dias aparece como **6**, e não 7. Útil para achar processos que **não reiniciam há muito tempo** (vazamento de memória, certificados carregados em memória):

```yaml
- alert: ProcessoSemRestartHaMuitoTempo
  expr: floor((time() - process_start_time_seconds{job="api"}) / 86400) >= 30
  labels:
    severity: info
  annotations:
    summary: "{{ $labels.instance }} está no ar há {{ $value }} dias sem restart"
```

### 2. Lotes prontos numa fila (RabbitMQ / Kafka)

```promql
floor(rabbitmq_queue_messages_ready{queue="invoices"} / 100)
```

Um consumidor que só processa **lotes cheios** (ex.: gerar NF-e em lotes de 100) fica "parado" enquanto a fila tem 99 mensagens. O dashboard mostra lotes prontos (`floor`) e sobra (`% 100`) lado a lado; um alerta de "sobra parada há muito tempo" pega o caso em que o timeout de flush do lote não está funcionando:

```yaml
- record: queue:full_batches:floor
  expr: floor(rabbitmq_queue_messages_ready{queue="invoices"} / 100)

- alert: LoteIncompletoParado
  expr: |
    queue:full_batches:floor{queue="invoices"} == 0
      and rabbitmq_queue_messages_ready{queue="invoices"} > 0
  for: 15m
  labels:
    severity: warning
  annotations:
    summary: "{{ $labels.queue }}: mensagens esperando há 15 min sem completar um lote"
```

### 3. Faixas de tempo para agrupar (bucketing para baixo)

```promql
count_values("dias_desde_deploy", floor((time() - kube_deployment_created) / 86400))
```

Quantos deployments têm 0, 1, 2 ... dias. `floor` garante faixas `[n, n+1)`, onde "1 dia" quer dizer "entre 24 h e 48 h".

### 4. Idade de certificados em dias inteiros

```promql
floor((probe_ssl_earliest_cert_expiry - time()) / 86400)
```

O blackbox_exporter expõe a expiração do certificado. `floor` dá "faltam **N dias completos**", que é como o time de segurança costuma falar (com 6.9 dias restantes, o painel mostra **6**, a favor da segurança).

---

## ✅ Quando usar

- **Unidades completas:** lotes/batches prontos (`floor(queue_messages / batch_size)`), caixas cheias, páginas completas.
- **Tempo "completo":** minutos, horas ou dias inteiros de uptime/idade.
- **Bucketing manual para baixo:** agrupar valores em faixas `[0,10)`, `[10,20)`... com `floor(x / 10) * 10` (sempre o **início** da faixa).
- **Quociente inteiro** junto com `%` para decompor um número (ex.: segundos → horas + minutos).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer saber **quantas unidades precisa** (pods, nós, GB cobrados) | [`ceil()`](../ceil/) |
| Quer o inteiro **mais próximo** ou múltiplos arbitrários (0.5, 25...) | [`round()`](../round/) |
| Quer só **exibir** menos casas decimais | a opção **Decimals** do painel no Grafana |
| Quer extrair hora/dia/mês de um timestamp | [`hour()`](../hour/), [`day_of_month()`](../day_of_month/)... |
| Quer impedir que o valor passe de um limite | [`clamp_max()`](../clamp_max/) |

## ⚠️ Pegadinhas

1. **Negativos:** `floor(-1.5) = -2`, não `-1`. `floor` **não é truncar**. Para truncar: `sgn(x) * floor(abs(x))`.
2. **Ponto flutuante:** `7.999999999999999` vira **7**. Se o valor "deveria" ser inteiro, some uma tolerância: `floor(x + 1e-9)`.
3. **Ordem das operações:** `floor(x) / 100` ≠ `floor(x / 100)`. Divida primeiro, arredonde depois.
4. **Agregação:** `sum(floor(x / 100))` (lotes cheios **por fila**) ≠ `floor(sum(x) / 100)` (juntando todas as mensagens). Escolha o que faz sentido fisicamente.
5. **Nome da métrica some** e **histogramas nativos são ignorados**, como em toda função matemática.

## 🎓 Na prova PCA

O que costuma cair:
- `floor()` recebe **instant vector**; `floor(2.5)` com escalar literal é **erro de parse**.
- Direção: `floor` vai para **-∞** (`floor(-1.5) = -2`), `ceil` vai para **+∞** (`ceil(-1.5) = -1`).
- Padrão clássico de uptime: `time() - process_start_time_seconds` (e não `rate()`, porque é um gauge de timestamp).

**1.** Quanto vale `floor(vector(-3.2))`?

- A) -3
- B) -4
- C) 3
- D) -3.2

<details><summary>Resposta</summary>

**B.** O maior inteiro **menor ou igual** a -3.2 é -4. `floor` não "corta as casas decimais" em negativos.
</details>

**2.** Qual expressão mostra quantos **dias completos** o processo está no ar?

- A) `floor(process_start_time_seconds / 86400)`
- B) `floor((time() - process_start_time_seconds) / 86400)`
- C) `ceil((time() - process_start_time_seconds) / 86400)`
- D) `floor(rate(process_start_time_seconds[1d]))`

<details><summary>Resposta</summary>

**B.** A) dá dias desde 1970. C) arredonda pra cima (6.1 dias viraria 7). D) `rate()` num gauge de timestamp não faz sentido (daria ~0).
</details>

**3.** Uma fila tem 250 mensagens e o consumidor processa lotes de 100. Quais valores retornam `floor(x / 100)` e `x % 100`?

- A) 3 e 50
- B) 2 e 50
- C) 2.5 e 0
- D) 2 e 0.5

<details><summary>Resposta</summary>

**B.** 2 lotes cheios e 50 mensagens de sobra. `%` é o operador de módulo do PromQL e devolve o resto na mesma unidade da entrada.
</details>

**4.** Como **truncar** `x` (remover casas decimais em direção ao zero) em PromQL, funcionando para positivos e negativos?

- A) `floor(x)`
- B) `round(x)`
- C) `sgn(x) * floor(abs(x))`
- D) `ceil(x) - 1`

<details><summary>Resposta</summary>

**C.** Tira o sinal, arredonda o módulo pra baixo e devolve o sinal. `floor(-1.5)` daria -2; a opção C dá -1.
</details>

## 📝 Cola rápida

- `floor(v instant-vector)` → maior inteiro **≤** valor (direção -∞). `floor(-1.5) = -2`, `floor(+Inf) = +Inf`.
- "Quantos **completos**?" (lotes, dias de uptime) → `floor`. "Quantos **preciso**?" → `ceil`.
- Uptime: `time() - process_start_time_seconds`; dias completos: `floor(... / 86400)`.
- `floor(x / n)` = quociente inteiro, `x % n` = resto.
- Truncar: `sgn(x) * floor(abs(x))`.

## 🔗 Relacionadas

[`ceil()`](../ceil/) · [`round()`](../round/) · [`sgn()`](../sgn/) · [`abs()`](../abs/) · [`time()`](../time/) · [`hour()`](../hour/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#floor
