# Como criar uma lição (uma função PromQL)

Cada função PromQL tem **uma pasta** em `functions/<nome-da-função>/` com:

```
functions/<fn>/
├── README.md            # a aula: analogia, assinatura, queries explicadas, resultado esperado, quando usar
├── lab.yaml             # painéis/queries -> vira dashboard Grafana (uid fn-<fn>) e casos de teste
└── setup/
    └── scenario.go      # gerador de métricas fake específico dessa lição
```

A lição de referência é **`functions/rate/`**. Copie o estilo, a estrutura e o nível de detalhe dela.

## Stack (versões fixas p/ tudo)

| | versão | URL |
|---|---|---|
| Prometheus | v3.15.0 | http://localhost:9095 |
| Grafana | 13.2.2 | http://localhost:3300 (anônimo/admin) |
| Gerador (Go, client_golang v1.24.1) | – | http://localhost:8088/metrics |

- `scrape_interval: 5s`. Native histograms habilitados (`scrape_native_histograms: true`) **e** classic buckets mantidos (`always_scrape_classic_histograms: true`).
- Feature flags: `promql-experimental-functions,st-storage,use-start-timestamps`.
- O gerador é um único binário Go. No build, cada `functions/<fn>/setup/scenario.go` é copiado para o mesmo `package main`.

## Regras do `scenario.go`

1. `package main`, e **nenhum identificador top-level**: só `func init() { Register(Scenario{...}) }`. Tudo mais fica dentro de closures, para não colidir com outras lições.
2. `Scenario.Name` = nome da pasta.
3. **Nomes de métricas começam com `<fn>_`** (ex.: `abs_temperature_offset_celsius`, `histogram_quantile_http_request_duration_seconds`). Única exceção: `target_info` (lição `info`).
4. Helpers disponíveis (ver `generator/lab.go`):
   - `NewFunc(name, help, prometheus.GaugeValue|CounterValue, labelNames, func(now time.Time) []Sample)`: valor **calculado na hora do scrape**. Ideal para ondas, rampas, resets programados e séries que somem (retorne `nil`). Prefira basear no **relógio de parede** (`now.Unix()`) e não no uptime, para que um restart do gerador não bagunce a lição.
   - `Sample{Labels, Value, Created}`. `Created` define o start timestamp de counters.
   - `Wave(t, period, base, amp, phase)`, `Saw(t, period)`, `Square(t, period)`, `Noise(amp)`, `Every(d, fn)`, `Elapsed()`, `StartTime`.
   - Também pode usar `prometheus.NewCounterVec`, `NewGaugeVec` e `NewHistogramVec` normais com goroutines (`Every`).
   - Native histogram: `prometheus.HistogramOpts{NativeHistogramBucketFactor: 1.1, Buckets: []float64{...}}` expõe **native + classic** ao mesmo tempo.
5. **Não use os labels `job`, `instance` nem `env`** nas suas métricas: o scrape já adiciona esses labels, e os seus viram `exported_*` (`honor_labels: false`). Use `host`, `pod`, `node`, `environment`...
   - **Counters: prefira valor calculado pelo relógio de parede** (ex.: "desde a meia-noite UTC") a `Every()`+`Add()`: o gerador é rebuildado a cada deploy e counters em memória voltam a zero.
   - **`Sample.Created` precisa ser estável** (mesmo valor exato entre scrapes). Com `st-storage`, um start timestamp que oscila gera o warning `sample has start time that overlaps`.
   - **Evite o sufixo `_info`** em métricas que não são info metrics: `info()` com matcher negado de `__name__` considera todas as `.+_info`.
6. Os padrões devem aparecer **rápido**: períodos de 1 a 5 min, para a lição ficar visível em ~5 a 10 min.
7. Um cenário pode ter vários "cases" (várias métricas) quando a função tem nuances.

## Regras do `lab.yaml`

Formato documentado no topo de `tools/gen_dashboards.py`. Categorias (pastas do Grafana):

```
01 - Matemática
02 - Counters (rate, increase...)
03 - Gauges (delta, deriv, previsão)
04 - Histogramas
05 - Agregação no tempo (_over_time)
06 - Ausência de dados
07 - Trigonometria
08 - Data e hora
09 - Labels e ordenação
10 - Tipos e conversões
```

- Painéis numerados ("1. ...", "2. ...") batendo com as seções do README.
- Sempre que possível, mostre **lado a lado** o dado cru e o resultado da função.
- `validate:` por query: `nonempty` (padrão), `empty` (ex.: `absent()` quando a série existe) ou `any`.

