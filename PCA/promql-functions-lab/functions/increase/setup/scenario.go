package main

// Cenário da lição increase().
//
// Valores calculados a partir do relógio de parede: restart do gerador não
// cria resets falsos.
//
// 1) increase_http_requests_total{code} -> imita o http_requests_total clássico
//    code="200"  +2 req/s (120/min)
//    code="500"  +1 erro a cada 10s (6/min) -> mostra a EXTRAPOLAÇÃO:
//                increase(...[1m]) dá 5.45, 6.55... e não "6" redondo.
//
// 2) increase_worker_jobs_processed_total{pod="worker-a"} -> 2 jobs/s e o pod
//    reinicia a cada 3 min (counter volta a 0). increase() compensa o reset;
//    a subtração ingênua "x - x offset 1m" fica NEGATIVA.
//
// 3) increase_kube_pod_container_status_restarts_total{namespace,pod,container}
//    imita o kube-state-metrics: pod "checkout-7d9f" reinicia a cada 2 min
//    (+1), pod "catalog-5b2c" nunca reinicia.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "increase",
		Setup: func(reg prometheus.Registerer) {
			secs := func(now time.Time) float64 { return float64(now.UnixMilli())/1000 - 1_790_000_000 }

			reg.MustRegister(NewFunc(
				"increase_http_requests_total",
				"Requisições HTTP por código de status (counter).",
				prometheus.CounterValue, []string{"code"},
				func(now time.Time) []Sample {
					t := secs(now)
					return []Sample{
						{Labels: []string{"200"}, Value: math.Floor(2 * t)},
						{Labels: []string{"500"}, Value: math.Floor(t / 10)},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"increase_worker_jobs_processed_total",
				"Jobs processados (2/s). O pod reinicia a cada 3 min (counter volta a zero).",
				prometheus.CounterValue, []string{"pod"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"worker-a"}, Value: math.Floor(2 * math.Mod(secs(now), 180))}}
				},
			))

			reg.MustRegister(NewFunc(
				"increase_kube_pod_container_status_restarts_total",
				"Restarts de container (imita kube-state-metrics). checkout reinicia a cada 2 min.",
				prometheus.CounterValue, []string{"namespace", "pod", "container"},
				func(now time.Time) []Sample {
					t := secs(now)
					return []Sample{
						{Labels: []string{"shop", "checkout-7d9f", "app"}, Value: math.Floor(t / 120)},
						{Labels: []string{"shop", "catalog-5b2c", "app"}, Value: 0},
					}
				},
			))
		},
	})
}
