# `changes()`: quantas vezes o valor mudou

> **Em uma frase:** `changes(v[janela])` conta quantas vezes, dentro da janela, uma amostra teve valor **diferente** da anterior, para cima ou para baixo. Perfeito para gauges "de estado": versão, health check, modo de operação.

| | |
|---|---|
| **Assinatura** | `changes(v range-vector) → instant-vector` |
| **Tipo de métrica** | ✅ Gauge de estado (versão, 0/1, enum) · ⚠️ Counter (funciona, mas raramente é útil) |
| **Unidade do resultado** | número de mudanças (inteiro, sem extrapolação) |
| **Dashboard** | http://localhost:3300/d/fn-changes |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o placar do jogo

No fim do jogo, alguém pergunta: "**quantas vezes o placar mudou?**". Você não soma os gols nem liga se foi gol do time A ou B: só conta **quantas vezes o número no telão foi trocado**.

```
amostras:  7   7   7   8   8   7   7   9
                       ↑       ↑       ↑
                    mudou   mudou   mudou      → changes = 3
```

- Subir ou descer, tanto faz: `7 → 8` e `8 → 7` contam igual.
- Ficar igual **não** conta.
- Uma amostra float seguida de uma native histogram (ou vice-versa) também conta como mudança.

Compare com o irmão [`resets()`](../resets/), que conta **só as quedas**.

---

## 🔧 Setup: o que o gerador fake expõe

O cenário ([`setup/scenario.go`](setup/scenario.go)) imita métricas reais de kube-state-metrics, blackbox_exporter e client libraries:

| Métrica | Tipo | Comportamento |
|---|---|---|
| `changes_kube_deployment_status_observed_generation{deployment="api"}` | gauge | generation do Deployment: **+1 a cada 2 min** (um rollout) |
| `changes_kube_deployment_status_observed_generation{deployment="worker"}` | gauge | **+1 a cada 5 min** |
| `changes_kube_deployment_status_observed_generation{deployment="legacy"}` | gauge | parado em **42**, nunca muda |
| `changes_probe_success{target="https://pagamentos.exemplo.com"}` | gauge 0/1 | sempre **1** |
| `changes_probe_success{target="https://frete.exemplo.com"}` | gauge 0/1 | alterna 1 ↔ 0 a **cada 15s** (flapping) |
| `changes_process_start_time_seconds{pod="api-1"}` | gauge | timestamp de start; o processo **reinicia a cada 3 min** |
| `changes_process_start_time_seconds{pod="api-2"}` | gauge | nunca reinicia |
| `changes_http_requests_total` | counter | +3/s, só para a pegadinha |

```bash
curl -s localhost:8088/metrics | grep '^changes_'
# changes_kube_deployment_status_observed_generation{deployment="api"} 218
# changes_kube_deployment_status_observed_generation{deployment="legacy"} 42
# changes_probe_success{target="https://frete.exemplo.com"} 0
# changes_process_start_time_seconds{pod="api-1"} 1.79037438e+09
```

## ▶️ Como rodar

```bash
# na raiz do projeto
docker compose up -d --build
# Prometheus: http://localhost:9095   Grafana: http://localhost:3300/d/fn-changes
```

Espere **~10 minutos** para a janela `[10m]` ficar cheia.

---

## 🔍 Queries passo a passo

### 1. Generation do Deployment (degraus)

```promql
changes_kube_deployment_status_observed_generation{deployment!="legacy"}
```

**Resultado esperado:** duas escadinhas. `api` sobe um degrau a cada 2 min; `worker`, a cada 5 min. (O `legacy` fica fora do gráfico porque está em 42 e achataria a escala.) No Kubernetes, a `generation` de um Deployment aumenta a cada mudança no spec, ou seja, a cada deploy.

---

### 2. Deploys nos últimos 10 minutos

```promql
changes(changes_kube_deployment_status_observed_generation[10m])
```

**Resultado esperado:**

| deployment | changes em 10 min |
|---|---|
| `api` | **5** (às vezes 4) |
| `worker` | **2** (às vezes 1) |
| `legacy` | **0** |

> 💡 O valor alterna entre dois inteiros dependendo de onde os degraus caem dentro da janela. Sempre inteiro: não há extrapolação.

---

### 3 e 4. Probe "flapping"

```promql
changes_probe_success                          # 0/1
changes(changes_probe_success[5m])             # quantas vezes mudou
changes(changes_probe_success[5m]) > 4         # alerta de flapping
```

