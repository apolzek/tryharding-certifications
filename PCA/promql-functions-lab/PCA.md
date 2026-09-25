# 🎓 Guia de estudo: PCA (Prometheus Certified Associate)

Este lab foi montado para estudar para a **PCA** da Linux Foundation / CNCF.
Confira sempre a página oficial, porque domínios e pesos podem mudar:
https://training.linuxfoundation.org/certification/prometheus-certified-associate/

## Formato (referência)

- Prova **online, supervisionada**, de **múltipla escolha**, com cerca de **90 minutos**.
- Nível **associate**: conceitual e prático, sem configuração em terminal ao vivo.

## Domínios e onde estudar neste lab

| Domínio (peso aprox.) | O que cai | Onde praticar aqui |
|---|---|---|
| **Observability Concepts** (~18%) | métricas vs logs vs traces, push vs pull, service discovery, SLO/SLI/SLA | [`rate`](functions/rate/) (RED), [`histogram_quantile`](functions/histogram_quantile/) (SLO), [`absent`](functions/absent/) |
| **Prometheus Fundamentals** (~20%) | arquitetura, TSDB, config de scrape, labels, staleness, tipos de dado (instant/range/scalar/string) | [`timestamp`](functions/timestamp/), [`vector`](functions/vector/), [`scalar`](functions/scalar/), [`present_over_time`](functions/present_over_time/) |
| **PromQL** (~28%, o maior!) | seletores, operadores, agregações, **funções**, subqueries, `offset`, `@`, vector matching | **todas as pastas de `functions/`** |
| **Instrumentation & Exporters** (~16%) | tipos de métrica (counter, gauge, histogram, summary), convenções de nome (`_total`, `_seconds`, `_bytes`), client libraries, exporters | [`rate`](functions/rate/), [`increase`](functions/increase/), [`resets`](functions/resets/), [`deriv`](functions/deriv/), `histogram_*` |
| **Alerting & Dashboarding** (~18%) | regras de alerta (`for`, labels, annotations), recording rules, Alertmanager (routing, grouping, inhibition, silences), Grafana | seções **🏭 Casos reais** de cada README, [`predict_linear`](functions/predict_linear/), [`absent_over_time`](functions/absent_over_time/), [`hour`](functions/hour/) |

## 🗺️ Trilha de 7 dias

| Dia | Tema | Lições |
|---|---|---|
| 1 | Counters | `rate` · `irate` · `increase` · `resets` |
| 2 | Gauges e previsão | `delta` · `idelta` · `deriv` · `predict_linear` · `changes` |
| 3 | Histogramas e SLO | `histogram_quantile` · `histogram_fraction` · `histogram_count/sum/avg` |
| 4 | Agregação no tempo | `avg/min/max/sum/count/quantile_over_time` · `last_over_time` · `stddev_over_time` |
| 5 | Ausência de dados e alertas | `absent` · `absent_over_time` · `present_over_time` · `time`/`timestamp` |
| 6 | Labels e joins | `label_replace` · `label_join` · `info` · `sort*` · `vector` · `scalar` |
| 7 | Revisão | ler as **📝 Colas rápidas** de todas as lições e refazer os quizzes 🎓 |

## ⚡ Top 15 pegadinhas que mais caem

1. `rate`/`increase`/`irate`/`resets` → **counters**. `delta`/`deriv`/`predict_linear`/`idelta` → **gauges**.
2. Funções `*_over_time`, `rate` etc. recebem **range vector** (`x[5m]`). Matemáticas (`abs`, `ceil`...) recebem **instant vector**.
3. **Primeiro `rate`, depois `sum`**, nunca o contrário.
4. `histogram_quantile` em histograma **clássico** precisa de `le` no `by`: `sum by (le) (rate(x_bucket[5m]))`.
5. `histogram_quantile` retorna um valor **estimado** (interpolação linear dentro do bucket).
6. `absent()` retorna **vazio** quando a série existe e **1** quando não existe. Serve para alertar "métrica sumiu".
7. `irate` usa só as **2 últimas amostras**, e `rate` faz a média da janela. Para alertas, use `rate`.
8. `increase` pode dar número **fracionado** por causa da extrapolação.
9. Funções de data (`hour`, `day_of_week`...) são em **UTC**.
10. `sort`/`sort_desc` só afetam **instant queries**.
11. `scalar()` retorna **NaN** se o vetor não tiver **exatamente 1** elemento.
12. `label_replace` **não** altera a série quando a regex não casa (retorna a série inalterada).
13. `or vector(0)` evita "No data" em painéis e alertas.
14. Janela do `rate` ≥ **4× scrape_interval**.
15. Quase todas as funções **removem o nome da métrica** (`__name__`) do resultado. `last_over_time`, `sort`, `label_*` preservam.

## 🔔 Alertas de casos reais rodando no lab

Veja [`prometheus/rules/`](prometheus/rules/): regras de alerta e recording rules **reais** (padrões do kube-prometheus / node-mixin), adaptadas às métricas fake. Abra http://localhost:9095/alerts para vê-las em `pending` e `firing` ao vivo.
