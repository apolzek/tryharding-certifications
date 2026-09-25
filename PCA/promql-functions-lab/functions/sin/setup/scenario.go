package main

// Cenário da lição sin().
//
// 1) sin_turbine_rotor_angle_radians{turbine}: ângulo da pá "A" de uma turbina
//    eólica, em RADIANOS. Dá uma volta completa (0 -> 2π) a cada 2 min e volta a 0
//    (dente-de-serra). sin(ângulo) = altura relativa da ponta da pá (-1..1).
//    turbine="T1" gira; turbine="T2" gira com 90° de atraso (fase).
//
// 2) sin_http_requests_per_second{service}: tráfego REAL de uma loja, que segue um
//    ciclo "diário" comprimido em 5 min: 100 + 50*sin(2π t/300) + ruído.
//    A cada 10 min há uma "queda" (incidente) de 60s em que o tráfego cai a 20%.
//    O modelo esperado é calculado NA QUERY com sin(2*pi()*time()/300).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sin",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"sin_turbine_rotor_angle_radians",
				"Ângulo da pá A da turbina, em radianos (0..2π, uma volta a cada 120s).",
				prometheus.GaugeValue, []string{"turbine"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					a1 := 2 * math.Pi * Saw(t, 120)
					a2 := 2 * math.Pi * Saw(t+30, 120) // 30s de 120s = 90° adiantada
					return []Sample{
						{Labels: []string{"T1"}, Value: a1},
						{Labels: []string{"T2"}, Value: a2},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"sin_http_requests_per_second",
				"Tráfego real (req/s) com ciclo 'diário' comprimido em 300s e um incidente a cada 10 min.",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					v := 100 + 50*math.Sin(2*math.Pi*t/300) + Noise(4)
					if math.Mod(t, 600) >= 400 && math.Mod(t, 600) < 460 {
						v *= 0.2 // incidente: tráfego despenca
					}
					return []Sample{{Labels: []string{"shop"}, Value: v}}
				},
			))
		},
	})
}
