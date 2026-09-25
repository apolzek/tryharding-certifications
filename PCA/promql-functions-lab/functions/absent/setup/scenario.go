package main

// Cenário da lição absent().
//
// absent_batch_heartbeat{batch} -> gauge (1 = "estou vivo"), exposto pelos jobs batch.
//   batch="reports" : sempre presente.
//   batch="billing" : presente 2 min, SOME 1 min (ciclo de 3 min). Simula um job que morreu.
// A métrica "absent_nonexistent_metric" NUNCA é exposta (para os exemplos da documentação).

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "absent",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"absent_batch_heartbeat",
				"Heartbeat dos jobs batch (1 = vivo). O batch billing some 1 min a cada 3 min.",
				prometheus.GaugeValue, []string{"batch"},
				func(now time.Time) []Sample {
					out := []Sample{{Labels: []string{"reports"}, Value: 1}}
					if (now.Unix()/60)%3 != 2 {
						out = append(out, Sample{Labels: []string{"billing"}, Value: 1})
					}
					return out
				},
			))
		},
	})
}
