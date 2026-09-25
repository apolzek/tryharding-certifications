# `min_over_time()`: o vale de cada série dentro da janela

> **Em uma frase:** `min_over_time(v[janela])` devolve, **para cada série**, o **menor valor** entre as amostras da janela. É a forma de enxergar "quase acabou" (conexões livres, disco livre, réplicas prontas) mesmo quando o gráfico "de longe" parece tranquilo.

| | |
|---|---|
| **Assinatura** | `min_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge (recursos **livres**, 0/1 de saúde, réplicas disponíveis) · ✅ resultado de `rate()` via subquery · ❌ Counter cru · ❌ histogram (ignorado) |
| **Unidade do resultado** | a **mesma** da entrada |
| **Dashboard** | http://localhost:3300/d/fn-min_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: a marca d'água da caixa d'água

A caixa d'água do prédio está **cheia agora**. Mas se você olhar a parede de dentro, vê uma **marca de sujeira bem lá embaixo**: em algum momento do dia ela **quase secou**. Essa marca é o `min_over_time`.

- O gauge `db_pool_available_connections` é o **nível agora**.
- `min_over_time(db_pool_available_connections[5m])` é a **marca mais baixa** dos últimos 5 min.

O `min_over_time` é o irmão espelhado do [`max_over_time`](../max_over_time/): use `max` para coisas que **não podem subir demais** (memória, latência, fila) e `min` para coisas que **não podem descer demais** (conexões livres, disco livre, pods prontos, saldo, bateria).

E, como em toda função `*_over_time` (veja a planilha em [`avg_over_time`](../avg_over_time/)):

```
 min_over_time(x[5m])   →  no TEMPO, por série     ("qual foi o pior momento do pool main?")
 min(x)                 ↓  ENTRE séries, agora    ("qual pool tem menos conexões livres agora?")
```

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `min_over_time_hikaricp_connections_idle{pool="main"}` | gauge | ~**40** livres (±3), mas **cai para 1–2 durante 15s** (3 scrapes) **a cada 3 min**, nos segundos 80..95 do ciclo |
| `min_over_time_hikaricp_connections_idle{pool="replica"}` | gauge | ~**20** livres (±2), estável |
| `min_over_time_probe_success{service="db"}` | gauge 0/1 | sempre **1** |
| `min_over_time_probe_success{service="api"}` | gauge 0/1 | **0 durante 10s** (2 scrapes) **a cada 2 min** |

O tamanho máximo do pool é **50** conexões.

```bash
curl -s localhost:8088/metrics | grep '^min_over_time_'
# min_over_time_hikaricp_connections_idle{pool="main"} 41
# min_over_time_probe_success{service="api"} 1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-min_over_time
```

Espere **~5 min** para a janela `[5m]` conter pelo menos uma queda.

---

## 🔍 Queries passo a passo

### 1. Conexões livres cruas

```promql
min_over_time_hikaricp_connections_idle
```

**Resultado esperado:** `main` oscilando entre 37 e 43, com **quedas em V** até 1–2 a cada 3 min; `replica` entre 18 e 22.

---

### 2. O mesmo dado com resolução de 1 minuto

```promql
last_over_time(min_over_time_hikaricp_connections_idle[1m:1m])
```

**O que faz:** a subquery `[1m:1m]` avalia a métrica só nas **viradas de minuto** (como faria um gráfico de 24h, que tem ~1 ponto por minuto ou menos).
**Resultado esperado:** degraus entre 37 e 43 para `main`, **sem nenhuma queda**. Olhando só isso, ninguém investigaria o pool.

---

### 3. `min_over_time`: a marca d'água

```promql
min_over_time(min_over_time_hikaricp_connections_idle[5m])
```

**Resultado esperado:**

| pool | valor |
|---|---|
| `main` | **1** (às vezes 2): o pool quase esgotou |
| `replica` | ≈ **18** |

---

### 4. ❌ `min` vs `avg` na mesma janela

```promql
min_over_time(min_over_time_hikaricp_connections_idle{pool="main"}[5m])   # ≈ 1
avg_over_time(min_over_time_hikaricp_connections_idle{pool="main"}[5m])   # ≈ 38
```

**Resultado esperado:** a média de 5 min fica em **~38** (3 amostras ruins em 60 quase não pesam). O mínimo mostra **1**. Se a sua aplicação dá timeout quando o pool chega a 0, é o mínimo que conta a história verdadeira.

❌ **O que dá errado:** um alerta `avg_over_time(hikaricp_connections_idle[5m]) < 5` **nunca dispara** aqui (a média fica em ~38), enquanto os usuários tomam timeout a cada 3 min. `min_over_time(...[5m]) < 5` dispara.

---

### 5 e 6. Um 0/1 de saúde: "falhou alguma vez?"

```promql
min_over_time_probe_success                  # cru
min_over_time(min_over_time_probe_success[1m])
```

**O que faz:** o mínimo de um sinal 0/1 é **0 se houve pelo menos um 0** na janela, senão 1. É um "E lógico" no tempo: "esteve saudável **o tempo todo**?".
**Resultado esperado:**
- cru: `api` com dois pontinhos em 0 a cada 2 min.
- `[1m]`: `api` fica em **0 por ~1 min** depois de cada flap (enquanto o 0 ainda está dentro da janela) e volta para 1. `db` = 1 sempre.

Compare: [`max_over_time`](../max_over_time/) de um 0/1 é o "OU lógico" ("esteve saudável **alguma vez**?") e [`avg_over_time`](../avg_over_time/) é a **fração** do tempo saudável.

---

### 7. Tabela de alerta

```promql
min_over_time(min_over_time_probe_success[5m]) == 0
```

**Resultado esperado:** uma linha só, `service="api"`, valor **0**. O filtro `== 0` remove quem esteve 100% saudável (`db`). Assim nasce um alerta "teve instabilidade nos últimos 5 min", que pega flaps curtos que um `probe_success == 0` simples só pegaria se a avaliação caísse exatamente nos 10s ruins.

---

### 8. Folga mínima do pool em %

```promql
min_over_time(min_over_time_hikaricp_connections_idle[5m]) / 50
```

**Resultado esperado:** `main` ≈ **2%**, `replica` ≈ **36%**.

---

## 🏭 Casos reais

### 1. Pool de conexões quase esgotado (HikariCP / Spring Boot)

A API de pagamentos dá timeouts esporádicos ("Connection is not available, request timed out after 30000ms"). O gráfico de `hikaricp_connections_idle` parece saudável (~40 livres). O vale só aparece com:

```yaml
- alert: PoolDeConexoesQuaseEsgotado
  expr: min_over_time(hikaricp_connections_idle{pool="HikariPool-1"}[5m]) < 2
  for: 0m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.instance }}: pool chegou a {{ $value }} conexões livres nos últimos 5 min"
