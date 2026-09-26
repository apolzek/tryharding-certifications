# 04 · Throughput negativo

> **Em uma frase:** o painel "API: req/s" mostra valores **negativos** de tempos em tempos, e o alerta `ApiSemTrafego` fica acordando o on-call à toa. Alguém usou `delta()` num counter.

| | |
|---|---|
| **Dificuldade** | ⭐ |
| **Tempo-alvo** | 15 min |
| **Tópicos PCA** | tipos de métrica (counter vs gauge), resets, `rate()` vs `delta()`/`deriv()`, `resets()`, "rate primeiro, agrega depois" |
| **Arquivos que você vai editar** | `work/prometheus/rules.yml` |

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ 📟 PAGE · ApiSemTrafego · severity=page · 03:14                          │
│ API recebendo -0.91 req/s                                                │
├──────────────────────────────────────────────────────────────────────────┤
│ (4ª vez esta noite. resolveu sozinho em 15s, de novo.)                   │
│                                                                          │
│ Mensagem do on-call no canal:                                            │
│ "req/s NEGATIVO?? como assim a API recebeu menos que zero requisições?   │
│  o painel de throughput está cheio de buracos pra baixo de zero. vou     │
│  silenciar esse alerta até alguém olhar."                                │
│                                                                          │
│ Pedido: fazer a recording rule job:http_requests:rate1m (que alimenta o  │
│ painel e o alerta) mostrar o throughput real (~10 req/s) SEMPRE.         │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 04
# espere ~1 min para ter dados, depois investigue
# edite work/prometheus/rules.yml  ->  ./reload.sh  ->  ./check.sh 04
```

## 🩺 Sintomas

- `job:http_requests:rate1m` no modo **Graph** (últimos 5 min): em torno de 10, com mergulhos até valores negativos a cada ~45s.
- `ApiSemTrafego` alterna entre *firing* e *inactive*.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> olhe o counter cru.</summary>

```promql
http_requests_total{job="api"}
```

Em **Graph**: um **dente de serra**. Sobe até ~450 e despenca para 0 a cada 45s. O processo está reiniciando (OOM, crashloop...) e todo counter volta a zero quando o processo recomeça.

Conte os resets:

```promql
resets(http_requests_total{job="api"}[5m])
```

**Resultado esperado:** um reset a cada 45s (≈ 6 depois de 5 minutos de cenário rodando).
</details>

<details>
<summary><b>Passo 2:</b> o que a recording rule faz?</summary>

```bash
curl -s localhost:9180/api/v1/rules | jq -r '.data.groups[].rules[] | select(.type=="recording") | .name + " = " + .query'
# job:http_requests:rate1m = sum by (job) (delta(http_requests_total[1m])) / 60
```

`delta()` é para **gauges**: calcula `último − primeiro` (extrapolado) e **não sabe o que é reset**. Se o counter foi de 430 para 0 e depois subiu até 150, `delta` diz "caiu 280": negativo.
</details>

<details>
<summary><b>Passo 3:</b> compare lado a lado.</summary>

```promql
sum by (job) (delta(http_requests_total{job="api"}[1m])) / 60
sum by (job) (rate(http_requests_total{job="api"}[1m]))
```

**Resultado esperado:** a linha do `delta` oscila e fica negativa; a do `rate` fica estável em ~**10**. O `rate()` detecta a queda, assume que o counter recomeçou do zero e soma o que veio depois.

Bônus: `deriv()` (regressão linear, também só para gauges) sofre do mesmo mal:

```promql
deriv(http_requests_total{job="api"}[1m])
```
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

A recording rule que alimenta o painel e o alerta usa `delta()` sobre um **counter**. `delta()`/`idelta()`/`deriv()` são funções de **gauge**: não tratam resets. Com o pod reiniciando a cada ~45s, toda janela de 1 minuto que contém um reset pode dar um valor negativo ou muito baixo, disparando `ApiSemTrafego < 1`.

O restart em si é um problema real (e merece alerta próprio), mas o painel e o alerta de tráfego não deveriam mentir por causa dele.
</details>

## 🔧 Correção

<details>
<summary>Spoiler: <code>work/prometheus/rules.yml</code></summary>

```yaml
      - record: job:http_requests:rate1m
        expr: sum by (job) (rate(http_requests_total[1m]))

      - alert: ApiSemTrafego
        expr: job:http_requests:rate1m{job="api"} < 1
        for: 1m

      # o sintoma real (restarts) com o seu próprio sinal
      - alert: ApiReiniciandoMuito
        expr: resets(http_requests_total{job="api"}[10m]) > 3
        labels: {severity: ticket}
