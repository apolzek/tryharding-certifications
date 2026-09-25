package main

// Cenário da lição max_of() — o "piso" entre dois escalares.
//
// 1) max_of_node_cpu_usage_percent{node} -> CPU por nó:
//      node-1 82 ± 13 (69..95) · node-2 60 ± 10 (50..70)   (ondas de 3 min)
//    max_of_cpu_baseline_percent -> limiar "aprendido" (baseline + margem, como
//    uma recording rule de p95 semanal), UMA série, onda de 60 a 95 (5 min).
//    Quando o baseline cai para 60 o alerta ficaria sensível demais ->
//      max_of(scalar(baseline), 80) cria um PISO de 80.
//
// 2) max_of_rabbitmq_queue_messages_ready -> fila, UMA série, onda de 0 a 1000 (4 min).
//    max_of_kube_horizontalpodautoscaler_spec_min_replicas = 2 (imita kube-state-metrics).
//    Réplicas = ceil(fila/100) iria a 0 quando a fila zera; o mínimo é 2 ->
//      max_of(ceil(scalar(fila)/100), scalar(min_replicas)).
//
// 3) max_of_backup_threshold_percent{replica="a"|"b"} = 85 -> 2 séries, então
//    scalar() = NaN e max_of(NaN, 80) = NaN (o "piso" NÃO salva você do NaN).

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "max_of",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"max_of_node_cpu_usage_percent",
				"Uso de CPU por nó (%).",
				prometheus.GaugeValue, []string{"node"},
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{
						{Labels: []string{"node-1"}, Value: Wave(t, 180, 82, 13, 0)},
						{Labels: []string{"node-2"}, Value: Wave(t, 180, 60, 10, 2)},
					}
				},
			))
			reg.MustRegister(NewFunc(
				"max_of_cpu_baseline_percent",
				"Limiar de CPU aprendido (baseline + margem, %). Uma série.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: Wave(float64(now.Unix()), 300, 77.5, 17.5, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"max_of_rabbitmq_queue_messages_ready",
				"Mensagens prontas na fila (imita rabbitmq_queue_messages_ready).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: Wave(float64(now.Unix()), 240, 500, 500, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"max_of_kube_horizontalpodautoscaler_spec_min_replicas",
				"minReplicas do HPA (imita kube-state-metrics).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample { return []Sample{{Value: 2}} },
			))
			reg.MustRegister(NewFunc(
				"max_of_backup_threshold_percent",
				"Limiar exportado por 2 réplicas (2 séries!).",
				prometheus.GaugeValue, []string{"replica"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"a"}, Value: 85}, {Labels: []string{"b"}, Value: 85}}
				},
			))
		},
	})
}
