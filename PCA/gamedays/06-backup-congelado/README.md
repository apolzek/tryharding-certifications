# 06 · O backup congelado

> **Em uma frase:** o banco precisou de restore e o backup mais recente tinha **dias**. O cron do backup morreu, mas a Pushgateway continuou servindo o último "sucesso" para sempre, e o alerta olhava só o status.

| | |
|---|---|
| **Dificuldade** | ⭐⭐ |
| **Tempo-alvo** | 20 min |
| **Tópicos PCA** | Pushgateway, `push_time_seconds`, batch jobs, `honor_labels`, `exported_job`, alertar pela idade do dado |
| **Arquivos que você vai editar** | `work/prometheus/prometheus.yml` e `work/prometheus/rules.yml` |

> ⏱️ **Tempo comprimido:** neste gameday, **"1 dia" = 30 segundos**. O backup deveria rodar a cada 30s; se passar disso sem rodar, é incidente.

---

## 📟 O chamado

```
┌──────────────────────────────────────────────────────────────────────────┐
│ INCIDENTE SEV1 · INC-2231 · "restore do db01 com dados de 6 dias atrás"  │
├──────────────────────────────────────────────────────────────────────────┤
│ Durante o restore do db01 descobrimos que o último backup é de 6 dias    │
│ atrás. O painel "Backups" mostra backup_last_status = 1 (sucesso) e o    │
│ alerta BackupNaoRodou nunca disparou.                                    │
│                                                                          │
│ O cron do backup foi removido por engano numa migração de servidor.      │
│ Esse não é o problema do monitoramento: o problema é que NINGUÉM SOUBE.  │
│                                                                          │
│ Pedido: BackupNaoRodou{job="backup", instance="db01"} deve disparar      │
│ quando o backup não rodar dentro do SLA (1 "dia" = 30s).                 │
└──────────────────────────────────────────────────────────────────────────┘
```

## ▶️ Como rodar

```bash
./start.sh 06          # sobe também a Pushgateway (:9183) e faz UM push do backup
# edite work/prometheus/*.yml  ->  ./reload.sh  ->  ./check.sh 06
```

## 🩺 Sintomas

- http://localhost:9183 (UI da Pushgateway): o grupo `job="backup", instance="db01"` com "last pushed" ficando cada vez mais velho.
- No Prometheus, as métricas do backup estão lá, com valor "de sucesso", todo scrape.

---

## 🔍 Investigação guiada

<details>
<summary><b>Passo 1:</b> a regra consegue ao menos achar as séries?</summary>

```promql
backup_last_status{job="backup"}
```

**Resultado esperado:** vazio! Agora sem o filtro de `job`:

```promql
backup_last_status
```

**Resultado esperado:**

```
backup_last_status{exported_instance="db01", exported_job="backup", instance="pushgateway:9091", job="pushgateway"} 1
```

O `job` e o `instance` que o backup enviou foram **renomeados** para `exported_job`/`exported_instance`, porque conflitavam com os labels que o Prometheus coloca no target (`job="pushgateway"`). É o comportamento padrão quando `honor_labels: false`.
</details>

<details>
<summary><b>Passo 2:</b> mesmo com o label certo, o status serviria?</summary>

```promql
backup_last_status
```

Vale **1** e vai valer 1 para sempre. A Pushgateway **não expira** métricas: ela serve o último valor recebido até alguém apagar o grupo. Um job que morreu não empurra "0"; ele simplesmente **para de empurrar**. Alertar em `status == 0` só pega o job que roda **e** falha; nunca o job que não roda.
</details>

<details>
<summary><b>Passo 3:</b> como saber a IDADE do dado?</summary>

A Pushgateway acrescenta a cada grupo:

```bash
curl -s localhost:9183/metrics | grep -E '^push_(time|failure_time)_seconds'
```

```promql
time() - push_time_seconds
```

**Resultado esperado:** um número de segundos crescendo sem parar. Esse é o sinal certo: "há quanto tempo o job não fala comigo". A métrica de negócio que o job empurra também serve:

```promql
time() - backup_last_success_timestamp_seconds
```
</details>

---

## 🎯 Causa raiz

<details>
<summary>Spoiler</summary>

