# 05 · Ninguém foi paginado

> **Em uma frase:** o checkout caiu, o alerta `CheckoutDown` ficou **firing** no Prometheus por 40 minutos... e o pager do time payments não tocou. O problema está entre o Prometheus e o celular.

| | |
|---|---|
| **Dificuldade** | ⭐⭐ |
| **Tempo-alvo** | 20 min |
| **Tópicos PCA** | routing tree, `matchers`, receiver default, `inhibit_rules` e `equal`, `amtool config routes test`, API v2 do Alertmanager |
| **Arquivos que você vai editar** | `work/alertmanager/alertmanager.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ ☎️  Ligação do gerente de plantão para o SRE · 16:48                     │
├──────────────────────────────────────────────────────────────────────────┤
│ "O checkout está fora há quase uma hora, o comercial descobriu pelo      │
│  Twitter. Eu abri o Prometheus e o alerta CheckoutDown está VERMELHO     │
│  desde 16:05. O on-call de payments jura que o celular não tocou.        │
│  Como assim o alerta dispara e ninguém recebe?"                          │
│                                                                          │
│ Pedido: fazer o CheckoutDown chegar no pager do time payments            │
│ (receiver pager-payments). Não mexa nas regras do Prometheus: elas       │
│ estão certas.                                                            │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 05
# edite work/alertmanager/alertmanager.yml  ->  ./reload.sh  ->  ./check.sh 05
```

O "pager" do time payments é o webhook do app: tudo que ele recebe aparece em http://localhost:9182/received.

## 🩺 Sintomas

- http://localhost:9180/alerts: `CheckoutDown` **firing**.
- http://localhost:9182/received: `[]`, o pager nunca recebeu nada.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> o alerta chegou no Alertmanager?</summary>

```bash
curl -s localhost:9181/api/v2/alerts | jq -c '.[] | {alert: .labels.alertname, labels: .labels, state: .status.state, receivers: [.receivers[].name]}'
```

**Resultado esperado:** dois alertas. `CheckoutDown` está no Alertmanager (então o Prometheus fez a parte dele), mas:
- `receivers: ["default-null"]`, ou seja, foi roteado para o receiver **padrão**, que não tem nenhuma integração (buraco negro);
- `state: "suppressed"`, ou seja, algo também o está suprimindo.

São **dois** problemas. Vamos um de cada vez.

