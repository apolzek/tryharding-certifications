package main

// Cenário da lição ts_of_first_over_time() (experimental).
//
// ts_of_first_over_time_app_build_info{version} -> gauge "info" (valor sempre 1)
//   Um "deploy" novo a cada 3 min: o label version muda (1.<n>.0) e a versão anterior
//   SOME para sempre. ts_of_first_over_time mostra QUANDO cada versão apareceu.
//
// ts_of_first_over_time_process_up -> gauge (valor 1) que existe SEMPRE.
//   Serve para mostrar que o resultado é "cortado" no início da janela.

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "ts_of_first_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"ts_of_first_over_time_app_build_info",
				"Versão da aplicação em execução (valor sempre 1). Muda a cada 3 min.",
				prometheus.GaugeValue, []string{"version"},
				func(now time.Time) []Sample {
					n := (now.Unix() / 180) % 1000
					return []Sample{{Labels: []string{fmt.Sprintf("1.%d.0", n)}, Value: 1}}
				},
			))
			reg.MustRegister(NewFunc(
				"ts_of_first_over_time_process_up",
				"Processo de pé (sempre 1).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample { return []Sample{{Value: 1}} },
			))
		},
	})
}
