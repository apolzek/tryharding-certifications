# 🎯 Desafios PromQL com correção automática

182 desafios práticos de PromQL, corrigidos **na hora** contra o Prometheus do
[`promql-functions-lab`](../promql-functions-lab/). Você lê um problema realista ("o time de SRE quer..."),
escreve a query e o `check.py` diz se ela está certa. Se errar, ele mostra **por quê** (faltou série, sobrou
label, valor diferente...).

> A PCA cobra PromQL como o maior domínio da prova (28%). Ler a documentação não basta:
> o que fixa é escrever query errada, ver o erro e entender a pegadinha. É isso que estes desafios treinam.

## ▶️ Pré-requisitos

```bash
# 1. o lab precisa estar no ar (Prometheus em :9095, gerador de métricas em :8088)
cd ../promql-functions-lab && tools/deploy.sh      # ou: docker compose up -d
# 2. dependências do corretor
pip install pyyaml requests
```

> Espere uns 10 minutos depois de subir o lab do zero: vários desafios usam janelas de `[5m]`, `[10m]` e `[1h]`.

## 🕹️ Como usar

```bash
cd challenges
./check.py list                          # todos os desafios (✅ = resolvido), agrupados por tópico
./check.py list --topic counters         # só um tópico
./check.py list --level advanced         # só um nível (basic | intermediate | advanced)
./check.py show 020                      # o enunciado completo
./check.py 020 'rate(rate_http_requests_total[5m])'   # responde (use ASPAS SIMPLES!)
./check.py 020 -f minha-resposta.promql  # resposta num arquivo (bom para queries longas)
./check.py hint 020                      # próxima dica (são 2 ou 3, cada vez mais explícitas)
./check.py solution 020                  # desiste: gabarito + explicação
./check.py progress                      # barra de progresso por tópico
./check.py exporter                      # seu progresso como métrica Prometheus (veja abaixo)
```

Exemplo de sessão:

```text
$ ./check.py 073 'rate(sort_desc_http_requests_total{code="500"}[5m]) / rate(sort_desc_http_requests_total[5m])'
✘ ainda não: labels diferentes: faltam 4 série(s), ex.: {service='cart'}; sobram 4 série(s), ex.: {code='500', env='lab', ...}
dica: ./check.py hint 073

$ ./check.py 073 'sum by (service) (rate(sort_desc_http_requests_total{code="500"}[5m])) / sum by (service) (rate(sort_desc_http_requests_total[5m]))'
✔ CORRETO! 4 série(s) batendo

Por que funciona:
payments 12%, checkout 5%, cart 1%, recommendations NaN (0/0: nenhum tráfego). ...
```