> Na UI do Alertmanager (http://localhost:9181), marque "Show inhibited" para ver alertas suprimidos.
</details>

<details>
<summary><b>Passo 2:</b> por que a rota de payments não casou?</summary>

Pergunte ao próprio Alertmanager para onde um alerta com esses labels iria:

```bash
docker exec pca-gamedays-alertmanager-1 amtool config routes show \
  --config.file=/etc/alertmanager/alertmanager.yml

docker exec pca-gamedays-alertmanager-1 amtool config routes test \
  --config.file=/etc/alertmanager/alertmanager.yml \
  alertname=CheckoutDown team=payments severity=critical
# default-null
```

A árvore mostra `{team="payment"}`. O alerta tem `team="payments"`. Matchers de igualdade são **exatos**: um `s` a menos e o alerta cai no receiver da raiz.
</details>

<details>
<summary><b>Passo 3:</b> quem está suprimindo o alerta?</summary>

```bash
curl -s localhost:9181/api/v2/alerts | jq -c '.[] | {alert: .labels.alertname, state: .status.state, inhibitedBy: .status.inhibitedBy}'
```

`CheckoutDown` tem `inhibitedBy: ["<fingerprint>"]`. O fingerprint é do alerta `DatalakeEmManutencao`. Olhe a `inhibit_rules`:

```bash
docker exec pca-gamedays-alertmanager-1 cat /etc/alertmanager/alertmanager.yml | grep -A4 inhibit_rules
```

A regra diz: "enquanto `DatalakeEmManutencao` estiver firing, suprima **qualquer** alerta `critical` ou `warning`". Sem `equal:`, uma inibição vale para **todos** os alertas que casem com `target_matchers`, de qualquer time, qualquer serviço. E o datalake está "em manutenção" há 3 meses (alguém esqueceu a flag ligada).
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

Duas falhas no Alertmanager, qualquer uma delas sozinha já teria impedido o page:

1. **Typo no matcher da rota:** `team="payment"` em vez de `team="payments"`. O alerta cai na rota raiz, cujo receiver `default-null` não tem integração nenhuma. Receiver vazio é válido na config, então nada reclama.
2. **Inhibit rule ampla demais:** criada pelo time data para a manutenção do datalake, sem `equal: [team]` e com `target_matchers` genérico (`severity=~"critical|warning"`). Enquanto o alerta de manutenção fica ativo, **todo** alerta crítico da empresa é suprimido.
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/alertmanager/alertmanager.yml</code></summary>

```yaml
route:
  receiver: oncall-geral          # raiz NUNCA é buraco negro
  group_by: [alertname]
  routes:
    - matchers: ['team="payments"']
      receiver: pager-payments
    - matchers: ['team="data"']
      receiver: slack-data

inhibit_rules:
  - source_matchers: ['alertname="DatalakeEmManutencao"']
    target_matchers: ['severity=~"critical|warning"']
    equal: [team]                 # só inibe alertas do MESMO time do alerta de origem

receivers:
  - name: oncall-geral
    webhook_configs:
      - url: http://app:9182/webhook
  - name: slack-data
  - name: pager-payments
    webhook_configs:
      - url: http://app:9182/webhook
        send_resolved: true
```

Valide antes de recarregar:

```bash
docker run --rm -v "$PWD/work/alertmanager:/a:ro" --entrypoint amtool \
  prom/alertmanager:v0.34.1 check-config /a/alertmanager.yml
./reload.sh && ./check.sh 05
curl -s localhost:9182/received | jq '.[-1]'
```

Arquivo completo em [`solution/alertmanager/alertmanager.yml`](solution/alertmanager/alertmanager.yml).
</details>

## 🛡️ Como evitar

- **Teste de roteamento no CI**, com os labels reais de cada alerta `page`:

```bash
amtool config routes test --config.file=alertmanager.yml --verify.receivers=pager-payments \
  alertname=CheckoutDown team=payments severity=critical
# exit != 0 se o receiver não for o esperado
```

- **O receiver da raiz sempre notifica alguém** (um canal "alertas-sem-dono" monitorado), nunca um receiver vazio.
- **Toda `inhibit_rule` tem `equal:`** (ex.: `[team]`, `[cluster]`, `[instance]`). Inibição sem `equal` é "desligue todo o resto do mundo".
- **Manutenção programada = silence com prazo**, não alerta sempre ligado + inhibit. Silêncios expiram sozinhos; flags esquecidas não:

```bash
amtool silence add --alertmanager.url=http://localhost:9181 \
  --duration=2h --comment="manutenção datalake CHG-1234" team=data
```

- **Heartbeat ponta a ponta (Watchdog / dead man's switch):** um alerta que **sempre** dispara (`expr: vector(1)`), roteado para um serviço externo que te avisa quando ele **para** de chegar. Pega Prometheus → Alertmanager → receiver quebrados.

## 📝 Postmortem (exemplo)

> **Resumo:** o checkout ficou indisponível das 16:05 às 17:02 UTC (57 min). O alerta `CheckoutDown` disparou às 16:05, mas nenhum page foi entregue. Detecção por clientes em rede social às 16:40.
>
> **Causa raiz:** (1) matcher `team="payment"` (typo) na rota de payments, introduzido na migração de `match:` para `matchers:` em março; (2) inhibit rule do time data sem `equal`, suprimindo todo alerta crítico enquanto `DatalakeEmManutencao` estava ativo (ativo desde junho).
>
> **Onde tivemos sorte:** o problema só apareceu num incidente de checkout; outros times estavam igualmente sem page há meses.
>
> **Ações:**
> 1. (corrigir) matcher e `equal: [team]`. ✅
> 2. (prevenir) `amtool config routes test --verify.receivers` no CI para todos os alertas `page`. **Dono:** plataforma.
> 3. (detectar) Watchdog com dead man's switch externo. **Dono:** SRE.
> 4. (prevenir) manutenção via silence com prazo; proibir inhibit sem `equal` (lint). **Dono:** plataforma.

## 🎓 Na prova PCA

<details>
<summary>Q1. An inhibit rule has <code>source_matchers: [alertname="NodeDown"]</code>, <code>target_matchers: [severity="warning"]</code> and no <code>equal</code>. What does it do while NodeDown fires on one node?</summary>

Suprime **todos** os alertas `severity="warning"` de **todos** os nós/serviços. Com `equal: [instance]`, só os warnings do mesmo nó.
</details>

<details>
<summary>Q2. Which command shows which receiver a given label set would be routed to?</summary>

`amtool config routes test --config.file=alertmanager.yml label1=value1 label2=value2`.
</details>
