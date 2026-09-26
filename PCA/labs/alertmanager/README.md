# Alerting: regras de alerta + Alertmanager

> **Em uma frase:** o **Prometheus** decide *se* algo está errado (regras de alerta) e o **Alertmanager** decide *quem* fica sabendo, *como* e *quando* (dedup, agrupamento, roteamento, inibição, silences).

| | |
|---|---|
| **Stack** | `prom/prometheus:v3.15.0` · `prom/alertmanager:v0.34.1` · 2 apps Python stdlib |
| **Prometheus** | http://localhost:9110 (veja `/alerts` e `/rules`) |
| **Alertmanager** | http://localhost:9111 |
| **Receiver espião** | http://localhost:9112/received (o que foi "enviado") |
| **App de métricas** | http://localhost:9113 (liga/desliga gauges por HTTP) |
| **Teste automático** | [`./test.sh`](test.sh) (~5 min) |

---

## 🧠 Analogia: o alarme de incêndio e a central de despacho

- Os **detectores de fumaça** espalhados pelo prédio são as **regras de alerta** do Prometheus. Eles só sabem dizer "tem fumaça aqui". Um detector sensível demais dispara com torrada queimada, por isso existe o `for:` ("só grite se a fumaça continuar por 1 minuto").
- A **central de despacho dos bombeiros** é o **Alertmanager**. Quando 40 detectores do mesmo andar tocam ao mesmo tempo, a central **não** manda 40 caminhões:
  - **Deduplicação:** o mesmo detector tocando 100 vezes é **um** chamado.
  - **Agrupamento (`group_by`):** "incêndio no 3º andar" é **um** chamado com 40 detectores dentro.
  - **Roteamento (`route`):** incêndio vai para os bombeiros, vazamento de gás para a companhia de gás.
  - **Inibição (`inhibit_rules`):** se o prédio inteiro está sem energia, não adianta avisar que o elevador parou.
  - **Silence:** "vamos testar os detectores do 5º andar das 14h às 15h, ignorem".
  - **`time_intervals`:** "o alarme da cozinha do restaurante fica mudo no horário do almoço, todo dia".

---

## 🏗️ Arquitetura

```
                 scrape 5s                  avalia regras a cada 5s
  ┌──────────┐  ─────────►  ┌────────────────────────────────────────┐
  │ app:9113 │              │ Prometheus :9110                       │
  │ /set ... │              │  rule_files: base.rules.yml + EX/*.rules│
  └──────────┘              │  inactive ─► pending ─(for)─► firing    │
                            │  série ALERTS{alertstate=...}           │
                            │  série ALERTS_FOR_STATE                 │
                            └───────────────┬────────────────────────┘
                     POST /api/v2/alerts    │ só firing e resolved
                                            ▼
┌───────────────────────────── Alertmanager :9111 ──────────────────────────────┐
│ dispatcher: percorre a ÁRVORE DE ROTAS e cria grupos (group_by)                │
│   grupo {alertname=HighLatency, cluster=eu}  ── group_wait / group_interval ─┐ │
│                                                                             ▼ │
│ pipeline por notificação:  inhibit ─► time_intervals ─► silences ─► dedup      │
│                            (nflog)   ─► retry ─► receiver                      │
└──────────────────────────────────────────────────┬────────────────────────────┘
                                                   │ webhook POST /hook/<nome>
                                                   ▼
                                        ┌──────────────────────┐
                                        │ receiver :9112       │
                                        │ GET /received (JSON) │
                                        └──────────────────────┘
```

O que é de quem (cai na prova):

| Responsabilidade | Prometheus | Alertmanager |
|---|---|---|
| Avaliar `expr`, `for`, `keep_firing_for` | ✅ | |
| `labels` / `annotations` e templates `{{ $labels }}` `{{ $value }}` | ✅ | |
| Séries `ALERTS` e `ALERTS_FOR_STATE` | ✅ | |
| Deduplicar, agrupar, rotear, inibir, silenciar | | ✅ |
| Integrações (PagerDuty, Slack, e-mail, webhook...) | | ✅ |
| Repetir a notificação (`repeat_interval`) | | ✅ |

