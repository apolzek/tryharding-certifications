package main

// Cenário da lição sqrt().
//
// 1) sqrt_cpu_usage_percent{host} -> uso de CPU com ruído uniforme:
//      calmo   : 40 ± 2   (desvio padrão teórico = 2/√3  ≈ 1.15 pontos)
//      agitado : 50 ± 15  (desvio padrão teórico = 15/√3 ≈ 8.66 pontos)
//    stdvar_over_time dá a VARIÂNCIA (%²); sqrt() volta para desvio padrão (%).
//
// 2) sqrt_ac_voltage_volts{phase="L1"} -> tensão alternada "em câmera lenta":
//    senóide de pico 311 V (período 60 s). Média ≈ 0, mas RMS = 311/√2 ≈ 220 V.
//
// 3) sqrt_vibration_mm_s{motor="bomba-1", axis} -> vibração em 3 eixos:
//      x = 3, y = 4, z = 0 ou 12 (liga/desliga a cada 2 min)
//    Magnitude total = sqrt(x² + y² + z²) = 5 (triângulo 3-4-5) ou 13 (3-4-12-13).

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "sqrt",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"sqrt_cpu_usage_percent",
				"Uso de CPU em %, com ruído.",
				prometheus.GaugeValue, []string{"host"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"calmo"}, Value: 40 + Noise(2)},
						{Labels: []string{"agitado"}, Value: 50 + Noise(15)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"sqrt_ac_voltage_volts",
				"Tensão instantânea da rede elétrica (senóide de pico 311 V, em câmera lenta: período 60 s).",
				prometheus.GaugeValue, []string{"phase"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"L1"}, Value: Wave(t, 60, 0, 311, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"sqrt_vibration_mm_s",
				"Velocidade de vibração do motor em cada eixo, em mm/s.",
				prometheus.GaugeValue, []string{"motor", "axis"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"bomba-1", "x"}, Value: 3},
						{Labels: []string{"bomba-1", "y"}, Value: 4},
						{Labels: []string{"bomba-1", "z"}, Value: 12 * Square(t, 240)},
					}
				},
			))
		},
	})
}