**O que faz:** um endpoint que cai e volta rapidamente é traiçoeiro: o `probe_success == 0` pode nunca durar o suficiente para disparar um alerta `for: 1m`, mas os usuários sofrem com erros intermitentes.

**Resultado esperado:**
- `pagamentos`: `changes` = **0** (sempre 1).
- `frete`: `changes[5m]` ≈ **20** (300s / 15s) → aparece na query `> 4` como 🔥.

---

### 5. Restarts via `process_start_time_seconds`

```promql
changes(changes_process_start_time_seconds[10m])
```

**O que faz:** toda client library expõe `process_start_time_seconds` (o Unix timestamp de quando o processo subiu). Esse valor **só muda quando o processo reinicia**, então `changes()` sobre ele = **número de restarts**.

**Resultado esperado:** `api-1` ≈ **3** (600s / 180s; às vezes 4), `api-2` = **0**.

---

### 6. Pegadinha: `changes()` num counter vivo

```promql
changes(changes_http_requests_total[1m])
count_over_time(changes_http_requests_total[1m]) - 1
```

**Resultado esperado:** as duas linhas **iguais**, em ≈ **11** (12 amostras em 1 min com scrape de 5s, 11 transições). Um counter que cresce muda em **todo** scrape, então `changes()` só está contando amostras. Para counters use [`rate()`](../rate/), [`increase()`](../increase/) ou [`resets()`](../resets/).

---

### 7. Painel de deploys

```promql
changes(changes_kube_deployment_status_observed_generation[10m])
```

**Resultado esperado:** barras `api` ≈ 5, `worker` ≈ 2, `legacy` 0. Em produção, com `[1d]`, isso vira "deploys por Deployment hoje".

---

## 🏭 Casos reais

### 1. Target "flapping" no blackbox_exporter / `up` (imitado pelos painéis 3-4)

O checkout depende da API de frete. O alerta `probe_success == 0 for 2m` nunca dispara, mas o time de atendimento recebe reclamações de "às vezes dá erro no frete". O motivo: o endpoint cai e volta a cada poucos segundos.

```yaml
groups:
- name: blackbox
  rules:
  - alert: EndpointFlapping
    expr: changes(probe_success[10m]) > 4
    for: 5m
    labels: {severity: warning}
    annotations:
      summary: "{{ $labels.instance }} mudou de estado {{ $value }}x em 10 min"
  - alert: TargetFlapping
    expr: changes(up[10m]) > 4
    for: 5m
```

**Decisão:** `changes` (e não `resets`) porque tanto a queda (1→0) quanto a volta (0→1) interessam.

### 2. Restarts sem Kubernetes (imitado pelo painel 5)

Um serviço em VM (systemd) não tem kube-state-metrics. Mas ele expõe `process_start_time_seconds`:

```yaml
- alert: ProcessoReiniciou
  expr: changes(process_start_time_seconds{job="billing"}[30m]) > 2
  labels: {severity: warning}
```

Bônus: `time() - process_start_time_seconds` = uptime em segundos.

### 3. Frequência de deploys (imitado pelos painéis 1, 2 e 7)

Métrica DORA "deployment frequency" direto do cluster:

```promql
sum by (namespace) (changes(kube_deployment_status_observed_generation[1d]))
```

E um alerta de "rollout em loop" (Argo CD/Flux brigando com alguém que editou o Deployment na mão):

```yaml
- alert: DeploymentMudandoDemais
  expr: changes(kube_deployment_status_observed_generation[15m]) > 5
```

### 4. Reload de configuração do próprio Prometheus

```promql
changes(prometheus_config_last_reload_success_timestamp_seconds[1h])   # quantos reloads com sucesso na última hora
```

## ✅ Quando usar

- **Contar deploys/reloads:** `changes(kube_deployment_status_observed_generation[1d])` ou `changes(prometheus_config_last_reload_success_timestamp_seconds[1h])`.
- **Flapping:** `changes(up[10m]) > 4`, `changes(probe_success[10m]) > 4`, `changes(kube_node_status_condition{condition="Ready",status="true"}[15m]) > 2`.
- **Restarts de processo:** `changes(process_start_time_seconds[1h]) > 2`.
- **Mudanças de estado/modo:** leader election (`changes(is_leader[1h])`), failover de banco, troca de réplica primária.
- **Timestamps de "última vez que X aconteceu":** `changes(last_backup_timestamp_seconds[1d]) == 0` → nenhum backup hoje.

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Quer **quanto** mudou, não quantas vezes | [`delta()`](../delta/) (gauge) ou [`increase()`](../increase/) (counter) |
| Quer contar **restarts** por um counter | [`resets()`](../resets/) |
| Gauge **contínuo** e ruidoso (temperatura, CPU) | muda em todo scrape: use [`delta()`](../delta/), [`deriv()`](../deriv/) ou `stddev_over_time()` |
| Quer saber **se** a métrica existiu na janela | [`present_over_time()`](../present_over_time/) |

