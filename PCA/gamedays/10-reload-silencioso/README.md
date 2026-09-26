# 10 · O reload silencioso

> **Em uma frase:** o PR que "monitora o novo serviço inventory" foi aprovado, mergeado e deployado. O inventory nunca apareceu no Prometheus. O reload falhou, o Prometheus seguiu feliz com a config **antiga**, e ninguém ficou sabendo.

| | |
|---|---|
| **Dificuldade** | ⭐⭐ |
| **Tempo-alvo** | 15 min |
| **Tópicos PCA** | `scrape_interval` vs `scrape_timeout`, `/-/reload`, `prometheus_config_last_reload_successful`, `promtool check config`, meta-monitoramento |
| **Arquivos que você vai editar** | `work/prometheus/prometheus.yml` e `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ TICKET #91002 · time inventory → SRE                                     │
├──────────────────────────────────────────────────────────────────────────┤
│ O PR #5120 (adiciona o job inventory no Prometheus) foi mergeado e o     │
│ pipeline de deploy da config ficou verde ontem. Mas:                     │
│  - up{job="inventory"} não existe;                                       │
│  - o alerta InventoryDown que veio no mesmo PR não aparece em /alerts.   │
│ O app está no ar (curl no /metrics responde).                            │
│                                                                          │
│ Pedido:                                                                  │
│  1. inventory sendo raspado;                                             │
│  2. um alerta PrometheusConfigReloadFailed para que um reload quebrado   │
│     NUNCA MAIS passe despercebido.                                       │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 10     # sobe com a config antiga e depois aplica o "deploy" do PR (via reload)
# edite work/prometheus/*.yml  ->  ./reload.sh  ->  ./check.sh 10
```

## 🩺 Sintomas

- `up{job="inventory"}` → vazio.
- `cat work/prometheus/prometheus.yml` **tem** o job `inventory`. Mas o Prometheus não.
- `curl -s localhost:9184/metrics | head -3` responde.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> o arquivo diz uma coisa; o que o Prometheus tem carregado?</summary>

```bash
curl -s localhost:9180/api/v1/status/config | jq -r '.data.yaml' | grep job_name
# - job_name: prometheus
# - job_name: orders
```

Na UI: Status → Configuration. O job `inventory` **não está** na config em memória.
</details>

<details>
<summary><b>Passo 2:</b> o último reload deu certo?</summary>

```promql
prometheus_config_last_reload_successful
```

**Resultado esperado:** **0**. E há quanto tempo a config atual está carregada:

```promql
time() - prometheus_config_last_reload_success_timestamp_seconds
```

Um reload que falha **não derruba** o Prometheus: ele loga o erro e continua rodando com a última config válida. É um comportamento seguro... e silencioso.
</details>

<details>
<summary><b>Passo 3:</b> qual é o erro?</summary>

```bash
docker logs pca-gamedays-prometheus-1 2>&1 | grep -i 'error reloading'
```

ou valide o arquivo sem precisar do servidor:

```bash
docker run --rm -v "$PWD/work/prometheus:/p:ro" --entrypoint promtool \
  prom/prometheus:v3.15.0 check config /p/prometheus.yml
```

**Resultado esperado:** `scrape timeout greater than scrape interval for scrape config with job name "inventory"`. O job herdou `scrape_interval: 5s` do `global` e definiu `scrape_timeout: 10s`.
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

O PR definiu `scrape_timeout: 10s` num job que herda `scrape_interval: 5s`. O Prometheus exige `scrape_timeout <= scrape_interval`, rejeitou a config inteira no reload e continuou com a anterior (sem o job `inventory` e sem as regras novas). O pipeline de deploy só copiava o arquivo e chamava `/-/reload`, sem checar o status HTTP (500) nem validar com `promtool` antes. E não havia alerta sobre `prometheus_config_last_reload_successful`.
</details>

## 🔧 Correção

<details>
<summary>Spoiler</summary>

`work/prometheus/prometheus.yml`:

