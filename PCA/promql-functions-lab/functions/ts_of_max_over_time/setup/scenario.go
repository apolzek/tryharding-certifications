package main

// Cenário da lição ts_of_max_over_time() (experimental).
//
// ts_of_max_over_time_node_cpu_utilisation_percent{host} -> gauge (uso de CPU em %, imita instance:node_cpu_utilisation:rate5m × 100)
//   host="web-1" : ~30% (±5) e um PICO de exatamente 95% por 5s (1 amostra) a cada 4 min.
//   host="web-2" : ~40% (±5) e um PLATÔ de exatamente 80% por 60s a cada 4 min.
//                  Empate no máximo -> ts_of_max_over_time devolve a ÚLTIMA amostra do platô.
//
// ts_of_max_over_time_http_requests_total{host="web-1"} -> COUNTER: 10 req/s, com rajada de 50 req/s
//   por 20s a cada 4 min (junto com a agulha de CPU). Mostra o erro de aplicar ts_of_max_over_time
//   direto num counter (o máximo é sempre a última amostra) e o jeito certo (rate + subquery).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "ts_of_max_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"ts_of_max_over_time_node_cpu_utilisation_percent",
				"Uso de CPU do host em % (gauge; imita a recording rule instance:node_cpu_utilisation:rate5m × 100 do node-mixin).",
				prometheus.GaugeValue, []string{"host"},
				func(now time.Time) []Sample {
					c := math.Mod(float64(now.Unix()), 240)
					web1 := 30 + Noise(5)
					if c < 5 {
						web1 = 95
					}
					web2 := 40 + Noise(5)
					if c >= 120 && c < 180 {
						web2 = 80
					}
					return []Sample{
						{Labels: []string{"web-1"}, Value: web1},
						{Labels: []string{"web-2"}, Value: web2},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"ts_of_max_over_time_http_requests_total",
				"Requisições HTTP recebidas (counter). 10 req/s, rajada de 50 req/s por 20s a cada 4 min.",
				prometheus.CounterValue, []string{"host"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					// integral determinística: 10/s sempre + 40/s extras nos 20 primeiros s de cada ciclo de 240s
					burst := math.Floor(t/240)*20 + math.Min(math.Mod(t, 240), 20)
					return []Sample{{Labels: []string{"web-1"}, Value: math.Mod(10*t+40*burst, 1e9)}}
				},
			))
		},
	})
}
