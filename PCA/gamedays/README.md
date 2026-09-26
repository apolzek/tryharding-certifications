# 🔥 Gamedays: "quebre o monitoramento" (Fase 5)

> **Em uma frase:** incidentes simulados em que **o próprio monitoramento está sutilmente quebrado**. Você recebe o chamado do on-call, investiga com PromQL e APIs, acha a causa raiz, corrige a config e escreve o postmortem. Um script confere se você resolveu de verdade.

Nos labs você aprendeu cada peça isolada. Aqui as peças já estão montadas **e uma delas está errada de um jeito que não aparece em lugar nenhum**: nada fica vermelho, nenhum log grita. É exatamente o tipo de problema que aparece em produção, e o tipo de raciocínio que a prova PCA cobra ("por que este alerta nunca dispara?", "por que esta query volta vazia?").

## 🧠 Analogia: simulado de incêndio

Brigada de incêndio não aprende a apagar fogo lendo o manual; ela faz **simulado**: alguém acende uma fogueira controlada e a equipe tem que achar o extintor certo, com o relógio correndo. Um gameday é isso para on-call: o fogo é controlado (roda na sua máquina, não acorda ninguém), mas o raciocínio é real.

---

## 🗺️ Os cenários

| # | Cenário | O que está quebrado (sem spoiler) | Tópicos PCA | Dificuldade |
|---|---|---|---|---|
| [01](01-alerta-que-nunca-dispara/) | O alerta que nunca dispara | erros intermitentes há 20 min e o pager mudo | `rate()`, janela vs `scrape_interval`, `for:` | ⭐ |
| [02](02-alvo-sumiu/) | O alvo que sumiu | o serviço caiu e nenhum alerta de `up` | `relabel_configs`, `keep`, `absent()` | ⭐ |
| [03](03-explosao-de-cardinalidade/) | Explosão de cardinalidade | Prometheus engordando depois de um deploy | cardinalidade, TSDB status, `metric_relabel_configs`, `sample_limit` | ⭐⭐ |
| [04](04-counter-negativo/) | Throughput negativo | painel com req/s negativo e page falso | counter vs `delta()`, resets, `rate()` | ⭐ |
| [05](05-ninguem-foi-paginado/) | Ninguém foi paginado | alerta firing no Prometheus, pager em silêncio | routing tree, matchers, `inhibit_rules`, `amtool` | ⭐⭐ |
| [06](06-backup-congelado/) | O backup congelado | backup parado há "dias", tudo verde | Pushgateway, `push_time_seconds`, `honor_labels` | ⭐⭐ |
| [07](07-p99-mentiroso/) | O p99 mentiroso | clientes reclamando de lentidão, p99 "ok" | histogramas, `histogram_quantile`, `le`, agregação | ⭐⭐ |
| [08](08-fuso-horario/) | Fuso horário | page às 6h da manhã, silêncio às 16h | `hour()` é UTC, `promtool test rules` | ⭐⭐ |
| [09](09-divisao-vazia/) | A divisão vazia | taxa de erro de 20% e alerta nunca avalia | vector matching, `on`/`ignoring`, agregação | ⭐ |
| [10](10-reload-silencioso/) | O reload silencioso | serviço novo "monitorado" que nunca aparece | `scrape_timeout`, reload, meta-monitoramento | ⭐⭐ |

Cada cenário tem:

```
NN-nome/
├── README.md       # o chamado (📟), sintomas, investigação guiada, causa, correção, como evitar, postmortem
├── scenario/       # a config QUEBRADA (o que está "em produção")
├── solution/       # a config corrigida (spoiler!)
├── pre/            # (só alguns) o estado ANTES do deploy que causou o incidente
├── check/          # (só alguns) testes promtool usados pelo check.sh
├── check.bash      # a verificação que o ./check.sh roda
└── hook.sh         # (só alguns) algo que acontece logo depois de subir (ex.: um push)
```

## 🏗️ Arquitetura

Uma única stack, escrita uma vez em [`_base/`](_base/), parametrizada por cenário:

```
                 ┌───────────── work/  (cópia VIVA da config: é aqui que você edita) ──────────┐
                 │  prometheus/prometheus.yml + rules.yml   alertmanager/alertmanager.yml       │
                 │  app/config.json  (quais "pods" o app finge ser)                             │
                 └──────────────┬───────────────────────────────┬──────────────────────────────┘
                                │                               │
 ┌──────────────┐  scrape  ┌────▼─────────┐   alertas   ┌───────▼──────┐  webhook  ┌───────────────┐
 │ app (python) │◄─────────│  Prometheus  │────────────►│ Alertmanager │──────────►│ app /webhook  │
 │ :9182,9184-9 │          │  :9180       │             │  :9181       │           │ "pager" fake  │
 └──────────────┘          └────▲─────────┘             └──────────────┘           │ GET /received │
 ┌──────────────┐  scrape       │                                                  └───────────────┘
 │ pushgateway  │───────────────┘   (só no cenário 06)
 │ :9183        │
 └──────────────┘
```

- **Prometheus `v3.15.0`** em http://localhost:9180 (com `--web.enable-lifecycle` e `--web.enable-admin-api`).
- **Alertmanager `v0.34.1`** em http://localhost:9181.
- **app** ([`_base/app/app.py`](_base/app/app.py)): alvo fake com perfis (`basic`, `flapping`, `cardinality`, `resetting`, `latency`, `mismatch`), um "pod" por porta, e também o **pager fake**: tudo que o Alertmanager enviar para `http://app:9182/webhook` aparece em http://localhost:9182/received.
- **Pushgateway `v1.11.3`** em http://localhost:9183 (só no cenário 06).

