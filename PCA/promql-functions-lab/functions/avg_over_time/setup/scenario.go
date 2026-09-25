package main

// Cenário da lição avg_over_time().
//
// 1) avg_over_time_cpu_usage_percent{pod} -> gauge MUITO ruidoso (±15 a cada scrape)
//    pod-a ~20%, pod-b ~50%, pod-c ~80%. Serve para comparar:
//      avg_over_time(x[1m])  -> média de CADA pod ao longo do tempo (3 linhas suaves)
//      avg(x)                -> média ENTRE os pods em cada instante (1 linha ruidosa ~50)
//
// 2) avg_over_time_rabbitmq_queue_messages_ready -> senóide com período de 4 min (100 ± 80).
//    avg_over_time com janela = período (4m) vira uma linha reta em ~100.
//
// 3) avg_over_time_probe_success{service} -> 0/1 (health check).
//    checkout: sempre 1. search: fica 0 durante 30s a cada 2 min (75% de disponibilidade).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "avg_over_time",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"avg_over_time_cpu_usage_percent",
				"Uso de CPU por pod em % (gauge bem ruidoso).",
				prometheus.GaugeValue, []string{"pod"},
				func(now time.Time) []Sample {
					out := []Sample{}
					for _, p := range []struct {
						name string
						base float64
					}{{"pod-a", 20}, {"pod-b", 50}, {"pod-c", 80}} {
						v := math.Min(100, math.Max(0, p.base+Noise(15)))
						out = append(out, Sample{Labels: []string{p.name}, Value: math.Round(v*10) / 10})
					}
					return out
				},
			))

			reg.MustRegister(NewFunc(
				"avg_over_time_rabbitmq_queue_messages_ready",
				"Mensagens na fila (onda de 4 min: 20..180).",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					t := float64(now.Unix())
					return []Sample{{Value: math.Round(Wave(t, 240, 100, 80, 0) + Noise(5))}}
				},
			))

			reg.MustRegister(NewFunc(
				"avg_over_time_probe_success",
				"1 se o health check passou, 0 se falhou.",
				prometheus.GaugeValue, []string{"service"},
				func(now time.Time) []Sample {
					search := 1.0
					if now.Unix()%120 >= 90 { // 30s fora a cada 2 min
						search = 0
					}
					return []Sample{
						{Labels: []string{"checkout"}, Value: 1},
						{Labels: []string{"search"}, Value: search},
					}
				},
			))
		},
	})
}
