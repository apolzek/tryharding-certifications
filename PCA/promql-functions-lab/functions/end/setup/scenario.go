package main

// Cenário da lição end().
//
// end() devolve o FIM da janela de uma consulta range (a borda direita do gráfico,
// normalmente "agora").
//
// end_temperature_celsius{room} -> temperatura de duas salas, ondas lentas:
//   datacenter  24 ± 4 °C (período 7 min)
//   escritorio  21 ± 2 °C (período 5 min)

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "end",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"end_temperature_celsius",
				"Temperatura da sala em graus Celsius.",
				prometheus.GaugeValue, []string{"room"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"datacenter"}, Value: Wave(t, 420, 24, 4, 0) + Noise(0.2)},
						{Labels: []string{"escritorio"}, Value: Wave(t, 300, 21, 2, 1) + Noise(0.2)},
					}
				},
			))
		},
	})
}