```

O cenário imita isso com `min_over_time_hikaricp_connections_idle`.

### 2. Flap de target: "caiu pelo menos uma vez?"

`up == 0` só dispara se a avaliação do alerta coincidir com o scrape ruim. Para pegar **qualquer** falha nos últimos 10 min:

```yaml
- alert: TargetInstavel
  expr: min_over_time(up{job="api"}[10m]) == 0
  labels: {severity: info}
```

Mesma ideia com `probe_success` do blackbox exporter (o cenário usa `min_over_time_probe_success`).

### 3. Disco: menor espaço livre do dia

```promql
min_over_time(node_filesystem_avail_bytes{mountpoint="/", fstype!="tmpfs"}[1d])
  / node_filesystem_size_bytes{mountpoint="/", fstype!="tmpfs"}
```

Mostra o "pior momento" (ex.: durante o backup noturno, que gera arquivos temporários) mesmo que agora o disco esteja com folga.

### 4. Réplicas disponíveis durante um rollout

```yaml
- alert: DeploymentFicouSemReplicas
  expr: min_over_time(kube_deployment_status_replicas_available{deployment="checkout"}[15m]) == 0
```

Detecta se, em algum momento do rollout, o deployment ficou com **zero** réplicas disponíveis.

---

## ✅ Quando usar

- **Recursos que acabam:** conexões livres, disco livre (`min_over_time(node_filesystem_avail_bytes[1h])`), bateria, réplicas disponíveis (`min_over_time(kube_deployment_status_replicas_available[10m])`).
- **"Saudável o tempo todo?"** com 0/1: `min_over_time(up[5m]) == 0` pega um target que caiu por um único scrape.
- **SLO de latência mínima / throughput mínimo:** `min_over_time(sum(rate(x[1m]))[1h:])` (pior minuto de tráfego, via subquery), útil para detectar "o tráfego zerou".
- **Painéis de longo prazo** sem esconder vales: `min_over_time(x[$__interval])`.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Menor valor **entre séries** agora | operador `min()` / `bottomk()` |
| Counter cru (o mínimo é só a primeira amostra da janela) | `min_over_time(rate(x[1m])[1h:])` |
| Valor típico | [`avg_over_time()`](../avg_over_time/) |
| Ignorar um único outlier | [`quantile_over_time(0.01, ...)`](../quantile_over_time/) |
| Detectar série que **sumiu** | [`absent_over_time()`](../absent_over_time/) / [`present_over_time()`](../present_over_time/) |

## ⚠️ Pegadinhas

1. **`min_over_time(up[5m]) == 0` não pega scrape faltando.** Se o target some (sem amostra), não há 0 para achar: a amostra simplesmente não existe. `up` resolve porque o Prometheus grava `up=0` em falhas; para métricas de aplicação, use [`count_over_time`](../count_over_time/).
2. **Counter cru:** o mínimo é a amostra mais antiga (ou o valor logo após um reset). Não significa nada.
3. **Outlier domina** (mesma pegadinha do `max`): um único scrape 0 por bug de coleta vira "o mínimo".
4. **O vale "gruda" pela janela inteira:** com `[5m]`, o alerta segue ativo por até 5 min depois da recuperação.
5. **Resultado sem `__name__`** e histogramas ignorados (só amostras float entram).

## 🎓 Na prova PCA

O que costuma cair:
- `min_over_time` (tempo, por série) vs `min` / `bottomk` (entre séries).
- Uso de `min_over_time(up[...]) == 0` para detectar flaps.
- Série **ausente** não é **0**: `*_over_time` só enxerga amostras que existem.
- Range vector na entrada, instant vector na saída.

**1.** Qual consulta dispara se o target teve **pelo menos um** scrape com falha nos últimos 10 min?

- A) `up == 0`
- B) `min_over_time(up[10m]) == 0`
- C) `max_over_time(up[10m]) == 0`
- D) `avg(up) == 0`

<details><summary>Resposta</summary>

**B.** O mínimo de um 0/1 é 0 se houve qualquer 0. A só vê o instante atual; C só dispara se **todos** foram 0; D agrega entre targets.
</details>

**2.** `min_over_time(node_filesystem_avail_bytes[1h])` retorna:

- A) O menor espaço livre de cada filesystem na última hora
- B) O filesystem com menos espaço agora
- C) O espaço livre 1 hora atrás
- D) Um único número para todo o cluster

<details><summary>Resposta</summary>

**A.** Por série, no tempo. B seria `bottomk(1, node_filesystem_avail_bytes)`; C seria `offset 1h`.
</details>

**3.** Uma aplicação para de expor `app_queue_free_slots` por 2 min (a série some). O que `min_over_time(app_queue_free_slots[5m])` mostra nesse período?

- A) 0
- B) O menor valor entre as amostras que existiram na janela
- C) NaN
- D) Erro

<details><summary>Resposta</summary>

**B.** Ausência não gera amostra 0. Para detectar a falta, use `count_over_time` ou `absent_over_time`.
</details>

**4.** Qual é o equivalente "E lógico no tempo" para um sinal 0/1?

- A) `max_over_time`
- B) `sum_over_time`
- C) `min_over_time`
- D) `count_over_time`

<details><summary>Resposta</summary>

**C.** `min_over_time` = 1 só se **todas** as amostras forem 1 ("OK o tempo todo"). `max_over_time` é o "OU".
</details>

## 📝 Cola rápida

- `min_over_time(x[j])` = menor amostra **de cada série** na janela; `min(x)` = menor **entre séries** agora.
- Para recursos que **acabam**: conexões livres, disco livre, réplicas disponíveis.
- 0/1: `min_over_time(up[10m]) == 0` = "falhou alguma vez" (E lógico no tempo).
- Ausência ≠ 0: série sumida não vira mínimo 0.
- Ignora histogramas; resultado sem `__name__`.

## 🔗 Relacionadas

[`max_over_time()`](../max_over_time/) · [`avg_over_time()`](../avg_over_time/) · [`quantile_over_time()`](../quantile_over_time/) · [`ts_of_min_over_time()`](../ts_of_min_over_time/) · [`count_over_time()`](../count_over_time/) · [`absent_over_time()`](../absent_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
