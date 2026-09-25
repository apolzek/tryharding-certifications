package main

// Cenário da lição clamp_max().
//
// 1) clamp_max_http_request_duration_seconds_p99{service="checkout"} -> imita uma
//    recording rule de p99. Normalmente 0.2..0.3 s, mas a cada 2 min há 20 s de
//    PICO de 30 s (timeout do gateway de pagamento). O pico achata o gráfico.
//    clamp_max(x, 2) corta o pico em 2 s e o "dia a dia" volta a ficar visível.
//
// 2) clamp_max_container_cpu_usage_seconds_total{pod} -> imita
//    container_cpu_usage_seconds_total (cAdvisor): counter cuja taxa oscila entre
//    0.2 e 1.2 cores (período 4 min).
//    clamp_max_kube_pod_container_resource_requests{pod, resource="cpu"} -> imita
//    kube_pod_container_resource_requests: request de 0.5 core.
//    uso / request vai de 40% até 240%; clamp_max(..., 1) -> 0..100% para um gauge.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "clamp_max",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"clamp_max_http_request_duration_seconds_p99",
				"p99 de latência do serviço (imita uma recording rule de histogram_quantile).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					v := Wave(t, 90, 0.25, 0.05, 0) + Noise(0.01)
					if math.Mod(t, 120) < 20 {
						v = 30
					}
					return []Sample{{Labels: []string{"checkout"}, Value: v}}
				},
			))
			cpu := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "clamp_max_container_cpu_usage_seconds_total",
				Help: "Segundos de CPU consumidos pelo container (imita container_cpu_usage_seconds_total).",
			}, []string{"pod"})
			reg.MustRegister(cpu)
			Every(time.Second, func() {
				t := float64(time.Now().Unix())
				cpu.WithLabelValues("checkout-6f7c").Add(math.Max(0, Wave(t, 240, 0.7, 0.5, 0)))
			})
			reg.MustRegister(NewFunc(
				"clamp_max_kube_pod_container_resource_requests",
				"Request de recursos do container (imita kube_pod_container_resource_requests).",
				prometheus.GaugeValue, []string{"pod", "resource"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"checkout-6f7c", "cpu"}, Value: 0.5}}
				},
			))
		},
	})
}
