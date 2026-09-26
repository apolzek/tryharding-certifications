# 08 · Fuso horário

> **Em uma frase:** o alerta do backoffice deveria tocar "só em horário comercial", mas acordou alguém às 6h da manhã e ficou mudo num incidente real às 16h. `hour()` é **UTC**.

| | |
|---|---|
| **Dificuldade** | ⭐⭐ |
| **Tempo-alvo** | 20 min |
| **Tópicos PCA** | `hour()`, `day_of_week()`, `time()`, `vector()`, `and on()`, UTC em todo o Prometheus, `promtool test rules` com `eval_time` |
| **Arquivos que você vai editar** | `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 💬 DM para o SRE de plantão · 09:12                                      │
├──────────────────────────────────────────────────────────────────────────┤
│ Ontem aconteceram duas coisas estranhas com o alerta                     │
│ BackofficeErrosHorarioComercial:                                         │
│                                                                          │
│  • 06:02 (Brasília) ele me ACORDOU. O backoffice nem é usado a essa hora │
│    e o combinado era só tocar das 09h às 18h.                            │
│  • 15:30-17:50 (Brasília) o backoffice ficou com 20% de erro (o          │
│    financeiro não conseguiu fechar o mês) e o alerta NÃO tocou.          │
│                                                                          │
│ Pedido: o alerta deve disparar só entre 09:00 e 17:59 de Brasília        │
│ (UTC-3), nem antes, nem depois.                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 08
# edite work/prometheus/rules.yml  ->  ./reload.sh  ->  ./check.sh 08
```

Como o resultado ao vivo depende da hora em que você está estudando, o `check.sh` deste cenário usa **`promtool test rules`** com um relógio fixo ([`check/tests.yml`](check/tests.yml)): ele simula 08:30, 10:00, 17:30 e 19:00 de Brasília e confere se o alerta dispara só nas horas certas. Além disso, confere que o Prometheus carregou a sua versão.

## 🩺 Sintomas

- O backoffice está com 20% de erro **agora** (o app está configurado assim).
- Se você estiver estudando entre 06h e 15h de Brasília, o alerta está firing; entre 15h e 06h, não. A hora "errada" de disparar é deslocada em exatamente 3 horas.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> que horas o Prometheus acha que são?</summary>

```promql
hour()
```

Compare com o relógio do seu computador (`date`) e com UTC (`date -u`). O Prometheus responde a hora **UTC**. Não existe configuração de fuso horário no servidor Prometheus: todo timestamp é Unix epoch, e `hour()`, `minute()`, `day_of_week()`, `day_of_month()` interpretam em UTC.

```bash
date; date -u
curl -s localhost:9180/api/v1/query --data-urlencode 'query=hour()' | jq -r '.data.result[0].value[1]'
```
</details>

<details>
<summary><b>Passo 2:</b> converta para Brasília.</summary>

Brasília é UTC-3 (sem horário de verão desde 2019). `hour()` aceita um vetor de timestamps:

```promql
hour(vector(time() - 3 * 3600))
```

**Resultado esperado:** a hora local de Brasília. Compare com o `hour()` puro: diferença de 3 (módulo 24).

> Por que não `hour() - 3`? Porque à 01h UTC isso dá **-2**, não 22. O deslocamento tem que ser aplicado no **timestamp**, antes de extrair a hora.
</details>

<details>
<summary><b>Passo 3:</b> reproduza as horas do chamado sem esperar o relógio.</summary>

`promtool test rules` começa o relógio em `1970-01-01 00:00 UTC` e deixa você avaliar em qualquer `eval_time`. Veja o teste que o check usa:

```bash
cat 08-fuso-horario/check/tests.yml
docker run --rm -v "$PWD/work/prometheus:/work:ro" -v "$PWD/08-fuso-horario/check:/check:ro" \
  --entrypoint promtool prom/prometheus:v3.15.0 test rules /check/tests.yml
```

