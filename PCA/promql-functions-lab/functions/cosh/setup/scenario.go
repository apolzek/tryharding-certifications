package main

// Cenário da lição cosh().
//
// 1) Linha de transmissão: o cabo pendurado entre duas torres forma uma CATENÁRIA,
//    y(x) = a*cosh(x/a). A flecha (quanto o meio do cabo cai) é:
//        flecha = a * (cosh(vão / (2a)) - 1)
//    cosh_powerline_catenary_param_meters{line}: "a" varia de 150 m (cabo quente,
//    dilatado, "frouxo") a 400 m (frio, esticado), período 5 min.
//    cosh_powerline_span_meters{line}: vão de 100 m.
// 2) cosh_temperature_deviation_celsius{rack}: desvio da temperatura do rack em
//    relação ao ideal (22 °C). Oscila entre -4 e +4 (período 3 min).
//    cosh() vira uma "penalidade em U": simétrica, nunca menor que 1.

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "cosh",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"cosh_powerline_catenary_param_meters", "Parâmetro a da catenária do cabo (m).",
				prometheus.GaugeValue, []string{"line"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"LT-138kV"}, Value: Wave(t, 300, 275, 125, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"cosh_powerline_span_meters", "Vão entre as torres (m).",
				prometheus.GaugeValue, []string{"line"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"LT-138kV"}, Value: 100}}
				},
			))
			reg.MustRegister(NewFunc(
				"cosh_temperature_deviation_celsius", "Desvio da temperatura do rack em relação a 22 °C.",
				prometheus.GaugeValue, []string{"rack"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"r42"}, Value: Wave(t, 180, 0, 4, 0)}}
				},
			))
		},
	})
}
