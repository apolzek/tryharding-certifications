# Exercício 04: desarme a bomba de cardinalidade

## O problema

`http_requests_by_user_total{user_id="..."}` cria **uma série por usuário**. O gerador de carga usa 5.000 usuários; em produção seriam milhões.

```bash
# o promtool NÃO acusa nada de errado no nome...
curl -s localhost:9131/metrics/bad | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics
# ...mas o --extended mostra quem domina a cardinalidade:
curl -s localhost:9131/metrics | docker run --rm -i --entrypoint promtool prom/prometheus:v3.15.0 check metrics --extended 2>/dev/null | head -5
```

No Prometheus:

```promql
count by (job) (http_requests_by_user_total)        # centenas e subindo
topk(5, count by (__name__) ({__name__=~".+"}))     # quem são as maiores métricas
```

Também dá para ver em **Status → TSDB Status** (http://localhost:9130/tsdb-status), seção *Top 10 series count by metric names*.

## O que fazer

Remova o label `user_id`. Se o negócio precisa de um recorte, use um label **limitado**: por exemplo `plan` (`free`, `pro`, `enterprise`, derivado de `user_id % 3`) numa métrica `checkouts_by_plan_total`.

## Como verificar

```bash
docker compose down -v && docker compose up -d --build --wait    # -v: zera o TSDB
curl -s localhost:9130/api/v1/query --data-urlencode 'query=count by (instance) (checkouts_by_plan_total)'
# no máximo 3 por instância
curl -s localhost:9130/api/v1/query --data-urlencode 'query=http_requests_by_user_total'
# "result":[]
```

<details><summary>✅ Solução</summary>

```go
checkoutsByPlan = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "checkouts_by_plan_total", Help: "Checkouts por plano do cliente.",
}, []string{"plan"})

func planOf(userID string) string {
    n, _ := strconv.Atoi(userID)
    return [...]string{"free", "pro", "enterprise"}[n%3]
}
// no handler:
checkoutsByPlan.WithLabelValues(planOf(r.Header.Get("X-User-Id"))).Inc()
```

```python
CHECKOUTS_BY_PLAN = Counter("checkouts_by_plan_total", "Checkouts por plano do cliente.", ["plan"], registry=BAD)
CHECKOUTS_BY_PLAN.labels(plan_of(h.headers.get("X-User-Id", ""))).inc()
```

Se não puder mexer no código, dá para **descartar** no Prometheus (o app continua pagando o custo de manter as séries em memória, mas o TSDB não):

```yaml
metric_relabel_configs:
  - source_labels: [__name__]
    regex: http_requests_by_user_total
    action: drop
```

E para limitar o estrago de qualquer alvo: `sample_limit: 5000` no scrape config (se passar, o scrape inteiro falha e `up` vira 0).

Dado por usuário/requisição vai para **logs** ou **traces**, não para métricas.
</details>
