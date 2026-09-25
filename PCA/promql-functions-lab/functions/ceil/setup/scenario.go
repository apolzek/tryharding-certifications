package main

// Cenário da lição ceil().  Imita um caso real de capacity planning no Kubernetes.
//
// 1) ceil_http_requests_total{service="checkout"} -> imita http_requests_total:
//    counter cuja VELOCIDADE oscila entre ~30 e ~450 req/s (onda de 4 min).
//    Teste de carga mostrou que 1 pod aguenta 60 req/s.
//    Réplicas necessárias = ceil(sum(rate(...)) / 60). Ex.: 130/60 = 2.17 -> 3 pods.
//
// 2) ceil_kube_deployment_spec_replicas{deployment="checkout"} -> imita o
//    kube-state-metrics: o deployment está fixo em 5 réplicas. Acima de 300 req/s
//    precisaríamos de 6, 7, 8 -> alerta "réplicas insuficientes".
//
// 3) ceil_storage_used_bytes{bucket="backups"} -> uso de disco que cresce de
//    0.3 GB até 5.1 GB em 5 min (e recomeça). O provedor cobra "por GB iniciado":
//    GB cobrados = ceil(bytes / 1e9) -> escadinha 1, 2, 3, 4, 5, 6.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "ceil",
		Setup: func(reg prometheus.Registerer) {
			reqs := prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: "ceil_http_requests_total",
				Help: "Requisições HTTP recebidas (imita http_requests_total).",
			}, []string{"service"})
			reg.MustRegister(reqs)
			Every(time.Second, func() {
				t := float64(time.Now().Unix())
				reqs.WithLabelValues("checkout").Add(math.Max(0, Wave(t, 240, 240, 210, 0)+Noise(3)))
			})

			reg.MustRegister(NewFunc(
				"ceil_kube_deployment_spec_replicas",
				"Réplicas desejadas do deployment (imita kube_deployment_spec_replicas).",
				prometheus.GaugeValue, []string{"deployment"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"checkout"}, Value: 5}}
				},
			))
			reg.MustRegister(NewFunc(
				"ceil_storage_used_bytes",
				"Bytes usados no bucket de armazenamento.",
				prometheus.GaugeValue, []string{"bucket"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Labels: []string{"backups"}, Value: 0.3e9 + 4.8e9*Saw(t, 300)}}
				},
			))
		},
	})
}
