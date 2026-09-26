# 03 · Filtro vs `bool` (e o NaN)

**Contexto:** `sort_cronjob_last_duration_seconds{cronjob}` tem a duração da última execução de 4 CronJobs:
backup 120, cleanup 8, report 45 e **sync = NaN** (o job nunca terminou).

```promql
# a) 1 ou 0 para CADA cronjob: passou de 30s?              -> desafio 219
sort_cronjob_last_duration_seconds > ___ 30

# b) só as séries com valor numérico (sem o NaN), só com operadores   -> desafio 220
sort_cronjob_last_duration_seconds ___ sort_cronjob_last_duration_seconds

# c) QUANTOS cronjobs passaram de 30s (1 série)             -> desafio 221
___(sort_cronjob_last_duration_seconds > 30)
```

```bash
cd ../../../../challenges && ./check.py 219 'sua query'   # idem 220, 221
```

💡 **Dica:** NaN é o único valor diferente de si mesmo. E `count` conta **séries**, não valores.

<details><summary>Solução</summary>

```promql
sort_cronjob_last_duration_seconds > bool 30
sort_cronjob_last_duration_seconds == sort_cronjob_last_duration_seconds
count(sort_cronjob_last_duration_seconds > 30)
```

- a) `backup 1, cleanup 0, report 1, sync 0` — o NaN vira 0 e o nome da métrica some.
- b) `NaN == NaN` é falso → o `sync` sai. `x != NaN` **não** funciona (é verdadeiro para todos).
- c) **2**. Cuidado: `count(x > bool 30)` = 4 (conta os zeros). `sum(x > bool 30)` = 2 também.
</details>
