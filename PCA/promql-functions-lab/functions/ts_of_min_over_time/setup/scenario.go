package main

// Cenário da lição ts_of_min_over_time() (experimental).
//
// ts_of_min_over_time_node_power_supply_capacity{power_supply} -> gauge (nível de bateria em %,
// imita node_power_supply_capacity do node_exporter)
//   power_supply="BAT0" (notebook) : ciclo de 5 min: descarrega 100%->0% em 180s, fica em 0% (desligado)
//                     por 60s e recarrega 0%->100% em 60s. O mínimo (0) se repete por 60s:
//                     ts_of_min_over_time devolve o timestamp da ÚLTIMA amostra em 0.
//   power_supply="UPS"  (nobreak)  : senóide 30%..90% com período 4 min (um único vale por ciclo).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "ts_of_min_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"ts_of_min_over_time_node_power_supply_capacity",
				"Nível de bateria em % (gauge; imita node_power_supply_capacity).",
				prometheus.GaugeValue, []string{"power_supply"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					c := math.Mod(t, 300)
					var phone float64
					switch {
					case c < 180:
						phone = 100 - 100*c/180
					case c < 240:
						phone = 0
					default:
						phone = 100 * (c - 240) / 60
					}
					tablet := math.Round(Wave(t, 240, 60, 30, 0)*10) / 10
					return []Sample{
						{Labels: []string{"BAT0"}, Value: math.Round(phone*10) / 10},
						{Labels: []string{"UPS"}, Value: tablet},
					}
				},
			))
		},
	})
}
