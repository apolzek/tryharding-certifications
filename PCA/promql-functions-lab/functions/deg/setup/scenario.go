package main

// Cenário da lição deg().
//
// 1) deg_antenna_azimuth_radians{antenna}: azimute de uma antena de radar, em
//    RADIANOS (como a maioria das bibliotecas de controle reporta). Dá uma volta
//    completa (0 -> 2π ≈ 6.283) a cada 2 min e recomeça.
// 2) deg_robot_arm_joint_radians{joint}: juntas de um braço robótico:
//    ombro oscila entre -0.785 e +0.785 rad (±45°), cotovelo entre 0 e 1.571 (0..90°),
//    período 3 min.
// 3) deg_motor_shaft_position_radians{motor}: posição ACUMULADA do eixo de um motor
//    (encoder multi-volta). Nunca volta a zero: cresce 2π a cada 60s dentro de
//    um ciclo de 5 min (até ~31.4 rad = 1800°). Mostra que deg() não faz "mod 360".

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "deg",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"deg_antenna_azimuth_radians", "Azimute da antena de radar (rad, 0..2π).",
				prometheus.GaugeValue, []string{"antenna"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"radar-1"}, Value: 2 * math.Pi * Saw(t, 120)}}
				},
			))
			reg.MustRegister(NewFunc(
				"deg_robot_arm_joint_radians", "Ângulo das juntas do braço robótico (rad).",
				prometheus.GaugeValue, []string{"joint"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{
						{Labels: []string{"ombro"}, Value: Wave(t, 180, 0, math.Pi/4, 0)},
						{Labels: []string{"cotovelo"}, Value: Wave(t, 180, math.Pi/4, math.Pi/4, math.Pi/2)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"deg_motor_shaft_position_radians", "Posição acumulada do eixo (encoder multi-volta, rad).",
				prometheus.GaugeValue, []string{"motor"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"esteira"}, Value: 2 * math.Pi * math.Mod(t, 300) / 60}}
				},
			))
		},
	})
}
