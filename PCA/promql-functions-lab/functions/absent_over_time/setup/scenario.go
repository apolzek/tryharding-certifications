package main

// Cenário da lição absent_over_time().
//
// absent_over_time_backup_heartbeat{backup} -> gauge (1 = backup rodando/reportando)
//   backup="files" : sempre presente.
//   backup="db"    : ciclo de 5 min: reporta durante 60s e fica 240s em silêncio.
//     absent()                 -> dispara logo no 1º scrape sem dado (4 min por ciclo)
//     absent_over_time(...[2m]) -> só dispara após 2 min de silêncio (2 min por ciclo)
// A métrica "absent_over_time_nonexistent_metric" NUNCA é exposta.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "absent_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"absent_over_time_backup_heartbeat",
				"Heartbeat dos backups (1 = reportando). O backup db só reporta 1 min a cada 5 min.",
				prometheus.GaugeValue, []string{"backup"},
				func(now time.Time) []Sample {
					out := []Sample{{Labels: []string{"files"}, Value: 1}}
					if math.Mod(float64(now.Unix()), 300) < 60 {
						out = append(out, Sample{Labels: []string{"db"}, Value: 1})
					}
					return out
				},
			))
		},
	})
}
