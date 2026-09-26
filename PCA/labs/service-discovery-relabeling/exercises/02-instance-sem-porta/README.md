# 02 · `replace`: `instance` sem a porta

**Cenário:** o time quer `instance="payments-api"` em vez de `instance="payments-api:8000"` (a porta é sempre 8000 e só polui os painéis). A tentativa abaixo deixou **todos** os alvos com `instance="8000"`.

```bash
./load.sh exercises/02-instance-sem-porta/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=apps-hostname' | jq -r '.data.activeTargets[].labels.instance'
# 8000  (x4)
curl -s localhost:9120/api/v1/query --data-urlencode 'query=app_requests_total{job="apps-hostname",route="/"}' | jq '.data.result | length'
# 3   <- eram para ser 4!
```

**Por que 3 séries?** `payments-api` e `payments-worker` têm os mesmos `team`/`env` e agora o mesmo `instance`: os dois alvos escrevem na **mesma série**. Os valores se alternam e o `rate()` enxerga "resets" falsos. Colisão de labels é um dos piores bugs de relabel porque **não dá erro**.

**Tarefa:** `instance` = só o hostname, um valor diferente por alvo.

> 💡 **Dica:** `$1`, `$2`... referenciam os grupos da regex. Um grupo que pare no `:` é `([^:]+)`.

<details><summary>Solução</summary>

```yaml
    relabel_configs:
      - source_labels: [__address__]
        regex: '([^:]+):\d+'
        target_label: instance
        replacement: '$1'
```
(`action: replace` é o padrão, por isso não aparece.)

```bash
./load.sh solutions/02-instance-sem-porta/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=apps-hostname' | jq -r '.data.activeTargets[].labels.instance' | sort
# checkout-api / payments-api / payments-worker / search-api
```

> Nota: reescrever `instance` **não muda** para onde o scrape vai. Quem manda no endereço é `__address__`, e ele continua `payments-api:8000`. Se `instance` não for definido no relabel, o Prometheus copia `__address__` para ele **no final**.
</details>
