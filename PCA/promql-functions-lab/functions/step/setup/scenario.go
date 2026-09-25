package main

// Cenário da lição step().
//
// step() devolve o INTERVALO ENTRE PONTOS (resolução) de uma consulta range.
// No Grafana ele muda com o zoom: 15 min -> 5s, 6h -> ~30s, 7 dias -> ~10 min.
//
// 1) step_http_requests_total -> counter, ~10 req/s (baseado no relógio de parede).
// 2) step_latency_seconds     -> gauge de latência: ~0.12s, mas com um PICO de 2.5s
//    que dura só 5s (um único scrape) a cada 47s. Picos curtos assim somem quando o
//    step do gráfico é maior que o scrape interval (a menos que você use max_over_time[step()]).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "step",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"step_http_requests_total",
				"Total de requisições HTTP (~10/s).",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					t := float64(now.Unix()) - 1.79e9
					return []Sample{{Value: 10*t + math.Floor(Wave(t, 90, 0, 30, 0))}}
				},
			))
			reg.MustRegister(NewFunc(
				"step_latency_seconds",
				"Latência da última requisição (s). Pico de 2.5s por 5s a cada 47s.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					if now.Unix()%47 < 5 {
						return []Sample{{Value: 2.5}}
					}
					return []Sample{{Value: 0.12 + Noise(0.02)}}
				},
			))
		},
	})
}
