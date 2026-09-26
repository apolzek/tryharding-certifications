# 🗓️ Plano de estudo PCA: 4 semanas

Trilha diária que liga **lição → desafios → lab → gameday → simulado**, com o Anki todo dia.
São cerca de **1h30 a 2h por dia útil** e **3h no sábado**, e o domingo fica livre (ou para colocar o atraso em dia).
Marque os `[ ]` conforme avança: o GitHub e a maioria dos editores deixam clicar nos checkboxes.

| Semana | Foco | Domínios (peso) | Horas |
|---|---|---|---|
| 1 | PromQL I: tipos, seletores, operadores, counters, gauges | PromQL (28%) | ~11 h |
| 2 | PromQL II + fundamentos: agregação, `_over_time`, histogramas, SD/relabel, TSDB, federation | PromQL · Fundamentals (20%) | ~12 h |
| 3 | Instrumentação, exporters, regras e Alertmanager + **1º simulado** | Instrumentation (16%) · Alerting (18%) | ~12 h |
| 4 | Observabilidade/SLO, gamedays, **simulados** e revisão | Observability (18%) · todos | ~11 h |
| ⭐ | [Semana da prova](#-semana-da-prova) | revisão | ~6 h |

## 🧰 Materiais (onde fica cada coisa)

| O quê | Caminho | Como usar |
|---|---|---|
| Lições das 90 funções | [`promql-functions-lab/functions/<fn>/`](promql-functions-lab/functions/) | `cd promql-functions-lab && docker compose up -d --build` → Prometheus :9095, Grafana :3300 |
| Guia da prova + Top 15 | [`promql-functions-lab/PCA.md`](promql-functions-lab/PCA.md) | leitura |
| Desafios PromQL (corretor) | [`challenges/`](challenges/) | `./check.py list --topic counters` · `./check.py show 020` · `./check.py 020 'sua query'` · `./check.py hint 020` · `./check.py progress` (precisa do promql-functions-lab rodando) |
| Labs | [`labs/<nome>/`](labs/) | leia o README, `docker compose up -d`, faça `exercises/`; `./test.sh` confere (use `KEEP=1` para manter a stack de pé) |
| Gamedays (incidentes) | [`gamedays/`](gamedays/) | `./start.sh 01` → investigue → `./check.sh 01` → `./stop.sh` (travou? `./solve.sh 01`) |
| Simulados | [`exam/`](exam/) | `exam/index.html` no navegador, ou no terminal: `./exam.py simulado` (60 questões nos pesos oficiais, 90 min) · `./exam.py treino --domain PromQL` (feedback imediato) · `./exam.py stats` |
| Flashcards | [`flashcards/`](flashcards/) | importe `dist/pca-flashcards.apkg` no Anki ([como fazer](flashcards/README.md)) · offline: `flashcards/flashcards.html` |
| Mapa mental | [`mindmap/index.html`](mindmap/index.html) | visão geral com links para tudo; boa para começar e fechar cada semana |

## 🔁 Rotina de todo dia (20-25 min, antes do resto)

- **Anki**: zere as **revisões** e depois faça os **novos** do subdeck da semana. Limite de novos: **40/dia** nas semanas 1-3, **20/dia** na semana 4 e **0** na semana da prova. Ligue o FSRS com retenção 0,90.
- Errou um card 3 vezes? Abra o link do verso e releie a seção da lição.
- Ao terminar o dia, anote em 1 linha o que ficou confuso. Esse é o assunto da revisão de sábado.

> Ordem dos subdecks: **Curadas** e **Top 15** primeiro (alto rendimento), depois **Labs**, e por último as **Lições PromQL** (~900 cards; não precisa zerar antes da prova).

---

## Dia 0: preparação (1 h)

- [ ] Docker funcionando; `cd promql-functions-lab && docker compose up -d --build` e abrir http://localhost:9095 e http://localhost:3300 (15 min)
- [ ] `cd challenges && ./check.py show 000 && ./check.py 000 'rate(rate_http_requests_total[1m])'` para testar o corretor (5 min)
- [ ] Anki instalado; importar `flashcards/dist/pca-flashcards.apkg`; configurar novos/dia = 40 e FSRS (15 min)
- [ ] Ler [`PCA.md`](promql-functions-lab/PCA.md) (domínios e Top 15) e passear pelo [mapa mental](mindmap/index.html) (20 min)
- [ ] Marcar a data da prova no calendário, cerca de 4 semanas e meia à frente

---

## Semana 1: PromQL I (tipos, operadores, counters, gauges)

Anki: `PCA::03 PromQL::Curadas` → `Top 15 pegadinhas` → `Lições PromQL` (tag `lesson::rate`, `lesson::increase`...).

### Dia 1 (seg): tipos de dado e seletores (1h45)
- [ ] Anki (20 min)
- [ ] Ler [`labs/promql-operators`](labs/promql-operators/) até **Seletores** e **Tempo** (`offset`, `@`, subquery) (35 min)
- [ ] Lições [`vector`](promql-functions-lab/functions/vector/), [`scalar`](promql-functions-lab/functions/scalar/), [`timestamp`](promql-functions-lab/functions/timestamp/) (20 min)
- [ ] Desafios **001-013** (`./check.py list --topic selectors-and-types`) (30 min)

### Dia 2 (ter): operadores, precedência e `bool` (1h45)
- [ ] Anki (20 min)
- [ ] Lab promql-operators: aritmética, comparação/`bool`, lógicos + exercícios `01-seletores-regex`, `02-offset-subquery`, `03-bool-e-nan` (45 min)
- [ ] Desafios **200-229** do tópico `operators` (seletores, subquery, precedência, `and`/`or`/`unless`): faça pelo menos os *basic* e *intermediate* (40 min)

### Dia 3 (qua): vector matching (1h45)
- [ ] Anki (20 min)
- [ ] Lab promql-operators: **Matching** (`on`/`ignoring`, `group_left`) + exercícios `04-procv-group-left`, `05-many-to-many` (40 min)
- [ ] Lição [`info`](promql-functions-lab/functions/info/) (join com `target_info`) (15 min)
- [ ] Desafios **230-238** (one-to-one, `group_left`, "PROCV") (30 min)

### Dia 4 (qui): counters (1h45)
- [ ] Anki (20 min)
- [ ] Lições [`rate`](promql-functions-lab/functions/rate/), [`irate`](promql-functions-lab/functions/irate/), [`increase`](promql-functions-lab/functions/increase/), [`resets`](promql-functions-lab/functions/resets/), com as queries rodando no Grafana (50 min)
- [ ] Desafios **020-030** (`--topic counters`) (35 min)

### Dia 5 (sex): gauges e previsão (1h30)
- [ ] Anki (20 min)
- [ ] Lições [`delta`](promql-functions-lab/functions/delta/), [`idelta`](promql-functions-lab/functions/idelta/), [`deriv`](promql-functions-lab/functions/deriv/), [`predict_linear`](promql-functions-lab/functions/predict_linear/), [`changes`](promql-functions-lab/functions/changes/) (40 min)
- [ ] Desafios **040-050** (`--topic gauges`) (30 min)

### Dia 6 (sáb): prática + primeiro incidente (3h)
- [ ] Anki (25 min)
- [ ] 🔥 Gameday **04 counter-negativo** (`cd gamedays && ./start.sh 04`) (45 min)
- [ ] Refazer os desafios que você errou na semana (`./check.py progress`) (40 min)
- [ ] Quizzes 🎓 **Na prova PCA** das lições `rate`, `increase`, `deriv`, `predict_linear` (30 min)
- [ ] Revisar a "linha do que ficou confuso" de cada dia; abrir o mapa mental no ramo **PromQL** (20 min)
- [ ] `./test.sh` do lab promql-operators passa? (10 min)

---

## Semana 2: PromQL II + Prometheus Fundamentals

Anki: terminar `PCA::03 PromQL::Curadas`, depois `PCA::02 Prometheus Fundamentals::Curadas` e `Labs`.

### Dia 8 (seg): agregações (1h45)
- [ ] Anki (20 min)
- [ ] Lab promql-operators: **Agregações** + exercício `06-agregacoes` (30 min)
- [ ] Desafios **060-075** (`--topic aggregation`) e **239-253** (`sum by`, `without`, `count_values`, NaN) (55 min)

### Dia 9 (ter): `_over_time` e ausência de dados (1h45)
- [ ] Anki (20 min)
- [ ] Lições [`avg_over_time`](promql-functions-lab/functions/avg_over_time/), [`max_over_time`](promql-functions-lab/functions/max_over_time/), [`count_over_time`](promql-functions-lab/functions/count_over_time/), [`last_over_time`](promql-functions-lab/functions/last_over_time/), [`absent`](promql-functions-lab/functions/absent/), [`absent_over_time`](promql-functions-lab/functions/absent_over_time/) (45 min)
- [ ] Desafios **080-093** (`over-time`) e **120-129** (`absence-and-staleness`) (40 min)

### Dia 10 (qua): histogramas (1h45)
- [ ] Anki (20 min)
- [ ] Lições [`histogram_quantile`](promql-functions-lab/functions/histogram_quantile/), [`histogram_fraction`](promql-functions-lab/functions/histogram_fraction/), [`histogram_avg`](promql-functions-lab/functions/histogram_avg/), [`histogram_count`](promql-functions-lab/functions/histogram_count/) (45 min)
- [ ] Desafios **100-112** (`--topic histograms`) (40 min)

### Dia 11 (qui): labels, tempo e service discovery (2h)
- [ ] Anki (20 min)
- [ ] Lições [`label_replace`](promql-functions-lab/functions/label_replace/), [`label_join`](promql-functions-lab/functions/label_join/), [`hour`](promql-functions-lab/functions/hour/), [`day_of_week`](promql-functions-lab/functions/day_of_week/) (30 min)
- [ ] Desafios **140-152** (`labels`) e **160-171** (`time`): só os *basic* (30 min)
- [ ] Lab [`service-discovery-relabeling`](labs/service-discovery-relabeling/): README + exercícios 01-05 (40 min)

### Dia 12 (sex): relabeling avançado + TSDB (2h)
- [ ] Anki (20 min)
- [ ] Lab service-discovery-relabeling: exercícios 06-10 (`hashmod`, `__param_target`, `honor_labels`, `sample_limit`) (45 min)
- [ ] Lab [`tsdb-storage`](labs/tsdb-storage/): README + exercícios (blocos, WAL, retenção, `promtool tsdb`) (55 min)

### Dia 13 (sáb): federation + incidentes (3h)
- [ ] Anki (25 min)
- [ ] Lab [`federation-remote-write`](labs/federation-remote-write/): README + exercícios (federation, `remote_write`, agent mode, `external_labels`) (70 min)
- [ ] 🔥 Gameday **02 alvo-sumiu** (40 min)
- [ ] 🔥 Gameday **03 explosao-de-cardinalidade** (40 min)
- [ ] Desafios *advanced* pendentes de `labels`/`time` (livre)

---

## Semana 3: Instrumentação, exporters, regras e Alertmanager

Anki: `PCA::04 Instrumentation & Exporters::*` e `PCA::05 Alerting & Dashboarding::*`.

### Dia 15 (seg): instrumentação (2h)
- [ ] Anki (20 min)
- [ ] Lab [`instrumentation`](labs/instrumentation/): README (tipos de métrica, nomes, unidades, client_golang/client_python) (40 min)
- [ ] Exercícios `01-nomes-e-unidades`, `02-buckets-para-slo`, `03-summary-para-histogram` (60 min)

### Dia 16 (ter): instrumentação II (1h45)
- [ ] Anki (20 min)
- [ ] Exercícios `04-bomba-de-cardinalidade`, `05-metrica-info`, `06-instrumentar-funcao`, `07-formatos-de-exposicao` (70 min)
- [ ] `./test.sh` do lab instrumentation (15 min)

### Dia 17 (qua): exporters e Pushgateway (1h45)
- [ ] Anki (20 min)
- [ ] Lab [`exporters-pushgateway`](labs/exporters-pushgateway/): node_exporter, blackbox (multi-target), textfile collector, Pushgateway (75 min)
- [ ] 🔥 Gameday **06 backup-congelado** (se sobrar tempo, senão no sábado)

### Dia 18 (qui): regras + testes com promtool (1h45)
- [ ] Anki (20 min)
- [ ] Lab [`recording-rules-testing`](labs/recording-rules-testing/): naming `level:metric:operations`, `promtool check rules`, `promtool test rules` (70 min)
- [ ] Refazer o quiz 🎓 da lição [`rate`](promql-functions-lab/functions/rate/) (recording rules usam `rate`) (15 min)

### Dia 19 (sex): Alertmanager (2h)
- [ ] Anki (20 min)
- [ ] Lab [`alertmanager`](labs/alertmanager/): README + exercícios `01-roteamento-severidade` a `04-silenciar` (60 min)
- [ ] Exercícios `05-arvore-de-rotas` a `08-janela-manutencao` (40 min)

### Dia 20 (sáb): incidentes de alerta + 1º simulado (3h)
- [ ] Anki (25 min)
- [ ] 🔥 Gameday **01 alerta-que-nunca-dispara** (40 min)
- [ ] 🔥 Gameday **05 ninguem-foi-paginado** (40 min)
- [ ] 📝 **Simulado 1**: [`exam/index.html`](exam/index.html) ou `cd exam && ./exam.py simulado` (60 questões / 90 min) (90 min)
- [ ] Anotar os **domínios < 75%**: eles viram prioridade da semana 4

---

## Semana 4: Observabilidade, SLO e simulados

Anki: `PCA::01 Observability Concepts::*` + o que sobrou das Curadas; novos/dia = **20**.

### Dia 22 (seg): conceitos de observabilidade (1h30)
- [ ] Anki (20 min)
- [ ] Mapa mental, ramo **Observability Concepts**: métricas × logs × traces, push × pull, white/black-box, USE/RED/Golden Signals (30 min)
- [ ] Cards curados `obs-*` na página [`flashcards.html#d=observability`](flashcards/flashcards.html#d=observability) em modo lista (20 min)
- [ ] Treino só do domínio: `./exam.py treino --domain <Observability...>` (nomes exatos em `./exam.py stats`) (20 min)

### Dia 23 (ter): SLO de ponta a ponta (2h)
- [ ] Anki (20 min)
- [ ] Lab [`slo-end-to-end`](labs/slo-end-to-end/): SLI de disponibilidade/latência, error budget, burn rate multi-window, dashboard Grafana (90 min)

### Dia 24 (qua): pontos fracos (1h45)
- [ ] Anki (20 min)
- [ ] Revisar os domínios < 75% do simulado 1: lições/labs correspondentes pelo [mapa mental](mindmap/index.html) (50 min)
- [ ] Desafios *advanced* de 2 tópicos à escolha (`./check.py list --level advanced`) (35 min)

### Dia 25 (qui): simulado 2 (2h)
- [ ] Anki (20 min)
- [ ] 📝 **Simulado 2**: `./exam.py simulado --lang all` (inclui as questões em PT extraídas das lições) (90 min)
- [ ] Revisar **cada** erro lendo a explicação (pt-BR) (em seguida)

### Dia 26 (sex): pegadinhas (1h30)
- [ ] Anki (20 min)
- [ ] Ler as seções **⚠️ Pegadinhas** de todos os labs + o Top 15 de [`PCA.md`](promql-functions-lab/PCA.md) (40 min)
- [ ] Anki: *filtered deck* `tag:kind::pegadinha is:due` ou `tag:kind::pegadinha rated:7:1` (30 min)

### Dia 27 (sáb): simulado 3 + gameday livre (3h)
- [ ] Anki (25 min)
- [ ] 📝 **Simulado 3** (90 min). Meta: **≥ 80%** em todos os domínios
- [ ] Repetir o gameday em que você mais sofreu, sem olhar a solução (45 min)
- [ ] `./test-all.sh` na raiz (opcional: garante que tudo roda na sua máquina)

---

## ⭐ Semana da prova

Os últimos 7 dias antes da data (podem coincidir com a semana 4, se a prova for logo depois dela).
**Anki com 0 cards novos**: só revisões.

- **D-7 a D-5 (1h/dia)**
  - [ ] Anki: só revisões (20 min)
  - [ ] Um simulado por dia, alternando domínios fracos e prova completa (40 min)
  - [ ] Reler as **📝 Colas rápidas** das lições de counters, histogramas e `absent` (15 min)
- **D-4 (1h)**
  - [ ] [`flashcards.html`](flashcards/flashcards.html) com **ocultar conhecidas** ligado: varredura dos cards Curados + Top 15, marcando com `k` o que já está sólido
- **D-3 (1h30)**
  - [ ] 📝 Último simulado completo, cronometrado, nas mesmas condições da prova (mesa limpa, sem consulta)
- **D-2 (45 min)**
  - [ ] Revisar os erros do último simulado + o [mapa mental](mindmap/index.html) inteiro, ramo a ramo
  - [ ] Checar os requisitos da prova online: ID com foto, webcam, sala vazia, check de sistema do PSI/Linux Foundation
- **D-1 (30 min, leve)**
  - [ ] Anki (revisões) e **parar**. Dormir bem vale mais que 50 cards a mais
- **Dia D**
  - [ ] Chegar 30 min antes para o check-in
  - [ ] Na prova: responda o que sabe, **marque para revisar** o que tomar mais de 2 min e volte no final (~90 min para ~60 questões: cerca de 1,5 min por questão)
  - [ ] Leia devagar os enunciados com **NOT / EXCEPT / BEST**, porque as pegadinhas moram aí

### Cola da última hora

- `rate`/`increase`/`irate` → counters · `delta`/`deriv`/`predict_linear` → gauges · primeiro `rate`, depois `sum`
- `histogram_quantile(φ, sum by (le) (rate(x_bucket[5m])))` · summary **não** agrega
- `absent()` → 1 quando **não** existe · `or vector(0)` evita "No data"
- `relabel_configs` = antes do scrape (targets) · `metric_relabel_configs` = depois (amostras)
- Alertmanager: `group_wait` 30s · `group_interval` 5m · `repeat_interval` 4h · silence = manual · inhibition = automática
- Defaults: scrape 1m · timeout 10s · evaluation 1m · retenção 15d · bloco 2h · lookback 5m
- Error budget 99,9% / 30d = **43,2 min** · RED = serviços · USE = recursos
- Nomes: `_total` em counters, unidades base (`_seconds`, `_bytes`), sem labels de alta cardinalidade
