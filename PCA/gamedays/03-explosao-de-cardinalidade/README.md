# 03 · Explosão de cardinalidade

> **Em uma frase:** depois do deploy 1.4.2 do checkout, o Prometheus começou a engordar sem parar. Alguém adicionou um label com **user_id**, e cada usuário novo virou uma série nova.

| | |
|---|---|
| **Dificuldade** | ⭐⭐ |
| **Tempo-alvo** | 20 min |
| **Tópicos PCA** | cardinalidade, `count by (__name__)`, `topk`, `/api/v1/status/tsdb`, `metric_relabel_configs`, `sample_limit`, `label_limit` |
| **Arquivos que você vai editar** | `work/prometheus/prometheus.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 📟 PAGE · PrometheusMuitasSeries · severity=ticket                       │
│ prometheus-0: head do Prometheus com 5.8k séries (ontem: 800)            │
├──────────────────────────────────────────────────────────────────────────┤
│ + comentário do on-call anterior:                                        │
│ "a memória do prometheus-0 subiu de 2 GB para 11 GB desde o deploy do    │
│  checkout 1.4.2. as queries do painel de checkout estão dando timeout.   │
│  se passar de 16 GB ele toma OOMKill e ficamos sem monitoramento."       │
│                                                                          │
│ Pedido: achar QUAL métrica/label explodiu e estancar sem perder as       │
│ métricas boas do checkout (http_requests_total precisa continuar lá).    │
│ Critério: o job checkout deve ingerir < 200 amostras por scrape.         │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 03
# edite work/prometheus/prometheus.yml  ->  ./reload.sh  ->  ./check.sh 03
```

O app começa com 500 usuários e ganha **60 usuários novos por segundo** (até 5000). Quanto mais você demora, maior o estrago.

## 🩺 Sintomas

- `prometheus_tsdb_head_series` subindo sem parar.
- `scrape_samples_scraped{job="checkout"}` na casa dos milhares, para um serviço que tem meia dúzia de métricas.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> a cabeça do TSDB está crescendo?</summary>

```promql
prometheus_tsdb_head_series
```

Veja no modo **Graph** por 5 minutos: uma rampa. Agora, quem mais contribui, por job:

```promql
count by (job) ({__name__=~".+"})
```

**Resultado esperado:** `checkout` com milhares de séries; `prometheus` com algumas centenas.
</details>

<details>
<summary><b>Passo 2:</b> qual MÉTRICA explodiu?</summary>

```promql
topk(5, count by (__name__) ({job="checkout"}))
```

**Resultado esperado:** `app_requests_by_user_total` com milhares de séries; `http_requests_total` com 2; `app_info` com 1.

> ⚠️ Em produção de verdade, `count by (__name__) ({__name__=~".+"})` sobre o Prometheus inteiro pode ser pesada. Prefira o endpoint do passo 3, que lê estatísticas prontas do head.
</details>

<details>
<summary><b>Passo 3:</b> qual LABEL explodiu? Use a API de status do TSDB.</summary>

```bash
curl -s localhost:9180/api/v1/status/tsdb | jq '.data.seriesCountByMetricName[:3]'
curl -s localhost:9180/api/v1/status/tsdb | jq '.data.labelValueCountByLabelName[:3]'
```

**Resultado esperado:** `app_requests_by_user_total` no topo das métricas, e `user_id` com milhares de valores distintos no topo dos labels. Na UI: Status → TSDB Status.

Confirme olhando o que o app expõe:

```bash
curl -s localhost:9182/metrics | grep -c app_requests_by_user_total
curl -s localhost:9182/metrics | grep app_requests_by_user_total | head -3
```
</details>

<details>
<summary><b>Passo 4:</b> quanto o job ingere por scrape, antes e depois do relabel?</summary>

```promql
scrape_samples_scraped{job="checkout"}
scrape_samples_post_metric_relabeling{job="checkout"}
scrape_series_added{job="checkout"}
```

`scrape_samples_scraped` é o que o app **expôs**; `scrape_samples_post_metric_relabeling` é o que **sobrou** depois dos `metric_relabel_configs` (hoje são iguais, porque não há nenhum). `scrape_series_added` > 0 em todo scrape = churn: séries novas o tempo todo.
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

O deploy 1.4.2 adicionou uma métrica de debug, `app_requests_by_user_total{user_id="..."}`. `user_id` é um label de **cardinalidade ilimitada**: cada usuário novo cria uma série nova no TSDB (índice + chunk em memória), e ela fica no head até a próxima compactação mesmo depois de parar de receber amostras. A memória do Prometheus é proporcional ao número de **séries ativas**, não ao volume de amostras.

Não havia nenhuma proteção (`sample_limit`, `label_limit`) no job, então nada impediu o crescimento.
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/prometheus/prometheus.yml</code></summary>