---

## ▶️ Como rodar

```bash
cd labs/alertmanager
docker compose up -d --wait         # sobe com a config completa em ./config/alertmanager.yml
```

A pasta montada como config do Alertmanager (e fonte de regras extras `*.rules.yml` do Prometheus) é a variável **`EX`**:

```bash
EX=./exercises/03-inibicao docker compose up -d --force-recreate --wait   # troca de exercício (estado zerado)
curl -X POST localhost:9111/-/reload     # recarrega o alertmanager.yml depois de editar
curl -X POST localhost:9110/-/reload     # recarrega as regras do Prometheus depois de editar
```

Ferramentas do dia a dia:

```bash
# liga/desliga métricas (todo parâmetro extra vira label)
curl 'localhost:9113/set?name=lab_up&value=0&instance=api-1&cluster=eu'
curl 'localhost:9113/del?name=lab_up&instance=api-1&cluster=eu'
curl localhost:9113/reset                 # apaga todas as séries
curl localhost:9113/list

# o que foi notificado?
curl -s localhost:9112/received | jq -c '.[] | {at, hook, status, groupLabels, alerts: [.alerts[].labels.instance]}'
curl -s 'localhost:9112/received?hook=pager&raw=1' | jq    # payload original completo
curl -X POST localhost:9112/reset
docker compose logs -f receiver                            # uma linha por notificação

# amtool (já vem na imagem do Alertmanager)
alias amtool='docker compose exec -T alertmanager amtool --alertmanager.url=http://localhost:9093'
```

Métricas disponíveis e alertas base ([`prometheus/rules/base.rules.yml`](prometheus/rules/base.rules.yml)):

| Gauge que você cria | Alerta | severity |
|---|---|---|
| `lab_up == 0` | `ServiceDown` | critical |
| `lab_error_ratio > 0.05` | `HighErrorRate` | critical |
| `lab_latency_seconds > 0.5` | `HighLatency` | warning |
| `lab_disk_free_bytes < 10e9` | `DiskFilling` | warning |
| `lab_flappy > 0` | `FlappingCheck` (`keep_firing_for: 30s`) | warning |

---

## 🔍 Passo a passo

### 1. Regra de alerta: anatomia

```yaml
groups:
  - name: lab-base
    interval: 5s                    # sobrescreve o evaluation_interval global só para este grupo
    rules:
      - alert: HighErrorRate        # vira o label alertname
        expr: lab_error_ratio > 0.05   # cada SÉRIE retornada vira UM alerta
        for: 2m                     # tempo em pending antes de firing (opcional, padrão 0)
        keep_firing_for: 5m         # continua firing por 5m depois que a expr some (opcional)
        labels:                     # somados aos labels da série; usados no ROTEAMENTO
          severity: critical
        annotations:                # texto livre para humanos; NÃO participam da identidade
          summary: "{{ $labels.instance }} com {{ $value | humanizePercentage }} de erros"
          runbook_url: https://runbooks.example.com/HighErrorRate
```

- `$labels` é o mapa de labels da série; `$value` é o valor da amostra (float). `$externalLabels` também existe.
- Funções úteis: `humanize` (1.2k), `humanize1024` (1.2Ki), `humanizeDuration` (1m 30s), `humanizePercentage` (0.25 → 25%), `humanizeTimestamp`, `printf "%.2f"`, `query "..."` (roda uma query dentro do template).
- **Labels** mudam a identidade (dois valores diferentes = dois alertas). Por isso **nunca** coloque `{{ $value }}` num label: a cada avaliação nasceria um alerta novo.

### 2. Estados: inactive → pending → firing

```bash
curl -s 'localhost:9113/set?name=lab_up&value=0&instance=api-1&cluster=eu'
curl -s localhost:9110/api/v1/alerts | jq '.data.alerts[] | {a: .labels.alertname, state, activeAt}'
```

