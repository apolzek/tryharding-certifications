package main

// Cenário da lição cos().
//
// 1) cos_solar_incidence_angle_degrees{panel}: ângulo (em GRAUS, como todo
//    inclinômetro reporta) entre o sol e a "normal" do painel.
//    panel="fixo": o sol "passa" pelo painel: ângulo vai 80° -> 0° -> 80° a cada 4 min.
//    panel="tracker": painel com rastreador solar, fica sempre perto de 5°.
// 2) cos_solar_panel_rated_watts{panel}: potência nominal (sol a pino) = 400 W.
//    Potência esperada = 400 * cos(rad(ângulo)).
// 3) cos_robot_arm_angle_radians{arm}: braço robótico de 2 m girando em
//    RADIANOS (uma volta a cada 3 min). x = 2*cos(θ), y = 2*sin(θ).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "cos",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"cos_solar_incidence_angle_degrees",
				"Ângulo de incidência do sol no painel, em graus (0 = sol perpendicular ao painel).",
				prometheus.GaugeValue, []string{"panel"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					// 80 -> 0 -> 80 em 240s (|triângulo|)
					fixed := 80 * math.Abs(2*Saw(t, 240)-1)
					return []Sample{
						{Labels: []string{"fixo"}, Value: fixed},
						{Labels: []string{"tracker"}, Value: 5 + Noise(1)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"cos_solar_panel_rated_watts",
				"Potência nominal do painel (sol perpendicular).",
				prometheus.GaugeValue, []string{"panel"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"fixo"}, Value: 400}, {Labels: []string{"tracker"}, Value: 400}}
				},
			))
			reg.MustRegister(NewFunc(
				"cos_robot_arm_angle_radians",
				"Ângulo do braço robótico (2 m) em radianos, uma volta a cada 180s.",
				prometheus.GaugeValue, []string{"arm"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"r1"}, Value: 2 * math.Pi * Saw(t, 180)}}
				},
			))
		},
	})
}
