package main

// Cenário da lição last_over_time().
//
// last_over_time_batch_items_processed{job} -> gauge ESPARSO: um batch job que só
// expõe a métrica por ~20s quando roda (depois a série some e fica "stale").
//   job="export": roda a cada 2 min
//   job="report": roda a cada 3 min
// O valor é o número de itens processados naquela execução e muda a cada rodada,
// num ciclo previsível:
//   export: 1000, 1250, 1500, 1750, 2000, 1000, ...
//   report: 300, 600, 900, 300, ...

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "last_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"last_over_time_batch_items_processed",
				"Itens processados pela última execução do batch job (só exposto por ~20s quando o job roda).",
				prometheus.GaugeValue, []string{"job_name"},
				func(now time.Time) []Sample {
					t := now.Unix()
					out := []Sample{}
					if t%120 < 20 {
						run := t / 120
						out = append(out, Sample{Labels: []string{"export"}, Value: float64(1000 + (run%5)*250)})
					}
					if t%180 < 20 {
						run := t / 180
						out = append(out, Sample{Labels: []string{"report"}, Value: float64(300 * (run%3 + 1))})
					}
					return out
				},
			))
		},
	})
}