Dica de fluxo: abra o Prometheus (http://localhost:9095) ou o Grafana Explore (http://localhost:3300/explore)
ao lado, experimente a query lá e só depois mande para o `check.py`. Cada desafio aponta a lição
(`📘 estude: ...`) com a teoria da função.

O progresso fica em `challenges/.progress.json` (ignorado pelo git). Para recomeçar do zero: `rm .progress.json`.
Para ter vários "perfis": `PCA_PROGRESS=~/meu-progresso.json ./check.py ...`.

## ⚖️ Como a correção funciona

O corretor **não compara texto**: ele roda a sua query e o gabarito no Prometheus, **no mesmo instante**
(10 s atrás, para não pegar um scrape "em voo"), e compara os **resultados**. Por isso formas diferentes
mas equivalentes passam (`increase(x[5m]) / 300` = `rate(x[5m])`, `sum(x) by (a)` = `sum by (a) (x)`...).

| O que é comparado | Regra |
|---|---|
| Tipo do resultado | vetor ≠ scalar ≠ range vector (`x` não é aceito onde se pede `scalar(x)`) |
| Conjunto de séries | os **labels** de cada série precisam bater (o `__name__` é ignorado; alguns desafios ignoram mais labels, ex.: `job`) |
| Valores | tolerância relativa de **2%** (alguns desafios usam outra); `NaN` = `NaN` |
| Ordem | só nos desafios de `sort`/`sort_desc`/`sort_by_label` (o enunciado avisa) |
| Range vector (`x[1m]`) | nº de amostras e último valor de cada série |
| Native histogram cru | contagem e soma de cada histograma |
| Resultado vazio | só é aceito quando o gabarito também está vazio naquele instante (ex.: `up == 0` com tudo no ar) |

Mensagens que você vai ver:

- `labels diferentes: faltam ... / sobram ...` → você agregou demais/de menos, esqueceu um filtro ou um `by`.
- `valor errado em {...}` → séries certas, conta errada (janela, unidade, função, precedência...).
- `sua query não roda` → erro de sintaxe/tipo, com a mensagem do Prometheus.
- `o gabarito voltou vazio agora` → os dados ainda estão chegando (lab recém-subido). Espere 1 min.

Todo desafio tem `wrong_answers` (erros típicos) que **o próprio teste garante que são rejeitados** e,
quando existe forma equivalente, `accepted_answers` que garantem que ela passa. Se você achar uma resposta
certa que foi rejeitada (ou uma errada que passou), é bug: abra uma issue/PR com a query.

## 🗺️ Tópicos

| Tópico | Ids | 🟢 | 🟡 | 🔴 | Total | Lições |
|---|---|---|---|---|---|---|
| `selectors-and-types` — matchers, regex ancorada, `offset`, `@`, range vs instant, scalar, subqueries, NaN | 001–013 | 6 | 5 | 2 | **13** | [querying basics](https://prometheus.io/docs/prometheus/latest/querying/basics/) |
| `counters` — `rate`, `irate`, `increase`, `resets`, rate-antes-de-sum | 020–030 | 5 | 4 | 2 | **11** | [rate](../promql-functions-lab/functions/rate/), [increase](../promql-functions-lab/functions/increase/), [irate](../promql-functions-lab/functions/irate/), [resets](../promql-functions-lab/functions/resets/) |
| `gauges` — `delta`, `idelta`, `deriv`, `predict_linear`, `changes`, `double_exponential_smoothing` | 040–050 | 4 | 6 | 1 | **11** | [delta](../promql-functions-lab/functions/delta/), [deriv](../promql-functions-lab/functions/deriv/), [predict_linear](../promql-functions-lab/functions/predict_linear/) |
| `aggregation` — `sum/avg/max/min/count/stddev/quantile/topk/bottomk/count_values/group`, `by`/`without` | 060–075 | 7 | 6 | 2 | **15** | [operadores de agregação](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators) |
| `over-time` — `*_over_time`, subqueries, `ts_of_max_over_time`, `mad_over_time`, z-score | 080–093 | 5 | 7 | 2 | **14** | [avg_over_time](../promql-functions-lab/functions/avg_over_time/), [quantile_over_time](../promql-functions-lab/functions/quantile_over_time/) |
| `histograms` — clássicos e native: `histogram_quantile/fraction/avg/count/sum/stddev/quantiles`, Apdex | 100–112 | 4 | 5 | 4 | **13** | [histogram_quantile](../promql-functions-lab/functions/histogram_quantile/), [histogram_fraction](../promql-functions-lab/functions/histogram_fraction/) |
| `absence-and-staleness` — `absent`, `absent_over_time`, `present_over_time`, `up`, staleness, séries sintéticas | 120–129 | 4 | 5 | 1 | **10** | [absent](../promql-functions-lab/functions/absent/), [absent_over_time](../promql-functions-lab/functions/absent_over_time/) |
| `labels` — `label_replace`, `label_join`, `info`, `sort*`, joins com `group_left` | 140–152 | 5 | 4 | 4 | **13** | [label_replace](../promql-functions-lab/functions/label_replace/), [info](../promql-functions-lab/functions/info/) |
| `time` — `time`, `timestamp`, `hour`, `minute`, `day_of_*`, `month`, `year`, `days_in_month`, fuso | 160–171 | 7 | 2 | 3 | **12** | [time](../promql-functions-lab/functions/time/), [timestamp](../promql-functions-lab/functions/timestamp/) |
| `alerting-expressions` — a `expr` de alertas reais: error ratio, SLO burn rate, disco enchendo, certificado, crashloop, clock skew, FinOps | 180–195 | 5 | 8 | 3 | **16** | [PCA.md](../promql-functions-lab/PCA.md) |
| `operators` — aritmética, comparação/`bool`, `and/or/unless`, vector matching (`on/ignoring/group_left`) | 200–253 | 19 | 26 | 9 | **54** | [labs/promql-operators](../labs/promql-operators/) |
| **Total** | | **71** | **78** | **33** | **182** | |

Níveis: 🟢 básico (uma função/um conceito) · 🟡 intermediário (combinar 2–3 coisas ou uma pegadinha clássica)
· 🔴 avançado (joins, subqueries, SLOs, detalhes que derrubam na prova).

Sugestão de trilha: `selectors-and-types` → `counters` → `gauges` → `aggregation` → `operators` → `over-time`
→ `histograms` → `labels` → `time` → `absence-and-staleness` → `alerting-expressions`. Faça todos os 🟢
primeiro (`./check.py list --level basic`), depois volte para os 🟡 e 🔴.

## 📊 Dashboard "Meu progresso PCA"

Estude Prometheus **usando** Prometheus: o `check.py exporter` expõe seu progresso como métricas, o
Prometheus do lab raspa (job `pca-progress`, alvo `host.docker.internal:9199`) e o Grafana mostra no
dashboard **Meu progresso PCA** → http://localhost:3300/d/pca-progress

```bash
./check.py exporter            # deixe rodando num terminal (Ctrl+C para parar)
# ou, em segundo plano e à prova de firewall:
./check.py exporter --docker   # container "pca-progress" publicando :9199 (parar: docker rm -f pca-progress)
```

| Métrica | Tipo | Labels |
|---|---|---|
| `pca_challenges_total` | gauge | `topic`, `level` |
| `pca_challenges_solved` | gauge | `topic`, `level` |
| `pca_challenge_attempts_total` | counter | `topic`, `level` |
| `pca_challenge_hints_used` | gauge | `topic`, `level` |
| `pca_exam_score_ratio` | gauge | `domain` (nota do último simulado de [`exam/`](../exam/), 0..1) |
| `pca_exam_overall_score_ratio`, `pca_exam_last_timestamp_seconds` | gauge | — |

O painel mostra % resolvido no total, por tópico e por nível, tentativas por acerto, dicas usadas,
a evolução ao longo do tempo e a nota do simulado por domínio. Abra os painéis: as queries também são
material de estudo (`sum by (topic) (...) / sum by (topic) (...)`, `increase()` no counter, `delta()` no gauge...).

**O painel "Exporter de progresso" está vermelho ("parado")?**
1. O exporter está rodando? `curl -s localhost:9199/metrics | head`
2. O alvo aparece em http://localhost:9095/targets (job `pca-progress`)? Se estiver `DOWN` com
   `context deadline exceeded`, o **firewall do host** (ex.: `ufw`) está bloqueando conexões
   container → host. Use `./check.py exporter --docker` (tráfego para porta publicada de container não
   passa pela chain INPUT) ou libere a porta: `sudo ufw allow from 172.16.0.0/12 to any port 9199 proto tcp`.
3. Mudou o `prometheus.yml`? Recarregue sem reiniciar: `curl -X POST localhost:9095/-/reload`.

## 💡 Dicas

- **Aspas simples** em volta da query no shell (`'...{code="500"}...'`), senão o bash come as aspas duplas e o `$1`.
- Leia a mensagem de erro com calma: "faltam/sobram séries" quase sempre é agregação (`by`/`without`) ou filtro.
- `rate` antes de `sum`, sempre. `increase`/`rate` só em counters; `delta`/`deriv` só em gauges.
- Janela de `rate` ≥ 4× o scrape interval (aqui o scrape é 5 s, então `[1m]` já é seguro; na vida real, `[5m]`).
- Unidades: tempo em **segundos**, frações de 0 a 1 (`histogram_quantile(0.99, ...)`, `> 0.05` = 5%).
- Travou? `hint` é de graça (mas o dashboard conta 😉). A 3ª dica geralmente é quase a resposta.
- Resolveu? Leia a explicação mesmo assim: ela traz a **pegadinha** que costuma cair na prova.

## 🛠️ Para quem mantém

- Formato do YAML: docstring no topo de [`check.py`](check.py). Um arquivo por desafio em
  `<tópico>/<id>-<slug>.yaml`; ids 001–199 = tópicos deste README, 200–299 = `operators`.
  Pastas começando com `_` ou `.` são ignoradas (bom para rascunhos).
- `./check.py selftest [ids|tópicos]`: roda todo gabarito, exige que as `accepted_answers` passem e que as
  `wrong_answers` sejam rejeitadas. `./test.sh` = selftest + smoke test da CLI e do exporter (precisa do lab no ar).
- As métricas do lab mudam com o tempo (ondas, rajadas, séries que somem). Uma `wrong_answer` que só é
  rejeitada "às vezes" é bug: ao criar/alterar um desafio, valide-o em vários instantes (ex.: a cada 2 min
  das últimas 14 h, chamando `query()`/`compare()` do `check.py` com `ts` no passado).
