package main

// Cenário da lição atan().
//
// 1) atan_drone_altitude_meters{drone}: altitude de um drone de inspeção.
//    Ciclo de 4 min: sobe 60s a 3 m/s, cruza 60s, desce 60s a 3 m/s, pousa 60s.
//    atan_drone_ground_speed_mps{drone}: velocidade horizontal constante 5 m/s.
//    Ângulo de subida = deg(atan(deriv(altitude[30s]) / velocidade)) ≈ ±31°.
// 2) atan_wind_direction_degrees{station}: direção do vento (0..360°), oscilando
//    em torno do NORTE (345° .. 15°), cruzando 0/360 o tempo todo.
//    Serve para mostrar por que média de ângulos precisa de sin/cos + atan2.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "atan",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"atan_drone_altitude_meters", "Altitude do drone (m).",
				prometheus.GaugeValue, []string{"drone"},
				func(now time.Time) []Sample {
					p := math.Mod(float64(now.UnixNano())/1e9, 240)
					var h float64
					switch {
					case p < 60:
						h = 3 * p
					case p < 120:
						h = 180
					case p < 180:
						h = 180 - 3*(p-120)
					default:
						h = 0
					}
					return []Sample{{Labels: []string{"inspetor-1"}, Value: h}}
				},
			))
			reg.MustRegister(NewFunc(
				"atan_drone_ground_speed_mps", "Velocidade horizontal do drone (m/s).",
				prometheus.GaugeValue, []string{"drone"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"inspetor-1"}, Value: 5}}
				},
			))
			reg.MustRegister(NewFunc(
				"atan_wind_direction_degrees", "Direção de onde vem o vento, em graus (0 = norte).",
				prometheus.GaugeValue, []string{"station"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					d := math.Mod(360+15*math.Sin(2*math.Pi*t/60)+Noise(2), 360)
					return []Sample{{Labels: []string{"aeroporto"}, Value: d}}
				},
			))
		},
	})
}