## Regras do `README.md` (em português)

Mesma estrutura de `functions/rate/README.md`:
título + "Em uma frase" · tabela (assinatura, tipo de métrica, unidade, dashboard, cenário) · 🧠 Analogia · 🔧 Setup (tabela das métricas fake + `curl`) · ▶️ Como rodar · 🔍 Queries passo a passo (query, **o que faz**, **resultado esperado** com números concretos) · ✅ Quando usar (casos reais de produção) · ❌ Quando NÃO usar (alternativas com links `../<fn>/`) · ⚠️ Pegadinhas · 🔗 Relacionadas · 📚 Referência (link `https://prometheus.io/docs/prometheus/latest/querying/functions/#<fn>`).

## 🎓 Foco: certificação PCA (Prometheus Certified Associate)

O objetivo do lab é **estudar para a PCA** (Linux Foundation / CNCF). Todo README deve ter também, **além** das seções acima:

- **🏭 Casos reais**: 2 a 4 cenários de produção **concretos** (ex.: "SRE do e-commerce na Black Friday", "alerta de disco do node_exporter", "SLO de latência do checkout"), usando **métricas reais conhecidas** (`node_cpu_seconds_total`, `node_filesystem_avail_bytes`, `http_requests_total`, `container_memory_working_set_bytes`, `kube_pod_container_status_restarts_total`, `up`, `process_start_time_seconds`, `apiserver_request_duration_seconds_bucket`...). Mostre a query real, **a regra de alerta ou recording rule em YAML** quando fizer sentido, e explique a decisão.
  O cenário fake deve **imitar** esses casos com nomes prefixados (ex.: `predict_linear_node_filesystem_avail_bytes`).
- **🎓 Na prova PCA**: o que costuma cair sobre essa função (tipo de métrica correto, range vs instant vector, retorno, armadilhas clássicas), com **3 a 5 perguntas estilo prova** (múltipla escolha A/B/C/D), **resposta e justificativa** num `<details><summary>Resposta</summary>...</details>`.
- **📝 Cola rápida**: bloco final com 3 a 5 bullets para revisar na véspera.

A estrutura final do README fica: título + "Em uma frase" · tabela · 🧠 Analogia · 🔧 Setup · ▶️ Como rodar · 🔍 Queries passo a passo · 🏭 Casos reais · ✅ Quando usar · ❌ Quando NÃO usar · ⚠️ Pegadinhas · 🎓 Na prova PCA · 📝 Cola rápida · 🔗 Relacionadas · 📚 Referência.

## 📏 Mínimo de casos por lição (checklist)

O usuário quer **casos suficientes para entender bem**. Toda lição precisa ter:

- [ ] **≥ 3 cases progressivos** no cenário e no dashboard: 🟢 básico (o comportamento puro) → 🟡 intermediário (nuance, pegadinha ou comparação com a função "vizinha") → 🔴 caso real de produção.
- [ ] **≥ 5 queries** explicadas no README, cada uma com **resultado esperado numérico** batendo com o que o dashboard mostra.
- [ ] **≥ 1 comparação lado a lado** (cru vs função, ou função vs alternativa, ex.: `rate` vs `irate`, `avg` vs `avg_over_time`).
- [ ] **≥ 1 exemplo do que dá errado** (uso incorreto) mostrado no dashboard ou em query.
- [ ] **≥ 2 casos reais** com YAML de alerta ou recording rule.
- [ ] **≥ 3 perguntas PCA** (trigonometria: ≥ 2).

## Fluxo de trabalho

```bash
tools/check-go.sh <fn>                      # compila núcleo + seu cenário
tools/deploy.sh                             # (flock) gera dashboards + rebuilda o gerador + sobe
python3 tools/validate.py <fn>              # testa queries + tira print (.lab-status/<fn>.png)
python3 tools/validate.py <fn> --notify     # idem + manda print no Discord
```

- **Nunca** rode `docker compose down`, nunca reinicie o Prometheus e nunca apague volumes. Só use `tools/deploy.sh`.
- **Não edite** arquivos compartilhados (`generator/*.go`, `tools/*`, `docker-compose.yml`, `prometheus/`, `grafana/`). Se precisar, descreva a mudança no relatório final.
- Depois do deploy, espere dados acumularem, por exemplo:
  `timeout 600 bash -c 'until python3 tools/validate.py <fn> >/dev/null 2>&1; do sleep 15; done'`
- **Olhe o print** (`.lab-status/<fn>.png`) antes de notificar: o dashboard precisa mostrar o comportamento que o README promete.
