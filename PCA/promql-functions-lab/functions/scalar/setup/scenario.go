package main

// Cenário da lição scalar().
//
// 1) scalar_http_requests_total{service} -> counter (imita http_requests_total). rate:
//      checkout ≈ 60 · cart ≈ 30 · search ≈ 10   (total ≈ 100)
//    Para calcular "% do total" dividindo cada série por UM número.
//
// 2) scalar_node_cpu_usage_percent{node} -> CPU por nó:
//      node-1 85 ± 10 · node-2 65 ± 10 · node-3 40 ± 5   (ondas de 3 min)
//    scalar_cpu_alert_threshold_percent -> limiar vindo de uma MÉTRICA (1 série só):
//      alterna 80 / 70 a cada 2 min (alguém "reconfigurou" o alerta).
//
// 3) scalar_config_threshold_percent{replica="a"|"b"} -> o MESMO limiar (80),
//    mas exportado por 2 réplicas do serviço de config. 2 séries => scalar() = NaN.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "scalar",
		Setup: func(reg prometheus.Registerer) {
			// Counter calculado pelo relógio (desde a meia-noite UTC): restart do gerador
			// não perde requisições. Velocidade = base ± 5% (onda de 4 min).
			reg.MustRegister(NewFunc(
				"scalar_http_requests_total",
				"Total de requisições HTTP por serviço (counter, imita http_requests_total).",
				prometheus.CounterValue, []string{"service"},
				func(now time.Time) []Sample {
					y, m, d := now.UTC().Date()
					midnight := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
					sec := now.Sub(midnight).Seconds()
					c := func(base, phase float64) float64 {
						return math.Floor(base*sec - 0.05*base*240/(2*math.Pi)*math.Cos(2*math.Pi*sec/240+phase) + base)
					}
					return []Sample{
						{Labels: []string{"checkout"}, Value: c(60, 0), Created: midnight},
						{Labels: []string{"cart"}, Value: c(30, 2), Created: midnight},
						{Labels: []string{"search"}, Value: c(10, 4), Created: midnight},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"scalar_node_cpu_usage_percent",
				"Uso de CPU por nó (%).",
				prometheus.GaugeValue, []string{"node"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"node-1"}, Value: Wave(t, 180, 85, 10, 0)},
						{Labels: []string{"node-2"}, Value: Wave(t, 180, 65, 10, 2)},
						{Labels: []string{"node-3"}, Value: Wave(t, 180, 40, 5, 4)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"scalar_cpu_alert_threshold_percent",
				"Limiar de alerta de CPU (%), configurável em runtime. Uma única série.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					v := 80.0
					if Square(float64(now.Unix()), 240) == 0 {
						v = 70
					}
					return []Sample{{Value: v}}
				},
			))
			reg.MustRegister(NewFunc(
				"scalar_config_threshold_percent",
				"O mesmo limiar (80%), só que exportado por 2 réplicas -> 2 séries.",
				prometheus.GaugeValue, []string{"replica"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"a"}, Value: 80},
						{Labels: []string{"b"}, Value: 80},
					}
				},
			))
		},
	})
}
