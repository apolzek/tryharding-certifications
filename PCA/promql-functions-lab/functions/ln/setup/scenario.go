package main

// Cenário da lição ln().
//
// 1) ln_rabbitmq_queue_messages{queue="payments.retry"} -> imita rabbitmq_queue_messages
//    durante uma TEMPESTADE DE RETRIES: cada falha gera novas tentativas, então a fila
//    cresce exponencialmente de 100 para 12 800 mensagens em 5 min (7 dobras, uma a cada
//    ~43 s) e é purgada. Taxa contínua r = ln(128)/300 ≈ 0.01617 /s.
//    ln(x) vira uma RETA; deriv(ln(x)) = r; tempo de dobra = ln(2)/r ≈ 43 s.
//
// 2) ln_probe_duration_seconds{target} -> imita probe_duration_seconds (blackbox_exporter):
//    4 sites respondem em 20..40 ms e 1 está quase em timeout (~8 s).
//    Média aritmética ≈ 1.6 s (dominada pelo outlier); média geométrica
//    exp(avg(ln(x))) ≈ 0.086 s.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "ln",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"ln_rabbitmq_queue_messages",
				"Mensagens na fila (imita rabbitmq_queue_messages) durante uma tempestade de retries.",
				prometheus.GaugeValue, []string{"queue"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					k := math.Log(128) / 300
					return []Sample{{Labels: []string{"payments.retry"}, Value: math.Round(100 * math.Exp(k*math.Mod(t, 300)))}}
				},
			))
			reg.MustRegister(NewFunc(
				"ln_probe_duration_seconds",
				"Duração da sonda HTTP em segundos (imita probe_duration_seconds do blackbox_exporter).",
				prometheus.GaugeValue, []string{"target"},
				func(now time.Time) []Sample {
					j := func(v float64) float64 { return v * (1 + Noise(0.05)) }
					return []Sample{
						{Labels: []string{"https://loja.exemplo.com"}, Value: j(0.020)},
						{Labels: []string{"https://api.exemplo.com"}, Value: j(0.030)},
						{Labels: []string{"https://cdn.exemplo.com"}, Value: j(0.025)},
						{Labels: []string{"https://auth.exemplo.com"}, Value: j(0.040)},
						{Labels: []string{"https://legado.exemplo.com"}, Value: j(8.0)},
					}
				},
			))
		},
	})
}