Como `ServiceDown` não tem `for`, ele vai direto para `firing` na próxima avaliação (~5s). No exercício [06](exercises/06-pending-for/) você vê o `pending` de 1 minuto.

O Prometheus grava duas séries sintéticas:

```promql
ALERTS                                  # {alertname, alertstate="pending"|"firing", ...labels} = 1
ALERTS{alertstate="firing"}             # o que está pegando fogo agora
ALERTS_FOR_STATE                        # valor = timestamp Unix de quando ficou ativo (activeAt)
```

`ALERTS_FOR_STATE` existe para o Prometheus **restaurar** o relógio do `for` depois de um restart (dentro de `--rules.alert.for-outage-tolerance`, padrão 1h).

**Só alertas `firing` (e os que acabaram de resolver) são enviados ao Alertmanager.** `pending` nunca sai do Prometheus.

### 3. `keep_firing_for`: anti pisca-pisca

```bash
curl -s 'localhost:9113/set?name=lab_flappy&value=1&instance=api-9&cluster=eu'; sleep 12
curl -s 'localhost:9113/set?name=lab_flappy&value=0&instance=api-9&cluster=eu'
watch -n5 "curl -s localhost:9110/api/v1/alerts | jq -c '.data.alerts[] | {a: .labels.alertname, state, keepFiringSince}'"
```

Resultado esperado (testado): a condição some, `keepFiringSince` é preenchido e o alerta **continua `firing` por ~30s** antes de sumir. Só então o Alertmanager manda o `resolved`.

### 4. Roteamento e agrupamento

Config completa comentada: [`config/alertmanager.yml`](config/alertmanager.yml).

```yaml
route:
  receiver: default           # raiz: obrigatória, casa com TUDO, sem matchers
  group_by: [alertname, cluster]
  group_wait: 5s              # 1a notificação de um grupo NOVO espera isso (junta a rajada)
  group_interval: 15s         # grupo JÁ notificado: novidades (novo alerta, resolved) esperam isso
  repeat_interval: 2m         # nada mudou: reenvia depois disso (produção: 4h, 12h...)
  routes:                     # filhas: testadas EM ORDEM, primeira que casa ganha
    - matchers: [team="db"]
      receiver: dba
      continue: true          # ...a não ser que tenha continue: true
    - matchers: [severity="critical"]
      receiver: pager
    - matchers: [severity="warning"]
      receiver: slack
```

- Filhas **herdam** `group_by`, timers e `receiver` do pai quando não definem.
- Se nenhuma filha casa, o alerta fica no **nó pai** (aqui, a raiz → `default`).
- `group_by: ['...']` = agrupar por todos os labels = **sem agrupamento**. `group_by: []` (ou omitido na raiz) = **tudo num grupo só**.
- Matchers: `=`, `!=`, `=~`, `!~`. Regex é **ancorada** (`=~"warn"` ≡ `^warn$`). Sintaxe antiga `match:` / `match_re:` ainda funciona, mas é *deprecated*.

Teste a árvore **sem mandar nada**:

```bash
amtool config routes                                                     # desenha a árvore (da instância rodando)
amtool config routes test --config.file=/etc/alertmanager/alertmanager.yml team=db severity=critical
# dba,pager
amtool config routes test --config.file=/etc/alertmanager/alertmanager.yml --tree severity=warning
amtool config routes test --config.file=/etc/alertmanager/alertmanager.yml --verify.receivers=pager team=db severity=critical
# dba,pager
# WARNING: Expected receivers did not match resolved receivers.   (exit 1)
```

### 5. Inibição

```yaml
inhibit_rules:
  - source_matchers: [severity="critical"]   # quem inibe
    target_matchers: [severity="warning"]    # quem é inibido
    equal: [cluster]                         # precisam ter o mesmo valor nesses labels
```

```bash
curl -s 'localhost:9113/set?name=lab_up&value=0&instance=api-1&cluster=eu'
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-1&cluster=eu'
curl -s 'localhost:9113/set?name=lab_latency_seconds&value=2&instance=api-2&cluster=us'
amtool alert query        # ServiceDown(eu) e HighLatency(us): active
amtool alert query -i     # HighLatency(eu): suppressed
```