```

```bash
./reload.sh && ./check.sh 04
```

O check exige que, nos últimos 50s, a série fique **sempre entre 5 e 15 req/s**, então ele espera a janela "limpar" dos valores antigos (~50s depois do reload). Maquiagem como `clamp_min(..., 0)` não passa: zero req/s também é mentira.
</details>

## 🛡️ Como evitar

- **Counter → `rate()`, `irate()`, `increase()`, `resets()`.** **Gauge → `delta()`, `idelta()`, `deriv()`, `predict_linear()`, `*_over_time()`.** Decore a tabela.
- **Rate primeiro, agrega depois:** `sum(rate(x[5m]))`, **nunca** `rate(sum(x)[5m:])`. A soma de vários counters não é um counter "honesto": quando um pod reinicia a soma cai, e o `rate` do subquery vê isso como um reset gigante (pico falso).
- **Nome da recording rule conta a história:** `level:metric:operations` (`job:http_requests:rate1m`). Quem lê `rate1m` espera um `rate()`. Um linter pode checar isso.
- **Lint automático:** [pint](https://github.com/cloudflare/pint) tem o check `promql/counter` que reclama de `delta()` em métrica com sufixo `_total`.
- **Teste com reset no `promtool`:**

```yaml
tests:
  - interval: 5s
    input_series:
      - series: 'http_requests_total{job="api", instance="a"}'
        values: '0+50x8 0+50x8 0+50x8'     # 10 req/s com dois restarts
    promql_expr_test:
      - expr: job:http_requests:rate1m < 5
        eval_time: 2m
        exp_samples: []                    # nunca abaixo de 5
```

## 📝 Postmortem (exemplo)

> **Resumo:** das 01:10 às 04:30 UTC o alerta `ApiSemTrafego` disparou 9 vezes, todas falsas. O on-call silenciou o alerta às 03:20; se a API tivesse caído de verdade, ninguém teria sido avisado até 08:00.
>
> **Causa raiz:** a recording rule `job:http_requests:rate1m` usava `delta()` sobre um counter. Os restarts do pod (OOM, investigado em outro postmortem) produziam valores negativos.
>
> **O que deu errado:** o silêncio foi criado sem data de expiração curta e sem ticket associado.
>
> **Ações:**
> 1. (corrigir) trocar `delta()` por `rate()`. ✅
> 2. (detectar) alerta `ApiReiniciandoMuito` baseado em `resets()`. **Dono:** time API.
> 3. (prevenir) pint no CI com `promql/counter`. **Dono:** plataforma.
> 4. (processo) silêncios de alertas `page` com no máximo 4h e link para ticket. **Dono:** SRE.

## 🎓 Na prova PCA

<details>
<summary>Q1. Which function should NOT be used on a counter? (a) rate (b) increase (c) delta (d) resets</summary>

**(c) `delta`.** É para gauges e não compensa resets. As outras três foram feitas para counters.
</details>

<details>
<summary>Q2. Why is <code>rate(sum(http_requests_total)[5m:])</code> wrong?</summary>

Porque a soma não é monotônica: quando **um** dos counters reseta, a soma cai e o `rate` trata isso como reset do total, gerando um pico falso. Faça `sum(rate(http_requests_total[5m]))`.
</details>
