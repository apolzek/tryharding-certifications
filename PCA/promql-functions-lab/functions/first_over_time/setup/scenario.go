package main

// Cenário da lição first_over_time().
//
// 1) first_over_time_queue_depth -> gauge contínuo (onda de 5 min, 100 ± 60).
//    Aqui first_over_time(x[2m]) ≈ x offset 2m (curva deslocada 2 min para a direita).
//
// 2) first_over_time_job_progress_percent{job_name="migration"} -> série que VIVE só
//    100s a cada 3 min: o job começa em 20% (retomado de um checkpoint) e sobe 0,8%/s
//    até 100%; depois a série some. É aqui que first_over_time e offset DIFEREM:
//      x offset 1m          -> vazio nos primeiros 60s de vida (não havia nada 1 min atrás)
//      first_over_time(x[1m]) -> já mostra 20 desde o 1º scrape.

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "first_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"first_over_time_queue_depth",
				"Mensagens na fila (onda de 5 min entre 40 e 160).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Value: math.Round(Wave(t, 300, 100, 60, 0))}}
				},
			))

			reg.MustRegister(NewFunc(
				"first_over_time_job_progress_percent",
				"Progresso do job de migração em %. A série só existe enquanto o job roda (100s a cada 3 min).",
				prometheus.GaugeValue, []string{"job_name"},
				func(now time.Time) []Sample {
					r := now.Unix() % 180
					if r >= 100 {
						return nil
					}
					return []Sample{{Labels: []string{"migration"}, Value: 20 + 0.8*float64(r)}}
				},
			))
		},
	})
}
