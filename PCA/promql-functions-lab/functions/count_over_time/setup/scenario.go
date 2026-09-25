package main

// Cenário da lição count_over_time().
//
// 1) count_over_time_node_hwmon_temp_celsius{exporter} -> gauge de um exporter
//    stable: aparece em TODO scrape (a cada 5s) -> count_over_time[1m] = 12
//    flaky:  SOME da exposição por 30s a cada 2 min (segundos 60..90 do ciclo),
//            como um exporter que falha ao coletar -> count_over_time[1m] cai até 6
//
// 2) count_over_time_request_latency_ms -> latência ~80 ms, mas nos primeiros 20s
//    de cada minuto fica ~300 ms (4 scrapes lentos por minuto).
//    count_over_time((x > 200)[5m:5s]) ≈ 20 amostras lentas em 5 min.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "count_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"count_over_time_node_hwmon_temp_celsius",
				"Temperatura lida por um exporter. O exporter 'flaky' some por 30s a cada 2 min.",
				prometheus.GaugeValue, []string{"exporter"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					out := []Sample{{Labels: []string{"stable"}, Value: math.Round(Wave(t, 300, 45, 5, 0)*10) / 10}}
					if r := now.Unix() % 120; r < 60 || r >= 90 {
						out = append(out, Sample{Labels: []string{"flaky"}, Value: math.Round(Wave(t, 300, 60, 5, 1)*10) / 10})
					}
					return out
				},
			))

			reg.MustRegister(NewFunc(
				"count_over_time_request_latency_ms",
				"Latência da última requisição em ms. ~80ms, mas ~300ms nos primeiros 20s de cada minuto.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					v := 80 + Noise(20)
					if now.Unix()%60 < 20 {
						v = 300 + Noise(30)
					}
					return []Sample{{Value: math.Round(v)}}
				},
			))
		},
	})
}
