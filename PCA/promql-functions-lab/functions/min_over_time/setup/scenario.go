package main

// Cenário da lição min_over_time().
//
// 1) min_over_time_hikaricp_connections_idle{pool} -> conexões LIVRES no pool (gauge)
//    main:    ~40 livres, mas a cada 3 min (segundos 80..95 do ciclo) cai para 1 ou 2
//             por só 15s (quase esgotou!). A queda nunca cai numa virada de minuto.
//    replica: ~20 livres, estável.
//
// 2) min_over_time_probe_success{service} -> 0/1
//    db:  sempre 1.
//    api: fica 0 por 10s a cada 2 min (um "flap" rápido).
//    min_over_time(x[5m]) == 0  ->  "falhou pelo menos uma vez nos últimos 5 min".

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "min_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"min_over_time_hikaricp_connections_idle",
				"Conexões livres no pool do banco (gauge, máx. 50).",
				prometheus.GaugeValue, []string{"pool"},
				func(now time.Time) []Sample {
					main := math.Round(40 + Noise(3))
					if r := now.Unix() % 180; r >= 80 && r < 95 {
						main = math.Round(1.5 + Noise(0.6))
					}
					return []Sample{
						{Labels: []string{"main"}, Value: main},
						{Labels: []string{"replica"}, Value: math.Round(20 + Noise(2))},
					}
				},
			))

			reg.MustRegister(NewFunc(
				"min_over_time_probe_success",
				"1 se o health check passou, 0 se falhou.",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					api := 1.0
					if r := now.Unix() % 120; r >= 40 && r < 50 {
						api = 0
					}
					return []Sample{
						{Labels: []string{"db"}, Value: 1},
						{Labels: []string{"api"}, Value: api},
					}
				},
			))
		},
	})
}
