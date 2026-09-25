package main

// Cenário da lição stdvar_over_time().
//
// stdvar_over_time_node_hwmon_in_volts{sensor} -> gauge (imita node_hwmon_in_volts do node_exporter) DETERMINÍSTICO (sem ruído),
// todos com média 20V, para a variância dar números "redondos":
//   sensor="square_big"   : onda quadrada 10V / 30V (período 60s) -> variância 100 V², desvio 10V
//   sensor="square_small" : onda quadrada 18V / 22V (período 60s) -> variância   4 V², desvio  2V
//   sensor="flat"         : constante 20V                          -> variância   0 V², desvio  0V

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "stdvar_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"stdvar_over_time_node_hwmon_in_volts",
				"Tensão medida pelo sensor de hardware (gauge, volts; imita node_hwmon_in_volts).",
				prometheus.GaugeValue, []string{"sensor"},
				func(now time.Time) []Sample {
					sq := Square(float64(now.Unix()), 60) // 1 na 1ª metade do minuto, 0 na 2ª
					return []Sample{
						{Labels: []string{"square_big"}, Value: 10 + 20*sq},
						{Labels: []string{"square_small"}, Value: 18 + 4*sq},
						{Labels: []string{"flat"}, Value: 20},
					}
				},
			))
		},
	})
}
