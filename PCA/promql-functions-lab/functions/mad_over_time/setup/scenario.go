package main

// Cenário da lição mad_over_time() (experimental).
//
// mad_over_time_probe_duration_seconds{target} -> gauge (imita probe_duration_seconds do blackbox_exporter)
//   target="api-go"   : 0.100s ± 0.010 (ruído uniforme). MAD ≈ 5ms, stddev ≈ 5.8ms.
//   target="api-java" : igual, mas 1 em cada 12 probes (1 por minuto) pega uma pausa de GC
//                       "stop-the-world" e vale 1.0s. MAD continua ≈ 6ms (robusto!), stddev ≈ 250ms.

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "mad_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"mad_over_time_probe_duration_seconds",
				"Duração do último probe HTTP em segundos (imita blackbox_exporter probe_duration_seconds).",
				prometheus.GaugeValue, []string{"target"},
				func(now time.Time) []Sample {
					out := 0.100 + Noise(0.010)
					if (now.Unix()/5)%12 == 0 {
						out = 1.0
					}
					return []Sample{
						{Labels: []string{"api-go"}, Value: 0.100 + Noise(0.010)},
						{Labels: []string{"api-java"}, Value: out},
					}
				},
			))
		},
	})
}
