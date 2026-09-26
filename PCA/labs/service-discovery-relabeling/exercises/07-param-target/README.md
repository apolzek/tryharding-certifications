# 07 · Padrão blackbox: `__param_target` + `__address__`

**Cenário:** exporters "multi-alvo" (blackbox, snmp) são raspados **uma vez por alvo sondado**: `GET prober:8000/probe?target=<alvo>`. A lista em `static_configs` é das coisas a **sondar**; o relabel precisa:
1. copiar o alvo para `__param_target` (vira `?target=` na URL);
2. copiar para `instance` (para a série dizer *o que* foi sondado);
3. trocar `__address__` pelo endereço do prober.

A config quebrada resulta em **1** alvo só, sondando o próprio prober:

```bash
./load.sh exercises/07-param-target/prometheus.yml
curl -s 'localhost:9120/api/v1/targets?scrapePool=probe-apps' | jq -r '.data.activeTargets[].scrapeUrl'
# http://prober:8000/probe?target=prober%3A8000
```
(Os 3 alvos ficaram com labels idênticos e o Prometheus **deduplicou**.)

**Tarefa:** 3 séries `probe_success{job="probe-apps"}`: `payments-api:8000`=1, `search-api:8000`=1, `nao-existe:8000`=0.

> 💡 **Dica:** as regras rodam em **sequência**; cada uma vê o resultado da anterior.

<details><summary>Solução</summary>

```yaml
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: prober:8000
```
```bash
./load.sh solutions/07-param-target/prometheus.yml
curl -s localhost:9120/api/v1/query --data-urlencode 'query=probe_success{job="probe-apps"}' | jq -r '.data.result[] | "\(.metric.instance) \(.value[1])"'
# nao-existe:8000 0
# payments-api:8000 1
# search-api:8000 1
```
É **exatamente** a config da doc do blackbox_exporter (lá o prober é `127.0.0.1:9115` e `params: {module: [http_2xx]}`).
</details>
