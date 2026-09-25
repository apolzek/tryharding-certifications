package main

// Cenário da lição day_of_year().
//
// 1) day_of_year_simulated_clock_timestamp_seconds -> relógio acelerado:
//    2 dias simulados por segundo real, percorrendo 01/jan/2027 → 31/dez/2028
//    (~6 min por ciclo). day_of_year() sobe até 365 (2027) e depois até 366 (2028, bissexto).
//
// 2) day_of_year_annual_budget_dollars{team}       -> orçamento ANUAL de cloud (gauge fixo)
//    day_of_year_budget_spent_year_to_date_dollars{team} -> gasto acumulado no ano (UTC):
//      platform gasta 20% ACIMA do ritmo (vai estourar)
//      data     gasta 10% ABAIXO do ritmo

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "day_of_year",
		Setup: func(reg prometheus.Registerer) {
			base := time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC).Unix()
			reg.MustRegister(NewFunc(
				"day_of_year_simulated_clock_timestamp_seconds",
				"Relógio acelerado: 2 dias simulados por segundo, 2027-2028.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					k := now.Unix() % 366
					return []Sample{{Value: float64(base + k*2*86400)}}
				},
			))
			budgets := map[string]float64{"platform": 120000, "data": 60000}
			pace := map[string]float64{"platform": 1.2, "data": 0.9}
			reg.MustRegister(NewFunc(
				"day_of_year_annual_budget_dollars",
				"Orçamento anual de cloud do time (USD).",
				prometheus.GaugeValue, []string{"team"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"platform"}, Value: budgets["platform"]}, {Labels: []string{"data"}, Value: budgets["data"]}}
				},
			))
			reg.MustRegister(NewFunc(
				"day_of_year_budget_spent_year_to_date_dollars",
				"Gasto de cloud acumulado desde 1º de janeiro (UTC), em USD.",
				prometheus.GaugeValue, []string{"team"},
				func(now time.Time) []Sample {
					u := now.UTC()
					jan1 := time.Date(u.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
					days := u.Sub(jan1).Hours() / 24
					var out []Sample
					for _, t := range []string{"platform", "data"} {
						out = append(out, Sample{Labels: []string{t}, Value: budgets[t] / 365 * pace[t] * days})
					}
					return out
				},
			))
		},
	})
}
