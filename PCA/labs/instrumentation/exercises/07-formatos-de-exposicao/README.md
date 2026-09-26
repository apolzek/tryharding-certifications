# Exercício 07: formatos de exposição, `_created` e exemplars

Sem código: só investigação com `curl`. Responda às perguntas e confira no `<details>`.

```bash
# 1) Prometheus text 0.0.4 (o default quando não há Accept)
curl -s localhost:9132/metrics | grep -A3 '^# HELP http_requests_total'

# 2) OpenMetrics 1.0
curl -s -H 'Accept: application/openmetrics-text; version=1.0.0' localhost:9132/metrics | grep -A4 '^# HELP http_requests '
curl -s -H 'Accept: application/openmetrics-text; version=1.0.0' localhost:9131/metrics | tail -1

# 3) Protobuf (o Go suporta; o Python não)
curl -s -o /dev/null -w '%{content_type}\n' \
  -H 'Accept: application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited' localhost:9131/metrics
curl -s -o /dev/null -w '%{content_type}\n' \
  -H 'Accept: application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily;encoding=delimited' localhost:9132/metrics

# 4) Exemplars
curl -s -H 'Accept: application/openmetrics-text' localhost:9131/metrics | grep -m2 'trace_id'
curl -s -G localhost:9130/api/v1/query_exemplars \
  --data-urlencode 'query=http_request_duration_seconds_bucket{job="app-go"}' \
  --data-urlencode "start=$(( $(date +%s) - 300 ))" --data-urlencode "end=$(date +%s)" | head -c 400
```

**Perguntas:**

1. No texto 0.0.4 do Python, qual é o `# TYPE` de `http_requests_created`? E no OpenMetrics?
2. No OpenMetrics, qual é o nome da **família** no `# TYPE` do counter? Por quê?
3. Qual é a última linha de um payload OpenMetrics?
4. Qual `content_type` o app Python devolve quando pedimos protobuf?
5. Onde fica o exemplar na linha, e o que o Prometheus precisa para guardá-lo?

<details><summary>✅ Respostas</summary>

1. No 0.0.4 não existe o conceito de "created", então o client Python expõe `http_requests_created` como uma **métrica gauge separada** (`# TYPE http_requests_created gauge`), que o Prometheus ingere como série comum. No OpenMetrics, `_created` é uma amostra **da própria família** do counter. Mesmo assim, o Prometheus 3.15 ingere essas linhas como séries (`count({__name__=~".*_created"})` no lab dá centenas). Para não expor: `PROMETHEUS_DISABLE_CREATED_SERIES=True` no Python; no Go, `EnableOpenMetricsTextCreatedSamples: false` (o default).
2. `# TYPE http_requests counter`: no OpenMetrics o nome da família do counter **não** tem `_total`; o `_total` é o sufixo da amostra. Por isso a metadata do Prometheus para o app Python aparece como `http_requests`, e para o Go (raspado em protobuf) como `http_requests_total`.
3. `# EOF`. Sem ela o payload OpenMetrics é inválido (detecta resposta truncada).
4. `text/plain; version=0.0.4; charset=utf-8`: o client Python não fala protobuf e cai no formato texto. O Go devolve `application/vnd.google.protobuf; ...; encoding=delimited`. É uma **negociação HTTP** (`Accept`): o Prometheus manda uma lista de formatos com pesos (definida por `scrape_protocols`) e o alvo responde com o melhor que **ele** suporta. Com `scrape_native_histograms: true`, o Prometheus coloca o protobuf em primeiro lugar, porque os formatos de texto estáveis (0.0.4 e OpenMetrics 1.0) não conseguem transportar native histograms; só o protobuf consegue.
5. Depois de ` # ` no fim da linha do bucket: `... 21 # {trace_id="5fb5..."} 0.0616 1.79e+09` (labels do exemplar, valor observado, timestamp). O Prometheus só guarda com `--enable-feature=exemplar-storage` e só recebe exemplars via **OpenMetrics ou protobuf** (nunca no text 0.0.4).
</details>
