package main

// Cenário da lição tanh().
//
// 1) tanh_queue_backlog_messages{queue}: backlog de uma fila. Fica em ~700 (±150) e, a
//    cada 5 min, explode até ~20 000 (pico de 90s). Escala enorme e sem teto.
//    tanh(backlog / 1000) vira um "índice de pressão" entre 0 e 1: 700 -> 0.60,
//    1000 -> 0.76, 3000 -> 0.995, 20000 -> 1.
// 2) tanh_error_ratio{service} (0..~0.1) e tanh_latency_p99_seconds{service}
//    (0.1..3 s): dois sinais com escalas totalmente diferentes, que viram um
//    "health score" único somando tanh(sinal / referência).
//    A cada 4 min há uma degradação de 60s.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "tanh",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"tanh_queue_backlog_messages", "Mensagens aguardando na fila.",
				prometheus.GaugeValue, []string{"queue"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					p := math.Mod(t, 300)
					v := 700 + Noise(150)
					if p < 90 { // pico em forma de sino: sobe e desce
						v += 20000 * math.Pow(math.Sin(math.Pi*p/90), 2)
					}
					return []Sample{{Labels: []string{"pedidos"}, Value: math.Max(0, v)}}
				},
			))
			bad := func(now time.Time) bool { return math.Mod(float64(now.Unix()), 240) < 60 }
			reg.MustRegister(NewFunc(
				"tanh_error_ratio", "Fração de requisições com erro (0..1).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					v := 0.002 + Noise(0.001)
					if bad(now) {
						v = 0.08 + Noise(0.01)
					}
					return []Sample{{Labels: []string{"api"}, Value: math.Max(0, v)}}
				},
			))
			reg.MustRegister(NewFunc(
				"tanh_latency_p99_seconds", "Latência p99 (s).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					v := 0.12 + Noise(0.02)
					if bad(now) {
						v = 2.5 + Noise(0.3)
					}
					return []Sample{{Labels: []string{"api"}, Value: v}}
				},
			))
		},
	})
}
