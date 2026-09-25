package main

// Cenário da lição hour().
//
// 1) hour_simulated_clock_timestamp_seconds -> relógio acelerado:
//    1 hora simulada a cada 10s reais (um dia inteiro em 4 min), segunda 04/jan/2027.
//    hour() dele vira um dente-de-serra 0..23.
//
// 2) hour_checkout_errors_ratio -> taxa de erro do checkout (~8%, acima do SLO de 5%).
//    Usada para "só alertar em horário comercial".
//
// 3) hour_kube_cronjob_status_last_successful_time -> backup noturno que roda
//    todo dia às 03:00 UTC (= 00:00 em Brasília). Valor = timestamp da última execução.

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "hour",
		Setup: func(reg prometheus.Registerer) {
			base := time.Date(2027, 1, 4, 0, 0, 0, 0, time.UTC).Unix()
			reg.MustRegister(NewFunc(
				"hour_simulated_clock_timestamp_seconds",
				"Relógio acelerado: 1 hora simulada a cada 10s.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					k := now.Unix() % 240
					return []Sample{{Value: float64(base + k*360)}}
				},
			))
			reg.MustRegister(NewFunc(
				"hour_checkout_errors_ratio",
				"Fração de requisições do checkout com erro (0-1).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample { return []Sample{{Value: 0.08 + Noise(0.01)}} },
			))
			reg.MustRegister(NewFunc(
				"hour_kube_cronjob_status_last_successful_time",
				"Imita kube-state-metrics: último sucesso do CronJob (backup noturno, todo dia 03:00 UTC).",
				prometheus.GaugeValue, []string{"cronjob"},
				func(now time.Time) []Sample {
					u := now.UTC()
					d := time.Date(u.Year(), u.Month(), u.Day(), 3, 0, 0, 0, time.UTC)
					if d.After(u) {
						d = d.AddDate(0, 0, -1)
					}
					return []Sample{{Labels: []string{"nightly-backup"}, Value: float64(d.Unix())}}
				},
			))
		},
	})
}