```yaml
  - job_name: inventory
    scrape_interval: 10s   # /metrics lento? aumente o intervalo junto
    scrape_timeout: 10s    # timeout <= interval
    static_configs:
      - targets: ['app:9184']
```

`work/prometheus/rules.yml` (grupo novo):

```yaml
  - name: meta
    rules:
      - alert: PrometheusConfigReloadFailed
        expr: prometheus_config_last_reload_successful == 0
        for: 1m
        labels:
          severity: page
        annotations:
          summary: "{{ $labels.instance }} não conseguiu recarregar a config: está rodando a versão ANTIGA"
```

```bash
./reload.sh && ./check.sh 10
```

> 💡 Repare que o `./reload.sh` mostra o erro na hora quando a config é inválida. Tente: coloque `scrape_timeout: 30s` no job inventory, rode `./reload.sh` e depois desfaça.
</details>

## 🛡️ Como evitar

- **`promtool check config` (e `check rules`) no CI**, antes do merge. Pega `scrape_timeout > scrape_interval`, YAML inválido, regra com PromQL inválida, arquivo de regras inexistente.
- **O deploy confere o reload:** `curl -fsS -X POST http://prometheus:9090/-/reload` (o `-f` faz o curl falhar com HTTP 500) e/ou checa `prometheus_config_last_reload_successful` depois.
- **Meta-monitoramento padrão** (vem pronto no [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) / [awesome-prometheus-alerts](https://samber.github.io/awesome-prometheus-alerts/)):

```yaml
- alert: PrometheusConfigReloadFailed
  expr: prometheus_config_last_reload_successful == 0
  for: 5m
- alert: AlertmanagerConfigReloadFailed
  expr: alertmanager_config_last_reload_successful == 0
  for: 5m
- alert: PrometheusRuleFailures
  expr: increase(prometheus_rule_evaluation_failures_total[5m]) > 0
- alert: PrometheusNotConnectedToAlertmanagers
  expr: prometheus_notifications_alertmanagers_discovered < 1
  for: 5m
```

- **Lembre das regras de `scrape_timeout`:** padrão 10s; nunca maior que `scrape_interval`; se você só define `scrape_interval: 5s` no job, o timeout padrão (10s) é **ajustado** automaticamente para 5s. O erro acontece quando você define os dois de forma incoerente.

## 📝 Postmortem (exemplo)

> **Resumo:** de 2026-09-24 17:40 UTC a 2026-09-25 10:15 UTC (~16h30), todas as mudanças de configuração do Prometheus (incluindo o novo job inventory e 3 alertas novos de outros times) ficaram sem efeito. O inventory rodou sem monitoramento nesse período.
>
> **Causa raiz:** `scrape_timeout: 10s` > `scrape_interval: 5s` no job inventory. O Prometheus rejeitou a config no reload e manteve a anterior; o pipeline ignorava o HTTP 500 do `/-/reload`.
>
> **O que deu certo:** o comportamento do Prometheus (manter a config válida anterior) evitou perda de monitoramento dos serviços existentes.
>
> **Ações:**
> 1. (corrigir) `scrape_interval: 10s` e `scrape_timeout: 10s` no inventory. ✅
> 2. (detectar) `PrometheusConfigReloadFailed` e `AlertmanagerConfigReloadFailed`. ✅
> 3. (prevenir) `promtool check config` no CI do repositório de config. **Dono:** plataforma.
> 4. (prevenir) pipeline falha se o `/-/reload` não retornar 200. **Dono:** plataforma.

## 🎓 Na prova PCA

<details>
<summary>Q1. What happens when you POST to <code>/-/reload</code> with an invalid configuration file?</summary>

O reload falha (HTTP 500 + log de erro), `prometheus_config_last_reload_successful` vira 0, e o Prometheus **continua rodando com a última configuração válida**.
</details>

<details>
<summary>Q2. Is <code>scrape_interval: 15s</code> with <code>scrape_timeout: 20s</code> a valid scrape config?</summary>

**Não.** `scrape_timeout` não pode ser maior que `scrape_interval`; o Prometheus rejeita a configuração.
</details>
