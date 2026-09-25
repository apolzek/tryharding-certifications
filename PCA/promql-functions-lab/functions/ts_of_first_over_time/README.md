# `ts_of_first_over_time()`: quando a série apareceu na janela (experimental)

> **Em uma frase:** `ts_of_first_over_time(v[janela])` devolve o **timestamp Unix (em segundos)** da amostra **mais antiga** de cada série **dentro da janela**. Serve para responder "desde quando essa série existe?" (limitado ao tamanho da janela).

| | |
|---|---|
| **Assinatura** | `ts_of_first_over_time(v range-vector) → instant-vector` |
| **Status** | 🧪 **Experimental**: exige `--enable-feature=promql-experimental-functions` (habilitado neste lab) |
| **Tipo de métrica** | ✅ Qualquer uma (o valor não importa, só o timestamp). Ideal para métricas "info" (`*_build_info`) |
| **Unidade do resultado** | **segundos desde 1970** (timestamp Unix, com fração) |
| **Dashboard** | http://localhost:3300/d/fn-ts_of_first_over_time |
| **Cenário** | [`setup/scenario.go`](setup/scenario.go) |

---

## 🧠 Analogia: o "membro desde" de um perfil

Em fóruns aparece "**membro desde** março de 2019". `ts_of_first_over_time` é isso para séries temporais: quando **cada combinação de labels** apareceu pela primeira vez.

Mas com uma limitação importante: ela **só olha dentro da janela**. É como um porteiro que só guarda o livro de visitas da **última hora**: para quem está no prédio há 3 horas, ele responde "desde que comecei a anotar" (o início da janela).

```
janela [10m]:        |<──────────────────────────────>|
série antiga:   ●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●●   → ts = início da janela (cortado!)
versão nova:                          ●●●●●●●●●●●●●●●●   → ts = quando ela apareceu (deploy)
```

Um "deploy" costuma mudar o label `version` de uma métrica `*_build_info`, criando uma **série nova**. Então `ts_of_first_over_time(app_build_info[1h])` = **horário de cada deploy** da última hora.

---

## 🔧 Setup: o que o gerador fake expõe

| Métrica | Tipo | Comportamento |
|---|---|---|
| `ts_of_first_over_time_app_build_info{version="1.N.0"}` | gauge (sempre 1) | imita `*_build_info`: um **deploy novo a cada 3 min**; o label `version` muda e a versão anterior **some para sempre** |
| `ts_of_first_over_time_process_up` | gauge (sempre 1) | existe **sempre** (como `up`); serve para mostrar o "corte" da janela |

```bash
curl -s localhost:8088/metrics | grep '^ts_of_first_over_time_'
# ts_of_first_over_time_app_build_info{version="1.537.0"} 1
# ts_of_first_over_time_process_up 1
```

## ▶️ Como rodar

```bash
tools/deploy.sh
# Grafana: http://localhost:3300/d/fn-ts_of_first_over_time
```

Espere **~10 min** para ver alguns deploys e a janela `[10m]` cheia.

---

## 🔍 Queries passo a passo

### 1. `build_info` cru: a escadinha de deploys

```promql
ts_of_first_over_time_app_build_info
```

**Resultado esperado:** uma linha em **1** que troca de **cor** (de série) a cada 3 min: cada cor é uma versão. Na legenda, 5 versões em 15 min.

---

### 2. Há quanto tempo a versão ATUAL está no ar

```promql
(time() - ts_of_first_over_time(ts_of_first_over_time_app_build_info[1h]))
  and on(version) ts_of_first_over_time_app_build_info
```

**O que faz:** calcula a "idade" de cada versão vista na última hora e usa `and on(version) <métrica>` para manter só a versão que **existe agora**.
**Resultado esperado:** **dente-de-serra de 0 a ~180s**: zera a cada deploy.

> 💡 Sem o `and`, as versões antigas continuariam no resultado (elas têm amostras na janela de 1h), cada uma com uma idade crescente.

---

### 3. Pegadinha: o resultado é cortado no início da janela

```promql
time() - ts_of_first_over_time(ts_of_first_over_time_process_up[5m])
time() - ts_of_first_over_time(ts_of_first_over_time_process_up[10m])
```

