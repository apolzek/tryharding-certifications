package main

// Cenário da lição sinh().
//
// 1) sinh_latency_zscore{service}: z-score da latência (quantos desvios-padrão a
//    latência atual está acima/abaixo da média histórica). Oscila entre -3 e +3
//    (período 4 min). sinh() funciona como um "amplificador de pânico":
//    perto de 0 fica ~igual (sinh(0.5)=0.52), mas em ±3 vira ±10.
//    service="checkout" oscila; service="search" fica estável em ~0.3.
// 2) sinh_powerline_catenary_param_meters{line}: parâmetro "a" da catenária de um
//    cabo de energia (a = tração horizontal / peso por metro). Quente -> cabo
//    dilata -> "a" menor. Varia entre 150 e 400 m (período 5 min).
//    sinh_powerline_span_meters{line}: vão entre as torres = 100 m.
//    Comprimento do cabo = 2a * sinh(vão / 2a).

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sinh",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"sinh_latency_zscore", "Z-score da latência (desvios-padrão em relação à média).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{
						{Labels: []string{"checkout"}, Value: Wave(t, 240, 0, 3, 0)},
						{Labels: []string{"search"}, Value: 0.3 + Noise(0.05)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"sinh_powerline_catenary_param_meters", "Parâmetro a da catenária do cabo (m).",
				prometheus.GaugeValue, []string{"line"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"LT-138kV"}, Value: Wave(t, 300, 275, 125, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"sinh_powerline_span_meters", "Vão entre as torres (m).",
				prometheus.GaugeValue, []string{"line"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"LT-138kV"}, Value: 100}}
				},
			))
		},
	})
}
