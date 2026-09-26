# 08 · `promtool check rules` + lint (quebre e conserte)

**Objetivo:** fazer [`rules.yml`](rules.yml) passar em **três** verificações:

```bash
cd labs/recording-rules-testing/exercises/08-check-rules-lint
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 check rules rules.yml                # sintaxe + PromQL + campos válidos
docker run --rm -v "$PWD:/w" -w /w --entrypoint promtool prom/prometheus:v3.15.0 check rules --lint-fatal rules.yml   # + regras duplicadas viram erro (exit 3)
../../lint-names.sh rules.yml            # convenção nível:métrica:operações
```

São **4** problemas.

## 💡 Dica

- Qual campo só existe em regra de **alerta**?
- Sem `--lint-fatal`, o promtool imprime `FAILED ... duplicate rule(s)` mas sai com **exit 0**. Em CI isso passa despercebido.
- No Prometheus 3 nomes UTF-8 são válidos, então o `-` em `job:http-requests:rate5m` **não** é pego pelo promtool. Mas ele quebra a convenção e obriga a consultar como `{"job:http-requests:rate5m"}`.

<details><summary>✅ Solução</summary>

1. `job:http-requests:rate5m` → `job:http_requests:rate5m`
2. remover `for: 5m` da recording rule (`invalid field 'for' in recording rule`)
3. remover a regra `job:up:sum` duplicada
4. fechar o parêntese: `increase(kube_pod_container_status_restarts_total[1h]) > 5`

Arquivo: [`solutions/08-check-rules-lint/rules.yml`](../../solutions/08-check-rules-lint/rules.yml).
</details>
