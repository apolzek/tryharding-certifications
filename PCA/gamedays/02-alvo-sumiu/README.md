# 02 · O alvo que sumiu

> **Em uma frase:** o serviço `payments` "está fora do monitoramento" desde um PR inocente, e o alerta `PaymentsDown` (`up == 0`) nunca vai disparar, porque `up{job="payments"}` **nem existe**.

| | |
|---|---|
| **Dificuldade** | ⭐ |
| **Tempo-alvo** | 15 min |
| **Tópicos PCA** | `relabel_configs`, `action: keep`, regex ancorada, `up`, `absent()`, `/api/v1/targets` |
| **Arquivos que você vai editar** | `work/prometheus/prometheus.yml` e `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 🔔 Slack #sre-oncall · 09:47                                             │
├──────────────────────────────────────────────────────────────────────────┤
│ @oncall o painel "Payments - Overview" está todo "No data" desde ontem   │
│ à tarde. Achei que era o Grafana, mas no Prometheus também não aparece   │
│ nada do job payments. E o PaymentsDown não disparou em momento nenhum.   │
│ O serviço está no ar? A gente está cego?                                 │
│                                                                          │
│ Pedido:                                                                  │
│  1. voltar a coletar o payments;                                         │
│  2. garantir que o PaymentsDown dispare também quando o target SOME      │
│     (não só quando ele responde com erro).                               │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 02
# edite work/prometheus/*.yml   ->  ./reload.sh  ->  ./check.sh 02
```

## 🩺 Sintomas

- `up{job="payments"}` → **Empty query result**.
- http://localhost:9180/targets não mostra o job `payments` com nenhum target ativo.
- O app está vivo: `curl -s localhost:9182/metrics | head` responde normalmente.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> o alerta está "ok" ou está cego?</summary>

```promql
up{job="payments"}
```

**Resultado esperado:** vazio. A regra `up{job="payments"} == 0` compara **nada** com zero: resultado vazio, alerta inactive para sempre. "Verde" aqui significa "não sei".
</details>

<details>
<summary><b>Passo 2:</b> o Prometheus sabe que esse target existe?</summary>

```bash
curl -s localhost:9180/api/v1/targets | jq '.data.droppedTargetCounts'
# { "payments": 1, "prometheus": 0 }
```

**Um target foi descartado** no job `payments`. Veja qual e com quais labels:

```bash
curl -s 'localhost:9180/api/v1/targets?state=dropped' | jq '.data.droppedTargets[].discoveredLabels'
```

**Resultado esperado:** entre outros labels internos (`__scheme__`, `__scrape_interval__`...), `__address__: "app:9182"`, `env: "production"`, `job: "payments"`, `team: "payments"`. Descoberto, mas **jogado fora pelo relabeling** antes do scrape.

> Na UI: Status → Service Discovery → `payments` mostra os targets descobertos e quais viraram "Dropped".
</details>

<details>
<summary><b>Passo 3:</b> qual regra de relabel descartou?</summary>

```bash
curl -s localhost:9180/api/v1/status/config | jq -r '.data.yaml' \
  | sed -n '/job_name: payments/,$p' | grep -A5 relabel_configs
#   relabel_configs:
#   - source_labels: [env]
#     separator: ;
#     regex: prod
#     replacement: $1
#     action: keep
```

O `keep` exige que `env` case com `prod`. O label do target é `production`. Em relabeling, **a regex é ancorada** (`^prod$`): `prod` **não** casa com `production`.
</details>

<details>
<summary><b>Passo 4:</b> como fazer o alerta enxergar "sumiu"?</summary>

```promql
absent(up{job="payments"})
```

**Resultado esperado:** `{job="payments"} 1`. O `absent()` devolve 1 **justamente quando não há série**, herdando os labels de igualdade do seletor. É o complemento que faltava ao `== 0`.
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

Um PR adicionou um `relabel_configs` com `action: keep` e `regex: prod` para "só raspar produção". Os targets usam `env: production`. Como as regex do relabeling são **ancoradas nas duas pontas**, nenhum target casou e **todos foram descartados antes do primeiro scrape**. Sem scrape, não existe `up{job="payments"}`, e o alerta `up == 0` não tem sobre o que comparar.

