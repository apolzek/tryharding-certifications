package main

// Cenário da lição min_of() — o "teto" entre dois escalares.
//
// 1) min_of_rabbitmq_queue_messages_ready -> mensagens prontas na fila (UMA série),
//    imita rabbitmq_queue_messages_ready. Onda de 0 a 1800 (5 min).
//    min_of_kube_horizontalpodautoscaler_spec_max_replicas = 10 -> imita o
//    kube-state-metrics (maxReplicas do HPA).
//    "Workers desejados" = ceil(fila/100) iria de 0 a 18, mas o HPA não passa de 10:
//      min_of(ceil(scalar(fila)/100), scalar(max_replicas))  -> TETO em 10.
//
// 2) Dois limites de rate limit, cada um numa métrica (1 série cada):
//      min_of_tenant_limit_rps = 500 (fixo, contrato do cliente)
//      min_of_global_limit_rps = 800 / 300, alternando a cada 2 min
//                                (o cluster reduz o limite global quando está sob pressão)
//    Limite efetivo = o MAIS RESTRITIVO = min_of(tenant, global) -> 500 / 300.
//
// 3) min_of_backend_limit_rps{replica="a"|"b"} = 400 -> 2 séries, então
//    scalar() vira NaN e min_of(NaN, 500) = NaN (o NaN "contamina").

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "min_of",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"min_of_rabbitmq_queue_messages_ready",
				"Mensagens prontas para consumo na fila (imita rabbitmq_queue_messages_ready).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: Wave(float64(now.Unix()), 300, 900, 900, 0)}}
				},
			))
			reg.MustRegister(NewFunc(
				"min_of_kube_horizontalpodautoscaler_spec_max_replicas",
				"maxReplicas do HPA (imita kube-state-metrics).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample { return []Sample{{Value: 10}} },
			))
			reg.MustRegister(NewFunc(
				"min_of_tenant_limit_rps",
				"Limite de requisições/s contratado pelo cliente.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample { return []Sample{{Value: 500}} },
			))
			reg.MustRegister(NewFunc(
				"min_of_global_limit_rps",
				"Limite global de requisições/s do cluster (cai sob pressão).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					v := 800.0
					if Square(float64(now.Unix()), 240) == 0 {
						v = 300
					}
					return []Sample{{Value: v}}
				},
			))
			reg.MustRegister(NewFunc(
				"min_of_backend_limit_rps",
				"Limite do backend, exportado por 2 réplicas (2 séries!).",
				prometheus.GaugeValue, []string{"replica"},
				func(now time.Time) []Sample {
					return []Sample{{Labels: []string{"a"}, Value: 400}, {Labels: []string{"b"}, Value: 400}}
				},
			))
		},
	})
}
