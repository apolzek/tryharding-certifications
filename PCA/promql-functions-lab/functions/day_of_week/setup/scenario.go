package main

// Cenário da lição day_of_week().
//
// 1) day_of_week_simulated_clock_timestamp_seconds -> relógio acelerado:
//    1 dia simulado a cada 20s reais, começando no domingo 03/jan/2027.
//    Uma semana inteira passa em 140s: day_of_week() vira uma escada 0,1,2,...,6.
//
// 2) day_of_week_checkout_errors_ratio -> taxa de erro do checkout (~8%, acima do SLO de 5%).
//    Usada para "alertar só em dia útil".
//
// 3) day_of_week_kube_cronjob_status_last_successful_time -> o relatório semanal roda toda
//    SEGUNDA às 06:00 UTC. O valor é o timestamp da última execução.

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "day_of_week",
		Setup: func(reg prometheus.Registerer) {
			base := time.Date(2027, 1, 3, 0, 0, 0, 0, time.UTC).Unix() // domingo
			reg.MustRegister(NewFunc(
				"day_of_week_simulated_clock_timestamp_seconds",
				"Relógio acelerado: 1 dia simulado a cada 20s, começando num domingo.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					k := now.Unix() % 140
					return []Sample{{Value: float64(base + k*4320)}}
				},
			))
			reg.MustRegister(NewFunc(
				"day_of_week_checkout_errors_ratio",
				"Fração de requisições do checkout com erro (0-1).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: 0.08 + Noise(0.01)}}
				},
			))
			reg.MustRegister(NewFunc(
				"day_of_week_kube_cronjob_status_last_successful_time",
				"Imita kube-state-metrics: último sucesso do CronJob weekly-report (toda segunda 06:00 UTC).",
				prometheus.GaugeValue, []string{"cronjob"},
				func(now time.Time) []Sample {
					u := now.UTC()
					d := time.Date(u.Year(), u.Month(), u.Day(), 6, 0, 0, 0, time.UTC)
					for d.Weekday() != time.Monday || d.After(u) {
						d = d.AddDate(0, 0, -1)
					}
					return []Sample{{Labels: []string{"weekly-report"}, Value: float64(d.Unix())}}
				},
			))
		},
	})
}
