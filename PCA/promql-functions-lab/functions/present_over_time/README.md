# `present_over_time()`: a série apareceu na janela?

> **Em uma frase:** `present_over_time(v[janela])` devolve **1** para **cada série que teve pelo menos uma amostra** na janela, ignorando os valores. Séries que não apareceram **não** viram 0: simplesmente não estão no resultado.

| | |
|---|---|
| **Assinatura** | `present_over_time(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Qualquer uma (gauge, counter, info, native histogram) |
| **Unidade do resultado** | sempre **1** (booleano "existiu") |
| **Dashboard** | http://localhost:3300/d/fn-present_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o carimbo no passaporte

Na imigração, o agente só quer saber: **"você entrou no país nos últimos 90 dias?"**. Não importa se ficou 1 dia ou 80, nem o que fez lá. Tem carimbo, é **sim**.

- Cada série é um **viajante**; cada amostra, um **dia no país**.
- `present_over_time(x[5m])` = "carimbo dos últimos 5 min": **1** para quem apareceu, **ausente** para quem não apareceu.
- Não existe "carimbo 0". Para saber quem **não** veio, é preciso comparar com outra lista (`unless`) ou usar [`absent_over_time()`](../absent_over_time/).

Família das "presenças":

| Pergunta | Função |
|---|---|
| "Esteve aqui?" (1 ou nada) | `present_over_time` |
| "Quantas vezes esteve?" | [`count_over_time`](../count_over_time/) |
| "Faltou o tempo todo?" (1 se **nenhuma** série bateu) | [`absent_over_time`](../absent_over_time/) |

E, como toda `*_over_time`, olha **no tempo, por série** (→). O operador `count(x)` olha **entre séries, agora** (↓).

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `present_over_time_worker_active_tasks{worker="worker-a"}` | gauge | valor **3**, **sempre** presente |
| `present_over_time_worker_active_tasks{worker="worker-b"}` | gauge | valor **5**, **vivo 60s, morto 120s** (ciclo de 3 min) |
| `present_over_time_worker_active_tasks{worker="worker-c"}` | gauge | valor **8**, "cron": vivo **20s a cada 5 min** |

Os valores 3, 5 e 8 existem só para provar que **não importam**: o resultado é sempre 1.

```bash
curl -s localhost:8088/metrics | grep '^present_over_time_'
# present_over_time_worker_active_tasks{worker="worker-a"} 3
# present_over_time_worker_active_tasks{worker="worker-b"} 5     <- às vezes
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-present_over_time
```

Espere **~5 min** para ver o worker-c aparecer pelo menos uma vez.

---

## 🔍 Queries passo a passo

### 1. Cru

```promql
present_over_time_worker_active_tasks
```

**Resultado esperado:** `worker-a` reto em 3; `worker-b` em 5 durante 1 min a cada 3 min; `worker-c` em 8 por 20s a cada 5 min. Fora disso, **buracos** (a série some).

---

### 2. Visto no último minuto?

```promql
present_over_time(present_over_time_worker_active_tasks[1m])
```

**Resultado esperado:** todas as linhas em **1** (nunca 3, 5 ou 8).
- `worker-a`: 1 sempre.
- `worker-b`: 1 por **~2 min** (60s vivo + 60s em que a última amostra ainda está na janela), ausente por ~1 min.
- `worker-c`: 1 por **~80 s** (20s vivo + 60s de janela), ausente por ~3m40s.

---

### 3. Janela maior que a maior ausência

```promql
present_over_time(present_over_time_worker_active_tasks[5m])
```

**Resultado esperado:** as três linhas em **1**, contínuas. A janela de 5 min sempre contém pelo menos uma aparição do `worker-c` (que aparece a cada 5 min).

---

### 4. `count(x)` vs `count(present_over_time(x[5m]))`

```promql
count(present_over_time_worker_active_tasks)                           # vivos agora
count(present_over_time(present_over_time_worker_active_tasks[5m]))    # distintos em 5 min
```

**Resultado esperado:** "vivos agora" oscila entre **1 e 3** (quase sempre 1 ou 2). "Vistos em 5 min" fica em **3**. É o jeito de responder "quantos workers **distintos** passaram por aqui na última hora?", mesmo que nunca tenham estado vivos ao mesmo tempo.

---

### 5. Quem sumiu?

```promql
present_over_time(present_over_time_worker_active_tasks[5m])
  unless present_over_time_worker_active_tasks
