# 04 · Provocar um fast burn e ver o alerta nascer

**Objetivo:** injetar falhas no serviço e acompanhar o `SLOErrorBudgetBurnFast` passando por **inactive → pending → firing**, no Prometheus e no Grafana.

## ▶️ Rodar

```bash
cd labs/slo-end-to-end
docker compose up -d --build --wait
# Grafana: http://localhost:3170/d/slo-checkout    Alertas: http://localhost:9170/alerts
sleep 60   # deixe o baseline encher as janelas curtas
curl -s 'localhost:9171/chaos?error_rate=0.05'
```

## 🎯 Tarefa

1. Antes de olhar: com 5% de erro e SLO de 99,9%, qual o **burn rate**? Em quanto tempo o budget de 12h (≈30d) acaba?
2. Acompanhe `curl -s localhost:9170/api/v1/alerts` a cada segundo. Anote quando vira `pending` e quando vira `firing`. Explique os dois tempos.
3. Quais **outros** alertas disparam? Por que, numa stack recém-criada, até o de 72m (≈3d) dispara rápido?
4. `curl -s 'localhost:9171/chaos?reset=1'`: em quanto tempo o fast burn some? E o Medium?
5. Repita com `error_rate=0.01`. Quem dispara agora?

<details><summary>✅ Solução</summary>

```bash
./solutions/04-fast-burn.sh
# t+0s   SLOErrorBudgetBurnFast=inactive  (burn rate 1m ≈ 0.6x)
# t+18s  SLOErrorBudgetBurnFast=pending   (burn rate 1m ≈ 14.7x)
# t+47s  SLOErrorBudgetBurnFast=firing    (burn rate 1m ≈ 39.0x)
```

1. Burn = 0,05 / 0,001 = **50x**. O budget de 12h acaba em 12h/50 ≈ **14 min** (em produção: 30d/50 ≈ 14h).
2. **pending** quando a janela de 1m (≈1h) passa de 14.4x: como ela mistura o tráfego bom de antes com o ruim de agora, leva ~29% da janela (≈ 18s). A janela de 5s já está alta quase na hora. **firing** 30s depois (`for: 30s`).
3. Medium, Slow e VerySlow também disparam: com poucos minutos de Prometheus, as janelas de 6m/24m/72m só **têm** poucos minutos de dados, então se comportam como janelas curtas. Em produção, o Prometheus tem dias de histórico e só o Fast (e depois o Medium) dispararia. Suba o lab, espere uns 15 min e repita para ver a diferença.
4. O Fast some em **segundos** (a janela de 5s ≈ 5m volta ao normal), mesmo com a janela de 1m ainda alta. O Medium, em ~30s (janela curta de 30s ≈ 30m). É a função da janela curta: **resetar rápido**.
5. 1% = **10x**: não passa de 14.4 (Fast fica quieto), mas passa de 6 (Medium dispara).

Script: [`solutions/04-fast-burn.sh`](../../solutions/04-fast-burn.sh) (variáveis `ERROR_RATE`, `LATENCY_MS`, `LATENCY_RATIO`, `ALERT`, `KEEP_CHAOS`).
</details>
