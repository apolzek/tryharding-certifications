package main

// Cenário da lição range().
//
// range() devolve a DURAÇÃO da janela de uma consulta range (end() - start()),
// o mesmo que $__range no Grafana.
//
// 1) range_http_requests_total -> counter, ~10 req/s, com pico de ~30 req/s durante
//    60s a cada 7 min (baseado no relógio de parede).
// 2) range_cpu_usage_ratio     -> gauge 0-1, onda 0.5 ± 0.3 (período 5 min).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "range",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"range_http_requests_total",
				"Total de requisições HTTP (~10/s, pico de ~30/s por 60s a cada 7 min).",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					t := float64(now.Unix()) - 1.79e9
					cycles := math.Floor(t / 420)
					inCycle := math.Min(math.Mod(t, 420), 60)
					// 10/s sempre + 20/s extra nos primeiros 60s de cada ciclo de 7 min
					return []Sample{{Value: 10*t + 20*(cycles*60+inCycle)}}
				},
			))
			reg.MustRegister(NewFunc(
				"range_cpu_usage_ratio",
				"Uso de CPU (0-1).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Max(0, Wave(float64(now.Unix()), 300, 0.5, 0.3, 0)+Noise(0.03))}}
				},
			))
		},
	})
}
