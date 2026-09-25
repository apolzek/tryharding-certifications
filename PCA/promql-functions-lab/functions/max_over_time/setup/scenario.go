package main

// Cenário da lição max_over_time().
//
// 1) max_over_time_container_memory_working_set_bytes{pod} -> gauge de memória
//    api-1: ~400 MiB, mas a cada 3 min (segundos 20..35 do ciclo) dá um PICO de ~950 MiB
//           por só 15s (3 scrapes). O pico NUNCA cai numa virada de minuto, então um
//           gráfico com resolução de 1 min não o enxerga.
//    api-2: ~550 MiB estável, sem picos.
//
// 2) max_over_time_http_requests_total -> counter: 5 req/s, com rajada de 50 req/s
//    durante 90s a cada 4 min. Serve para o caso com SUBQUERY:
//      max_over_time(rate(x[1m])[10m:]) -> maior req/s dos últimos 10 min.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "max_over_time",
		Setup: func(reg prometheus.Registerer) {
			const mib = 1024 * 1024
			reg.MustRegister(NewFunc(
				"max_over_time_container_memory_working_set_bytes",
				"Memória usada pelo pod (gauge). api-1 tem picos curtos a cada 3 min.",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					api1 := Wave(t, 300, 400, 30, 0) + Noise(5)
					if r := now.Unix() % 180; r >= 20 && r < 35 {
						api1 = 950 // sempre o mesmo valor: ts_of_max_over_time aponta para o pico mais recente
					}
					api2 := 550 + Noise(5)
					return []Sample{
						{Labels: []string{"api-1"}, Value: math.Round(api1 * mib)},
						{Labels: []string{"api-2"}, Value: math.Round(api2 * mib)},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"max_over_time_http_requests_total",
				"Requisições HTTP (counter). 5 req/s, rajada de 50 req/s por 90s a cada 4 min.",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					// integral determinística desde a meia-noite UTC (86400 = 360 ciclos de 240s)
					day := now.Unix() % 86400
					cycles := float64(day / 240)
					r := float64(day % 240)
					v := cycles*(240*5+90*45) + 5*r + 45*math.Min(90, math.Max(0, r-100))
					return []Sample{{Value: v}}
				},
			))
		},
	})
}