Tudo roda com `scrape_interval: 5s` e `for:` curtinhos, para que um incidente "de horas" aconteça em segundos.

## ▶️ Como rodar

Requisitos: Docker (com compose), `curl` e `jq`.

```bash
cd PCA/gamedays

./start.sh 01        # sobe o cenário 01 QUEBRADO (derruba o anterior)
                     # leia 01-*/README.md: o chamado está lá
vim work/prometheus/rules.yml   # investigue e corrija em work/
./reload.sh          # recarrega Prometheus + Alertmanager (mostra o erro, se houver)
./check.sh 01        # ✅ RESOLVIDO  ou  ❌ AINDA QUEBRADO: <o que ainda está errado>

./solve.sh 01        # desistiu? aplica a solução oficial (spoiler)
./stop.sh            # derruba tudo e apaga os dados
./test.sh            # teste automático de TODOS os cenários (quebrado ❌ → solução ✅)
./test.sh 05         # só um
```

> 💡 `./start.sh NN` sempre **recomeça do zero** (apaga `work/` e os dados). Quer tentar de novo? Rode de novo.

`./check.sh` espera alguns segundos até a condição virar verdade (alerta disparar, série sumir...), então **não se assuste se ele demorar até ~1-2 min** em cenários que dependem de `for:` ou de janelas de `rate()`. Para uma resposta rápida: `CHECK_TIMEOUT=5 ./check.sh 01`.

---

## 🎮 Como funciona um gameday

### Solo (estudo)

1. `./start.sh NN` e abra **só a seção 📟 O chamado** do README. Resista à tentação de ler o resto.
2. Cronometre. Investigue com a UI (http://localhost:9180), `curl` nas APIs e PromQL.
3. Travou? Abra **um** `<details>` da investigação guiada por vez. Cada dica aberta custa pontos (veja a rubrica).
4. Corrija em `work/`, `./reload.sh`, `./check.sh NN`.
5. Escreva o postmortem em 10 minutos usando o [template](POSTMORTEM-TEMPLATE.md) e compare com o exemplo do cenário.

### Em time (2 a 6 pessoas)

| Papel | Quem | O que faz |
|---|---|---|
| **Game master** | 1 pessoa que já conhece o cenário | roda o `./start.sh`, "entrega" o chamado (lê em voz alta ou cola no Slack), responde perguntas **como o sistema responderia** ("o que o `/targets` mostra?" → mostra), controla o relógio e as dicas |
| **Incident commander** | 1 pessoa | coordena, decide hipóteses, pede atualizações a cada 10 min, declara "mitigado" |
| **Investigadores** | 1-3 pessoas | rodam queries, leem config, propõem a correção |
| **Escriba** | 1 pessoa (pode acumular) | anota a linha do tempo **em UTC** enquanto acontece (vira o postmortem) |

Regras do jogo:

- O game master **não diz a resposta**; só revela dicas quando o time pede (e isso custa pontos).
- Toda hipótese é dita em voz alta antes de ser testada ("acho que é o relabel; vou olhar `/api/v1/targets`").
- Ninguém edita `work/` sem avisar o incident commander (em produção, duas pessoas mexendo na mesma config ao mesmo tempo é outro incidente).
- Ao final: 15 minutos de **retro blameless** + postmortem. Troquem os papéis no próximo cenário.

### 🏆 Rubrica de pontuação (100 pontos por cenário)

| Critério | Pontos | Como ganhar |
|---|---|---|
| **Resolveu** | 40 | `./check.sh NN` → ✅ RESOLVIDO |
| **Tempo** | 15 | ≤ 15 min: 15 · ≤ 30 min: 10 · ≤ 45 min: 5 · depois: 0 |
| **Dicas** | 15 | 15 − 5 por `<details>` da investigação aberto (mín. 0). `./solve.sh` zera este item **e** o "Resolveu" |
| **Causa raiz explicada** | 10 | explicou o **mecanismo** (ex.: "regex é ancorada, `prod` não casa com `production`"), não só o sintoma |
| **Como evitar** | 10 | propôs pelo menos **uma** proteção de *detecção* (meta-alerta, `absent()`, teste `promtool`...) além da correção |
| **Postmortem** | 10 | linha do tempo em UTC, impacto, causa, ≥ 3 ações com dono (use o [template](POSTMORTEM-TEMPLATE.md)) |

Faixas: **90+** pronto para on-call · **70-89** sólido · **50-69** revise o lab do tópico · **< 50** refaça o cenário daqui a uma semana.

---

## ⚠️ Pegadinhas gerais (valem para todos os cenários)

- **Editou e esqueceu o reload.** O Prometheus não relê arquivos sozinho. `./reload.sh` e confira `prometheus_config_last_reload_successful`.
- **Reload falhou e ninguém viu.** Um reload com config inválida **mantém a config antiga rodando**. O `./reload.sh` mostra o erro; em produção, só um alerta te conta (cenário 10).
- **"Não tem série" ≠ "valor é zero".** Comparações (`== 0`, `> 0.1`) sobre um resultado vazio devolvem vazio, e vazio **nunca** dispara alerta. Metade destes cenários é variação disso.
- **A UI mostra o que o Prometheus TEM, não o que o seu arquivo DIZ.** Use Status → Configuration e Status → Rules para ver o que está **carregado**.

## 📚 Referências

- [Google SRE Book: Postmortem Culture](https://sre.google/sre-book/postmortem-culture/)
- [Google SRE Workbook: Incident Response](https://sre.google/workbook/incident-response/)
- [Prometheus: Alerting best practices](https://prometheus.io/docs/practices/alerting/)
- [Prometheus: Configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/)
- [Alertmanager: Configuration](https://prometheus.io/docs/alerting/latest/configuration/)