**Resultado esperado:** duas linhas **planas**: ≈ **300s** com `[5m]` e ≈ **600s** com `[10m]` (menos ~5s, o intervalo entre o início da janela e a 1ª amostra dentro dela). A série existe há muito mais tempo, mas a função **não vê** antes da janela.
**Moral:** "idade" medida assim é `min(idade real, tamanho da janela)`.

---

### 4. Histórico de deploys (tabela)

```promql
ts_of_first_over_time(ts_of_first_over_time_app_build_info[15m]) * 1000   # entrou
ts_of_last_over_time(ts_of_first_over_time_app_build_info[15m]) * 1000    # última amostra
ts_of_last_over_time(...[15m]) - ts_of_first_over_time(...[15m])          # tempo de vida
```

**Resultado esperado:** uma linha por versão dos últimos 15 min, com o horário de entrada (espaçados de 3 em 3 min) e tempo de vida ≈ **175s** (3 min menos um intervalo de scrape). A versão atual tem tempo de vida menor (ainda está viva) e a mais antiga pode aparecer "cortada" pela janela.

---

### 5. 🟡 O que dá errado: esquecer o `and on(version)`

```promql
time() - ts_of_first_over_time(ts_of_first_over_time_app_build_info[1h])
```

**Resultado esperado:** **várias** linhas (uma por versão vista na última hora), cada uma começando em 0 quando a versão foi lançada e **subindo para sempre**, mesmo depois que a versão morreu. No fim do painel, a versão mais antiga está com ~15 min (ou até 1h) de "idade", embora tenha vivido só 3 min. Compare com o painel 2, onde o `and on(version) ts_of_first_over_time_app_build_info` deixa só a versão viva.

---

## 🏭 Casos reais

### 1. Anotação de deploys no Grafana / "quando essa versão subiu?"

Quase toda aplicação expõe `*_build_info{version, revision}` (ex.: `prometheus_build_info`, `kube_pod_container_info{image}`). O horário de cada deploy da última semana:

```promql
ts_of_first_over_time(prometheus_build_info[7d]) * 1000
```

Numa tabela do Grafana com unidade `dateTimeAsIso`, vira um changelog automático.

### 2. Alerta: "versão recém-lançada com erros" (janela de canário)

Só alertar sobre erros se a versão atual tem **menos de 30 min** no ar (período de canário):

```yaml
- alert: NewReleaseErrorBudgetBurn
  expr: |
    (
      sum by (version) (rate(http_requests_total{code=~"5.."}[5m]))
        / sum by (version) (rate(http_requests_total[5m]))
    ) > 0.02
    and on(version)
    (time() - ts_of_first_over_time(app_build_info[1h])) < 1800
  labels: {severity: page}
  annotations:
    summary: "Versão {{ $labels.version }} (< 30 min no ar) com > 2% de erros: considere rollback"
```

### 3. "Série nova" / explosão de cardinalidade

Séries que surgiram nos últimos 10 min (label novo inesperado, pod novo):

```yaml
- record: job:series_created_last_10m:count
  expr: count by (job) ((time() - ts_of_first_over_time({job="api"}[1h])) < 600)
- alert: CardinalitySpike
  expr: job:series_created_last_10m:count > 1000
  for: 10m
```

Um salto nesse número é sinal de **explosão de cardinalidade** (um label com user_id, por exemplo).

### 4. Uptime do processo? Prefira `process_start_time_seconds`

Para "há quanto tempo o processo está de pé", a métrica certa é `time() - process_start_time_seconds`, **sem limite de janela**. `ts_of_first_over_time(up[1h])` cortaria em 1h.

---

## ✅ Quando usar

- **Horário de deploys/mudanças** a partir de métricas `*_build_info`.
- **Janela de canário**: condicionar alertas à idade da versão.
- **Detectar séries novas** (cardinalidade, pods novos).

## ❌ Quando NÃO usar

| Situação | Use em vez disso |
|---|---|
| Uptime do processo | `time() - process_start_time_seconds` |
| Quer o **valor** da primeira amostra | [`first_over_time()`](../first_over_time/) |
| Quer o horário da última amostra | [`ts_of_last_over_time()`](../ts_of_last_over_time/) |
| A série é mais velha que qualquer janela razoável | métrica de start time (`process_start_time_seconds`, `kube_pod_start_time`) |
| Ambiente sem a flag experimental | `first_over_time` + `timestamp` não resolvem; use métricas de start time |

