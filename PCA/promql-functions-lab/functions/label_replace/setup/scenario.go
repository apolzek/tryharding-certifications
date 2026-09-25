package main

// Cenário da lição label_replace() — imita casos reais de Kubernetes + node_exporter.
//
// 1) label_replace_node_cpu_usage_percent{endpoint="10.0.0.5:9100"}
//    (imita métricas do node_exporter, cujo instance é "IP:porta")
//      10.0.0.5 ≈ 70%   10.0.0.6 ≈ 40%   10.0.0.7 ≈ 20%   (ondas de ±5)
//    label_replace_kube_node_info{node, internal_ip} = 1
//    (imita kube_node_info do kube-state-metrics, que traz o NOME do nó e o IP SEM porta)
//      worker-1 -> 10.0.0.5 · worker-2 -> 10.0.0.6 · worker-3 -> 10.0.0.7
//    Com label_replace extraímos o IP e fazemos o join para mostrar CPU por NOME de nó.
//
// 2) label_replace_container_memory_working_set_bytes{pod, image}
//    (imita container_memory_working_set_bytes do cAdvisor)
//      checkout: 200 + 220 + 180 = 600 MiB   (Deployment, pods "checkout-<hash>-<id>")
//      payments: 300 + 340       = 640 MiB
//      redis-0 : 512 MiB  (StatefulSet! a regex de Deployment NÃO casa)
//
// 3) Métricas que chamam a mesma coisa por nomes de label diferentes:
//      label_replace_http_requests_per_second{service="checkout"}               120 / payments 60 / cart 30
//      label_replace_kube_deployment_status_replicas_available{deployment="checkout"} 3 / payments 2 / cart 1
//    (imita kube_deployment_status_replicas_available do kube-state-metrics)
//    label_replace copia deployment -> service para o join (req/s por réplica: 40 / 30 / 30).

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "label_replace",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"label_replace_node_cpu_usage_percent",
				"Uso de CPU por nó (%). endpoint = IP:porta do node_exporter.",
				prometheus.GaugeValue, []string{"endpoint"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"10.0.0.5:9100"}, Value: Wave(t, 180, 70, 5, 0)},
						{Labels: []string{"10.0.0.6:9100"}, Value: Wave(t, 180, 40, 5, 2)},
						{Labels: []string{"10.0.0.7:9100"}, Value: Wave(t, 180, 20, 5, 4)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"label_replace_kube_node_info",
				"Info do nó (imita kube_node_info): nome do nó e IP interno, valor sempre 1.",
				prometheus.GaugeValue, []string{"node", "internal_ip"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"worker-1", "10.0.0.5"}, Value: 1},
						{Labels: []string{"worker-2", "10.0.0.6"}, Value: 1},
						{Labels: []string{"worker-3", "10.0.0.7"}, Value: 1},
					}
				},
			))

			const mib = 1024 * 1024
			reg.MustRegister(NewFunc(
				"label_replace_container_memory_working_set_bytes",
				"Memória (working set) por pod, em bytes. O nome do pod carrega o nome do deployment.",
				prometheus.GaugeValue, []string{"pod", "image"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					w := func(base, phase float64) float64 { return Wave(t, 240, base*mib, 4*mib, phase) }
					return []Sample{
						{Labels: []string{"checkout-7d9f8b6c4-x2k9p", "registry.local/shop/checkout:1.8.2"}, Value: w(200, 0)},
						{Labels: []string{"checkout-7d9f8b6c4-q8w7e", "registry.local/shop/checkout:1.8.2"}, Value: w(220, 2)},
						{Labels: []string{"checkout-7d9f8b6c4-m3n4b", "registry.local/shop/checkout:1.8.2"}, Value: w(180, 4)},
						{Labels: []string{"payments-5f6b7c8d9-a1b2c", "registry.local/shop/payments:2.1.0"}, Value: w(300, 1)},
						{Labels: []string{"payments-5f6b7c8d9-z9y8x", "registry.local/shop/payments:2.1.0"}, Value: w(340, 3)},
						{Labels: []string{"redis-0", "docker.io/library/redis:7.2"}, Value: w(512, 5)},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"label_replace_http_requests_per_second",
				"Requisições por segundo por serviço (label service).",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"checkout"}, Value: Wave(t, 300, 120, 6, 0)},
						{Labels: []string{"payments"}, Value: Wave(t, 300, 60, 3, 1)},
						{Labels: []string{"cart"}, Value: Wave(t, 300, 30, 1.5, 2)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"label_replace_kube_deployment_status_replicas_available",
				"Réplicas disponíveis por deployment (imita kube-state-metrics). Label deployment, não service!",
				prometheus.GaugeValue, []string{"deployment"},
				func(now time.Time) []Sample {
					return []Sample{
						{Labels: []string{"checkout"}, Value: 3},
						{Labels: []string{"payments"}, Value: 2},
						{Labels: []string{"cart"}, Value: 1},
					}
				},
			))
		},
	})
}