Resultado esperado (testado): `pager` recebe `ServiceDown`; `slack` recebe só o `HighLatency` do `us`.

### 6. Silences

```bash
amtool silence add alertname=HighLatency instance=api-2 --duration=30m --author=eu --comment="deploy"
amtool silence query                         # ativos
amtool silence query -q instance=api-2       # só IDs
amtool alert query -s                        # alertas silenciados
amtool silence expire $(amtool silence query -q instance=api-2)
amtool silence query --expired
amtool alert add alertname=TesteManual severity=critical cluster=lab --annotation='summary="injetado via amtool"'
```

Silence usa **matchers** (iguais aos das rotas), tem **início, fim, autor e comentário**, e é guardado no Alertmanager (não na config). Pela UI: http://localhost:9111/#/silences.

### 7. `time_intervals`

```yaml
time_intervals:
  - name: madrugada
    time_intervals:
      - times: [{ start_time: "00:00", end_time: "06:00" }]
        weekdays: ["monday:friday"]
        location: America/Sao_Paulo   # padrão: UTC
route:
  routes:
    - matchers: [team="batch"]
      mute_time_intervals: [madrugada]      # NÃO notifica dentro da janela
    - matchers: [team="suporte-comercial"]
      active_time_intervals: [horario-comercial]   # SÓ notifica dentro da janela
```

Campos: `times`, `weekdays`, `days_of_month` (aceita negativos: `-1` = último dia), `months`, `years`, `location`. `mute_time_intervals` / `active_time_intervals` valem **por rota** (não na raiz).

### 8. Resolved

Quando a `expr` deixa de retornar a série, o Prometheus manda o alerta com `endsAt` = agora. O Alertmanager espera o `group_interval` e envia a notificação com `"status": "resolved"` **se** o receiver tiver `send_resolved: true` (padrão `true` em webhook/PagerDuty/OpsGenie, **`false` em Slack e e-mail**).

```bash
curl -s 'localhost:9113/set?name=lab_up&value=1&instance=api-1&cluster=eu'
sleep 25; curl -s localhost:9112/received | jq -c '.[] | {hook, status}'
```

E se o Prometheus morrer sem mandar o resolved? Todo alerta firing sai do Prometheus com um `endsAt` "no futuro" (≈ 4 × o maior entre `--rules.alert.resend-delay` (1m) e o intervalo de avaliação) e é **reenviado periodicamente**, empurrando o `endsAt` para frente. Se os reenvios param, o `endsAt` vence e o Alertmanager resolve o alerta sozinho. O `resolve_timeout` (padrão 5m) só se aplica a alertas que chegam **sem** `endsAt` (clientes que não são o Prometheus, `amtool alert add`...).

### 9. Alta disponibilidade (conceito)

```
 Prometheus A ──┬──► Alertmanager 1 ◄─┐
                ├──► Alertmanager 2 ◄─┼── gossip (memberlist, :9094): silences + notification log
 Prometheus B ──┴──► Alertmanager 3 ◄─┘
```

- **Cada Prometheus manda para TODOS os Alertmanagers** (lista em `alerting.alertmanagers`). **Nunca** coloque um load balancer na frente.
- Os Alertmanagers formam cluster com `--cluster.peer=am-2:9094 --cluster.peer=am-3:9094` e replicam por **gossip** os **silences** e o **notification log** (quem já foi notificado).
- Cada instância espera `posição no cluster × --cluster.peer-timeout` (15s) antes de enviar; se a anterior já enviou (visto no nflog), ela não reenvia. Resultado: **deduplicação entre instâncias** e garantia *at-least-once* (em partição de rede, pode chegar duplicado; nunca "nenhum").
- Neste lab é uma instância só, com `--cluster.listen-address=` (vazio) para desligar o gossip.

---

## 🏭 Casos reais

