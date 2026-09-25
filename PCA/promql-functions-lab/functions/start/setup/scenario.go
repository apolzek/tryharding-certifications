package main

// Cenário da lição start().
//
// start() devolve o INÍCIO da janela de uma consulta range (a borda esquerda do gráfico).
//
// 1) start_orders_total -> counter de pedidos, ~3 pedidos/s (baseado no relógio de
//    parede, então não zera quando o gerador é reiniciado).
// 2) start_queue_depth  -> gauge: tamanho de uma fila, onda lenta 100 ± 40 (período 10 min).

import (
	"math"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	Register(Scenario{
		Name: "start",
		Setup: func(reg prometheus.Registerer) {
			reg.MustRegister(NewFunc(
				"start_orders_total",
				"Total de pedidos recebidos (~3/s).",
				prometheus.CounterValue, nil,
				func(now time.Time) []Sample {
					t := float64(now.Unix()) - 1.79e9
					return []Sample{{Value: math.Floor(3*t + 20*math.Sin(2*math.Pi*t/300))}}
				},
			))
			reg.MustRegister(NewFunc(
				"start_queue_depth",
				"Mensagens aguardando na fila.",
				prometheus.GaugeValue, nil,
				func(now time.Time) []Sample {
					return []Sample{{Value: math.Round(Wave(float64(now.Unix()), 600, 100, 40, 0) + Noise(3))}}
				},
			))
		},
	})
}
