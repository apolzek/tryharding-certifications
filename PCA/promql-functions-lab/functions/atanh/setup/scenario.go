package main

// Cenário da lição atanh().
//
// 1) atanh_disk_utilization_ratio{disk}: ocupação de um disco (0..1). Enche de
//    0.30 até 1.00 em 3 min, fica 30s CHEIO (exatamente 1.0) e é limpo.
//    atanh(uso) vira um "índice de estresse" que explode perto de 100%:
//    0.5 -> 0.55, 0.9 -> 1.47, 0.99 -> 2.65, 1.0 -> +Inf.
// 2) atanh_correlation_coefficient{pair}: coeficiente de correlação de Pearson
//    (-1..1) calculado por um job de análise entre pares de métricas.
//    Para fazer MÉDIA de correlações, a estatística usa a transformação de
//    Fisher: tanh(avg(atanh(r))). Um dos pares tem um bug e reporta 1.02 -> NaN.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "atanh",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"atanh_disk_utilization_ratio", "Fração do disco ocupada (0..1).",
				prometheus.GaugeValue, []string{"disk"},
				func(now time.Time) []Sample {
					p := math.Mod(float64(now.UnixNano())/1e9, 240)
					v := 1.0
					if p < 180 {
						v = 0.30 + 0.70*p/180
						if v > 0.999 {
							v = 0.999
						}
					} else if p >= 210 {
						v = 0.30
					}
					return []Sample{{Labels: []string{"/data"}, Value: v}}
				},
			))
			reg.MustRegister(NewFunc(
				"atanh_correlation_coefficient", "Correlação de Pearson entre pares de métricas (-1..1).",
				prometheus.GaugeValue, []string{"pair"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{
						{Labels: []string{"cpu~latencia"}, Value: Wave(t, 240, 0.80, 0.15, 0)},
						{Labels: []string{"gc~latencia"}, Value: Wave(t, 240, 0.60, 0.20, 2)},
						{Labels: []string{"fila~latencia"}, Value: Wave(t, 240, 0.97, 0.02, 4)},
						{Labels: []string{"cache~latencia"}, Value: Wave(t, 240, -0.50, 0.30, 1)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"atanh_buggy_correlation_coefficient", "Correlação reportada por um job com bug (às vezes > 1).",
				prometheus.GaugeValue, []string{"pair"},
				func(now time.Time) []Sample {
					t := float64(now.UnixNano()) / 1e9
					return []Sample{{Labels: []string{"disco~latencia"}, Value: Wave(t, 120, 0.95, 0.07, 0)}}
				},
			))
		},
	})
}