### Caso 1: roteamento estilo kube-prometheus (Watchdog + InfoInhibitor)

A árvore padrão que o `kube-prometheus` / `kube-prometheus-stack` gera:

```yaml
route:
  receiver: "null"
  group_by: [namespace]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 12h
  routes:
    - matchers: [alertname="Watchdog"]        # alerta que SEMPRE dispara: prova que o pipeline está vivo
      receiver: "null"                         # (em produção: um dead man's switch, ex. healthchecks.io)
    - matchers: [alertname="InfoInhibitor"]
      receiver: "null"
    - matchers: [severity="critical"]
      receiver: pagerduty
    - receiver: slack                          # rota sem matchers = casa com tudo o que sobrou
inhibit_rules:
  - source_matchers: [severity="critical"]
    target_matchers: [severity=~"warning|info"]
    equal: [namespace, alertname]
  - source_matchers: [severity="warning"]
    target_matchers: [severity="info"]
    equal: [namespace, alertname]
  - source_matchers: [alertname="InfoInhibitor"]   # alertas info só notificam se houver algo mais grave junto
    target_matchers: [severity="info"]
    equal: [namespace]
receivers:
  - name: "null"
```

### Caso 2: PagerDuty + Slack com templates

```yaml
global:
  slack_api_url_file: /etc/alertmanager/secrets/slack-webhook   # nunca commite a URL
  resolve_timeout: 5m
templates:
  - /etc/alertmanager/templates/*.tmpl
receivers:
  - name: pagerduty
    pagerduty_configs:
      - routing_key_file: /etc/alertmanager/secrets/pd-routing-key   # Events API v2
        severity: '{{ .CommonLabels.severity }}'
        description: '{{ .CommonAnnotations.summary }}'
        details:
          firing: '{{ template "pagerduty.default.instances" .Alerts.Firing }}'
  - name: slack
    slack_configs:
      - channel: '#alerts-{{ .CommonLabels.team }}'
        send_resolved: true              # padrão é false no Slack!
        title: '[{{ .Status | toUpper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .CommonLabels.alertname }}'
        text: >-
          {{ range .Alerts }}• {{ .Annotations.summary }} <{{ .Annotations.runbook_url }}|runbook>
          {{ end }}
```

Note a diferença de templates: **na regra** (Prometheus) usa-se `$labels`/`$value`; **no Alertmanager** o dado é a *notificação*: `.Status`, `.Alerts`, `.GroupLabels`, `.CommonLabels`, `.CommonAnnotations`, `.ExternalURL`.

### Caso 3: roteamento por time com sub-árvore

```yaml
route:
  receiver: sre-default
  group_by: [alertname, cluster, namespace]
  routes:
    - matchers: [team="payments"]
      group_by: [alertname, service]       # filha pode redefinir group_by
      receiver: payments-slack
      routes:
        - matchers: [severity="critical"]
          receiver: payments-pagerduty
          repeat_interval: 1h
        - matchers: [alertname=~"Checkout.*"]
          receiver: payments-slack
          active_time_intervals: [horario-comercial]
    - matchers: [namespace=~"kube-system|monitoring"]
      receiver: platform
```

### Caso 4: dead man's switch

```yaml
# regra (sempre verdadeira)
- alert: Watchdog
  expr: vector(1)
  labels: { severity: none }
# rota: manda a cada 1m para um serviço externo que ALERTA quando o ping PARA de chegar
- matchers: [alertname="Watchdog"]
  receiver: deadmansswitch
  repeat_interval: 1m
  group_wait: 0s
  group_interval: 1m
```

---

## 🧪 Exercícios