1. **Alerta pelo status, não pela idade.** Métricas na Pushgateway são "congeladas": o último push fica exposto indefinidamente. `backup_last_status == 0` nunca dispara para um job que simplesmente parou de rodar.
2. **Faltou `honor_labels: true` no scrape da Pushgateway.** Os labels `job`/`instance` do push viraram `exported_job`/`exported_instance`, então até o seletor `{job="backup"}` voltava vazio. Mesmo que o job empurrasse `status=0`, o alerta não teria visto.
</details>

## 🔧 Correção

<details>
<summary>Spoiler</summary>

`work/prometheus/prometheus.yml`:

```yaml
  - job_name: pushgateway
    honor_labels: true
    static_configs:
      - targets: ['pushgateway:9091']
```

`work/prometheus/rules.yml`:

```yaml
      - alert: BackupNaoRodou
        expr: time() - push_time_seconds{job="backup"} > 30
        labels:
          severity: page
        annotations:
          summary: "backup de {{ $labels.instance }} não roda há {{ $value | humanizeDuration }}"

      - alert: BackupFalhou          # o caso "rodou e falhou" continua coberto
        expr: backup_last_status{job="backup"} == 0
        labels:
          severity: page
```

```bash
./reload.sh && ./check.sh 06
```
</details>

## 🛡️ Como evitar

- **Batch job = alerte pela idade.** Em produção (backup diário):

```yaml
- alert: BackupNaoRodou
  expr: time() - backup_last_success_timestamp_seconds{job="backup"} > 26 * 3600
  labels: {severity: page}
- alert: BackupSumiu          # nem o grupo existe (pushgateway reiniciou sem persistência?)
  expr: absent(backup_last_success_timestamp_seconds{job="backup"})
  for: 1h
```

  Prefira `*_last_success_timestamp_seconds` a `push_time_seconds`: um job que roda e **falha** também empurra (e atualiza `push_time_seconds`), mas não atualiza o timestamp de sucesso.
- **Sempre `honor_labels: true` no job da Pushgateway** (e no de federação): quem sabe o `job`/`instance` real é quem empurrou.
- **Pushgateway só para batch jobs de nível de serviço.** Não use para "converter push em pull" de serviços de longa duração; você perde o `up` e herda métricas zumbis.
- **Persistência:** `--persistence.file` na Pushgateway, senão um restart apaga os grupos (e o `absent()` acima te avisa).
- **Grupos órfãos:** quando um job é descomissionado, apague o grupo (`curl -X DELETE localhost:9183/metrics/job/backup/instance/db01`), senão ele fica "congelado" para sempre.

## 📝 Postmortem (exemplo)

> **Resumo:** o backup do db01 não rodou entre 2026-09-14 e 2026-09-20. Descobrimos durante um restore (SEV1 INC-2231), que recuperou dados com 6 dias de defasagem; 3h de trabalho de reconciliação manual.
>
> **Gatilho:** remoção acidental do cron na migração do servidor de jobs.
>
> **Causa raiz (detecção):** o alerta monitorava `backup_last_status == 0`, que não muda quando o job deixa de rodar (a Pushgateway mantém o último valor). Além disso, o scrape da Pushgateway não tinha `honor_labels: true`, e o seletor `{job="backup"}` não encontrava nada.
>
> **Ações:**
> 1. (corrigir) `honor_labels: true` e alerta por idade (`time() - ..._last_success_timestamp_seconds`). ✅
> 2. (detectar) `absent()` para os grupos de todos os batch jobs críticos. **Dono:** SRE.
> 3. (prevenir) teste de restore mensal automatizado. **Dono:** DBA.
> 4. (prevenir) crons gerenciados por código (ex.: CronJob no k8s), sem edição manual. **Dono:** plataforma.

## 🎓 Na prova PCA

<details>
<summary>Q1. A batch job pushed metrics to the Pushgateway once and then stopped running. What does Prometheus scrape from the Pushgateway afterwards?</summary>

O **último valor empurrado**, indefinidamente. A Pushgateway não expira métricas; por isso se alerta em `time() - push_time_seconds` (ou num timestamp de sucesso).
</details>

<details>
<summary>Q2. Without <code>honor_labels: true</code>, a pushed metric with <code>job="backup"</code> scraped by job <code>pushgateway</code> ends up with which labels?</summary>

`job="pushgateway"` e `exported_job="backup"`. Em conflito, o label do target vence e o original ganha o prefixo `exported_`.
</details>
