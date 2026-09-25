package main

// Cenário da lição days_in_month().
//
// 1) days_in_month_simulated_clock_timestamp_seconds -> relógio acelerado:
//    1 mês simulado a cada 12s, percorrendo jan/2027 → dez/2028 (24 meses, ~5 min).
//    days_in_month() dele vira uma escada 31, 28, 31, 30, ... e 29 em fev/2028.
//
// 2) days_in_month_cloud_cost_month_to_date_dollars{team} -> custo acumulado no mês (UTC):
//      platform gasta ~40 USD/dia
//      data     gasta ~25 USD/dia
//    days_in_month_cloud_budget_monthly_dollars{team} -> orçamento mensal:
//      platform 1000 USD (vai estourar em meses de 30/31 dias)
//      data      900 USD (folgado)
//
// 3) days_in_month_node_total_hourly_cost{node,instance_type} -> imita o OpenCost:
//      2 × m5.large (0.096 USD/h) + 1 × c5.2xlarge (0.34 USD/h) = 0.532 USD/h
//      Projeção mensal = soma × 24 × days_in_month()

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "days_in_month",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"days_in_month_simulated_clock_timestamp_seconds",
				"Relógio acelerado: 1 mês simulado a cada 12s (dia 15 de cada mês), 2027-2028.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					idx := int(now.Unix()%288) / 12 // 0..23
					return []Sample{{Value: float64(time.Date(2027, time.Month(1+idx), 15, 12, 0, 0, 0, time.UTC).Unix())}}
				},
			))
			daily := map[string]float64{"platform": 40, "data": 25}
			reg.MustRegister(NewFunc(
				"days_in_month_cloud_cost_month_to_date_dollars",
				"Custo de cloud acumulado desde o dia 1 do mês (UTC), em USD.",
				prometheus.GaugeValue, []string{"team"},
				func(now time.Time) []Sample {
					u := now.UTC()
					first := time.Date(u.Year(), u.Month(), 1, 0, 0, 0, 0, time.UTC)
					days := u.Sub(first).Hours() / 24
					return []Sample{
						{Labels: []string{"platform"}, Value: daily["platform"] * days},
						{Labels: []string{"data"}, Value: daily["data"] * days},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"days_in_month_node_total_hourly_cost",
				"Imita OpenCost node_total_hourly_cost: custo por hora de cada nó (USD/h).",
				prometheus.GaugeValue, []string{"node", "instance_type"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"node-1", "m5.large"}, Value: 0.096},
						{Labels: []string{"node-2", "m5.large"}, Value: 0.096},
						{Labels: []string{"node-3", "c5.2xlarge"}, Value: 0.34},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"days_in_month_cloud_budget_monthly_dollars",
				"Orçamento mensal de cloud do time (USD).",
				prometheus.GaugeValue, []string{"team"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"platform"}, Value: 1000}, {Labels: []string{"data"}, Value: 900}}
				},
			))
		},
	})
}
