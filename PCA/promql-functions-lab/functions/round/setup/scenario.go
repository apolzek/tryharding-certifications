package main

// Cenário da lição round().
//
// 1) round_node_hwmon_temp_celsius{chip,sensor} -> imita node_hwmon_temp_celsius
//    (node_exporter): temperatura com muitas casas decimais e ruído:
//    onda 62 ± 4 °C (período 3 min) + ruído de ±0.4 °C.
//    Serve para round(x), round(x, 0.5) e round(x, 5).
//
// 2) round_container_memory_working_set_bytes{pod} -> imita
//    container_memory_working_set_bytes (cAdvisor): 12 pods com memória entre
//    ~100 MiB e ~1.3 GiB, cada um oscilando ±40 MiB (período 4 min).
//    Bucketing: count_values("faixa_mib", round(x / 2^20, 256)) = quantos pods em
//    cada faixa de ~256 MiB.

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "round",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"round_node_hwmon_temp_celsius",
				"Temperatura do sensor de hardware (imita node_hwmon_temp_celsius).",
				prometheus.GaugeValue, []string{"chip", "sensor"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"platform_coretemp_0", "temp1"}, Value: Wave(t, 180, 62, 4, 0) + Noise(0.4)}}
				},
			))
			reg.MustRegister(NewFunc(
				"round_container_memory_working_set_bytes",
				"Memória em uso pelo container (imita container_memory_working_set_bytes).",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					out := make([]Sample, 0, 12)
					for i := 0; i < 12; i++ {
						mib := 100 + 110*float64(i) + Wave(t, 240, 0, 40, float64(i))
						out = append(out, Sample{Labels: []string{fmt.Sprintf("worker-%02d", i+1)}, Value: mib * 1024 * 1024})
					}
					return out
				},
			))
		},
	})
}
