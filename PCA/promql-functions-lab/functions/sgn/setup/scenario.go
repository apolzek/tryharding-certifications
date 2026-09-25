package main

// Cenário da lição sgn().
//
// 1) sgn_kafka_consumergroup_lag{consumergroup, topic} -> imita kafka_consumergroup_lag
//    (kafka_exporter): mensagens que o consumidor ainda não leu.
//      billing  : "triângulo" 0 -> 500 em 2 min (lag crescendo) e 500 -> 0 em 2 min (consumidor alcançando)
//      emails   : constante em 42 (estável)
//      payments : ~100 com ruído de ±2 (estável, mas "tremendo")
//    sgn(deriv(...)) mostra a TENDÊNCIA: +1 atrasando, -1 recuperando, 0 estável.
//
// 2) sgn_http_requests_total{service="api"} -> imita http_requests_total: counter cuja
//    velocidade é uma onda 100 ± 50 req/s com período de 10 min.
//    sgn(rate(x[1m]) - rate(x[1m] offset 5m)) = "mais ou menos tráfego que 5 min atrás?"
//    (na vida real seria offset 1w: "comparado com a semana passada").

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sgn",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"sgn_kafka_consumergroup_lag",
				"Lag do consumer group em mensagens (imita kafka_consumergroup_lag).",
				prometheus.GaugeValue, []string{"consumergroup", "topic"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					tri := 500 * (1 - math.Abs(2*Saw(t, 240)-1))
					return []Sample{
						{Labels: []string{"billing", "orders"}, Value: math.Round(tri)},
						{Labels: []string{"emails", "notifications"}, Value: 42},
						{Labels: []string{"payments", "orders"}, Value: 100 + Noise(2)},
					}
				},
			))
			reqs := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "sgn_http_requests_total",
				Help: "Requisições HTTP recebidas (imita http_requests_total).",
			}, []string{"service"})
			reg.MustRegister(reqs)
			Every(time.Second, func() {
				t := float64(time.Now().Unix())
				reqs.WithLabelValues("api").Add(math.Max(0, Wave(t, 600, 100, 50, 0)+Noise(2)))
			})
		},
	})
}
