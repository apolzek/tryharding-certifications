package main

// Cenário da lição irate().
//
// Todos os valores são calculados a partir do relógio de parede (now), então
// um restart do gerador NÃO bagunça a lição (não cria resets falsos).
//
// 1) irate_http_requests_total{route="/api/orders"} -> counter "nervoso":
//    base de 5 req/s, mas a cada 60s chega uma RAJADA de 100 req/s por 15s.
//    rate(...[1m]) vê sempre ~28.75 (média) e esconde as rajadas;
//    irate(...) mostra 5 -> 100 -> 5. (imita o http_requests_total clássico)
//
// 2) irate_node_network_transmit_errs_total{device="eth0"} -> counter LENTO
//    (imita o node_exporter): +1 erro de transmissão a cada 40s.
//    irate() vira um "pente" de picos (0 ou 0.2/s), ilegível;
//    rate(...[5m]) mostra a média honesta de 0.025 erros/s.
//
// 3) irate_worker_jobs_processed_total{pod="worker-a"} -> 3 jobs/s,
//    o pod reinicia a cada 2 min (counter volta a 0). irate() compensa o reset.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "irate",
		Setup: func(reg prometheus.Registerer) {
			// segundos desde uma época fixa (set/2026) -> counters pequenos e monotônicos
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }

			reg.MustRegister(NewFunc(
				"irate_http_requests_total",
				"Requisições HTTP (counter). 5 req/s de base + rajada de 100 req/s por 15s a cada 60s.",
				prometheus.CounterValue, []string{"route"},
				func(now time.Time) []Sample {
					t := secs(now)
					const base, burst, burstLen, period = 5.0, 100.0, 15.0, 60.0
					cycles := math.Floor(t / period)
					inBurst := math.Min(math.Mod(t, period), burstLen)
					v := base*t + (burst-base)*(cycles*burstLen+inBurst)
					return []Sample{{Labels: []string{"/api/orders"}, Value: math.Floor(v)}}
				},
			))

			reg.MustRegister(NewFunc(
				"irate_node_network_transmit_errs_total",
				"Erros de transmissão na interface (counter lento, imita node_exporter): +1 a cada 40s.",
				prometheus.CounterValue, []string{"device"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"eth0"}, Value: math.Floor(secs(now) / 40)}}
				},
			))

			reg.MustRegister(NewFunc(
				"irate_worker_jobs_processed_total",
				"Jobs processados (3/s). O pod reinicia a cada 2 min (counter volta a zero).",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"worker-a"}, Value: math.Floor(3 * math.Mod(secs(now), 120))}}
				},
			))
		},
	})
}
