package main

// Cenário da lição minute().
//
// 1) minute_kube_cronjob_status_last_schedule_time{cronjob} -> última execução de jobs "cron":
//    cleanup  roda a cada 5 min   (*/5 * * * *)      -> minuto 0, 5, 10, ...
//    report   roda a cada 15 min começando no minuto 2 (2-59/15 * * * *) -> 2, 17, 32, 47
//
// 2) minute_api_errors_ratio -> taxa de erro da API. Há um deploy automático nos
//    minutos 0, 1 e 2 de cada dezena (x0..x2): durante ele o erro sobe para ~20%.
//    Fora disso fica em ~1%. Serve para mostrar como silenciar uma "janela de manutenção".

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "minute",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"minute_kube_cronjob_status_last_schedule_time",
				"Timestamp Unix (s) da última execução do job agendado.",
				prometheus.GaugeValue, []string{"cronjob"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"cleanup"}, Value: math.Floor(t/300) * 300},
						{Labels: []string{"report"}, Value: math.Floor((t-120)/900)*900 + 120},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"minute_api_errors_ratio",
				"Fração de requisições da API com erro (0-1). Sobe durante o deploy (minutos x0-x2).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					if now.UTC().Minute()%10 < 3 {
						return []Sample{{Value: 0.20 + Noise(0.03)}}
					}
					return []Sample{{Value: 0.01 + Noise(0.005)}}
				},
			))
		},
	})
}