Duas falhas: a do relabel (gatilho) e a do alerta que só cobre "target responde mal", nunca "target sumiu" (causa de não ter sido detectado).
</details>

## 🔧 Correção

<details>
<summary>Spoiler</summary>

`work/prometheus/prometheus.yml`:

```yaml
    relabel_configs:
      - source_labels: [env]
        regex: prod|production      # ou: production
        action: keep
```

`work/prometheus/rules.yml`:

```yaml
      - alert: PaymentsDown
        expr: up{job="payments"} == 0 or absent(up{job="payments"})
        for: 30s
```

```bash
./reload.sh && ./check.sh 02
```

O `check.sh` confere o target no ar **e** roda um `promtool test rules` em que a série `up{job="payments"}` não existe (arquivo [`check/tests.yml`](check/tests.yml)).
</details>

## 🛡️ Como evitar

- **Todo job crítico tem um `absent()`** (ou um alerta genérico de "job sem targets"):

```yaml
- alert: JobSemTargets
  expr: |
    absent(up{job="payments"})
    or absent(up{job="checkout"})
  for: 5m
  labels: {severity: page}
```

- **Meta-alerta de targets descartados** (funciona para qualquer job):

```yaml
# prometheus_sd_discovered_targets conta os descobertos por job ("config") ANTES do relabel
# (name="scrape" exclui o SD dos Alertmanagers, que tem name="notify")
- alert: ScrapePoolVazio
  expr: |
    sum by (config) (prometheus_sd_discovered_targets{name="scrape"}) > 0
    unless on(config) label_replace(count by (job) (up), "config", "$1", "job", "(.*)")
  for: 10m
  labels: {severity: ticket}
```

- **Teste o relabel antes do merge.** `promtool check config` valida sintaxe mas não diz "este keep vai descartar tudo". Revise na UI de staging (Status → Service Discovery) ou em https://relabeler.promlabs.com.
- **Prefira `regex` explícita e documentada**, ex.: `regex: (prod|production)` com comentário, e padronize os valores de `env` entre times.

## 📝 Postmortem (exemplo)

> **Resumo:** de 15:10 UTC (D-1) a 09:55 UTC (D0), o serviço `payments` ficou sem nenhuma coleta de métricas. Não houve indisponibilidade do serviço, mas por ~19h qualquer falha teria passado sem alerta.
>
> **Causa raiz:** `relabel_configs` com `action: keep` e `regex: prod`; os targets usam `env: production` e a regex de relabel é ancorada, então todos foram descartados. O alerta `PaymentsDown` usava apenas `up == 0`, que não dispara quando a série não existe.
>
> **Detecção:** manual, por um dev estranhando o dashboard vazio.
>
> **Ações:**
> 1. (corrigir) regex `prod|production`. ✅
> 2. (detectar) `absent(up{job=...})` em todos os alertas de disponibilidade. **Dono:** SRE.
> 3. (detectar) alerta `ScrapePoolVazio` para qualquer job com targets descobertos e zero ativos. **Dono:** plataforma.
> 4. (prevenir) padronizar o label `env` (`production`) e documentar no guia de onboarding. **Dono:** plataforma.

## 🎓 Na prova PCA

<details>
<summary>Q1. A relabel rule has <code>source_labels: [env]</code>, <code>regex: prod</code>, <code>action: keep</code>. A target has <code>env="production"</code>. What happens?</summary>

**O target é descartado.** Regex de relabel é ancorada (`^(?:prod)$`), então `production` não casa, e `keep` descarta quem não casa.
</details>

<details>
<summary>Q2. Why does <code>up{job="x"} == 0</code> not fire when the target is removed from service discovery? What expression covers both cases?</summary>

Porque sem target não existe série `up{job="x"}`; comparar vazio dá vazio. Use `up{job="x"} == 0 or absent(up{job="x"})`.
</details>