## ⚠️ Pegadinhas

1. **Experimental**: sem a feature flag, erro de função desconhecida.
2. **Cortado na janela** (query 3): "idade" nunca passa do tamanho da janela.
3. **Séries que já morreram continuam no resultado** enquanto tiverem amostras na janela: filtre com `and on(...) metrica` (query 2).
4. **Timestamp do 1º scrape**, não do evento real: pode atrasar até 1 scrape interval.
5. **Janelas longas (`[7d]`) são caras**: leem todas as amostras do período. Use recording rules ou janelas curtas em dashboards.
6. Parecido com `first_over_time(m[1m])` vs `m offset 1m` na doc: as funções `_over_time` olham **dentro** da janela; `offset` pega a amostra mais recente **antes** do ponto deslocado.

---

## 🎓 Na prova PCA

`ts_of_first_over_time` é **experimental**; o que a prova cobra são os conceitos vizinhos: `*_build_info`, `process_start_time_seconds`, `time()`, `first_over_time`, e a diferença entre `offset` e funções `_over_time`.

**1.** Qual a forma **recomendada** de calcular o uptime de um processo instrumentado com client library oficial?

- A) `time() - ts_of_first_over_time(up[1h])`
- B) `time() - process_start_time_seconds`
- C) `count_over_time(up[1h]) * 15`
- D) `timestamp(up)`

<details><summary>Resposta</summary>

**B.** `process_start_time_seconds` é exposto pelas client libraries e não depende de janela. A) seria cortado em 1h (e é experimental).
</details>

**2.** Uma série existe há 3 horas. Quanto vale aproximadamente `time() - ts_of_first_over_time(x[1h])`?

- A) 10800
- B) 3600
- C) 0
- D) Vazio

<details><summary>Resposta</summary>

**B.** A função só enxerga a janela: a amostra mais antiga dentro de `[1h]` tem ~1h de idade.
</details>

**3.** Qual a diferença entre `first_over_time(x[5m])` e `x offset 5m`?

- A) Nenhuma.
- B) `first_over_time` pega a primeira amostra **dentro** da janela de 5m; `offset 5m` pega a amostra mais recente **antes** do instante deslocado (podendo estar fora da janela).
- C) `offset` só funciona com counters.
- D) `first_over_time` é experimental.

<details><summary>Resposta</summary>

**B.** Exatamente o que a documentação explica sobre `first_over_time`. (Ela é estável; experimentais são `ts_of_*` e `mad_over_time`.)
</details>

**4.** O label `version` de `app_build_info` muda a cada deploy. Quantas séries `app_build_info` existem, no total, na última hora se houve 4 deploys?

- A) 1
- B) 4
- C) 5
- D) Depende do scrape interval

<details><summary>Resposta</summary>

**C.** A versão que estava rodando antes do 1º deploy + 4 novas = 5 combinações de labels = 5 séries. Cada valor de label distinto é uma série diferente.
</details>

---

## 📝 Cola rápida

- `ts_of_first_over_time(x[w])` = timestamp (s) da amostra **mais antiga** na janela; **experimental**.
- Resultado **cortado** no início da janela: "idade" ≤ tamanho da janela.
- `*_build_info` + `ts_of_first_over_time` = horário de deploys. Filtre a versão atual com `and on(version) metrica`.
- Uptime de processo: `time() - process_start_time_seconds` (não esta função).
- `first_over_time` (valor, estável) ≠ `ts_of_first_over_time` (timestamp, experimental).

## 🔗 Relacionadas

[`first_over_time()`](../first_over_time/) · [`ts_of_last_over_time()`](../ts_of_last_over_time/) · [`timestamp()`](../timestamp/) · [`time()`](../time/) · [`present_over_time()`](../present_over_time/) · [`start_timestamp()`](../start_timestamp/)

## 📚 Referência

https://prometheus.io/docs/prometheus/latest/querying/functions/#aggregation_over_time
