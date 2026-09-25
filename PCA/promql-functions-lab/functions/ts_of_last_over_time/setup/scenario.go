package main

// Cenário da lição ts_of_last_over_time() (experimental).
//
// ts_of_last_over_time_sensor_temperature_celsius{sensor} -> gauge
//   sensor="healthy" : reporta sempre (~22°C).
//   sensor="flaky"   : reporta por 90s e SOME por 90s (ciclo de 3 min) — simula um
//                      sensor com Wi-Fi ruim. Enquanto some, a série fica "stale".
//   sensor="retired" : reporta só 60s a cada 15 min (equipamento quase desativado). Passa mais
//                      tempo sem dado do que a janela [10m] -> some do resultado de ts_of_last_over_time.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "ts_of_last_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"ts_of_last_over_time_sensor_temperature_celsius",
				"Temperatura reportada pelo sensor (gauge, °C).",
				prometheus.GaugeValue, []string{"sensor"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					out := []Sample{{Labels: []string{"healthy"}, Value: Wave(t, 300, 22, 1, 0) + Noise(0.2)}}
					if math.Mod(t, 180) < 90 {
						out = append(out, Sample{Labels: []string{"flaky"}, Value: Wave(t, 300, 25, 1, 1) + Noise(0.2)})
					}
					if math.Mod(t, 900) < 60 {
						out = append(out, Sample{Labels: []string{"retired"}, Value: 18 + Noise(0.2)})
					}
					return out
				},
			))
		},
	})
}