## ⚠️ Pegadinhas

1. **Mudanças entre scrapes são invisíveis:** se o backend cai e volta em 2s e o scrape é de 15s, o Prometheus pode nunca ver o `0`. `changes()` só conta o que foi **amostrado**.
2. **Ida e volta conta 2:** `1 → 0 → 1` são **2** mudanças. Para "quantas quedas", use `resets()` (conta só descidas) num gauge 0/1 com cuidado, ou `count_over_time((x == 0)[10m:])`.
3. **Ruído de ponto flutuante:** `0.1 + 0.2` vs `0.3` são valores diferentes. Gauges "contínuos" mudam em quase todo scrape.
4. **Série nova não é mudança:** se a versão estiver num **label** (`build_info{version="1.2.3"} 1`), cada versão é uma série nova com valor sempre 1, e `changes()` dá 0. Aí você conta séries: `count(count_over_time(build_info[1h]))`.
5. **Sem extrapolação:** o resultado é inteiro e depende de quantas mudanças caíram dentro da janela.

## 🎓 Na prova PCA

O que costuma cair:
- `changes` recebe **range vector**, devolve **instant vector** com o **número de vezes que o valor mudou** (inteiro, sem extrapolação).
- Conta **subidas e descidas**; valores iguais consecutivos não contam.
- Contraste clássico: `changes` (qualquer mudança) × `resets` (só quedas, para counters).
- Uso típico: gauges de estado (`up`, `probe_success`, versões, timestamps de start/reload).

**1.** Qual expressão detecta um target que ficou alternando entre up e down nos últimos 10 minutos?
- A) `resets(up[10m]) > 4`
- B) `changes(up[10m]) > 4`
- C) `delta(up[10m]) > 4`
- D) `rate(up[10m]) > 4`

<details><summary>Resposta</summary>

**B.** `changes` conta toda mudança 0↔1. A só contaria quedas (e `up` não é counter); C daria no máximo ±1; D é para counters.
</details>

**2.** Amostras de um gauge na janela: `3, 3, 5, 5, 3, 3`. Qual é o valor de `changes()`?
- A) 0
- B) 1
- C) 2
- D) 5

<details><summary>Resposta</summary>

**C.** 3→5 e 5→3. Os pares repetidos (3,3 e 5,5) não contam.
</details>

**3.** Qual métrica, exposta por padrão pelas client libraries, permite contar restarts de um processo com `changes()`?
- A) `process_resident_memory_bytes`
- B) `process_start_time_seconds`
- C) `process_open_fds`
- D) `go_goroutines`

<details><summary>Resposta</summary>

**B.** O timestamp de start só muda quando o processo reinicia. As outras mudam o tempo todo.
</details>

**4.** Qual é o tipo de retorno de `changes(x[5m])`?
- A) Range vector
- B) Instant vector
- C) Scalar
- D) String

<details><summary>Resposta</summary>

**B.** Como todas as funções que recebem range vector e "resumem" a janela (`rate`, `resets`, `*_over_time`...), devolve um instant vector com uma amostra por série.
</details>

## 📝 Cola rápida

- `changes(v[janela])` = quantas vezes o valor **mudou** (↑ ou ↓), inteiro.
- `resets` = só **quedas** (counters). `changes` = qualquer mudança (gauges de estado).
- Flapping: `changes(up[10m]) > 4` / `changes(probe_success[10m]) > 4`.
- Restarts: `changes(process_start_time_seconds[1h])`.
- Em counter vivo ou gauge contínuo, `changes` ≈ nº de amostras − 1 (inútil).

## 🔗 Relacionadas

[`resets()`](../resets/) · [`delta()`](../delta/) · [`idelta()`](../idelta/) · [`increase()`](../increase/) · [`count_over_time()`](../count_over_time/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#changes
