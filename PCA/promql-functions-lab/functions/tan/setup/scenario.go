package main

// Cenário da lição tan().
//
// 1) tan_ramp_inclination_degrees{ramp}: inclinação (graus) de uma rampa de
//    carga ajustável. Sobe de 0° até 45° e volta, a cada 4 min.
//    Rampa -> "grade %" (como placa de estrada): 100 * tan(rad(θ)).  45° = 100%.
// 2) tan_antenna_elevation_degrees{antenna}: elevação de uma antena que rastreia
//    um satélite passando pelo zênite: vai de 30° até 89.9° (quase vertical)
//    fica ~25s travada em 89.9° e volta, a cada 3 min. tan() explode perto de 90° (assíntota).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "tan",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"tan_ramp_inclination_degrees",
				"Inclinação da rampa de carga, em graus (0..45).",
				prometheus.GaugeValue, []string{"ramp"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					tri := 1 - math.Abs(2*Saw(t, 240)-1) // 0 -> 1 -> 0
					return []Sample{{Labels: []string{"doca-1"}, Value: 45 * tri}}
				},
			))
			reg.MustRegister(NewFunc(
				"tan_antenna_elevation_degrees",
				"Elevação da antena rastreadora, em graus (30..89.9).",
				prometheus.GaugeValue, []string{"antenna"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					tri := 1 - math.Abs(2*Saw(t, 180)-1) // 0 -> 1 -> 0
					// sobe linear e "trava" em 89.9° por ~25s no topo (satélite no zênite)
					v := math.Min(89.9, 30+70*tri)
					return []Sample{{Labels: []string{"sat-dish"}, Value: v}}
				},
			))
		},
	})
}
