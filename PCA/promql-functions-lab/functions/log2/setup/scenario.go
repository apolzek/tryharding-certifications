package main

// Cenário da lição log2().
//
// 1) log2_kube_deployment_status_replicas{deployment="checkout"} -> imita
//    kube_deployment_status_replicas durante um pico com HPA agressivo: as réplicas
//    DOBRAM a cada minuto: 2 -> 4 -> 8 -> 16 -> 32, e o ciclo recomeça a cada 5 min.
//    log2(x) = 1, 2, 3, 4, 5 (escada linear);  log2(x / 2) = dobras desde o normal.
//
// 2) log2_container_memory_working_set_bytes{pod} -> imita container_memory_working_set_bytes:
//    4 pods com ~300 MiB, ~700 MiB, ~1.5 GiB e ~3.1 GiB (±3%, onda de 3 min).
//    2 ^ ceil(log2(x)) = próxima potência de 2 (512 MiB, 1 GiB, 2 GiB, 4 GiB)
//    -> sugestão de limite de memória.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "log2",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"log2_kube_deployment_status_replicas",
				"Réplicas do deployment (imita kube_deployment_status_replicas). Dobram a cada minuto.",
				prometheus.GaugeValue, []string{"deployment"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					step := math.Floor(math.Mod(t, 300) / 60) // 0..4
					return []Sample{{Labels: []string{"checkout"}, Value: math.Pow(2, 1+step)}}
				},
			))
			reg.MustRegister(NewFunc(
				"log2_container_memory_working_set_bytes",
				"Memória em uso pelo container (imita container_memory_working_set_bytes).",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					mib := func(v, ph float64) float64 { return v * (1 + 0.03*math.Sin(2*math.Pi*t/180+ph)) * 1024 * 1024 }
					return []Sample{
						{Labels: []string{"api-7d9f"}, Value: mib(300, 0)},
						{Labels: []string{"worker-5c2a"}, Value: mib(700, 1)},
						{Labels: []string{"search-9b1e"}, Value: mib(1536, 2)},
						{Labels: []string{"cache-3f8d"}, Value: mib(3174, 3)},
					}
				},
			))
		},
	})
}