**Resultado esperado (com a regra quebrada):** falha em `11h30m` (08:30 BRT, disparou cedo demais) e em `20h30m` (17:30 BRT, não disparou).
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

A regra usava `and on() hour() >= 9 < 18`, assumindo que `hour()` devolve a hora local. Ela devolve UTC. A janela "09h-18h" virou 09h-18h **UTC** = 06h-15h de Brasília: o alerta passou a disparar 3 horas cedo demais e a ficar mudo nas últimas 3 horas do expediente.
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/prometheus/rules.yml</code></summary>

```yaml
      - alert: BackofficeErrosHorarioComercial
        expr: |
          (
              sum(rate(http_requests_total{job="backoffice", code="500"}[1m]))
            /
              sum(rate(http_requests_total{job="backoffice"}[1m]))
            > 0.05
          )
          and on() hour(vector(time() - 3 * 3600)) >= 9 < 18
        for: 1m
```

```bash
./reload.sh && ./check.sh 08
```
</details>

## 🛡️ Como evitar

- **Não coloque horário comercial dentro da `expr`.** Isso é política de **notificação**, e o Alertmanager tem `time_intervals` com **fuso horário** de verdade (e horário de verão, se o país tiver):

```yaml
# alertmanager.yml
time_intervals:
  - name: horario-comercial
    time_intervals:
      - weekdays: ['monday:friday']
        times:
          - start_time: '09:00'
            end_time: '18:00'
        location: 'America/Sao_Paulo'

route:
  receiver: oncall
  routes:
    - matchers: ['team="backoffice"']
      receiver: backoffice-pager
      active_time_intervals: [horario-comercial]   # só notifica dentro do intervalo
```

  Assim o alerta continua **firing** no Prometheus (visível em dashboards e no histórico `ALERTS`), e só a **notificação** respeita o horário.
- **Se precisar mesmo na PromQL**, isole numa recording rule reutilizável e testada:

```yaml
- record: brt:horario_comercial
  expr: |
    (hour(vector(time() - 3*3600)) >= 9 < 18)
    and on() (day_of_week(vector(time() - 3*3600)) >= 1 <= 5)
```

- **Teste regras com tempo** em `promtool test rules` usando vários `eval_time` (ex.: `11h30m`, `13h`, `20h30m`, `22h` como no [`check/tests.yml`](check/tests.yml)).
- **Postmortems e linhas do tempo sempre em UTC**, com a hora local entre parênteses quando ajudar.

## 📝 Postmortem (exemplo)

> **Resumo:** em 2026-09-25, das 18:30 às 20:50 UTC (15:30-17:50 BRT), o backoffice ficou com ~20% de erro e o fechamento mensal do financeiro atrasou 1 dia. O alerta `BackofficeErrosHorarioComercial` não notificou. Às 09:02 UTC (06:02 BRT) o mesmo alerta havia acordado o on-call fora do horário combinado.
>
> **Causa raiz:** a expressão usava `hour()` assumindo hora local; `hour()` é UTC. A janela efetiva era 06h-15h BRT.
>
> **Ações:**
> 1. (corrigir) converter o timestamp (`hour(vector(time() - 3*3600))`). ✅
> 2. (prevenir) migrar restrições de horário para `time_intervals` do Alertmanager com `location: America/Sao_Paulo`. **Dono:** SRE.
> 3. (prevenir) testes `promtool` com `eval_time` nas bordas do horário. **Dono:** time backoffice.

## 🎓 Na prova PCA

<details>
<summary>Q1. In which timezone does PromQL's <code>hour()</code> return its result?</summary>

**UTC**, sempre. Não existe configuração de timezone no Prometheus para funções de data.
</details>

<details>
<summary>Q2. Where should "only notify during business hours" preferably be implemented?</summary>

No **Alertmanager**, com `time_intervals` + `active_time_intervals` (ou `mute_time_intervals`) na rota, que suportam `location` (timezone).
</details>
