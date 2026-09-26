# 🔥 PromQL Functions Lab

Laboratório **prático** para aprender **todas** as funções PromQL do Prometheus, uma por uma, com foco na certificação **PCA (Prometheus Certified Associate)**.

> 🎓 **Estudando para a PCA?** Comece pelo [**PCA.md**](PCA.md): domínios da prova, trilha de 7 dias, top 15 pegadinhas e alertas de casos reais rodando ao vivo.

Cada função tem sua própria pasta em [`functions/`](functions/) com:

- 📘 **`README.md`**: a aula, com analogia, assinatura, queries explicadas passo a passo, **resultado esperado**, quando usar e quando **não** usar, e pegadinhas.
- 🧪 **`setup/scenario.go`**: um gerador de **métricas fake** desenhado para aquela função (ondas, picos, resets, séries que somem, histogramas...).
- 🏭 **Casos reais** de produção (node_exporter, kube-state-metrics, SLOs), com regras de alerta em YAML.
- 🎓 **Quiz estilo PCA** com respostas comentadas e uma **📝 cola rápida** por função.
- 📊 **`lab.yaml`**: os painéis do **dashboard Grafana** dessa função, que também servem de casos de teste.

## Stack (mesmas versões para tudo)

| Componente | Versão | URL |
|---|---|---|
| Prometheus | **v3.15.0** | http://localhost:9095 |
| Grafana | **13.2.2** | http://localhost:3300 (login anônimo como admin) |
| Gerador de métricas (Go + client_golang v1.24.1) | – | http://localhost:8088/metrics |

Configuração relevante do Prometheus ([`prometheus/prometheus.yml`](prometheus/prometheus.yml)):
- `scrape_interval: 5s`, para as lições "andarem" rápido;
- **native histograms** + classic histograms ao mesmo tempo;
- `--enable-feature=promql-experimental-functions,st-storage,use-start-timestamps`, que habilita as funções experimentais (`info`, `sort_by_label`, `double_exponential_smoothing`, `mad_over_time`, `ts_of_*`, `start/end/range/step`, `min_of/max_of`) e `start_timestamp()`.

## 🚀 Subindo

```bash
tools/deploy.sh          # gera dashboards, builda o gerador com todos os cenários e sobe tudo
# ou, se os dashboards já estiverem gerados:
docker compose up -d --build
```

