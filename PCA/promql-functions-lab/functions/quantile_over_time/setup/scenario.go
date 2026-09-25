package main

// Cenário da lição quantile_over_time().
//
// quantile_over_time_probe_duration_seconds{target} -> gauge que imita o probe_duration_seconds
// do blackbox_exporter (quanto demorou o último probe HTTP no endpoint).
//   target="checkout" : ~0.20s (±0.05) quase sempre, mas 1 em cada 30 probes (um "slot" de 5s
//                       a cada 150s) leva 3s. ~3% das amostras são picos.
//                       -> max_over_time vê 3s; p95 ignora (~0.25); p99 pega (3s).
//   target="catalog"  : bimodal (cache). 4 em 5 probes são cache HIT ~0.10s, 1 em 5 é MISS ~1.0s.
//                       -> avg ≈ 0.28 (um valor que nunca acontece!), p50 ≈ 0.10, p95 ≈ 1.0.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "quantile_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"quantile_over_time_probe_duration_seconds",
				"Duração do último probe HTTP em segundos (imita blackbox_exporter probe_duration_seconds).",
				prometheus.GaugeValue, []string{"target"},
				func(now time.Time) []Sample {
					slot := now.Unix() / 5 // muda a cada scrape (5s)
					checkout := 0.20 + Noise(0.05)
					if slot%30 == 0 {
						checkout = 3.0
					}
					catalog := 0.10 + Noise(0.01)
					if slot%5 == 0 {
						catalog = 1.0 + Noise(0.05)
					}
					return []Sample{
						{Labels: []string{"checkout"}, Value: math.Max(0, checkout)},
						{Labels: []string{"catalog"}, Value: math.Max(0, catalog)},
					}
				},
			))
		},
	})
}