```

**O que faz:** "estava nos últimos 5 min" **menos** "está agora" (o `unless` remove do lado esquerdo as séries com os mesmos labels do lado direito).
**Resultado esperado:** `worker-b` aparece durante os 2 min em que está morto; `worker-c` durante quase todo o ciclo. `worker-a` só aparece em blips curtos quando alguém roda `tools/deploy.sh` (o gerador reinicia e **todas** as séries somem por alguns segundos: o alerta funciona!).

É a base do alerta "**uma série que existia sumiu**" sem precisar escrever um `absent()` por label.

---

### 6. Tabela

(instantânea) **Resultado esperado:** três linhas (`worker-a`, `worker-b`, `worker-c`), todas com valor **1** e **sem** `__name__`.

### 7. ❌ O que dá errado: procurar o "0"

```promql
present_over_time(present_over_time_worker_active_tasks[1m]) == 0
```

**Resultado esperado:** **vazio, sempre** (o painel 7 mostra "No data" de propósito). Mesmo quando o `worker-c` está sumido há 4 min, não existe uma série com valor 0 para o filtro achar. Para "quem sumiu", use o `unless` do item 5; para "não existe nenhuma", use `absent_over_time`.

---

## 🏭 Casos reais

### 1. "Um node exporter sumiu" (sem listar os nodes à mão)

`absent(up{job="node"})` só dispara quando **todos** somem. Para pegar **um** node que parou de ser descoberto (ex.: removido do service discovery por engano):

```yaml
- alert: NodeExporterSumiu
  expr: |
    present_over_time(up{job="node"}[1h])
      unless up{job="node"}
  for: 5m
  labels: {severity: warning}
  annotations:
    summary: "{{ $labels.instance }} existia na última hora e não existe mais"
```

(Note: se o target **existe** mas falha, `up == 0` já resolve. Este alerta é para quando o target **desaparece** do SD.)

### 2. Inventário: quantos pods distintos rodaram hoje?

```promql
count(present_over_time(kube_pod_info{namespace="checkout"}[1d]))
```

Útil para detectar **churn** (pods sendo recriados em loop): se o deployment tem 3 réplicas e o número de pods distintos no dia é 250, algo está em crashloop/rolando deploy o tempo todo.

### 3. Batch jobs: "o cron rodou hoje?"

```yaml
- alert: BackupNaoRodou
  expr: |
    absent_over_time(backup_duration_seconds{job="backup"}[26h])
