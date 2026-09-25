package main

// Cenário da lição double_exponential_smoothing() (antigo holt_winters).
//
// Valores calculados a partir do relógio de parede.
//
// 1) double_exponential_smoothing_node_load1{host="web-1"} -> GAUGE ruidoso com
//    TENDÊNCIA (imita o load average de 1 min do node_exporter): rampa em
//    triângulo de 6 min (3 min subindo de 1.0 a 7.0, 3 min descendo de volta)
//    + ruído de ±1.2. Ótimo para comparar sf (suavização) e tf (tendência).
//
// 2) double_exponential_smoothing_probe_duration_seconds{target} -> GAUGE
//    (imita o blackbox_exporter) que alterna entre 0.1s e 0.3s (DEGRAU) a cada
//    2 min, com ruído de ±0.015s. Mostra o "overshoot": com tf alto, a
//    suavização passa do ponto depois de uma mudança brusca.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "double_exponential_smoothing",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }

			reg.MustRegister(NewFunc(
				"double_exponential_smoothing_node_load1",
				"Load average de 1 min (gauge): rampa sobe/desce a cada 3 min + muito ruído.",
				prometheus.GaugeValue, []string{"host"},
				func(now time.Time) []Sample {
					s := Saw(secs(now), 360)   // 0..1 em 6 min
					tri := 1 - math.Abs(2*s-1) // triângulo 0..1..0
					v := 1 + 6*tri + Noise(1.2)
					return []Sample{{Labels: []string{"web-1"}, Value: math.Round(v*100) / 100}}
				},
			))

			reg.MustRegister(NewFunc(
				"double_exponential_smoothing_probe_duration_seconds",
				"Duração do probe HTTP em segundos (gauge): degrau 0.1 <-> 0.3s a cada 2 min + ruído.",
				prometheus.GaugeValue, []string{"target"},
				func(now time.Time) []Sample {
					v := 0.1 + 0.2*Square(secs(now), 240) + Noise(0.015)
					return []Sample{{Labels: []string{"https://checkout.exemplo.com"}, Value: math.Round(v*1000) / 1000}}
				},
			))
		},
	})
}