| # | Exercício | Tipo | Tema |
|---|---|---|---|
| 01 | [roteamento-severidade](exercises/01-roteamento-severidade/) | conserte | matchers, rota raiz |
| 02 | [agrupamento](exercises/02-agrupamento/) | conserte | `group_by`, `'...'` |
| 03 | [inibicao](exercises/03-inibicao/) | conserte | `inhibit_rules`, `equal` |
| 04 | [silenciar](exercises/04-silenciar/) | complete | `amtool silence add/query/expire` |
| 05 | [arvore-de-rotas](exercises/05-arvore-de-rotas/) | conserte | `continue`, ordem, regex ancorada, `routes test` |
| 06 | [pending-for](exercises/06-pending-for/) | conserte | `for`, `pending`, `ALERTS` |
| 07 | [resolved-e-templates](exercises/07-resolved-e-templates/) | conserte | `send_resolved`, `humanizePercentage` |
| 08 | [janela-manutencao](exercises/08-janela-manutencao/) | conserte | `mute_time_intervals` vs `active_time_intervals` |

O [`test.sh`](test.sh) aplica cada solução de [`solutions/`](solutions/) e confere no receiver o que chegou e o que **não** chegou.

---

## ⚠️ Pegadinhas

1. **`for` fica no Prometheus**, `group_wait` no Alertmanager. Tempo mínimo até a primeira notificação ≈ scrape + avaliação + `for` + `group_wait`.
2. **Filhas: primeira que casa ganha.** Sem `continue: true`, as irmãs de baixo nunca são avaliadas.
3. **Regex ancorada**: `severity=~"crit"` não casa com `critical`.
4. **`group_by: ['...']`** desliga o agrupamento. Omitido = tudo num grupo.
5. **`$value` em label** cria um alerta novo a cada avaliação. Use em **annotation**.
6. **Label inexistente** em template vira string vazia, sem erro.
7. **Slack e e-mail têm `send_resolved: false` por padrão.**
8. **Inibição e silence não apagam alertas**: eles continuam visíveis como *suppressed*.
9. **Não coloque load balancer** entre Prometheus e Alertmanagers em HA.
10. **`weekdays: ["monday:sunday"]` é inválido**: o intervalo não "dá a volta"; use `sunday:saturday` ou `monday:friday`.
11. `repeat_interval` curto demais gera fadiga; mas ele nunca reenvia antes do `group_interval`.
12. Editar arquivo não basta: `POST /-/reload` (ou SIGHUP). No Prometheus, só com `--web.enable-lifecycle`.

---

## 🎓 Na prova PCA

**1.** An alerting rule has `for: 5m`. The expression becomes true at 10:00 and stays true. What is the state of the alert at 10:02?
- A) inactive  B) pending  C) firing  D) resolved

<details><summary>Resposta</summary>

**B.** Enquanto o `for` não se cumpre, o alerta fica **pending** (visível em `ALERTS{alertstate="pending"}`) e **não** é enviado ao Alertmanager. Vira firing por volta de 10:05.
</details>

**2.** Which component is responsible for grouping, deduplication and routing of alerts?
- A) Prometheus server  B) Pushgateway  C) Alertmanager  D) Exporters

<details><summary>Resposta</summary>

**C.** O Prometheus só avalia as regras e envia os alertas; dedup, agrupamento, roteamento, inibição, silences e integrações são do **Alertmanager**.
</details>

**3.** A route tree has two child routes. The first matches `severity="critical"` and the second matches `team="db"`. An alert with `severity="critical", team="db"` arrives. Which receivers are notified?
- A) Both  B) Only the first route's receiver  C) Only the second route's receiver  D) The root receiver

<details><summary>Resposta</summary>

**B.** Filhas são avaliadas em ordem e a **primeira que casa** vence. Para também chegar na segunda, a primeira precisaria de `continue: true` (exercício 05).
</details>

**4.** You want warnings to be suppressed while a critical alert fires **for the same cluster**. What do you configure?
- A) A silence with `severity="warning"`
- B) An `inhibit_rules` entry with source `severity="critical"`, target `severity="warning"`, `equal: [cluster]`
- C) `group_by: [cluster]`
- D) `mute_time_intervals`

<details><summary>Resposta</summary>

**B.** Inibição é a supressão **condicionada a outro alerta**. Silence (A) é manual e não depende de outro alerta; `group_by` só junta; `mute_time_intervals` é por horário.
</details>

