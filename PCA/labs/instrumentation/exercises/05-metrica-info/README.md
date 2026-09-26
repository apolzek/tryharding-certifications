# Exercício 05: adicione uma métrica `_info`

Queremos saber **qual versão** está rodando em cada instância e poder "anexar" essa informação a outras queries.

## O que fazer

Exponha `app_info{version="1.4.2", commit="9f3c2ab", language="go|python"} 1` nos dois apps.

- **Go:** um `GaugeVec` com os labels, `.Set(1)` uma vez no `init()`.
- **Python:** o tipo `Info` do `prometheus_client` (`Info("app", ...)` expõe `app_info`).

## Como verificar

```bash
docker compose up -d --build --wait
curl -s localhost:9131/metrics | grep '^app_info'
curl -s localhost:9132/metrics | grep '^app_info'
curl -s localhost:9130/api/v1/query --data-urlencode 'query=count by (version) (app_info)'
# version="1.4.2" -> 3 (duas réplicas Go + Python)
```

O padrão de uso é o **join** com `group_left`, que copia o label `version` para outra métrica:

```promql
sum by (instance, job) (rate(http_requests_total[1m]))
  * on (instance, job) group_left (version) app_info
```

<details><summary>✅ Solução</summary>

```go
appInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
    Name: "app_info", Help: "Metadados da aplicação (valor sempre 1).",
}, []string{"version", "commit", "language"})
// init():
good.MustRegister(appInfo)
appInfo.WithLabelValues("1.4.2", "9f3c2ab", "go").Set(1)
```

```python
from prometheus_client import Info
APP_INFO = Info("app", "Metadados da aplicação.")
APP_INFO.info({"version": "1.4.2", "commit": "9f3c2ab", "language": "python"})
```

Detalhe: no formato texto 0.0.4 o `Info` do Python aparece como `# TYPE app_info gauge`; no OpenMetrics aparece como `# TYPE app info` (tipo `info` de verdade). Por isso a metadata no Prometheus mostra `gauge` para o Go e `info` para o Python.
</details>