Abra o Grafana em http://localhost:3300 → **Dashboards** → pastas `01 - Matemática` … `10 - Tipos e conversões`.
Cada dashboard tem uid `fn-<função>` (ex.: http://localhost:3300/d/fn-rate).

> Espere **5 a 10 minutos** depois de subir: várias lições usam janelas de `[5m]` e cenários com ciclos de alguns minutos.

## 🧭 Estrutura

```
.
├── docker-compose.yml         # prometheus + grafana + generator
├── .env                       # versões fixas (PROMETHEUS_VERSION, GRAFANA_VERSION)
├── prometheus/prometheus.yml
├── grafana/provisioning/      # datasource + provider de dashboards
├── generator/                 # núcleo do gerador Go (helpers: NewFunc, Wave, Saw, Every...)
├── functions/<fn>/            # 1 pasta por função: README.md, lab.yaml, setup/scenario.go
├── dashboards/                # GERADO a partir dos lab.yaml (+ cópia de static-dashboards/)
├── static-dashboards/         # dashboards feitos à mão (ex.: "Meu progresso PCA" dos challenges/)
└── tools/
    ├── deploy.sh              # gera dashboards + rebuilda gerador + sobe
    ├── check-go.sh <fn>       # compila núcleo + cenário
    ├── gen_dashboards.py      # lab.yaml -> dashboard Grafana
    ├── validate.py <fn>       # roda todas as queries da lição + screenshot (+ --notify Discord)
    └── gen_index.py           # atualiza o índice abaixo
```

Quer criar/editar uma lição? Veja [CONTRIBUTING.md](CONTRIBUTING.md).

## 🧪 Rodando só algumas lições

O gerador aceita `LAB_SCENARIOS=rate,increase` (variável de ambiente) para expor só essas métricas.

## 📚 Índice de funções

<!-- INDEX:START -->
_90 funções. ✅ = testada e validada._

### 01 - Matemática

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`abs()`](functions/abs/) | tira o sinal: o tamanho do desvio, pra cima ou pra baixo | [dashboard](http://localhost:3300/d/fn-abs) |
| ✅ | [`ceil()`](functions/ceil/) | arredonda PRA CIMA: quantos pods/GB eu preciso? | [dashboard](http://localhost:3300/d/fn-ceil) |
| ✅ | [`clamp()`](functions/clamp/) | prende o valor numa faixa [min, max] | [dashboard](http://localhost:3300/d/fn-clamp) |
| ✅ | [`clamp_max()`](functions/clamp_max/) | teto: nada fica acima do máximo | [dashboard](http://localhost:3300/d/fn-clamp_max) |
| ✅ | [`clamp_min()`](functions/clamp_min/) | piso: nada fica abaixo do mínimo | [dashboard](http://localhost:3300/d/fn-clamp_min) |
| ✅ | [`exp()`](functions/exp/) | e elevado a x: crescimento composto e 'desfazer' o logaritmo | [dashboard](http://localhost:3300/d/fn-exp) |
| ✅ | [`floor()`](functions/floor/) | arredonda PRA BAIXO: quantos lotes CHEIOS / minutos COMPLETOS? | [dashboard](http://localhost:3300/d/fn-floor) |
| ✅ | [`ln()`](functions/ln/) | logaritmo natural: taxa de crescimento, tempo de dobra e média geométrica | [dashboard](http://localhost:3300/d/fn-ln) |
| ✅ | [`log10()`](functions/log10/) | logaritmo base 10: ORDEM DE GRANDEZA (µs, ms, s) e decibéis | [dashboard](http://localhost:3300/d/fn-log10) |
| ✅ | [`log2()`](functions/log2/) | logaritmo base 2: quantas DOBRAS? qual a próxima potência de 2? | [dashboard](http://localhost:3300/d/fn-log2) |
| ✅ | [`round()`](functions/round/) | inteiro mais próximo, ou múltiplo mais próximo (to_nearest) | [dashboard](http://localhost:3300/d/fn-round) |
| ✅ | [`sgn()`](functions/sgn/) | só o sinal: +1, 0 ou -1 (subindo, parado, descendo) | [dashboard](http://localhost:3300/d/fn-sgn) |
| ✅ | [`sqrt()`](functions/sqrt/) | raiz quadrada: de variância para desvio padrão, RMS e magnitude | [dashboard](http://localhost:3300/d/fn-sqrt) |

### 02 - Counters (rate, increase...)

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`changes()`](functions/changes/) | quantas vezes o valor mudou | [dashboard](http://localhost:3300/d/fn-changes) |
| ✅ | [`increase()`](functions/increase/) | quanto um counter cresceu na janela | [dashboard](http://localhost:3300/d/fn-increase) |
| ✅ | [`irate()`](functions/irate/) | velocidade instantânea de um counter | [dashboard](http://localhost:3300/d/fn-irate) |
| ✅ | [`rate()`](functions/rate/) | velocidade média de um counter | [dashboard](http://localhost:3300/d/fn-rate) |
| ✅ | [`resets()`](functions/resets/) | quantas vezes um counter voltou para trás | [dashboard](http://localhost:3300/d/fn-resets) |

### 03 - Gauges (delta, deriv, previsão)

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`delta()`](functions/delta/) | diferença entre o começo e o fim da janela (gauges) | [dashboard](http://localhost:3300/d/fn-delta) |
| ✅ | [`deriv()`](functions/deriv/) | inclinação por segundo de um gauge (regressão linear) | [dashboard](http://localhost:3300/d/fn-deriv) |
| ✅ | [`double_exponential_smoothing()`](functions/double_exponential_smoothing/) | suavizar ruído sem perder a tendência | [dashboard](http://localhost:3300/d/fn-double_exponential_smoothing) |
| ✅ | [`idelta()`](functions/idelta/) | diferença entre as 2 últimas amostras (gauges) | [dashboard](http://localhost:3300/d/fn-idelta) |
| ✅ | [`predict_linear()`](functions/predict_linear/) | onde o gauge vai estar daqui a N segundos | [dashboard](http://localhost:3300/d/fn-predict_linear) |

### 04 - Histogramas

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`histogram_avg()`](functions/histogram_avg/) | a média de um native histogram | [dashboard](http://localhost:3300/d/fn-histogram_avg) |
| ✅ | [`histogram_count()`](functions/histogram_count/) | quantas observações tem o native histogram | [dashboard](http://localhost:3300/d/fn-histogram_count) |
| ✅ | [`histogram_fraction()`](functions/histogram_fraction/) | que fração ficou entre dois valores (SLO, Apdex) | [dashboard](http://localhost:3300/d/fn-histogram_fraction) |
| ✅ | [`histogram_quantile()`](functions/histogram_quantile/) | percentis (p50, p90, p99) a partir de histogramas | [dashboard](http://localhost:3300/d/fn-histogram_quantile) |
| ✅ | [`histogram_quantiles()`](functions/histogram_quantiles/) | vários percentis numa query só (experimental) | [dashboard](http://localhost:3300/d/fn-histogram_quantiles) |
| ✅ | [`histogram_stddev()`](functions/histogram_stddev/) | o quanto os valores se espalham em volta da média | [dashboard](http://localhost:3300/d/fn-histogram_stddev) |
| ✅ | [`histogram_stdvar()`](functions/histogram_stdvar/) | a variância (desvio padrão ao quadrado) | [dashboard](http://localhost:3300/d/fn-histogram_stdvar) |
| ✅ | [`histogram_sum()`](functions/histogram_sum/) | a soma dos valores observados | [dashboard](http://localhost:3300/d/fn-histogram_sum) |

### 05 - Agregação no tempo (_over_time)

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`avg_over_time()`](functions/avg_over_time/) | média de CADA série ao longo do tempo | [dashboard](http://localhost:3300/d/fn-avg_over_time) |
| ✅ | [`count_over_time()`](functions/count_over_time/) | quantas amostras cada série tem na janela | [dashboard](http://localhost:3300/d/fn-count_over_time) |
| ✅ | [`first_over_time()`](functions/first_over_time/) | a primeira amostra DENTRO da janela | [dashboard](http://localhost:3300/d/fn-first_over_time) |
| ✅ | [`last_over_time()`](functions/last_over_time/) | o último valor visto na janela | [dashboard](http://localhost:3300/d/fn-last_over_time) |
| ✅ | [`mad_over_time()`](functions/mad_over_time/) | dispersão robusta a outliers (experimental) | [dashboard](http://localhost:3300/d/fn-mad_over_time) |
| ✅ | [`max_over_time()`](functions/max_over_time/) | o pico de cada série na janela | [dashboard](http://localhost:3300/d/fn-max_over_time) |
| ✅ | [`min_over_time()`](functions/min_over_time/) | o vale de cada série na janela | [dashboard](http://localhost:3300/d/fn-min_over_time) |
| ✅ | [`present_over_time()`](functions/present_over_time/) | a série apareceu na janela? (sempre 1) | [dashboard](http://localhost:3300/d/fn-present_over_time) |
| ✅ | [`quantile_over_time()`](functions/quantile_over_time/) | percentil de um gauge ao longo do tempo | [dashboard](http://localhost:3300/d/fn-quantile_over_time) |
| ✅ | [`stddev_over_time()`](functions/stddev_over_time/) | o quanto um gauge 'balança' na janela | [dashboard](http://localhost:3300/d/fn-stddev_over_time) |
| ✅ | [`stdvar_over_time()`](functions/stdvar_over_time/) | variância (desvio padrão ao quadrado) | [dashboard](http://localhost:3300/d/fn-stdvar_over_time) |
| ✅ | [`sum_over_time()`](functions/sum_over_time/) | somar as amostras de cada série na janela | [dashboard](http://localhost:3300/d/fn-sum_over_time) |
| ✅ | [`ts_of_first_over_time()`](functions/ts_of_first_over_time/) | quando a série apareceu na janela (experimental) | [dashboard](http://localhost:3300/d/fn-ts_of_first_over_time) |
| ✅ | [`ts_of_last_over_time()`](functions/ts_of_last_over_time/) | quando foi a última amostra? (experimental) | [dashboard](http://localhost:3300/d/fn-ts_of_last_over_time) |
| ✅ | [`ts_of_max_over_time()`](functions/ts_of_max_over_time/) | QUANDO aconteceu o pico (experimental) | [dashboard](http://localhost:3300/d/fn-ts_of_max_over_time) |
| ✅ | [`ts_of_min_over_time()`](functions/ts_of_min_over_time/) | QUANDO aconteceu o mínimo (experimental) | [dashboard](http://localhost:3300/d/fn-ts_of_min_over_time) |

### 06 - Ausência de dados

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`absent()`](functions/absent/) | alarme de 'a métrica sumiu' | [dashboard](http://localhost:3300/d/fn-absent) |
| ✅ | [`absent_over_time()`](functions/absent_over_time/) | 'nenhum dado nos últimos N minutos' | [dashboard](http://localhost:3300/d/fn-absent_over_time) |

### 07 - Trigonometria

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`acos()`](functions/acos/) | arco-cosseno: o ângulo da escada | [dashboard](http://localhost:3300/d/fn-acos) |
| ✅ | [`acosh()`](functions/acosh/) | inversa do cosh: domínio x ≥ 1 | [dashboard](http://localhost:3300/d/fn-acosh) |
| ✅ | [`asin()`](functions/asin/) | arco-seno: do sensor de volta ao ângulo | [dashboard](http://localhost:3300/d/fn-asin) |
| ✅ | [`asinh()`](functions/asinh/) | o 'log simétrico' que aceita zero e negativos | [dashboard](http://localhost:3300/d/fn-asinh) |
| ✅ | [`atan()`](functions/atan/) | arco-tangente: ângulo de subida e direção | [dashboard](http://localhost:3300/d/fn-atan) |
| ✅ | [`atanh()`](functions/atanh/) | inversa do tanh: domínio (-1, 1) | [dashboard](http://localhost:3300/d/fn-atanh) |
| ✅ | [`cos()`](functions/cos/) | cosseno (em radianos!) | [dashboard](http://localhost:3300/d/fn-cos) |
| ✅ | [`cosh()`](functions/cosh/) | cosseno hiperbólico: a curva do cabo pendurado | [dashboard](http://localhost:3300/d/fn-cosh) |
| ✅ | [`deg()`](functions/deg/) | de radianos para graus | [dashboard](http://localhost:3300/d/fn-deg) |
| ✅ | [`pi()`](functions/pi/) | a constante π | [dashboard](http://localhost:3300/d/fn-pi) |
| ✅ | [`rad()`](functions/rad/) | de graus para radianos | [dashboard](http://localhost:3300/d/fn-rad) |
| ✅ | [`sin()`](functions/sin/) | seno (em radianos!) | [dashboard](http://localhost:3300/d/fn-sin) |
| ✅ | [`sinh()`](functions/sinh/) | seno hiperbólico: o amplificador simétrico | [dashboard](http://localhost:3300/d/fn-sinh) |
| ✅ | [`tan()`](functions/tan/) | tangente: inclinação e assíntotas | [dashboard](http://localhost:3300/d/fn-tan) |
| ✅ | [`tanh()`](functions/tanh/) | tangente hiperbólica: o 'clamp' suave | [dashboard](http://localhost:3300/d/fn-tanh) |

### 08 - Data e hora

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`day_of_month()`](functions/day_of_month/) | que dia do mês é (1 a 31, em UTC) | [dashboard](http://localhost:3300/d/fn-day_of_month) |
| ✅ | [`day_of_week()`](functions/day_of_week/) | dia da semana (0 = domingo … 6 = sábado, UTC) | [dashboard](http://localhost:3300/d/fn-day_of_week) |
| ✅ | [`day_of_year()`](functions/day_of_year/) | dia do ano (1 a 365/366, UTC) | [dashboard](http://localhost:3300/d/fn-day_of_year) |
| ✅ | [`days_in_month()`](functions/days_in_month/) | quantos dias tem o mês (28 a 31, UTC) | [dashboard](http://localhost:3300/d/fn-days_in_month) |
| ✅ | [`end()`](functions/end/) | o fim da janela do gráfico (experimental) | [dashboard](http://localhost:3300/d/fn-end) |
| ✅ | [`hour()`](functions/hour/) | hora do dia (0 a 23, em UTC!) | [dashboard](http://localhost:3300/d/fn-hour) |
| ✅ | [`minute()`](functions/minute/) | minuto da hora (0 a 59) | [dashboard](http://localhost:3300/d/fn-minute) |
| ✅ | [`month()`](functions/month/) | mês do ano (1 = janeiro … 12 = dezembro, UTC) | [dashboard](http://localhost:3300/d/fn-month) |
| ✅ | [`range()`](functions/range/) | a duração da janela do gráfico (experimental) | [dashboard](http://localhost:3300/d/fn-range) |
| ✅ | [`start()`](functions/start/) | o início da janela do gráfico (experimental) | [dashboard](http://localhost:3300/d/fn-start) |
| ✅ | [`start_timestamp()`](functions/start_timestamp/) | quando o counter nasceu (created timestamp) | [dashboard](http://localhost:3300/d/fn-start_timestamp) |
| ✅ | [`step()`](functions/step/) | a distância entre pontos do gráfico (experimental) | [dashboard](http://localhost:3300/d/fn-step) |
| ✅ | [`time()`](functions/time/) | o relógio da consulta (segundos desde 1970, UTC) | [dashboard](http://localhost:3300/d/fn-time) |
| ✅ | [`timestamp()`](functions/timestamp/) | QUANDO a amostra foi coletada | [dashboard](http://localhost:3300/d/fn-timestamp) |
| ✅ | [`year()`](functions/year/) | o ano (UTC) de um timestamp | [dashboard](http://localhost:3300/d/fn-year) |

### 09 - Labels e ordenação

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`info()`](functions/info/) | enriquecer séries com labels de target_info (sem group_left) | [dashboard](http://localhost:3300/d/fn-info) |
| ✅ | [`label_join()`](functions/label_join/) | juntar vários labels em um só | [dashboard](http://localhost:3300/d/fn-label_join) |
| ✅ | [`label_replace()`](functions/label_replace/) | criar/reescrever um label com regex | [dashboard](http://localhost:3300/d/fn-label_replace) |
| ✅ | [`sort()`](functions/sort/) | ordenar pelo valor (crescente) | [dashboard](http://localhost:3300/d/fn-sort) |
| ✅ | [`sort_by_label()`](functions/sort_by_label/) | ordenar pelo NOME (label), em ordem natural | [dashboard](http://localhost:3300/d/fn-sort_by_label) |
| ✅ | [`sort_by_label_desc()`](functions/sort_by_label_desc/) | ordenar pelo NOME (label), decrescente | [dashboard](http://localhost:3300/d/fn-sort_by_label_desc) |
| ✅ | [`sort_desc()`](functions/sort_desc/) | ordenar pelo valor (decrescente): o "Top N" | [dashboard](http://localhost:3300/d/fn-sort_desc) |

### 10 - Tipos e conversões

| | Função | O que ensina | Grafana |
|---|---|---|---|
| ✅ | [`max_of()`](functions/max_of/) | o maior de dois escalares (um "piso" dinâmico) | [dashboard](http://localhost:3300/d/fn-max_of) |
| ✅ | [`min_of()`](functions/min_of/) | o menor de dois escalares (um "teto" dinâmico) | [dashboard](http://localhost:3300/d/fn-min_of) |
| ✅ | [`scalar()`](functions/scalar/) | transformar um vetor de 1 elemento em número puro | [dashboard](http://localhost:3300/d/fn-scalar) |
| ✅ | [`vector()`](functions/vector/) | transformar um número em um vetor (o famoso "or vector(0)") | [dashboard](http://localhost:3300/d/fn-vector) |

<!-- INDEX:END -->

## 🗺️ Trilha sugerida para iniciantes

1. **Counters:** `rate` → `increase` → `irate` → `resets`
2. **Gauges:** `delta` → `deriv` → `predict_linear`
3. **Agregação no tempo:** `avg_over_time` → `max_over_time` → `quantile_over_time`
4. **Histogramas:** `histogram_quantile` → `histogram_fraction` → `histogram_avg`
5. **Alertas de ausência:** `absent` → `absent_over_time`
6. **Labels:** `label_replace` → `label_join` → `info`
7. **Tempo:** `time` → `timestamp` → `hour`/`day_of_week`
8. O resto (matemática, trigonometria, ordenação) conforme a curiosidade.
