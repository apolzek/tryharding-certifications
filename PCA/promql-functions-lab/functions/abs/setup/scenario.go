package main

// Cenário da lição abs().
//
// 1) abs_node_timex_offset_seconds{node} -> imita node_timex_offset_seconds (node_exporter): desvio do relógio do nó em relação ao NTP.
//    Pode ser POSITIVO (adiantado) ou NEGATIVO (atrasado).
//      node-a: oscila entre -0.08s e +0.08s (período 4 min)
//      node-b: oscila entre -0.03s e +0.03s (sempre dentro da tolerância)
//      node-c: constante em -0.12s (sempre ATRASADO: o alerta ingênuo "> 0.05" nunca pega)
//
// 2) abs_temperature_celsius{room} e abs_temperature_setpoint_celsius{room}
//    -> o ar-condicionado mira 22°C, mas a temperatura oscila 22 ± 3°C (período 3 min).
//       O erro com sinal (temp - setpoint) tem MÉDIA ~0, mas o erro absoluto médio é ~1.9°C.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "abs",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"abs_node_timex_offset_seconds",
				"Offset do relógio do nó em relação ao NTP (imita node_timex_offset_seconds do node_exporter), em segundos (positivo = adiantado, negativo = atrasado).",
				prometheus.GaugeValue, []string{"node"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"node-a"}, Value: Wave(t, 240, 0, 0.08, 0)},
						{Labels: []string{"node-b"}, Value: Wave(t, 240, 0, 0.03, math.Pi)},
						{Labels: []string{"node-c"}, Value: -0.12 + Noise(0.002)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"abs_temperature_celsius",
				"Temperatura medida na sala, em °C.",
				prometheus.GaugeValue, []string{"room"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"datacenter"}, Value: Wave(t, 180, 22, 3, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"abs_temperature_setpoint_celsius",
				"Temperatura alvo (setpoint) do ar-condicionado, em °C.",
				prometheus.GaugeValue, []string{"room"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"datacenter"}, Value: 22}}
				},
			))
		},
	})
}