Descarte a métrica **depois do scrape e antes da ingestão** com `metric_relabel_configs`, e adicione um limite como rede de proteção:

```yaml
  - job_name: checkout
    sample_limit: 1000       # passou disso? o scrape INTEIRO falha (up=0) e nada é ingerido
    label_limit: 30
    static_configs:
      - targets: ['app:9182']
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: app_requests_by_user_total
        action: drop
```

```bash
./reload.sh && ./check.sh 03
```

**Por que não `labeldrop: user_id`?** Tirar só o label deixaria milhares de amostras com **o mesmo conjunto de labels** no mesmo scrape. O Prometheus rejeita as duplicadas (`prometheus_target_scrapes_sample_duplicate_timestamp_total` sobe) e o valor que sobra não significa nada. Se a métrica agregada fosse útil, o certo é o app expô-la **sem** o `user_id`.

**Por que não `relabel_configs`?** `relabel_configs` roda **antes** do scrape, sobre os labels do *target*; ele não vê nomes de métrica. Para filtrar métricas, é `metric_relabel_configs`.

> Depois do fix, as séries antigas continuam no head (memória) até a próxima compactação (~2h), e no disco até a retenção. Com o admin API habilitado dá para apagá-las das consultas e do disco já (a memória do head só é liberada na compactação):
> ```bash
> curl -X POST -g 'localhost:9180/api/v1/admin/tsdb/delete_series?match[]=app_requests_by_user_total'
> curl -X POST localhost:9180/api/v1/admin/tsdb/clean_tombstones
> ```
</details>

## 🛡️ Como evitar

- **`sample_limit` em todo job** (ou `global.sample_limit` no Prometheus 3.x como teto padrão), com folga de ~2× o normal. Quando estoura, o scrape falha e `up` vai a 0: você é avisado, em vez de o Prometheus inteiro cair.
- **Alerta de limite chegando perto:**

```yaml
- alert: ScrapeProximoDoSampleLimit
  expr: |
    scrape_samples_post_metric_relabeling
      / on(job, instance) group_left() scrape_sample_limit > 0.8
  for: 15m
  labels: {severity: ticket}
```
  (`scrape_sample_limit` é uma das "extra scrape metrics": no Prometheus 3.15 ligue com `extra_scrape_metrics: true` no `global` ou no job; a antiga flag `--enable-feature=extra-scrape-metrics` ainda funciona, mas está sendo descontinuada.)

- **Alerta de crescimento do head e de churn:**

```yaml
- alert: PrometheusSeriesCrescendoRapido
  expr: delta(prometheus_tsdb_head_series[1h]) / prometheus_tsdb_head_series offset 1h > 0.5
  for: 15m
- alert: ChurnDeSeriesAlto
  expr: sum by (job) (rate(scrape_series_added[10m])) > 100
  for: 15m
```

- **Regra de revisão de código:** nenhum label com valores não limitados (`user_id`, `request_id`, `email`, `path` com IDs, timestamps). Isso vai em logs/traces, não em métricas.

## 📝 Postmortem (exemplo)

> **Resumo:** após o deploy do checkout 1.4.2 (10:12 UTC), as séries ativas do prometheus-0 cresceram de ~800k para ~5,8M em 3h e a memória de 2 GB para 11 GB. Queries de dashboard deram timeout das 12:30 às 13:20 UTC. Sem perda de dados.
>
> **Causa raiz:** nova métrica de debug `app_requests_by_user_total` com label `user_id` (cardinalidade ilimitada), sem `sample_limit` no job para conter o crescimento.
>
> **Ações:**
> 1. (mitigar) `metric_relabel_configs` com `drop` da métrica + `delete_series`. ✅
> 2. (corrigir) remover a métrica do código do checkout. **Dono:** time checkout.
> 3. (prevenir) `sample_limit` e `label_limit` em todos os jobs. **Dono:** plataforma.
> 4. (detectar) alertas de crescimento do head e de churn. **Dono:** SRE.
> 5. (prevenir) checklist de revisão de instrumentação: nada de IDs em labels. **Dono:** guilda de observabilidade.

## 🎓 Na prova PCA

<details>
<summary>Q1. You need to drop a high-cardinality metric exposed by a target before it is stored. Which config block? (a) relabel_configs (b) metric_relabel_configs (c) write_relabel_configs (d) alert_relabel_configs</summary>

**(b) `metric_relabel_configs`**, que roda depois do scrape e antes da ingestão. `relabel_configs` atua nos labels do target (antes do scrape); `write_relabel_configs` só afeta o que vai para remote write (o dado continua no TSDB local).
</details>

<details>
<summary>Q2. What happens when a target exposes more samples than <code>sample_limit</code>?</summary>

O scrape inteiro é considerado **falho**: nenhuma amostra daquele scrape é ingerida e `up` fica 0 para o target.
</details>
