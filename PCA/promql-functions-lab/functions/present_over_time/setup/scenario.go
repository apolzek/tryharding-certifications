package main

// Cenário da lição present_over_time().
//
// present_over_time_worker_active_tasks{worker} -> gauge com quantas tarefas cada
// worker está executando. A série só existe enquanto o worker está vivo:
//   worker-a: sempre vivo
//   worker-b: vivo 60s, morto 120s (ciclo de 3 min)
//   worker-c: "cron": vivo só 20s a cada 5 min
// Valores: a=3, b=5, c=8. O VALOR não importa para present_over_time(): ele só pergunta "apareceu na janela?".

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "present_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"present_over_time_worker_active_tasks",
				"Tarefas em execução no worker. A série some quando o worker morre.",
				prometheus.GaugeValue, []string{"worker"},
				func(now time.Time) []Sample {
					t := now.Unix()
					out := []Sample{{Labels: []string{"worker-a"}, Value: 3}}
					if t%180 < 60 {
						out = append(out, Sample{Labels: []string{"worker-b"}, Value: 5})
					}
					if t%300 < 20 {
						out = append(out, Sample{Labels: []string{"worker-c"}, Value: 8})
					}
					return out
				},
			))
		},
	})
}
