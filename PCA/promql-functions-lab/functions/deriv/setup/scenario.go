package main

// Cenário da lição deriv().
//
// Valores calculados a partir do relógio de parede.
//
// 1) deriv_container_memory_working_set_bytes{pod} -> GAUGE de memória:
//    pod="leaky"    vazamento: cresce 1 MiB/s, de 200 MiB até 800 MiB,
//                   quando leva OOM kill (a cada 10 min).
//    pod="healthy"  estável em ~300 MiB.
//    Ruído nos dois: ±3 MiB sempre e, em ~10% dos scrapes, um PICO de +150 MiB
//    (alocação temporária) -> atrapalha muito o delta(), pouco o deriv().
//    deriv(...[2m]) -> leaky ≈ 1 MiB/s (1.048.576 B/s), healthy ≈ 0.
//
// 2) deriv_node_hwmon_temp_celsius{host="rack-01"} -> senóide de 5 min
//    entre 18 e 28°C. deriv() é positiva quando esquenta, negativa quando
//    esfria e ZERO nos picos/vales (inclinação máx. = 2π·5/300 ≈ 0.105 °C/s ≈ 6.3 °C/min).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "deriv",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }
			const MiB = 1024 * 1024

			reg.MustRegister(NewFunc(
				"deriv_container_memory_working_set_bytes",
				"Memória usada por pod (gauge). 'leaky' vaza 1 MiB/s e leva OOM a cada 10 min.",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := secs(now)
					noise := func() float64 {
						n := Noise(3 * MiB)
						if Noise(1) > 0.8 { // ~10% das vezes
							n += 150 * MiB
						}
						return n
					}
					return []Sample{
						{Labels: []string{"leaky"}, Value: math.Round(200*MiB + MiB*math.Mod(t, 600) + noise())},
						{Labels: []string{"healthy"}, Value: math.Round(300*MiB + noise())},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"deriv_node_hwmon_temp_celsius",
				"Temperatura do sensor do rack (gauge, imita node_hwmon_temp_celsius): onda de 5 min entre 18 e 28°C.",
				prometheus.GaugeValue, []string{"host"},
				func(now time.Time) []Sample {
					v := Wave(secs(now), 300, 23, 5, 0) + Noise(0.1)
					return []Sample{{Labels: []string{"rack-01"}, Value: math.Round(v*100) / 100}}
				},
			))
		},
	})
}