**5.** Which setting controls how long Alertmanager waits before sending a notification about **new alerts added to a group that was already notified**?
- A) `group_wait`  B) `group_interval`  C) `repeat_interval`  D) `resolve_timeout`

<details><summary>Resposta</summary>

**B.** `group_wait` = primeira notificação de um grupo **novo**; `group_interval` = mudanças num grupo **já notificado**; `repeat_interval` = reenvio quando **nada mudou**.
</details>

**6.** In a highly available setup with 3 Alertmanagers, how should Prometheus be configured?
- A) Send to a load balancer in front of the Alertmanagers
- B) Send to only one Alertmanager; they forward to each other
- C) Send alerts to all Alertmanager instances
- D) Use federation between Alertmanagers

<details><summary>Resposta</summary>

**C.** Cada Prometheus manda para **todos**. Os Alertmanagers usam **gossip** para compartilhar silences e o notification log, deduplicando entre si.
</details>

**7.** What does the `ALERTS_FOR_STATE` series store?
- A) The number of times the alert fired
- B) The Unix timestamp at which the alert became active, used to restore `for` state after a restart
- C) The current value of the expression
- D) The receiver that was notified

<details><summary>Resposta</summary>

**B.** O valor é o `activeAt`. Ao reiniciar, o Prometheus lê essa série e retoma a contagem do `for` em vez de recomeçar do zero.
</details>

**8.** Which `amtool` command validates that an alert with labels `team=db severity=critical` is delivered to the `dba` receiver?
- A) `amtool check-config team=db severity=critical`
- B) `amtool config routes test --verify.receivers=dba team=db severity=critical`
- C) `amtool alert query team=db`
- D) `amtool silence add team=db`

<details><summary>Resposta</summary>

**B.** `config routes test` resolve a árvore para um conjunto de labels; `--verify.receivers` faz sair com código ≠ 0 se o resultado for diferente (ótimo em CI).
</details>

**9.** Which of the following should be an **annotation** rather than a **label** on an alerting rule?
- A) `severity: critical`  B) `team: payments`  C) `summary: "{{ $value }} errors"`  D) `service: checkout`

<details><summary>Resposta</summary>

**C.** Labels identificam o alerta e servem para roteamento/agrupamento. Um valor que muda (como `$value`) em label criaria um alerta novo a cada avaliação.
</details>

---

## 📝 Cola rápida

- Estados: `inactive` → `pending` (`for`) → `firing`. Só firing/resolved vão ao AM. `keep_firing_for` segura o firing depois que a condição some.
- `ALERTS{alertstate}` e `ALERTS_FOR_STATE` (valor = activeAt).
- Templates na regra: `{{ $labels.x }}`, `{{ $value | humanizePercentage }}`. No AM: `.CommonLabels`, `.Alerts.Firing`, `.Status`.
- Timers: `group_wait` (grupo novo) · `group_interval` (mudança no grupo) · `repeat_interval` (reenvio) · `resolve_timeout` (sem `endsAt`).
- Roteamento: raiz casa tudo; filhas em ordem; primeira ganha; `continue: true` para seguir; herança de config; regex ancorada.
- Inibição: `source_matchers` inibe `target_matchers` quando `equal` bate.
- Silence: ad hoc, por matchers, com fim; `amtool silence add|query|expire`.
- `time_intervals` + `mute_time_intervals` (não notifica dentro) / `active_time_intervals` (só notifica dentro).
- HA: Prometheus → **todos** os AMs; gossip replica silences + nflog.
- `amtool check-config` · `amtool config routes [test --verify.receivers=...]` · `amtool alert query [-s|-i]`.

## 📚 Referências

- https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
- https://prometheus.io/docs/prometheus/latest/configuration/template_reference/
- https://prometheus.io/docs/alerting/latest/alertmanager/
- https://prometheus.io/docs/alerting/latest/configuration/
- https://github.com/prometheus/alertmanager#amtool
- https://github.com/prometheus-operator/kube-prometheus