```

E o painel positivo, "quais jobs rodaram nas últimas 24h":

```promql
present_over_time(backup_duration_seconds[24h])
```

### 4. Rollout: quais versões rodaram na última hora?

Durante um deploy canário, pods da versão nova e da antiga convivem. Para um painel "versões vistas na última hora" a partir de uma métrica `*_build_info` (sempre 1, com o label `version`):

```promql
count by (version) (present_over_time(app_build_info{app="checkout"}[1h]))
```

Resultado típico: `{version="1.41.0"} 3` e `{version="1.42.0"} 3` enquanto o rollout está no ar; 1h depois de terminado, só a nova.

---

## ✅ Quando usar

- **"Existiu nos últimos X?"** sem ligar para o valor.
- **Contar séries distintas** num período: `count(present_over_time(x[1d]))`.
- **Detectar séries que sumiram** com `unless`.
- **Painéis de inventário** (quais jobs/pods/versões apareceram).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Alertar que **nenhuma** série com esses labels existe | [`absent_over_time()`](../absent_over_time/) / [`absent()`](../absent/) |
| Quer **quantas** amostras | [`count_over_time()`](../count_over_time/) |
| Quer o **valor** da última amostra | [`last_over_time()`](../last_over_time/) |
| Target existe mas falha no scrape | `up == 0` |

## ⚠️ Pegadinhas

1. **Nunca devolve 0.** Série ausente = série fora do resultado. `present_over_time(x[5m]) == 0` **nunca** retorna nada.
2. **Valor ignorado:** uma amostra com valor 0 ou NaN conta como "presente".
3. **Staleness markers não contam** como presença.
4. **Janela vs intervalo:** para algo que aparece a cada N min, use janela **> N** senão haverá "buracos" de presença.
5. **Resultado sem `__name__`**: com um seletor que pega **várias métricas** (ex.: `present_over_time({job="api"}[1h])`), séries de métricas diferentes com os mesmos labels colidem e a consulta falha com `vector cannot contain metrics with the same labelset`. Use um nome de métrica no seletor.
6. **Custo:** `count(present_over_time(kube_pod_info[30d]))` lê todas as séries (e amostras) do mês: em clusters grandes é uma consulta cara. Prefira recording rules ou janelas menores.

## 🎓 Na prova PCA

O que costuma cair:
- `present_over_time` retorna **1** ou **nada**; `absent_over_time` retorna **1 quando não há nada**. Não confundir.
- Recebe **range vector**.
- Diferença entre "série não existe" e "série com valor 0".

**1.** O que `present_over_time(http_requests_total{code="500"}[10m])` retorna para uma série que teve 12 amostras com valores 0, 0, 3, 5...?

- A) 12
- B) 5
- C) 1
- D) 0

<details><summary>Resposta</summary>

**C.** `present_over_time` sempre retorna 1 para séries com pelo menos uma amostra, ignorando valores. 12 seria `count_over_time`; 5 seria `last_over_time`/`max_over_time`.
</details>

**2.** Uma série não teve **nenhuma** amostra nos últimos 10 min. O que `present_over_time(x[10m])` retorna para ela?

- A) 0
- B) NaN
- C) Nada (a série não aparece no resultado)
- D) 1

<details><summary>Resposta</summary>

**C.** Funções `*_over_time` não inventam séries. Para "1 quando falta", use `absent_over_time`.
</details>

**3.** Qual expressão lista séries que existiram na última hora mas **não existem agora**?

- A) `absent(x)`
- B) `present_over_time(x[1h]) unless x`
- C) `present_over_time(x[1h]) == 0`
- D) `count_over_time(x[1h]) == 0`

<details><summary>Resposta</summary>

**B.** `unless` remove do lado esquerdo o que existe agora. C e D nunca retornam nada (não há 0); A só funciona se **todas** as séries sumirem e não diz quais.
</details>

**4.** Para contar quantos pods **distintos** existiram no último dia:

- A) `count(kube_pod_info)`
- B) `count_over_time(kube_pod_info[1d])`
- C) `count(present_over_time(kube_pod_info[1d]))`
- D) `sum(kube_pod_info)`

<details><summary>Resposta</summary>

**C.** `present_over_time` gera 1 por série vista no dia; `count` conta as séries. A e D contam só os de agora; B conta amostras **por série**.
</details>

## 📝 Cola rápida

- `present_over_time(x[j])` = **1** por série que teve ≥ 1 amostra na janela. **Nunca 0.**
- Valor ignorado. Staleness markers não contam.
- `count(present_over_time(x[j]))` = séries **distintas** no período.
- `present_over_time(x[j]) unless x` = séries que **sumiram**.
- Oposto: `absent_over_time` (1 quando **nada** existe).

## 🔗 Relacionadas

[`absent_over_time()`](../absent_over_time/) · [`absent()`](../absent/) · [`count_over_time()`](../count_over_time/) · [`last_over_time()`](../last_over_time/) · [`first_over_time()`](../first_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
