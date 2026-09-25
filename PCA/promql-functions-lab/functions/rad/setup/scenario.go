package main

// Cenário da lição rad().
//
// 1) rad_solar_tracker_tilt_degrees{tracker}: inclinação de um rastreador solar
//    de eixo único, em GRAUS: segue o sol de -60° (manhã) a +60° (tarde) em 4 min
//    e "volta para o leste" de uma vez.
// 2) rad_solar_panel_width_meters{tracker}: largura do painel = 2 m.
//    Sombra projetada no chão (largura horizontal) = 2 * cos(rad(tilt)).
//    Altura da borda levantada = 1 * sin(rad(|tilt|)) (metade do painel).
// 3) rad_turbine_blade_pitch_degrees{turbine}: passo (pitch) das pás de uma
//    turbina eólica: ~2° com vento normal, e 30s de "embandeiramento" a 90°
//    (proteção contra vento forte) a cada 3 min.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "rad",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"rad_solar_tracker_tilt_degrees", "Inclinação do rastreador solar (graus, -60..60).",
				prometheus.GaugeValue, []string{"tracker"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"fileira-7"}, Value: -60 + 120*Saw(t, 240)}}
				},
			))
			reg.MustRegister(NewFunc(
				"rad_solar_panel_width_meters", "Largura do painel (m).",
				prometheus.GaugeValue, []string{"tracker"},
				func(now time.Time) []Sample { return []Sample{{Labels: []string{"fileira-7"}, Value: 2}} },
			))
			reg.MustRegister(NewFunc(
				"rad_turbine_blade_pitch_degrees", "Ângulo de passo das pás (graus).",
				prometheus.GaugeValue, []string{"turbine"},
				func(now time.Time) []Sample {
					v := 2 + Noise(0.5)
					if math.Mod(float64(now.Unix()), 180) < 30 {
						v = 90
					}
					return []Sample{{Labels: []string{"T1"}, Value: v}}
				},
			))
		},
	})
}
