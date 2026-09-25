package main

// Cenário da lição acosh().
//
// 1) acosh_traffic_peak_ratio{service}: tráfego atual / tráfego de referência
//    (baseline). Um "multiplicador de pico".
//    service="checkout": entre 1x e 50x (Black Friday relâmpago a cada 4 min) —
//        sempre >= 1, domínio feliz do acosh.
//    service="batch":     entre 0.5x e 3x (período 3 min). Quando cai abaixo de 1
//        (madrugada) -> acosh() = NaN.
// 2) acosh_powerline_sag_ratio{line}: razão T_torre / T_meio de um cabo pendurado
//    (catenária). Fisicamente é cosh(vão / 2a) >= 1. acosh() recupera vão/2a.
//    acosh_powerline_span_meters{line} = 100 m. Então a = vão / (2*acosh(razão)).
//    "a" real varia entre 150 e 400 m (período 5 min).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "acosh",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"acosh_traffic_peak_ratio", "Tráfego atual dividido pelo baseline (1 = normal).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					p := math.Mod(t, 240)
					co := 1.2 + Noise(0.1)
					if p < 80 {
						co = 1 + 49*math.Pow(math.Sin(math.Pi*p/80), 2)
					}
					return []Sample{
						{Labels: []string{"checkout"}, Value: co},
						{Labels: []string{"batch"}, Value: Wave(t, 180, 1.75, 1.25, 0)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"acosh_powerline_sag_ratio", "Tração na torre / tração no ponto mais baixo (= cosh(vão/2a)).",
				prometheus.GaugeValue, []string{"line"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					a := Wave(t, 300, 275, 125, 0)
					return []Sample{{Labels: []string{"LT-138kV"}, Value: math.Cosh(100 / (2 * a))}}
				},
			))
			reg.MustRegister(NewFunc(
				"acosh_powerline_span_meters", "Vão entre as torres (m).",
				prometheus.GaugeValue, []string{"line"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"LT-138kV"}, Value: 100}}
				},
			))
		},
	})
}
