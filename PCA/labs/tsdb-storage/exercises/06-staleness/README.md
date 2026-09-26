# 06 · Staleness: stale markers vs lookback

**Objetivo:** ver com os próprios olhos **quando** uma série some do resultado das queries.

O gerador tem 3 botões:

| `/control?...` | efeito |
|---|---|
| `ephemeral=0` | a série `tsdb_demo_ephemeral` some do `/metrics` (o alvo continua **UP**) |
| `timestamped=0` | a série `tsdb_demo_timestamped` some; ela era exposta **com timestamp explícito** |
| `down=1` | o `/metrics` inteiro responde 503 (alvo **DOWN**) |

## 🎯 Tarefa

1. `curl -s 'localhost:9151/control?ephemeral=0&timestamped=0'` e consulte as duas séries a cada 5s. Qual some na hora? Qual demora? Quanto?
2. Ache o **stale marker** no disco com `promtool tsdb dump --match=tsdb_demo_ephemeral`.
3. `curl -s 'localhost:9151/control?down=1'`: o que acontece com `up` e com as séries `tsdb_demo_*`?
4. Volte: `curl -s 'localhost:9151/control?ephemeral=1&timestamped=1&down=0'`.

<details><summary>✅ Solução</summary>

- `tsdb_demo_ephemeral` some **no próximo scrape** (≤ 5s): o Prometheus percebeu que a série estava no scrape anterior e não está no atual e gravou um **stale marker** (um NaN especial). No `promtool tsdb dump` aparece como `NaN <timestamp>`.
- `tsdb_demo_timestamped` continua aparecendo por **5 minutos** (o lookback delta): séries com timestamp explícito **não recebem stale marker**, porque o Prometheus não sabe se o exportador simplesmente não tem dado novo. O mesmo vale para dados de backfill e, em geral, para quem manda timestamp próprio.
- Alvo DOWN: `up` vira `0` e **todas** as séries daquele alvo ganham stale marker de uma vez (somem na hora). Por isso `absent(up{job="x"})` é diferente de `up{job="x"} == 0`.

Script: [`solutions/06-staleness.sh`](../../solutions/06-staleness.sh).
</details>
